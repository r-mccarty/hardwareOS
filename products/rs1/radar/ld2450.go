// Package radar implements radar sensor communication.
package radar

import (
	"encoding/binary"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/jetkvm/kvm/platform/logging"
	"go.bug.st/serial"
)

var logger = logging.GetSubsystemLogger("radar")

const (
	// LD2450 frame markers
	frameHeader = 0xAA
	frameTail   = 0x55CC

	// Serial port settings
	defaultBaudRate = 256000
	defaultPort     = "/dev/ttyS3"

	// Maximum targets the LD2450 can track
	maxTargets = 3
)

// Target represents a single detected target from the LD2450
type Target struct {
	X        int16   // X position in mm (relative to sensor)
	Y        int16   // Y position in mm (relative to sensor)
	Speed    int16   // Speed in mm/s (positive = approaching)
	Distance uint16  // Distance from sensor in mm
	Valid    bool    // Whether this target slot contains valid data
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
// Target 1 (8 bytes): X(2) Y(2) Speed(2) Resolution(2)
// Target 2 (8 bytes): X(2) Y(2) Speed(2) Resolution(2)
// Target 3 (8 bytes): X(2) Y(2) Speed(2) Resolution(2)
// Tail (2 bytes): 0x55 0xCC
func (r *LD2450) parseData(data []byte) {
	// Simplified parsing - look for frame header
	for i := 0; i < len(data)-29; i++ {
		if data[i] == 0xAA && data[i+1] == 0xFF && data[i+2] == 0x03 && data[i+3] == 0x00 {
			// Check tail
			if data[i+28] == 0x55 && data[i+29] == 0xCC {
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
		offset := i * 8
		target := Target{
			X:        int16(binary.LittleEndian.Uint16(data[offset : offset+2])),
			Y:        int16(binary.LittleEndian.Uint16(data[offset+2 : offset+4])),
			Speed:    int16(binary.LittleEndian.Uint16(data[offset+4 : offset+6])),
			Distance: binary.LittleEndian.Uint16(data[offset+6 : offset+8]),
		}
		// Target is valid if X or Y is non-zero
		target.Valid = target.X != 0 || target.Y != 0
		frame.Targets[i] = target
	}

	return frame
}
