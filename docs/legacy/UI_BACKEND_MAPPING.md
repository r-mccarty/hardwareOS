# UI to Backend Mapping

This document maps frontend UI features to their corresponding Go backend files. Use this to understand which backend code to modify when changing a feature.

## Overview

The frontend communicates with the backend via:
1. **JSON-RPC over WebRTC DataChannel** - Real-time bidirectional communication (primary)
2. **REST API** - HTTP endpoints for auth, file uploads, setup
3. **HID-RPC** - Binary protocol for high-frequency keyboard/mouse input

## Feature Mapping

### Video Streaming (Main KVM View)

| UI Component | Go File(s) | Description |
|--------------|------------|-------------|
| Video feed | `webrtc.go`, `internal/native/video.go` | H.264 video via WebRTC |
| Video quality slider | `jsonrpc.go` → `rpcSetStreamQualityFactor` | Adjusts encoding quality |
| Video state (resolution, fps) | `jsonrpc.go` → `rpcGetVideoState` | Returns current video info |
| EDID settings | `jsonrpc.go` → `rpcGetEDID`, `rpcSetEDID` | Custom EDID configuration |
| HDMI sleep mode | `video.go` → `rpcGetVideoSleepMode`, `rpcSetVideoSleepMode` | Auto-sleep on no signal |

### Keyboard & Mouse Input

| UI Component | Go File(s) | Description |
|--------------|------------|-------------|
| Keyboard input | `hidrpc.go` → `rpcKeyboardReport` | Sends HID keyboard reports |
| Key press/release | `hidrpc.go` → `rpcKeypressReport` | Single key events |
| Absolute mouse | `hidrpc.go` → `rpcAbsMouseReport` | Touch/tablet mode (x,y coords) |
| Relative mouse | `hidrpc.go` → `rpcRelMouseReport` | Traditional mouse (dx,dy deltas) |
| Mouse wheel | `hidrpc.go` → `rpcWheelReport` | Scroll events |
| Keyboard LED state | `hidrpc.go` → `rpcGetKeyboardLedState` | Caps/Num/Scroll lock indicators |
| Keyboard layout | `jsonrpc.go` → `rpcGetKeyboardLayout`, `rpcSetKeyboardLayout` | Layout selection |
| Keyboard macros | `jsonrpc.go` → `getKeyboardMacros`, `setKeyboardMacros` | Macro sequences |
| Execute macro | `jsonrpc.go` → `rpcExecuteKeyboardMacro` | Run a macro |
| Mouse jiggler | `jiggler.go` → `rpcSetJigglerState`, `rpcGetJigglerConfig` | Prevent screen sleep |

### Terminal (WebRTC Shell)

| UI Component | Go File(s) | Description |
|--------------|------------|-------------|
| Terminal session | `terminal.go` → `handleTerminalChannel` | PTY shell over WebRTC DataChannel |
| Terminal resize | `terminal.go` → `TerminalSize` struct | Sends `{rows, cols}` JSON |
| Shell process | `terminal.go` → `/bin/sh` | Spawns shell via `creack/pty` |

### Virtual Media (Mount ISO/IMG)

| UI Component | Go File(s) | Description |
|--------------|------------|-------------|
| Mount from URL | `virtual_media.go` → `rpcMountWithHTTP` | Stream ISO from HTTP |
| Mount from storage | `virtual_media.go` → `rpcMountWithStorage` | Mount uploaded file |
| Unmount | `virtual_media.go` → `rpcUnmountImage` | Eject virtual media |
| Storage mode (File/CD-ROM) | `jsonrpc.go` → `rpcSetMassStorageMode` | USB mass storage type |
| Virtual media state | `virtual_media.go` → `rpcGetVirtualMediaState` | Current mount status |
| Upload file | `web.go` → `handleUploadHttp` | HTTP multipart upload |
| List storage files | `virtual_media.go` → `rpcListStorageFiles` | Files in `/userdata/jetkvm/images/` |
| Delete storage file | `virtual_media.go` → `rpcDeleteStorageFile` | Remove uploaded file |
| Storage space | `virtual_media.go` → `rpcGetStorageSpace` | Disk usage info |

### USB Emulation Settings

| UI Component | Go File(s) | Description |
|--------------|------------|-------------|
| USB emulation toggle | `jsonrpc.go` → `rpcSetUsbEmulationState` | Enable/disable USB gadget |
| USB devices config | `jsonrpc.go` → `rpcGetUsbDevices`, `rpcSetUsbDevices` | Enable/disable keyboard, mouse, mass storage |
| USB config (VID/PID) | `jsonrpc.go` → `rpcGetUsbConfig`, `rpcSetUsbConfig` | Vendor/Product IDs |
| USB state | `usb.go` → `rpcGetUSBState` | Connection status |

### Network Settings

