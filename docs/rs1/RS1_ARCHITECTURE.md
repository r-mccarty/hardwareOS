# OpticWorks RS-1 System Architecture

The RS-1 is the first product built on **HardwareOS**, running on the Rockchip RV1106G hardware target. It provides vision/radar sensor fusion for real-time spatial tracking and occupancy monitoring.

## HardwareOS Platform

HardwareOS is an embedded automation platform that provides:
- WebRTC streaming (video + data channels)
- JSON-RPC command/event protocol
- OTA updates for apps and models
- Network management (DHCP, WiFi, static IP)
- Automation features (Wake-on-LAN, mDNS)
- Process supervision and crash recovery

The RS-1 extends HardwareOS with sensor fusion capabilities specific to spatial tracking.

## Hardware Platform

### Rockchip RV1106G SoC
- **CPU**: ARM Cortex-A7 @ 1.2GHz
- **NPU**: 0.5 TOPS RKNN for neural network inference
- **ISP**: Image Signal Processor with LDCH (Lens Distortion Correction)
- **Video Encoder**: H.265/H.264 hardware encoding
- **Memory**: 256MB DDR3

### Sensors
| Sensor | Interface | Purpose |
|--------|-----------|---------|
| SC3336 | MIPI CSI-2 | 3MP camera (2304x1296 @ 30fps) |
| LD2450 | UART 256000 baud | 24GHz mmWave radar (3 targets) |

### Connectivity
- Ethernet (100Mbps)
- WiFi (optional module)
- LCD touchscreen (status display)

---

## Software Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                        Browser / iPhone App                          │
│                    (React UI / RoomPlan Setup)                       │
└────────────────────────────────┬────────────────────────────────────┘
                                 │ WebRTC + HTTPS
┌────────────────────────────────┴────────────────────────────────────┐
│                         Go Application                               │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌────────────┐  │
│  │ WebRTC      │  │ Fusion      │  │ JSON-RPC    │  │ Config     │  │
│  │ Streaming   │  │ Engine      │  │ API         │  │ Manager    │  │
│  └──────┬──────┘  └──────┬──────┘  └─────────────┘  └────────────┘  │
│         │                │                                           │
│  ┌──────┴──────┐  ┌──────┴──────┐  ┌─────────────────────────────┐  │
│  │ Video Track │  │ WorldState  │  │ RoomPlan API                │  │
│  │ (H.265)     │  │ DataChannel │  │ (POST /api/setup/roomplan)  │  │
│  └──────┬──────┘  └──────┬──────┘  └─────────────────────────────┘  │
│         │                │                                           │
│  ┌──────┴────────────────┴──────┐  ┌─────────────────────────────┐  │
│  │      gRPC Native Proxy       │  │    Radar UART Parser        │  │
│  │  (internal/native)           │  │  (internal/radar)           │  │
│  └──────────────┬───────────────┘  └──────────────┬──────────────┘  │
└─────────────────┼──────────────────────────────────┼────────────────┘
                  │ gRPC                             │ Serial
┌─────────────────┴──────────────────────────────────┼────────────────┐
│                 Native Process (C)                  │                │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐ │                │
│  │ Camera/ISP  │  │ NPU         │  │ Video       │ │                │
│  │ Pipeline    │─▶│ Inference   │  │ Encoder     │ │                │
│  └─────────────┘  └──────┬──────┘  └──────┬──────┘ │                │
│                          │                │        │                │
│                   VisionFrame       H.265 Stream   │                │
└──────────────────────────┴────────────────┴────────┴────────────────┘
                                                            │
                                                     ┌──────┴──────┐
                                                     │   LD2450    │
                                                     │   Radar     │
                                                     └─────────────┘
```

---

## Data Flow

### 1. Vision Pipeline (30 FPS)
```
SC3336 MIPI → V4L2 → ISP (LDCH) → [Fork]
                                    ├─→ RGA Resize (1080p) → H.265 Encoder → WebRTC Video Track
                                    └─→ RGA Resize (640x640) → NPU (YOLOv8) → VisionFrame
