<div align="center">

# HardwareOS

### By OpticWorks

[Website](https://optic.works) | [Documentation](docs/) | [Issues](https://github.com/opticworks/hardwareOS/issues)

</div>

HardwareOS is an open-source embedded platform for building connected sensor devices. Built on a foundation of low-latency WebRTC streaming, sensor fusion, and OTA updates, it enables rapid development of spatial awareness products.

## OpticWorks RS-1

The **RS-1** is the flagship product—a vision/radar sensor fusion device for real-time occupancy tracking and spatial awareness.

### Capabilities

| Feature | Description |
|---------|-------------|
| **Sensor Fusion** | Combines 24GHz radar with camera vision for accurate tracking |
| **Real-time Streaming** | 30Hz WorldState updates via WebRTC DataChannel |
| **Kalman Filtering** | 4-state tracking with velocity estimation |
| **Hungarian Algorithm** | Optimal detection-to-track association |
| **Room Calibration** | iPhone RoomPlan integration for sensor pose configuration |
| **Low Latency** | End-to-end latency under 100ms |

### Hardware

- **Rockchip RV1106G3** - ARM Cortex-A7 @ 1.2GHz, 256MB RAM
- **SC3336 MIPI Camera** - 3MP with LDCH lens distortion correction (4-lane MIPI CSI)
- **RKNN NPU** - 1.0 TOPS for YOLOv8 inference
- **HLK-LD2450** - 24GHz mmWave radar (3 targets, 6m range)

## Architecture

```
┌───────────────────────────────────────────────────────────────┐
│                    Browser / iPhone App                        │
└───────────────────────────────▲───────────────────────────────┘
                                │ WebRTC (30Hz JSON)
┌───────────────────────────────┴───────────────────────────────┐
│                       Go Application                           │
│                                                                │
│  ┌──────────────────────────────────────────────────────────┐ │
│  │                    WorldState Manager                     │ │
│  └──────────────────────────────▲───────────────────────────┘ │
│                                 │                              │
│  ┌──────────────────────────────┴───────────────────────────┐ │
│  │                     Fusion Engine                         │ │
│  │  ┌─────────────────┐      ┌─────────────────┐            │ │
│  │  │    Hungarian    │ ───▶ │     Kalman      │            │ │
│  │  │   (associate)   │      │    (filter)     │            │ │
│  │  └────────▲────────┘      └─────────────────┘            │ │
│  └───────────┼──────────────────────────────────────────────┘ │
│              │                                                 │
│  ┌───────────┴───────────┐       ┌────────────────────────┐  │
│  │   Radar Parser (Go)   │       │   RoomPlan API (REST)  │  │
│  └───────────▲───────────┘       └────────────────────────┘  │
└──────────────┼────────────────────────────────────────────────┘
               │ Serial (256kbps)
┌──────────────┴────────────────┐
│     LD2450 mmWave Radar       │
│   24GHz · 3 targets · 6m      │
└───────────────────────────────┘
```

**Data Flow:** Radar → Hungarian (match detections) → Kalman (filter/track) → WorldState → WebRTC

*Vision pipeline (Camera → NPU) not yet implemented - see `targets/rv1106/native/`*

## Quick Start

### Development Deploy

```bash
# Full build and deploy
./dev_deploy.sh -r <DEVICE_IP>

# Backend only (faster iteration)
./dev_deploy.sh -r <DEVICE_IP> --skip-ui-build

# Run tests on device
./dev_deploy.sh -r <DEVICE_IP> --run-go-tests
```

### Local Development

```bash
# Run tests
go test ./products/rs1/...

# Build
go build ./...

# Frontend development
cd ui && npm run dev
```

## Project Structure

```
hardwareos/
├── products/rs1/           # RS-1 product implementation
│   ├── fusion/             # Sensor fusion algorithms
│   │   ├── engine.go       # Fusion coordinator
│   │   ├── kalman.go       # Kalman filter
│   │   ├── hungarian.go    # Data association
│   │   └── transform.go    # Coordinate transforms
│   ├── radar/              # LD2450 radar driver
│   ├── worldstate.go       # Occupancy state manager
│   └── init.go             # Product initialization
├── targets/rv1106/         # RV1106 hardware target
│   └── native/             # Native C code + gRPC
├── platform/               # Platform abstractions
├── internal/               # Internal packages
├── ui/                     # React frontend
└── docs/                   # Documentation
```

## Documentation

See `docs/README.md` for the RS-1 docs index, platform docs, PRD templates, and the legacy archive.

### RS-1 Product (Current)

| Document | Description |
|----------|-------------|
| [RS1_ARCHITECTURE.md](docs/rs1/RS1_ARCHITECTURE.md) | System overview and data flow |
| [DEVELOPMENT.md](docs/rs1/DEVELOPMENT.md) | Build, deploy, and test workflows |
| [FUSION_ENGINE.md](docs/rs1/FUSION_ENGINE.md) | Kalman filter and Hungarian algorithm |
| [RADAR_INTEGRATION.md](docs/rs1/RADAR_INTEGRATION.md) | LD2450 protocol implementation |
| [VISION_PIPELINE.md](docs/rs1/VISION_PIPELINE.md) | Camera/ISP/NPU pipeline |
| [WORLDSTATE_PROTOCOL.md](docs/rs1/WORLDSTATE_PROTOCOL.md) | WebRTC streaming protocol |
| [ROOMPLAN_API.md](docs/rs1/ROOMPLAN_API.md) | Room calibration endpoints |
| [CLOUD_API.md](docs/CLOUD_API.md) | Cloud API (v1, current) |

### Platform + Legacy

Platform docs live under [docs/platform](docs/platform), with legacy material archived in [docs/legacy](docs/legacy/README.md).

## API Endpoints

### RoomPlan Configuration

```bash
# Upload room configuration
curl -X POST http://device/api/setup/roomplan \
  -H "Content-Type: application/json" \
  -d '{
    "room_width": 10.0,
    "room_height": 8.0,
    "sensor_pose": [1,0,0,0, 0,1,0,0, 0,0,1,0, 2.5,4.0,2.8,1]
  }'

# Get current configuration
curl http://device/api/setup/roomplan
```

### WorldState Streaming

Connect via WebRTC and open the `worldstate` DataChannel to receive 30Hz updates:

```json
{
  "timestamp_ns": 1703251200000000000,
  "frame_number": 12345,
  "occupant_count": 2,
  "objects": [
    {
      "track_id": "track_001",
      "x": 3.5,
      "y": 2.1,
      "vx": 0.5,
      "vy": -0.2,
      "confidence": 0.95,
      "heading": 68.2,
      "speed": 0.54
    }
  ]
}
```

## Contributing

We welcome contributions! See our [Code of Conduct](CODE_OF_CONDUCT.md) for guidelines.

## License

GNU General Public License v2.0 - see [LICENSE](LICENSE) for details.

---

**OpticWorks** - Building spatial intelligence for the physical world
