# Sensor Fusion Engine

The RS-1 fusion engine combines vision detections from the NPU with radar measurements to produce accurate, tracked positions in room coordinates.

## Overview

```
VisionFrame (30Hz) ──┬──▶ Hungarian Association ──▶ Kalman Filter ──▶ WorldState
                     │
RadarFrame (10Hz) ───┘     (match by azimuth)        (predict/update)    (tracked objects)
```

### Why Fusion?

| Sensor | Strengths | Weaknesses |
|--------|-----------|------------|
| Camera | Object classification, precise azimuth | No direct range measurement |
| Radar | Accurate range/velocity, works in dark | No classification, lower angular resolution |

Fusing both sensors provides:
- Accurate 2D position (range from radar, azimuth from camera)
- Object classification with position
- Velocity estimation
- Robust tracking in varying conditions

---

## Package Structure

```
products/rs1/fusion/
├── engine.go        # Main fusion coordinator
├── hungarian.go     # Data association algorithm
├── kalman.go        # State estimation filter
└── transform.go     # Coordinate transformations
```

---

## Data Association (Hungarian Algorithm)

The Hungarian algorithm optimally matches camera detections to radar targets by minimizing total angular distance.

### Location

`products/rs1/fusion/hungarian.go`

### Algorithm

1. Build cost matrix using angular difference between vision and radar azimuths
2. Apply threshold (15 degrees max) - beyond threshold, cost is infinite
3. Solve assignment problem using Hungarian algorithm
4. Return matched pairs and unmatched indices

### Key Constants

```go
const MaxAssociationAngle = 15.0  // degrees
const NoMatchCost = 1e6           // prevents bad matches
```

### Function Signature

```go
func AssociateDetections(visionAzimuths, radarAzimuths []float64) (
    associations []Association,
    unmatchedVision []int,
    unmatchedRadar []int,
)
```

### Example

```go
// Vision detections at -10°, 5°, 45°
// Radar targets at -8°, 50°

visionAzimuths := []float64{-10, 5, 45}
radarAzimuths := []float64{-8, 50}

associations, unmatchedV, unmatchedR := AssociateDetections(visionAzimuths, radarAzimuths)
// associations: [{VisionIdx:0, RadarIdx:0, Cost:2}, {VisionIdx:2, RadarIdx:1, Cost:5}]
// unmatchedVision: [1]  (5° vision has no match)
// unmatchedRadar: []
```

---

## Kalman Filter

A 4-state Kalman filter estimates position and velocity for each tracked object.

### Location

`products/rs1/fusion/kalman.go`

### State Vector

```
x = [px, py, vx, vy]^T

px, py = position (meters)
vx, vy = velocity (m/s)
```

### Matrices

| Matrix | Purpose | Values |
|--------|---------|--------|
| **P** (initial) | State covariance | diag(1, 1, 10, 10) |
| **Q** | Process noise | diag(0.1, 0.1, 1.0, 1.0) |
| **R** | Measurement noise | diag(0.5, 0.5) |
| **H** | Measurement matrix | [[1,0,0,0], [0,1,0,0]] |

### Key Methods

```go
// Create filter at initial position
kf := NewKalmanFilter(x, y float64)

// Advance state by time delta
kf.Predict(now time.Time)

// Incorporate measurement
kf.Update(measX, measY float64)

// Get current state
x, y, vx, vy := kf.State()

// Track lifecycle
kf.MarkMissed()        // No measurement this frame
kf.ShouldDelete() bool // True if track is stale
```

### Track Lifecycle

- **Creation**: First detection creates track with Kalman filter
- **Update**: Each matched detection calls `Update()`
- **Prediction**: Every frame calls `Predict()` to advance state
- **Deletion**: After 30 missed frames (3 seconds at 10Hz)

---

## Fusion Engine

The main orchestrator that coordinates data flow.

### Location

`products/rs1/fusion/engine.go`

### Track Type

```go
type Track struct {
    ID               string
    Filter           *KalmanFilter
    ClassID          int
    CameraConfidence float64
    RadarConfidence  float64
    IsRadarOnly      bool
    IsVisionOnly     bool
    FirstSeen        time.Time
}
```

### Processing Flow

1. **Receive** vision frame or radar frame
2. **Extract** azimuths from both sensor inputs
3. **Associate** using Hungarian algorithm
4. **Update** matched tracks with Kalman filter
5. **Create** new tracks for unmatched persistent detections
6. **Predict** all track states forward
7. **Delete** stale tracks
8. **Output** via `GetObjects()`

