# Time Synchronization

This document explains how time synchronization works in JetKVM, including NTP, HTTP fallback, and RTC (Real-Time Clock) persistence.

## Why Time Sync is Needed

JetKVM is an embedded device without a battery-backed real-time clock (RTC) that maintains time during power-off. When the device boots:

1. **System time defaults to build time** - Without synchronization, the system clock may be incorrect
2. **TLS certificates require accurate time** - HTTPS connections and cloud authentication fail with wrong time
3. **Logging timestamps become meaningless** - Debugging requires accurate timestamps
4. **Session tokens and cookies may expire** - Authentication breaks with clock skew

The device checks if time sync is needed by comparing the current system time against the build timestamp:

```go
// timesync.go
func isTimeSyncNeeded() bool {
    // If current time is BEFORE the build time, sync is needed
    if now.Sub(builtTime) < 0 {
        return true
    }
    return false
}
```

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                     TimeSync Manager                            │
├─────────────────────────────────────────────────────────────────┤
│  Pre-checks: Network online? IPv4 ready? IPv6 ready?           │
└────────────────────────┬────────────────────────────────────────┘
                         │
         ┌───────────────┼───────────────┐
         ▼               ▼               ▼
┌─────────────┐  ┌─────────────┐  ┌─────────────┐
│ NTP (DHCP)  │  │ NTP (Static)│  │ HTTP Date   │
│ Servers     │  │ Servers     │  │ Headers     │
└─────────────┘  └─────────────┘  └─────────────┘
         │               │               │
         └───────────────┼───────────────┘
                         ▼
              ┌─────────────────┐
              │  Set System     │
              │  Time (date -s) │
              └────────┬────────┘
                       ▼
              ┌─────────────────┐
              │  Update RTC     │
              │  (/dev/rtc)     │
              └─────────────────┘
```

## Sync Methods

### 1. NTP (Network Time Protocol)

The primary sync method using standard NTP protocol (UDP port 123).

**Default NTP Server IPs** (no DNS lookup needed):
| Provider | IPv4 | IPv6 |
|----------|------|------|
| Cloudflare | `162.159.200.1`, `162.159.200.123` | `2606:4700:f1::1`, `2606:4700:f1::123` |
| Google | `216.239.35.0`, `216.239.35.4`, `216.239.35.8`, `216.239.35.12` | `2001:4860:4806::`, etc. |

**Default NTP Hostnames** (requires DNS):
- `time.apple.com`
- `time.aws.com`
- `time.windows.com`
- `time.google.com`
- `time.cloudflare.com`
- `pool.ntp.org`

**NTP Sources (in order of preference)**:
1. **DHCP-provided NTP servers** (`ntp_dhcp`) - From DHCP Option 42
2. **User-configured NTP servers** (`ntp_user_provided`) - Custom static IPs
3. **Fallback NTP IPs** (`ntp`) - Hardcoded Google/Cloudflare IPs
4. **Fallback NTP hostnames** - Requires DNS resolution

### 2. HTTP Time Sync

Fallback method that extracts time from HTTP `Date` response headers.

**Default HTTP URLs**:
```
http://www.gstatic.com/generate_204         (Google)
http://cp.cloudflare.com/                   (Cloudflare)
http://edge-http.microsoft.com/captiveportal/generate_204  (Microsoft)
```

HTTP sync is less accurate than NTP (no RTT compensation) but works in restrictive networks that block NTP.

## Sync Flow

### Startup Sequence

```
1. Device boots, network initializes
           ↓
2. initTimeSync() creates TimeSync instance
   - Finds RTC device (/dev/rtc or /dev/rtc0)
   - Reads current RTC time
           ↓
3. timeSync.Start() begins sync loop
           ↓
4. Wait for network (100ms check interval)
           ↓
5. Sync attempt (2s timeout per server)
           ↓
6. On success:
   - Set system time (date -s)
   - Update RTC device
   - Wait 1 hour, repeat
           ↓
7. On failure:
   - Retry with exponential backoff (5s → 1min max)
