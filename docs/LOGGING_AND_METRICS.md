# Logging and Metrics

This document covers the logging system and Prometheus metrics in JetKVM.

## Logging System

JetKVM uses [zerolog](https://github.com/rs/zerolog) for structured logging with subsystem-based log levels.

### Architecture

```
┌─────────────────────────────────────────────────────────┐
│  Root Logger (zerolog)                                  │
│  - Console output (stdout)                              │
│  - SSE streaming (/log-stream)                          │
└──────────────────────┬──────────────────────────────────┘
                       │
       ┌───────────────┼───────────────┐
       ▼               ▼               ▼
┌─────────────┐ ┌─────────────┐ ┌─────────────┐
│ jetkvm      │ │ cloud       │ │ webrtc      │  ... (20+ subsystems)
│ logger      │ │ logger      │ │ logger      │
└─────────────┘ └─────────────┘ └─────────────┘
```

### Subsystem Loggers

Each component has its own logger defined in `log.go`:

| Logger | Subsystem | Purpose |
|--------|-----------|---------|
| `logger` | jetkvm | Main application |
| `cloudLogger` | cloud | Cloud connectivity |
| `websocketLogger` | websocket | WebSocket connections |
| `webrtcLogger` | webrtc | WebRTC/video streaming |
| `nativeLogger` | native | Native C/LVGL bridge |
| `networkLogger` | network | Network management |
| `jsonRpcLogger` | jsonrpc | JSON-RPC protocol |
| `hidRPCLogger` | hidrpc | HID-RPC binary protocol |
| `usbLogger` | usb | USB gadget |
| `displayLogger` | display | LCD/backlight |
| `otaLogger` | ota | OTA updates |
| `serialLogger` | serial | Serial console |
| `timesyncLogger` | timesync | NTP time sync |
| `nbdLogger` | nbd | Network block device |
| `wolLogger` | wol | Wake-on-LAN |
| `websecureLogger` | websecure | TLS certificates |
| `failsafeLogger` | failsafe | Failsafe mode |
| `watchdogLogger` | watchdog | Hardware watchdog |
| `terminalLogger` | terminal | Terminal emulation |
| `ginLogger` | gin | HTTP framework |

### Log Levels

| Level | Value | Description |
|-------|-------|-------------|
| `TRACE` | -1 | Most verbose, detailed debugging |
| `DEBUG` | 0 | Debug information |
| `INFO` | 1 | Informational messages |
| `WARN` | 2 | Warnings |
| `ERROR` | 3 | Errors (default level) |
| `FATAL` | 4 | Fatal errors |
| `PANIC` | 5 | Panic-level errors |
| `DISABLE` | 7 | Logging disabled |

### Configuring Log Levels

#### Environment Variables

Set log levels per-subsystem using environment variables:

```bash
# Enable TRACE for specific subsystems
export JETKVM_LOG_TRACE="cloud,websocket,jsonrpc"

# Enable DEBUG for all subsystems
export JETKVM_LOG_DEBUG="all"

# Enable ERROR logging for all, TRACE for specific
export JETKVM_LOG_ERROR="all"
export JETKVM_LOG_TRACE="native,hidrpc"
```

#### Development Script

The `dev_deploy.sh` script sets logging automatically:

```bash
# Default trace scopes
./dev_deploy.sh -r <IP>
# Sets: JETKVM_LOG_TRACE="jetkvm,cloud,websocket,native,jsonrpc"

# Custom trace scopes
./dev_deploy.sh -r <IP> --log-trace "usb,hidrpc,jsonrpc"
```

#### Pion (WebRTC) Logging

Pion WebRTC library logs are integrated via `internal/logging/pion.go`:

```bash
# Pion-specific environment variables also work
export PION_LOG_TRACE="all"
```

### Log Output

#### Console Output

Logs are written to stdout with this format:
```
2024-01-15T10:30:45Z INF cloud connected to cloud server component=cloud
```

Format: `timestamp level message component=subsystem [additional fields]`

#### SSE Streaming (Live Logs)

Logs are also streamed via Server-Sent Events for live viewing:

```bash
# View in browser (HTML interface)
curl http://<device-ip>/log-stream

# Raw SSE stream
curl -H "Accept: text/event-stream" http://<device-ip>/log-stream
```

The SSE endpoint is attached in `internal/logging/sse.go` and provides:
- Real-time log streaming to multiple clients
- HTML viewer at `/log-stream`
- Automatic client connection management

### Using Loggers in Code

```go
import "github.com/jetkvm/kvm/internal/logging"

// Get a subsystem logger
var myLogger = logging.GetSubsystemLogger("mysubsystem")

// Use zerolog methods
myLogger.Info().Msg("something happened")
myLogger.Error().Err(err).Str("key", "value").Msg("error occurred")
myLogger.Trace().Int("count", 42).Msg("detailed trace")
```

### Linting Rules

The `.golangci.yml` enforces logging rules:

```yaml
forbidigo:
  forbid:
    - pattern: ^fmt\.Print.*$
      msg: Do not commit print statements. Use logger package.
    - pattern: ^log\.(Fatal|Panic|Print)(f|ln)?.*$
      msg: Do not commit log statements. Use logger package.
```

**Do not use:**
- `fmt.Print`, `fmt.Println`, `fmt.Printf`
- `log.Print`, `log.Fatal`, `log.Panic`

**Use instead:**
- `logging.GetSubsystemLogger("name")` with zerolog methods

## Prometheus Metrics

JetKVM exposes Prometheus metrics at `/metrics` for monitoring.

### Metrics Endpoint

```bash
curl http://<device-ip>/metrics
```

Or with authentication if enabled:
```bash
curl http://api:<password>@<device-ip>/metrics
```

### Available Metrics

#### Version Info

```
# HELP jetkvm_build_info Build information
# TYPE jetkvm_build_info gauge
jetkvm_build_info{branch="dev",goversion="go1.24.4",revision="abc123",version="0.5.1"} 1
```

Registered in `prometheus.go`:
```go
prometheus.MustRegister(versioncollector.NewCollector("jetkvm"))
```

#### DC Power Metrics (ATX Extension)

When DC power control is enabled (`dc_metrics.go`):

| Metric | Type | Description |
|--------|------|-------------|
| `jetkvm_dc_current_amperes` | Gauge | Current draw in amps |
| `jetkvm_dc_power_watts` | Gauge | Power consumption in watts |
| `jetkvm_dc_voltage_volts` | Gauge | Voltage in volts |
| `jetkvm_dc_power_state` | Gauge | Power state (1=on, 0=off) |

#### Time Sync Metrics

Defined in `internal/timesync/metrics.go`:

| Metric | Type | Description |
|--------|------|-------------|
| `jetkvm_timesync_status` | Gauge | Sync status (1=success, 0=fail) |
| `jetkvm_timesync_total` | Counter | Total sync attempts |
| `jetkvm_timesync_success_total` | Counter | Successful syncs |

**NTP Metrics (per server):**

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `jetkvm_timesync_ntp_request_total` | Counter | url | Requests sent |
| `jetkvm_timesync_ntp_success_total` | Counter | url | Successful responses |
| `jetkvm_timesync_ntp_server_last_rtt` | Gauge | url | Last RTT (ms) |
| `jetkvm_timesync_ntp_server_rtt` | Histogram | url | RTT distribution |
| `jetkvm_timesync_ntp_server_info` | Gauge | url, reference, stratum, precision | Server info |

**HTTP Time Sync Metrics (per server):**

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `jetkvm_timesync_http_request_total` | Counter | url | Requests sent |
| `jetkvm_timesync_http_success_total` | Counter | url | Successful responses |
| `jetkvm_timesync_http_cancel_total` | Counter | url | Cancelled requests |
| `jetkvm_timesync_http_server_last_rtt` | Gauge | url | Last RTT (ms) |
| `jetkvm_timesync_http_server_rtt` | Histogram | url | RTT distribution |

### Adding New Metrics

```go
import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

// Auto-registered gauge
var myGauge = promauto.NewGauge(prometheus.GaugeOpts{
    Name: "jetkvm_my_metric",
    Help: "Description of the metric",
})

// Manual registration (for conditional metrics)
var myCounter = prometheus.NewCounter(prometheus.CounterOpts{
    Name: "jetkvm_my_counter",
    Help: "Description",
})

func init() {
    prometheus.MustRegister(myCounter)
}

// Update metrics
myGauge.Set(42.0)
myCounter.Inc()
```

### Metric Types

| Type | Use Case | Methods |
|------|----------|---------|
| `Gauge` | Current value (can go up/down) | `Set()`, `Inc()`, `Dec()`, `Add()` |
| `Counter` | Cumulative count (only increases) | `Inc()`, `Add()` |
| `Histogram` | Distribution of values | `Observe()` |
| `Summary` | Similar to histogram with quantiles | `Observe()` |

### Prometheus Integration Example

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'jetkvm'
    static_configs:
      - targets: ['192.168.1.100:80']
    basic_auth:
      username: 'api'
      password: 'your-device-password'
```

## Performance Profiling

When Developer Mode is enabled, pprof endpoints are available:

```bash
# Enable Developer Mode in device settings first
# Then access profiling endpoints:

curl http://api:<password>@<device-ip>/developer/pprof/
curl http://api:<password>@<device-ip>/developer/pprof/heap
curl http://api:<password>@<device-ip>/developer/pprof/goroutine
curl http://api:<password>@<device-ip>/developer/pprof/profile?seconds=30
```

## Viewing Logs on Device

```bash
# SSH to device
ssh root@<device-ip>

# View live application logs
tail -f /var/log/jetkvm.log

# View with filtering
tail -f /var/log/jetkvm.log | grep "cloud"
```

## Key Files Reference

| File | Purpose |
|------|---------|
| `log.go` | Subsystem logger definitions |
| `prometheus.go` | Version metrics initialization |
| `dc_metrics.go` | DC power Prometheus metrics |
| `internal/logging/logger.go` | Logger implementation, level management |
| `internal/logging/root.go` | Root logger initialization |
| `internal/logging/sse.go` | SSE log streaming endpoint |
| `internal/logging/pion.go` | Pion WebRTC logging adapter |
| `internal/logging/utils.go` | Logging utilities |
| `internal/timesync/metrics.go` | Time sync Prometheus metrics |
| `.golangci.yml` | Lint rules forbidding print statements |
