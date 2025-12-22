package kvm

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/erikdubbelboer/gspt"
	"github.com/gwatts/rootcerts"
	platformConfig "github.com/jetkvm/kvm/platform/config"
	"github.com/jetkvm/kvm/platform/ota"
	"github.com/jetkvm/kvm/products/rs1"
	"github.com/jetkvm/kvm/products/rs1/fusion"
)

var appCtx context.Context
var procPrefix string
var rs1Product *rs1.RS1Product

func init() {
	// Set RS-1 as the default product brand
	platformConfig.SetBrand(platformConfig.RS1Brand)
	procPrefix = platformConfig.Brand().ProcessPrefix + ": [app]"
}

func setProcTitle(status string) {
	if status != "" {
		status = " " + status
	}
	title := fmt.Sprintf("%s%s", procPrefix, status)
	gspt.SetProcTitle(title)
}

func Main() {
	brand := platformConfig.Brand()
	setProcTitle("starting")

	logger.Log().Str("product", brand.ProductName).Msg("Starting Up")

	checkFailsafeReason()
	if failsafeModeActive {
		procPrefix = brand.ProcessPrefix + ": [app+failsafe]"
		logger.Warn().Str("reason", failsafeModeReason).Msg("failsafe mode activated")
	}

	LoadConfig()

	var cancel context.CancelFunc
	appCtx, cancel = context.WithCancel(context.Background())
	defer cancel()

	systemVersionLocal, appVersionLocal, err := GetLocalVersion()
	if err != nil {
		logger.Warn().Err(err).Msg("failed to get local version")
	}

	logger.Info().
		Str("product", brand.ProductName).
		Interface("system_version", systemVersionLocal).
		Interface("app_version", appVersionLocal).
		Msg("starting")

	go runWatchdog()

	setProcTitle("initNative")
	initNative(systemVersionLocal, appVersionLocal)
	initDisplay()

	http.DefaultClient.Timeout = 1 * time.Minute

	err = rootcerts.UpdateDefaultTransport()
	if err != nil {
		logger.Warn().Err(err).Msg("failed to load Root CA certificates")
	}
	logger.Info().
		Int("ca_certs_loaded", len(rootcerts.Certs())).
		Msg("loaded Root CA certificates")

	initOta()

	http.DefaultClient.Timeout = 1 * time.Minute

	// Initialize network
	setProcTitle("initNetwork")
	if err := initNetwork(); err != nil {
		logger.Error().Err(err).Msg("failed to initialize network")
		os.Exit(1)
	}

	// Initialize time sync
	setProcTitle("initTimeSync")
	initTimeSync()
	timeSync.Start()

	// Initialize mDNS
	setProcTitle("initMdns")
	if err := initMdns(); err != nil {
		logger.Error().Err(err).Msg("failed to initialize mDNS")
	}

	// Initialize RS-1 product
	setProcTitle("initRS1")
	rs1Product, err = rs1.Init()
	if err != nil {
		logger.Error().Err(err).Msg("failed to initialize RS-1 product")
	} else {
		// Wire up accessors for kvm package
		SetRS1WorldStateGetter(func() interface{} {
			if rs1Product == nil {
				return nil
			}
			return rs1Product.GetWorldState().GetSnapshot()
		})
		SetRS1FusionEngineSetter(func(t *fusion.TransformMatrix) {
			if rs1Product != nil {
				rs1Product.GetFusionEngine().GetEngine().SetRoomTransform(t)
			}
		})

		// Start RS-1 services
		if err := rs1Product.Start(); err != nil {
			logger.Error().Err(err).Msg("failed to start RS-1 services")
		}
	}

	setProcTitle("initPrometheus")
	initPrometheus()

	// start video sleep mode timer
	startVideoSleepModeTicker()

	go func() {
		// wait for 15 minutes before starting auto-update checks
		time.Sleep(15 * time.Minute)

		for {
			logger.Info().Bool("auto_update_enabled", config.AutoUpdateEnabled).Msg("auto-update check")
			if !config.AutoUpdateEnabled {
				logger.Debug().Msg("auto-update disabled")
				time.Sleep(5 * time.Minute)
				continue
			}

			if currentSession != nil {
				logger.Debug().Msg("skipping update since a session is active")
				time.Sleep(1 * time.Minute)
				continue
			}

			if isTimeSyncNeeded() || !timeSync.IsSyncSuccess() {
				logger.Debug().Msg("system time is not synced, will retry in 30 seconds")
				time.Sleep(30 * time.Second)
				continue
			}

			includePreRelease := config.IncludePreRelease
			err = otaState.TryUpdate(context.Background(), ota.UpdateParams{
				DeviceID:          GetDeviceID(),
				IncludePreRelease: includePreRelease,
			})
			if err != nil {
				logger.Warn().Err(err).Msg("failed to auto update")
			}

			time.Sleep(1 * time.Hour)
		}
	}()

	go RunWebServer()

	go RunWebSecureServer()
	if config.TLSMode != "" {
		startWebSecureServer()
	}

	go RunWebsocketClient()
	initPublicIPState()

	initSerialPort()

	setProcTitle("ready")

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs

	// Graceful shutdown
	if rs1Product != nil {
		rs1Product.Stop()
	}

	logger.Log().Str("product", brand.ProductName).Msg("Shutting Down")
}
