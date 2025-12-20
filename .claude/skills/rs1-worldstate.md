# RS-1 WorldState Streaming Development

This skill provides guidance for implementing the WebRTC WorldState DataChannel for the OpticWorks RS-1 platform.

## Overview

The WorldState channel:
- Streams tracked object positions at 30Hz
- Uses Protocol Buffers for efficient serialization
- Integrates with existing Pion WebRTC infrastructure
- Supports multiple concurrent clients

## Key Files

| File | Purpose |
|------|---------|
| `worldstate.go` | WorldState streaming implementation |
| `webrtc.go` | DataChannel setup (modified) |
| `internal/native/proto/native.proto` | Protobuf message definitions |
| `internal/fusion/worldstate.go` | WorldState types |

## Protobuf Schema

### Message Definitions

Add to `internal/native/proto/native.proto`:

```protobuf
message TrackedObject {
  int32 track_id = 1;
  float x = 2;
  float y = 3;
  float vx = 4;
  float vy = 5;
  int32 class_id = 6;
  float confidence = 7;
  bool is_radar_only = 8;
  bool is_vision_only = 9;
  float heading = 10;
  float speed = 11;
}

message WorldState {
  repeated TrackedObject objects = 1;
  int64 timestamp_ns = 2;
  uint64 frame_number = 3;
  int32 radar_targets = 4;
  int32 vision_detections = 5;
  float fusion_latency_ms = 6;
}
```

### Generate Go Code

```bash
protoc --go_out=. --go-grpc_out=. internal/native/proto/native.proto
```

## WebRTC DataChannel Setup

### Modify `webrtc.go`

```go
// Add to Session struct
type Session struct {
    // ... existing fields ...
    WorldStateChannel *webrtc.DataChannel
}

// In handleDataChannel function
func handleDataChannel(d *webrtc.DataChannel, session *Session) {
    switch d.Label() {
    // ... existing cases ...

    case "worldstate":
        session.WorldStateChannel = d
        d.OnOpen(func() {
            logger.Debug().Str("label", d.Label()).Msg("worldstate channel opened")
            go session.streamWorldState()
        })
        d.OnClose(func() {
            logger.Debug().Str("label", d.Label()).Msg("worldstate channel closed")
            session.WorldStateChannel = nil
        })
    }
}
```

## Streaming Implementation

### `worldstate.go`

```go
package kvm

import (
    "math"
    "time"

    "github.com/pion/webrtc/v4"
    "google.golang.org/protobuf/proto"

    pb "github.com/jetkvm/kvm/targets/rv1106/native/proto"
    "github.com/jetkvm/kvm/internal/fusion"
)

var (
    fusionEngine *fusion.FusionEngine
    frameCounter uint64
)

func initWorldState(radar *radar.State) {
    fusionEngine = fusion.NewFusionEngine(radar)
    fusionEngine.Start()
}

func (s *Session) streamWorldState() {
    ticker := time.NewTicker(33 * time.Millisecond) // 30Hz
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            if s.WorldStateChannel == nil {
                return
            }
            if s.WorldStateChannel.ReadyState() != webrtc.DataChannelStateOpen {
                return
            }

            if err := s.sendWorldState(); err != nil {
                logger.Debug().Err(err).Msg("worldstate send failed")
                return
            }
        }
    }
}

func (s *Session) sendWorldState() error {
    state := fusionEngine.GetWorldState()
    frameCounter++

    pbState := convertToProto(state, frameCounter)

    data, err := proto.Marshal(pbState)
    if err != nil {
        return err
    }

    return s.WorldStateChannel.Send(data)
}

func convertToProto(state fusion.WorldState, frameNum uint64) *pb.WorldState {
    pbState := &pb.WorldState{
        TimestampNs: state.Timestamp.UnixNano(),
        FrameNumber: frameNum,
        Objects:     make([]*pb.TrackedObject, len(state.Objects)),
    }

    for i, obj := range state.Objects {
        // Calculate heading and speed from velocity
        heading := math.Atan2(obj.VX, obj.VY) * 180 / math.Pi
        speed := math.Sqrt(obj.VX*obj.VX + obj.VY*obj.VY)

        pbState.Objects[i] = &pb.TrackedObject{
            TrackId:      int32(obj.TrackID),
            X:            float32(obj.X),
            Y:            float32(obj.Y),
            Vx:           float32(obj.VX),
            Vy:           float32(obj.VY),
            ClassId:      int32(obj.ClassID),
            Confidence:   float32(obj.Confidence),
            IsRadarOnly:  obj.IsRadarOnly,
            IsVisionOnly: obj.IsVisionOnly,
            Heading:      float32(heading),
            Speed:        float32(speed),
        }
    }

    return pbState
}
```