```

### Sync Order

The default ordering is:
1. `ntp_dhcp` - Try DHCP-provided NTP servers first
2. `ntp` - Fall back to hardcoded NTP servers
3. `http` - Last resort: HTTP Date headers

The ordering is configurable via `time_sync_ordering` in network config.

## Configuration

### Time Sync Mode

| Mode | NTP | HTTP | Description |
|------|-----|------|-------------|
| `ntp_and_http` | ✓ | ✓ | Use both methods (default) |
| `ntp_only` | ✓ | ✗ | Only use NTP |
| `http_only` | ✗ | ✓ | Only use HTTP |
| `custom` | ✓ | ✓ | Use custom server lists |

### Configuration Options

In `/userdata/kvm_config.json` under `network_config`:

```json
{
  "network_config": {
    "time_sync_mode": "ntp_and_http",
    "time_sync_ordering": ["ntp_dhcp", "ntp", "http"],
    "time_sync_disable_fallback": false,
    "time_sync_parallel": 4,
    "time_sync_ntp_servers": [],
    "time_sync_http_urls": []
  }
}
```

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| `time_sync_mode` | string | `ntp_and_http` | Sync method mode |
| `time_sync_ordering` | array | `["ntp_dhcp", "ntp", "http"]` | Order to try sync methods |
| `time_sync_disable_fallback` | bool | `false` | Disable fallback servers |
| `time_sync_parallel` | int | `4` | Concurrent queries per chunk |
| `time_sync_ntp_servers` | array | `[]` | Custom NTP server IPs |
| `time_sync_http_urls` | array | `[]` | Custom HTTP URLs |

### Ordering Options

| Value | Description |
|-------|-------------|
| `ntp_dhcp` | NTP servers from DHCP |
| `ntp_user_provided` | User-configured NTP servers |
| `ntp` | Fallback NTP servers (IPs then hostnames) |
| `http_user_provided` | User-configured HTTP URLs |
| `http` | Fallback HTTP URLs |

## RTC (Real-Time Clock)

### Purpose

The RTC is a hardware clock that maintains time when the system is off (if battery-backed). On JetKVM, the RTC:
- Persists time across reboots
- Provides initial time before network sync
- May not be battery-backed (depends on hardware revision)

### Device Paths

The code searches these paths in order:
```
/dev/rtc
/dev/rtc0
/dev/rtc1
/dev/misc/rtc
/dev/misc/rtc0
/dev/misc/rtc1
```

### Operations

```go
// Read RTC time
rtcTime, err := t.readRtcTime()

