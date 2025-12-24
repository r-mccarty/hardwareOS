# Camera Hardware-in-the-Loop Integration Plan

Comprehensive integration tracking for RS-1 vision pipeline testing with multiple camera sensors.

---

## HIL Test Environment Setup

### Infrastructure Overview

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           N100 Node (Coder Workspace)                        │
│                           coder.hardwareos.com                               │
├─────────────────────────────────────────────────────────────────────────────┤
│  • Go 1.24.4, Node.js 22.x                                                  │
│  • rkdeveloptool, adb, picocom                                              │
│  • Infisical secrets pre-loaded                                             │
│  • GitHub CLI authenticated                                                  │
└───────────────────────────┬─────────────────────────────────────────────────┘
                            │
              Ethernet (enp1s0 ↔ eth0)
              192.168.1.1 ↔ 192.168.1.100
                            │
┌───────────────────────────┴─────────────────────────────────────────────────┐
│                        Luckfox Pico Max (RV1106G3)                          │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│   ┌─────────────┐    ┌─────────────┐    ┌─────────────┐                    │
│   │ USB-C Port  │    │ 20-pin CSI  │    │ GPIO Header │                    │
│   │ (Host Mode) │    │  Connector  │    │  (Power In) │                    │
│   └──────┬──────┘    └──────┬──────┘    └──────┬──────┘                    │
│          │                  │                  │                            │
│          ▼                  ▼                  ▼                            │
│   ┌────────────┐    ┌────────────┐    ┌────────────────┐                   │
│   │ USB Camera │    │ MIPI CSI   │    │ 5V Power Supply│                   │
│   │ (via OTG)  │    │  Camera    │    │ (Pin 39 + GND) │                   │
│   └────────────┘    └────────────┘    └────────────────┘                   │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Luckfox Pico Max USB Host Mode

The Luckfox USB-C port is **device-only by default** (for ADB/RNDIS). To connect USB cameras,
switch to **host mode**:

#### Prerequisites

Before switching to host mode, you must set up an alternative connection since RNDIS will be lost:

1. **Connect Ethernet cable** between N100 (`enp1s0`) and Luckfox (`eth0`)
2. **Configure N100 ethernet**:
   ```bash
   sudo ip addr add 192.168.1.1/24 dev enp1s0
   ```
3. **Configure Luckfox ethernet** (via ADB while still in device mode):
   ```bash
   sudo adb shell
   # On Luckfox:
   ip addr add 192.168.1.100/24 dev eth0
   ip link set eth0 up
   ```

#### Enable USB Host Mode

```bash
# On Luckfox (via ADB or SSH)
luckfox-config
# Navigate: Advanced Options → USB → select "host"
# Reboot
```

#### After Host Mode Enabled

| Connection | Status |
|------------|--------|
| USB RNDIS | ❌ Lost (expected) |
| ADB | ❌ Lost (expected) |
| Ethernet SSH | ✅ Use this |
| Serial console | ✅ Backup option |

### Power Supply via GPIO Header

When USB-C is in host mode, power the board via GPIO header pins:

#### Pinout

| Pin | Name | Description |
|-----|------|-------------|
| 36 | VBUS | 5V from USB (output only when in device mode) |
| 38 | GND | Ground |
| **39** | **VSYS** | **5V input (recommended for external power)** |
| 40 | GND | Ground |

#### Wiring Diagram

```
5V USB Power Adapter                    Luckfox Pico Max
┌─────────────────┐                    ┌─────────────────┐
│                 │                    │   GPIO Header   │
│    +5V (Red) ───┼────────────────────┼──► Pin 39 (VSYS)│
│                 │                    │                 │
│    GND (Black)──┼────────────────────┼──► Pin 38 (GND) │
│                 │                    │                 │
└─────────────────┘                    └─────────────────┘

Requirements:
• 5V 2A USB power adapter (or better)
• Dupont jumper wires or spliced USB cable
```

> **Warning**: Double-check polarity before connecting. Reversing power will damage the board.

### Complete HIL Wiring

