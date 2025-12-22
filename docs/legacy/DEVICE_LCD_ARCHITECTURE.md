# Device LCD/Touchscreen UI Architecture

This document covers how the JetKVM device's physical LCD and touchscreen work, including the LVGL graphics library, EEZ Studio integration, and the Go↔C communication bridge.

## Overview

The device LCD is a 240x300 pixel touchscreen driven by:
- **LVGL** (Light and Versatile Graphics Library) - C graphics framework
- **EEZ Studio** - Visual UI designer that generates C code
- **CGO** - Go's C Foreign Function Interface for Go↔C communication
- **Linux Framebuffer** (`/dev/fb0`) - Direct hardware access

```
┌─────────────────────────────────────┐
│  Go Application                     │
│  - State management                 │
│  - Screen switching logic           │
│  - Label/status updates             │
└──────────────┬──────────────────────┘
               │ CGO calls
               ▼
┌─────────────────────────────────────┐
│  C/LVGL Layer                       │
│  - LVGL rendering engine            │
│  - EEZ-generated UI screens         │
│  - Touchscreen input handling       │
└──────────────┬──────────────────────┘
               │ Direct I/O
               ▼
┌─────────────────────────────────────┐
│  Hardware                           │
│  - /dev/fb0 (framebuffer)           │
│  - /dev/input/event* (touchscreen)  │
└─────────────────────────────────────┘
```

## CGO Bridge

### File Locations

| File | Purpose |
|------|---------|
| `internal/native/cgo_linux.go` | Go↔C bridge with CGO directives |
| `internal/native/cgo/screen.c` | LVGL initialization and control |
| `internal/native/cgo/ui_index.h` | Object name-to-pointer mapping |

### How Go Calls C

The CGO bridge in `cgo_linux.go` links against the native library:

```c
#cgo LDFLAGS: -Lcgo/lib -ljknative -llvgl
#cgo CFLAGS: -Icgo/include
```

All CGO calls are protected by a mutex for thread safety:

```go
var cgoLock sync.Mutex

func uiLabelSetText(objName string, text string) (bool, error) {
    cgoLock.Lock()
    defer cgoLock.Unlock()

    result := C.jetkvm_ui_set_text(C.CString(objName), C.CString(text))
    return result != 0, nil
}
```

### Key Go→C Functions

| Go Function | C Function | Purpose |
|-------------|------------|---------|
| `uiInit(rotation)` | `jetkvm_ui_init()` | Initialize LVGL with rotation |
| `uiTick()` | `jetkvm_ui_tick()` | Process LVGL frame (called every 5ms) |
| `uiSetVar(name, value)` | `jetkvm_ui_set_var()` | Set UI state variable |
| `uiSwitchToScreen(name)` | `jetkvm_ui_load_screen()` | Switch active screen |
| `uiLabelSetText(obj, text)` | `jetkvm_ui_set_text()` | Update label text |
| `uiObjAddState(obj, state)` | `jetkvm_ui_add_state()` | Add LVGL state (e.g., checked) |
| `uiObjFadeIn/Out(obj, ms)` | `jetkvm_ui_fade_in/out()` | Animate opacity |
| `uiDispSetRotation(deg)` | `jetkvm_ui_set_rotation()` | Rotate display |

### C→Go Callbacks

C code notifies Go of events through registered callbacks:

| Callback | Channel | Purpose |
|----------|---------|---------|
| `jetkvm_go_indev_handler` | `indevEventChan` | Touchscreen events |
| `jetkvm_go_rpc_handler` | `rpcEventChan` | UI button actions |
| `jetkvm_go_video_state_handler` | `videoStateChan` | HDMI state changes |

## LVGL Integration

### Initialization (`screen.c`)

```c
void lvgl_init(u_int16_t rotation) {
    lv_init();

    // Create framebuffer display driver
    disp = lv_linux_fbdev_create();
    lv_display_set_resolution(disp, 240, 300);
    lv_linux_fbdev_set_file(disp, "/dev/fb0");

    lvgl_set_rotation(disp, rotation);

    // Discover touchscreen devices
    lv_evdev_discovery_start(evdev_discovery_cb, NULL);

    // Initialize EEZ-generated UI
    ui_init();
}
```

### Frame Update Loop

Go runs LVGL's timer handler every 5ms (~200 FPS):

```go
// internal/native/display.go
func (n *Native) tickUI() {
    for {
        uiTick()
        time.Sleep(5 * time.Millisecond)
    }
}
```

```c
// screen.c
void lvgl_tick(void) {
    lv_timer_handler();  // Process animations, redraws
    ui_tick();           // EEZ tick function
}
```

### Object Lookup by Name

Objects are accessed by string name through a lookup table:

