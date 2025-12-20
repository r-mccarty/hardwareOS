package config

// ProductConfig is the interface that product-specific configurations must implement.
// This allows the platform to work with different products (RS-1, etc.) without
// knowing their specific configuration details.
type ProductConfig interface {
	// GetProductName returns the human-readable product name
	GetProductName() string

	// GetProductCode returns the short product identifier
	GetProductCode() string

	// Validate checks that the configuration is valid
	Validate() error
}

// ProductInitializer is the interface for product initialization.
// Products implement this to register their specific handlers and services.
type ProductInitializer interface {
	// Init initializes the product
	Init() error

	// Start starts product-specific services
	Start() error

	// Stop stops product-specific services
	Stop()

	// RegisterHandlers registers product-specific RPC handlers
	RegisterHandlers(registry HandlerRegistry) error
}

// HandlerRegistry allows products to register their RPC handlers
type HandlerRegistry interface {
	// RegisterHandler registers an RPC handler
	RegisterHandler(name string, handler interface{}) error
}