## Client Integration

### JavaScript/TypeScript

```typescript
// Using protobufjs
import { WorldState } from './proto/native';

class WorldStateClient {
    private channel: RTCDataChannel | null = null;
    private onStateCallback: ((state: WorldState) => void) | null = null;

    connect(pc: RTCPeerConnection) {
        this.channel = pc.createDataChannel('worldstate', {
            ordered: true,
        });
        this.channel.binaryType = 'arraybuffer';

        this.channel.onmessage = (event) => {
            const data = new Uint8Array(event.data);
            try {
                const state = WorldState.decode(data);
                this.onStateCallback?.(state);
            } catch (err) {
                console.error('Failed to decode WorldState:', err);
            }
        };
    }

    onWorldState(callback: (state: WorldState) => void) {
        this.onStateCallback = callback;
    }
}

// Usage
const client = new WorldStateClient();
client.onWorldState((state) => {
    console.log(`Frame ${state.frameNumber}: ${state.objects.length} objects`);

    for (const obj of state.objects) {
        console.log(`  Track ${obj.trackId}: (${obj.x.toFixed(2)}, ${obj.y.toFixed(2)})`);
    }
});
```

### Swift (iOS)

```swift
import SwiftProtobuf
import WebRTC

class WorldStateHandler: NSObject, RTCDataChannelDelegate {
    var onWorldState: ((Native_WorldState) -> Void)?

    func dataChannel(_ dataChannel: RTCDataChannel,
                     didReceiveMessageWith buffer: RTCDataBuffer) {
        guard let data = buffer.data as Data? else { return }

        do {
            let state = try Native_WorldState(serializedData: data)
            DispatchQueue.main.async {
                self.onWorldState?(state)
            }
        } catch {
            print("Decode error: \(error)")
        }
    }
}
```

## Channel Configuration Options

### Reliability Modes

```go
// Reliable ordered (default) - guarantees delivery
channelConfig := &webrtc.DataChannelInit{
    Ordered: func() *bool { b := true; return &b }(),
}

// Unreliable ordered - lower latency, some drops OK
channelConfig := &webrtc.DataChannelInit{
    Ordered:        func() *bool { b := true; return &b }(),
    MaxRetransmits: func() *uint16 { n := uint16(0); return &n }(),
}

// Unreliable unordered - lowest latency
channelConfig := &webrtc.DataChannelInit{
    Ordered:        func() *bool { b := false; return &b }(),
    MaxRetransmits: func() *uint16 { n := uint16(0); return &n }(),
}
```

### Bandwidth Estimation

```
Typical message size: 50-200 bytes (1-5 objects)
At 30Hz: 1.5-6 KB/s
Negligible compared to video stream (~500 KB/s)
```

## JSON-RPC Fallback

For clients that can't use DataChannels:

```go
// In jsonrpc.go
func rpcGetWorldState() (interface{}, error) {
    state := fusionEngine.GetWorldState()
    return map[string]interface{}{
        "objects":     state.Objects,
        "frame_number": state.FrameNum,
        "timestamp":   state.Timestamp.UnixMilli(),
    }, nil
}

// Register handler
rpcHandlers["getWorldState"] = RPCHandler{Func: rpcGetWorldState}
```

