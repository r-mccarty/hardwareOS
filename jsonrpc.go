package kvm

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/pion/webrtc/v4"
	"github.com/rs/zerolog"
	"go.bug.st/serial"

	"github.com/jetkvm/kvm/internal/diagnostics"
	"github.com/jetkvm/kvm/internal/supervisor"
	"github.com/jetkvm/kvm/internal/utils"
)

// ansiRegex matches ANSI escape sequences for stripping from log output
var ansiRegex = regexp.MustCompile(`[\x1b\x9b][[\]()#;?]*(?:(?:(?:[a-zA-Z\d]*(?:;[a-zA-Z\d]*)*)?\x07)|(?:(?:\d{1,4}(?:;\d{0,4})*)?[\dA-PRZcf-ntqry=><~]))`)

// cleanLogOutput strips ANSI codes and unescapes newlines/tabs for readable output
func cleanLogOutput(s string) string {
	s = ansiRegex.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "\\n", "\n")
	s = strings.ReplaceAll(s, "\\t", "\t")
	return s
}

type JSONRPCRequest struct {
	JSONRPC string         `json:"jsonrpc"`
	Method  string         `json:"method"`
	Params  map[string]any `json:"params,omitempty"`
	ID      any            `json:"id,omitempty"`
}

type JSONRPCResponse struct {
	JSONRPC string `json:"jsonrpc"`
	Result  any    `json:"result,omitempty"`
	Error   any    `json:"error,omitempty"`
	ID      any    `json:"id"`
}

type JSONRPCEvent struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type DisplayRotationSettings struct {
	Rotation string `json:"rotation"`
}

type BacklightSettings struct {
	MaxBrightness int `json:"max_brightness"`
	DimAfter      int `json:"dim_after"`
	OffAfter      int `json:"off_after"`
}

func writeJSONRPCResponse(response JSONRPCResponse, session *Session) {
	responseBytes, err := json.Marshal(response)
	if err != nil {
		jsonRpcLogger.Warn().Err(err).Msg("Error marshalling JSONRPC response")
		return
	}
	err = session.RPCChannel.SendText(string(responseBytes))
	if err != nil {
		jsonRpcLogger.Warn().Err(err).Msg("Error sending JSONRPC response")
		return
	}
}

func writeJSONRPCEvent(event string, params any, session *Session) {
	request := JSONRPCEvent{
		JSONRPC: "2.0",
		Method:  event,
		Params:  params,
	}
	requestBytes, err := json.Marshal(request)
	if err != nil {
		jsonRpcLogger.Warn().Err(err).Msg("Error marshalling JSONRPC event")
		return
	}
	if session == nil || session.RPCChannel == nil {
		jsonRpcLogger.Info().Msg("RPC channel not available")
		return
	}

	requestString := string(requestBytes)
	scopedLogger := jsonRpcLogger.With().
		Str("data", requestString).
		Logger()

	scopedLogger.Trace().Msg("sending JSONRPC event")

	err = session.RPCChannel.SendText(requestString)
	if err != nil {
		scopedLogger.Warn().Err(err).Msg("error sending JSONRPC event")
		return
	}
}

func onRPCMessage(message webrtc.DataChannelMessage, session *Session) {
	var request JSONRPCRequest
	err := json.Unmarshal(message.Data, &request)
	if err != nil {
		jsonRpcLogger.Warn().
			Str("data", string(message.Data)).
			Err(err).
			Msg("Error unmarshalling JSONRPC request")

		errorResponse := JSONRPCResponse{
			JSONRPC: "2.0",
			Error: map[string]any{
				"code":    -32700,
				"message": "Parse error",
			},
			ID: 0,
		}
		writeJSONRPCResponse(errorResponse, session)
		return
	}

	scopedLogger := jsonRpcLogger.With().
		Str("method", request.Method).
		Interface("params", request.Params).
		Interface("id", request.ID).Logger()

	scopedLogger.Trace().Msg("Received RPC request")
	t := time.Now()

	handler, ok := rpcHandlers[request.Method]
	if !ok {
		errorResponse := JSONRPCResponse{
			JSONRPC: "2.0",
			Error: map[string]any{
				"code":    -32601,
				"message": "Method not found",
			},
			ID: request.ID,
		}
		writeJSONRPCResponse(errorResponse, session)
		return
	}

	result, err := callRPCHandler(scopedLogger, handler, request.Params)
	if err != nil {
		scopedLogger.Error().Err(err).Msg("Error calling RPC handler")
		errorResponse := JSONRPCResponse{
			JSONRPC: "2.0",
			Error: map[string]any{
				"code":    -32603,
				"message": "Internal error",
				"data":    err.Error(),
			},
			ID: request.ID,
		}
		writeJSONRPCResponse(errorResponse, session)
		return
	}

	scopedLogger.Trace().Dur("duration", time.Since(t)).Interface("result", result).Msg("RPC handler returned")

	response := JSONRPCResponse{
		JSONRPC: "2.0",
		Result:  result,
		ID:      request.ID,
	}
	writeJSONRPCResponse(response, session)
}

