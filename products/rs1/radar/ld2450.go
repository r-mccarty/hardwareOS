// Package radar implements radar sensor communication for the HLK-LD2450 24GHz mmWave radar.
// The LD2450 provides up to 3 simultaneous target tracking with range (0.2-6m),
// velocity measurement via Doppler, and ±60° horizontal coverage.
package radar

import (
	"encoding/binary"
	"errors"
	"io"
	"math"
	"sync"
	"time"

	"github.com/jetkvm/kvm/platform/logging"
	"go.bug.st/serial"
)

var logger = logging.GetSubsystemLogger("radar")

const (
	// LD2450 frame markers
	headerByte0 = 0xAA
	headerByte1 = 0xFF
	headerByte2 = 0x03
	headerByte3 = 0x00
	tailByte0   = 0x55
	tailByte1   = 0xCC

	// Serial port settings
	defaultBaudRate = 256000
	defaultPort     = "/dev/ttyS3"

	// Maximum targets the LD2450 can track
	maxTargets = 3

	// Target data size in bytes
	targetSize = 8
)

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
	Targets   [maxTargets]Target
	Timestamp time.Time
}

// LD2450 handles communication with the LD2450 mmWave radar sensor
type LD2450 struct {
	mu       sync.RWMutex
	port     serial.Port
	portPath string
	baudRate int
	running  bool
	stopCh   chan struct{}
	frameCh  chan Frame
	lastFrame Frame
}

// NewLD2450 creates a new LD2450 radar interface
func NewLD2450(portPath string, baudRate int) *LD2450 {
	if portPath == "" {
		portPath = defaultPort
	}
	if baudRate == 0 {
		baudRate = defaultBaudRate
	}

	return &LD2450{
		portPath: portPath,
		baudRate: baudRate,
		frameCh:  make(chan Frame, 10),
		stopCh:   make(chan struct{}),
	}
}

// Start opens the serial port and begins reading frames
func (r *LD2450) Start() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.running {
		return errors.New("radar already running")
	}

	mode := &serial.Mode{
		BaudRate: r.baudRate,
		DataBits: 8,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
	}

	port, err := serial.Open(r.portPath, mode)
	if err != nil {
		return err
	}

	r.port = port
	r.running = true
	r.stopCh = make(chan struct{})

	go r.readLoop()

	logger.Info().Str("port", r.portPath).Int("baud", r.baudRate).Msg("LD2450 radar started")
	return nil
}

// Stop stops the radar reader and closes the port
func (r *LD2450) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.running {
		return nil
	}

	r.running = false
	close(r.stopCh)

	if r.port != nil {
		r.port.Close()
		r.port = nil
	}

	logger.Info().Msg("LD2450 radar stopped")
	return nil
}

// Frames returns a channel that receives parsed radar frames
func (r *LD2450) Frames() <-chan Frame {
	return r.frameCh
}

// LastFrame returns the most recent frame
func (r *LD2450) LastFrame() Frame {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.lastFrame
}

// readLoop continuously reads and parses frames from the serial port
func (r *LD2450) readLoop() {
	buf := make([]byte, 256)

	for {
		select {
		case <-r.stopCh:
			return
		default:
		}

		n, err := r.port.Read(buf)
		if err != nil {
			if err != io.EOF {
				logger.Warn().Err(err).Msg("error reading from radar")
			}
			continue
		}

		if n > 0 {
			r.parseData(buf[:n])
		}
	}
}

// parseData parses raw bytes looking for valid LD2450 frames
// LD2450 frame format:
// Header (4 bytes): 0xAA 0xFF 0x03 0x00
// Target 1 (8 bytes): X(2) Y(2) Speed(2) Reserved(2)
// Target 2 (8 bytes): X(2) Y(2) Speed(2) Reserved(2)
// Target 3 (8 bytes): X(2) Y(2) Speed(2) Reserved(2)
// Tail (2 bytes): 0x55 0xCC
func (r *LD2450) parseData(data []byte) {
	// Look for frame header: 0xAA 0xFF 0x03 0x00
	for i := 0; i < len(data)-29; i++ {
		if data[i] == headerByte0 && data[i+1] == headerByte1 &&
			data[i+2] == headerByte2 && data[i+3] == headerByte3 {
			// Check tail: 0x55 0xCC
			if data[i+28] == tailByte0 && data[i+29] == tailByte1 {
				frame := r.parseFrame(data[i+4 : i+28])
				r.mu.Lock()
				r.lastFrame = frame
				r.mu.Unlock()

				select {
				case r.frameCh <- frame:
				default:
					// Channel full, drop frame
				}
				i += 29
			}
		}
	}
}

// parseFrame parses the target data portion of a frame
func (r *LD2450) parseFrame(data []byte) Frame {
	frame := Frame{
		Timestamp: time.Now(),
	}

	for i := 0; i < maxTargets; i++ {
		offset := i * targetSize
		target := Target{
			X:     int16(binary.LittleEndian.Uint16(data[offset : offset+2])),
			Y:     int16(binary.LittleEndian.Uint16(data[offset+2 : offset+4])),
			Speed: int16(binary.LittleEndian.Uint16(data[offset+4 : offset+6])),
			// bytes 6-7 are reserved
		}
		// Target is valid if X or Y is non-zero
		target.Valid = target.X != 0 || target.Y != 0
		if target.Valid {
			target.CalculatePolar()
		}
		frame.Targets[i] = target
	}

	return frame
}

// ValidTargets returns a slice containing only valid targets from the frame
func (f *Frame) ValidTargets() []Target {
	targets := make([]Target, 0, maxTargets)
	for _, t := range f.Targets {
		if t.Valid {
			targets = append(targets, t)
		}
	}
	return targets
}
