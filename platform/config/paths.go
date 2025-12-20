package config

import (
	"os"
	"path/filepath"
)

// Paths provides filesystem path abstractions based on the active brand.
// This ensures consistent path usage across the platform.
type Paths struct {
	brand *BrandConfig
}

// NewPaths creates a Paths instance using the active brand
func NewPaths() *Paths {
	return &Paths{brand: Brand()}
}

// DataDir returns the base data directory
func (p *Paths) DataDir() string {
	return p.brand.DataPath
}

// ConfigFile returns the main configuration file path
func (p *Paths) ConfigFile() string {
	return p.brand.ConfigPath
}

// ConfigBackupFile returns the backup configuration file path
func (p *Paths) ConfigBackupFile() string {
	return p.brand.ConfigPath + ".bak"
}

// TLSDir returns the TLS certificate directory
func (p *Paths) TLSDir() string {
	return p.brand.TLSPath
}

// ImagesDir returns the images/storage directory
func (p *Paths) ImagesDir() string {
	return p.brand.ImagesPath
}

// CrashDumpDir returns the crash dump directory
func (p *Paths) CrashDumpDir() string {
	return filepath.Join(p.brand.DataPath, "crashdump")
}

// LogFile returns the last log file path
func (p *Paths) LogFile() string {
	return filepath.Join(p.brand.DataPath, "last.log")
}

// BinDir returns the binary directory
func (p *Paths) BinDir() string {
	return filepath.Join(p.brand.DataPath, "bin")
}

// AppBinary returns the application binary path
func (p *Paths) AppBinary() string {
	return filepath.Join(p.BinDir(), p.brand.ProductCode+"_app")
}

// AppUpdateFile returns the app update file path
func (p *Paths) AppUpdateFile() string {
	return filepath.Join(p.brand.DataPath, p.brand.ProductCode+"_app.update")
}

// SystemUpdateFile returns the system update file path
func (p *Paths) SystemUpdateFile() string {
	return filepath.Join(p.brand.DataPath, "update_system.tar")
}

// FailsafeMarker returns the failsafe trigger file path
func (p *Paths) FailsafeMarker() string {
	return filepath.Join(p.brand.DataPath, ".enablefailsafe")
}

// DevModeMarker returns the developer mode marker file path
func (p *Paths) DevModeMarker() string {
	return filepath.Join(p.brand.DataPath, "devmode.enable")
}

// EnsureDirectories creates all necessary data directories
func (p *Paths) EnsureDirectories() error {
	dirs := []string{
		p.DataDir(),
		p.TLSDir(),
		p.ImagesDir(),
		p.CrashDumpDir(),
		p.BinDir(),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return nil
}

// DefaultPaths returns paths using the default brand
func DefaultPaths() *Paths {
	return NewPaths()
}
