# Sensor Fusion Engine

The RS-1 fusion engine combines vision detections from the NPU with radar measurements to produce accurate, tracked positions in room coordinates at 30Hz.

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
- Accurate 2D/3D position (range from radar, azimuth from camera)
- Object classification with position
- Robust tracking in varying conditions

---

## Package Structure

```
internal/fusion/
├── engine.go        # Main fusion loop
├── hungarian.go     # Data association algorithm
├── kalman.go        # State estimation filter
├── worldstate.go    # Output types
└── transform.go     # Coordinate transformations
```

---

## Data Association (Hungarian Algorithm)

The Hungarian algorithm matches camera detections to radar targets by minimizing total angular distance.

### `internal/fusion/hungarian.go`

```go
package fusion

import (
    "math"

    "gonum.org/v1/gonum/optimize/hungarian"
)

const (
    maxAssociationAngle = 15.0 // degrees - max allowed mismatch
    noMatchCost         = 1e6  // Large cost to prevent bad matches
)

// Association represents a matched pair
type Association struct {
    VisionIdx int
    RadarIdx  int
    Cost      float64
}

// AssociateDetections matches vision detections to radar targets
// Returns: associations, unmatched vision indices, unmatched radar indices
func AssociateDetections(visionAzimuths []float64, radarAzimuths []float64) (
    []Association, []int, []int,
) {
    nVision := len(visionAzimuths)
    nRadar := len(radarAzimuths)

    if nVision == 0 || nRadar == 0 {
        // No matches possible
        unmatchedVision := make([]int, nVision)
        for i := range unmatchedVision {
            unmatchedVision[i] = i
        }
        unmatchedRadar := make([]int, nRadar)
        for i := range unmatchedRadar {
            unmatchedRadar[i] = i
        }
        return nil, unmatchedVision, unmatchedRadar
    }

    // Build cost matrix
    size := max(nVision, nRadar)
    costMatrix := make([][]float64, size)
    for i := range costMatrix {
        costMatrix[i] = make([]float64, size)
        for j := range costMatrix[i] {
            if i < nVision && j < nRadar {
                angleDiff := math.Abs(visionAzimuths[i] - radarAzimuths[j])
                if angleDiff > maxAssociationAngle {
                    costMatrix[i][j] = noMatchCost
                } else {
                    costMatrix[i][j] = angleDiff
                }
            } else {
                costMatrix[i][j] = noMatchCost
            }
        }
    }

    // Run Hungarian algorithm
    assignment := hungarian.Solve(costMatrix)

    // Extract results
    var associations []Association
    matchedVision := make(map[int]bool)
    matchedRadar := make(map[int]bool)

    for vIdx, rIdx := range assignment {
        if vIdx < nVision && rIdx < nRadar {
            cost := costMatrix[vIdx][rIdx]
            if cost < noMatchCost {
                associations = append(associations, Association{
                    VisionIdx: vIdx,
                    RadarIdx:  rIdx,
                    Cost:      cost,
                })
                matchedVision[vIdx] = true
                matchedRadar[rIdx] = true
            }
        }
    }

    // Find unmatched
    var unmatchedVision, unmatchedRadar []int
    for i := 0; i < nVision; i++ {
        if !matchedVision[i] {
            unmatchedVision = append(unmatchedVision, i)
        }
    }
    for i := 0; i < nRadar; i++ {
        if !matchedRadar[i] {
            unmatchedRadar = append(unmatchedRadar, i)
        }
    }

    return associations, unmatchedVision, unmatchedRadar
}
```

---

## Kalman Filter

A 2D Kalman filter estimates position and velocity for each tracked object.

### State Vector
```
x = [px, py, vx, vy]^T

px, py = position (meters)
vx, vy = velocity (m/s)
```

### `internal/fusion/kalman.go`