```
                                    ┌─────────────────────────────────┐
┌──────────────┐                    │      Luckfox Pico Max          │
│    N100      │    Ethernet        │                                 │
│              ├────────────────────┤ eth0 (192.168.1.100)           │
│  (enp1s0)    │  192.168.1.x/24    │                                 │
│192.168.1.1   │                    │  ┌───────────────────────────┐ │
└──────────────┘                    │  │ USB-C (Host Mode)         │ │
                                    │  │         │                 │ │
┌──────────────┐                    │  │    ┌────┴────┐            │ │
│ 5V 2A Power  │   Pin 39 (VSYS)    │  │    │USB-C to │            │ │
│   Adapter    ├────────────────────┤──┼───►│USB-A OTG│            │ │
│              │   Pin 38 (GND)     │  │    │ Adapter │            │ │
│   GND ───────┼────────────────────┤──┼────┴────┬────┘            │ │
└──────────────┘                    │  │         │                 │ │
                                    │  └─────────┼─────────────────┘ │
                                    │            │                   │
                                    │     ┌──────┴──────┐            │
                                    │     │ USB Camera  │            │
                                    │     │ (Innomaker) │            │
                                    │     └─────────────┘            │
                                    │                                 │
                                    │  ┌───────────────────────────┐ │
                                    │  │ 20-pin CSI Connector      │ │
                                    │  │         │                 │ │
                                    │  │    ┌────┴────┐            │ │
                                    │  │    │  MIPI   │            │ │
                                    │  │    │ Camera  │            │ │
                                    │  │    │(SC3336) │            │ │
                                    │  │    └─────────┘            │ │
                                    │  └───────────────────────────┘ │
                                    └─────────────────────────────────┘
```

### Bill of Materials (HIL Setup)

| Item | Qty | Purpose | Source |
|------|-----|---------|--------|
| USB-C to USB-A OTG adapter | 1 | Connect USB camera to Luckfox | Amazon (~$5) |
| 5V 2A USB power adapter | 1 | Power Luckfox via GPIO | Any USB charger |
| Dupont jumper wires (F-F) | 2 | Connect power to GPIO header | Electronics store |
| Ethernet cable (Cat5e+) | 1 | N100 ↔ Luckfox connection | Any length needed |
| Innomaker 720P USB camera | 1 | USB camera testing | Amazon (~$17) |

### Quick Start: USB Camera HIL Test

```bash
# 1. On N100 (Coder workspace) - Configure ethernet
sudo ip addr add 192.168.1.1/24 dev enp1s0

# 2. On Luckfox (via ADB) - Configure ethernet before switching modes
sudo adb shell 'ip addr add 192.168.1.100/24 dev eth0 && ip link set eth0 up'

# 3. Test ethernet connectivity
ping 192.168.1.100

# 4. On Luckfox - Enable USB host mode
sudo adb shell 'luckfox-config'
# Select: Advanced Options → USB → host → OK → Reboot

# 5. Power off, connect:
#    - 5V power to Pin 39 (VSYS) + Pin 38 (GND)
#    - USB camera via OTG adapter to USB-C port
#    - Ethernet cable

# 6. Power on and verify camera
ssh root@192.168.1.100
v4l2-ctl --list-devices
v4l2-ctl --device=/dev/video0 --list-formats-ext
```

---

## Hardware Inventory

### Development Boards

| Board | Qty | Status | Notes |
|-------|-----|--------|-------|
| Luckfox Pico Pro/Max | 2 | 🚢 Ordered | Waveshare, ~2-3 weeks |

### Camera Sensors

| Sensor | Qty | Interface | Status | ETA |
|--------|-----|-----------|--------|-----|
| Innomaker 720P USB | 1 | USB UVC | 🚢 Ordered | Tomorrow |
| SC3336 3MP | 2 | MIPI CSI | 🚢 Ordered | ~2-3 weeks |
| MIS5001 5MP Wide | 1 | MIPI CSI | 🚢 Ordered | ~2-3 weeks |
| IMX415 8MP (A) | 1 | MIPI CSI | 🚢 Ordered | ~2-3 weeks |
| IMX415 8MP (B) | 1 | MIPI CSI | 🚢 Ordered | ~2-3 weeks |
| ISG1321 1.38MP Mono | 1 | MIPI CSI | 🚢 Ordered | ~2-3 weeks |

### Accessories

| Item | Qty | Status | Notes |
|------|-----|--------|-------|
| 1.69" LCD Display | 1 | 🚢 Ordered | For on-device UI |
| USB-C to USB-A OTG adapter | 1 | 📋 Needed | For USB camera connection |
| 5V 2A USB power adapter | 1 | 📋 Needed | Power via GPIO when USB-C is host |
| Dupont jumper wires (F-F) | 2 | 📋 Needed | Connect power to GPIO header |
| Ethernet cable | 1 | 📋 Needed | N100 ↔ Luckfox connection |

---

## Integration Track 1: Innomaker USB 720P (Immediate)

