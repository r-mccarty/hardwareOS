package kvm

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/jetkvm/kvm/internal/confparser"
	"github.com/jetkvm/kvm/platform/logging"
	"github.com/jetkvm/kvm/platform/network/types"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	DefaultAPIURL = "https://api.optic.works"
)

type WakeOnLanDevice struct {
	Name       string `json:"name"`
	MacAddress string `json:"macAddress"`
}

type Config struct {
	CloudURL             string               `json:"cloud_url"`
	UpdateAPIURL         string               `json:"update_api_url"`
	CloudAppURL          string               `json:"cloud_app_url"`
	CloudToken           string               `json:"cloud_token"`
	GoogleIdentity       string               `json:"google_identity"`
	AutoUpdateEnabled    bool                 `json:"auto_update_enabled"`
	IncludePreRelease    bool                 `json:"include_pre_release"`
	HashedPassword       string               `json:"hashed_password"`
	LocalAuthToken       string               `json:"local_auth_token"`
	LocalAuthMode        string               `json:"localAuthMode"` //TODO: fix it with migration
	LocalLoopbackOnly    bool                 `json:"local_loopback_only"`
	WakeOnLanDevices     []WakeOnLanDevice    `json:"wake_on_lan_devices"`
	ActiveExtension      string               `json:"active_extension"`
	DisplayRotation      string               `json:"display_rotation"`
	DisplayMaxBrightness int                  `json:"display_max_brightness"`
	DisplayDimAfterSec   int                  `json:"display_dim_after_sec"`
	DisplayOffAfterSec   int                  `json:"display_off_after_sec"`
	TLSMode              string               `json:"tls_mode"` // options: "self-signed", "user-defined", ""
	NetworkConfig        *types.NetworkConfig `json:"network_config"`
	DefaultLogLevel      string               `json:"default_log_level"`
	VideoSleepAfterSec   int                  `json:"video_sleep_after_sec"`
	VideoQualityFactor   float64              `json:"video_quality_factor"`
	NativeMaxRestart     uint                 `json:"native_max_restart_attempts"`
}

// GetUpdateAPIURL returns the update API URL
func (c *Config) GetUpdateAPIURL() string {
	if c.UpdateAPIURL == "" {
		return DefaultAPIURL
	}
	return strings.TrimSuffix(c.UpdateAPIURL, "/") + "/releases"
}

// GetDisplayRotation returns the display rotation
func (c *Config) GetDisplayRotation() uint16 {
	rotationInt, err := strconv.ParseUint(c.DisplayRotation, 10, 16)
	if err != nil {
		logger.Warn().Err(err).Msg("invalid display rotation, using default")
		return 270
	}
	return uint16(rotationInt)
}

// SetDisplayRotation sets the display rotation
func (c *Config) SetDisplayRotation(rotation string) error {
	_, err := strconv.ParseUint(rotation, 10, 16)
	if err != nil {
		logger.Warn().Err(err).Msg("invalid display rotation, using default")
		return err
	}
	c.DisplayRotation = rotation
	return nil
}

const configPath = "/userdata/kvm_config.json"

func getDefaultConfig() Config {
	return Config{
		CloudURL:             DefaultAPIURL,
		UpdateAPIURL:         DefaultAPIURL,
		CloudAppURL:          "https://app.optic.works",
		AutoUpdateEnabled:    true,
		ActiveExtension:      "",
		DisplayRotation:      "270",
		DisplayMaxBrightness: 64,
		DisplayDimAfterSec:   120,  // 2 minutes
		DisplayOffAfterSec:   1800, // 30 minutes
		TLSMode:              "",
		NetworkConfig: func() *types.NetworkConfig {
			c := &types.NetworkConfig{}
			_ = confparser.SetDefaultsAndValidate(c)
			return c
		}(),
		DefaultLogLevel:    "INFO",
		VideoQualityFactor: 1.0,
	}
}

var (
	config     *Config
	configLock = &sync.Mutex{}
)

var (
	configSuccess = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "hardwareos_config_last_reload_successful",
			Help: "The last configuration load succeeded",
		},
	)
	configSuccessTime = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "hardwareos_config_last_reload_success_timestamp_seconds",
			Help: "Timestamp of last successful config load",
		},
	)
)

func LoadConfig() {
	configLock.Lock()
	defer configLock.Unlock()

	if config != nil {
		logger.Debug().Msg("config already loaded, skipping")
		return
	}

	// load the default config
	defaultConfig := getDefaultConfig()
	config = &defaultConfig

	file, err := os.Open(configPath)
	if err != nil {
		logger.Debug().Msg("default config file doesn't exist, using default")
		configSuccess.Set(1.0)
		configSuccessTime.SetToCurrentTime()
		return
	}
	defer file.Close()

	// load and merge the default config with the user config
	loadedConfig := defaultConfig
	if err := json.NewDecoder(file).Decode(&loadedConfig); err != nil {
		logger.Warn().Err(err).Msg("config file JSON parsing failed")
		configSuccess.Set(0.0)
		return
	}

	if loadedConfig.NetworkConfig == nil {
		loadedConfig.NetworkConfig = getDefaultConfig().NetworkConfig
	}

	config = &loadedConfig

	logging.GetRootLogger().UpdateLogLevel(config.DefaultLogLevel)

	configSuccess.Set(1.0)
	configSuccessTime.SetToCurrentTime()

	logger.Info().Str("path", configPath).Msg("config loaded")
}

func SaveConfig() error {
	return saveConfig(configPath)
}

func SaveBackupConfig() error {
	return saveConfig(configPath + ".bak")
}

func saveConfig(path string) error {
	configLock.Lock()
	defer configLock.Unlock()

	logger.Trace().Str("path", path).Msg("Saving config")

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(config); err != nil {
		return fmt.Errorf("failed to encode config: %w", err)
	}

	if err := file.Sync(); err != nil {
		return fmt.Errorf("failed to wite config: %w", err)
	}

	logger.Info().Str("path", path).Msg("config saved")
	return nil
}

func ensureConfigLoaded() {
	if config == nil {
		LoadConfig()
	}
}