```go
package fusion

import (
    "time"

    "gonum.org/v1/gonum/mat"
)

// KalmanFilter tracks a single object's state
type KalmanFilter struct {
    // State: [x, y, vx, vy]
    x *mat.VecDense

    // State covariance
    P *mat.Dense

    // Process noise
    Q *mat.Dense

    // Measurement noise (position only)
    R *mat.Dense

    // Last update time
    lastUpdate time.Time

    // Track lifetime
    age int
    hits int
    misses int
}

// NewKalmanFilter creates a filter with initial position
func NewKalmanFilter(x, y float64) *KalmanFilter {
    kf := &KalmanFilter{
        x:          mat.NewVecDense(4, []float64{x, y, 0, 0}),
        P:          mat.NewDense(4, 4, nil),
        Q:          mat.NewDense(4, 4, nil),
        R:          mat.NewDense(2, 2, nil),
        lastUpdate: time.Now(),
        age:        0,
        hits:       1,
        misses:     0,
    }

    // Initial covariance (high uncertainty in velocity)
    kf.P.Set(0, 0, 1.0)   // x variance
    kf.P.Set(1, 1, 1.0)   // y variance
    kf.P.Set(2, 2, 10.0)  // vx variance
    kf.P.Set(3, 3, 10.0)  // vy variance

    // Process noise
    q := 0.1
    kf.Q.Set(0, 0, q)
    kf.Q.Set(1, 1, q)
    kf.Q.Set(2, 2, q*10)
    kf.Q.Set(3, 3, q*10)

    // Measurement noise
    kf.R.Set(0, 0, 0.5) // x measurement variance
    kf.R.Set(1, 1, 0.5) // y measurement variance

    return kf
}

// Predict advances state to current time
func (kf *KalmanFilter) Predict(now time.Time) {
    dt := now.Sub(kf.lastUpdate).Seconds()
    if dt <= 0 {
        return
    }

    // State transition matrix
    // [1 0 dt 0 ]
    // [0 1 0  dt]
    // [0 0 1  0 ]
    // [0 0 0  1 ]
    F := mat.NewDense(4, 4, []float64{
        1, 0, dt, 0,
        0, 1, 0, dt,
        0, 0, 1, 0,
        0, 0, 0, 1,
    })

    // Predict state: x = F * x
    var newX mat.VecDense
    newX.MulVec(F, kf.x)
    kf.x = &newX

    // Predict covariance: P = F * P * F' + Q
    var FP, newP mat.Dense
    FP.Mul(F, kf.P)
    newP.Mul(&FP, F.T())
    newP.Add(&newP, kf.Q)
    kf.P = &newP

    kf.lastUpdate = now
    kf.age++
}

// Update incorporates a measurement
func (kf *KalmanFilter) Update(measX, measY float64) {
    // Measurement matrix (observe position only)
    // [1 0 0 0]
    // [0 1 0 0]
    H := mat.NewDense(2, 4, []float64{
        1, 0, 0, 0,
        0, 1, 0, 0,
    })

    // Innovation: y = z - H * x
    z := mat.NewVecDense(2, []float64{measX, measY})
    var Hx mat.VecDense
    Hx.MulVec(H, kf.x)
    var y mat.VecDense
    y.SubVec(z, &Hx)

    // Innovation covariance: S = H * P * H' + R
    var HP, S mat.Dense
    HP.Mul(H, kf.P)
    S.Mul(&HP, H.T())
    S.Add(&S, kf.R)

    // Kalman gain: K = P * H' * S^-1
    var Sinv, PHt, K mat.Dense
    Sinv.Inverse(&S)
    PHt.Mul(kf.P, H.T())
    K.Mul(&PHt, &Sinv)

    // Update state: x = x + K * y
    var Ky mat.VecDense
    Ky.MulVec(&K, &y)
    kf.x.AddVec(kf.x, &Ky)

    // Update covariance: P = (I - K * H) * P
    I := mat.NewDense(4, 4, nil)
    for i := 0; i < 4; i++ {
        I.Set(i, i, 1)
    }
    var KH, IKH, newP mat.Dense
    KH.Mul(&K, H)
    IKH.Sub(I, &KH)
    newP.Mul(&IKH, kf.P)
    kf.P = &newP

    kf.hits++
    kf.misses = 0
}

// MarkMissed increments miss counter
func (kf *KalmanFilter) MarkMissed() {
    kf.misses++
}

// State returns current position and velocity
func (kf *KalmanFilter) State() (x, y, vx, vy float64) {
    return kf.x.AtVec(0), kf.x.AtVec(1), kf.x.AtVec(2), kf.x.AtVec(3)
}

// ShouldDelete returns true if track should be removed
func (kf *KalmanFilter) ShouldDelete() bool {
    return kf.misses > 5 || (kf.age > 10 && kf.hits < 3)
}
```

---

## Fusion Engine

The main orchestrator that ties everything together.

### `internal/fusion/engine.go`

