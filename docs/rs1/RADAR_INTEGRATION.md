# LD2450 Radar Integration

The RS-1 uses the HiLink LD2450 24GHz mmWave radar for range and velocity measurement. This document covers the UART protocol, parsing implementation, and integration with the fusion engine.

## Implementation Status

| Component | Status | Location |
|-----------|--------|----------|
| LD2450 UART Parser | ✅ Implemented | `products/rs1/radar/ld2450.go` |
| Polar Coordinate Calculation | ✅ Implemented | `Target.CalculatePolar()` |
| Fusion Engine Integration | ✅ Implemented | `products/rs1/radar.go` |
| Unit Tests | ✅ Implemented | `products/rs1/radar/ld2450_test.go` |
| Prometheus Metrics | ⏳ Planned | - |
| Configuration Commands | ⏳ Planned | - |

### Recent Changes

**2024-12-22: Radar Integration Spike**
- Enhanced LD2450 parser with polar coordinate calculation (Range/Azimuth from X/Y)
- Connected RadarManager to FusionEngine via `SetFusionEngine()`
- RadarManager converts radar `Target` to fusion `Detection` objects
- Added confidence calculation based on range and target movement
- Added processing statistics (frames processed, targets detected)
- Added comprehensive unit tests for polar calculations and frame parsing

## Hardware Specifications

### HLK-LD2450
- **Frequency**: 24GHz FMCW
- **Detection Range**: 0.2m - 6m
- **Angle Coverage**: ±60 degrees horizontal
- **Max Targets**: 3 simultaneous
- **Update Rate**: ~10Hz
- **Interface**: UART 256000 baud, 8N1

### Connection
| LD2450 Pin | RV1106 Pin | Description |
|------------|------------|-------------|
| VCC | 5V | Power supply |
| GND | GND | Ground |
| TX | UART0_RX | Radar data output |
| RX | UART0_TX | Configuration input |

Default UART device: `/dev/ttyS0`

---

## Protocol Specification

### Frame Structure

Each frame contains up to 3 target reports.

```
┌──────────┬──────────┬──────────┬──────────────────────┬──────────┬──────────┐
│  Header  │  Length  │  Target  │     Target Data      │ Checksum │  Tail    │
│ (4 bytes)│ (2 bytes)│   Count  │    (N * 8 bytes)     │ (1 byte) │ (2 bytes)│
└──────────┴──────────┴──────────┴──────────────────────┴──────────┴──────────┘
```

### Field Details

| Field | Bytes | Value | Description |
|-------|-------|-------|-------------|
| Header | 4 | `0xAA 0xFF 0x03 0x00` | Frame start marker |
| Length | 2 | Little-endian | Payload length (excluding header/tail) |
| Target Count | 1 | 0-3 | Number of valid targets |
| Target Data | 8 * N | See below | Per-target measurements |
| Checksum | 1 | XOR | XOR of all payload bytes |
| Tail | 2 | `0x55 0xCC` | Frame end marker |

### Target Data Structure (8 bytes per target)

```
┌───────────┬───────────┬───────────┬───────────┐
│   X (mm)  │   Y (mm)  │ Speed(cm/s)│ Reserved │
│  (int16)  │  (int16)  │  (int16)  │  (2 bytes)│
└───────────┴───────────┴───────────┴───────────┘
```

| Field | Type | Range | Description |
|-------|------|-------|-------------|
| X | int16 | ±6000 | X position in mm (positive = right) |
| Y | int16 | 0-6000 | Y position in mm (distance from sensor) |
| Speed | int16 | ±32767 | Radial velocity in cm/s (positive = approaching) |

### Coordinate System

```
        +Y (forward)
          │
          │
          │
    ──────┼────── +X (right)
          │
          │
       [LD2450]
```

---

## Go Implementation

> **Note**: The actual implementation is in `products/rs1/radar/` rather than `internal/radar/`.
> See source files for the authoritative implementation.

### `products/rs1/radar/ld2450.go`

Core types and UART parser:

```go
package radar

// Target represents a single detected target from the LD2450
type Target struct {
    X        int16   // X position in mm (relative to sensor, positive = right)
    Y        int16   // Y position in mm (forward from sensor)
    Speed    int16   // Speed in cm/s (positive = approaching)
    Distance uint16  // Distance from sensor in mm (calculated)
    Valid    bool    // Whether this target slot contains valid data

    // Polar coordinates (calculated from X/Y)
    Range   float64 // Distance in meters
    Azimuth float64 // Angle in degrees (0 = forward, positive = right)
}

// CalculatePolar computes Range and Azimuth from Cartesian X/Y coordinates
func (t *Target) CalculatePolar() {
    xMeters := float64(t.X) / 1000.0
    yMeters := float64(t.Y) / 1000.0

    t.Range = math.Sqrt(xMeters*xMeters + yMeters*yMeters)
    t.Azimuth = math.Atan2(xMeters, yMeters) * 180.0 / math.Pi
    t.Distance = uint16(t.Range * 1000) // Store in mm for compatibility
}

// Frame represents a complete LD2450 data frame
type Frame struct {
    Targets   [3]Target  // Fixed array of 3 targets (LD2450 max)
    Timestamp time.Time
}

// ValidTargets returns a slice containing only valid targets from the frame
func (f *Frame) ValidTargets() []Target {
    targets := make([]Target, 0, 3)
    for _, t := range f.Targets {
        if t.Valid {
            targets = append(targets, t)
        }
    }
    return targets
}

// LD2450 handles communication with the LD2450 mmWave radar sensor
type LD2450 struct {
    // ... (see source for full implementation)
}

// Frames returns a channel that receives parsed radar frames
func (r *LD2450) Frames() <-chan Frame

// LastFrame returns the most recent frame
func (r *LD2450) LastFrame() Frame
```

