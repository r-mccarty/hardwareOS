# Testing Methodology and Coverage

## Overview
JetKVM testing is device-first: most meaningful validation happens against real hardware, with automated Go tests for core logic and Playwright E2E tests for UI flows. The repository favors integration-level checks over heavy unit-test isolation because the product spans firmware, networking, and a live device UI.

## Test Layers
### Go Unit and Package Tests
- Location: alongside source as `*_test.go` files in the repo root, `internal/`, and `pkg/`.
- Runner: `go test ./...` or `make test`.
- Coverage focus: core backend logic, configuration, and service behavior that can run on host without a device.

### Device-Integrated Go Tests
- Runner: `./dev_deploy.sh -r <IP> --run-go-tests` or `make build_dev_test` + device execution.
- Method: cross-compiles tests and executes them on the device using `gotestsum`.
- Coverage focus: hardware- and platform-dependent code paths (native integrations, device services).

### UI End-to-End Tests (Playwright)
- Location: `ui/e2e/*.spec.ts`.
- Runner: `make test_e2e` or `cd ui && JETKVM_URL=http://<IP> npm run test:e2e`.
- Coverage focus: key user workflows and device settings. Examples include HDMI sleep mode, USB device handling, config reset, and round-trip input checks.

### Linting and Static Checks
- Backend: `make lint` runs `go vet`.
- Frontend: `cd ui && npm run lint` (ESLint + Prettier rules, also compiles i18n messages).

## Coverage Summary
- Strongest automation: backend package tests and Playwright E2E flows against a real device.
- Moderate coverage: device-integrated Go tests for hardware paths (requires device access).
- Limited automation: UI unit tests are not present; UI correctness is validated via E2E tests and manual verification.
- No repo-wide coverage thresholds or coverage reports are configured.

## Running Tests Locally
```bash
# Go unit tests
make test

# Device-integrated Go tests
./dev_deploy.sh -r <IP> --run-go-tests

# UI linting
cd ui && npm run lint

# Playwright E2E (device required)
cd ui && JETKVM_URL=http://<IP> npm run test:e2e
```

## Failsafe Mode (Testing Without Hardware)

The application includes a **failsafe mode** that allows testing without RV1106 hardware. In this mode, the native subsystem (video capture, display, hardware encoders) is bypassed, but all Go-based functionality remains operational.

### What Works in Failsafe Mode
- JSON-RPC API handlers
- Network management (DHCP, static IP, WiFi)
- Cloud connectivity and WebSocket signaling
- OTA update checking (not applying)
- Configuration load/save
- mDNS discovery
- Wake-on-LAN
- Prometheus metrics
- Web server (HTTP/HTTPS)

### What Does NOT Work in Failsafe Mode
- Video capture and streaming
- Display/LCD output
- Hardware encoder (H.264/H.265)
- Native C code execution
- Touchscreen input

### Activating Failsafe Mode

**Option 1: Environment Variable (Recommended for Testing)**
```bash
JETKVM_FORCE_FAILSAFE=1 ./bin/opticworks-rs1_app
```

**Option 2: Failsafe File (On Device)**
```bash
touch /userdata/jetkvm/.enablefailsafe
# Restart application - file is auto-removed after activation
```

**Option 3: Automatic Activation**
Failsafe mode activates automatically when:
- The last crash log contains video-related panics
- The native subsystem repeatedly fails to start

### Use Cases

1. **Local Development**: Test API changes without cross-compiling for ARM
2. **CI/CD Pipelines**: Run integration tests in x86_64 containers
3. **Debugging**: Isolate issues to Go code vs native code
4. **API Development**: Develop and test JSON-RPC handlers

### Example: Running in Docker for CI
```bash
# Build for local architecture (skip native)
go build -tags netgo,timetzdata,nomsgpack -o bin/test-app ./cmd/main.go

# Run in failsafe mode
JETKVM_FORCE_FAILSAFE=1 ./bin/test-app
```

### Detecting Failsafe Mode
The application logs failsafe activation:
```
WARN failsafe mode activated reason=failsafe_env_set
```

Clients receive a `failsafeMode` JSON-RPC event:
```json
{"jsonrpc": "2.0", "method": "failsafeMode", "params": {"active": true, "reason": "failsafe_env_set"}}
```

## Adding Tests
- Backend: add `*_test.go` next to the package under test.
- Device flows: add/update Playwright specs in `ui/e2e/` and keep device assumptions explicit.
- When changing user-visible UI, validate with Playwright (if applicable) and manual device testing.