// Set RTC time (after system time is synced)
err := t.setRtcTime(now)
```

The RTC is updated every time a successful sync occurs.

## Timing Constants

| Constant | Value | Description |
|----------|-------|-------------|
| Initial wait | 3 seconds | Wait before first sync attempt |
| Query timeout | 2 seconds | Timeout per NTP/HTTP query |
| Retry step | 5 seconds | Backoff increment on failure |
| Max retry interval | 1 minute | Maximum backoff delay |
| Sync interval | 1 hour | Time between successful syncs |
| Network check interval | 100ms | Interval to check network status |

## Parallel Queries

To improve reliability and speed, multiple servers are queried in parallel:

1. Server list is shuffled (randomized)
2. Queries are sent in chunks (default: 4 concurrent)
3. First successful response wins
4. Remaining queries are cancelled

```go
// Example: 12 NTP servers, chunk size 4
// Round 1: Query servers 0-3 in parallel
// Round 2: Query servers 4-7 if Round 1 failed
// Round 3: Query servers 8-11 if Round 2 failed
```

## IPv4/IPv6 Filtering

NTP servers are filtered based on current network connectivity:

```go
func (t *TimeSync) filterNTPServers(ntpServers []string) {
    if hasIPv4 && ip.To4() != nil {
        // Include IPv4 server
    }
    if hasIPv6 && ip.To16() != nil {
        // Include IPv6 server
    }
}
```

This prevents failed queries to IPv6 servers when only IPv4 is available (and vice versa).

## Prometheus Metrics

All time sync operations are instrumented for monitoring.

### General Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `jetkvm_timesync_status` | Gauge | 1 = synced, 0 = not synced |
| `jetkvm_timesync_total` | Counter | Total sync attempts |
| `jetkvm_timesync_success_total` | Counter | Successful syncs |
| `jetkvm_timesync_rtc_update_total` | Counter | RTC updates |

### NTP Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `jetkvm_timesync_ntp_request_total` | Counter | `url` | Requests per server |
| `jetkvm_timesync_ntp_success_total` | Counter | `url` | Successes per server |
| `jetkvm_timesync_ntp_server_last_rtt` | Gauge | `url` | Last RTT (ms) |
| `jetkvm_timesync_ntp_server_rtt` | Histogram | `url` | RTT distribution |
| `jetkvm_timesync_ntp_server_info` | Gauge | `url`, `reference`, `stratum`, `precision` | Server info |

### HTTP Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `jetkvm_timesync_http_request_total` | Counter | `url` | Requests per URL |
| `jetkvm_timesync_http_success_total` | Counter | `url` | Successes per URL |
| `jetkvm_timesync_http_cancel_total` | Counter | `url` | Cancelled requests |
| `jetkvm_timesync_http_server_last_rtt` | Gauge | `url` | Last RTT (ms) |
| `jetkvm_timesync_http_server_rtt` | Histogram | `url` | RTT distribution |
| `jetkvm_timesync_http_server_info` | Gauge | `url`, `http_code` | Server info |

## Troubleshooting

### Time Not Syncing

1. **Check network connectivity**
   ```bash
   ping 162.159.200.1  # Cloudflare NTP
   ```

2. **Check if NTP port is blocked**
   ```bash
   nc -u 162.159.200.1 123
   ```

3. **Check sync status via metrics**
   ```bash
   curl http://<device>/metrics | grep timesync
   ```

4. **Check logs**
   ```bash
   grep timesync /var/log/jetkvm.log
   ```

### Using HTTP Fallback in Restrictive Networks

If NTP is blocked, configure HTTP-only sync:

```json
{
  "network_config": {
    "time_sync_mode": "http_only"
  }
}
```

### Custom NTP Servers

For private networks with internal NTP:

```json
{
  "network_config": {
    "time_sync_mode": "custom",
    "time_sync_ordering": ["ntp_user_provided", "ntp"],
    "time_sync_ntp_servers": ["10.0.0.1", "10.0.0.2"]
  }
}
```

## File Locations

| File | Purpose |
|------|---------|
| `timesync.go` | Top-level initialization and check |
| `internal/timesync/timesync.go` | Core sync logic |
| `internal/timesync/ntp.go` | NTP query implementation |
| `internal/timesync/http.go` | HTTP time query |
| `internal/timesync/rtc.go` | RTC device detection |
| `internal/timesync/rtc_linux.go` | Linux RTC read/write |
| `internal/timesync/metrics.go` | Prometheus metrics |
| `internal/network/types/config.go` | Configuration options |

## Example: Complete Sync Flow

```
1. Device boots at 2024-01-01 00:00:00 (no RTC battery)
           ↓
2. Built timestamp: 2024-06-15 12:00:00
   Current time < Build time → sync needed
           ↓
3. Network comes up, DHCP provides:
   - IP: 192.168.1.100
   - NTP servers: 192.168.1.1
           ↓
4. Query 192.168.1.1 (DHCP NTP) → timeout
           ↓
5. Query 162.159.200.1 (Cloudflare) → success
   Response: 2024-07-20 14:30:45, RTT: 15ms
           ↓
6. Set system time: date -s "2024-07-20 14:30:45"
           ↓
7. Update RTC: ioctl(fd, RTC_SET_TIME, ...)
           ↓
8. Metrics updated:
   jetkvm_timesync_status = 1
   jetkvm_timesync_success_total++
           ↓
9. Sleep 1 hour, repeat
```
