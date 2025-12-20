# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**OpticWorks RS-1** is a vision/radar sensor fusion platform for spatial tracking, built on the Rockchip RV1106G. This codebase is pivoting from the original JetKVM (KVM-over-IP) functionality to become the RS-1 base image.

### The Pivot

The RS-1 leverages JetKVM's solved problems:
- **CGO Bridge**: Go-to-C communication for hardware access
- **WebRTC Streaming**: Low-latency video and data channels
- **Cross-Compilation**: ARM toolchain for RV1106G
- **Supervisor/OTA**: Process management and remote updates

We are replacing KVM functionality with:
- **SC3336 MIPI Camera** with ISP lens distortion correction
- **RKNN NPU** for YOLOv8 object detection
- **LD2450 Radar** for range/velocity measurement
- **Sensor Fusion Engine** with Kalman filtering
- **WorldState Streaming** via WebRTC DataChannel
- **RoomPlan API** for iPhone integration

### Architecture Documentation

See `docs/rs1/` for detailed RS-1 architecture:

| Document | Description |
|----------|-------------|
| [RS1_ARCHITECTURE.md](docs/rs1/RS1_ARCHITECTURE.md) | System overview, package structure, data flow |
| [VISION_PIPELINE.md](docs/rs1/VISION_PIPELINE.md) | Camera/ISP/NPU implementation |
| [RADAR_INTEGRATION.md](docs/rs1/RADAR_INTEGRATION.md) | LD2450 protocol and UART parsing |
| [FUSION_ENGINE.md](docs/rs1/FUSION_ENGINE.md) | Kalman filter, Hungarian algorithm |
| [WORLDSTATE_PROTOCOL.md](docs/rs1/WORLDSTATE_PROTOCOL.md) | Protobuf schema, WebRTC streaming |
| [ROOMPLAN_API.md](docs/rs1/ROOMPLAN_API.md) | iPhone RoomPlan integration |

### Claude Skills

Development guidance is available in `.claude/skills/`:

| Skill | Use When |
|-------|----------|
| `rs1-vision-pipeline.md` | Working on camera, ISP, or NPU code |
| `rs1-radar-protocol.md` | Implementing LD2450 UART parsing |
| `rs1-sensor-fusion.md` | Kalman filter or data association |
| `rs1-worldstate.md` | WebRTC DataChannel streaming |

---

## RS-1 Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                        Browser / iPhone App                          │
└────────────────────────────────┬────────────────────────────────────┘
                                 │ WebRTC + HTTPS
┌────────────────────────────────┴────────────────────────────────────┐
│                         Go Application                               │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌────────────┐  │
│  │ WebRTC      │  │ Fusion      │  │ JSON-RPC    │  │ RoomPlan   │  │
│  │ Streaming   │  │ Engine      │  │ API         │  │ API        │  │
│  └──────┬──────┘  └──────┬──────┘  └─────────────┘  └────────────┘  │
│         │                │                                           │
│  ┌──────┴──────┐  ┌──────┴──────┐                                   │
│  │ Video Track │  │ WorldState  │                                   │
│  │ (H.265)     │  │ DataChannel │                                   │
│  └──────┬──────┘  └──────┬──────┘                                   │
│         │                │                                           │
│  ┌──────┴────────────────┴──────┐  ┌─────────────────────────────┐  │
│  │      gRPC Native Proxy       │  │    Radar UART Parser        │  │
│  └──────────────┬───────────────┘  └──────────────┬──────────────┘  │
└─────────────────┼──────────────────────────────────┼────────────────┘
                  │ gRPC                             │ Serial
