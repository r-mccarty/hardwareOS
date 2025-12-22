# Frontend Architecture Documentation

This document covers the JetKVM frontend UI architecture and how it communicates with the backend, intended for developers planning custom integrations or modifications.

## Tech Stack

- **React 19** with TypeScript
- **Vite** for bundling (no SSR framework)
- **React Router** for client-side routing
- **Zustand** for state management
- **Tailwind CSS 4** for styling
- **Headless UI** for accessible components
- **Framer Motion** for animations

## Backend Communication

The frontend uses three communication channels with the backend:

### 1. WebSocket + WebRTC Signaling

Initial connection establishment uses WebSocket for WebRTC signaling:

```
Device mode:  ws(s)://{device-host}/webrtc/signaling/client
Cloud mode:   wss://{cloud-api}/webrtc/signaling/client?id={device-id}
```

The WebSocket exchanges SDP offers/answers and ICE candidates to establish a peer-to-peer WebRTC connection.

### 2. JSON-RPC over WebRTC DataChannel

Primary device control uses JSON-RPC 2.0 over a reliable, ordered RTCDataChannel.

**Hook:** `useJsonRpc()` in `src/hooks/useJsonRpc.ts`

```typescript
// Usage
const { send } = useJsonRpc();
send("methodName", { param1: "value" }, (response) => {
  // Handle response
});
```

**Message Format:**
```json
{
  "jsonrpc": "2.0",
  "method": "getKeyboardMacros",
  "params": {},
  "id": 1
}
```

**Common RPC Methods:**
| Method | Purpose |
|--------|---------|
| `keyboardReport` | Send keyboard state (fallback) |
| `absMouseReport` / `relMouseReport` | Send mouse events (fallback) |
| `getKeyboardMacros` / `setKeyboardMacros` | Macro management |
| `getUSBState` | USB gadget status |
| `getNetworkSettings` | Network configuration |
| `setDisplayRotation` | Screen rotation |

### 3. HID-RPC (Binary Protocol)

High-performance binary protocol for keyboard/mouse input over WebRTC DataChannels.

**Hook:** `useHidRpc()` in `src/hooks/useHidRpc.ts`

**Channels:**
- `rpcHidChannel` - Reliable, ordered (keyboard events)
- `rpcHidUnreliableChannel` - Unreliable, ordered (mouse movement)
- `rpcHidUnreliableNonOrderedChannel` - Best effort (high-frequency updates)

**Methods:**
```typescript
const { reportKeypressEvent, reportAbsMouseEvent, reportRelMouseEvent } = useHidRpc();

// Keyboard
reportKeypressEvent(keycode, isPressed);

// Mouse (absolute mode)
reportAbsMouseEvent(x, y, buttons);

// Mouse (relative mode)
reportRelMouseEvent(deltaX, deltaY, buttons);
```

### 4. REST API (HTTP)

Used for authentication, device listing (cloud), and some configuration.

**Helper:** `src/api.ts`

```typescript
import { GET, POST } from "@/api";

// Cloud API
const devices = await GET(`${CLOUD_API}/devices`);

// Device API
const status = await GET("/device/status");
```

## State Management (Zustand Stores)

All stores are defined in `src/hooks/stores.ts`:

| Store | Purpose |
|-------|---------|
| `useRTCStore` | WebRTC connection, data channels, media stream |
| `useVideoStore` | Video dimensions, HDMI state, video element ref |
| `useMouseStore` | Mouse position tracking |
| `useHidStore` | Keyboard LED state, keys down, USB state |
| `useSettingsStore` | User preferences (persisted to localStorage) |
| `useUiStore` | UI state (sidebar, modals, terminal type) |
| `useMountMediaStore` | Virtual media mounting state |
| `useUpdateStore` | OTA update progress |
| `useMacrosStore` | Keyboard macros |
| `useNetworkStateStore` | Network configuration |
| `useUserStore` | Current user (cloud mode) |
| `useDeviceStore` | App/system versions |
| `useFailsafeModeStore` | Failsafe mode state |

### Key Store: useRTCStore

```typescript
const {
  peerConnection,      // RTCPeerConnection instance
  rpcDataChannel,      // JSON-RPC channel
  rpcHidChannel,       // HID-RPC channel
  mediaStream,         // Video stream
  peerConnectionState, // "connected", "disconnected", etc.
} = useRTCStore();
```

### Key Store: useSettingsStore (Persisted)

```typescript
const {
  mouseMode,          // "absolute" | "relative"
  keyboardLayout,     // Keyboard layout name
  debugMode,          // Show debug panel
  developerMode,      // Enable developer features
  displayRotation,    // Screen rotation
} = useSettingsStore();
```

## Routing Structure

Routes are defined in `src/main.tsx` and components live in `src/routes/`:

