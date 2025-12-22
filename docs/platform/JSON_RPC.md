# JSON-RPC Usage

This document describes how JSON-RPC is used for device control and status updates, and where it sits alongside other message buses in the JetKVM stack.

## Transport and Lifecycle
- JSON-RPC runs over a reliable WebRTC data channel labeled `rpc` (`webrtc.go`).
- The channel is created during WebRTC session setup (local or cloud signaling).
- Messages are processed in-order via `session.rpcQueue`, then dispatched by `rpcHandlers` in `jsonrpc.go`.

## Message Format
Requests follow JSON-RPC 2.0 with `method`, `params`, and `id`:

```json
{ "jsonrpc": "2.0", "method": "getNetworkSettings", "params": {}, "id": 1 }
```

Responses return `result` or `error` with the same `id`:

```json
{ "jsonrpc": "2.0", "result": { "hostname": "jetkvm" }, "id": 1 }
```

The device also emits server-side events with `method` + `params` and no `id` (see "Events").

## Common Method Groups
The full method registry lives in `jsonrpc.go` (`rpcHandlers`). Highlights by area:
- **Device + Cloud:** `getDeviceID`, `getCloudState`, `deregisterDevice`, `getLocalLoopbackOnly`.
- **Network:** `getNetworkState`, `getNetworkSettings`, `setNetworkSettings`, `renewDHCPLease`.
- **Video + Display:** `getVideoState`, `getEDID`, `setEDID`, `getVideoSleepMode`, `setVideoSleepMode`, `setDisplayRotation`.
- **USB + Input:** `getUSBState`, `getUsbDevices`, `setUsbDevices`, `keyboardReport`, `absMouseReport`.
- **Virtual Media:** `mountWithHTTP`, `mountWithStorage`, `listStorageFiles`, `startStorageFileUpload`.
- **OTA:** `getUpdateStatus`, `tryUpdate`, `tryUpdateComponents`, `getLocalVersion`.

## Events (Server -> Client)
Events are emitted with `writeJSONRPCEvent` and are received by the UI as payloads with a `method` but no `id`.
Common events include:
- `otaState`, `otaProgress`
- `videoInputState`, `usbState`, `networkState`
- `failsafeMode`, `keyboardLedState`, `keysDownState`, `keyboardMacroState`
- `atxState`, `dcState`, `willReboot`, `otherSessionConnected`

## Adding or Changing Methods
1. Implement a `rpcX` function in the relevant Go file.
2. Register it in the `rpcHandlers` map in `jsonrpc.go` with parameter order.
3. Update the UI hook or callers (`ui/src/hooks/useJsonRpc.ts`) if new params or return shapes are added.
4. If the change affects device behavior or settings, ensure it persists to `/userdata/kvm_config.json` when needed.

## Failsafe Considerations
When failsafe mode is active, the UI blocks video-related RPC methods (see `ui/src/hooks/useJsonRpc.ts`). If you add new video-related RPCs, consider whether they should be blocked in failsafe mode.

## Other Message Buses in Use
JSON-RPC is not the only channel. The stack also uses:
- **HID-RPC (binary):** keyboard/mouse input over WebRTC data channels (`hidrpc.go`, `internal/hidrpc`).
- **WebRTC data channels (raw):** `terminal`, `serial`, and upload channels for shell access, serial passthrough, and file uploads.
- **gRPC (local IPC):** app <-> native process control via Unix sockets (`internal/native`).
- **WebSocket signaling:** WebRTC SDP/ICE exchange for local and cloud (`web.go`, `cloud.go`).
- **HTTP REST:** auth, setup, and device status endpoints (`web.go`).
- **SSE logs:** developer log streaming (`internal/logging`).
- **Prometheus metrics:** `/metrics` endpoint (`prometheus.go`).
