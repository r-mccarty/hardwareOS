# RS-1 Vision Pipeline Development

This skill provides guidance for developing the native C vision pipeline for the OpticWorks RS-1 platform.

## Overview

The vision pipeline runs in the native C process and handles:
- SC3336 MIPI camera initialization via V4L2
- Rockchip ISP configuration with LDCH lens correction
- RKNN NPU inference for YOLOv8 object detection
- H.265 video encoding via MPP
- gRPC event streaming to the Go application

## Key Files

| File | Purpose |
|------|---------|
| `internal/native/cgo/camera.c` | SC3336 sensor initialization |
| `internal/native/cgo/isp.c` | ISP/LDCH configuration |
| `internal/native/cgo/npu.c` | RKNN YOLOv8 inference |
| `internal/native/cgo/video.c` | Pipeline orchestration (heavily modified from JetKVM) |
| `internal/native/proto/native.proto` | VisionFrame, VisionDetection messages |

## Architecture

```
SC3336 MIPI → V4L2 → ISP (LDCH) → [Fork]
                                    ├→ RGA (1080p) → H.265 Encoder → video_send_frame()
                                    └→ RGA (640x640) → NPU → vision_send_frame()
```

## Development Guidelines

### Camera Initialization

Use V4L2 for MIPI CSI camera access:

```c
// Open camera device
int fd = open("/dev/video0", O_RDWR);

// Set format for MIPI sensor
struct v4l2_format fmt = {0};
fmt.type = V4L2_BUF_TYPE_VIDEO_CAPTURE_MPLANE;
fmt.fmt.pix_mp.width = 2304;
fmt.fmt.pix_mp.height = 1296;
fmt.fmt.pix_mp.pixelformat = V4L2_PIX_FMT_NV12;  // After ISP
fmt.fmt.pix_mp.num_planes = 2;
ioctl(fd, VIDIOC_S_FMT, &fmt);
```

### ISP with LDCH

Enable lens distortion correction for accurate azimuth calculation:

```c
#include <rkaiq/rk_aiq_user_api2_imgproc.h>

// Initialize AIQ context
rk_aiq_sys_ctx_t* ctx = rk_aiq_uapi2_sysctl_init(...);

// Enable LDCH (Lens Distortion Correction for Horizontal lines)
rk_aiq_uapi2_setLdchEn(ctx, true);

// Set maximum correction for cylindrical projection
rk_aiq_uapi2_setLdchCorrectLevel(ctx, 255);

// Start 3A processing
rk_aiq_uapi2_sysctl_start(ctx);
```

### NPU Inference

Load and run RKNN model:

```c
#include <rknn_api.h>

// Initialize
rknn_context ctx;
rknn_init(&ctx, model_data, model_size, 0, NULL);

// Set input (640x640 RGB)
rknn_input inputs[1] = {0};
inputs[0].index = 0;
inputs[0].type = RKNN_TENSOR_UINT8;
inputs[0].fmt = RKNN_TENSOR_NHWC;
inputs[0].buf = rgb_data;
inputs[0].size = 640 * 640 * 3;
rknn_inputs_set(ctx, 1, inputs);

// Run inference
rknn_run(ctx, NULL);

// Get outputs
rknn_output outputs[3];
rknn_outputs_get(ctx, 3, outputs, NULL);

// Parse YOLO output format
parse_yolo_detections(outputs, detections, &count);
```

### Azimuth Calculation

With LDCH cylindrical projection, X pixel maps linearly to azimuth:

```c
float calculate_azimuth(float bbox_center_x_normalized, float fov_degrees) {
    // bbox_center_x_normalized: 0.0 (left) to 1.0 (right)
    float center_offset = bbox_center_x_normalized - 0.5f;
    return center_offset * fov_degrees;  // degrees from center
}

// Example: FOV = 120 degrees
// X = 0.25 → azimuth = -30 degrees (left of center)
// X = 0.75 → azimuth = +30 degrees (right of center)
```

### Frame Callback

Report detections to Go via gRPC:

```c
void vision_send_frame(VisionDetection* detections, int count, int64_t timestamp_ns) {
    // This callback is registered in ctrl.c
    // Sends VisionFrame to the gRPC stream
}
```

## Testing

### On Device

```bash
# Check camera device
v4l2-ctl --list-devices
v4l2-ctl -d /dev/video0 --all

# Test ISP
cat /proc/rkisp0-vir0

# Check NPU
cat /proc/rknpu/version
```

### Model Conversion

Convert ONNX to RKNN:

```python
from rknn.api import RKNN

rknn = RKNN()
rknn.config(target_platform='rv1106', quantized_dtype='w8a8')
rknn.load_onnx('yolov8n.onnx')
rknn.build(do_quantization=True, dataset='calibration.txt')
rknn.export_rknn('yolov8n.rknn')
```

## Performance Targets

| Stage | Target Latency |
|-------|----------------|
| Camera capture | < 5ms |
| ISP processing | < 5ms |
| RGA resize | < 2ms |
| NPU inference | < 30ms |
| H.265 encode | < 10ms |
| Total | < 50ms |

## Common Issues

### Camera Not Detected
- Check `/dev/video*` devices
- Verify kernel module loaded: `lsmod | grep sc3336`
- Check I2C communication: `i2cdetect -y 3`

### ISP Errors
- Ensure IQ files present in `/etc/iqfiles/`
- Check ISP driver: `dmesg | grep rkisp`

### NPU Failures
- Verify model format with `rknn_toolkit2`
- Check memory: NPU needs contiguous memory
- Model may need re-quantization

## References

- [VISION_PIPELINE.md](../docs/rs1/VISION_PIPELINE.md) - Architecture documentation
- [RS1_ARCHITECTURE.md](../docs/rs1/RS1_ARCHITECTURE.md) - System overview
- Rockchip MPP documentation
- RKNN-Toolkit2 documentation
