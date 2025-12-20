# Vision Pipeline Architecture

The RS-1 vision pipeline captures frames from the SC3336 MIPI sensor, applies lens distortion correction, runs YOLOv8 inference on the NPU, and streams H.265 video via WebRTC.

## Hardware Components

### SC3336 MIPI Sensor
- **Resolution**: 2304 x 1296 (3MP)
- **Frame Rate**: 30 FPS
- **Interface**: MIPI CSI-2, 2 lanes
- **Pixel Format**: RAW Bayer (RGGB)
- **FOV**: ~120 degrees (with wide-angle lens)

### Rockchip ISP
- **Input**: RAW Bayer from sensor
- **Output**: YUV420 (NV12)
- **Features**:
  - 3A (Auto Exposure, Auto White Balance, Auto Focus)
  - LDCH (Lens Distortion Correction for Horizontal lines)
  - HDR (High Dynamic Range)
  - Noise Reduction

### Rockchip NPU
- **Performance**: 0.5 TOPS
- **Framework**: RKNN (Rockchip Neural Network)
- **Model Format**: `.rknn` (converted from ONNX/PyTorch)
- **Input**: RGB 640x640
- **Latency**: ~30ms per frame

### Video Encoder (MPP)
- **Codec**: H.265 (HEVC) - more efficient than H.264
- **Bitrate**: VBR, 512-2000 kbps
- **GOP**: 60 frames
- **Profile**: Main

---

## Pipeline Architecture

```
┌──────────────┐
│   SC3336     │
│ MIPI Sensor  │
└──────┬───────┘
       │ RAW Bayer (2304x1296)
       ▼
┌──────────────┐
│  Rockchip    │
│     ISP      │
│  (+ LDCH)    │
└──────┬───────┘
       │ NV12 (2304x1296)
       ▼
┌──────────────┐
│    Fork      │
└──┬───────┬───┘
   │       │
   ▼       ▼
┌──────┐ ┌──────────┐
│ RGA  │ │   RGA    │
│Resize│ │  Resize  │
│1080p │ │ 640x640  │
└──┬───┘ └────┬─────┘
   │          │
   ▼          ▼
┌──────┐ ┌──────────┐
│H.265 │ │   NPU    │
│Encode│ │ YOLOv8   │
└──┬───┘ └────┬─────┘
   │          │
   ▼          ▼
WebRTC    VisionFrame
Video      (gRPC)
Track
```

---

## Implementation Files

### `internal/native/cgo/camera.c`

Initializes the SC3336 MIPI sensor via V4L2.

```c
#include <linux/videodev2.h>
#include "camera.h"

#define CAMERA_DEV "/dev/video0"
#define SENSOR_WIDTH 2304
#define SENSOR_HEIGHT 1296

static int camera_fd = -1;

int camera_init(void) {
    camera_fd = open(CAMERA_DEV, O_RDWR);
    if (camera_fd < 0) {
        LOG_ERROR("Failed to open camera device");
        return -1;
    }

    // Set MIPI format
    struct v4l2_format fmt = {0};
    fmt.type = V4L2_BUF_TYPE_VIDEO_CAPTURE_MPLANE;
    fmt.fmt.pix_mp.width = SENSOR_WIDTH;
    fmt.fmt.pix_mp.height = SENSOR_HEIGHT;
    fmt.fmt.pix_mp.pixelformat = V4L2_PIX_FMT_SRGGB10;
    fmt.fmt.pix_mp.num_planes = 1;

    if (ioctl(camera_fd, VIDIOC_S_FMT, &fmt) < 0) {
        LOG_ERROR("Failed to set camera format");
        return -1;
    }

    // Configure frame rate
    struct v4l2_streamparm parm = {0};
    parm.type = V4L2_BUF_TYPE_VIDEO_CAPTURE_MPLANE;
    parm.parm.capture.timeperframe.numerator = 1;
    parm.parm.capture.timeperframe.denominator = 30;
    ioctl(camera_fd, VIDIOC_S_PARM, &parm);

    return 0;
}
```

### `internal/native/cgo/isp.c`

Configures the Rockchip ISP with LDCH lens correction.

