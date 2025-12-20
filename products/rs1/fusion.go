package rs1

import (
	"github.com/jetkvm/kvm/products/rs1/fusion"
)

// FusionEngine wraps the sensor fusion engine
type FusionEngine struct {
	engine *fusion.Engine
}

// NewFusionEngine creates a new fusion engine wrapper
func NewFusionEngine() *FusionEngine {
	return &FusionEngine{
		engine: fusion.NewEngine(),
	}
}

// Start begins fusion processing
func (f *FusionEngine) Start() {
	f.engine.Start()
	logger.Info().Msg("fusion engine started")
}

// Stop halts fusion processing
func (f *FusionEngine) Stop() {
	f.engine.Stop()
}

// GetEngine returns the underlying fusion engine
func (f *FusionEngine) GetEngine() *fusion.Engine {
	return f.engine
}
