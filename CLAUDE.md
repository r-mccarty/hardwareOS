# CLAUDE.md

This file provides guidance to Claude Code when working with this repository.

## Project Overview

**HardwareOS** is an embedded platform for building connected sensor devices. The **OpticWorks RS-1** is the flagship product—a vision/radar sensor fusion device for real-time occupancy tracking.

### Current Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                        Browser / iPhone App                          │
└────────────────────────────────┬────────────────────────────────────┘
                                 │ WebRTC DataChannel (30Hz JSON)
┌────────────────────────────────┴────────────────────────────────────┐
│                         Go Application                               │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌────────────┐  │
│  │ WebRTC      │  │ Fusion      │  │ WorldState  │  │ RoomPlan   │  │
│  │ Streaming   │  │ Engine      │  │ Manager     │  │ API        │  │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘  └────────────┘  │
│         │                │                │                          │
│         │         ┌──────┴──────┐         │                          │
│         │         │   Kalman    │◄────────┘                          │
│         │         │   Filters   │                                    │
│         │         └──────┬──────┘                                    │
│         │                │                                           │
│         │         ┌──────┴──────┐                                    │
│         │         │  Hungarian  │                                    │
│         │         │  Algorithm  │                                    │
│         │         └──────┬──────┘                                    │
│  ┌──────┴────────────────┴──────┐  ┌─────────────────────────────┐  │
│  │      gRPC Native Proxy       │  │    Radar UART Parser        │  │
│  └──────────────┬───────────────┘  └──────────────┬──────────────┘  │
└─────────────────┼──────────────────────────────────┼────────────────┘
                  │ gRPC                             │ Serial (256kbps)
┌─────────────────┴──────────────┐    ┌──────────────┴────────────────┐
│         Native Process (C)      │    │         LD2450 Radar          │
│  Camera → ISP → NPU → Encoder   │    │    24GHz mmWave Sensor        │
└─────────────────────────────────┘    └──────────────────────────────┘
```

---

## Package Structure

```
hardwareos/
├── main.go                         # Application entry + RS-1 init
├── config.go                       # Configuration (includes RoomConfig)
├── webrtc.go                       # WebRTC sessions + worldstate channel
├── webrtc_worldstate.go            # WorldState streaming (30Hz)
├── roomplan.go                     # RoomPlan API handlers
├── web.go                          # HTTP routes
│
├── products/rs1/                   # RS-1 PRODUCT
│   ├── init.go                     # Product initialization
│   ├── config.go                   # RS-1 specific config
│   ├── worldstate.go               # Occupancy state manager
│   ├── fusion.go                   # Fusion engine wrapper
│   ├── radar.go                    # Radar manager wrapper
│   │
│   ├── fusion/                     # SENSOR FUSION ALGORITHMS
│   │   ├── engine.go               # Fusion coordinator
│   │   ├── kalman.go               # Kalman filter [px,py,vx,vy]
│   │   ├── hungarian.go            # Hungarian data association
│   │   ├── transform.go            # Coordinate transforms
│   │   └── *_test.go               # Unit tests
│   │
│   └── radar/                      # LD2450 RADAR DRIVER
│       ├── ld2450.go               # UART parser
│       ├── types.go                # Target types
│       └── state.go                # Thread-safe state latch
│
├── targets/rv1106/                 # RV1106 HARDWARE TARGET
│   └── native/                     # Native C + gRPC
│       ├── proto/native.proto      # Protobuf definitions
│       └── cgo/                    # CGO bridge
│
├── platform/                       # Platform abstractions
│   ├── config/                     # Brand configuration
│   ├── logging/                    # Zerolog wrapper
│   ├── network/                    # Network management
│   └── ota/                        # OTA updates
│
├── internal/                       # Internal packages
└── ui/                             # React frontend
```

---

## Key Components

### Sensor Fusion (`products/rs1/fusion/`)

| File | Purpose |
|------|---------|
| `engine.go` | Coordinates radar + vision, manages tracks |
| `kalman.go` | 4-state Kalman filter [px, py, vx, vy] |
| `hungarian.go` | Optimal detection-to-track association |
| `transform.go` | Sensor → room coordinate transforms |

**Kalman Filter Parameters:**
- State: `[x, y, vx, vy]`
- Process noise Q: `diag(0.1, 0.1, 1.0, 1.0)`
- Measurement noise R: `diag(0.5, 0.5)`
- Max missed frames: 30 (3 seconds at 10Hz)

**Hungarian Algorithm:**
- Max association angle: 15 degrees
- Uses azimuth matching between radar and vision

### WorldState Streaming (`webrtc_worldstate.go`)

- 30Hz JSON streaming over WebRTC DataChannel
- Channel label: `"worldstate"`
- Connects via `SetRS1WorldStateGetter()` accessor

**Message Format:**
```json
{
  "timestamp_ns": 1703251200000000000,
  "frame_number": 12345,
  "occupant_count": 2,
  "objects": [{
    "track_id": "track_001",
    "x": 3.5, "y": 2.1,
    "vx": 0.5, "vy": -0.2,
    "confidence": 0.95,
    "heading": 68.2, "speed": 0.54,
    "is_radar_only": false, "is_vision_only": false
  }]
}
```

### RoomPlan API (`roomplan.go`)

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/api/setup/roomplan` | POST | Upload room configuration |
| `/api/setup/roomplan` | GET | Get current configuration |

