<div align="center">

# HardwareOS

### By OpticWorks

[Website](https://optic.works) | [Issues](https://github.com/opticworks/hardwareOS/issues) | [Docs](https://optic.works/docs)

</div>

HardwareOS is an open-source embedded automation platform developed by OpticWorks. It provides a common foundation for building connected hardware devices with features like WebRTC streaming, sensor fusion, OTA updates, and home automation integration.

## RS-1: Flagship Product

The **OpticWorks RS-1** is the first product built on HardwareOS—a vision/radar sensor fusion device for spatial tracking. It combines:

- **SC3336 MIPI Camera** with ISP lens distortion correction
- **RKNN NPU** for YOLOv8 object detection
- **LD2450 24GHz Radar** for range/velocity measurement
- **Sensor Fusion Engine** with Kalman filtering
- **WorldState Streaming** via WebRTC DataChannel
- **RoomPlan API** for iPhone integration

## Features

- **Ultra-low Latency Streaming** - 1080p@60FPS video with 30-60ms latency using H.265 encoding
- **Remote Access** - Cloud connectivity via WebRTC through OpticWorks Cloud
- **Open-source Software** - Written in Go on Linux, easily customizable through SSH
- **OTA Updates** - Automatic firmware and model updates
- **Sensor Fusion** - Real-time object tracking with vision + radar

## Platform Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        HardwareOS                            │
│    (Platform: WebRTC, JSON-RPC, OTA, Networking, WoL)       │
├─────────────────────────────────────────────────────────────┤
│                    Hardware Targets                          │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐         │
│  │   RV1106G   │  │   (Future)  │  │   (Future)  │         │
│  │  First HW   │  │             │  │             │         │
│  └─────────────┘  └─────────────┘  └─────────────┘         │
├─────────────────────────────────────────────────────────────┤
│                       Products                               │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐         │
│  │ OpticWorks  │  │   (Future)  │  │   (Future)  │         │
│  │    RS-1     │  │             │  │             │         │
│  └─────────────┘  └─────────────┘  └─────────────┘         │
└─────────────────────────────────────────────────────────────┘
```

## Contributing

We welcome contributions from the community! Whether it's improving the firmware, adding new features, or enhancing documentation, your input is valuable. Please read our [Code of Conduct](/CODE_OF_CONDUCT.md) before contributing.

## Getting Help

- **Documentation**: Visit our [Docs](https://optic.works/docs)
- **Issues**: Report bugs or request features on [GitHub Issues](https://github.com/opticworks/hardwareOS/issues)

# Development

HardwareOS is written in Go & TypeScript, with some components in C. An intermediate level of Go & TypeScript knowledge is recommended.

The project contains:
- **Backend**: Go application running on the device
- **Frontend**: React/TypeScript UI served by the device
- **Native**: C code for hardware access (camera, ISP, NPU)

For comprehensive development information, see **[docs/DEVELOPMENT.md](docs/DEVELOPMENT.md)**.

## Quick Start

For quick device development, use the `./dev_deploy.sh` script:

```bash
./dev_deploy.sh -r <DEVICE_IP>              # Build and deploy everything
./dev_deploy.sh -r <DEVICE_IP> --skip-ui-build  # Backend only (faster)
```

## Backend

The backend is written in Go and handles device management, cloud API integration, sensor fusion, and WebRTC streaming.

## Frontend

The frontend is written in React and TypeScript. Build targets:
- `device`: Production build for the hardware device
- `cloud-development`: Development against cloud backend
- `cloud-production`: Production cloud deployment

## Documentation

| Document | Description |
|----------|-------------|
| [DEVELOPMENT.md](docs/DEVELOPMENT.md) | Development setup and workflows |
| [ARCHITECTURE.md](docs/ARCHITECTURE.md) | System architecture overview |
| [RS1_ARCHITECTURE.md](docs/rs1/RS1_ARCHITECTURE.md) | RS-1 specific architecture |

## License

This project is licensed under the GNU General Public License v2.0 - see the [LICENSE](LICENSE) file for details.

---

**OpticWorks** - Building the future of connected hardware
