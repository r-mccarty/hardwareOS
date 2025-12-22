# Startup Sequence

This document describes what happens when the JetKVM device boots, from the point the Linux kernel hands off to userspace.

## Overview

JetKVM is an embedded Linux device. There is no traditional getty/login shell. The `jetkvm_app` binary is the main application that runs after boot and manages all device functionality.

```
Linux Kernel Boot
       ↓
Init System (BusyBox init)
       ↓
/userdata/jetkvm/bin/jetkvm_app (as supervisor)
       ↓
jetkvm_app (as child/application)
       ↓
Device Ready (HTTP on :80, mDNS, Cloud connection)
```

## Process Architecture

The `jetkvm_app` binary runs in two modes:

### 1. Supervisor Mode (Parent)

When started without the `JETKVM_CHILD_ID` environment variable, the binary runs as a supervisor:

```go
// cmd/main.go
func main() {
    childID := os.Getenv(supervisor.EnvChildID)
    switch childID {
    case "":
        doSupervise()  // Run as supervisor
    case kvm.GetBuiltAppVersion():
        program()      // Run as application
    }
}
```

The supervisor:
- Spawns itself as a child process with `JETKVM_CHILD_ID=<version>`
- Captures stdout/stderr to log files
- Creates crash dumps on abnormal exit (`/userdata/jetkvm/crashdump/`)
- Forwards SIGTERM to the child process

### 2. Application Mode (Child)

When `JETKVM_CHILD_ID` matches the built version, `kvm.Main()` runs the actual application.

## Initialization Sequence

The `Main()` function in `main.go` initializes components in this order:

| Step | Function | Description |
|------|----------|-------------|
| 1 | `checkFailsafeReason()` | Check for failsafe mode triggers |
| 2 | `LoadConfig()` | Load `/userdata/kvm_config.json` |
| 3 | `runWatchdog()` | Start hardware watchdog (goroutine) |
| 4 | `initUsbGadget()` | Initialize USB HID gadget (keyboard, mouse) |
| 5 | `initNative()` | Start native process (LVGL, video capture) |
| 6 | `initDisplay()` | Initialize LCD display updates |
| 7 | `rootcerts.UpdateDefaultTransport()` | Load CA certificates |
| 8 | `initOta()` | Initialize OTA update system |
| 9 | `initNetwork()` | Initialize network manager (eth0) |
| 10 | `initTimeSync()` | Initialize NTP time sync |
| 11 | `timeSync.Start()` | Start time sync loop |
| 12 | `initMdns()` | Initialize mDNS responder |
| 13 | `initPrometheus()` | Register Prometheus metrics |
| 14 | `setInitialVirtualMediaState()` | Initialize virtual media |
| 15 | `initImagesFolder()` | Create images storage folder |
| 16 | `initJiggler()` | Initialize mouse jiggler |
| 17 | `startVideoSleepModeTicker()` | Start video sleep timer |
| 18 | `RunWebServer()` | Start HTTP server on port 80 (goroutine) |
| 19 | `RunWebSecureServer()` | Start HTTPS server if TLS enabled (goroutine) |
| 20 | `RunWebsocketClient()` | Connect to cloud API (goroutine) |
| 21 | `initPublicIPState()` | Initialize public IP detection |
| 22 | `initSerialPort()` | Initialize serial console |

After initialization, the main goroutine waits for `SIGINT` or `SIGTERM`.

## Key Components

### Hardware Watchdog

```go
// hw.go
func runWatchdog() {
    file, _ := os.OpenFile("/dev/watchdog", os.O_WRONLY, 0)
    ticker := time.NewTicker(10 * time.Second)
    for {
        select {
        case <-ticker.C:
            file.Write([]byte{0})  // Pet the watchdog
        case <-appCtx.Done():
            file.Write([]byte("V"))  // Disarm on shutdown
            return
        }
    }
}
```

The watchdog reboots the device if not petted within ~30 seconds (kernel configured).

### Failsafe Mode

