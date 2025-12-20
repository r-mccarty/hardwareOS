# LD2450 Radar Integration

The RS-1 uses the HiLink LD2450 24GHz mmWave radar for range and velocity measurement. This document covers the UART protocol, parsing implementation, and integration with the fusion engine.

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

### `internal/radar/types.go`

```go
package radar

import (
    "math"
    "time"
)

// RadarTarget represents a single detected target
type RadarTarget struct {
    X         int16     // X position in mm
    Y         int16     // Y position in mm
    Speed     int16     // Radial velocity in cm/s
    Azimuth   float64   // Calculated angle in degrees
    Range     float64   // Calculated distance in meters
    Timestamp time.Time // Detection timestamp
}

// RadarFrame represents a complete radar scan
type RadarFrame struct {
    Targets   []RadarTarget
    Timestamp time.Time
    FrameNum  uint32
}

// CalculatePolar computes azimuth and range from Cartesian coordinates
func (t *RadarTarget) CalculatePolar() {
    xMeters := float64(t.X) / 1000.0
    yMeters := float64(t.Y) / 1000.0

    t.Range = math.Sqrt(xMeters*xMeters + yMeters*yMeters)
    t.Azimuth = math.Atan2(xMeters, yMeters) * 180.0 / math.Pi
}
```

### `internal/radar/ld2450.go`

```go
package radar

import (
    "encoding/binary"
    "fmt"
    "io"
    "time"

    "go.bug.st/serial"
)

const (
    headerByte0   = 0xAA
    headerByte1   = 0xFF
    headerByte2   = 0x03
    headerByte3   = 0x00
    tailByte0     = 0x55
    tailByte1     = 0xCC
    maxTargets    = 3
    targetSize    = 8
    baudRate      = 256000
)

// LD2450 represents the radar sensor interface
type LD2450 struct {
    port      serial.Port
    frameNum  uint32
    onFrame   func(RadarFrame)
    stopChan  chan struct{}
}

// NewLD2450 creates a new radar interface
func NewLD2450(portPath string) (*LD2450, error) {
    mode := &serial.Mode{
        BaudRate: baudRate,
        DataBits: 8,
        Parity:   serial.NoParity,
        StopBits: serial.OneStopBit,
    }

    port, err := serial.Open(portPath, mode)
    if err != nil {
        return nil, fmt.Errorf("failed to open serial port: %w", err)
    }

    return &LD2450{
        port:     port,
        stopChan: make(chan struct{}),
    }, nil
}

// SetFrameCallback registers a callback for new frames
func (r *LD2450) SetFrameCallback(cb func(RadarFrame)) {
    r.onFrame = cb
}

// Start begins reading radar data
func (r *LD2450) Start() {
    go r.readLoop()
}

// Stop stops the radar reader
func (r *LD2450) Stop() {
    close(r.stopChan)
    r.port.Close()
}

func (r *LD2450) readLoop() {
    buf := make([]byte, 256)
    ringBuf := make([]byte, 0, 512)

    for {
        select {
        case <-r.stopChan:
            return
        default:
        }

        n, err := r.port.Read(buf)
        if err != nil {
            if err != io.EOF {
                logger.Error().Err(err).Msg("radar read error")
            }
            continue
        }

        ringBuf = append(ringBuf, buf[:n]...)

        // Process complete frames
        for {
            frame, consumed := r.parseFrame(ringBuf)
            if consumed == 0 {
                break
            }
            ringBuf = ringBuf[consumed:]

            if frame != nil && r.onFrame != nil {
                r.onFrame(*frame)
            }
        }

        // Prevent buffer overflow
        if len(ringBuf) > 256 {
            ringBuf = ringBuf[len(ringBuf)-256:]
        }
    }
}

func (r *LD2450) parseFrame(data []byte) (*RadarFrame, int) {
    // Find header
    headerIdx := -1
    for i := 0; i <= len(data)-4; i++ {
        if data[i] == headerByte0 && data[i+1] == headerByte1 &&
           data[i+2] == headerByte2 && data[i+3] == headerByte3 {
            headerIdx = i
            break
        }
    }

    if headerIdx < 0 {
        return nil, 0
    }

    // Skip bytes before header
    if headerIdx > 0 {
        return nil, headerIdx
    }

    // Check minimum frame length
    if len(data) < 10 { // header(4) + length(2) + count(1) + checksum(1) + tail(2)
        return nil, 0
    }

    // Parse length
    payloadLen := int(binary.LittleEndian.Uint16(data[4:6]))
    frameLen := 4 + 2 + payloadLen + 1 + 2 // header + length + payload + checksum + tail

    if len(data) < frameLen {
        return nil, 0 // Incomplete frame
    }

    // Verify tail
    if data[frameLen-2] != tailByte0 || data[frameLen-1] != tailByte1 {
        return nil, 1 // Invalid frame, skip one byte
    }

    // Verify checksum
    checksum := byte(0)
    for i := 6; i < frameLen-3; i++ {
        checksum ^= data[i]
    }
    if checksum != data[frameLen-3] {
        return nil, 1 // Checksum mismatch
    }

    // Parse targets
    targetCount := int(data[6])
    if targetCount > maxTargets {
        targetCount = maxTargets
    }

    frame := &RadarFrame{
        Targets:   make([]RadarTarget, 0, targetCount),
        Timestamp: time.Now(),
        FrameNum:  r.frameNum,
    }
    r.frameNum++

    for i := 0; i < targetCount; i++ {
        offset := 7 + i*targetSize
        if offset+targetSize > frameLen-3 {
            break
        }

        target := RadarTarget{
            X:         int16(binary.LittleEndian.Uint16(data[offset:])),
            Y:         int16(binary.LittleEndian.Uint16(data[offset+2:])),
            Speed:     int16(binary.LittleEndian.Uint16(data[offset+4:])),
            Timestamp: frame.Timestamp,
        }
        target.CalculatePolar()
        frame.Targets = append(frame.Targets, target)
    }

    return frame, frameLen
}
```

