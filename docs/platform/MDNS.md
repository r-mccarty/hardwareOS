# mDNS (Multicast DNS)

This document explains how mDNS works in JetKVM, enabling local network discovery without a DNS server.

## Overview

mDNS allows devices on a local network to discover each other using human-readable names like `jetkvm-abc123.local` instead of IP addresses. JetKVM uses the [Pion mDNS](https://github.com/pion/mdns) library to implement this.

## How It Works

### Discovery Flow

```
1. User types "jetkvm.local" in browser
              ↓
2. Browser sends mDNS query to 224.0.0.251:5353 (IPv4)
   or ff02::fb:5353 (IPv6)
              ↓
3. JetKVM receives query, checks if name matches
              ↓
4. JetKVM responds with its IP address
              ↓
5. Browser connects to the resolved IP
```

### Multicast Addresses

| Protocol | Address | Port |
|----------|---------|------|
| IPv4 | `224.0.0.251` | 5353 |
| IPv6 | `ff02::fb` | 5353 |

## Advertised Names

JetKVM advertises these local names:

| Name | Example | Source |
|------|---------|--------|
| Hostname | `jetkvm-abc123.local` | Device serial number |
| FQDN | `jetkvm-abc123.domain.local` | Hostname + configured domain |
| Custom hostname | `mykvm.local` | User-configured hostname |

Names are automatically suffixed with `.local` if not already present.

## Configuration

### mDNS Mode

Configurable via Web UI at **Settings → Network → mDNS**:

| Mode | Description | IPv4 | IPv6 |
|------|-------------|------|------|
| `auto` | Respond on both protocols (default) | ✓ | ✓ |
| `ipv4_only` | Respond only to IPv4 queries | ✓ | ✗ |
| `ipv6_only` | Respond only to IPv6 queries | ✗ | ✓ |
| `disabled` | mDNS completely disabled | ✗ | ✗ |

### Configuration File

In `/userdata/kvm_config.json`:

```json
{
  "network_config": {
    "hostname": "jetkvm-abc123",
    "domain": "",
    "mdns_mode": "auto"
  }
}
```

## Initialization

### Startup Sequence

```go
// main.go - during initialization
initMdns()  // Create mDNS instance (does not start yet)

// network.go - after network is ready
networkStateChanged() {
    if mDNS != nil {
        restartMdns()  // Start/restart with current config
    }
}
```

mDNS is initialized early but **does not start until the network interface is up**. This ensures the device has an IP address to advertise.

### Options Setup

```go
// network.go
func getMdnsOptions() *mdns.MDNSOptions {
    return &mdns.MDNSOptions{
        LocalNames: []string{
            networkManager.Hostname(),  // e.g., "jetkvm-abc123"
            networkManager.FQDN(),      // e.g., "jetkvm-abc123.local"
        },
        ListenOptions: &mdns.MDNSListenOptions{
            IPv4: true,  // Based on mdns_mode config
            IPv6: true,
        },
    }
}
```

## Network State Integration

mDNS automatically restarts when network state changes:

```go
// network.go
func networkStateChanged(ifName string, state types.InterfaceState) {
    // ... other handlers ...

    // Always restart mDNS when network state changes
    if mDNS != nil {
        restartMdns()
    }
}
```

This handles:
- Network interface coming online
- IP address changes (DHCP renewal)
- Hostname changes
- mDNS mode changes

## Implementation Details

### File Locations

| File | Purpose |
|------|---------|
| `mdns.go` | Top-level initialization |
| `internal/mdns/mdns.go` | mDNS server wrapper |
| `network.go` | mDNS options and restart logic |
| `internal/network/types/config.go` | mDNS mode configuration |

### MDNS Struct

```go
type MDNS struct {
    conn          *pion_mdns.Conn  // Pion mDNS connection
    lock          sync.Mutex       // Thread safety
    localNames    []string         // Names to advertise
    listenOptions *MDNSListenOptions
}
```

### Key Methods

| Method | Description |
|--------|-------------|
| `NewMDNS(opts)` | Create new mDNS instance |
| `Start()` | Start the mDNS server |
| `Restart()` | Stop and restart with current config |
| `Stop()` | Stop the mDNS server |
| `SetOptions(opts)` | Update config and restart |

## Client Discovery

### From macOS/Linux

```bash
# Discover JetKVM devices
dns-sd -B _http._tcp local

# Resolve a specific name
dns-sd -G v4v6 jetkvm-abc123.local

# Or using avahi (Linux)
avahi-browse -art
avahi-resolve -n jetkvm-abc123.local
```

### From Windows

```powershell
# Windows 10+ has mDNS support
ping jetkvm-abc123.local

# Or use Bonjour Browser
```

### From Browser

Simply navigate to:
```
http://jetkvm-abc123.local
```

Most modern browsers support `.local` domains via mDNS.

## Troubleshooting

### Device Not Discoverable

1. **Check mDNS mode**: Ensure it's not `disabled`
   ```bash
   # SSH to device
   cat /userdata/kvm_config.json | grep mdns_mode
   ```

2. **Check network state**: mDNS only works when network is up
   ```bash
   # Check interface
   ip addr show eth0
   ```

3. **Check firewall**: Ensure UDP port 5353 is not blocked

4. **Check logs**: Look for mDNS startup messages
   ```bash
   # In device logs
   grep -i mdns /var/log/jetkvm.log
   ```

### Name Conflicts

If another device has the same name, mDNS behavior is undefined. Ensure unique hostnames:
- Default: `jetkvm-<serial>` (unique per device)
- Custom: Set a unique hostname in settings

### IPv6 Issues

If IPv6 mDNS isn't working:
1. Check if IPv6 is enabled on the network
2. Try `ipv4_only` mode as a workaround
3. Verify the device has an IPv6 address

## Security Considerations

- mDNS is **unencrypted** and **unauthenticated**
- Only use on trusted local networks
- mDNS responses can be spoofed
- Consider disabling mDNS if not needed

## Interaction with Other Services

### WebRTC/ICE

mDNS names can be used in ICE candidates for local WebRTC connections, allowing peer discovery without a STUN server on local networks.

### Cloud Connection

mDNS is purely local. Cloud connections use:
- Device ID (serial number) for identification
- Cloud API for discovery and signaling

## Example: Complete Flow

```
1. Device boots, network initializes
   ↓
2. DHCP assigns IP 192.168.1.100
   ↓
3. networkStateChanged() called
   ↓
4. restartMdns() starts mDNS server
   - Binds to 224.0.0.251:5353 (IPv4)
   - Binds to ff02::fb:5353 (IPv6)
   - Advertises: jetkvm-abc123.local
   ↓
5. User opens browser, types jetkvm-abc123.local
   ↓
6. Browser sends mDNS query
   ↓
7. Device responds: jetkvm-abc123.local → 192.168.1.100
   ↓
8. Browser connects to http://192.168.1.100
   ↓
9. Web UI loads, user can control device
```
