# RS-1 Radar Protocol Development

This skill provides guidance for implementing the LD2450 radar UART protocol parser for the OpticWorks RS-1 platform.

## Overview

The HLK-LD2450 is a 24GHz mmWave radar that provides:
- Up to 3 simultaneous target tracking
- Range measurement (0.2m - 6m)
- Velocity measurement (Doppler)
- ±60 degree horizontal coverage

## Key Files

| File | Purpose |
|------|---------|
| `internal/radar/ld2450.go` | UART protocol parser |
| `internal/radar/types.go` | RadarTarget, RadarFrame structures |
| `internal/radar/state.go` | Thread-safe state latch |
| `serial.go` | Serial port initialization (modified from JetKVM ATX/DC) |

## LD2450 Protocol Specification

### Frame Structure

```
┌──────────┬──────────┬──────────┬──────────────────────┬──────────┬──────────┐
│  Header  │  Length  │  Count   │     Target Data      │ Checksum │  Tail    │
│ 4 bytes  │ 2 bytes  │ 1 byte   │   8 bytes × N        │ 1 byte   │ 2 bytes  │
└──────────┴──────────┴──────────┴──────────────────────┴──────────┴──────────┘

Header:   0xAA 0xFF 0x03 0x00
Length:   Little-endian uint16 (payload length)
Count:    Number of targets (0-3)
Checksum: XOR of all payload bytes
Tail:     0x55 0xCC
```

### Target Data (8 bytes per target)

```
Offset  Size   Type    Description
0       2      int16   X position (mm, positive = right)
2       2      int16   Y position (mm, forward from sensor)
4       2      int16   Speed (cm/s, positive = approaching)
6       2      -       Reserved
```

### Coordinate System

```
        +Y (forward)
          │
          │     Target
          │    ●
          │   /
          │  / range
          │ /
    ──────┼────── +X (right)
          │
          │
       [LD2450]
```

## Implementation Guidelines

### Serial Port Setup

```go
import "go.bug.st/serial"

func openRadarPort(path string) (serial.Port, error) {
    mode := &serial.Mode{
        BaudRate: 256000,  // LD2450 default
        DataBits: 8,
        Parity:   serial.NoParity,
        StopBits: serial.OneStopBit,
    }
    return serial.Open(path, mode)
}
```

### Frame Parser State Machine

```go
type parserState int

const (
    stateWaitHeader parserState = iota
    stateReadLength
    stateReadPayload
    stateVerifyChecksum
    stateWaitTail
)

func parseFrame(data []byte) (*RadarFrame, int) {
    // Find header: 0xAA 0xFF 0x03 0x00
    headerIdx := findHeader(data)
    if headerIdx < 0 {
        return nil, 0
    }

    // Skip bytes before header
    if headerIdx > 0 {
        return nil, headerIdx
    }

    // Need at least header + length + count + checksum + tail
    if len(data) < 10 {
        return nil, 0  // Wait for more data
    }

    // Parse length
    payloadLen := binary.LittleEndian.Uint16(data[4:6])
    frameLen := 4 + 2 + int(payloadLen) + 1 + 2

    if len(data) < frameLen {
        return nil, 0  // Incomplete frame
    }

    // Verify tail
    if data[frameLen-2] != 0x55 || data[frameLen-1] != 0xCC {
        return nil, 1  // Bad frame, skip one byte
    }

    // Verify checksum
    checksum := byte(0)
    for i := 6; i < frameLen-3; i++ {
        checksum ^= data[i]
    }
    if checksum != data[frameLen-3] {
        return nil, 1  // Checksum mismatch
    }

    // Parse targets
    frame := parseTargets(data, payloadLen)
    return frame, frameLen
}
```

### Target Parsing

```go
func parseTargets(data []byte, payloadLen uint16) *RadarFrame {
    targetCount := int(data[6])
    if targetCount > 3 {
        targetCount = 3
    }

    frame := &RadarFrame{
        Targets:   make([]RadarTarget, 0, targetCount),
        Timestamp: time.Now(),
    }

    for i := 0; i < targetCount; i++ {
        offset := 7 + i*8
        target := RadarTarget{
            X:     int16(binary.LittleEndian.Uint16(data[offset:])),
            Y:     int16(binary.LittleEndian.Uint16(data[offset+2:])),
            Speed: int16(binary.LittleEndian.Uint16(data[offset+4:])),
        }

        // Calculate polar coordinates
        target.CalculatePolar()
        frame.Targets = append(frame.Targets, target)
    }

    return frame
}

func (t *RadarTarget) CalculatePolar() {
    xMeters := float64(t.X) / 1000.0
    yMeters := float64(t.Y) / 1000.0

    t.Range = math.Sqrt(xMeters*xMeters + yMeters*yMeters)
    t.Azimuth = math.Atan2(xMeters, yMeters) * 180.0 / math.Pi
}
```