| UI Component | Go File(s) | Description |
|--------------|------------|-------------|
| Network state | `network.go` → `rpcGetNetworkState` | IP addresses, link status |
| Network settings | `network.go` → `rpcGetNetworkSettings`, `rpcSetNetworkSettings` | DHCP/static, IPv4/IPv6 |
| Renew DHCP | `network.go` → `rpcRenewDHCPLease` | Force lease renewal |
| Public IP | `network.go` → `rpcGetPublicIPAddresses` | External IP detection |
| mDNS | `mdns.go`, `network.go` → `restartMdns` | Local discovery |

### Display (Device LCD) Settings

| UI Component | Go File(s) | Description |
|--------------|------------|-------------|
| Display rotation | `jsonrpc.go` → `rpcSetDisplayRotation` | 0°, 90°, 180°, 270° |
| Backlight settings | `jsonrpc.go` → `rpcSetBacklightSettings` | Brightness, dim/off timers |
| Display updates | `display.go` → `requestDisplayUpdate` | LCD content refresh |

### Authentication & Setup

| UI Component | Go File(s) | Description |
|--------------|------------|-------------|
| Login page | `web.go` → `handleLogin` | POST `/auth/login-local` |
| Logout | `web.go` → `handleLogout` | POST `/auth/logout` |
| Device setup | `web.go` → `handleSetup` | POST `/device/setup` |
| Device status | `web.go` → `handleDeviceStatus` | GET `/device/status` |
| Create password | `web.go` → `handleCreatePassword` | POST `/auth/password-local` |
| Change password | `web.go` → `handleUpdatePassword` | PUT `/auth/password-local` |
| Delete password | `web.go` → `handleDeletePassword` | DELETE `/auth/local-password` |

### Cloud Connection

| UI Component | Go File(s) | Description |
|--------------|------------|-------------|
| Cloud state | `cloud.go` → `rpcGetCloudState` | Connection status |
| Cloud register | `cloud.go` → `handleCloudRegister` | POST `/cloud/register` |
| Deregister | `cloud.go` → `rpcDeregisterDevice` | Remove from cloud |
| Cloud URL config | `jsonrpc.go` → `rpcSetCloudUrl` | API endpoint override |

### Developer Mode & SSH

| UI Component | Go File(s) | Description |
|--------------|------------|-------------|
| Developer mode toggle | `jsonrpc.go` → `rpcSetDevModeState` | Creates `/userdata/jetkvm/devmode.enable` |
| SSH key management | `jsonrpc.go` → `rpcGetSSHKeyState`, `rpcSetSSHKeyState` | Authorized keys |
| pprof endpoints | `web.go` → `/developer/pprof/*` | Performance profiling |

### TLS/HTTPS Settings

| UI Component | Go File(s) | Description |
|--------------|------------|-------------|
| TLS state | `jsonrpc.go` → `rpcGetTLSState` | Enabled, mode, cert info |
| TLS config | `jsonrpc.go` → `rpcSetTLSState` | Enable/disable, upload certs |
| HTTPS server | `web_tls.go` → `RunWebSecureServer` | Port 443 server |

### OTA Updates

| UI Component | Go File(s) | Description |
|--------------|------------|-------------|
| Check for updates | `ota.go` → `rpcCheckUpdateComponents` | Query update server |
| Current version | `ota.go` → `rpcGetLocalVersion` | App/system versions |
| Update status | `ota.go` → `rpcGetUpdateStatus` | Download progress |
| Trigger update | `ota.go` → `rpcTryUpdate`, `rpcTryUpdateComponents` | Start OTA |
| Auto-update toggle | `jsonrpc.go` → `rpcSetAutoUpdateState` | Enable/disable |
| Dev channel toggle | `ota.go` → `rpcSetDevChannelState` | Pre-release updates |

### Hardware Extensions

| UI Component | Go File(s) | Description |
|--------------|------------|-------------|
| Active extension | `jsonrpc.go` → `rpcGetActiveExtension`, `rpcSetActiveExtension` | ATX/DC power |
| **ATX Power Control** | | |
| ATX state (LED) | `atx.go` → `rpcGetATXState` | Power/HDD LED status |
| ATX power action | `atx.go` → `rpcSetATXPowerAction` | Short/long press, reset |
| **DC Power Control** | | |
| DC power state | `dc.go` → `rpcGetDCPowerState` | Voltage, current, power |
| DC power toggle | `dc.go` → `rpcSetDCPowerState` | On/off |
| DC restore state | `dc.go` → `rpcSetDCRestoreState` | Power-on behavior |

### Serial Console

| UI Component | Go File(s) | Description |
|--------------|------------|-------------|
| Serial settings | `jsonrpc.go` → `rpcGetSerialSettings`, `rpcSetSerialSettings` | Baud, parity, etc. |
| Serial data | `serial.go` → via WebRTC DataChannel | TTY I/O |

### Wake-on-LAN