```
/                           → Redirects to /devices (cloud) or device UI
/login                      → Cloud login
/login-local                → Device local login
/welcome/*                  → Device setup wizard

/devices                    → Device list (cloud only)
/devices/:id                → Main KVM control interface
/devices/:id/mount          → Virtual media mounting
/devices/:id/settings/*     → Settings pages
  /general                  → General settings
  /mouse                    → Mouse configuration
  /keyboard                 → Keyboard layout
  /video                    → Video quality
  /hardware                 → Hardware settings
  /network                  → Network configuration
  /access                   → Access control
  /appearance               → Theme settings
  /macros                   → Macro management
```

## Key Components

### Video & Input (`src/components/`)

| Component | File | Purpose |
|-----------|------|---------|
| `WebRTCVideo` | `WebRTCVideo.tsx` | Video display, keyboard/mouse capture |
| `VirtualKeyboard` | `VirtualKeyboard.tsx` | On-screen keyboard |
| `Terminal` | `Terminal.tsx` | Serial/KVM console (xterm.js) |
| `ActionBar` | `ActionBar.tsx` | Video control toolbar |

### Layout

| Component | File | Purpose |
|-----------|------|---------|
| `Header` | `Header.tsx` | Top navigation, user menu |
| `Sidebar` | `sidebar/` | Side panel with stats |
| `Modal` | `Modal.tsx` | Generic modal dialog |

### Settings

| Component | Purpose |
|-----------|---------|
| `SettingsPageHeader` | Settings page title/description |
| `SettingsItem` | Individual setting row |
| `SettingsSectionHeader` | Section divider |

## Data Flow Example: Keyboard Input

```
1. User presses key
   ↓
2. WebRTCVideo.tsx onKeyDown event
   ↓
3. useKeyboard().handleKeyPress()
   ↓
4. Check if HID-RPC channel is ready
   ↓
5a. If ready: useHidRpc().reportKeypressEvent()
    → Binary message via rpcHidChannel
   ↓
5b. If not ready: useJsonRpc().send("keyboardReport", ...)
    → JSON-RPC via rpcDataChannel
   ↓
6. Backend processes input, sends to target machine
   ↓
7. Backend sends LED state update via HID-RPC
   ↓
8. useHidStore.setKeyboardLedState() updates
   ↓
9. Components re-render with new LED indicators
```

## Configuration

### Environment Variables (`src/ui.config.ts`)

```typescript
CLOUD_API    // Cloud backend URL (from VITE_CLOUD_API)
DEVICE_API   // Device API URL (empty = current host)
```

### Build Modes

```bash
npm run dev              # Device development (requires device IP)
npm run dev:cloud        # Cloud development mode
npm run build:device     # Production build for device
npm run build:prod       # Production build for cloud
```

## Integration Points

To integrate with a custom backend, you'll need to:

1. **WebRTC Signaling Endpoint**
   - Implement `/webrtc/signaling/client` WebSocket endpoint
   - Handle SDP offer/answer and ICE candidate exchange

2. **JSON-RPC Handler**
   - Accept JSON-RPC 2.0 messages over RTCDataChannel
   - Implement required methods (see `jsonrpc.go` in backend)

3. **HID-RPC Protocol** (optional, for performance)
   - Binary protocol for keyboard/mouse
   - See `internal/hidrpc/` in backend for message formats

4. **REST Endpoints**
   - `/device/status` - Device status check
   - `/device` - Device info
   - `/auth/*` - Authentication endpoints

## File Structure

```
ui/src/
├── main.tsx              # Entry point, router config
├── root.tsx              # Root outlet component
├── api.ts                # REST API helpers
├── ui.config.ts          # Environment config
├── index.css             # Tailwind base styles
├── routes/               # Page components
│   ├── devices.$id.tsx   # Main KVM control page
│   └── devices.$id.settings.*.tsx
├── hooks/
│   ├── stores.ts         # Zustand stores
│   ├── useJsonRpc.ts     # JSON-RPC hook
│   ├── useHidRpc.ts      # HID-RPC hook
│   ├── useKeyboard.ts    # Keyboard input
│   └── useMouse.ts       # Mouse input
├── components/           # Reusable UI components
└── localization/         # i18n messages
```

## Notes for Custom Integration

1. **All user-facing strings use localization** - Use `m.key_name()` functions, not hardcoded strings

2. **Device vs Cloud modes** - The UI adapts based on `isOnDevice` detection. Cloud mode adds device selection and authentication.

3. **Failsafe mode** - The UI has a failsafe mode that blocks certain operations. Check `useFailsafeModeStore`.

4. **Feature flags** - Version-gated features use `useFeatureFlag()` hook

5. **Video enhancements** - CSS filters for brightness/contrast/saturation are applied client-side

6. **Pointer lock** - In relative mouse mode, the browser's Pointer Lock API is used for seamless mouse capture