### Thread-Safe State Latch

```go
type State struct {
    mu          sync.RWMutex
    latestFrame *RadarFrame
    updatedAt   time.Time
}

const staleThreshold = 200 * time.Millisecond

func (s *State) Update(frame RadarFrame) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.latestFrame = &frame
    s.updatedAt = time.Now()
}

func (s *State) GetLatest() *RadarFrame {
    s.mu.RLock()
    defer s.mu.RUnlock()

    if s.latestFrame == nil || time.Since(s.updatedAt) > staleThreshold {
        return nil  // No data or stale
    }

    // Return copy to prevent race conditions
    frameCopy := *s.latestFrame
    targetsCopy := make([]RadarTarget, len(s.latestFrame.Targets))
    copy(targetsCopy, s.latestFrame.Targets)
    frameCopy.Targets = targetsCopy

    return &frameCopy
}
```

## Configuration Commands

The LD2450 supports configuration via UART commands:

### Query Firmware Version

```
TX: 0xFD 0xFC 0xFB 0xFA 0x02 0x00 0xA0 0x00 0x04 0x03 0x02 0x01
RX: [version string in response]
```

### Set Baud Rate

```
TX: 0xFD 0xFC 0xFB 0xFA 0x04 0x00 0xA1 0x00 [baud_index] 0x00 0x04 0x03 0x02 0x01

baud_index: 0x01=9600, 0x02=19200, 0x03=38400, 0x04=57600,
            0x05=115200, 0x06=230400, 0x07=256000, 0x08=460800
```

### Enable/Disable Tracking

```
TX: 0xFD 0xFC 0xFB 0xFA 0x04 0x00 0xFF 0x00 [enable] 0x00 0x04 0x03 0x02 0x01

enable: 0x01=enable, 0x00=disable
```

## Testing

### Verify UART Communication

```bash
# On device
stty -F /dev/ttyS0 256000 raw -echo
cat /dev/ttyS0 | xxd

# Expected output pattern:
# aa ff 03 00 [length] [count] [data...] [checksum] 55 cc
```

### Unit Tests

```go
func TestParseFrame(t *testing.T) {
    // Valid frame with 1 target at (1000mm, 2000mm), speed 50 cm/s
    data := []byte{
        0xAA, 0xFF, 0x03, 0x00,  // Header
        0x09, 0x00,              // Length = 9
        0x01,                    // 1 target
        0xE8, 0x03,              // X = 1000 (little-endian)
        0xD0, 0x07,              // Y = 2000
        0x32, 0x00,              // Speed = 50
        0x00, 0x00,              // Reserved
        0x00,                    // Checksum (calculate)
        0x55, 0xCC,              // Tail
    }

    // Calculate checksum
    checksum := byte(0)
    for i := 6; i < len(data)-3; i++ {
        checksum ^= data[i]
    }
    data[len(data)-3] = checksum

    frame, consumed := parseFrame(data)
    assert.Equal(t, len(data), consumed)
    assert.NotNil(t, frame)
    assert.Len(t, frame.Targets, 1)
    assert.Equal(t, int16(1000), frame.Targets[0].X)
    assert.Equal(t, int16(2000), frame.Targets[0].Y)
}
```

## Common Issues

### No Data Received
- Check wiring (TX/RX may be swapped)
- Verify 5V power supply (radar needs ~100mA)
- Check baud rate: default is 256000

### Corrupt Frames
- Add capacitor (100µF) near radar VCC
- Reduce cable length
- Shield cables from EMI

### Intermittent Detection
- Radar is Doppler-based: stationary objects may not be detected
- Minimum range is 0.2m
- Very slow movement (<0.1 m/s) may be filtered

## Prometheus Metrics

```go
var (
    radarFramesTotal = promauto.NewCounter(prometheus.CounterOpts{
        Name: "rs1_radar_frames_total",
        Help: "Total radar frames received",
    })
    radarTargetsGauge = promauto.NewGauge(prometheus.GaugeOpts{
        Name: "rs1_radar_targets_current",
        Help: "Current number of radar targets",
    })
    radarParseErrors = promauto.NewCounter(prometheus.CounterOpts{
        Name: "rs1_radar_parse_errors_total",
        Help: "Total radar frame parse errors",
    })
)
```

## References

- [RADAR_INTEGRATION.md](../docs/rs1/RADAR_INTEGRATION.md) - Architecture documentation
- [FUSION_ENGINE.md](../docs/rs1/FUSION_ENGINE.md) - How radar data is fused
- HLK-LD2450 datasheet (search for official documentation)