func rpcPing() (string, error) {
	return "pong", nil
}

func rpcGetDeviceID() (string, error) {
	return GetDeviceID(), nil
}

func rpcReboot(force bool) error {
	logger.Info().Msg("Got reboot request via RPC")
	return hwReboot(force, nil, 0)
}

func rpcGetStreamQualityFactor() (float64, error) {
	return config.VideoQualityFactor, nil
}

func rpcSetStreamQualityFactor(factor float64) error {
	logger.Info().Float64("factor", factor).Msg("Setting stream quality factor")
	err := nativeInstance.VideoSetQualityFactor(factor)
	if err != nil {
		return err
	}

	config.VideoQualityFactor = factor
	if err := SaveConfig(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	return nil
}

func rpcGetAutoUpdateState() (bool, error) {
	return config.AutoUpdateEnabled, nil
}

func rpcSetAutoUpdateState(enabled bool) (bool, error) {
	config.AutoUpdateEnabled = enabled
	if err := SaveConfig(); err != nil {
		return config.AutoUpdateEnabled, fmt.Errorf("failed to save config: %w", err)
	}
	return enabled, nil
}

func rpcSetDisplayRotation(params DisplayRotationSettings) error {
	currentRotation := config.DisplayRotation
	if currentRotation == params.Rotation {
		return nil
	}

	err := config.SetDisplayRotation(params.Rotation)
	if err != nil {
		return err
	}

	_, err = nativeInstance.DisplaySetRotation(config.GetDisplayRotation())
	if err != nil {
		return err
	}

	if err := SaveConfig(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	return err
}

func rpcGetDisplayRotation() (*DisplayRotationSettings, error) {
	return &DisplayRotationSettings{
		Rotation: config.DisplayRotation,
	}, nil
}

func rpcSetBacklightSettings(params BacklightSettings) error {
	blConfig := params

	// NOTE: by default, the frontend limits the brightness to 64, as that's what the device originally shipped with.
	if blConfig.MaxBrightness > 255 || blConfig.MaxBrightness < 0 {
		return fmt.Errorf("maxBrightness must be between 0 and 255")
	}

	if blConfig.DimAfter < 0 {
		return fmt.Errorf("dimAfter must be a positive integer")
	}

	if blConfig.OffAfter < 0 {
		return fmt.Errorf("offAfter must be a positive integer")
	}

	config.DisplayMaxBrightness = blConfig.MaxBrightness
	config.DisplayDimAfterSec = blConfig.DimAfter
	config.DisplayOffAfterSec = blConfig.OffAfter

	if err := SaveConfig(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	logger.Info().Int("max_brightness", config.DisplayMaxBrightness).Int("dim_after", config.DisplayDimAfterSec).Int("off_after", config.DisplayOffAfterSec).Msg("rpc: display: settings applied")

	// If the device started up with auto-dim and/or auto-off set to zero, the display init
	// method will not have started the tickers. So in case that has changed, attempt to start the tickers now.
	startBacklightTickers()

	// Wake the display after the settings are altered, this ensures the tickers
	// are reset to the new settings, and will bring the display up to maxBrightness.
	// Calling with force set to true, to ignore the current state of the display, and force
	// it to reset the tickers.
	wakeDisplay(true, "backlight_settings_changed")
	return nil
}

func rpcGetBacklightSettings() (*BacklightSettings, error) {
	return &BacklightSettings{
		MaxBrightness: config.DisplayMaxBrightness,
		DimAfter:      int(config.DisplayDimAfterSec),
		OffAfter:      int(config.DisplayOffAfterSec),
	}, nil
}

const (
	devModeFile = "/userdata/jetkvm/devmode.enable"
	sshKeyDir   = "/userdata/dropbear/.ssh"
	sshKeyFile  = "/userdata/dropbear/.ssh/authorized_keys"
)

type DevModeState struct {
	Enabled bool `json:"enabled"`
}

type SSHKeyState struct {
	SSHKey string `json:"sshKey"`
}

func rpcGetDevModeState() (DevModeState, error) {
	devModeEnabled := false
	if _, err := os.Stat(devModeFile); err != nil {
		if !os.IsNotExist(err) {
			return DevModeState{}, fmt.Errorf("error checking dev mode file: %w", err)
		}
	} else {
		devModeEnabled = true
	}

	return DevModeState{
		Enabled: devModeEnabled,
	}, nil
}

func rpcSetDevModeState(enabled bool) error {
	if enabled {
		if _, err := os.Stat(devModeFile); os.IsNotExist(err) {
			if err := os.MkdirAll(filepath.Dir(devModeFile), 0755); err != nil {
				return fmt.Errorf("failed to create directory for devmode file: %w", err)
			}
			if err := os.WriteFile(devModeFile, []byte{}, 0644); err != nil {
				return fmt.Errorf("failed to create devmode file: %w", err)
			}
		} else {
			logger.Debug().Msg("dev mode already enabled")
			return nil
		}
	} else {
		if _, err := os.Stat(devModeFile); err == nil {
			if err := os.Remove(devModeFile); err != nil {
				return fmt.Errorf("failed to remove devmode file: %w", err)
			}
		} else if os.IsNotExist(err) {
			logger.Debug().Msg("dev mode already disabled")
			return nil
		} else {
			return fmt.Errorf("error checking dev mode file: %w", err)
		}
	}

	cmd := exec.Command("dropbear.sh")
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Warn().Err(err).Bytes("output", output).Msg("Failed to start/stop SSH")
		return fmt.Errorf("failed to start/stop SSH, you may need to reboot for changes to take effect")
	}

	return nil
}

func rpcGetSSHKeyState() (string, error) {
	keyData, err := os.ReadFile(sshKeyFile)
	if err != nil {
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("error reading SSH key file: %w", err)
		}
	}
	return string(keyData), nil
}

func rpcSetSSHKeyState(sshKey string) error {
	if sshKey == "" {
		// Remove SSH key file if empty string is provided
		if err := os.Remove(sshKeyFile); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove SSH key file: %w", err)
		}
		return nil
	}

	// Validate SSH key
	if err := utils.ValidateSSHKey(sshKey); err != nil {
		return err
	}

	// Create directory if it doesn't exist
	if err := os.MkdirAll(sshKeyDir, 0700); err != nil {
		return fmt.Errorf("failed to create SSH key directory: %w", err)
	}

	// Write SSH key to file
	if err := os.WriteFile(sshKeyFile, []byte(sshKey), 0600); err != nil {
		return fmt.Errorf("failed to write SSH key: %w", err)
	}

	return nil
}

