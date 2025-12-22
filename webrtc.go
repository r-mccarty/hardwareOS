package kvm

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/gin-gonic/gin"
	"github.com/jetkvm/kvm/internal/diagnostics"
	"github.com/jetkvm/kvm/platform/logging"
	"github.com/pion/webrtc/v4"
	"github.com/rs/zerolog"
)

type Session struct {
	peerConnection    *webrtc.PeerConnection
	VideoTrack        *webrtc.TrackLocalStaticSample
	ControlChannel    *webrtc.DataChannel
	RPCChannel        *webrtc.DataChannel
	WorldStateChannel *webrtc.DataChannel // RS-1: WorldState streaming channel

	rpcQueue chan webrtc.DataChannelMessage

	lastKeepAliveArrivalTime time.Time  // Track when last keep-alive packet arrived
	lastTimerResetTime       time.Time  // Track when auto-release timer was last reset
	keepAliveJitterLock      sync.Mutex // Protect jitter compensation timing state
}

var (
	actionSessions      int = 0
	activeSessionsMutex     = &sync.Mutex{}
)

func incrActiveSessions() int {
	activeSessionsMutex.Lock()
	defer activeSessionsMutex.Unlock()

	actionSessions++
	return actionSessions
}

func decrActiveSessions() int {
	activeSessionsMutex.Lock()
	defer activeSessionsMutex.Unlock()

	actionSessions--
	return actionSessions
}

func getActiveSessions() int {
	activeSessionsMutex.Lock()
	defer activeSessionsMutex.Unlock()

	return actionSessions
}

// GetDiagnosticsInfo returns WebRTC diagnostic info for the diagnostics package.
func (s *Session) GetDiagnosticsInfo() diagnostics.SessionInfo {
	info := diagnostics.SessionInfo{
		HasCurrentSession: true,
	}

	if s.peerConnection != nil {
		pc := s.peerConnection
		info.ICEConnectionState = pc.ICEConnectionState().String()
		info.SignalingState = pc.SignalingState().String()
		info.ConnectionState = pc.ConnectionState().String()

		var channels []diagnostics.DataChannelInfo
		if s.ControlChannel != nil {
			channels = append(channels, diagnostics.DataChannelInfo{
				Label: s.ControlChannel.Label(),
				State: s.ControlChannel.ReadyState().String(),
			})
		}
		if s.RPCChannel != nil {
			channels = append(channels, diagnostics.DataChannelInfo{
				Label: s.RPCChannel.Label(),
				State: s.RPCChannel.ReadyState().String(),
			})
		}
		if s.WorldStateChannel != nil {
			channels = append(channels, diagnostics.DataChannelInfo{
				Label: s.WorldStateChannel.Label(),
				State: s.WorldStateChannel.ReadyState().String(),
			})
		}
		info.DataChannels = channels
	}

	return info
}

func (s *Session) resetKeepAliveTime() {
	s.keepAliveJitterLock.Lock()
	defer s.keepAliveJitterLock.Unlock()
	s.lastKeepAliveArrivalTime = time.Time{} // Reset keep-alive timing tracking
	s.lastTimerResetTime = time.Time{}       // Reset auto-release timer tracking
}

type SessionConfig struct {
	ICEServers []string
	LocalIP    string
	IsCloud    bool
	ws         *websocket.Conn
	Logger     *zerolog.Logger
}

func (s *Session) ExchangeOffer(offerStr string) (string, error) {
	b, err := base64.StdEncoding.DecodeString(offerStr)
	if err != nil {
		return "", err
	}
	offer := webrtc.SessionDescription{}
	err = json.Unmarshal(b, &offer)
	if err != nil {
		return "", err
	}
	// Set the remote SessionDescription
	if err = s.peerConnection.SetRemoteDescription(offer); err != nil {
		return "", err
	}

	// Create answer
	answer, err := s.peerConnection.CreateAnswer(nil)
	if err != nil {
		return "", err
	}

	// Sets the LocalDescription, and starts our UDP listeners
	if err = s.peerConnection.SetLocalDescription(answer); err != nil {
		return "", err
	}

	localDescription, err := json.Marshal(s.peerConnection.LocalDescription())
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(localDescription), nil
}