**Transform Matrix Format:** Column-major 4x4 homogeneous matrix. Last row must be `[0, 0, 0, 1]`.

### Radar Driver (`products/rs1/radar/`)

- **Protocol:** LD2450 binary UART @ 256000 baud
- **Capabilities:** 3 targets, 6m range, azimuth + distance
- **Update rate:** 10Hz

---

## Build Commands

```bash
# Development deploy
./dev_deploy.sh -r <DEVICE_IP>              # Full build + deploy
./dev_deploy.sh -r <DEVICE_IP> --skip-ui-build  # Backend only

# Testing
go test ./products/rs1/...                  # RS-1 unit tests
go test ./products/rs1/fusion/... -v        # Fusion tests verbose
./dev_deploy.sh -r <DEVICE_IP> --run-go-tests   # On-device tests

# Linting
go vet ./products/rs1/...                   # Vet RS-1 packages
```

---

## Implementation Status

### Completed

| Component | Location | Status |
|-----------|----------|--------|
| Kalman Filter | `fusion/kalman.go` | Done |
| Hungarian Algorithm | `fusion/hungarian.go` | Done |
| Transform Matrix | `fusion/transform.go` | Done |
| Fusion Engine | `fusion/engine.go` | Done |
| WorldState Manager | `worldstate.go` | Done |
| WebRTC Streaming | `webrtc_worldstate.go` | Done |
| RoomPlan API | `roomplan.go` | Done |
| LD2450 Radar | `radar/` | Done |
| Product Integration | `main.go`, `init.go` | Done |

### Pending

| Component | Location | Notes |
|-----------|----------|-------|
| Vision Pipeline | `targets/rv1106/native/` | Requires C implementation |
| NPU Integration | `native/cgo/npu.c` | YOLOv8 inference |
| Camera Driver | `native/cgo/camera.c` | SC3336 MIPI |
| ISP/LDCH | `native/cgo/isp.c` | Lens distortion correction |

---

## Configuration

Configuration at `/userdata/kvm_config.json`:

```json
{
  "cloud_url": "https://api.optic.works",
  "auto_update_enabled": true,
  "room_config": {
    "width": 10.0,
    "height": 8.0,
    "sensor_pose": [1,0,0,0, 0,1,0,0, 0,0,1,0, 2.5,4.0,2.8,1],
    "room_polygon": [],
    "obstacles": []
  }
}
```

---

## Development Notes

### Logging
Use `platform/logging` package. Enable trace:
```bash
export LOG_TRACE_SCOPES="rs1,fusion,radar,worldstate"
```

### Cross-Compilation
ARM target: `GOARCH=arm GOARM=7`. Docker build uses `ghcr.io/jetkvm/buildkit:latest`.

### Testing Fusion Algorithms
```bash
go test ./products/rs1/fusion/... -v -run TestKalman
go test ./products/rs1/fusion/... -v -run TestAssociate
go test ./products/rs1/fusion/... -v -run TestTransform
```

### Adding New Tracks
Tracks are auto-created in `engine.go` when:
1. Unmatched radar detection persists
2. Unmatched vision detection persists

Tracks are deleted when:
- `KalmanFilter.ShouldDelete()` returns true (30 missed frames)

---

## Documentation

### RS-1 Product
| Document | Description |
|----------|-------------|
| [RS1_ARCHITECTURE.md](docs/rs1/RS1_ARCHITECTURE.md) | System overview |
| [FUSION_ENGINE.md](docs/rs1/FUSION_ENGINE.md) | Algorithm details |
| [RADAR_INTEGRATION.md](docs/rs1/RADAR_INTEGRATION.md) | LD2450 protocol |
| [VISION_PIPELINE.md](docs/rs1/VISION_PIPELINE.md) | Camera/NPU pipeline |
| [WORLDSTATE_PROTOCOL.md](docs/rs1/WORLDSTATE_PROTOCOL.md) | Streaming format |
| [ROOMPLAN_API.md](docs/rs1/ROOMPLAN_API.md) | REST endpoints |

### Platform
| Document | Description |
|----------|-------------|
| [DEVELOPMENT.md](docs/DEVELOPMENT.md) | Dev setup |
| [ARCHITECTURE.md](docs/ARCHITECTURE.md) | Platform architecture |
| [OTA_FLOW.md](docs/OTA_FLOW.md) | Update system |

---

## Common Tasks

### Adding a new sensor input
1. Create package in `products/rs1/<sensor>/`
2. Implement detection callback to fusion engine
3. Register in `init.go`

### Modifying fusion parameters
1. Edit constants in `fusion/kalman.go` or `fusion/hungarian.go`
2. Run tests: `go test ./products/rs1/fusion/...`

### Adding new WorldState fields
1. Update `worldStateMessage` in `webrtc_worldstate.go`
2. Update `trackedObjectJSON` struct
3. Update protobuf if needed: `targets/rv1106/native/proto/native.proto`

### Debugging fusion
Enable trace logging:
```go
logger.Trace().
    Str("track_id", track.ID).
    Float64("x", x).
    Float64("y", y).
    Msg("track updated")
```
