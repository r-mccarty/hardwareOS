# RS-1 Sensor Fusion Development

This skill provides guidance for implementing the sensor fusion engine that combines camera and radar data for the OpticWorks RS-1 platform.

## Overview

The fusion engine:
- Associates camera detections with radar targets
- Maintains tracked object states using Kalman filtering
- Outputs WorldState at 30Hz
- Transforms sensor coordinates to room coordinates

## Key Files

| File | Purpose |
|------|---------|
| `internal/fusion/engine.go` | Main fusion loop and track management |
| `internal/fusion/hungarian.go` | Data association algorithm |
| `internal/fusion/kalman.go` | State estimation filter |
| `internal/fusion/worldstate.go` | Output types |
| `internal/fusion/transform.go` | Coordinate transformations |

## Architecture

```
VisionFrame (30Hz) ─┬─→ Hungarian Association ─→ Kalman Update ─→ WorldState
                    │    (match by azimuth)       (predict/update)
RadarFrame (10Hz) ──┘
```

## Hungarian Algorithm for Data Association

The Hungarian algorithm finds the optimal assignment that minimizes total cost.

### Cost Matrix

For N camera detections and M radar targets, build an NxM cost matrix where each cell is the angular distance:

```go
func buildCostMatrix(visionAzimuths, radarAzimuths []float64) [][]float64 {
    n := len(visionAzimuths)
    m := len(radarAzimuths)
    size := max(n, m)

    cost := make([][]float64, size)
    for i := range cost {
        cost[i] = make([]float64, size)
        for j := range cost[i] {
            if i < n && j < m {
                diff := math.Abs(visionAzimuths[i] - radarAzimuths[j])
                if diff > maxAssociationAngle {
                    cost[i][j] = noMatchCost  // 1e6 = impossible match
                } else {
                    cost[i][j] = diff
                }
            } else {
                cost[i][j] = noMatchCost
            }
        }
    }
    return cost
}
```

### Using gonum

```go
import "gonum.org/v1/gonum/optimize/hungarian"

func associate(costMatrix [][]float64) []int {
    // Returns assignment where assignment[i] = j means row i matches column j
    return hungarian.Solve(costMatrix)
}
```

### Association Thresholds

```go
const (
    maxAssociationAngle = 15.0  // degrees - maximum allowed mismatch
    noMatchCost         = 1e6   // Large cost to prevent bad matches
)
```

## Kalman Filter Implementation

### State Vector

```
x = [px, py, vx, vy]^T

px, py = position in room coordinates (meters)
vx, vy = velocity (m/s)
```

### System Matrices

**State Transition (constant velocity model)**:
```go
// F matrix for dt seconds
F := mat.NewDense(4, 4, []float64{
    1, 0, dt, 0,   // x = x + vx*dt
    0, 1, 0, dt,   // y = y + vy*dt
    0, 0, 1, 0,    // vx = vx
    0, 0, 0, 1,    // vy = vy
})
```

**Observation (position only)**:
```go
// H matrix - we only observe position
H := mat.NewDense(2, 4, []float64{
    1, 0, 0, 0,
    0, 1, 0, 0,
})
```

### Predict Step

```go
func (kf *KalmanFilter) Predict(dt float64) {
    // Build F for current dt
    F := buildStateTransition(dt)

    // x = F * x
    var newX mat.VecDense
    newX.MulVec(F, kf.x)
    kf.x = &newX

    // P = F * P * F' + Q
    var FP, newP mat.Dense
    FP.Mul(F, kf.P)
    newP.Mul(&FP, F.T())
    newP.Add(&newP, kf.Q)
    kf.P = &newP
}
```

### Update Step

