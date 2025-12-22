# RS-1 Development Guide

This guide captures the current RS-1 workflows for building, testing, and deploying.

## Build and Deploy

```bash
# Build + deploy to device
./dev_deploy.sh -r <DEVICE_IP>

# Backend only (skip UI rebuild)
./dev_deploy.sh -r <DEVICE_IP> --skip-ui-build

# Run Go tests on device
./dev_deploy.sh -r <DEVICE_IP> --run-go-tests
```

## Local Builds

```bash
# Device binaries
make build_dev
make build_release

# Frontend build (writes to static/)
make frontend
```

## Tests

```bash
# Go unit tests
make test
# or
go test ./...

# UI E2E (device required)
make test_e2e
# or
cd ui && JETKVM_URL=http://<DEVICE_IP> npm run test:e2e
```

## UI Development

```bash
# Local UI against a device
cd ui && npm run dev

# Cloud UI against the Workers API
cd ui && npm run dev:cloud
```

## Cloud API (Workers)

```bash
cd services/cloud-api
npm run dev
```

## Notes

- Device configuration: `/userdata/opticworks/config.json` (legacy: `/userdata/kvm_config.json`).
- Enable verbose logging with `LOG_TRACE_SCOPES` (example: `rs1,fusion,radar`).
- For frontend device routing, use `DEVICE_PROXY_URL` when needed.
