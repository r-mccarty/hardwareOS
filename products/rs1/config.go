// Package rs1 implements the OpticWorks RS-1 product.
// RS-1 is a vision/radar sensor fusion device for room occupancy detection.
package rs1

// RS1Config contains RS-1 specific configuration
type RS1Config struct {
	// Room configuration
	RoomConfig *RoomConfig `json:"room_config,omitempty"`

	// Camera settings
	CameraConfig *CameraConfig `json:"camera_config,omitempty"`

	// Radar settings
	RadarConfig *RadarConfig `json:"radar_config,omitempty"`

	// Detection zones
	DetectionZones []DetectionZone `json:"detection_zones,omitempty"`

	// AI model path
	ModelPath string `json:"model_path,omitempty"`
}

// RoomConfig defines the physical room dimensions
type RoomConfig struct {
	Width  float64 `json:"width"`  // Room width in meters
	Height float64 `json:"height"` // Room height in meters
	Depth  float64 `json:"depth"`  // Room depth in meters
}

// CameraConfig defines camera parameters
type CameraConfig struct {
	Enabled    bool    `json:"enabled"`
	Resolution string  `json:"resolution"` // e.g., "1920x1080"
	FOV        float64 `json:"fov"`        // Field of view in degrees
	MountAngle float64 `json:"mount_angle"` // Camera tilt angle
}

// RadarConfig defines radar parameters
type RadarConfig struct {
	Enabled      bool    `json:"enabled"`
	MaxRange     float64 `json:"max_range"`     // Maximum detection range in meters
	Sensitivity  float64 `json:"sensitivity"`   // Detection sensitivity 0-1
	UpdateRateHz int     `json:"update_rate_hz"` // Update rate in Hz
}

// DetectionZone defines a named detection zone
type DetectionZone struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	X        float64 `json:"x"`      // Zone center X
	Y        float64 `json:"y"`      // Zone center Y
	Width    float64 `json:"width"`  // Zone width
	Height   float64 `json:"height"` // Zone height
	Enabled  bool    `json:"enabled"`
}

// DefaultConfig returns the default RS-1 configuration
func DefaultConfig() *RS1Config {
	return &RS1Config{
		RoomConfig: &RoomConfig{
			Width:  5.0,
			Height: 3.0,
			Depth:  5.0,
		},
		CameraConfig: &CameraConfig{
			Enabled:    true,
			Resolution: "1920x1080",
			FOV:        90.0,
			MountAngle: 0.0,
		},
		RadarConfig: &RadarConfig{
			Enabled:      true,
			MaxRange:     6.0,
			Sensitivity:  0.5,
			UpdateRateHz: 10,
		},
		DetectionZones: []DetectionZone{},
		ModelPath:      "/userdata/models/yolov8n.rknn",
	}
}
