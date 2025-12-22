# CLAUDE.md

Agent entrypoint for the HardwareOS repository.

## What This Is

**HardwareOS** - Embedded platform for connected sensor devices.
**OpticWorks RS-1** - Flagship product: vision/radar sensor fusion for occupancy tracking.

## Directory Structure

```
hardwareos/
├── main.go, config.go, web.go      # Application core
├── webrtc.go, webrtc_worldstate.go # WebRTC streaming
├── roomplan.go                     # RoomPlan API
├── products/rs1/                   # RS-1 product code
│   ├── fusion/                     # Kalman filter, Hungarian algorithm
│   └── radar/                      # LD2450 driver
├── services/cloud-api/             # Cloudflare Workers backend (TypeScript)
├── targets/rv1106/native/          # Native C code (camera/ISP/NPU stubs)
├── platform/                       # Shared services (logging, ota, network)
├── ui/                             # React frontend
└── docs/                           # Documentation
```

## Commands

```bash
# Build & deploy
./dev_deploy.sh -r <DEVICE_IP>
./dev_deploy.sh -r <DEVICE_IP> --skip-ui-build

# Test
go test ./products/rs1/...

# Cloud API
cd services/cloud-api && npm run dev
```

## Key Documentation

| Topic | File |
|-------|------|
| Platform architecture | `docs/ARCHITECTURE.md` |
| Fusion algorithms | `docs/rs1/FUSION_ENGINE.md` |
| Radar protocol | `docs/rs1/RADAR_INTEGRATION.md` |
| WorldState streaming | `docs/rs1/WORLDSTATE_PROTOCOL.md` |
| Cloud API spec | `docs/rs1/CLOUD_API_v2.md` |
| Secrets (Infisical) | `docs/SECRETS.md` |

## Implementation Status

**Complete**: Sensor fusion, radar driver, WorldState streaming, RoomPlan API, Cloud API
**Pending**: Vision pipeline (requires Rockchip SDK)

See `CHANGELOG.md` for detailed implementation notes.

## Patterns

- **Logging**: Use `platform/logging` package. Trace: `LOG_TRACE_SCOPES="rs1,fusion,radar"`
- **Radar**: Runs in Go via `go.bug.st/serial` (not native process)
- **Secrets**: Stored in Infisical, not in repo. See `docs/SECRETS.md`
- **Cross-compile**: `GOARCH=arm GOARM=7`
