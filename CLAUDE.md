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
| RS-1 architecture | `docs/rs1/RS1_ARCHITECTURE.md` |
| RS-1 development | `docs/rs1/DEVELOPMENT.md` |
| Fusion algorithms | `docs/rs1/FUSION_ENGINE.md` |
| Radar protocol | `docs/rs1/RADAR_INTEGRATION.md` |
| RoomPlan API | `docs/rs1/ROOMPLAN_API.md` |
| WorldState streaming | `docs/rs1/WORLDSTATE_PROTOCOL.md` |
| Vision pipeline | `docs/rs1/VISION_PIPELINE.md` |
| Cloud API spec (v1) | `docs/CLOUD_API.md` |
| Secrets (legacy) | `docs/legacy/SECRETS.md` |

## Implementation Status

**Complete**: RS-1 radar + sensor fusion, WorldState streaming, RoomPlan API, 3D visualization + CapturedRoom integration, Cloud API
**Pending**: Vision pipeline production implementation (requires Rockchip SDK)

See `CHANGELOG.md` for detailed implementation notes.

## Patterns

- **Logging**: Use `platform/logging` package. Trace: `LOG_TRACE_SCOPES="rs1,fusion,radar"`
- **WorldState**: WebRTC DataChannel label `worldstate`, 30Hz JSON stream
- **RoomPlan**: `POST /api/setup/roomplan` persists room config + sensor pose
- **Radar**: Runs in Go via `go.bug.st/serial` (not native process)
- **Secrets**: Stored in Infisical, not in repo. See `docs/legacy/SECRETS.md`
- **Cross-compile**: `GOARCH=arm GOARM=7`
