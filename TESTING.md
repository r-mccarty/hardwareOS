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

## Adding Tests
- Backend: add `*_test.go` next to the package under test.
- Device flows: add/update Playwright specs in `ui/e2e/` and keep device assumptions explicit.
- When changing user-visible UI, validate with Playwright (if applicable) and manual device testing.