```c
// ui_index.h
typedef struct {
    const char *name;
    lv_obj_t **obj;
} ui_obj_map;

extern ui_obj_map ui_objects[];

// screen.c
lv_obj_t *ui_get_obj(const char *name) {
    for (size_t i = 0; i < ui_objects_size; i++) {
        if (strcmp(ui_objects[i].name, name) == 0) {
            return *ui_objects[i].obj;
        }
    }
    return NULL;
}
```

This allows Go to update UI elements dynamically:

```go
nativeInstance.UpdateLabelIfChanged("home_info_ipv4_addr", "192.168.1.100")
```

## EEZ Studio

### What EEZ Studio Does

EEZ Studio is a visual UI designer that generates C code for LVGL. The workflow:

1. Design screens visually in EEZ Studio GUI
2. Define object names, styles, and actions
3. Export generates C files in `internal/native/eez/src/ui/`
4. C code is compiled into the native binary

### Generated Files

| File | Size | Purpose |
|------|------|---------|
| `screens.c` | 154KB | Screen layouts and object creation |
| `screens.h` | - | Screen enums, object pointers, function declarations |
| `styles.c` | 17KB | LVGL style definitions |
| `vars.c` | - | UI variable getters/setters |
| `actions.c` | 6KB | Button/gesture event handlers |
| `images.c` | - | Image asset definitions |

### Defined Screens

| Screen | Purpose |
|--------|---------|
| `boot_screen` | Initial loading |
| `home_screen` | Main status display |
| `no_network_screen` | Ethernet disconnected |
| `menu_screen` | Main menu |
| `menu_advanced_screen` | Advanced settings menu |
| `menu_network_screen` | Network settings menu |
| `about_screen` | Device info (versions, serial) |
| `status_screen` | Detailed status |
| `reset_config_screen` | Config reset confirmation |
| `reboot_screen` | Reboot confirmation |
| `rebooting_screen` | Reboot in progress |

## Touchscreen Input

### Hardware Path

```
Touch hardware → /dev/input/eventX → LVGL evdev driver → Go callback
```

### Event Discovery

LVGL automatically discovers touchscreen devices:

```c
static void evdev_discovery_cb(lv_indev_t *indev, lv_evdev_type_t type, void *user_data) {
    if (type != LV_EVDEV_TYPE_ABS) return;  // Only absolute position (touchscreen)

    lv_indev_set_display(indev, disp);
    lv_indev_add_event_cb(indev, handle_indev_event, LV_EVENT_ALL, NULL);
}
```

### Event Flow

```c
// screen.c
static void handle_indev_event(lv_event_t *e) {
    lv_event_code_t code = lv_event_get_code(e);
    jetkvm_indev_handler((int)code);  // Calls Go callback
}
```

```go
// cgo_linux.go
//export jetkvm_go_indev_handler
func jetkvm_go_indev_handler(code C.int) {
    indevEventChan <- int(code)
}
```

### Event Types

| LVGL Event | Meaning |
|------------|---------|
| `LV_EVENT_PRESSED` | Touch down |
| `LV_EVENT_RELEASED` | Touch up |
| `LV_EVENT_SHORT_CLICKED` | Tap |
| `LV_EVENT_LONG_PRESSED` | Long press (1+ second) |
| `LV_EVENT_GESTURE` | Swipe detected |

## UI Actions (RPC)

When users tap buttons, EEZ-generated action handlers call Go:

### Action Flow

```
User taps "Reset Config"
    ↓
EEZ action: action_reset_config()
    ↓
C: ui_call_rpc_handler("resetConfig", "{}")
    ↓
CGO: jetkvm_go_rpc_handler("resetConfig")
    ↓
Go: rpcEventChan <- "resetConfig"
    ↓
Go handler: rpcResetConfig()
```

### Registered Actions

| Button | RPC Method | Go Handler |
|--------|------------|------------|
| Reset Config | `resetConfig` | `rpcResetConfig()` |
| Reboot | `reboot` | `rpcReboot(true)` |
| Toggle DHCP | `toggleDHCPClient` | `rpcToggleDHCPClient()` |

### Handler Registration (`native.go`)

```go
OnRpcEvent: func(event string) {
    switch event {
    case "resetConfig":
        rpcResetConfig()
    case "reboot":
        rpcReboot(true)
    case "toggleDHCPClient":
        rpcToggleDHCPClient()
    }
}
```

## Display Management (Go Side)

### File: `display.go`

The Go application manages high-level display logic:

### Screen Switching

```go
func switchToMainScreen() {
    if networkManager.IsUp() {
        nativeInstance.SwitchToScreenIfDifferent("home_screen")
    } else {
        nativeInstance.SwitchToScreenIfDifferent("no_network_screen")
    }
}
```