Failsafe mode activates if:
1. `JETKVM_FORCE_FAILSAFE=1` environment variable is set
2. `/userdata/jetkvm/.enablefailsafe` file exists
3. Previous crash log contains panic traces

In failsafe mode:
- Native video/display is disabled (uses `EmptyNativeInterface`)
- Web UI shows failsafe banner
- Video-related JSON-RPC methods are blocked

### USB Gadget

Initializes the Linux USB gadget subsystem:
- Creates HID devices for keyboard and mouse
- Opens `/dev/hidg*` devices for input injection
- Monitors USB connection state (`/sys/class/udc/`)

### Native Process

Spawns a separate process for hardware interaction:
- LVGL graphics rendering to `/dev/fb0`
- H.264 video capture and encoding
- Touchscreen input handling

Communication via gRPC over Unix sockets.

### Network Manager

```go
// network.go
func initNetwork() error {
    nm := nmlite.NewNetworkManager(context.Background(), networkLogger)
    nm.SetHostname(hostname, domain)
    nm.SetOnInterfaceStateChange(networkStateChanged)
    nm.AddInterface("eth0", config.NetworkConfig)
    networkManager = nm
    return nil
}
```

Manages:
- DHCP client (jetdhcpc or udhcpc)
- Static IP configuration
- IPv4 and IPv6
- Hostname via `/etc/hostname`

### mDNS

Advertises the device on the local network:
- `jetkvm-<serial>.local`
- Configured hostname

### Web Server

```go
// web.go
func RunWebServer() {
    r := setupRouter()  // Gin router
    r.Run(":80")        // HTTP on port 80
}
```

Serves:
- Static web UI from embedded filesystem
- REST API endpoints (`/device/*`, `/auth/*`, `/storage/*`)
- WebSocket for WebRTC signaling (`/webrtc/signaling/*`)
- Prometheus metrics (`/metrics`)
- Log streaming (`/log-stream`)

### Cloud Connection

```go
// cloud.go
func RunWebsocketClient() {
    // Connects to wss://api.jetkvm.com/device/websocket
    // Handles cloud registration, session management
}
```

Maintains persistent WebSocket to cloud API for:
- Remote access sessions
- Cloud registration/adoption
- Session signaling

### Auto-Update

Starts 15 minutes after boot:

```go
go func() {
    time.Sleep(15 * time.Minute)
    for {
        if config.AutoUpdateEnabled && currentSession == nil {
            otaState.TryUpdate(...)
        }
        time.Sleep(1 * time.Hour)
    }
}()
```

## File Paths

| Path | Purpose |
|------|---------|
| `/userdata/jetkvm/bin/jetkvm_app` | Main application binary |
| `/userdata/kvm_config.json` | Configuration file |
| `/userdata/jetkvm/crashdump/` | Crash dump logs |
| `/userdata/jetkvm/last.log` | Last application log |
| `/userdata/jetkvm/tls/` | TLS certificates |
| `/userdata/jetkvm/.enablefailsafe` | Failsafe trigger file |
| `/dev/watchdog` | Hardware watchdog device |
| `/dev/fb0` | LCD framebuffer |
| `/dev/hidg*` | USB HID gadget devices |

## Process Tree

When running normally:
```
jetkvm: [supervisor] started (pid=123)
└── jetkvm: [app] ready
    └── jetkvm: [native] (video/display process)
```

Process titles are set via `gspt.SetProcTitle()` for easy identification.

## Shutdown

On `SIGTERM` or `SIGINT`:
1. Main goroutine receives signal
2. Context is cancelled
3. Watchdog is disarmed (writes "V")
4. Application exits cleanly

## Boot Timing

Approximate timing from power-on:
- Kernel boot: ~3-5 seconds
- Application start: ~1-2 seconds
- Network ready: ~2-5 seconds (DHCP)
- Web UI available: ~5-10 seconds total

## No Getty/Login Shell

This is an embedded device without:
- Traditional login prompt
- TTY console (except serial for debugging)
- SSH by default (must enable developer mode)

All interaction is via:
- Web UI (HTTP/HTTPS)
- Cloud dashboard
- Physical touchscreen
- Serial console (if connected)