### Key Methods

```go
// Create engine
engine := NewEngine()

// Start processing loop
engine.Start()

// Feed sensor data
engine.ProcessVisionFrame(visionFrame)
engine.ProcessRadarFrame(radarTargets)

// Get current tracks
objects := engine.GetObjects()

// Set room transform (from RoomPlan API)
engine.SetRoomTransform(transform)

// Stop engine
engine.Stop()
```

### Output Format

```go
type FusedObject struct {
    ID           string
    X            float64  // Room X (meters)
    Y            float64  // Room Y (meters)
    VelocityX    float64  // m/s
    VelocityY    float64  // m/s
    ClassID      int      // COCO class (-1 = unknown)
    Confidence   float64  // 0-1
    IsRadarOnly  bool
    IsVisionOnly bool
    LastSeen     time.Time
}
```

---

## Coordinate Transform

Converts positions from sensor coordinates to room coordinates.

### Location

`products/rs1/fusion/transform.go`

### Types

```go
// 2D position in meters
type Position struct {
    X float64 `json:"x"`
    Y float64 `json:"y"`
}

// 4x4 homogeneous transform (column-major)
type TransformMatrix struct {
    M [16]float64 `json:"m"`
}
```

### Matrix Layout

```
Column-major:
[m0  m4  m8  m12]   [Rx  Ux  Fx  Tx]
[m1  m5  m9  m13] = [Ry  Uy  Fy  Ty]
[m2  m6  m10 m14]   [Rz  Uz  Fz  Tz]
[m3  m7  m11 m15]   [0   0   0   1 ]
```

### Key Methods

```go
// Identity transform (no change)
t := NewIdentityTransform()

// Apply transform to position
roomPos := t.Apply(sensorPos)

// Validate matrix (last row must be [0,0,0,1])
valid := t.IsValid()

// Get translation component
translation := t.Translation()
```

### Example: Sensor at (2, 3) rotated 90°

```go
// 90° rotation + translation (2, 3)
transform := &TransformMatrix{
    M: [16]float64{
        0, -1, 0, 0,  // Column 0: rotated X axis
        1,  0, 0, 0,  // Column 1: rotated Y axis
        0,  0, 1, 0,  // Column 2: Z axis (unchanged)
        2,  3, 0, 1,  // Column 3: translation
    },
}

sensorPos := Position{X: 1, Y: 0}
roomPos := transform.Apply(sensorPos)
// roomPos = {X: 2, Y: 4}  (rotated + translated)
```

---

## Tuning Parameters

### Association

| Parameter | Default | Description |
|-----------|---------|-------------|
| `MaxAssociationAngle` | 15.0° | Max angular difference for match |
| `NoMatchCost` | 1e6 | Cost for impossible matches |

### Kalman Filter

| Parameter | Default | Description |
|-----------|---------|-------------|
| Initial P (pos) | 1.0 | Position uncertainty |
| Initial P (vel) | 10.0 | Velocity uncertainty |
| Q (pos) | 0.1 | Position process noise |
| Q (vel) | 1.0 | Velocity process noise |
| R | 0.5 | Measurement noise |
| Max missed | 30 | Frames before deletion |

### Adjusting for Different Scenarios

**Fast-moving objects:**
```go
// Increase velocity uncertainty and process noise
kf.P.Set(2, 2, 50.0)  // vx variance
kf.P.Set(3, 3, 50.0)  // vy variance
kf.Q.Set(2, 2, 5.0)   // vx process noise
kf.Q.Set(3, 3, 5.0)   // vy process noise
```

**High-precision sensors:**
```go
// Decrease measurement noise
kf.R.Set(0, 0, 0.1)
kf.R.Set(1, 1, 0.1)
```

---

## Testing

```bash
# Run all fusion tests
go test ./products/rs1/fusion/... -v

# Test specific components
go test ./products/rs1/fusion/... -v -run TestKalman
go test ./products/rs1/fusion/... -v -run TestAssociate
go test ./products/rs1/fusion/... -v -run TestTransform
```

---

## See Also

- [RS1_ARCHITECTURE.md](RS1_ARCHITECTURE.md) - System overview
- [VISION_PIPELINE.md](VISION_PIPELINE.md) - Vision detections input
- [RADAR_INTEGRATION.md](RADAR_INTEGRATION.md) - Radar measurements input
- [WORLDSTATE_PROTOCOL.md](WORLDSTATE_PROTOCOL.md) - Output streaming format