```go
func (kf *KalmanFilter) Update(measX, measY float64) {
    z := mat.NewVecDense(2, []float64{measX, measY})

    // Innovation: y = z - H*x
    var Hx mat.VecDense
    Hx.MulVec(H, kf.x)
    var y mat.VecDense
    y.SubVec(z, &Hx)

    // Innovation covariance: S = H*P*H' + R
    var HP, S mat.Dense
    HP.Mul(H, kf.P)
    S.Mul(&HP, H.T())
    S.Add(&S, kf.R)

    // Kalman gain: K = P*H'*S^-1
    var Sinv, PHt, K mat.Dense
    Sinv.Inverse(&S)
    PHt.Mul(kf.P, H.T())
    K.Mul(&PHt, &Sinv)

    // Update state: x = x + K*y
    var Ky mat.VecDense
    Ky.MulVec(&K, &y)
    kf.x.AddVec(kf.x, &Ky)

    // Update covariance: P = (I - K*H)*P
    var KH, IKH, newP mat.Dense
    KH.Mul(&K, H)
    IKH.Sub(eye4, &KH)
    newP.Mul(&IKH, kf.P)
    kf.P = &newP
}
```

### Tuning Parameters

```go
// Process noise Q (how much we expect state to change)
Q := mat.NewDense(4, 4, []float64{
    0.1, 0,   0,   0,     // Position uncertainty
    0,   0.1, 0,   0,
    0,   0,   1.0, 0,     // Velocity uncertainty (higher)
    0,   0,   0,   1.0,
})

// Measurement noise R (sensor accuracy)
R := mat.NewDense(2, 2, []float64{
    0.5, 0,    // Position measurement variance
    0,   0.5,
})
```

## Track Management

### Track Lifecycle

```go
type Track struct {
    ID          int
    Filter      *KalmanFilter
    ClassID     int
    Confidence  float64
    Age         int    // Frames since creation
    Hits        int    // Successful associations
    Misses      int    // Consecutive missed associations
    LastSeen    time.Time
}

// Create new track
func createTrack(pos Position, classID int) *Track {
    return &Track{
        ID:       nextID(),
        Filter:   NewKalmanFilter(pos.X, pos.Y),
        ClassID:  classID,
        Age:      0,
        Hits:     1,
        Misses:   0,
        LastSeen: time.Now(),
    }
}

// Should track be deleted?
func (t *Track) ShouldDelete() bool {
    // Delete if too many consecutive misses
    if t.Misses > 5 {
        return true
    }
    // Delete if never confirmed (low hit rate)
    if t.Age > 10 && t.Hits < 3 {
        return true
    }
    return false
}
```

### Track-to-Detection Association

```go
func (e *FusionEngine) processVisionFrame(vf *VisionFrame) {
    radarFrame := e.radarState.GetLatest()

    // Extract azimuths
    visionAzimuths := extractAzimuths(vf.Detections)
    radarAzimuths := extractRadarAzimuths(radarFrame)

    // Associate
    associations, unmatchedVision, unmatchedRadar := AssociateDetections(
        visionAzimuths, radarAzimuths,
    )

    // Process matched pairs (fused)
    for _, assoc := range associations {
        pos := fuseMeasurement(vf.Detections[assoc.VisionIdx],
                               radarFrame.Targets[assoc.RadarIdx])
        track := e.findOrCreateTrack(pos)
        track.Filter.Update(pos.X, pos.Y)
        track.Hits++
        track.Misses = 0
    }

    // Process unmatched vision (vision-only tracks)
    for _, vIdx := range unmatchedVision {
        // Handle without range info
    }

    // Process unmatched radar (radar-only tracks)
    for _, rIdx := range unmatchedRadar {
        pos := radarToRoom(radarFrame.Targets[rIdx])
        track := e.findOrCreateTrack(pos)
        track.Filter.Update(pos.X, pos.Y)
        track.IsRadarOnly = true
    }

    // Mark unassociated tracks as missed
    e.markMissedTracks()

    // Delete stale tracks
    e.pruneStale()
}
```

## Coordinate Transformation

### Sensor to Room Transform