func newSession(config SessionConfig) (*Session, error) {
	webrtcSettingEngine := webrtc.SettingEngine{
		LoggerFactory: logging.GetPionDefaultLoggerFactory(),
	}
	iceServer := webrtc.ICEServer{}

	var scopedLogger *zerolog.Logger
	if config.Logger != nil {
		l := config.Logger.With().Str("component", "webrtc").Logger()
		scopedLogger = &l
	} else {
		scopedLogger = webrtcLogger
	}

	if config.IsCloud {
		if config.ICEServers == nil {
			scopedLogger.Info().Msg("ICE Servers not provided by cloud")
		} else {
			iceServer.URLs = config.ICEServers
			scopedLogger.Info().Interface("iceServers", iceServer.URLs).Msg("Using ICE Servers provided by cloud")
		}

		if config.LocalIP == "" || net.ParseIP(config.LocalIP) == nil {
			scopedLogger.Info().Str("localIP", config.LocalIP).Msg("Local IP address not provided or invalid, won't set NAT1To1IPs")
		} else {
			webrtcSettingEngine.SetNAT1To1IPs([]string{config.LocalIP}, webrtc.ICECandidateTypeSrflx)
			scopedLogger.Info().Str("localIP", config.LocalIP).Msg("Setting NAT1To1IPs")
		}
	}

	api := webrtc.NewAPI(webrtc.WithSettingEngine(webrtcSettingEngine))
	peerConnection, err := api.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{iceServer},
	})
	if err != nil {
		scopedLogger.Warn().Err(err).Msg("Failed to create PeerConnection")
		return nil, err
	}

	session := &Session{peerConnection: peerConnection}
	session.rpcQueue = make(chan webrtc.DataChannelMessage, 256)

	go func() {
		for msg := range session.rpcQueue {
			go onRPCMessage(msg, session)
		}
	}()

	peerConnection.OnDataChannel(func(d *webrtc.DataChannel) {
		defer func() {
			if r := recover(); r != nil {
				scopedLogger.Error().Interface("error", r).Msg("Recovered from panic in DataChannel handler")
			}
		}()

		scopedLogger.Info().Str("label", d.Label()).Uint16("id", *d.ID()).Msg("New DataChannel")

		switch d.Label() {
		case "rpc":
			session.RPCChannel = d
			d.OnMessage(func(msg webrtc.DataChannelMessage) {
				// Enqueue to ensure ordered processing
				session.rpcQueue <- msg
			})
			// Wait for channel to be open before sending initial state
			d.OnOpen(func() {
				triggerOTAStateUpdate(otaState.ToRPCState())
				triggerVideoStateUpdate()
				notifyFailsafeMode(session)
			})
		case "terminal":
			handleTerminalChannel(d)
		case "serial":
			handleSerialChannel(d)
		case "worldstate":
			session.WorldStateChannel = d
			d.OnOpen(func() {
				scopedLogger.Debug().Msg("worldstate channel opened")
				go session.streamWorldState()
			})
			d.OnClose(func() {
				scopedLogger.Debug().Msg("worldstate channel closed")
				session.WorldStateChannel = nil
			})
		}
	})

	session.VideoTrack, err = webrtc.NewTrackLocalStaticSample(webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeH264}, "video", "kvm")
	if err != nil {
		scopedLogger.Warn().Err(err).Msg("Failed to create VideoTrack")
		return nil, err
	}

	rtpSender, err := peerConnection.AddTrack(session.VideoTrack)
	if err != nil {
		scopedLogger.Warn().Err(err).Msg("Failed to add VideoTrack to PeerConnection")
		return nil, err
	}

	// Read incoming RTCP packets
	// Before these packets are returned they are processed by interceptors. For things
	// like NACK this needs to be called.
	go func() {
		rtcpBuf := make([]byte, 1500)
		for {
			if _, _, rtcpErr := rtpSender.Read(rtcpBuf); rtcpErr != nil {
				return
			}
		}
	}()
	var isConnected bool

	peerConnection.OnICECandidate(func(candidate *webrtc.ICECandidate) {
		scopedLogger.Info().Interface("candidate", candidate).Msg("WebRTC peerConnection has a new ICE candidate")
		if candidate != nil {
			err := wsjson.Write(context.Background(), config.ws, gin.H{"type": "new-ice-candidate", "data": candidate.ToJSON()})
			if err != nil {
				scopedLogger.Warn().Err(err).Msg("failed to write new-ice-candidate to WebRTC signaling channel")
			}
		}
	})

	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		scopedLogger.Info().Str("connectionState", connectionState.String()).Msg("ICE Connection State has changed")
		if connectionState == webrtc.ICEConnectionStateConnected {
			if !isConnected {
				isConnected = true
				onActiveSessionsChanged()
				if incrActiveSessions() == 1 {
					onFirstSessionConnected()
				}
			}
		}
		//state changes on closing browser tab disconnected->failed, we need to manually close it
		if connectionState == webrtc.ICEConnectionStateFailed {
			scopedLogger.Debug().Msg("ICE Connection State is failed, closing peerConnection")
			_ = peerConnection.Close()
		}
		if connectionState == webrtc.ICEConnectionStateClosed {
			scopedLogger.Debug().Msg("ICE Connection State is closed")
			if session == currentSession {
				currentSession = nil
			}
			// Stop RPC processor
			if session.rpcQueue != nil {
				close(session.rpcQueue)
				session.rpcQueue = nil
			}

			if isConnected {
				isConnected = false
				onActiveSessionsChanged()
				if decrActiveSessions() == 0 {
					scopedLogger.Info().Msg("last session disconnected, stopping video stream")
					onLastSessionDisconnected()
				}
			}
		}
	})
	return session, nil
}

func onActiveSessionsChanged() {
	notifyFailsafeMode(currentSession)
	requestDisplayUpdate(true, "active_sessions_changed")
}

func onFirstSessionConnected() {
	notifyFailsafeMode(currentSession)
	_ = nativeInstance.VideoStart()
	stopVideoSleepModeTicker()
}

func onLastSessionDisconnected() {
	_ = nativeInstance.VideoStop()
	startVideoSleepModeTicker()
}
