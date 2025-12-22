# Porting Guide (RV1106G and Custom Boards)

This guide outlines the hardware assumptions, code touch points, and bring-up steps for porting JetKVM to a custom RV1106G-based board.

## 1) Baseline Architecture
- Go app (`main.go`) orchestrates services, config, networking, USB gadget, WebRTC, and OTA.
- Native process (`internal/native/*`, C/CGo) handles HDMI capture, H.264 encode, and on-device LVGL UI.
- Local and cloud access both use WebRTC with JSON-RPC + HID-RPC over data channels.

## 2) Hardware Dependencies (Current Assumptions)
### Video capture + EDID
- `/dev/video0` (V4L2 capture) and `/dev/v4l-subdev2` (EDID/log status).
- EDID is managed via `VIDIOC_G_EDID`/`VIDIOC_S_EDID` in `internal/native/cgo/edid.c`.

### H.264 encoding (Rockchip MPP)
- `internal/native/cgo/video.c` uses `rk_mpi_venc`, `rk_mpi_sys`, `rk_mpi_mb`, `rk_mpi_mmz`.
- Input format: YUV422 (YUYV) and H.264 VBR output.

### On-device display + touch
- LVGL + fbdev uses `/dev/fb0` in `internal/native/cgo/screen.c`.
- Touchscreen input via LVGL evdev discovery.

### HDMI sleep mode control
- Sysfs: `/sys/devices/platform/ff470000.i2c/i2c-4/4-000f/sleep_mode`.

### USB gadget
- Uses Linux configfs gadget (`internal/usbgadget/*`).
- Mass storage, keyboard, and mouse gadgets are expected.

### Other OS interfaces
- Watchdog: `/dev/watchdog` (`hw.go`).
- Serial extension: `/dev/ttyS3` (`serial.go`).
- Network interface: `eth0` (`network.go`).
- Virtual media NBD: `/dev/nbd0` (`block_device.go`).

## 3) Porting Checklist (High Level)
1. Validate kernel features: V4L2, media controller, MPP, fbdev/DRM, configfs USB gadget, nbd.
2. Map device nodes to your board (video, subdev, framebuffer, serial, watchdog).
3. Update native build toolchain and libraries (Rockchip SDK paths, MPP libs).
4. Adjust LVGL display size/rotation to match your panel.
5. Verify USB gadget descriptors and capabilities.
6. Bring up WebRTC streaming + HID input on a target host.

## 4) Code Touch Points (What to Change)
### Video pipeline
- `internal/native/cgo/video.c`: video device node, pixel format, encoder settings.
- `internal/native/cgo/edid.c`: subdevice path (`/dev/v4l-subdev2`).
- `video.go` + `native.go`: app-side control and video state updates.

### Display + touch
- `internal/native/cgo/screen.c`: framebuffer path (`/dev/fb0`), LVGL display size.
- `internal/native/eez/*`: UI assets and layout.
- `display.go`: runtime UI state updates (network status, USB state, etc.).

### HDMI sleep mode
- `internal/native/cgo/video.c`: sleep mode sysfs path.

### USB gadget
- `internal/usbgadget/*`: gadget configuration and USB function wiring.
- `usb.go`, `usb_mass_storage.go`: runtime behavior and virtual media.

### Board services
- `hw.go`: watchdog handling.
- `serial.go`: extension controller UART device and protocol.
- `network.go`: network interface name and DHCP behavior.

### File system expectations
- Config: `/userdata/kvm_config.json`.
- Logs: `/userdata/jetkvm/last.log`, crash dumps in `/userdata/jetkvm/crashdump/`.
- Virtual media storage: `/userdata/jetkvm/images`.

## 5) Toolchain and Native Build
- Native build uses `internal/native/cgo/CMakeLists.txt` with Rockchip SDK paths.
- `scripts/build_cgo.sh` expects a buildkit or SDK at `/opt/jetkvm-native-buildkit`.
- Linked libs: `rockchip_mpp`, `rga`, `lvgl`.

If your SDK path differs, update `CMAKE_TOOLCHAIN_FILE` and `RK_SDK_BASE` in the native build scripts/CMake.

## 6) Bring-Up Steps (Suggested)
1. **Kernel validation**
   - Confirm `/dev/video0`, `/dev/v4l-subdev*`, `/dev/fb0`, `/dev/watchdog`.
   - Confirm USB gadget configfs is mounted and supports mass storage + HID.
2. **Native process**
   - Run `make build_dev` and start the app.
   - Watch native logs for video state and LVGL init.
3. **WebRTC path**
   - Access device UI and confirm video stream appears.
   - Verify HID input (keyboard/mouse) to target host.
4. **USB mass storage**
   - Mount a local or HTTP ISO and confirm the target sees a USB device.
5. **OTA + config**
   - Check `/device/status`, `otaState` updates, and config persistence.

## 7) Common Porting Pitfalls
- Wrong V4L2 subdevice index for EDID.
- Pixel format mismatch (YUYV vs other formats).
- Missing fbdev or mismatched display resolution.
- USB gadget not configured in kernel or device tree.
- Interface name not `eth0`.
- Watchdog/serial device nodes differ across kernels.

## 8) Validation Matrix (Quick Reference)
| Subsystem | Expected Signal | Primary Files |
| --- | --- | --- |
| Video capture | H.264 frames to WebRTC | `internal/native/cgo/video.c`, `webrtc.go` |
| EDID | Custom EDID roundtrip | `internal/native/cgo/edid.c` |
| Display UI | LVGL UI visible | `internal/native/cgo/screen.c`, `display.go` |
| USB HID | Key/mouse input to host | `hidrpc.go`, `usb.go` |
| Mass storage | Target sees USB disk | `usb_mass_storage.go`, `block_device.go` |
| Networking | DHCP + mDNS | `network.go`, `mdns.go` |
| OTA | Update status + reboot | `ota.go`, `internal/ota/*` |

## 9) Recommended Modularity Improvements (Optional)
- Make device nodes and interface names configurable via config or env.
- Add a board profile file to centralize paths and hardware features.
- Wrap native video paths behind a small abstraction to support multiple capture pipelines.
