# Native-Side Peripherals (Rockchip)

This document summarizes the SoC peripherals and kernel interfaces used by the native (C/CGo) layer. It is intended to guide porting to custom RV1106G boards.

## HDMI Capture and Control
**Files:** `internal/native/cgo/video.c`, `internal/native/cgo/edid.c`
- **Video device:** `/dev/video0` (V4L2 capture)
- **Subdevice:** `/dev/v4l-subdev2` (EDID + log status)
- **IOCTLs:** `VIDIOC_G_EDID`, `VIDIOC_S_EDID`, `VIDIOC_LOG_STATUS`
- **Purpose:** capture HDMI frames, manage EDID, and report video status

## Rockchip Media Processing Platform (MPP)
**Files:** `internal/native/cgo/video.c`
- Uses Rockchip MPP to encode H.264:
  - `rk_mpi_venc`, `rk_mpi_sys`, `rk_mpi_mb`, `rk_mpi_mmz`
- Encodes YUV422 input into H.264 for WebRTC streaming.

## HDMI Sleep Mode (I2C Sysfs)
**Files:** `internal/native/cgo/video.c`
- **Sysfs path:** `/sys/devices/platform/ff470000.i2c/i2c-4/4-000f/sleep_mode`
- **Purpose:** disable HDMI sleep mode to keep capture alive.

## On-Device Display (LVGL)
**Files:** `internal/native/cgo/screen.c`, `internal/native/cgo/ctrl.c`
- **Framebuffer:** `/dev/fb0` (LVGL fbdev backend)
- **Input:** LVGL evdev discovery for touchscreen events
- **Purpose:** render local UI and react to on-device input.

## Linked Rockchip Libraries
**Files:** `internal/native/cgo/CMakeLists.txt`
- Links `rockchip_mpp` and `rga` (RGA is linked but not directly called in C sources).

## Notes for Custom Boards
- Device nodes (`/dev/video0`, `/dev/v4l-subdev2`, `/dev/fb0`) and the HDMI sleep sysfs path are hard-coded. Adjust for your board or add configuration.
- If capture pipelines differ (e.g., different V4L2 subdevice index), EDID control may need new paths.
- LVGL display resolution and rotation are set in `internal/native/cgo/screen.c` and UI assets live in `internal/native/eez/`.
