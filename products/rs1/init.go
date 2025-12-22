package rs1

import (
	"github.com/jetkvm/kvm/platform/logging"
)

var logger = logging.GetSubsystemLogger("rs1")

// RS1Product represents the RS-1 product instance
type RS1Product struct {
	config *RS1Config
	radar  *RadarManager
	fusion *FusionEngine
	world  *WorldState
}

// Init initializes the RS-1 product
func Init() (*RS1Product, error) {
	logger.Info().Msg("initializing RS-1 product")

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
		logger.Info().Msg("radar connected to fusion engine")
	}

	// Initialize world state
	product.world = NewWorldState()

	logger.Info().Msg("RS-1 product initialized")
	return product, nil
}

// Start starts the RS-1 product services
func (p *RS1Product) Start() error {
	logger.Info().Msg("starting RS-1 services")

	// Start radar if available
	if p.radar != nil {
		if err := p.radar.Start(); err != nil {
			logger.Warn().Err(err).Msg("failed to start radar")
		}
	}

	// Start fusion engine
	p.fusion.Start()

	// Start world state updates
	p.world.Start()

	return nil
}

// Stop stops the RS-1 product services
func (p *RS1Product) Stop() {
	logger.Info().Msg("stopping RS-1 services")

	if p.world != nil {
		p.world.Stop()
	}
	if p.fusion != nil {
		p.fusion.Stop()
	}
	if p.radar != nil {
		p.radar.Stop()
	}
}

// GetConfig returns the current RS-1 configuration
func (p *RS1Product) GetConfig() *RS1Config {
	return p.config
}

// GetWorldState returns the current world state
func (p *RS1Product) GetWorldState() *WorldState {
	return p.world
}
