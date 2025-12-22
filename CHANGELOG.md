# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

#### RS-1 Vision Pipeline Spike (2024-12-22)

Initial spike implementation of the camera data ingestion pipeline for the OpticWorks RS-1 product. This provides the foundational architecture for the camera → ISP → NPU vision pipeline.

**Native C Components** (`targets/rv1106/native/cgo/`):

| File | Description |
|------|-------------|
| `camera.c/h` | SC3336 MIPI sensor initialization via V4L2 (2304x1296 @ 30fps, NV12 output) |
| `isp.c/h` | Rockchip ISP wrapper with LDCH lens distortion correction and 3A algorithms |
| `npu.c/h` | RKNN YOLOv8 inference wrapper with NMS post-processing and angle calculation |
| `vision_pipeline.c/h` | Pipeline orchestrator that forks frames to encoder and NPU paths |

**Key Features**:
- Camera capture with V4L2 MMAP buffer management
- LDCH (Lens Distortion Correction) with full cylindrical projection (level 255) for accurate azimuth/elevation calculation
- YOLOv8 inference with configurable confidence threshold
- Parallel video encoding and NPU inference via frame forking
- Thread-safe callback-based frame delivery

**Protocol Buffer Additions** (`targets/rv1106/native/proto/native.proto`):
- `VisionDetection`: Bounding box, class ID, confidence, azimuth/elevation angles
- `VisionFrame`: Array of detections with timing metadata
- `CameraState`, `IspState`, `NpuState`: Component status messages
- `VisionPipelineState`: Aggregate pipeline status
- Vision pipeline RPCs: `VisionPipelineStart`, `VisionPipelineStop`, `VisionPipelineGetState`
- Camera control RPCs: `CameraSetExposure`, `CameraSetOrientation`
- ISP control RPCs: `IspSetLdchEnabled`, `IspSetLdchLevel`
- NPU control RPCs: `NpuLoadModel`, `NpuSetConfidenceThreshold`

**Go Bindings** (`targets/rv1106/native/`):
- `vision.go`: High-level `VisionPipeline` type with configuration and async callbacks
- `vision_cgo_linux.go`: CGO wrappers for C callbacks with channel-based delivery

**Note**: This is a spike implementation with stub functions for SDK dependencies (RKAIQ, RKNN, RGA, MPP). Production implementation requires Rockchip SDK libraries.

**Data Flow**:
```
SC3336 MIPI → V4L2 → ISP (LDCH) → [Fork]
                                    ├─→ RGA Resize (1080p) → H.265 Encoder → VideoFrame
                                    └─→ RGA Resize (640x640) → NPU YOLOv8 → VisionFrame
```

---

## Previous Changes

### Cloud API Service (2024-12-21)
- Added Cloudflare Workers cloud API service
- Added GET /devices endpoint for frontend device list
- Added implementation discussion to CLOUD_API_v2 spec

### Infrastructure
- Added Infisical integration for secrets management
- Updated SECRETS.md with OpticWorks paths