| UI Component | Go File(s) | Description |
|--------------|------------|-------------|
| WOL devices list | `jsonrpc.go` → `rpcGetWakeOnLanDevices` | Saved MAC addresses |
| Save WOL devices | `jsonrpc.go` → `rpcSetWakeOnLanDevices` | Update device list |
| Send WOL packet | `wol.go` → `rpcSendWOLMagicPacket` | Magic packet |
| WOL HTTP endpoint | `web.go` → `handleSendWOLMagicPacket` | POST `/device/send-wol/:mac-addr` |

### System Operations

| UI Component | Go File(s) | Description |
|--------------|------------|-------------|
| Reboot | `jsonrpc.go` → `rpcReboot` | Device restart |
| Reset config | `jsonrpc.go` → `rpcResetConfig` | Factory defaults |
| Device ID | `jsonrpc.go` → `rpcGetDeviceID` | Serial number |
| Diagnostics | `jsonrpc.go` → `rpcGetDiagnostics` | System info dump |
| Ping | `jsonrpc.go` → `rpcPing` | Health check |

### Settings Pages

| Settings Route | Primary Go Files | Key RPC Methods |
|----------------|------------------|-----------------|
| General | `config.go`, `ota.go` | `getLocalVersion`, `getAutoUpdateState` |
| Video | `video.go`, `jsonrpc.go` | `getStreamQualityFactor`, `getEDID` |
| Mouse | `hidrpc.go`, `jsonrpc.go` | `getJigglerState`, `getUsbDevices` |
| Keyboard | `hidrpc.go`, `jsonrpc.go` | `getKeyboardLayout`, `getKeyboardMacros` |
| Network | `network.go` | `getNetworkSettings`, `getNetworkState` |
| Hardware | `atx.go`, `dc.go`, `jsonrpc.go` | `getActiveExtension`, `getATXState` |
| Access | `web.go`, `jsonrpc.go` | `getDevModeState`, `getTLSState` |
| Appearance | `display.go`, `jsonrpc.go` | `getBacklightSettings`, `getDisplayRotation` |
| Advanced | `jsonrpc.go` | `getUsbConfig`, `getLocalLoopbackOnly` |
| Macros | `jsonrpc.go` | `getKeyboardMacros`, `setKeyboardMacros` |

## WebRTC DataChannel Types

| Channel Label | Handler | Purpose |
|---------------|---------|---------|
| `rpc` | `jsonrpc.go` → `onRPCMessage` | JSON-RPC commands |
| `hid` | `hidrpc.go` | Binary HID reports |
| `terminal` | `terminal.go` → `handleTerminalChannel` | Shell I/O |
| `disk` | `virtual_media.go` | Virtual media data |
| `serial` | `serial.go` | Serial console I/O |

## REST API Endpoints

| Endpoint | Method | Handler | Auth Required |
|----------|--------|---------|---------------|
| `/auth/login-local` | POST | `handleLogin` | No |
| `/device/status` | GET | `handleDeviceStatus` | No |
| `/device/setup` | POST | `handleSetup` | No |
| `/metrics` | GET | Prometheus | No |
| `/webrtc/session` | POST | `handleWebRTCSession` | Yes |
| `/webrtc/signaling/client` | GET | `handleLocalWebRTCSignal` | Yes |
| `/cloud/register` | POST | `handleCloudRegister` | Yes |
| `/cloud/state` | GET | `handleCloudState` | Yes |
| `/device` | GET | `handleDevice` | Yes |
| `/auth/logout` | POST | `handleLogout` | Yes |
| `/auth/password-local` | POST/PUT/DELETE | Password management | Yes |
| `/storage/upload` | POST | `handleUploadHttp` | Yes |
| `/device/send-wol/:mac-addr` | POST | `handleSendWOLMagicPacket` | Yes |
| `/developer/pprof/*` | GET | pprof handlers | Basic Auth + Dev Mode |

## File Quick Reference

| File | Primary Purpose |
|------|-----------------|
| `web.go` | HTTP server, REST endpoints, auth middleware |
| `jsonrpc.go` | JSON-RPC handler dispatch, all `rpc*` functions |
| `webrtc.go` | WebRTC peer connection, session management |
| `hidrpc.go` | Binary HID protocol for keyboard/mouse |
| `terminal.go` | WebRTC-based PTY terminal |
| `virtual_media.go` | ISO/IMG mounting, HTTP streaming |
| `network.go` | Network manager integration |
| `cloud.go` | Cloud API WebSocket connection |
| `display.go` | LCD display control |
| `video.go` | Video streaming state |
| `ota.go` | OTA update logic |
| `config.go` | Configuration load/save |
| `usb.go` | USB gadget state |
| `atx.go` | ATX power control extension |
| `dc.go` | DC power control extension |
| `wol.go` | Wake-on-LAN packets |
| `serial.go` | Serial console |
| `jiggler.go` | Mouse jiggler feature |