### `internal/radar/state.go`

Provides thread-safe access to the latest radar frame for the fusion engine.

```go
package radar

import (
    "sync"
    "time"
)

const staleThreshold = 200 * time.Millisecond

// State maintains the latest radar frame with thread-safe access
type State struct {
    mu          sync.RWMutex
    latestFrame *RadarFrame
    updatedAt   time.Time
}

// NewState creates a new radar state holder
func NewState() *State {
    return &State{}
}

// Update stores a new radar frame
func (s *State) Update(frame RadarFrame) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.latestFrame = &frame
    s.updatedAt = time.Now()
}

// GetLatest returns the latest frame if not stale
// Returns nil if no frame or frame is too old
func (s *State) GetLatest() *RadarFrame {
    s.mu.RLock()
    defer s.mu.RUnlock()

    if s.latestFrame == nil {
        return nil
    }

    if time.Since(s.updatedAt) > staleThreshold {
        return nil // Data too old
    }

    // Return a copy to prevent race conditions
    frameCopy := *s.latestFrame
    targetsCopy := make([]RadarTarget, len(s.latestFrame.Targets))
    copy(targetsCopy, s.latestFrame.Targets)
    frameCopy.Targets = targetsCopy

    return &frameCopy
}

// IsStale returns true if radar data is outdated
func (s *State) IsStale() bool {
    s.mu.RLock()
    defer s.mu.RUnlock()
    return s.latestFrame == nil || time.Since(s.updatedAt) > staleThreshold
}
```

---

## Integration with Main Application

### Initialization in `main.go`

```go
var radarState *radar.State

func initRadar() error {
    radarState = radar.NewState()

    ld2450, err := radar.NewLD2450(config.RadarConfig.UartPath)
    if err != nil {
        return fmt.Errorf("failed to initialize radar: %w", err)
    }

    ld2450.SetFrameCallback(func(frame radar.RadarFrame) {
        radarState.Update(frame)

        // Optional: emit metrics
        radarFramesTotal.Inc()
        for _, t := range frame.Targets {
            radarTargetDistance.Observe(t.Range)
        }
    })

    ld2450.Start()
    logger.Info().Str("port", config.RadarConfig.UartPath).Msg("radar initialized")
    return nil
}
```

### Configuration

```go
type RadarConfig struct {
    UartPath   string  `json:"uart_path"`    // e.g., "/dev/ttyS0"
    BaudRate   int     `json:"baud_rate"`    // 256000
    MaxRangeM  float64 `json:"max_range_m"`  // 6.0
    Enabled    bool    `json:"enabled"`      // true
}
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