```c
#include <rkaiq/rk_aiq_user_api2_imgproc.h>
#include "isp.h"

static rk_aiq_sys_ctx_t* aiq_ctx = NULL;

int isp_init(void) {
    // Initialize AIQ (Auto Image Quality) context
    rk_aiq_static_info_t static_info;
    rk_aiq_uapi2_sysctl_enumStaticMetasByPhyId(0, &static_info);

    rk_aiq_working_mode_t mode = RK_AIQ_WORKING_MODE_NORMAL;
    aiq_ctx = rk_aiq_uapi2_sysctl_init(
        static_info.sensor_info.sensor_name,
        "/etc/iqfiles",
        NULL, NULL
    );

    if (!aiq_ctx) {
        LOG_ERROR("Failed to initialize AIQ");
        return -1;
    }

    // Enable LDCH (Lens Distortion Correction)
    rk_aiq_uapi2_setLdchEn(aiq_ctx, true);

    // Set correction level (0-255, higher = more correction)
    // 255 = full cylindrical projection for linear pixel-to-angle math
    rk_aiq_uapi2_setLdchCorrectLevel(aiq_ctx, 255);

    // Start 3A processing
    rk_aiq_uapi2_sysctl_prepare(aiq_ctx, 0, 0, mode);
    rk_aiq_uapi2_sysctl_start(aiq_ctx);

    return 0;
}

int isp_set_ldch_mesh(const char* mesh_path) {
    // Load custom LDCH mesh for specific lens calibration
    // Mesh file contains per-pixel displacement vectors
    return rk_aiq_uapi2_setLdchMeshPath(aiq_ctx, mesh_path);
}
```

### `internal/native/cgo/npu.c`

Runs YOLOv8 inference on the Rockchip NPU.

```c
#include <rknn_api.h>
#include "npu.h"

static rknn_context ctx;
static rknn_input_output_num io_num;
static rknn_tensor_attr* output_attrs;

#define NPU_INPUT_SIZE 640
#define MAX_DETECTIONS 100

int npu_init(const char* model_path) {
    // Load RKNN model
    FILE* fp = fopen(model_path, "rb");
    if (!fp) {
        LOG_ERROR("Failed to open model: %s", model_path);
        return -1;
    }

    fseek(fp, 0, SEEK_END);
    size_t model_size = ftell(fp);
    fseek(fp, 0, SEEK_SET);

    void* model_data = malloc(model_size);
    fread(model_data, 1, model_size, fp);
    fclose(fp);

    int ret = rknn_init(&ctx, model_data, model_size, 0, NULL);
    free(model_data);

    if (ret < 0) {
        LOG_ERROR("rknn_init failed: %d", ret);
        return -1;
    }

    // Query input/output info
    rknn_query(ctx, RKNN_QUERY_IN_OUT_NUM, &io_num, sizeof(io_num));

    output_attrs = malloc(io_num.n_output * sizeof(rknn_tensor_attr));
    for (int i = 0; i < io_num.n_output; i++) {
        output_attrs[i].index = i;
        rknn_query(ctx, RKNN_QUERY_OUTPUT_ATTR, &output_attrs[i], sizeof(rknn_tensor_attr));
    }

    return 0;
}

int npu_inference(uint8_t* rgb_data, int width, int height,
                  VisionDetection* detections, int* count) {
    // Set input
    rknn_input inputs[1];
    memset(inputs, 0, sizeof(inputs));
    inputs[0].index = 0;
    inputs[0].type = RKNN_TENSOR_UINT8;
    inputs[0].size = NPU_INPUT_SIZE * NPU_INPUT_SIZE * 3;
    inputs[0].fmt = RKNN_TENSOR_NHWC;
    inputs[0].buf = rgb_data;

    int ret = rknn_inputs_set(ctx, 1, inputs);
    if (ret < 0) return ret;

    // Run inference
    ret = rknn_run(ctx, NULL);
    if (ret < 0) return ret;

    // Get outputs
    rknn_output outputs[io_num.n_output];
    memset(outputs, 0, sizeof(outputs));
    for (int i = 0; i < io_num.n_output; i++) {
        outputs[i].want_float = 1;
    }
    rknn_outputs_get(ctx, io_num.n_output, outputs, NULL);

    // Parse YOLO output (post-processing)
    *count = parse_yolo_output(outputs, detections, MAX_DETECTIONS);

    rknn_outputs_release(ctx, io_num.n_output, outputs);
    return 0;
}

void npu_shutdown(void) {
    rknn_destroy(ctx);
    free(output_attrs);
}
```