## Synchronization with Video

### Timestamp Alignment

```typescript
class SynchronizedRenderer {
    private videoTimestamp: number = 0;
    private worldStates: Map<number, WorldState> = new Map();

    onVideoFrame(frame: VideoFrame) {
        this.videoTimestamp = frame.timestamp;
        this.render();
    }

    onWorldState(state: WorldState) {
        this.worldStates.set(state.timestampNs, state);
        // Keep last 10 states for interpolation
        this.pruneOldStates();
        this.render();
    }

    render() {
        // Find closest WorldState to video timestamp
        const closest = this.findClosestState(this.videoTimestamp);
        if (closest) {
            this.drawOverlay(closest);
        }
    }

    private findClosestState(videoTs: number): WorldState | null {
        let best: WorldState | null = null;
        let bestDiff = Infinity;

        for (const [ts, state] of this.worldStates) {
            const diff = Math.abs(ts / 1e6 - videoTs);  // Convert ns to ms
            if (diff < bestDiff) {
                bestDiff = diff;
                best = state;
            }
        }

        return best;
    }
}
```

## Metrics and Monitoring

```go
var (
    worldstateFramesSent = promauto.NewCounter(prometheus.CounterOpts{
        Name: "rs1_worldstate_frames_total",
        Help: "Total WorldState frames sent",
    })
    worldstateClientsGauge = promauto.NewGauge(prometheus.GaugeOpts{
        Name: "rs1_worldstate_clients",
        Help: "Current WorldState channel clients",
    })
    worldstateLatency = promauto.NewHistogram(prometheus.HistogramOpts{
        Name:    "rs1_worldstate_send_latency_seconds",
        Help:    "WorldState send latency",
        Buckets: prometheus.ExponentialBuckets(0.0001, 2, 10),
    })
)

func (s *Session) sendWorldState() error {
    start := time.Now()
    defer func() {
        worldstateLatency.Observe(time.Since(start).Seconds())
    }()

    // ... send logic ...
    worldstateFramesSent.Inc()
    return nil
}
```

## Testing

### Unit Tests

```go
func TestProtoConversion(t *testing.T) {
    state := fusion.WorldState{
        Objects: []fusion.TrackedObject{
            {TrackID: 1, X: 2.5, Y: 3.0, ClassID: 0},
        },
        Timestamp: time.Now(),
    }

    pb := convertToProto(state, 100)

    assert.Equal(t, uint64(100), pb.FrameNumber)
    assert.Len(t, pb.Objects, 1)
    assert.Equal(t, int32(1), pb.Objects[0].TrackId)
    assert.InDelta(t, 2.5, float64(pb.Objects[0].X), 0.001)
}
```

### E2E Tests

```go
func TestWorldStateE2E(t *testing.T) {
    // Create test WebRTC connection
    pc, _ := webrtc.NewPeerConnection(webrtc.Configuration{})
    defer pc.Close()

    // Create data channel
    dc, _ := pc.CreateDataChannel("worldstate", nil)

    received := make(chan *pb.WorldState, 1)
    dc.OnMessage(func(msg webrtc.DataChannelMessage) {
        state := &pb.WorldState{}
        proto.Unmarshal(msg.Data, state)
        received <- state
    })

    // ... setup signaling ...

    // Wait for data
    select {
    case state := <-received:
        assert.NotNil(t, state)
        assert.Greater(t, state.FrameNumber, uint64(0))
    case <-time.After(5 * time.Second):
        t.Fatal("Timeout waiting for WorldState")
    }
}
```

## References

- [WORLDSTATE_PROTOCOL.md](../docs/rs1/WORLDSTATE_PROTOCOL.md) - Protocol specification
- [RS1_ARCHITECTURE.md](../docs/rs1/RS1_ARCHITECTURE.md) - System overview
- Pion WebRTC documentation: https://github.com/pion/webrtc
- Protocol Buffers documentation: https://protobuf.dev/
