# Repository Guidelines

## Project Structure & Module Organization
- Go backend lives in the repo root with entry points in `main.go` and `cmd/`, plus shared packages in `internal/`, `pkg/`, and `platform/`.
- Product-specific code lives in `products/` (RS-1 radar, fusion, worldstate).
- Native pipeline + CGO bindings live in `targets/rv1106/native/`.
- Cloud API service is in `services/cloud-api/` (Cloudflare Workers).
- Frontend UI is in `ui/` (React + TypeScript). Static build output is committed to `static/` and served by the device.
- Tests are co-located with Go code as `*_test.go` files; UI end-to-end specs live in `ui/e2e/`.
- Assets and device resources are under `resource/` and `ui/public/`, with UI in-page assets in `ui/src/assets/`.
- Build and release automation is in `Makefile` and `scripts/`.

## Build, Test, and Development Commands
- `./dev_deploy.sh -r <IP>`: build backend + frontend and deploy to a device for live testing.
- `./dev_deploy.sh -r <IP> --skip-ui-build`: faster backend-only iteration; add `--run-go-tests` to execute Go tests on device.
- `make build_dev` / `make build_release`: produce device binaries (uses local toolchain or Docker buildkit).
- `make frontend`: build the device UI into `static/`.
- `make test` or `go test ./...`: run Go unit tests.
- `make test_e2e` or `cd ui && JETKVM_URL=http://<IP> npm run test:e2e`: Playwright E2E tests against a device.
- `cd ui && npm run dev` (or `npm run dev:ssl`): local UI against a device; `cd services/cloud-api && npm run dev` for the Workers API.

## Coding Style & Naming Conventions
- Go follows standard formatting (`gofmt`) and vetting (`go vet` via `make lint`). Use lowercase package names and `snake_case.go` file names.
- React components are PascalCase (`DeviceCard.tsx`), hooks are `useX`, and utilities are lower camelCase.
- UI strings must be localized: add keys to `ui/localization/messages/en.json` using `snake_case`, run `npm run i18n:compile` (or `npm run lint`), then use `m.key_name()` in `.tsx`.
- UI formatting is enforced with ESLint + Prettier (including Tailwind class sorting).

## Testing Guidelines
- Go tests live next to source files and follow `*_test.go` naming.
- E2E tests live in `ui/e2e/*.spec.ts` and run via Playwright with `JETKVM_URL` set.
- Run device tests with `./dev_deploy.sh -r <IP> --run-go-tests` when hardware integration is involved.

## Commit & Pull Request Guidelines
- Commit subjects are descriptive and imperative (e.g., "Increase RESET_CONFIG_DELAY..."). Optional scopes like `refactor(e2e):` appear in history; include issue/PR refs like `(#1081)` when relevant.
- PRs should include: a clear summary, testing performed, and screenshots for UI changes.
- Before opening a PR, ensure tests pass and new UI strings are localized (see `docs/rs1/DEVELOPMENT.md`).

## Security & Configuration Tips
- Device configuration is stored at `/userdata/opticworks/config.json` (RS-1) or `/userdata/kvm_config.json` (legacy); do not commit secrets.
- Useful dev env vars: `LOG_TRACE_SCOPES` for verbose logging and `DEVICE_PROXY_URL` for frontend dev.
