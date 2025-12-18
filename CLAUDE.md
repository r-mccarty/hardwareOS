# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

JetKVM is an open-source KVM over IP solution for remote management of computers. The project consists of:
- **Backend**: Go application running on an ARM-based embedded device (Rockchip RV1106)
- **Frontend**: React/TypeScript web UI served by the device

## Build Commands

### Full Development Deploy (to device)
```bash
./dev_deploy.sh -r <DEVICE_IP>              # Build and deploy everything
./dev_deploy.sh -r <DEVICE_IP> --skip-ui-build  # Backend only (faster)
```

### Frontend Development
```bash
cd ui
npm install
./dev_device.sh <DEVICE_IP>   # Live development with hot reload
npm run lint                  # Run ESLint
npm run build:device          # Production build for device
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

### E2E Tests
```bash
cd ui && JETKVM_URL="http://<DEVICE_IP>" npm run test:e2e
```

## Architecture

### Backend (Go)
- Entry point: `cmd/main.go`
- Root-level files handle core functionality:
  - `web.go` - HTTP/API endpoints (Gin framework)
  - `jsonrpc.go` - JSON-RPC over WebSocket for real-time communication
  - `config.go` - Configuration management (`/userdata/kvm_config.json` on device)
  - `webrtc.go` - WebRTC for video/audio streaming
  - `display.go` - Device LCD/touchscreen control
  - `cloud.go` - JetKVM Cloud connectivity
- `internal/` packages:
  - `native/cgo/` - C code for hardware interaction (HDMI capture, touchscreen via LVGL)
  - `usbgadget/` - USB device emulation (keyboard, mouse, mass storage)
  - `hidrpc/` - HID device communication protocol

### Frontend (React/TypeScript)
- `ui/src/routes/` - Page components
- `ui/src/components/` - Reusable UI components
- `ui/src/hooks/` - State management (Zustand), RPC handling, virtual devices
- Build outputs to `static/` directory (gzipped for device)

### Communication Flow
1. Browser connects to device via WebSocket
2. JSON-RPC handles bidirectional commands (keyboard/mouse input, settings)
3. WebRTC streams H.264 video from HDMI capture

## Key Development Notes

### Cross-Compilation
The backend cross-compiles for ARM (GOARCH=arm, GOARM=7). Without the native buildkit (`/opt/jetkvm-native-buildkit`), builds automatically run in Docker using `ghcr.io/jetkvm/buildkit:latest`.

### Logging
Use the `internal/logging` package, not `fmt.Print` or standard `log` (enforced by golangci-lint). Enable trace logging:
```bash
export LOG_TRACE_SCOPES="jetkvm,cloud,websocket,native,jsonrpc"
```

### Localization (Frontend)
All user-facing strings must use the paraglide-js localization system:
```bash
npm run i18n              # Resort, validate, compile translations
npm run i18n:audit        # Check for issues
```
Use `m.key_name()` function calls instead of hardcoded strings.

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