```go
package fusion

import (
    "sync"
    "time"

    "github.com/jetkvm/kvm/internal/radar"
    "github.com/jetkvm/kvm/internal/native/proto"
)

// FusionEngine combines vision and radar data
type FusionEngine struct {
    radarState  *radar.State
    visionChan  chan *proto.VisionFrame

    tracks      map[int]*Track
    nextTrackID int
    tracksMu    sync.RWMutex

    transform   *TransformMatrix
    outputChan  chan WorldState

    stopChan    chan struct{}
}

// Track represents a tracked object
type Track struct {
    ID          int
    Filter      *KalmanFilter
    ClassID     int
    Confidence  float64
    IsRadarOnly bool
    IsVisionOnly bool
    LastSeen    time.Time
}

// NewFusionEngine creates a new fusion engine
func NewFusionEngine(radarState *radar.State) *FusionEngine {
    return &FusionEngine{
        radarState: radarState,
        visionChan: make(chan *proto.VisionFrame, 10),
        tracks:     make(map[int]*Track),
        nextTrackID: 1,
        transform:  NewIdentityTransform(),
        outputChan: make(chan WorldState, 10),
        stopChan:   make(chan struct{}),
    }
}

// SetRoomTransform updates the sensor-to-room transform
func (e *FusionEngine) SetRoomTransform(t *TransformMatrix) {
    e.transform = t
}

// VisionInput returns the channel for vision frames
func (e *FusionEngine) VisionInput() chan<- *proto.VisionFrame {
    return e.visionChan
}

// Output returns the channel for world state
func (e *FusionEngine) Output() <-chan WorldState {
    return e.outputChan
}

// Start begins the fusion loop
func (e *FusionEngine) Start() {
    go e.fusionLoop()
}

// Stop halts the fusion engine
func (e *FusionEngine) Stop() {
    close(e.stopChan)
}

func (e *FusionEngine) fusionLoop() {
    ticker := time.NewTicker(33 * time.Millisecond) // 30Hz
    defer ticker.Stop()

    for {
        select {
        case <-e.stopChan:
            return

        case visionFrame := <-e.visionChan:
            e.processVisionFrame(visionFrame)

        case <-ticker.C:
            e.predictAndOutput()
        }
    }
}

func (e *FusionEngine) processVisionFrame(vf *proto.VisionFrame) {
    now := time.Now()

    // Get radar data
    radarFrame := e.radarState.GetLatest()

    // Extract azimuths for association
    visionAzimuths := make([]float64, len(vf.Detections))
    for i, d := range vf.Detections {
        visionAzimuths[i] = float64(d.AzimuthDeg)
    }

    var radarAzimuths []float64
    if radarFrame != nil {
        radarAzimuths = make([]float64, len(radarFrame.Targets))
        for i, t := range radarFrame.Targets {
            radarAzimuths[i] = t.Azimuth
        }
    }

    // Associate detections
    associations, unmatchedVision, unmatchedRadar := AssociateDetections(
        visionAzimuths, radarAzimuths,
    )

    e.tracksMu.Lock()
    defer e.tracksMu.Unlock()

    // Process matched pairs (fused detections)
    for _, assoc := range associations {
        vd := vf.Detections[assoc.VisionIdx]
        rt := radarFrame.Targets[assoc.RadarIdx]

        // Convert radar to room coordinates
        sensorPos := Position{X: float64(rt.X) / 1000, Y: float64(rt.Y) / 1000}
        roomPos := e.transform.Apply(sensorPos)

        // Find or create track
        track := e.findNearestTrack(roomPos, 1.0)
        if track == nil {
            track = e.createTrack(roomPos, int(vd.ClassId), float64(vd.Confidence))
        }

        track.Filter.Update(roomPos.X, roomPos.Y)
        track.ClassID = int(vd.ClassId)
        track.Confidence = float64(vd.Confidence)
        track.IsRadarOnly = false
        track.IsVisionOnly = false
        track.LastSeen = now
    }

    // Process vision-only detections
    for _, vIdx := range unmatchedVision {
        vd := vf.Detections[vIdx]
        // Without radar, we can only estimate direction, not range
        // Use a default range or skip
        // For now, mark as vision-only with estimated position
    }

    // Process radar-only detections
    if radarFrame != nil {
        for _, rIdx := range unmatchedRadar {
            rt := radarFrame.Targets[rIdx]
            sensorPos := Position{X: float64(rt.X) / 1000, Y: float64(rt.Y) / 1000}
            roomPos := e.transform.Apply(sensorPos)

            track := e.findNearestTrack(roomPos, 0.5)
            if track == nil {
                track = e.createTrack(roomPos, -1, 0) // Unknown class
            }

            track.Filter.Update(roomPos.X, roomPos.Y)
            track.IsRadarOnly = true
            track.IsVisionOnly = false
            track.LastSeen = now
        }
    }

    // Mark missed tracks
    for _, track := range e.tracks {
        if time.Since(track.LastSeen) > 100*time.Millisecond {
            track.Filter.MarkMissed()
        }
    }

    // Delete stale tracks
    for id, track := range e.tracks {
        if track.Filter.ShouldDelete() {
            delete(e.tracks, id)
        }
    }
}

func (e *FusionEngine) predictAndOutput() {
    now := time.Now()

    e.tracksMu.RLock()
    defer e.tracksMu.RUnlock()

    objects := make([]TrackedObject, 0, len(e.tracks))
    for _, track := range e.tracks {
        track.Filter.Predict(now)
        x, y, vx, vy := track.Filter.State()

        objects = append(objects, TrackedObject{
            TrackID:      track.ID,
            X:            x,
            Y:            y,
            VX:           vx,
            VY:           vy,
            ClassID:      track.ClassID,
            Confidence:   track.Confidence,
            IsRadarOnly:  track.IsRadarOnly,
            IsVisionOnly: track.IsVisionOnly,
        })
    }

    worldState := WorldState{
        Objects:   objects,
        Timestamp: now,
        FrameNum:  0, // Set by caller
    }

    select {
    case e.outputChan <- worldState:
    default:
        // Drop if channel full
    }
}

func (e *FusionEngine) findNearestTrack(pos Position, maxDist float64) *Track {
    var nearest *Track
    minDist := maxDist

    for _, track := range e.tracks {
        x, y, _, _ := track.Filter.State()
        dist := pos.DistanceTo(Position{X: x, Y: y})
        if dist < minDist {
            minDist = dist
            nearest = track
        }
    }

    return nearest
}

func (e *FusionEngine) createTrack(pos Position, classID int, confidence float64) *Track {
    track := &Track{
        ID:         e.nextTrackID,
        Filter:     NewKalmanFilter(pos.X, pos.Y),
        ClassID:    classID,
        Confidence: confidence,
        LastSeen:   time.Now(),
    }
    e.nextTrackID++
    e.tracks[track.ID] = track
    return track
}

// GetWorldState returns current state (for RPC queries)
func (e *FusionEngine) GetWorldState() WorldState {
    e.tracksMu.RLock()
    defer e.tracksMu.RUnlock()

    objects := make([]TrackedObject, 0, len(e.tracks))
    for _, track := range e.tracks {
        x, y, vx, vy := track.Filter.State()
        objects = append(objects, TrackedObject{
            TrackID:      track.ID,
            X:            x,
            Y:            y,
            VX:           vx,
            VY:           vy,
            ClassID:      track.ClassID,
            Confidence:   track.Confidence,
            IsRadarOnly:  track.IsRadarOnly,
            IsVisionOnly: track.IsVisionOnly,
        })
    }

    return WorldState{
        Objects:   objects,
        Timestamp: time.Now(),
    }
}
```