### Label Updates

```go
func updateDisplay() {
    // Update network info
    nativeInstance.UpdateLabelIfChanged("home_info_ipv4_addr", networkManager.IPv4String())
    nativeInstance.UpdateLabelIfChanged("home_info_mac_addr", networkManager.MACString())

    // Update status indicators
    nativeInstance.UpdateLabelIfChanged("usb_status_label", usbStateText)
    nativeInstance.UpdateLabelIfChanged("hdmi_status_label", hdmiStateText)
}
```

### Backlight Control

```go
// Brightness via sysfs
func setDisplayBrightness(brightness int, reason string) {
    os.WriteFile("/sys/class/backlight/backlight/brightness",
                 []byte(strconv.Itoa(brightness)), 0644)
}

// Wake on activity
func wakeDisplay(force bool, reason string) {
    setDisplayBrightness(100, reason)
    resetDimTimer()
}
```

### Cloud Status Animation

```go
func doCloudBlink(ctx context.Context) {
    for range cloudBlinkTicker.C {
        if cloudConnectionState != CloudConnectionStateConnecting {
            continue
        }
        nativeInstance.UIObjFadeOut("ui_Home_Header_Cloud_Status_Icon", 1000)
        time.Sleep(1 * time.Second)
        nativeInstance.UIObjFadeIn("ui_Home_Header_Cloud_Status_Icon", 1000)
        time.Sleep(1 * time.Second)
    }
}
```

## Complete Data Flow Example

### Updating IPv4 Address on Display

```
1. Network interface comes online
   ↓
2. Go: requestDisplayUpdate(true, "network_up")
   ↓
3. Go: updateDisplay()
   ↓
4. Go: nativeInstance.UpdateLabelIfChanged("home_info_ipv4_addr", "192.168.1.100")
   ↓
5. CGO: C.jetkvm_ui_set_text("home_info_ipv4_addr", "192.168.1.100")
   ↓
6. C: ui_get_obj("home_info_ipv4_addr") → lv_obj_t*
   ↓
7. C: lv_label_set_text(obj, "192.168.1.100")
   ↓
8. LVGL marks object dirty
   ↓
9. Next lvgl_tick() → redraws to /dev/fb0
   ↓
10. LCD controller DMA → physical display updates
```

## Building the Native Code

### Prerequisites

The native build requires the JetKVM buildkit (ARM cross-compiler):

```bash
# Buildkit location
/opt/jetkvm-native-buildkit/

# Or build in Docker
make build_dev  # Automatically uses Docker if buildkit not found
```

### Build Commands

```bash
# Build native library only
make build_native

# Build with debug symbols
CMAKE_BUILD_TYPE=Debug make build_native
```

### Symlink Requirement

The EEZ UI code requires a symlink:

```bash
# internal/native/cgo/ui must point to ../eez/src/ui
cd internal/native/cgo
ls -la ui  # Should show: ui -> ../eez/src/ui

# If broken, recreate:
rm ui && ln -s ../eez/src/ui ui
```

## Key Files Reference

| File | Purpose |
|------|---------|
| `display.go` | Go display management (backlight, updates) |
| `native.go` | Native process initialization, RPC handlers |
| `internal/native/cgo_linux.go` | CGO bridge, Go↔C function wrappers |
| `internal/native/cgo/screen.c` | LVGL init, tick, object access |
| `internal/native/cgo/ui_index.h` | Object name mapping |
| `internal/native/eez/src/ui/screens.c` | EEZ-generated screen definitions |
| `internal/native/eez/src/ui/actions.c` | EEZ-generated button handlers |
| `internal/native/eez/jetkvm.eez-project` | EEZ Studio project file |

## Adding New UI Elements

To add a new UI element:

1. **Design in EEZ Studio** - Open `jetkvm.eez-project`, add objects visually
2. **Export/Build** - Click Build in EEZ Studio to regenerate C code
3. **Rebuild native** - `make build_native`
4. **Access from Go** - Use the object name in Go code:

```go
nativeInstance.UpdateLabelIfChanged("my_new_label", "Hello World")
nativeInstance.UIObjAddState("my_new_button", "LV_STATE_CHECKED")
```

## Architecture Notes

1. **Thread Safety** - All CGO calls use `cgoLock` mutex to prevent concurrent C access
2. **Change Detection** - `UpdateLabelIfChanged` only redraws if text actually changed
3. **200 FPS Tick** - 5ms tick interval ensures smooth animations
4. **String-Based Access** - Objects accessed by name for flexibility without recompilation
5. **Separation of Concerns** - Go handles state/logic, C handles real-time rendering