**Purpose**: Validate V4L2 → NPU → Fusion pipeline without MIPI CSI dependencies

### Specifications

| Spec | Value |
|------|-------|
| Resolution | 1280 × 720 |
| FOV | 120° diagonal (~100° horizontal) |
| Interface | USB 2.0 UVC |
| Pixel Format | YUYV / MJPEG |
| Frame Rate | 30 FPS |

### Integration Checklist

- [ ] **Hardware Setup**
  - [ ] Camera arrives and physical inspection
  - [ ] Connect to Luckfox USB port
  - [ ] Verify power draw acceptable

- [ ] **V4L2 Bring-up**
  - [ ] Verify device appears (`v4l2-ctl --list-devices`)
  - [ ] Query supported formats (`v4l2-ctl --list-formats-ext`)
  - [ ] Test frame capture (`v4l2-ctl --stream-mmap`)
  - [ ] Measure actual frame rate

- [ ] **Pipeline Integration**
  - [ ] Create USB camera capture path (bypass ISP)
  - [ ] Configure FOV: `horizontal_fov_deg = 100.0`
  - [ ] Disable LDCH (not available for USB)
  - [ ] Test NV12/YUYV → RGB conversion for NPU

- [ ] **NPU Validation**
  - [ ] YOLOv8 inference runs successfully
  - [ ] Measure inference latency
  - [ ] Verify detection output format
  - [ ] Test person detection accuracy

- [ ] **Fusion Integration**
  - [ ] Vision detections reach fusion engine
  - [ ] Azimuth calculations correct (within FOV tolerance)
  - [ ] Radar + vision fusion produces fused tracks

- [ ] **WebRTC Streaming**
  - [ ] Software encode path works (no MPP)
  - [ ] Video visible in web UI
  - [ ] Latency acceptable for debugging

### Configuration Changes

```c
// For USB camera path
#define USB_CAMERA_WIDTH  1280
#define USB_CAMERA_HEIGHT 720
#define USB_CAMERA_FOV_H  100.0f  // Adjusted for diagonal spec
```

### Success Criteria

| Metric | Target | Notes |
|--------|--------|-------|
| Frame capture | 30 FPS | V4L2 MMAP |
| NPU inference | < 50ms | May be slower without RGA |
| End-to-end latency | < 200ms | Acceptable for HIL |
| Detection accuracy | > 80% | Person class |

---

## Integration Track 2: SC3336 3MP (Production Sensor)

**Purpose**: Production-equivalent testing with full ISP pipeline

### Specifications

| Spec | Value |
|------|-------|
| Resolution | 2304 × 1296 |
| Pixel Size | ~2.5 µm |
| FOV | ~120° horizontal (wide lens) |
| Interface | MIPI CSI-2, 2 lanes |
| Pixel Format | RAW Bayer (RGGB) |
| Frame Rate | 30 FPS |

### Integration Checklist

- [ ] **Hardware Setup**
  - [ ] Camera arrives and physical inspection
  - [ ] Connect to Luckfox 20-pin CSI connector
  - [ ] Verify ribbon cable orientation

- [ ] **V4L2 / ISP Bring-up**
  - [ ] Verify sensor detected in dmesg
  - [ ] Check device nodes (`/dev/video*`)
  - [ ] Load IQ files from `/etc/iqfiles/`
  - [ ] Initialize RKAIQ context

- [ ] **ISP Configuration**
  - [ ] Enable 3A (AE/AWB/AF)
  - [ ] Configure LDCH lens correction
  - [ ] Set LDCH level = 255 (cylindrical projection)
  - [ ] Test HDR mode (optional)

- [ ] **Full Pipeline**
  - [ ] RAW → ISP → NV12 working
  - [ ] RGA resize to 640×640 for NPU
  - [ ] RGA resize to 1920×1080 for encoder
  - [ ] H.265 hardware encode working

- [ ] **Calibration**
  - [ ] Measure actual horizontal FOV
  - [ ] Verify LDCH cylindrical projection
  - [ ] Test azimuth accuracy at known angles
  - [ ] Document lens distortion characteristics

- [ ] **Performance Validation**
  - [ ] 30 FPS sustained capture
  - [ ] < 50ms end-to-end latency
  - [ ] NPU inference < 30ms
  - [ ] Low-light performance testing

### Configuration

