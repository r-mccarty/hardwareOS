# Network Management

This document explains how network management works in JetKVM, including DHCP, static IP, IPv6, and integration with other subsystems.

## Overview

JetKVM uses a custom lightweight network manager (`nmlite`) to handle all network configuration. The device has a single Ethernet interface (`eth0`) that supports both IPv4 and IPv6.

```
┌─────────────────────────────────────────────────────────────────┐
│                        network.go                               │
│  (Integration layer - connects nmlite to app subsystems)        │
└─────────────────────────┬───────────────────────────────────────┘
                          │
┌─────────────────────────▼───────────────────────────────────────┐
│                   NetworkManager (nmlite)                        │
│  - Interface management                                          │
│  - State change callbacks                                        │
│  - Hostname/resolv.conf management                               │
└─────────────────────────┬───────────────────────────────────────┘
                          │
┌─────────────────────────▼───────────────────────────────────────┐
│                   InterfaceManager                               │
│  - Per-interface state tracking                                  │
│  - DHCP client management                                        │
│  - Static IP configuration                                       │
│  - Netlink monitoring                                            │
└────────────┬────────────────────────────────┬───────────────────┘
             │                                │
┌────────────▼────────────┐    ┌──────────────▼──────────────────┐
│      DHCPClient         │    │         Static Config           │
│  ┌─────────────────┐    │    │  - IPv4 address/netmask/gateway │
│  │   jetdhcpc      │    │    │  - IPv6 prefix/gateway          │
│  │   (native Go)   │    │    │  - DNS servers                  │
│  └─────────────────┘    │    └─────────────────────────────────┘
│  ┌─────────────────┐    │
│  │    udhcpc       │    │
│  │ (BusyBox proc)  │    │
│  └─────────────────┘    │
└─────────────────────────┘
```

## Configuration

### Configuration File

Network settings are stored in `/userdata/kvm_config.json` under `network_config`:

```json
{
  "network_config": {
    "dhcp_client": "udhcpc",
    "hostname": "jetkvm-abc123",
    "domain": "",
    "http_proxy": "",

    "ipv4_mode": "dhcp",
    "ipv4_static": null,

    "ipv6_mode": "slaac",
    "ipv6_static": null,

    "mdns_mode": "auto",
    "lldp_mode": "basic",
    "lldp_tx_tlvs": ["chassis", "port", "system", "vlan"],

    "time_sync_mode": "ntp_and_http",
    "time_sync_ordering": ["ntp_dhcp", "ntp", "http"]
  }
}
```

### IPv4 Modes

| Mode | Description |
|------|-------------|
| `dhcp` | Obtain IP via DHCP (default) |
| `static` | Use static IP configuration |
| `disabled` | Disable IPv4 |

### IPv6 Modes

| Mode | Description |
|------|-------------|
| `slaac` | Stateless Address Autoconfiguration (default) |
| `dhcpv6` | DHCPv6 only |
| `slaac_and_dhcpv6` | Both SLAAC and DHCPv6 |
| `static` | Static IPv6 configuration |
| `link_local` | Link-local address only |
| `disabled` | Disable IPv6 |

### Static IP Configuration

**IPv4 Static:**
```json
{
  "ipv4_mode": "static",
  "ipv4_static": {
    "address": "192.168.1.100",
    "netmask": "255.255.255.0",
    "gateway": "192.168.1.1",
    "dns": ["8.8.8.8", "8.8.4.4"]
  }
}
```

**IPv6 Static:**
```json
{
  "ipv6_mode": "static",
  "ipv6_static": {
    "prefix": "2001:db8::100/64",
    "gateway": "2001:db8::1",
    "dns": ["2001:4860:4860::8888"]
  }
}
```

## DHCP Clients

JetKVM supports two DHCP clients:

### udhcpc (Default)

BusyBox's DHCP client, spawned as external process.

**Pros:**
- Well-tested, widely used
- Handles edge cases well

**Cons:**
- External process management
- Less control over behavior

### jetdhcpc (Native Go)

Custom Go implementation of DHCP client.

**Pros:**
- Native Go, no external process
- Full control over behavior
- Supports DHCPv6

**Cons:**
- Newer, less battle-tested

**Switching DHCP clients:**
```go
// Via RPC
rpcToggleDHCPClient()  // Toggles between udhcpc and jetdhcpc, requires reboot
```