func rpcGetTLSState() TLSState {
	return getTLSState()
}

func rpcSetTLSState(state TLSState) error {
	err := setTLSState(state)
	if err != nil {
		return fmt.Errorf("failed to set TLS state: %w", err)
	}

	if err := SaveConfig(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	return nil
}

type RPCHandler struct {
	Func   any
	Params []string
}

// call the handler but recover from a panic to ensure our RPC thread doesn't collapse on malformed calls
func callRPCHandler(logger zerolog.Logger, handler RPCHandler, params map[string]any) (result any, err error) {
	// Use defer to recover from a panic
	defer func() {
		if r := recover(); r != nil {
			// Convert the panic to an error
			if e, ok := r.(error); ok {
				err = e
			} else {
				err = fmt.Errorf("panic occurred: %v", r)
			}
		}
	}()

	// Call the handler
	result, err = riskyCallRPCHandler(logger, handler, params)
	return result, err // do not combine these two lines into one, as it breaks the above defer function's setting of err
}

func riskyCallRPCHandler(logger zerolog.Logger, handler RPCHandler, params map[string]any) (any, error) {
	handlerValue := reflect.ValueOf(handler.Func)
	handlerType := handlerValue.Type()

	if handlerType.Kind() != reflect.Func {
		return nil, errors.New("handler is not a function")
	}

	numParams := handlerType.NumIn()
	paramNames := handler.Params // Get the parameter names from the RPCHandler

	if len(paramNames) != numParams {
		err := fmt.Errorf("mismatch between handler parameters (%d) and defined parameter names (%d)", numParams, len(paramNames))
		logger.Error().Strs("paramNames", paramNames).Err(err).Msg("Cannot call RPC handler")
		return nil, err
	}

	args := make([]reflect.Value, numParams)

	for i := range numParams {
		paramType := handlerType.In(i)
		paramName := paramNames[i]
		paramValue, ok := params[paramName]
		if !ok {
			err := fmt.Errorf("missing parameter: %s", paramName)
			logger.Error().Err(err).Msg("Cannot marshal arguments for RPC handler")
			return nil, err
		}

		convertedValue := reflect.ValueOf(paramValue)
		if !convertedValue.Type().ConvertibleTo(paramType) {
			if paramType.Kind() == reflect.Slice && (convertedValue.Kind() == reflect.Slice || convertedValue.Kind() == reflect.Array) {
				newSlice := reflect.MakeSlice(paramType, convertedValue.Len(), convertedValue.Len())
				for j := 0; j < convertedValue.Len(); j++ {
					elemValue := convertedValue.Index(j)
					if elemValue.Kind() == reflect.Interface {
						elemValue = elemValue.Elem()
					}
					if !elemValue.Type().ConvertibleTo(paramType.Elem()) {
						// Handle float64 to uint8 conversion
						if elemValue.Kind() == reflect.Float64 && paramType.Elem().Kind() == reflect.Uint8 {
							intValue := int(elemValue.Float())
							if intValue < 0 || intValue > 255 {
								return nil, fmt.Errorf("value out of range for uint8: %v for parameter %s", intValue, paramName)
							}
							newSlice.Index(j).SetUint(uint64(intValue))
						} else {
							fromType := elemValue.Type()
							toType := paramType.Elem()
							return nil, fmt.Errorf("invalid element type in slice for parameter %s: from %v to %v", paramName, fromType, toType)
						}
					} else {
						newSlice.Index(j).Set(elemValue.Convert(paramType.Elem()))
					}
				}
				args[i] = newSlice
			} else if paramType.Kind() == reflect.Struct && convertedValue.Kind() == reflect.Map {
				jsonData, err := json.Marshal(convertedValue.Interface())
				if err != nil {
					return nil, fmt.Errorf("failed to marshal map to JSON: %v for parameter %s", err, paramName)
				}

				newStruct := reflect.New(paramType).Interface()
				if err := json.Unmarshal(jsonData, newStruct); err != nil {
					return nil, fmt.Errorf("failed to unmarshal JSON into struct: %v for parameter %s", err, paramName)
				}
				args[i] = reflect.ValueOf(newStruct).Elem()
			} else {
				return nil, fmt.Errorf("invalid parameter type for: %s, type: %s", paramName, paramType.Kind())
			}
		} else {
			args[i] = convertedValue.Convert(paramType)
		}
	}

	logger.Trace().Msg("Calling RPC handler")
	results := handlerValue.Call(args)

	if len(results) == 0 {
		return nil, nil
	}

	if len(results) == 1 {
		if ok, err := asError(results[0]); ok {
			return nil, err
		}
		return results[0].Interface(), nil
	}

	if len(results) == 2 {
		if ok, err := asError(results[1]); ok {
			if err != nil {
				return nil, err
			}
		}
		return results[0].Interface(), nil
	}

	return nil, fmt.Errorf("too many return values from handler: %d", len(results))
}

func asError(value reflect.Value) (bool, error) {
	if value.Type().Implements(reflect.TypeOf((*error)(nil)).Elem()) {
		if value.IsNil() {
			return true, nil
		}
		return true, value.Interface().(error)
	}
	return false, nil
}

func rpcIsUpdatePending() (bool, error) {
	return otaState.IsUpdatePending(), nil
}

func rpcGetWakeOnLanDevices() ([]WakeOnLanDevice, error) {
	if config.WakeOnLanDevices == nil {
		return []WakeOnLanDevice{}, nil
	}
	return config.WakeOnLanDevices, nil
}

type SetWakeOnLanDevicesParams struct {
	Devices []WakeOnLanDevice `json:"devices"`
}

func rpcSetWakeOnLanDevices(params SetWakeOnLanDevicesParams) error {
	config.WakeOnLanDevices = params.Devices
	return SaveConfig()
}

func rpcResetConfig() error {
	defaultConfig := getDefaultConfig()
	config = &defaultConfig
	if err := SaveConfig(); err != nil {
		return fmt.Errorf("failed to reset config: %w", err)
	}

	logger.Info().Msg("Configuration reset to default")
	return nil
}

func rpcGetDiagnostics() (string, error) {
	var sb strings.Builder

	// Section 1: Application log (last.log) - last 500 lines
	sb.WriteString("=== APPLICATION LOG ===\n")
	if data, err := os.ReadFile(supervisor.AppLogPath); err == nil {
		content := cleanLogOutput(string(data))
		lines := strings.Split(content, "\n")
		if len(lines) > 500 {
			content = "... (truncated, showing last 500 lines)\n" + strings.Join(lines[len(lines)-500:], "\n")
		}
		sb.WriteString(content)
	} else {
		sb.WriteString(fmt.Sprintf("Error reading log: %v\n", err))
	}
	sb.WriteString("\n\n")

	// Collect diagnostics to a buffer (not to last.log)
	var diagBuf strings.Builder
	diag := diagnostics.New(diagnostics.Options{
		Writer: &diagBuf,
		GetSessionInfo: func() diagnostics.SessionInfo {
			info := diagnostics.SessionInfo{
				ActiveSessions:    getActiveSessions(),
				HasCurrentSession: currentSession != nil,
			}
			if currentSession != nil {
				sessionInfo := currentSession.GetDiagnosticsInfo()
				info.ICEConnectionState = sessionInfo.ICEConnectionState
				info.SignalingState = sessionInfo.SignalingState
				info.ConnectionState = sessionInfo.ConnectionState
				info.DataChannels = sessionInfo.DataChannels
			}
			return info
		},
	})
	diag.LogAll("download")

	// Section 2: System diagnostics
	sb.WriteString("=== SYSTEM DIAGNOSTICS ===\n")
	sb.WriteString(diagBuf.String())
	sb.WriteString("\n")

	// Section 3: Last crash log
	lastCrashPath := filepath.Join(supervisor.ErrorDumpDir, supervisor.ErrorDumpLastFile)
	sb.WriteString("=== LAST CRASH LOG ===\n")
	if data, err := os.ReadFile(lastCrashPath); err == nil {
		sb.WriteString(cleanLogOutput(string(data)))
	} else {
		sb.WriteString(fmt.Sprintf("No crash log found: %v\n", err))
	}
	sb.WriteString("\n\n")

	// Section 4: Recent crash dumps (excluding last-crash.log)
	sb.WriteString("=== RECENT CRASH DUMPS ===\n")
	if entries, err := os.ReadDir(supervisor.ErrorDumpDir); err == nil {
		type fileInfo struct {
			name    string
			modTime time.Time
		}
		var crashFiles []fileInfo
		for _, entry := range entries {
			if entry.IsDir() || entry.Name() == supervisor.ErrorDumpLastFile {
				continue
			}
			// Match jetkvm-*.log pattern
			if !strings.HasPrefix(entry.Name(), "jetkvm-") || !strings.HasSuffix(entry.Name(), ".log") {
				continue
			}
			info, err := entry.Info()
			if err != nil {
				continue
			}
			crashFiles = append(crashFiles, fileInfo{name: entry.Name(), modTime: info.ModTime()})
		}
		// Sort by modification time (newest first)
		sort.Slice(crashFiles, func(i, j int) bool {
			return crashFiles[i].modTime.After(crashFiles[j].modTime)
		})

		// Take 12 most recent crash dumps
		if len(crashFiles) > 12 {
			crashFiles = crashFiles[:12]
		}

		for i, cf := range crashFiles {
			sb.WriteString(fmt.Sprintf("--- %s ---\n", cf.name))
			crashPath := filepath.Join(supervisor.ErrorDumpDir, cf.name)
			if data, err := os.ReadFile(crashPath); err == nil {
				content := cleanLogOutput(string(data))
				// First file is full, rest are last 50 lines
				if i > 0 {
					lines := strings.Split(content, "\n")
					if len(lines) > 100 {
						content = "... (truncated)\n" + strings.Join(lines[len(lines)-50:], "\n")
					}
				}
				sb.WriteString(content)
			} else {
				sb.WriteString(fmt.Sprintf("Error reading: %v\n", err))
			}
			sb.WriteString("\n")
		}
		if len(crashFiles) == 0 {
			sb.WriteString("No crash dumps found\n")
		}
	} else {
		sb.WriteString(fmt.Sprintf("Error reading crash directory: %v\n", err))
	}
	sb.WriteString("\n")

	// Section 5: Configuration
	sb.WriteString("=== CONFIGURATION ===\n")
	if data, err := os.ReadFile(configPath); err == nil {
		sb.WriteString(string(data))
	} else {
		sb.WriteString(fmt.Sprintf("Error reading config: %v\n", err))
	}
	sb.WriteString("\n")

	return sb.String(), nil
}

type SerialSettings struct {
	BaudRate string `json:"baudRate"`
	DataBits string `json:"dataBits"`
	StopBits string `json:"stopBits"`
	Parity   string `json:"parity"`
}

func rpcGetSerialSettings() (SerialSettings, error) {
	settings := SerialSettings{
		BaudRate: strconv.Itoa(serialPortMode.BaudRate),
		DataBits: strconv.Itoa(serialPortMode.DataBits),
		StopBits: "1",
		Parity:   "none",
	}

	switch serialPortMode.StopBits {
	case serial.OneStopBit:
		settings.StopBits = "1"
	case serial.OnePointFiveStopBits:
		settings.StopBits = "1.5"
	case serial.TwoStopBits:
		settings.StopBits = "2"
	}

	switch serialPortMode.Parity {
	case serial.NoParity:
		settings.Parity = "none"
	case serial.OddParity:
		settings.Parity = "odd"
	case serial.EvenParity:
		settings.Parity = "even"
	case serial.MarkParity:
		settings.Parity = "mark"
	case serial.SpaceParity:
		settings.Parity = "space"
	}

	return settings, nil
}

var serialPortMode = defaultMode

func rpcSetSerialSettings(settings SerialSettings) error {
	baudRate, err := strconv.Atoi(settings.BaudRate)
	if err != nil {
		return fmt.Errorf("invalid baud rate: %v", err)
	}
	dataBits, err := strconv.Atoi(settings.DataBits)
	if err != nil {
		return fmt.Errorf("invalid data bits: %v", err)
	}

	var stopBits serial.StopBits
	switch settings.StopBits {
	case "1":
		stopBits = serial.OneStopBit
	case "1.5":
		stopBits = serial.OnePointFiveStopBits
	case "2":
		stopBits = serial.TwoStopBits
	default:
		return fmt.Errorf("invalid stop bits: %s", settings.StopBits)
	}

	var parity serial.Parity
	switch settings.Parity {
	case "none":
		parity = serial.NoParity
	case "odd":
		parity = serial.OddParity
	case "even":
		parity = serial.EvenParity
	case "mark":
		parity = serial.MarkParity
	case "space":
		parity = serial.SpaceParity
	default:
		return fmt.Errorf("invalid parity: %s", settings.Parity)
	}
	serialPortMode = &serial.Mode{
		BaudRate: baudRate,
		DataBits: dataBits,
		StopBits: stopBits,
		Parity:   parity,
	}

	_ = port.SetMode(serialPortMode)

	return nil
}

func rpcSetCloudUrl(apiUrl string, appUrl string) error {
	currentCloudURL := config.CloudURL
	config.CloudURL = apiUrl
	config.CloudAppURL = appUrl

	if currentCloudURL != apiUrl {
		disconnectCloud(fmt.Errorf("cloud url changed from %s to %s", currentCloudURL, apiUrl))
	}

	if publicIPState != nil {
		publicIPState.SetCloudflareEndpoint(apiUrl)
	}

	if err := SaveConfig(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	return nil
}

func rpcGetLocalLoopbackOnly() (bool, error) {
	return config.LocalLoopbackOnly, nil
}

func rpcSetLocalLoopbackOnly(enabled bool) error {
	// Check if the setting is actually changing
	if config.LocalLoopbackOnly == enabled {
		return nil
	}

	// Update the setting
	config.LocalLoopbackOnly = enabled
	if err := SaveConfig(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	return nil
}

// Platform RPC handlers - keep only platform-relevant functionality
var rpcHandlers = map[string]RPCHandler{
	// Core
	"ping":        {Func: rpcPing},
	"reboot":      {Func: rpcReboot, Params: []string{"force"}},
	"getDeviceID": {Func: rpcGetDeviceID},

	// Cloud
	"deregisterDevice": {Func: rpcDeregisterDevice},
	"getCloudState":    {Func: rpcGetCloudState},
	"setCloudUrl":      {Func: rpcSetCloudUrl, Params: []string{"apiUrl", "appUrl"}},

	// Network
	"getNetworkState":    {Func: rpcGetNetworkState},
	"getNetworkSettings": {Func: rpcGetNetworkSettings},
	"setNetworkSettings": {Func: rpcSetNetworkSettings, Params: []string{"settings"}},
	"renewDHCPLease":     {Func: rpcRenewDHCPLease},

	// Video/Stream
	"getVideoState":          {Func: rpcGetVideoState},
	"getStreamQualityFactor": {Func: rpcGetStreamQualityFactor},
	"setStreamQualityFactor": {Func: rpcSetStreamQualityFactor, Params: []string{"factor"}},

	// Display
	"setDisplayRotation":   {Func: rpcSetDisplayRotation, Params: []string{"params"}},
	"getDisplayRotation":   {Func: rpcGetDisplayRotation},
	"setBacklightSettings": {Func: rpcSetBacklightSettings, Params: []string{"params"}},
	"getBacklightSettings": {Func: rpcGetBacklightSettings},

	// Updates
	"getAutoUpdateState":  {Func: rpcGetAutoUpdateState},
	"setAutoUpdateState":  {Func: rpcSetAutoUpdateState, Params: []string{"enabled"}},
	"getDevChannelState":  {Func: rpcGetDevChannelState},
	"setDevChannelState":  {Func: rpcSetDevChannelState, Params: []string{"enabled"}},
	"getLocalVersion":     {Func: rpcGetLocalVersion},
	"getUpdateStatus":     {Func: rpcGetUpdateStatus},
	"tryUpdate":           {Func: rpcTryUpdate},
	"isUpdatePending":     {Func: rpcIsUpdatePending},

	// Security
	"getDevModeState": {Func: rpcGetDevModeState},
	"setDevModeState": {Func: rpcSetDevModeState, Params: []string{"enabled"}},
	"getSSHKeyState":  {Func: rpcGetSSHKeyState},
	"setSSHKeyState":  {Func: rpcSetSSHKeyState, Params: []string{"sshKey"}},
	"getTLSState":     {Func: rpcGetTLSState},
	"setTLSState":     {Func: rpcSetTLSState, Params: []string{"state"}},

	// Automation
	"getWakeOnLanDevices":   {Func: rpcGetWakeOnLanDevices},
	"setWakeOnLanDevices":   {Func: rpcSetWakeOnLanDevices, Params: []string{"params"}},
	"sendWOLMagicPacket":    {Func: rpcSendWOLMagicPacket, Params: []string{"macAddress"}},

	// Serial
	"getSerialSettings": {Func: rpcGetSerialSettings},
	"setSerialSettings": {Func: rpcSetSerialSettings, Params: []string{"settings"}},

	// Config
	"resetConfig":           {Func: rpcResetConfig},
	"getLocalLoopbackOnly":  {Func: rpcGetLocalLoopbackOnly},
	"setLocalLoopbackOnly":  {Func: rpcSetLocalLoopbackOnly, Params: []string{"enabled"}},

	// Diagnostics
	"getDiagnostics":        {Func: rpcGetDiagnostics},
	"getPublicIPAddresses":  {Func: rpcGetPublicIPAddresses, Params: []string{"refresh"}},
	"checkPublicIPAddresses": {Func: rpcCheckPublicIPAddresses},
}
