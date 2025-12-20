# WorldState Protocol

The RS-1 streams tracked object positions to connected clients via a WebRTC DataChannel using Protocol Buffers. This document specifies the protocol and integration details.

## Overview

```
Fusion Engine (30Hz) → Protobuf Serialize → WebRTC DataChannel → Browser/App
```

The `worldstate` channel provides:
- Real-time tracked object positions in room coordinates
- Object classification and confidence scores
- Velocity estimates
- Track lifecycle information

---

## Protobuf Schema

### `internal/native/proto/native.proto` additions

```protobuf
syntax = "proto3";

package native;

// TrackedObject represents a fused, tracked entity
message TrackedObject {
  int32 track_id = 1;       // Unique track identifier (stable across frames)
  float x = 2;              // Room X coordinate (meters from origin)
  float y = 3;              // Room Y coordinate (meters from origin)
  float vx = 4;             // X velocity (m/s)
  float vy = 5;             // Y velocity (m/s)
  int32 class_id = 6;       // Object class: -1=unknown, 0=person, 1=vehicle, 2=animal
  float confidence = 7;     // Detection confidence (0.0-1.0)
  bool is_radar_only = 8;   // True if only radar detects this object
  bool is_vision_only = 9;  // True if only camera detects this object
  float heading = 10;       // Movement direction (degrees, 0=+Y, 90=+X)
  float speed = 11;         // Speed magnitude (m/s)
}

// WorldState represents the complete tracked environment
message WorldState {
  repeated TrackedObject objects = 1;  // All currently tracked objects
  int64 timestamp_ns = 2;              // Unix timestamp in nanoseconds
  uint64 frame_number = 3;             // Monotonic frame counter
  int32 radar_targets = 4;             // Raw radar target count (for diagnostics)
  int32 vision_detections = 5;         // Raw vision detection count
  float fusion_latency_ms = 6;         // Processing latency
}
```

---

## WebRTC DataChannel Setup

### Go Backend (`webrtc.go`)

```go
case "worldstate":
    session.WorldStateChannel = d
    d.OnOpen(func() {
        logger.Debug().Msg("worldstate channel opened")
        go session.streamWorldState()
    })
    d.OnClose(func() {
        logger.Debug().Msg("worldstate channel closed")
        session.WorldStateChannel = nil
    })
```

### Streaming Implementation (`worldstate.go`)

```go
package kvm

import (
    "time"

    "github.com/pion/webrtc/v4"
    "google.golang.org/protobuf/proto"

    pb "github.com/jetkvm/kvm/internal/native/proto"
)

var fusionEngine *fusion.FusionEngine
var frameCounter uint64

func (s *Session) streamWorldState() {
    ticker := time.NewTicker(33 * time.Millisecond) // 30Hz
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            if s.WorldStateChannel == nil {
                return
            }

            state := fusionEngine.GetWorldState()
            frameCounter++

            pbState := &pb.WorldState{
                Objects:           make([]*pb.TrackedObject, len(state.Objects)),
                TimestampNs:       state.Timestamp.UnixNano(),
                FrameNumber:       frameCounter,
                FusionLatencyMs:   0, // TODO: measure
            }

            for i, obj := range state.Objects {
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
                    Heading:      float32(math.Atan2(obj.VX, obj.VY) * 180 / math.Pi),
                    Speed:        float32(math.Sqrt(obj.VX*obj.VX + obj.VY*obj.VY)),
                }
            }

            data, err := proto.Marshal(pbState)
            if err != nil {
                logger.Error().Err(err).Msg("failed to marshal worldstate")
                continue
            }

            err = s.WorldStateChannel.Send(data)
            if err != nil {
                logger.Debug().Err(err).Msg("worldstate send failed")
                return
            }
        }
    }
}
```

---

## Client Integration

### JavaScript/TypeScript

```typescript
import { WorldState, TrackedObject } from './proto/native_pb';

interface WorldStateHandler {
  onWorldState(state: WorldState): void;
}

class RS1Client {
  private worldStateChannel: RTCDataChannel | null = null;
  private handler: WorldStateHandler;

  constructor(handler: WorldStateHandler) {
    this.handler = handler;
  }

  setupWorldStateChannel(pc: RTCPeerConnection) {
    this.worldStateChannel = pc.createDataChannel('worldstate', {
      ordered: true,
      // Use unreliable for lower latency if needed:
      // maxRetransmits: 0,
    });

    this.worldStateChannel.binaryType = 'arraybuffer';

    this.worldStateChannel.onmessage = (event: MessageEvent) => {
      const data = new Uint8Array(event.data);
      const state = WorldState.decode(data);
      this.handler.onWorldState(state);
    };

    this.worldStateChannel.onclose = () => {
      console.log('WorldState channel closed');
    };
  }
}

// Usage
const client = new RS1Client({
  onWorldState(state: WorldState) {
    console.log(`Frame ${state.frameNumber}: ${state.objects.length} objects`);

    for (const obj of state.objects) {
      console.log(`  Track ${obj.trackId}: (${obj.x.toFixed(2)}, ${obj.y.toFixed(2)}) ` +
                  `class=${obj.classId} conf=${obj.confidence.toFixed(2)}`);
    }
  }
});
```