---

## WorldState Types

### `internal/fusion/worldstate.go`

```go
package fusion

import "time"

// TrackedObject represents a fused, tracked entity
type TrackedObject struct {
    TrackID      int     // Unique track identifier
    X            float64 // Room X coordinate (meters)
    Y            float64 // Room Y coordinate (meters)
    VX           float64 // X velocity (m/s)
    VY           float64 // Y velocity (m/s)
    ClassID      int     // Object class (-1 = unknown)
    Confidence   float64 // Detection confidence (0-1)
    IsRadarOnly  bool    // Only seen by radar
    IsVisionOnly bool    // Only seen by camera
}

// WorldState represents the complete tracked environment
type WorldState struct {
    Objects   []TrackedObject
    Timestamp time.Time
    FrameNum  uint64
}

// Class IDs
const (
    ClassUnknown = -1
    ClassPerson  = 0
    ClassVehicle = 1
    ClassAnimal  = 2
)
```

---

## Coordinate Transform

### `internal/fusion/transform.go`

```go
package fusion

import "math"

// Position represents a 2D point
type Position struct {
    X, Y float64
}

// DistanceTo calculates Euclidean distance
func (p Position) DistanceTo(other Position) float64 {
    dx := p.X - other.X
    dy := p.Y - other.Y
    return math.Sqrt(dx*dx + dy*dy)
}

// TransformMatrix is a 4x4 homogeneous transformation matrix
type TransformMatrix struct {
    M [16]float64 // Column-major order
}

// NewIdentityTransform creates an identity matrix
func NewIdentityTransform() *TransformMatrix {
    return &TransformMatrix{
        M: [16]float64{
            1, 0, 0, 0,
            0, 1, 0, 0,
            0, 0, 1, 0,
            0, 0, 0, 1,
        },
    }
}

// Apply transforms a position from sensor to room coordinates
func (t *TransformMatrix) Apply(sensor Position) Position {
    // Assuming 2D transform (Z=0)
    // [x']   [m0 m4 m8  m12] [x]
    // [y'] = [m1 m5 m9  m13] [y]
    // [z']   [m2 m6 m10 m14] [0]
    // [1 ]   [m3 m7 m11 m15] [1]

    return Position{
        X: t.M[0]*sensor.X + t.M[4]*sensor.Y + t.M[12],
        Y: t.M[1]*sensor.X + t.M[5]*sensor.Y + t.M[13],
    }
}
```

---

## See Also

- [RS1_ARCHITECTURE.md](RS1_ARCHITECTURE.md) - System overview
- [VISION_PIPELINE.md](VISION_PIPELINE.md) - Vision detections input
- [RADAR_INTEGRATION.md](RADAR_INTEGRATION.md) - Radar measurements input
- [WORLDSTATE_PROTOCOL.md](WORLDSTATE_PROTOCOL.md) - Output format