---

## Protobuf Messages

### `internal/native/proto/native.proto` additions

```protobuf
// Vision detection from NPU
message VisionDetection {
  int32 track_id = 1;       // Tracking ID (assigned by NPU or fusion)
  float bbox_x = 2;         // Bounding box center X (normalized 0-1)
  float bbox_y = 3;         // Bounding box center Y (normalized 0-1)
  float bbox_width = 4;     // Bounding box width (normalized 0-1)
  float bbox_height = 5;    // Bounding box height (normalized 0-1)
  int32 class_id = 6;       // Object class (0=person, 1=vehicle, etc.)
  float confidence = 7;     // Detection confidence (0-1)
  float azimuth_deg = 8;    // Horizontal angle from camera center
  float elevation_deg = 9;  // Vertical angle from camera center
}

// Frame of detections from NPU
message VisionFrame {
  repeated VisionDetection detections = 1;
  int64 timestamp_ns = 2;   // Frame timestamp (nanoseconds since epoch)
  int32 frame_number = 3;   // Monotonic frame counter
  int32 inference_ms = 4;   // NPU inference time
}

// Add to Event.oneof
message Event {
  string type = 1;
  oneof data {
    VideoState video_state = 2;
    string indev_event = 3;
    string rpc_event = 4;
    VideoFrame video_frame = 5;
    VisionFrame vision_frame = 6;  // NEW
  }
}
```

---

## Azimuth Calculation

With LDCH cylindrical projection enabled, pixel X position maps linearly to azimuth angle:

```c
float calculate_azimuth(float bbox_center_x, int image_width, float fov_degrees) {
    // bbox_center_x is normalized (0-1)
    // With cylindrical projection, linear mapping is valid
    float center_offset = bbox_center_x - 0.5f;  // -0.5 to +0.5
    return center_offset * fov_degrees;          // degrees from center
}
```

Example: FOV = 120 degrees
- Object at X=0.0 → azimuth = -60 degrees (left edge)
- Object at X=0.5 → azimuth = 0 degrees (center)
- Object at X=1.0 → azimuth = +60 degrees (right edge)

---

## Performance Targets

| Stage | Target | Notes |
|-------|--------|-------|
| Camera Capture | 30 FPS | V4L2 MMAP buffers |
| ISP Processing | < 5ms | Hardware accelerated |
| RGA Resize | < 2ms | Hardware accelerated |
| NPU Inference | < 30ms | YOLOv8n model |
| H.265 Encode | < 10ms | Hardware accelerated |
| Total Latency | < 50ms | End-to-end |

---

## Model Management

### Model Location
```
/userdata/models/
├── yolov8n.rknn       # Default model (nano)
├── yolov8s.rknn       # Optional (small)
└── custom.rknn        # User-provided
```

### OTA Model Updates
The OTA system can push new `.rknn` files to the device:
```go
type ModelUpdate struct {
    Version  string `json:"version"`
    URL      string `json:"url"`
    Checksum string `json:"checksum"`
    Path     string `json:"path"` // /userdata/models/yolov8n.rknn
}
```

### Model Conversion
Convert PyTorch/ONNX to RKNN using RKNN-Toolkit2:
```python
from rknn.api import RKNN

rknn = RKNN()
rknn.config(target_platform='rv1106')
rknn.load_onnx('yolov8n.onnx')
rknn.build(do_quantization=True, dataset='calibration.txt')
rknn.export_rknn('yolov8n.rknn')
```

---

## See Also

- [RS1_ARCHITECTURE.md](RS1_ARCHITECTURE.md) - System overview
- [FUSION_ENGINE.md](FUSION_ENGINE.md) - How vision detections are fused with radar
