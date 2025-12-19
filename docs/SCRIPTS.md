# Scripts Reference

This document lists the scripts in this repository and what they do. Paths are repo-relative.

## Root Scripts
- `publish_source.sh`: squashes the current `main` branch and force-pushes it to the public repo (adds a `public` remote, uses a temp branch).

## Build and Release Scripts (`scripts/`)
- `scripts/dev_deploy.sh`: main device dev workflow. Builds backend/frontend, deploys to a device over SSH, and can run Go tests or native debug builds.
- `scripts/build_utils.sh`: shared helpers for colored logging and Docker-based builds (`do_make`, `build_docker_image`).
- `scripts/build_cgo.sh`: builds the native C/CGo library with the Rockchip toolchain and generates `ui_index.c`.
- `scripts/deploy_cloud_app.sh`: builds and uploads the cloud UI to R2 (versioned and optional root deployment).
- `scripts/test_release_on_device.sh`: deploys a binary to a device, verifies version, runs UI E2E tests, then restores the previous binary.
- `scripts/ci_helper.sh`: CI helper to prepare Docker build context or run `make` inside Docker.
- `scripts/generate_proto.sh`: regenerates gRPC stubs from `internal/native/proto/native.proto`.
- `scripts/update_netboot_xyz.sh`: fetches the latest netboot.xyz ISO into `resource/` and can auto-commit the update.
- `scripts/release.sh`: system release helper (currently exits early after unpacking `update.img`).
- `scripts/configure_vscode.py`: writes `.vscode/c_cpp_properties.json` for the Rockchip toolchain.

## Native UI Codegen (`internal/native/cgo/`)
- `internal/native/cgo/ui_index.gen.sh`: generates `ui_index.c` from LVGL UI headers.

## Device Test Runner (`resource/`)
- `resource/dev_test.sh`: wrapper invoked by device test bundles; runs Go test binaries and optionally emits JSON output.

## UI Dev and Localization Tools (`ui/`)
- `ui/dev_device.sh`: starts the Vite dev server pointed at a device IP (`JETKVM_PROXY_URL`).
- `ui/tools/resort_messages.py`: re-sorts localization JSON files (keeps `$schema` first).
- `ui/tools/find_excess_messages.py`: finds keys present in other locales but not `en.json`.
- `ui/tools/find_unused_messages.py`: scans the UI codebase for unused localization keys.
- `ui/tools/find_duplicate_translations.py`: detects duplicate translation targets in `en.json`.

## Notes
- Most deployment scripts assume SSH access to the device and paths under `/userdata/jetkvm/`.
- Release and cloud deploy scripts require external tools (e.g., `docker`, `rclone`, `jq`, `curl`).