```c
// Production sensor config
#define CAMERA_SENSOR_WIDTH  2304
#define CAMERA_SENSOR_HEIGHT 1296
#define CAMERA_SENSOR_FPS    30

// ISP config
g_pipeline.config.ldch_enabled = true;
g_pipeline.config.ldch_level = 255;
g_pipeline.config.horizontal_fov_deg = 120.0f;
```

### Success Criteria

| Metric | Target | Notes |
|--------|--------|-------|
| Frame capture | 30 FPS | Hardware ISP |
| ISP latency | < 5ms | With LDCH |
| NPU inference | < 30ms | YOLOv8n |
| H.265 encode | < 10ms | Hardware MPP |
| End-to-end | < 50ms | Full pipeline |
| Azimuth accuracy | ±2° | With LDCH calibration |

---

## Integration Track 3: MIS5001 5MP Wide-Angle

**Purpose**: Alternate sensor validation with matching FOV

### Specifications

| Spec | Value |
|------|-------|
| Resolution | 2592 × 1944 (5MP) |
| Pixel Size | ~1.6 µm |
| FOV | ~120° horizontal (wide lens version) |
| Interface | MIPI CSI-2, 2 lanes |
| Frame Rate | 30 FPS |

### Integration Checklist

- [ ] **Hardware Setup**
  - [ ] Connect to Luckfox 20-pin CSI
  - [ ] Verify sensor detection

- [ ] **ISP Configuration**
  - [ ] Load MIS5001 IQ files
  - [ ] Configure resolution (may need downscale)
  - [ ] Enable LDCH with appropriate mesh

- [ ] **Comparison Testing**
  - [ ] Side-by-side with SC3336
  - [ ] Low-light performance comparison
  - [ ] FOV accuracy verification
  - [ ] Color reproduction comparison

- [ ] **Documentation**
  - [ ] Record any driver/config differences
  - [ ] Note IQ file requirements
  - [ ] Document as SC3336 fallback option

### Configuration

```c
// MIS5001 config (may need adjustment)
#define CAMERA_SENSOR_WIDTH  2592  // or downscaled
#define CAMERA_SENSOR_HEIGHT 1944
#define CAMERA_SENSOR_FPS    30

g_pipeline.config.horizontal_fov_deg = 120.0f;  // Verify with actual lens
```

### Success Criteria

| Metric | Target | Notes |
|--------|--------|-------|
| Feature parity | 100% | Same pipeline as SC3336 |
| FOV match | ±5° | Should be ~120° |
| Low-light | Acceptable | May be worse than SC3336 |

---

## Integration Track 4: IMX415 8MP (Sony STARVIS)

**Purpose**: Low-light performance benchmark and 4K capability testing

### Specifications

| Spec | Value |
|------|-------|
| Resolution | 3840 × 2160 (4K) |
| Pixel Size | 1.45 µm |
| FOV | ~95° horizontal (stock lens) |
| Interface | MIPI CSI-2, 2/4 lanes |
| Frame Rate | 30 FPS (4K), 60 FPS (1080p) |
| Technology | STARVIS BSI |
| Min Illumination | 0.4 lux |

### Integration Checklist

- [ ] **Hardware Setup**
  - [ ] Connect IMX415 module (A)
  - [ ] Verify sensor detection
  - [ ] Note any power requirements

- [ ] **ISP Configuration**
  - [ ] Load IMX415 IQ files
  - [ ] Configure for 1080p (match SC3336 output)
  - [ ] Enable STARVIS low-light optimizations

- [ ] **Low-Light Testing**
  - [ ] Test at various lux levels (0.5, 1, 5, 10, 50)
  - [ ] Compare noise levels with SC3336
  - [ ] Measure detection accuracy in low light
  - [ ] Document minimum usable illumination

- [ ] **4K Testing (Optional)**
  - [ ] Test full 4K capture
  - [ ] Measure ISP/encode performance at 4K
  - [ ] Evaluate if 4K adds value for RS-1

- [ ] **FOV Consideration**
  - [ ] Document actual FOV (~95°)
  - [ ] Adjust azimuth calculations
  - [ ] Consider lens swap for wider FOV

### Configuration

```c
// IMX415 config (1080p mode for comparison)
#define CAMERA_SENSOR_WIDTH  1920
#define CAMERA_SENSOR_HEIGHT 1080
#define CAMERA_SENSOR_FPS    30

g_pipeline.config.horizontal_fov_deg = 95.0f;  // Stock lens
```

### Success Criteria

| Metric | Target | Notes |
|--------|--------|-------|
| 1 lux detection | > 90% | Person detection |
| 0.5 lux detection | > 70% | STARVIS advantage |
| Noise level | Baseline | Compare to SC3336 |

