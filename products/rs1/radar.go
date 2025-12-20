package rs1

import (
	"github.com/jetkvm/kvm/products/rs1/radar"
)

// RadarManager wraps the LD2450 radar and converts to RS-1 coordinate system
type RadarManager struct {
	ld2450  *radar.LD2450
	config  *RadarConfig
	running bool
}

// NewRadarManager creates a new radar manager
func NewRadarManager(config *RadarConfig) (*RadarManager, error) {
	if config == nil {
		config = &RadarConfig{
			Enabled:      true,
			MaxRange:     6.0,
			Sensitivity:  0.5,
			UpdateRateHz: 10,
		}
	}

	rm := &RadarManager{
		ld2450: radar.NewLD2450("", 0), // Use defaults
		config: config,
	}

	return rm, nil
}

// Start begins radar data acquisition
func (rm *RadarManager) Start() error {
	if !rm.config.Enabled {
		logger.Info().Msg("radar disabled in config")
		return nil
	}

	if err := rm.ld2450.Start(); err != nil {
		return err
	}

	rm.running = true

	// Start processing frames
	go rm.processFrames()

	return nil
}

// Stop halts radar data acquisition
func (rm *RadarManager) Stop() {
	if rm.running {
		rm.ld2450.Stop()
		rm.running = false
	}
}

// processFrames converts LD2450 frames to RS-1 coordinate system
func (rm *RadarManager) processFrames() {
	for frame := range rm.ld2450.Frames() {
		// Convert targets to RS-1 world coordinates
		for i := range frame.Targets {
			if frame.Targets[i].Valid {
				// Convert mm to meters
				x := float64(frame.Targets[i].X) / 1000.0
				y := float64(frame.Targets[i].Y) / 1000.0

				// Filter by max range
				dist := x*x + y*y
				if dist > rm.config.MaxRange*rm.config.MaxRange {
					continue
				}

				// TODO: Publish to fusion engine
				_ = x
				_ = y
			}
		}
	}
}

// IsRunning returns whether radar is active
func (rm *RadarManager) IsRunning() bool {
	return rm.running
}
