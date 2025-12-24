# CLAUDE.md

Agent entrypoint for the HardwareOS repository.

## What This Is

**HardwareOS** - Embedded platform for OpticWorks sensor devices.
**OpticWorks RS-1** - Flagship occupancy sensor with radar + vision, RoomPlan, and live WorldState streaming.

## Directory Structure

```
hardwareos/
├── cmd/                            # Go entrypoints
├── *.go                            # Application core wiring (config, web, RoomPlan, WebRTC)
├── internal/, pkg/, platform/      # Shared Go packages and services
├── products/rs1/                   # RS-1 radar, fusion, worldstate
├── targets/rv1106/native/          # Native C pipeline, proto, CGO bindings
├── services/cloud-api/             # Cloudflare Workers backend (TypeScript)
├── ui/                             # React frontend + 3D visualization
├── static/                         # Committed UI build output
└── docs/                           # RS-1 docs, PRDs, and legacy archive
```

## Commands

```bash
# Build & deploy
./dev_deploy.sh -r <DEVICE_IP>
./dev_deploy.sh -r <DEVICE_IP> --skip-ui-build
./dev_deploy.sh -r <DEVICE_IP> --run-go-tests

# Build
make build_dev
make build_release
make frontend

# Test
go test ./...
make test_e2e

# UI
cd ui && npm run dev

# Cloud API
cd services/cloud-api && npm run dev
```

## Key Documentation

| Topic | File |
|-------|------|
| Docs index | `docs/README.md` |
| Platform architecture | `docs/platform/ARCHITECTURE.md` |
| RS-1 architecture | `docs/rs1/RS1_ARCHITECTURE.md` |
| RS-1 development | `docs/rs1/DEVELOPMENT.md` |
| Fusion algorithms | `docs/rs1/FUSION_ENGINE.md` |
| Radar protocol | `docs/rs1/RADAR_INTEGRATION.md` |
| RoomPlan API | `docs/rs1/ROOMPLAN_API.md` |
| WorldState streaming | `docs/rs1/WORLDSTATE_PROTOCOL.md` |
| Vision pipeline | `docs/rs1/VISION_PIPELINE.md` |
| Camera sensors | `docs/rs1/CAMERA_SENSORS.md` |
| Cloud API spec (v1) | `docs/CLOUD_API.md` |
| Secrets | `docs/platform/SECRETS.md` |

## Implementation Status

**Complete**: RS-1 radar + sensor fusion, WorldState streaming, RoomPlan API, 3D visualization + CapturedRoom integration, Cloud API
**Pending**: Vision pipeline production implementation (requires Rockchip SDK)

See `CHANGELOG.md` for detailed implementation notes.

## Coder Workspace (HIL Testing)

This repo is deployed to `coder.hardwareos.com` for hardware-in-the-loop testing with the Luckfox Pico Max dev board.

### Pre-configured Access

| Service | Status | Notes |
|---------|--------|-------|
| **GitHub CLI** | Authenticated | `gh` commands work, account: `r-mccarty` |
| **Infisical Secrets** | Pre-loaded | All secrets in `~/.env.secrets` |
| **Go** | v1.24.4 | Ready for builds |
| **Node.js** | v22.21.0 | UI dependencies installed |

### Luckfox Pico Max (Dev Board)

| Method | Status | Command |
|--------|--------|---------|
| **ADB** | Working | `sudo adb shell` |
| **USB RNDIS** | Working | `ping 172.32.0.93` (after IP setup) |
| **Ethernet** | Available | Connect cable to `enp1s0` |

**First-time RNDIS setup**:
```bash
sudo ip addr add 172.32.0.1/24 dev enx5aef472011ac
adb kill-server && sudo adb start-server
sudo adb shell
```

### Build Limitation

Docker builds require BuildKit which has overlay issues in nested containers. Workarounds:
1. **GitHub Actions**: Use CI for release builds
2. **Host Docker**: Run builds outside Coder container on N100 host
3. **Pre-built binaries**: Deploy via SCP from another machine

### HIL Testing Docs

| Doc | Purpose |
|-----|---------|
| `docs/rs1/CAMERA_HIL_INTEGRATION.md` | Camera sensor integration plan |
| `docs/rs1/CAMERA_SENSORS.md` | Sensor specs and comparison |
| `docs/rs1/ROOMPLAN_INTEGRATION_TESTING.md` | iPhone LiDAR testing |

## Patterns

- **Logging**: Use `platform/logging` package. Trace: `LOG_TRACE_SCOPES="rs1,fusion,radar"`
- **WorldState**: WebRTC DataChannel label `worldstate`, 30Hz JSON stream
- **RoomPlan**: `POST /api/setup/roomplan` persists room config + sensor pose
- **Radar**: Runs in Go via `go.bug.st/serial` (not native process)
- **Secrets**: Stored in Infisical, not in repo. See `docs/platform/SECRETS.md`
- **Cross-compile**: `GOARCH=arm GOARM=7`
