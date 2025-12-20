// Package config provides platform-level configuration abstractions.
package config

// BrandConfig contains product branding and default service endpoints.
// This allows different products to customize their identity while
// sharing the same platform codebase.
type BrandConfig struct {
	// Product identity
	ProductName string // Display name (e.g., "HardwareOS", "OpticWorks RS-1")
	ProductCode string // Short code (e.g., "hardwareos", "rs1")

	// Default API endpoints
	DefaultAPIURL string // Default cloud API (e.g., "https://api.optic.works")
	DefaultAppURL string // Default cloud app (e.g., "https://app.optic.works")

	// TLS/Certificate settings
	TLSDomain      string // Default domain for self-signed certs (e.g., "opticworks.local")
	TLSCAName      string // CA name for self-signed certs
	TLSOrganization string // Organization name for certs
	TLSOU          string // Organizational unit for certs

	// Network identity
	HostnamePrefix string // Prefix for auto-generated hostname (e.g., "opticworks")

	// Storage paths
	DataPath    string // Base data path (e.g., "/userdata/opticworks")
	TLSPath     string // TLS certificate storage path
	ConfigPath  string // Configuration file path
	ImagesPath  string // Storage for images/files

	// Process naming
	ProcessPrefix string // Process title prefix (e.g., "hardwareos")
}

// DefaultBrand is the default HardwareOS branding
var DefaultBrand = BrandConfig{
	ProductName: "HardwareOS",
	ProductCode: "hardwareos",

	DefaultAPIURL: "https://api.optic.works",
	DefaultAppURL: "https://app.optic.works",

	TLSDomain:       "opticworks.local",
	TLSCAName:       "OpticWorks Self-Signed CA",
	TLSOrganization: "OpticWorks",
	TLSOU:           "OpticWorks Self-Signed",

	HostnamePrefix: "opticworks",

	DataPath:   "/userdata/opticworks",
	TLSPath:    "/userdata/opticworks/tls",
	ConfigPath: "/userdata/opticworks/config.json",
	ImagesPath: "/userdata/opticworks/images",

	ProcessPrefix: "hardwareos",
}

// RS1Brand is the OpticWorks RS-1 specific branding
var RS1Brand = BrandConfig{
	ProductName: "OpticWorks RS-1",
	ProductCode: "rs1",

	DefaultAPIURL: "https://api.optic.works",
	DefaultAppURL: "https://app.optic.works",

	TLSDomain:       "opticworks.local",
	TLSCAName:       "OpticWorks Self-Signed CA",
	TLSOrganization: "OpticWorks",
	TLSOU:           "OpticWorks Self-Signed",

	HostnamePrefix: "opticworks-rs1",

	DataPath:   "/userdata/opticworks",
	TLSPath:    "/userdata/opticworks/tls",
	ConfigPath: "/userdata/opticworks/config.json",
	ImagesPath: "/userdata/opticworks/images",

	ProcessPrefix: "opticworks-rs1",
}

// activeBrand is the currently active brand configuration
var activeBrand = DefaultBrand

// SetBrand sets the active brand configuration
func SetBrand(brand BrandConfig) {
	activeBrand = brand
}

// GetBrand returns the active brand configuration
func GetBrand() BrandConfig {
	return activeBrand
}

// Brand returns a pointer to the active brand for convenient field access
func Brand() *BrandConfig {
	return &activeBrand
}
