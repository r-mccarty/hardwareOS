# HardwareOS Architecture

This document describes the system architecture of HardwareOS, an embedded platform for connected sensor devices. The OpticWorks RS-1 is the flagship product built on this platform.

## System Overview

HardwareOS follows a layered architecture separating platform services from product-specific logic:

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           Client Applications                            │
│              (Browser, iPhone App, Home Automation)                      │
└────────────────────────────────┬────────────────────────────────────────┘
                                 │ WebRTC / HTTPS / WebSocket
┌────────────────────────────────┴────────────────────────────────────────┐
│                            Go Application                                │
│  ┌──────────────────────────────────────────────────────────────────┐   │
│  │                         PLATFORM LAYER                            │   │
│  │  WebRTC │ JSON-RPC │ OTA │ Network │ mDNS │ Auth │ Config        │   │
│  └──────────────────────────────────────────────────────────────────┘   │
│  ┌──────────────────────────────────────────────────────────────────┐   │
│  │                         PRODUCT LAYER                             │   │
│  │                         (RS-1 Product)                            │   │
│  │  Fusion Engine │ WorldState │ Radar Driver │ Vision Pipeline      │   │
│  └──────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────┬───────────────────────────────────────┘
                                  │ gRPC + Serial
┌─────────────────────────────────┴───────────────────────────────────────┐
│                          Hardware Layer                                  │
│              Native Process (C) │ LD2450 Radar │ Sensors                │
└─────────────────────────────────────────────────────────────────────────┘
```

## Process Model

```
┌─────────────────────────────────────────────────────────────────┐
│                          Linux OS                                │
│  ┌─────────────┐                                                │
│  │ Supervisor  │──spawns──► Go Application                      │
│  │ cmd/main.go │           main.go + products/rs1/              │
│  └─────────────┘                    │                           │
│                                     │ spawns                    │
│                           ┌─────────▼─────────┐                 │
│                           │  Native Process   │                 │
│                           │ targets/rv1106/   │                 │
│                           └───────────────────┘                 │
└─────────────────────────────────────────────────────────────────┘
```

- **Supervisor** (`cmd/main.go`): Validates executable, spawns app process, captures crash logs
- **Go Application** (`main.go`): Orchestrates all services, initializes products
- **Native Process**: Handles hardware-intensive work (camera, ISP, NPU, display)

## Startup Sequence

1. **Supervisor Start**: Validate executable, set up crash handling
2. **App Init**: Load configuration from `/userdata/kvm_config.json`
3. **Watchdog**: Start hardware watchdog at `/dev/watchdog`
4. **Native Proxy**: Spawn native process, establish gRPC channel
5. **Display**: Initialize LVGL touchscreen UI
6. **Network**: Configure eth0, DHCP, routing
7. **Time Sync**: NTP synchronization
8. **mDNS**: Register device for local discovery
9. **RS-1 Product**: Initialize fusion engine, radar, worldstate
10. **Web Servers**: HTTP, HTTPS, WebSocket, WebRTC
11. **Cloud**: Connect to OpticWorks Cloud (optional)

## Platform Layer

### WebRTC Streaming (`webrtc.go`)

Handles peer connections and data channels:

| Channel | Purpose |
|---------|---------|
| `video` | H.265 video track from native |
| `rpc` | JSON-RPC commands |
| `worldstate` | RS-1 occupancy streaming |
| `terminal` | Debug terminal |

### Configuration (`config.go`)

JSON configuration at `/userdata/kvm_config.json`:

```go
type Config struct {
    CloudURL           string       `json:"cloud_url"`
    AutoUpdateEnabled  bool         `json:"auto_update_enabled"`
    RoomConfig         *RoomConfig  `json:"room_config,omitempty"`
    // ... other fields
}
```

### Network (`network.go`, `platform/network/`)

- DHCP client for automatic configuration
- Static IP support
- WiFi management (if hardware supports)
- Network state change notifications

### OTA Updates (`platform/ota/`)

- Checks update API for new versions
- Downloads and applies system + app updates
- Model updates for NPU (RS-1)

## Product Layer (RS-1)

### Package Structure

```
products/rs1/
├── init.go           # Product initialization
├── config.go         # RS-1 specific configuration
├── worldstate.go     # Occupancy state manager
├── fusion.go         # Fusion engine wrapper
├── radar.go          # Radar manager wrapper
│
├── fusion/           # Sensor fusion algorithms
│   ├── engine.go     # Fusion coordinator
│   ├── kalman.go     # Kalman filter
│   ├── hungarian.go  # Data association
│   └── transform.go  # Coordinate transforms
│
└── radar/            # LD2450 driver
    ├── ld2450.go     # UART protocol
    ├── types.go      # Data types
    └── state.go      # Thread-safe state