## Interface State

The `InterfaceState` structure tracks the current network status:

```go
type InterfaceState struct {
    InterfaceName string    // "eth0"
    Hostname      string    // Device hostname
    MACAddress    string    // Hardware address
    Up            bool      // Link is up
    Online        bool      // Has global unicast address
    IPv4Ready     bool      // Has IPv4 address
    IPv6Ready     bool      // Has global IPv6 address
    IPv4Address   string    // Primary IPv4 address
    IPv6Address   string    // Primary global IPv6 address
    IPv6LinkLocal string    // Link-local IPv6 address
    IPv6Gateway   string    // IPv6 default gateway
    IPv4Addresses []string  // All IPv4 addresses
    IPv6Addresses []IPv6Address  // All IPv6 addresses with metadata
    NTPServers    []net.IP  // NTP servers from DHCP
    DHCPLease4    *DHCPLease  // IPv4 DHCP lease info
    DHCPLease6    *DHCPLease  // IPv6 DHCP lease info
    LastUpdated   time.Time
}
```

### State Change Reasons

| Reason | Description |
|--------|-------------|
| `IfStateOperStateChanged` | Link up/down changed |
| `IfStateOnlineStateChanged` | Online status changed |
| `IfStateMACAddressChanged` | MAC address changed |
| `IfStateIPAddressesChanged` | IP addresses changed |

## Network State Change Handling

When network state changes, `network.go` orchestrates updates to other subsystems:

```go
func networkStateChanged(ifName string, state types.InterfaceState) {
    // 1. Update LCD display
    waitCtrlAndRequestDisplayUpdate(true, "network_state_changed")

    // 2. Notify UI via JSON-RPC WebSocket
    writeJSONRPCEvent("networkState", state, currentSession)

    // 3. Trigger time sync if coming online
    if state.Online {
        triggerTimeSyncOnNetworkStateChange()
    }

    // 4. Update public IP detection
    setPublicIPReadyState(state.IPv4Ready, state.IPv6Ready)

    // 5. Restart mDNS with new addresses
    restartMdns()
}
```

## Integration with Other Subsystems

### Time Sync

When network comes online:
1. DHCP-provided NTP servers are passed to time sync
2. A sync attempt is triggered immediately

```go
func triggerTimeSyncOnNetworkStateChange() {
    // Get NTP servers from DHCP
    ntpServers := networkManager.NTPServers()
    timeSync.SetDhcpNtpAddresses(ntpServers)

    // Trigger sync
    timeSync.Sync()
}
```

### mDNS

mDNS is restarted whenever network state changes to:
- Re-advertise with new IP addresses
- Update hostname if changed

```go
func restartMdns() {
    options := &mdns.MDNSOptions{
        LocalNames: []string{
            networkManager.Hostname(),  // e.g., "jetkvm-abc123"
            networkManager.FQDN(),      // e.g., "jetkvm-abc123.local"
        },
        ListenOptions: &mdns.MDNSListenOptions{
            IPv4: true,  // Based on mdns_mode config
            IPv6: true,
        },
    }
    mDNS.SetOptions(options)
}
```

### LCD Display

The device LCD shows current IP address. Display updates are triggered on network state changes.

### Cloud Connection

The WebSocket connection to the cloud may reconnect when network state changes.

## Reboot Requirements

Some network configuration changes require a device reboot:

| Change | Requires Reboot |
|--------|-----------------|
| DHCP client type | Yes |
| IPv4 mode (dhcp ↔ static) | Yes |
| IPv4 static config | Yes |
| IPv6 mode (with udhcpc) | Yes |
| Hostname | Yes |
| mDNS mode | No |
| Time sync settings | No |

The `shouldRebootForNetworkChange()` function determines if reboot is needed and provides post-reboot redirect info for the UI.

## RPC Endpoints

| Method | Description |
|--------|-------------|
| `getNetworkState` | Get current interface state |
| `getNetworkSettings` | Get network configuration |
| `setNetworkSettings` | Apply new configuration (may reboot) |
| `renewDHCPLease` | Force DHCP renewal |
| `getPublicIPAddresses` | Get detected public IPs |
| `checkPublicIPAddresses` | Force public IP refresh |