### Swift (iOS)

```swift
import SwiftProtobuf
import WebRTC

class RS1WorldStateHandler: NSObject, RTCDataChannelDelegate {
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
            print("Failed to decode WorldState: \(error)")
        }
    }

    func dataChannelDidChangeState(_ dataChannel: RTCDataChannel) {
        print("WorldState channel state: \(dataChannel.readyState.rawValue)")
    }
}

// Usage
let handler = RS1WorldStateHandler()
handler.onWorldState = { state in
    print("Received \(state.objects.count) tracked objects")

    for obj in state.objects {
        print("Track \(obj.trackID): (\(obj.x), \(obj.y)) class=\(obj.classID)")
    }
}
```

---

## Room Coordinate System

The WorldState uses a right-handed coordinate system anchored to the room:

```
         +Y (forward/north)
          │
          │
          │
    ──────┼────── +X (right/east)
          │
          │
       Origin
    (room corner)
```

### Coordinate Details

| Field | Unit | Range | Description |
|-------|------|-------|-------------|
| `x` | meters | 0 to room_width | Distance from left wall |
| `y` | meters | 0 to room_height | Distance from back wall |
| `vx` | m/s | -10 to +10 | Velocity towards right (+X) |
| `vy` | m/s | -10 to +10 | Velocity forward (+Y) |
| `heading` | degrees | -180 to +180 | Direction of movement (0=+Y) |
| `speed` | m/s | 0 to 15 | Velocity magnitude |

---

## Class IDs

| ID | Class | Description |
|----|-------|-------------|
| -1 | Unknown | Radar-only detection, no classification |
| 0 | Person | Human detected |
| 1 | Vehicle | Car, bicycle, etc. |
| 2 | Animal | Pet, wildlife |

---

## Channel Configuration

### Reliability Options

| Mode | Config | Use Case |
|------|--------|----------|
| Reliable | `ordered: true` | Default, ensures delivery |
| Unreliable-Ordered | `maxRetransmits: 0, ordered: true` | Lower latency, tolerate drops |
| Unreliable-Unordered | `maxRetransmits: 0, ordered: false` | Lowest latency |

For most applications, reliable-ordered is recommended. Clients can interpolate between frames if drops occur.

### Bandwidth

- Typical message size: 50-200 bytes (1-5 objects)
- At 30Hz: 1.5-6 KB/s
- Negligible compared to video stream

---

## Synchronization

### With Video

The WorldState `timestamp_ns` can be compared to video frame PTS for synchronization:

```typescript
class SynchronizedRenderer {
  private videoFrameTime: number = 0;
  private worldState: WorldState | null = null;

  onVideoFrame(frame: VideoFrame) {
    this.videoFrameTime = frame.timestamp;
    this.render();
  }

  onWorldState(state: WorldState) {
    this.worldState = state;
    this.render();
  }

  render() {
    if (!this.worldState) return;

    // WorldState is typically 1-2 frames ahead due to processing
    // Apply to overlay on video
    this.drawOverlay(this.worldState);
  }
}
```

---

## Diagnostics

### Prometheus Metrics

```go
var (
    worldstateFramesSent = promauto.NewCounter(prometheus.CounterOpts{
        Name: "rs1_worldstate_frames_sent_total",
        Help: "Total WorldState frames sent",
    })
    worldstateLatency = promauto.NewHistogram(prometheus.HistogramOpts{
        Name:    "rs1_worldstate_latency_seconds",
        Help:    "WorldState serialization and send latency",
        Buckets: prometheus.ExponentialBuckets(0.001, 2, 10),
    })
)
```

### JSON-RPC Query

```json
// Request
{"jsonrpc": "2.0", "method": "getWorldState", "id": 1}

// Response
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "objects": [
      {"trackId": 1, "x": 2.5, "y": 3.2, "classId": 0, "confidence": 0.92}
    ],
    "frameNumber": 12345
  }
}
```

---

## See Also

- [RS1_ARCHITECTURE.md](RS1_ARCHITECTURE.md) - System overview
- [FUSION_ENGINE.md](FUSION_ENGINE.md) - How WorldState is generated
- [ROOMPLAN_API.md](ROOMPLAN_API.md) - Room coordinate setup