```

### Data Flow

```
LD2450 Radar ──10Hz──► RadarManager ──────────────────────┐
                                                          │
                                                          ▼
                                                   ┌──────────────┐
SC3336 Camera ──► ISP ──► NPU ──► VisionFrame ───►│ Fusion Engine│
                                                   │   Kalman +   │
                                                   │  Hungarian   │
                                                   └──────┬───────┘
                                                          │
                                                          ▼
                                                   ┌──────────────┐
                                                   │  WorldState  │
                                                   │   Manager    │
                                                   └──────┬───────┘
                                                          │ 30Hz
                                                          ▼
                                                   ┌──────────────┐
                                                   │   WebRTC     │
                                                   │ DataChannel  │
                                                   └──────────────┘
```

### Fusion Engine

The fusion engine combines radar and vision detections:

1. **Input**: Radar targets (range, azimuth) + Vision detections (bounding boxes)
2. **Association**: Hungarian algorithm matches detections to existing tracks
3. **State Estimation**: Kalman filter predicts and updates track state
4. **Output**: Fused object list with position, velocity, confidence

### WorldState

Maintains current room occupancy:

- Updated at 10Hz from fusion engine
- Snapshots streamed at 30Hz via WebRTC
- Zone occupancy tracking (future)

## Native Subsystem

### Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    Native Process (C)                    │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────┐  │
│  │   Camera    │  │     ISP     │  │      NPU        │  │
│  │  SC3336     │──│    LDCH     │──│    YOLOv8       │  │
│  │   MIPI      │  │  Correction │  │   Inference     │  │
│  └─────────────┘  └─────────────┘  └─────────────────┘  │
│                                            │            │
│                                            ▼            │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────┐  │
│  │    LVGL     │  │   Encoder   │  │  VisionFrame    │  │
│  │     UI      │  │    H.265    │  │   (protobuf)    │  │
│  └─────────────┘  └─────────────┘  └─────────────────┘  │
└────────────────────────┬────────────────────────────────┘
                         │ gRPC + Unix Sockets
                         ▼
                    Go Application
```

### Communication

- **gRPC**: Control commands (camera settings, display, NPU config)
- **Unix Socket**: Video frame streaming
- **Protobuf**: Structured data (VisionFrame, VideoFrame)

## API Layer

### REST Endpoints

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/api/setup/roomplan` | POST | Upload room configuration |
| `/api/setup/roomplan` | GET | Get room configuration |
| `/api/device` | GET | Device information |
| `/api/auth/*` | * | Authentication |

### WebRTC Signaling

- Local: HTTP-based SDP exchange
- Cloud: WebSocket relay through OpticWorks Cloud

### JSON-RPC

Commands over WebRTC `rpc` channel:

```json
{"jsonrpc": "2.0", "method": "getDeviceInfo", "id": 1}
{"jsonrpc": "2.0", "method": "setVideoQuality", "params": {"factor": 0.8}, "id": 2}
```

## Observability

### Metrics

Prometheus metrics at `/metrics`:

- `hardwareos_config_last_reload_successful`
- `hardwareos_fusion_tracks_total`
- `hardwareos_radar_targets_total`
- `hardwareos_webrtc_sessions_active`

### Logging

Structured logging via zerolog:

```bash
export LOG_TRACE_SCOPES="rs1,fusion,radar,worldstate"
```

### Crash Handling

- Crash dumps: `/userdata/jetkvm/crashdump/`
- Supervisor restarts on crash
- Failsafe mode after repeated failures

## Configuration Files

| Path | Purpose |
|------|---------|
| `/userdata/kvm_config.json` | Main configuration |
| `/userdata/models/*.rknn` | NPU models |
| `/etc/network/interfaces` | Network config |

## Key Design Decisions

1. **Go + C Split**: Go for orchestration, C for hardware-intensive work
2. **Product Abstraction**: Platform services separate from product logic
3. **WebRTC First**: Low-latency streaming over standard protocols
4. **Graceful Degradation**: Failsafe mode if hardware fails
5. **Local + Cloud**: Works offline, enhanced with cloud connectivity