```

### 2. Radar Pipeline (10 Hz)
```
LD2450 → UART → Frame Parser → RadarFrame → State Latch (sync.RWMutex)
```

### 3. Fusion Pipeline (30 Hz)
```
VisionFrame ─┬─→ Hungarian Association ─→ Kalman Update ─→ WorldState
RadarFrame ──┘
```

### 4. Output Pipeline (30 Hz)
```
WorldState → Protobuf Serialize → WebRTC DataChannel → Browser
```

---

## Package Structure

```
/workspaces/hardwareos/
├── cmd/main.go                    # Supervisor process
├── main.go                        # Application entry point
├── config.go                      # RS-1 configuration
├── webrtc.go                      # WebRTC session management
├── worldstate.go                  # WorldState streaming
├── roomplan.go                    # RoomPlan API handlers
├── internal/
│   ├── native/
│   │   ├── cgo/
│   │   │   ├── camera.c           # SC3336 initialization
│   │   │   ├── isp.c              # ISP/LDCH configuration
│   │   │   ├── npu.c              # RKNN inference
│   │   │   └── video.c            # Pipeline orchestration
│   │   └── proto/
│   │       └── native.proto       # VisionFrame, VisionDetection
│   ├── radar/
│   │   ├── ld2450.go              # UART protocol parser
│   │   ├── types.go               # RadarTarget, RadarFrame
│   │   └── state.go               # Async state latch
│   └── fusion/
│       ├── engine.go              # FusionEngine main loop
│       ├── hungarian.go           # Data association
│       ├── kalman.go              # State estimation
│       ├── worldstate.go          # TrackedObject types
│       └── transform.go           # Coordinate transforms
└── docs/rs1/                      # This documentation
```

---

## Configuration

Configuration is stored at `/userdata/kvm_config.json` (path retained for compatibility).

### RS-1 Config Schema
```json
{
  "cloud_url": "https://api.optic.works",
  "auto_update_enabled": true,
  "room_config": {
    "width": 10.0,
    "height": 8.0,
    "sensor_pose": {
      "m": [1,0,0,0, 0,1,0,0, 0,0,1,0, 2.5,4.0,2.8,1]
    }
  },
  "camera_config": {
    "resolution": "1080p",
    "fps": 30,
    "exposure_mode": "auto"
  },
  "radar_config": {
    "uart_path": "/dev/ttyS0",
    "baud_rate": 256000,
    "max_range_m": 6.0
  },
  "model_path": "/userdata/models/yolov8n.rknn",
  "detection_zones": []
}
```

---

## API Endpoints

### WebRTC Signaling
- `POST /webrtc/session` - Create WebRTC session
- `POST /webrtc/session/{id}/ice` - Add ICE candidate

### Configuration
- `GET /api/device` - Device info
- `GET /api/setup/roomplan` - Get room configuration
- `POST /api/setup/roomplan` - Upload RoomPlan data
- `POST /api/detection-zones` - Configure detection zones

### Real-time Data
- WebRTC Video Track - H.265 camera stream
- WebRTC `worldstate` DataChannel - 30Hz TrackedObject stream
- WebRTC `rpc` DataChannel - JSON-RPC commands

---

## Inherited from JetKVM

The RS-1 retains these JetKVM components:

| Component | Purpose | Location |
|-----------|---------|----------|
| Supervisor | Crash recovery, process restart | `cmd/main.go` |
| OTA Updates | App and model updates | `internal/ota/` |
| WebRTC | Video and data streaming | `webrtc.go` |
| JSON-RPC | Command/event protocol | `jsonrpc.go` |
| mDNS | Device discovery | `internal/mdns/` |
| Network | DHCP, static IP, WiFi | `internal/network/` |
| Prometheus | Metrics export | `prometheus.go` |
| LVGL UI | LCD touchscreen display | `internal/native/cgo/screen.c` |

---

## Removed from JetKVM

| Component | Reason |
|-----------|--------|
| USB Gadget | No keyboard/mouse emulation needed |
| HID RPC | No HID device input |
| Virtual Media | No ISO mounting |
| EDID/HDMI | Using MIPI camera, not HDMI capture |
| ATX/DC Power | No power control extensions |
| Jiggler | No mouse movement simulation |
| Wake-on-LAN | Not applicable |

---

## See Also

- [VISION_PIPELINE.md](VISION_PIPELINE.md) - Camera/ISP/NPU details
- [RADAR_INTEGRATION.md](RADAR_INTEGRATION.md) - LD2450 protocol
- [FUSION_ENGINE.md](FUSION_ENGINE.md) - Sensor fusion algorithms
- [WORLDSTATE_PROTOCOL.md](WORLDSTATE_PROTOCOL.md) - Protobuf schema
- [ROOMPLAN_API.md](ROOMPLAN_API.md) - iPhone integration