---

## Integration Track 5: ISG1321 1.38MP Monochrome Global Shutter

**Purpose**: Motion artifact analysis and specialized testing

### Specifications

| Spec | Value |
|------|-------|
| Resolution | 1280 × 1080 |
| Pixel Size | TBD |
| Shutter | Global (no rolling shutter) |
| Color | Monochrome |
| Interface | MIPI CSI-2 |

### Integration Checklist

- [ ] **Hardware Setup**
  - [ ] Connect ISG1321 module
  - [ ] Verify sensor detection
  - [ ] Note monochrome output handling

- [ ] **Driver/ISP**
  - [ ] Check for ISG1321 driver support
  - [ ] Configure for monochrome output
  - [ ] Disable color processing in ISP

- [ ] **Motion Testing**
  - [ ] Compare rolling vs global shutter artifacts
  - [ ] Test with fast-moving subjects
  - [ ] Evaluate for high-speed tracking scenarios

- [ ] **NPU Compatibility**
  - [ ] Test YOLOv8 with grayscale input
  - [ ] May need model retrain or conversion
  - [ ] Document accuracy differences

### Use Cases

| Scenario | Value |
|----------|-------|
| Fast motion | No rolling shutter skew |
| Industrial | Strobe lighting compatible |
| Low bandwidth | Mono = 1/3 data |

### Success Criteria

| Metric | Target | Notes |
|--------|--------|-------|
| Motion artifacts | None | Global shutter benefit |
| Detection accuracy | > 70% | May be lower for mono |
| Driver support | Working | May need community driver |

---

## Testing Matrix

### Sensor Comparison Tests

Run identical tests across all sensors:

| Test | USB 720P | SC3336 | MIS5001 | IMX415 | ISG1321 |
|------|----------|--------|---------|--------|---------|
| Frame rate (FPS) | | | | | |
| Capture latency (ms) | | | | | |
| NPU inference (ms) | | | | | |
| End-to-end (ms) | | | | | |
| Detection @ 50 lux | | | | | |
| Detection @ 10 lux | | | | | |
| Detection @ 1 lux | | | | | |
| Azimuth accuracy (°) | | | | | |
| Power draw (mW) | | | | | |

### Test Environments

| Environment | Lux Level | Description |
|-------------|-----------|-------------|
| Bright office | 300-500 | Overhead fluorescent |
| Dim office | 50-100 | Evening, some lights on |
| Low light | 5-20 | Night light only |
| Near dark | 0.5-2 | STARVIS territory |

---

## Integration Schedule

```
Week 0 (Now)
├── Order Innomaker USB camera ✓
├── Order Waveshare kit ✓
└── Create integration plan ✓

Week 1
├── USB camera arrives
├── Track 1: USB integration
└── Validate NPU → Fusion pipeline

Weeks 2-3
├── Waveshare shipment arrives
├── Track 2: SC3336 integration (priority)
└── Full ISP pipeline validation

Weeks 3-4
├── Track 3: MIS5001 comparison
├── Track 4: IMX415 low-light testing
└── Track 5: ISG1321 evaluation

Week 5+
├── Complete comparison matrix
├── Document findings
└── Select production configuration
```

---

## Quick Reference: Sensor Swap Procedure

### Switching Between MIPI CSI Sensors

1. **Power off** the Luckfox board
2. **Disconnect** current camera ribbon cable
3. **Connect** new camera (mind orientation!)
4. **Update config** if sensor differs:
   ```bash
   # Edit vision pipeline config
   vi /userdata/opticworks/vision_config.json
   ```
5. **Reboot** and verify:
   ```bash
   dmesg | grep -i camera
   v4l2-ctl --list-devices
   ```

### Switching Between USB and CSI

USB and CSI can run simultaneously on different device nodes:
- CSI: `/dev/video0` (rkisp), `/dev/video1` (output)
- USB: `/dev/video2` or `/dev/video3`

Select input source in application config.

---

## References

- [CAMERA_SENSORS.md](CAMERA_SENSORS.md) - Sensor specifications and comparison
- [VISION_PIPELINE.md](VISION_PIPELINE.md) - Pipeline architecture
- [ROOMPLAN_INTEGRATION_TESTING.md](ROOMPLAN_INTEGRATION_TESTING.md) - HIL testing methodology
- [Luckfox CSI Camera Wiki](https://wiki.luckfox.com/Luckfox-Pico/CSI-Camera/)