```go
type TransformMatrix struct {
    M [16]float64  // Column-major 4x4 matrix
}

func (t *TransformMatrix) Apply(sensor Position) Position {
    // 2D transform (Z=0)
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

### Fusing Camera Azimuth with Radar Range

```go
func fuseMeasurement(vision *VisionDetection, radar RadarTarget) Position {
    // Camera provides better azimuth (LDCH corrected)
    azimuth := float64(vision.AzimuthDeg) * math.Pi / 180.0

    // Radar provides accurate range
    rangeM := radar.Range

    // Convert polar to Cartesian (sensor frame)
    return Position{
        X: rangeM * math.Sin(azimuth),
        Y: rangeM * math.Cos(azimuth),
    }
}
```

## Performance Optimization

### Batch Matrix Operations

```go
// Pre-allocate matrices to avoid garbage collection
var (
    tempMatrix1 mat.Dense
    tempMatrix2 mat.Dense
    tempVec     mat.VecDense
)

func (kf *KalmanFilter) predictReuse() {
    // Reuse pre-allocated matrices
    tempMatrix1.Mul(F, kf.P)
    tempMatrix2.Mul(&tempMatrix1, F.T())
    kf.P.Add(&tempMatrix2, kf.Q)
}
```

### Limit Track Count

```go
const maxTracks = 20

func (e *FusionEngine) createTrack(pos Position) *Track {
    if len(e.tracks) >= maxTracks {
        // Remove oldest low-confidence track
        e.evictLowestConfidence()
    }
    // Create new track
}
```

## Testing

### Unit Tests for Kalman Filter

```go
func TestKalmanConvergence(t *testing.T) {
    kf := NewKalmanFilter(0, 0)

    // Simulate object moving at constant velocity
    truePos := Position{X: 0, Y: 0}
    velocity := Position{X: 1.0, Y: 0.5}  // m/s

    for i := 0; i < 100; i++ {
        dt := 0.033  // 30Hz
        truePos.X += velocity.X * dt
        truePos.Y += velocity.Y * dt

        // Add measurement noise
        measX := truePos.X + rand.NormFloat64()*0.1
        measY := truePos.Y + rand.NormFloat64()*0.1

        kf.Predict(dt)
        kf.Update(measX, measY)
    }

    // Check convergence
    x, y, vx, vy := kf.State()
    assert.InDelta(t, truePos.X, x, 0.2)
    assert.InDelta(t, truePos.Y, y, 0.2)
    assert.InDelta(t, velocity.X, vx, 0.3)
    assert.InDelta(t, velocity.Y, vy, 0.3)
}
```

### Integration Tests

```go
func TestFusionAssociation(t *testing.T) {
    engine := NewFusionEngine(mockRadarState)

    // Simulate camera detecting person at 30 degrees
    visionFrame := &VisionFrame{
        Detections: []*VisionDetection{
            {AzimuthDeg: 30.0, ClassId: 0, Confidence: 0.9},
        },
    }

    // Simulate radar detecting target at similar angle
    mockRadarState.Update(RadarFrame{
        Targets: []RadarTarget{
            {X: 1000, Y: 1732, Azimuth: 30.0},  // ~2m at 30 degrees
        },
    })

    engine.processVisionFrame(visionFrame)

    // Should create single fused track
    state := engine.GetWorldState()
    assert.Len(t, state.Objects, 1)
    assert.False(t, state.Objects[0].IsRadarOnly)
    assert.False(t, state.Objects[0].IsVisionOnly)
}
```

## References

- [FUSION_ENGINE.md](../docs/rs1/FUSION_ENGINE.md) - Architecture documentation
- [VISION_PIPELINE.md](../docs/rs1/VISION_PIPELINE.md) - Camera input
- [RADAR_INTEGRATION.md](../docs/rs1/RADAR_INTEGRATION.md) - Radar input
- gonum documentation: https://pkg.go.dev/gonum.org/v1/gonum
