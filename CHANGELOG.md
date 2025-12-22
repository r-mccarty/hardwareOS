# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

#### Documentation and process updates (2025-12-22)

- Added `docs/README.md` index and `docs/rs1/DEVELOPMENT.md` for current RS-1 workflows.
- Added PRD template/index and PR checklist updates for proposal tracking.
- Added changelog CI enforcement and archived pre-RS-1 docs under `docs/legacy/` with stubs.
- Updated `docs/CLOUD_API.md` to match the Workers v1 implementation; shelved v2 spec in `docs/legacy/`.

#### Apple RoomPlan CapturedRoom Integration (2024-12-22)

Integration with Apple's RoomPlan framework to render 3D room geometry from iPhone/iPad LiDAR scans.

**New Types** (`ui/src/types/`):

| File | Description |
|------|-------------|
| `capturedRoom.ts` | TypeScript types for Apple CapturedRoom JSON format |

**Parser Utilities** (`ui/src/utils/`):

| File | Description |
|------|-------------|
| `capturedRoomParser.ts` | Converts CapturedRoom JSON → ExtendedRoomPlanState |
| `mockCapturedRoom.ts` | Mock 6m × 5m office for demo mode testing |

**3D Components** (`ui/src/components/visualization/Room/`):

| File | Description |
|------|-------------|
| `Wall3D.tsx` | Semi-transparent wall rendering with transform support |
| `Door3D.tsx` | Orange door frame visualization |
| `Window3D.tsx` | Blue glass windows with cross dividers |
| `Furniture3D.tsx` | Category-colored furniture bounding boxes |
| `CapturedRoomRenderer.tsx` | Orchestrates all CapturedRoom geometry |

**Key Features**:
- Full CapturedRoom JSON parsing (surfaces, objects, floors)
- 4x4 column-major transform matrix support
- Category-based furniture colors (table=indigo, chair=violet, sofa=pink, storage=amber)
- Furniture collision avoidance for demo occupants
- Backward compatible with simple RoomPlan format
- StatsOverlay shows "RoomPlan" badge with surface/object counts

**Data Flow**:
```
iPhone RoomPlan Scan → CapturedRoom JSON → POST /api/setup/roomplan
                                                    ↓
                              Frontend Parser → 3D Visualization
                                                    ↓
                                      Tracked Occupants (real-time)
```

**Demo Mode**:
- Mock office: 4 walls, 1 door, 2 windows, 7 furniture items
- Simulated occupants walk around avoiding furniture bounds
- Access at `/demo` route

---

#### RS-1 Complete Sensor Fusion Implementation (2024-12-22)

Complete implementation of sensor fusion, WorldState streaming, and RoomPlan API for the OpticWorks RS-1 product.

**Fusion Algorithms** (`products/rs1/fusion/`):

| File | Description |
|------|-------------|
| `kalman.go` | 4-state Kalman filter [px, py, vx, vy] using gonum/mat |
| `hungarian.go` | Hungarian algorithm for optimal detection-to-track association |
| `transform.go` | Position and TransformMatrix types for coordinate transforms |
| `engine.go` | Fusion coordinator with track management |

**Key Features**:
- Kalman filter with configurable process/measurement noise
- Hungarian algorithm with 15-degree angular threshold
- 4x4 homogeneous transformation matrices (column-major)
- Automatic track creation/deletion lifecycle
- Thread-safe track access with RWMutex

**WorldState Streaming** (`webrtc_worldstate.go`):
- 30Hz JSON streaming over WebRTC DataChannel
- Channel label: `"worldstate"`
- Message format includes: timestamp, frame number, occupant count, tracked objects

**RoomPlan API** (`roomplan.go`):
- `POST /api/setup/roomplan`: Upload room configuration with sensor pose
- `GET /api/setup/roomplan`: Retrieve current configuration
- Transform matrix validation (last row must be [0,0,0,1])
- Persistent storage in config.json

**Integration**:
- RS-1 product initialization in `main.go`
- WorldState getter/setter accessor functions
- Graceful shutdown handling

**Data Flow**:
```
Radar (10Hz) ──┬──► Hungarian ──► Kalman ──► WorldState ──► WebRTC (30Hz)
Vision (30Hz) ─┘    Association     Filter     Manager      DataChannel
```

---

#### RS-1 3D Visualization MVP (2024-12-22)

Tesla FSD-style 3D visualization for real-time occupancy tracking using React Three Fiber.

**Components** (`ui/src/components/visualization/`):

| File | Description |
|------|-------------|
| `Scene.tsx` | Main R3F Canvas with dark theme, fog, and lighting |
| `Room/RoomFloor.tsx` | Blue grid floor scaled to room dimensions |
| `Room/RoomBoundary.tsx` | Room outline with obstacle rendering |
| `Occupants/HumanSilhouette.tsx` | Color-coded human meshes (cyan=fused, blue=radar, green=vision) |
| `Occupants/OccupantManager.tsx` | Renders tracked objects from WorldState store |
| `Sensor/SensorIndicator.tsx` | Device position with FOV cone visualization |
| `Camera/CameraController.tsx` | OrbitControls for scene navigation |
| `UI/StatsOverlay.tsx` | Occupancy count and connection status overlay |

**Hooks** (`ui/src/hooks/`):

| File | Description |
|------|-------------|
| `useWorldState.ts` | WebRTC DataChannel subscription for real-time updates |
| `useRoomPlan.ts` | RoomPlan API fetching with sensible defaults |
| `useWorldStateDemo.ts` | Demo mode with simulated walking occupants |

**Stores** (Zustand, in `ui/src/hooks/stores.ts`):
- `useWorldStateStore`: Tracked objects, connection state, update timestamps
- `useRoomPlanStore`: Room dimensions, sensor pose, loading/error state

**Routes**:
- `/demo`: Standalone demo with simulated occupants (no device required)

**Dependencies**: `@react-three/fiber`, `@react-three/drei`, `three`, `maath`

**Visual Style**:
- Dark background (#0a0a0f) with atmospheric fog
- Blue grid lines (#2563eb) matching Tesla FSD aesthetic
- Human silhouettes with capsule body, sphere head, and heading arrow
- Color-coded by sensor source (fused/radar-only/vision-only)

---

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