┌─────────────────┴──────────────┐    ┌──────────────┴────────────────┐
│         Native Process (C)      │    │         LD2450 Radar          │
│  Camera → ISP → NPU → Encoder   │    │    24GHz mmWave Sensor        │
└─────────────────────────────────┘    └──────────────────────────────┘
```

### Package Structure (Target)

```
/workspaces/hardwareos/
├── cmd/main.go                    # Supervisor process
├── main.go                        # Application entry point
├── config.go                      # RS-1 configuration
├── webrtc.go                      # WebRTC session management
├── worldstate.go                  # WorldState streaming (NEW)
├── roomplan.go                    # RoomPlan API (NEW)
├── internal/
│   ├── native/
│   │   ├── cgo/
│   │   │   ├── camera.c           # SC3336 initialization (NEW)
│   │   │   ├── isp.c              # ISP/LDCH config (NEW)
│   │   │   ├── npu.c              # RKNN inference (NEW)
│   │   │   └── video.c            # Pipeline (MODIFIED)
│   │   └── proto/native.proto     # VisionFrame messages (MODIFIED)
│   ├── radar/                     # LD2450 package (NEW)
│   │   ├── ld2450.go
│   │   ├── types.go
│   │   └── state.go
│   └── fusion/                    # Sensor fusion (NEW)
│       ├── engine.go
│       ├── hungarian.go
│       ├── kalman.go
│       └── worldstate.go
└── docs/rs1/                      # RS-1 documentation
```

---

## Build Commands

### Full Development Deploy (to device)
```bash
./dev_deploy.sh -r <DEVICE_IP>              # Build and deploy everything
./dev_deploy.sh -r <DEVICE_IP> --skip-ui-build  # Backend only (faster)
```

### Backend Development
```bash
make test                     # Run Go unit tests locally
make lint                     # Run go vet
go test ./...                 # Standard Go testing
```

### Run Tests on Device
```bash
./dev_deploy.sh -r <DEVICE_IP> --run-go-tests       # Build, deploy, and run tests
./dev_deploy.sh -r <DEVICE_IP> --run-go-tests-only  # Run tests only
```

### Frontend Development
```bash
cd ui
npm install
./dev_device.sh <DEVICE_IP>   # Live development with hot reload
npm run lint                  # Run ESLint
npm run build:device          # Production build for device
```

---

## Implementation Status

### Phase 1: Codebase Cleanup (Remove KVM Features)
**Status**: Planned

Files to DELETE:
- `internal/usbgadget/` - USB HID emulation
- `internal/hidrpc/` - HID protocol
- `jiggler.go`, `wol.go`, `hidrpc.go`
- `usb_mass_storage.go`, `block_device*.go`

Files to MODIFY:
- `config.go` - Remove USB/keyboard configs
- `jsonrpc.go` - Remove ~40 KVM-specific handlers
- `serial.go` - Replace ATX/DC with radar
- `webrtc.go` - Remove HID channels

### Phase 2: Native Vision Pipeline
**Status**: Planned

New files:
- `internal/native/cgo/camera.c` - SC3336 MIPI init
- `internal/native/cgo/isp.c` - ISP with LDCH
- `internal/native/cgo/npu.c` - RKNN YOLOv8

### Phase 3: Radar Integration
**Status**: Planned

New package: `internal/radar/`
- LD2450 UART protocol parser
- Thread-safe state latch

### Phase 4: Sensor Fusion
**Status**: Planned

New package: `internal/fusion/`
- Hungarian algorithm for association
- Kalman filter for state estimation

### Phase 5: WorldState Streaming
**Status**: Planned

New WebRTC DataChannel: `worldstate`
- 30Hz Protobuf streaming
- TrackedObject messages

### Phase 6: RoomPlan API
**Status**: Planned

New endpoints:
- `POST /api/setup/roomplan`
- `GET /api/setup/roomplan`

---

## Key Development Notes

### Cross-Compilation
The backend cross-compiles for ARM (GOARCH=arm, GOARM=7). Without the native buildkit (`/opt/jetkvm-native-buildkit`), builds automatically run in Docker using `ghcr.io/jetkvm/buildkit:latest`.

### Logging
Use the `internal/logging` package, not `fmt.Print` or standard `log` (enforced by golangci-lint). Enable trace logging:
```bash
export LOG_TRACE_SCOPES="jetkvm,cloud,websocket,native,jsonrpc,radar,fusion"
```

### Device Access
```bash
ssh root@<DEVICE_IP>
tail -f /var/log/jetkvm.log           # Application logs
cat /userdata/kvm_config.json         # Current configuration
```

### Native Code (CGO)
The `internal/native/cgo/ui` symlink must point to `../eez/src/ui`. If builds fail with missing `ui/ui.h`, recreate the symlink:
```bash
cd internal/native/cgo && rm ui && ln -s ../eez/src/ui ui
```

### Localization (Frontend)
All user-facing strings must use the paraglide-js localization system:
```bash
npm run i18n              # Resort, validate, compile translations
npm run i18n:audit        # Check for issues
```

---

## Configuration

RS-1 configuration stored at `/userdata/kvm_config.json`:

```json
{
  "cloud_url": "https://api.optic.works",
  "auto_update_enabled": true,
  "room_config": {
    "width": 10.0,
    "height": 8.0,
    "sensor_pose": { "m": [1,0,0,0, 0,1,0,0, 0,0,1,0, 2.5,4.0,2.8,1] }
  },
  "camera_config": {
    "resolution": "1080p",
    "fps": 30
  },
  "radar_config": {
    "uart_path": "/dev/ttyS0",
    "baud_rate": 256000
  },
  "model_path": "/userdata/models/yolov8n.rknn"
}
```

---

## Retained from JetKVM

| Component | Purpose | Status |
|-----------|---------|--------|
| WebRTC | Video/data streaming | Keep |
| JSON-RPC | Command/event protocol | Keep (modify handlers) |
| OTA | App and model updates | Keep (add model updates) |
| Supervisor | Crash recovery | Keep |
| mDNS | Device discovery | Keep |
| Network | DHCP, WiFi, static IP | Keep |
| Prometheus | Metrics | Keep (add fusion metrics) |
| LVGL UI | LCD touchscreen | Keep |

---

## Removed from JetKVM

| Component | Reason |
|-----------|--------|
| USB Gadget | No keyboard/mouse emulation |
| HID RPC | No HID device input |
| Virtual Media | No ISO mounting |
| EDID/HDMI | Using MIPI camera |
| ATX/DC Power | No power control |
| Jiggler | No mouse simulation |
| Wake-on-LAN | Not applicable |
