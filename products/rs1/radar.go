package rs1

import (
	"fmt"
	"sync"

	"github.com/jetkvm/kvm/products/rs1/fusion"
	"github.com/jetkvm/kvm/products/rs1/radar"
)

// RadarManager wraps the LD2450 radar and converts to RS-1 coordinate system
type RadarManager struct {
	mu      sync.RWMutex
	ld2450  *radar.LD2450
	config  *RadarConfig
	fusion  *fusion.Engine
	running bool
	stopCh  chan struct{}

	// Stats
	framesProcessed uint64
	targetsDetected uint64
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
		stopCh: make(chan struct{}),
	}

	return rm, nil
}

// SetFusionEngine connects the radar manager to a fusion engine
func (rm *RadarManager) SetFusionEngine(engine *fusion.Engine) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.fusion = engine
}

// Start begins radar data acquisition
func (rm *RadarManager) Start() error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if !rm.config.Enabled {
		logger.Info().Msg("radar disabled in config")
		return nil
	}

	if rm.running {
		return fmt.Errorf("radar already running")
	}

	if err := rm.ld2450.Start(); err != nil {
		return err
	}

	rm.running = true
	rm.stopCh = make(chan struct{})

	// Start processing frames
	go rm.processFrames()

	logger.Info().
		Float64("max_range", rm.config.MaxRange).
		Msg("radar manager started")

	return nil
}

// Stop halts radar data acquisition
func (rm *RadarManager) Stop() {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if !rm.running {
		return
	}

	rm.running = false
	close(rm.stopCh)
	rm.ld2450.Stop()

	logger.Info().
		Uint64("frames_processed", rm.framesProcessed).
		Uint64("targets_detected", rm.targetsDetected).
		Msg("radar manager stopped")
}

// processFrames converts LD2450 frames to fusion detections
func (rm *RadarManager) processFrames() {
	for {
		select {
		case <-rm.stopCh:
			return
		case frame, ok := <-rm.ld2450.Frames():
			if !ok {
				return
			}
			rm.handleFrame(frame)
		}
	}
}

// handleFrame processes a single radar frame and submits to fusion
func (rm *RadarManager) handleFrame(frame radar.Frame) {
	rm.mu.Lock()
	rm.framesProcessed++
	fusionEngine := rm.fusion
	maxRange := rm.config.MaxRange
	rm.mu.Unlock()

	// Get valid targets that are within range
	validTargets := frame.ValidTargets()
	detections := make([]fusion.Detection, 0, len(validTargets))

	for i, target := range validTargets {
		// Filter by max range (polar coordinates already calculated)
		if target.Range > maxRange {
			continue
		}

		// Convert to fusion Detection
		// Note: Radar X/Y are already in meters after polar calculation,
		// but we use the original mm values divided by 1000 for consistency
		det := fusion.Detection{
			ID:         fmt.Sprintf("radar_%d_%d", frame.Timestamp.UnixNano(), i),
			X:          float64(target.X) / 1000.0, // Convert mm to meters
			Y:          float64(target.Y) / 1000.0, // Convert mm to meters
			Confidence: rm.calculateConfidence(target),
			Source:     fusion.SourceRadar,
			Timestamp:  frame.Timestamp,
		}
		detections = append(detections, det)
	}

	rm.mu.Lock()
	rm.targetsDetected += uint64(len(detections))
	rm.mu.Unlock()

	// Submit to fusion engine if connected
	if fusionEngine != nil && len(detections) > 0 {
		fusionEngine.SubmitRadarDetections(detections)

		logger.Debug().
			Int("targets", len(detections)).
			Msg("submitted radar detections to fusion")
	}
}

// calculateConfidence computes detection confidence based on radar signal
// The LD2450 doesn't provide signal strength, so we use range and speed heuristics
func (rm *RadarManager) calculateConfidence(target radar.Target) float64 {
	// Base confidence
	confidence := 0.8

	// Reduce confidence at longer ranges (radar accuracy degrades)
	if target.Range > 4.0 {
		confidence -= 0.1
	} else if target.Range > 5.0 {
		confidence -= 0.2
	}

	// Moving targets have higher confidence (Doppler radar works better)
	if target.Speed != 0 {
		confidence += 0.1
	}

	// Clamp to 0-1 range
	if confidence > 1.0 {
		confidence = 1.0
	} else if confidence < 0.1 {
		confidence = 0.1
	}

	return confidence
}

// IsRunning returns whether radar is active
func (rm *RadarManager) IsRunning() bool {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.running
}

// GetStats returns current processing statistics
func (rm *RadarManager) GetStats() (framesProcessed, targetsDetected uint64) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.framesProcessed, rm.targetsDetected
}

// GetLastFrame returns the most recent radar frame
func (rm *RadarManager) GetLastFrame() radar.Frame {
	return rm.ld2450.LastFrame()
}