## Public IP Detection

For privacy reasons, public IP detection is only enabled when the device is adopted to JetKVM Cloud.

```go
func initPublicIPState() {
    ps := myip.NewPublicIPState(&myip.PublicIPStateConfig{
        CloudflareEndpoint: config.CloudURL,
        IPv4: false,  // Enabled when cloud-adopted
        IPv6: false,
    })
    publicIPState = ps
}
```

## Hostname and DNS

### Hostname Sources

1. **User-configured**: Set via network settings
2. **Default**: `jetkvm-<serial>` (from device serial number)

### resolv.conf Management

The `ResolvConfManager` handles `/etc/resolv.conf`:
- Writes hostname to `/etc/hostname`
- Manages DNS servers from DHCP and static config
- Handles domain search path

## HTTP Proxy

JetKVM supports HTTP proxy for outbound connections:

```json
{
  "network_config": {
    "http_proxy": "http://proxy.example.com:8080"
  }
}
```

The proxy is used for:
- Cloud API connections
- OTA update downloads
- Time sync HTTP fallback

## LLDP (Link Layer Discovery Protocol)

JetKVM can send LLDP frames to identify itself on the network:

| Mode | Description |
|------|-------------|
| `disabled` | No LLDP frames sent |
| `basic` | Send basic identification (default) |
| `all` | Send all available TLVs |

**TLV Types:**
- `chassis` - Chassis ID
- `port` - Port description
- `system` - System name/description
- `vlan` - VLAN information

## File Locations

| File | Purpose |
|------|---------|
| `network.go` | Integration layer |
| `pkg/nmlite/manager.go` | NetworkManager |
| `pkg/nmlite/interface.go` | InterfaceManager |
| `pkg/nmlite/interface_state.go` | State tracking |
| `pkg/nmlite/dhcp.go` | DHCP client wrapper |
| `pkg/nmlite/jetdhcpc/` | Native Go DHCP client |
| `pkg/nmlite/udhcpc/` | udhcpc process wrapper |
| `pkg/nmlite/static.go` | Static IP configuration |
| `pkg/nmlite/resolvconf.go` | resolv.conf management |
| `pkg/nmlite/hostname.go` | Hostname management |
| `pkg/nmlite/link/` | Netlink interface |
| `internal/network/types/` | Type definitions |

## Netlink Monitoring

The network manager uses Linux netlink to monitor interface changes:

```go
// pkg/nmlite/link/manager.go
type NetlinkManager struct {
    // Monitors for:
    // - Link state changes (up/down)
    // - Address changes (add/remove)
    // - Route changes
}
```

This provides real-time updates without polling.

## Example: DHCP Flow

```
1. Device boots, eth0 comes up
           ↓
2. InterfaceManager starts DHCPClient
           ↓
3. DHCPClient sends DHCP DISCOVER
           ↓
4. DHCP server responds with OFFER
           ↓
5. DHCPClient sends REQUEST
           ↓
6. DHCP server sends ACK with:
   - IP address
   - Subnet mask
   - Gateway
   - DNS servers
   - NTP servers (Option 42)
   - Lease time
           ↓
7. DHCPClient configures interface
           ↓
8. Netlink detects address change
           ↓
9. InterfaceManager updates state
           ↓
10. networkStateChanged() called:
    - Updates display
    - Notifies UI
    - Triggers time sync
    - Restarts mDNS
           ↓
11. Device is online and accessible
```

## Troubleshooting

### No IP Address

1. **Check cable connection**
   ```bash
   cat /sys/class/net/eth0/carrier  # Should be 1
   ```

2. **Check link state**
   ```bash
   ip link show eth0
   ```

3. **Check DHCP client**
   ```bash
   ps aux | grep -E "udhcpc|jetdhcpc"
   ```

4. **Check logs**
   ```bash
   grep network /var/log/jetkvm.log
   ```

### Wrong DNS Servers

Check `/etc/resolv.conf`:
```bash
cat /etc/resolv.conf
```

### Static IP Not Working

Verify configuration in `/userdata/kvm_config.json`:
- Correct `ipv4_mode: "static"`
- Valid `ipv4_static` object with all required fields

### Network Changes Require Reboot

Some changes (DHCP client, IPv4 mode, static IP) require reboot. The UI will prompt for confirmation.