### `products/rs1/radar.go`

RadarManager connects the LD2450 to the fusion engine:

```go
package rs1

// RadarManager wraps the LD2450 radar and converts to RS-1 coordinate system
type RadarManager struct {
    ld2450  *radar.LD2450
    config  *RadarConfig
    fusion  *fusion.Engine
    // ... stats tracking
}

// SetFusionEngine connects the radar manager to a fusion engine
func (rm *RadarManager) SetFusionEngine(engine *fusion.Engine)

// Start begins radar data acquisition
func (rm *RadarManager) Start() error

// handleFrame processes a single radar frame and submits to fusion
func (rm *RadarManager) handleFrame(frame radar.Frame) {
    validTargets := frame.ValidTargets()
    detections := make([]fusion.Detection, 0, len(validTargets))

    for i, target := range validTargets {
        if target.Range > rm.config.MaxRange {
            continue
        }

        det := fusion.Detection{
            ID:         fmt.Sprintf("radar_%d_%d", frame.Timestamp.UnixNano(), i),
            X:          float64(target.X) / 1000.0, // mm to meters
            Y:          float64(target.Y) / 1000.0,
            Confidence: rm.calculateConfidence(target),
            Source:     fusion.SourceRadar,
            Timestamp:  frame.Timestamp,
        }
        detections = append(detections, det)
    }

    if rm.fusion != nil && len(detections) > 0 {
        rm.fusion.SubmitRadarDetections(detections)
    }
}

// calculateConfidence uses range and movement heuristics
// (LD2450 doesn't provide signal strength)
func (rm *RadarManager) calculateConfidence(target radar.Target) float64 {
    confidence := 0.8
    if target.Range > 4.0 { confidence -= 0.1 }
    if target.Speed != 0  { confidence += 0.1 }  // Doppler works better
    return clamp(confidence, 0.1, 1.0)
}
```

---

## Integration with Main Application

### Initialization in `products/rs1/init.go`

The RS-1 product initializes components in dependency order:

```go
// Init initializes the RS-1 product
func Init() (*RS1Product, error) {
    product := &RS1Product{
        config: DefaultConfig(),
    }

    // Initialize fusion engine first (other components depend on it)
    product.fusion = NewFusionEngine()

    // Initialize radar and connect to fusion
    radar, err := NewRadarManager(product.config.RadarConfig)
    if err != nil {
        logger.Warn().Err(err).Msg("failed to initialize radar")
    } else {
        // Wire radar to fusion engine
        radar.SetFusionEngine(product.fusion.GetEngine())
        product.radar = radar
    }

    // Initialize world state
    product.world = NewWorldState()

    return product, nil
}
```

### Configuration

```go
// RadarConfig defines radar parameters (products/rs1/config.go)
type RadarConfig struct {
    Enabled      bool    `json:"enabled"`        // Enable radar sensor
    MaxRange     float64 `json:"max_range"`      // Maximum detection range (meters)
    Sensitivity  float64 `json:"sensitivity"`    // Detection sensitivity 0-1
    UpdateRateHz int     `json:"update_rate_hz"` // Target update rate
}

// Default: Enabled=true, MaxRange=6.0, Sensitivity=0.5, UpdateRateHz=10
```

### Data Flow

```
LD2450 UART (/dev/ttyS3)
    │
    ▼
LD2450.readLoop()
    │ parseData() → parseFrame() → CalculatePolar()
    ▼
Frame channel (buffered, 10 frames)
    │
    ▼
RadarManager.processFrames()
    │ handleFrame() → calculateConfidence()
    ▼
fusion.Engine.SubmitRadarDetections()
    │
    ▼
FusionEngine.processRadarDetections()
    │ findNearestObject() → updateObjectWithRadar()
    ▼
WorldState (occupants updated)
```

---

## Troubleshooting

### No Data Received
1. Check wiring (TX/RX may be swapped)
2. Verify baud rate: `stty -F /dev/ttyS0 256000`
3. Check permissions: `chmod 666 /dev/ttyS0`
4. Test with: `cat /dev/ttyS0 | xxd`

### Corrupt Frames
1. Check power supply (5V, sufficient current)
2. Reduce cable length
3. Add capacitor near radar VCC

### Missing Targets
1. Targets below 0.2m range are filtered
2. Stationary targets may not be detected (Doppler radar)
3. Check max range configuration

---

## See Also

- [RS1_ARCHITECTURE.md](RS1_ARCHITECTURE.md) - System overview
- [FUSION_ENGINE.md](FUSION_ENGINE.md) - How radar data is fused with vision
