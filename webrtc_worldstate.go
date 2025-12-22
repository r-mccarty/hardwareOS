package kvm

import (
	"encoding/json"
	"math"
	"time"

	"github.com/pion/webrtc/v4"
)

// worldStateMessage is the JSON format for worldstate streaming
// We use JSON instead of protobuf for simplicity and browser compatibility
type worldStateMessage struct {
	Timestamp     int64               `json:"timestamp_ns"`
	FrameNumber   uint64              `json:"frame_number"`
	Objects       []trackedObjectJSON `json:"objects"`
	OccupantCount int                 `json:"occupant_count"`
}

type trackedObjectJSON struct {
	TrackID      string  `json:"track_id"`
	X            float64 `json:"x"`
	Y            float64 `json:"y"`
	VX           float64 `json:"vx"`
	VY           float64 `json:"vy"`
	Confidence   float64 `json:"confidence"`
	IsRadarOnly  bool    `json:"is_radar_only"`
	IsVisionOnly bool    `json:"is_vision_only"`
	Heading      float64 `json:"heading"`
	Speed        float64 `json:"speed"`
}

// streamWorldState streams worldstate updates to the client at 30Hz
func (s *Session) streamWorldState() {
	ticker := time.NewTicker(33 * time.Millisecond) // ~30Hz
	defer ticker.Stop()

	var frameCounter uint64

	for {
		select {
		case <-ticker.C:
			// Check if channel is still open
			if s.WorldStateChannel == nil ||
				s.WorldStateChannel.ReadyState() != webrtc.DataChannelStateOpen {
				webrtcLogger.Debug().Msg("worldstate channel no longer open, stopping stream")
				return
			}

			// Get worldstate from RS-1 product
			snapshot := getRS1WorldStateSnapshot()
			if snapshot == nil {
				continue
			}

			frameCounter++

			// Build message
			msg := worldStateMessage{
				Timestamp:     snapshot.Timestamp.UnixNano(),
				FrameNumber:   frameCounter,
				Objects:       make([]trackedObjectJSON, 0, len(snapshot.Occupants)),
				OccupantCount: snapshot.OccupantCount,
			}

			for _, occ := range snapshot.Occupants {
				obj := trackedObjectJSON{
					TrackID:      occ.ID,
					X:            occ.X,
					Y:            occ.Y,
					Confidence:   occ.Confidence,
					IsRadarOnly:  occ.IsRadarOnly,
					IsVisionOnly: occ.IsVisionOnly,
				}

				if occ.Velocity != nil {
					obj.VX = occ.Velocity.X
					obj.VY = occ.Velocity.Y
					obj.Speed = occ.Velocity.Speed
					// Calculate heading (degrees from +Y axis)
					if obj.Speed > 0.01 {
						obj.Heading = math.Atan2(obj.VX, obj.VY) * 180.0 / math.Pi
					}
				}

				msg.Objects = append(msg.Objects, obj)
			}

			// Marshal and send
			data, err := json.Marshal(msg)
			if err != nil {
				webrtcLogger.Warn().Err(err).Msg("failed to marshal worldstate")
				continue
			}

			if err := s.WorldStateChannel.Send(data); err != nil {
				webrtcLogger.Debug().Err(err).Msg("failed to send worldstate, stopping stream")
				return
			}
		}
	}
}

// RS-1 product accessor - will be set from main.go
var rs1WorldStateGetter func() interface{}

// SetRS1WorldStateGetter sets the function to get RS-1 world state snapshots
func SetRS1WorldStateGetter(getter func() interface{}) {
	rs1WorldStateGetter = getter
}

// getRS1WorldStateSnapshot returns the current world state snapshot
// Returns nil if RS-1 product is not initialized
func getRS1WorldStateSnapshot() *worldStateSnapshotInternal {
	if rs1WorldStateGetter == nil {
		return nil
	}

	snapshot := rs1WorldStateGetter()
	if snapshot == nil {
		return nil
	}

	// Type assert to internal type
	// This is a workaround to avoid circular imports
	if ws, ok := snapshot.(*worldStateSnapshotInternal); ok {
		return ws
	}

	// Try to convert from RS-1 WorldStateSnapshot
	// Use reflection-free approach by checking for expected structure
	return convertToInternal(snapshot)
}

// worldStateSnapshotInternal mirrors the RS-1 WorldStateSnapshot type
type worldStateSnapshotInternal struct {
	Timestamp     time.Time
	Occupants     []occupantInternal
	OccupantCount int
}

type occupantInternal struct {
	ID           string
	X            float64
	Y            float64
	Confidence   float64
	Source       string
	Velocity     *velocityInternal
	IsRadarOnly  bool
	IsVisionOnly bool
}

type velocityInternal struct {
	X     float64
	Y     float64
	Speed float64
}

// convertToInternal converts the RS-1 WorldStateSnapshot to internal type
func convertToInternal(snapshot interface{}) *worldStateSnapshotInternal {
	// Use JSON as a bridge to avoid circular imports
	data, err := json.Marshal(snapshot)
	if err != nil {
		return nil
	}

	var internal worldStateSnapshotInternal
	if err := json.Unmarshal(data, &internal); err != nil {
		return nil
	}

	return &internal
}
