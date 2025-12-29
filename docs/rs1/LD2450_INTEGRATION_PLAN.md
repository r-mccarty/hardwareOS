# LD2450 Radar Integration Plan

Comprehensive plan for integrating the HiLink LD2450 24GHz mmWave radar with the Luckfox Pico Max, including living room calibration procedures using common tools.

---

## Overview

This document covers:
1. **Physical hookup** of LD2450 to Luckfox Pico Max GPIO header
2. **Living room calibration scheme** using tape measure + iPhone RoomPlan
3. **Vision calibration** using USB UVC camera
4. **Ground truth methodology** with RoomPlan as reference
5. **Alternative depth sensing** evaluation (Luxonis OAK)

---

## Part 1: LD2450 to Luckfox Pico Max Hardware Hookup

### LD2450 Specifications

| Parameter | Value |
|-----------|-------|
| **Frequency** | 24GHz FMCW |
| **Detection Range** | 0.2m - 6m |
| **Horizontal FOV** | ±60° (120° total) |
| **Max Targets** | 3 simultaneous |
| **Update Rate** | ~10Hz |
| **Interface** | UART 256000 baud, 8N1 |
| **Power** | 5V DC, ~200mA |
| **Connector** | 4-pin (VCC, GND, TX, RX) |

### Luckfox Pico Max GPIO Header Pinout

The Luckfox Pico Max has a 40-pin GPIO header. The UART pins we need are:

```
                    Luckfox Pico Max GPIO Header
                    (Looking at top of board)

    3.3V  (1)  (2)  5V        ◄── LD2450 VCC
    GPIO  (3)  (4)  5V
    GPIO  (5)  (6)  GND       ◄── LD2450 GND
    GPIO  (7)  (8)  UART1_TX  ── /dev/ttyS1
    GND   (9)  (10) UART1_RX
    GPIO  (11) (12) GPIO
    GPIO  (13) (14) GND
    GPIO  (15) (16) GPIO
    3.3V  (17) (18) GPIO
    SPI   (19) (20) GND
    SPI   (21) (22) GPIO
    SPI   (23) (24) SPI
    GND   (25) (26) GPIO
    I2C   (27) (28) I2C
    GPIO  (29) (30) GND
    GPIO  (31) (32) GPIO
    GPIO  (33) (34) GND
    GPIO  (35) (36) GPIO
    GPIO  (37) (38) GND       ◄── Power GND (VSYS option)
    5V    (39) (40) GND       ◄── VSYS input for external power
```

### UART Device Mapping

| Device | Pins | Notes |
|--------|------|-------|
| `/dev/ttyS0` | UART0 | Often console (may conflict) |
| `/dev/ttyS1` | Pin 8 (TX), Pin 10 (RX) | **Recommended for LD2450** |
| `/dev/ttyS3` | Alternate mapping | Check device tree |

> **Note**: The code defaults to `/dev/ttyS3` but this may need adjustment for Luckfox. Verify with `ls /dev/ttyS*` after boot.

### Wiring Diagram

```
         LD2450 Radar                    Luckfox Pico Max
    ┌───────────────────┐            ┌───────────────────────┐
    │                   │            │    GPIO Header        │
    │   VCC (Red) ──────┼────────────┼──► Pin 2 (5V)         │
    │                   │            │                       │
    │   GND (Black) ────┼────────────┼──► Pin 6 (GND)        │
    │                   │            │                       │
    │   TX  (Yellow) ───┼────────────┼──► Pin 10 (UART1_RX)  │
    │                   │            │                       │
    │   RX  (Green) ────┼────────────┼──► Pin 8 (UART1_TX)   │
    │                   │            │                       │
    └───────────────────┘            └───────────────────────┘

    Wire colors are typical; verify with your LD2450 module.
    TX→RX, RX→TX (cross-over connection).
```

### Bill of Materials

| Item | Qty | Purpose | Source |
|------|-----|---------|--------|
| LD2450 Radar Module | 1 | 24GHz mmWave sensor | HiLink, AliExpress (~$8) |
| Dupont F-F Jumper Wires | 4 | GPIO connections | Electronics store |
| Logic Level Shifter (3.3V↔5V) | 1 | **Optional** - LD2450 is 3.3V tolerant | Only if issues arise |

### Hardware Setup Checklist

- [ ] **Verify LD2450 pinout** - Check silkscreen labels on your specific module
- [ ] **Connect power first** - VCC to 5V (Pin 2), GND to GND (Pin 6)
- [ ] **Connect UART** - LD2450 TX → Luckfox Pin 10 (RX), LD2450 RX → Luckfox Pin 8 (TX)
- [ ] **Check power budget** - LD2450 draws ~200mA; with USB camera (~200mA) = ~400mA total
- [ ] **Secure connections** - Use heat shrink or tape to prevent shorts

### Software Verification

```bash
# On Luckfox (via SSH or ADB shell)

# 1. Check available UART devices
ls -la /dev/ttyS*

# 2. Set correct permissions
chmod 666 /dev/ttyS1

# 3. Configure baud rate
stty -F /dev/ttyS1 256000 raw -echo

# 4. Test raw data stream (should see binary data)
cat /dev/ttyS1 | xxd | head -20

# 5. Look for LD2450 frame headers (AA FF 03 00)
cat /dev/ttyS1 | xxd | grep "aa ff 03 00"
```

### Troubleshooting

| Issue | Cause | Solution |
|-------|-------|----------|
| No data on UART | TX/RX swapped | Swap the TX and RX wires |
| Garbage data | Wrong baud rate | Verify 256000 baud setting |
| Intermittent data | Poor connections | Check/resolder Dupont connectors |
| Device not found | Wrong ttyS | Try `/dev/ttyS0`, `/dev/ttyS1`, `/dev/ttyS3` |
| Power issues | Insufficient current | Use separate 5V supply for LD2450 |

---

## Part 2: Living Room Calibration Scheme

### Philosophy

Calibration uses **iPhone RoomPlan as the ground truth** for room geometry and coordinate system. The radar and camera are then calibrated relative to this reference using simple tools available in any home.

### Equipment Needed

| Item | Purpose | Notes |
|------|---------|-------|
| **iPhone with LiDAR** | RoomPlan scanning (ground truth) | iPhone 12 Pro or newer |
| **Tape Measure** | Distance verification | 5m+ range, metric preferred |
| **Painter's Tape** | Mark reference positions | Won't damage floors |
| **Smartphone Timer** | Timing walk tests | Any phone |
| **Notepad** | Record measurements | Physical or digital |

### Calibration Procedure Overview

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                      Living Room Calibration Workflow                        │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                              │
│  Phase 1: Room Geometry (iPhone RoomPlan)                                   │
│  ├── Scan room with iPhone                                                  │
│  ├── Mark sensor mounting position in scan                                  │
│  └── Export CapturedRoom JSON                                               │
│                                                                              │
│  Phase 2: Reference Grid (Tape Measure)                                     │
│  ├── Measure and mark 1m grid on floor                                      │
│  ├── Create known reference positions (0°, ±30°, ±60°)                     │
│  └── Record exact distances from sensor                                     │
│                                                                              │
│  Phase 3: Radar Calibration                                                 │
│  ├── Place target at each reference position                                │
│  ├── Record radar measurements                                              │
│  ├── Calculate offset/scale corrections                                     │
│  └── Validate with walk-through test                                        │
│                                                                              │
│  Phase 4: Vision Calibration                                                │
│  ├── Stand at each reference position                                       │
│  ├── Capture detection coordinates                                          │
│  ├── Calculate FOV and lens correction                                      │
│  └── Validate detection accuracy                                            │
│                                                                              │
│  Phase 5: Fusion Validation                                                 │
│  ├── Walk through room at normal pace                                       │
│  ├── Record fused track positions                                           │
│  └── Compare to reference path                                              │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

### Phase 1: Room Geometry Capture (iPhone RoomPlan)

#### Step 1.1: Prepare the Room
- Clear clutter that might confuse the scanner
- Ensure adequate lighting (>50 lux)
- Decide on sensor mounting location (wall-mounted recommended)

#### Step 1.2: Scan with iPhone
```
Recommended App: Mappedin Scan (free, exports GeoJSON)
Alternative: Apple's RoomPlan sample app (requires Xcode)

1. Open scanning app
2. Scan entire room, ensuring all walls are captured
3. Walk slowly along each wall
4. Capture doors, windows, and major furniture
5. Complete scan and review for completeness
```

#### Step 1.3: Mark Sensor Position
Before finishing the scan, place the sensor (or a marker representing it) at the intended mounting position:
- **Height**: 1.2m - 1.5m recommended (chest height)
- **Orientation**: Facing into the room, centered on coverage area
- **Clearance**: Minimum 0.3m from corners for radar side lobes

#### Step 1.4: Export and Upload
```bash
# Export JSON from app (AirDrop to Mac or save to Files)
# Transfer to development machine
# Upload to RS-1
curl -X POST http://<device-ip>/api/setup/roomplan \
  -H "Content-Type: application/json" \
  -d @room_scan.json
```

---

### Phase 2: Reference Grid Setup

#### Step 2.1: Mark Grid Origin
The sensor position is the origin (0, 0). Mark this on the floor directly below the sensor.

```
                      Reference Grid Layout

                           +Y (forward)
                              │
           3m ────────────────┼────────────────── 3m
                              │
                     ┌────────┼────────┐
           2m ───────│   30°  │  30°   │───────── 2m
                     │    ╲   │   ╱    │
                     │     ╲  │  ╱     │
           1m ───────│      ╲ │ ╱      │───────── 1m
                     │  60°  ╲│╱  60°  │
                     └────────●────────┘
                             0,0
                           [SENSOR]
                              │
    ───────────────────────────────────────────── -X ◄──► +X
         -3m      -2m     -1m  0  +1m     +2m      +3m
```

#### Step 2.2: Measure and Mark Reference Positions

Using tape measure and painter's tape, mark these positions on the floor:

| Position ID | Distance | Angle | X (m) | Y (m) | Purpose |
|-------------|----------|-------|-------|-------|---------|
| C1 | 1.0m | 0° | 0.00 | 1.00 | Center near |
| C2 | 2.0m | 0° | 0.00 | 2.00 | Center mid |
| C3 | 3.0m | 0° | 0.00 | 3.00 | Center far |
| L30-2 | 2.0m | -30° | -1.00 | 1.73 | Left 30° |
| R30-2 | 2.0m | +30° | +1.00 | 1.73 | Right 30° |
| L60-2 | 2.0m | -60° | -1.73 | 1.00 | Left 60° |
| R60-2 | 2.0m | +60° | +1.73 | 1.00 | Right 60° |
| L45-3 | 3.0m | -45° | -2.12 | 2.12 | Left 45° far |
| R45-3 | 3.0m | +45° | +2.12 | 2.12 | Right 45° far |

**Calculation Reference**:
- X = Distance × sin(Angle)
- Y = Distance × cos(Angle)

#### Step 2.3: Verify with Tape Measure

Double-check each position:
```
For each marked position:
1. Measure straight-line distance to sensor origin
2. Verify angle using adjacent wall as reference
3. Record actual measured values (may differ slightly from theoretical)
```

---

### Phase 3: Radar Calibration

#### Step 3.1: Static Target Test

Use a person standing still at each reference position:

```bash
# On development machine, capture radar data
# Start the RS-1 application with debug logging
LOG_TRACE_SCOPES="radar,fusion" ./opticworks-rs1_app

# In another terminal, watch radar output
curl -s http://<device-ip>/api/debug/radar | jq
```

#### Step 3.2: Record Measurements

| Position | Expected X | Expected Y | Radar X | Radar Y | Error X | Error Y |
|----------|------------|------------|---------|---------|---------|---------|
| C1 | 0.00 | 1.00 | | | | |
| C2 | 0.00 | 2.00 | | | | |
| C3 | 0.00 | 3.00 | | | | |
| L30-2 | -1.00 | 1.73 | | | | |
| R30-2 | +1.00 | 1.73 | | | | |
| L60-2 | -1.73 | 1.00 | | | | |
| R60-2 | +1.73 | 1.00 | | | | |

**Note**: Stand still for 5+ seconds at each position. Average multiple readings.

#### Step 3.3: Calculate Calibration Parameters

```python
# calibration_analysis.py
import numpy as np

# Measured data (fill in from your tests)
expected = np.array([
    [0.00, 1.00],   # C1
    [0.00, 2.00],   # C2
    [0.00, 3.00],   # C3
    [-1.00, 1.73],  # L30-2
    [+1.00, 1.73],  # R30-2
])

measured = np.array([
    # Fill in radar measurements
])

# Calculate scale and offset
# Simple linear regression: measured = scale * expected + offset
scale_x = np.polyfit(expected[:, 0], measured[:, 0], 1)[0]
scale_y = np.polyfit(expected[:, 1], measured[:, 1], 1)[0]
offset_x = np.mean(measured[:, 0] - scale_x * expected[:, 0])
offset_y = np.mean(measured[:, 1] - scale_y * expected[:, 1])

print(f"Radar Calibration:")
print(f"  Scale X: {scale_x:.4f}")
print(f"  Scale Y: {scale_y:.4f}")
print(f"  Offset X: {offset_x:.4f}m")
print(f"  Offset Y: {offset_y:.4f}m")
```

#### Step 3.4: Apply Calibration

Update RS-1 configuration:
```json
{
  "radar": {
    "calibration": {
      "scale_x": 1.02,
      "scale_y": 0.98,
      "offset_x": 0.05,
      "offset_y": -0.03,
      "rotation_deg": 2.0
    }
  }
}
```

#### Step 3.5: Validation Walk Test

1. Walk a known path (e.g., along the center line from 3m to 1m)
2. Record tracked positions via WorldState
3. Compare to expected path
4. Target accuracy: **±0.3m position, ±5° angle**

---

### Phase 4: Vision Calibration (USB UVC Camera)

#### Step 4.1: Camera Setup

```bash
# On Luckfox, verify camera is detected
v4l2-ctl --list-devices

# Check supported formats
v4l2-ctl --device=/dev/video0 --list-formats-ext

# Expected output for Innomaker 720P:
# - YUYV 1280x720 @ 30fps
# - MJPEG 1280x720 @ 30fps
```

#### Step 4.2: FOV Verification

Using the reference grid:

1. Stand at position L60-2 (2m, -60°) - you should be at the **edge** of camera view
2. Stand at position R60-2 (2m, +60°) - should also be at edge
3. If visible beyond ±60°, camera FOV is wider than expected

**Measure actual FOV**:
```
Actual FOV = 2 × arctan(visible_width / 2 / distance)

Example: If a 3m wide view is visible at 2m distance:
FOV = 2 × arctan(3 / 2 / 2) = 2 × arctan(0.75) ≈ 73°
```

#### Step 4.3: Detection Position Calibration

For each reference position:

1. Stand at marked position
2. Capture detection bounding box via API or logs
3. Calculate detected azimuth angle
4. Compare to known angle

```bash
# Debug API to get current detections
curl -s http://<device-ip>/api/debug/vision | jq '.detections'
```

| Position | Expected Angle | Detected Angle | Error |
|----------|----------------|----------------|-------|
| C2 | 0° | | |
| L30-2 | -30° | | |
| R30-2 | +30° | | |
| L60-2 | -60° | | |
| R60-2 | +60° | | |

#### Step 4.4: Calculate FOV Correction

If systematic errors exist, adjust the configured FOV:

```
Configured FOV = Nominal FOV × (measured_span / expected_span)

Example: If targets at ±60° appear at ±55° in detections:
Correction factor = 60° / 55° = 1.09
New FOV config = 100° × 1.09 = 109°
```

Update configuration:
```json
{
  "vision": {
    "horizontal_fov_deg": 109.0,
    "calibration": {
      "azimuth_offset_deg": 2.0
    }
  }
}
```

---

### Phase 5: Fusion Validation

#### Step 5.1: Walk Test Protocol

1. Start recording WorldState stream
2. Walk a defined path:
   - Start at C3 (3m, center)
   - Walk to L30-2 (2m, -30°)
   - Walk to R30-2 (2m, +30°)
   - Walk to C1 (1m, center)
   - Return to C3

3. Time the walk (~30 seconds total)
4. Stop recording

#### Step 5.2: Analyze Track Quality

```bash
# Record WorldState for 30 seconds
timeout 30 websocat ws://<device-ip>/ws/worldstate > walk_test.json

# Analyze position errors
python analyze_walk_test.py walk_test.json expected_path.json
```

#### Step 5.3: Success Criteria

| Metric | Target | Good | Acceptable |
|--------|--------|------|------------|
| Position accuracy | ±0.3m | ±0.5m | ±0.75m |
| Track continuity | >95% | >90% | >80% |
| Ghost detections | <5% | <10% | <15% |
| Detection latency | <200ms | <500ms | <1s |

---

## Part 3: Ground Truth Methodology

### RoomPlan as Primary Reference

Apple RoomPlan provides:
- **Room geometry** accurate to ±2cm
- **Object positions** (furniture, doors, windows)
- **Consistent coordinate system** tied to physical room

### Coordinate System Alignment

```
                RoomPlan Coordinates          RS-1 Sensor Coordinates

                      +Z (up)                       +Y (forward)
                        │                              │
                        │                              │
                        │                              │
          +Y ───────────┼─────────── -Y     -X ────────┼──────── +X
         (left)         │          (right)  (left)     │       (right)
                        │                              │
                        │                           [SENSOR]
                     +X (forward)

    Transform: sensor_x = roomplan_y
               sensor_y = roomplan_x
               (Rotation + translation based on sensor_pose matrix)
```

### Calibration Verification Protocol

1. **Scan room** with RoomPlan, marking sensor position
2. **Upload to RS-1** via `/api/setup/roomplan`
3. **Verify transformation** by walking to known furniture positions
4. **Check that detections appear at correct room coordinates**

---

## Part 4: Alternative Depth Sensing Evaluation

### Current Approach: LD2450 Radar + RGB Camera

| Aspect | Advantage | Limitation |
|--------|-----------|------------|
| **Range accuracy** | ±5cm from 0.2-6m | Multipath in corners |
| **Angular accuracy** | ±5° within ±60° FOV | Limited to 3 targets |
| **Lighting** | Works in complete darkness | N/A |
| **Cost** | ~$8 per unit | N/A |
| **Privacy** | No images captured | N/A |
| **Doppler** | Detects motion/stationary | Stationary may be missed |

### Alternative: Luxonis OAK-D Series

| Model | Depth Range | FOV | RGB | NPU | Price |
|-------|-------------|-----|-----|-----|-------|
| OAK-D Lite | 0.2-9m | 73° HFOV | 4K | 4 TOPS | ~$149 |
| OAK-D S2 | 0.2-15m | 75° HFOV | 12MP | 4 TOPS | ~$249 |
| OAK-D Pro | 0.15-35m | 75° HFOV | 12MP | 4 TOPS | ~$299 |

#### Luxonis Advantages

1. **Single device** replaces radar + camera
2. **Higher angular resolution** (per-pixel depth)
3. **Built-in NPU** offloads Luckfox
4. **Longer range** with active IR (Pro model)
5. **Better multi-target** handling

#### Luxonis Disadvantages

1. **Higher cost** (10-30× LD2450)
2. **USB bandwidth** requirements (USB 3.0 preferred)
3. **Power consumption** (2-4W vs 1W)
4. **No Doppler** (depth-only, no velocity)
5. **Privacy concerns** (captures RGB images)

### Recommendation

**Start with LD2450 + USB camera** for initial development:
- Lower cost for prototyping
- Simpler integration
- Adequate for living room scales

**Consider Luxonis for**:
- Commercial deployments requiring higher accuracy
- Larger spaces (>6m range)
- Multi-room tracking without sensor per room
- Use cases where on-device NPU is beneficial

### Hybrid Approach (Future)

```
┌─────────────────────────────────────────────────────────────────┐
│                    Hybrid Sensor Strategy                        │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│   LD2450 Radar          Luxonis OAK-D           USB Camera      │
│   ┌──────────┐          ┌──────────┐           ┌──────────┐     │
│   │ Low cost │          │ High acc │           │ Fallback │     │
│   │ Doppler  │          │ Depth+RGB│           │ RGB only │     │
│   │ Dark OK  │          │ IR option│           │ Cheap    │     │
│   └────┬─────┘          └────┬─────┘           └────┬─────┘     │
│        │                     │                      │           │
│        └─────────────────────┼──────────────────────┘           │
│                              ▼                                   │
│                    ┌─────────────────┐                          │
│                    │  Fusion Engine  │                          │
│                    └────────┬────────┘                          │
│                             ▼                                    │
│                    ┌─────────────────┐                          │
│                    │   WorldState    │                          │
│                    └─────────────────┘                          │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

---

## Part 5: Complete HIL Setup with LD2450

### Full Wiring Diagram

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
│ Spliced USB  │   Pin 39 (VSYS)    │  │    │USB-C to │            │ │
│   Cable      ├────────────────────┤──┼───►│USB-A OTG│            │ │
│ (from N100)  │   Pin 38 (GND)     │  │    │ Adapter │            │ │
│   GND ───────┼────────────────────┤──┼────┴────┬────┘            │ │
└──────────────┘                    │  │         │                 │ │
                                    │  └─────────┼─────────────────┘ │
                                    │            │                   │
┌──────────────┐                    │     ┌──────┴──────┐            │
│   LD2450     │  Pin 2 (5V)        │     │ USB Camera  │            │
│   Radar      ├────────────────────┤     │ (Innomaker) │            │
│              │  Pin 6 (GND)       │     └─────────────┘            │
│  GND ────────┼────────────────────┤                                │
│              │                    │  ┌───────────────────────────┐ │
│  TX ─────────┼────────────────────┤──┤ Pin 10 (UART1_RX)         │ │
│              │                    │  │                           │ │
│  RX ─────────┼────────────────────┤──┤ Pin 8 (UART1_TX)          │ │
│              │                    │  └───────────────────────────┘ │
└──────────────┘                    │                                 │
                                    │  ┌───────────────────────────┐ │
                                    │  │ 20-pin CSI Connector      │ │
                                    │  │  (for MIPI camera later)  │ │
                                    │  └───────────────────────────┘ │
                                    └─────────────────────────────────┘
```

### Power Budget

| Device | Current @ 5V | Power |
|--------|--------------|-------|
| Luckfox Pico Max (active) | 180mA | 0.9W |
| USB Camera | 150mA | 0.75W |
| LD2450 Radar | 200mA | 1.0W |
| **Total** | **530mA** | **2.65W** |

N100 USB 3.0 port provides 900mA — sufficient with margin.

### Startup Checklist

- [ ] Power connected to Pin 39/38 (via spliced USB from N100)
- [ ] Ethernet connected between N100 and Luckfox
- [ ] USB-C in host mode with camera connected via OTG adapter
- [ ] LD2450 connected to UART1 (Pins 2, 6, 8, 10)
- [ ] All cables secured and strain-relieved

### Verification Commands

```bash
# On Luckfox (via SSH at 192.168.1.100)

# 1. Check UART devices
ls -la /dev/ttyS*

# 2. Verify radar data stream
cat /dev/ttyS1 | xxd | head -5
# Expected: Lines containing "aa ff 03 00" (LD2450 header)

# 3. Check USB camera
v4l2-ctl --list-devices
# Expected: USB camera appears

# 4. Test camera capture
v4l2-ctl --device=/dev/video0 --stream-mmap --stream-count=10
# Expected: 10 frames captured successfully

# 5. Start RS-1 application with debug
LOG_TRACE_SCOPES="radar,vision,fusion" ./opticworks-rs1_app

# 6. On dev machine, verify WorldState stream
curl -s http://192.168.1.100/api/worldstate | jq
```

---

## Appendix A: Calibration Data Template

### Radar Calibration Log

```
Date: ____________
Operator: ____________
Room: ____________
Sensor Height: ____________ m
Sensor Orientation: ____________°

Reference Positions:
| ID | Expected X | Expected Y | Measured X | Measured Y | Notes |
|----|------------|------------|------------|------------|-------|
| C1 |            |            |            |            |       |
| C2 |            |            |            |            |       |
| C3 |            |            |            |            |       |
| L30-2 |         |            |            |            |       |
| R30-2 |         |            |            |            |       |
| L60-2 |         |            |            |            |       |
| R60-2 |         |            |            |            |       |

Calculated Corrections:
  Scale X: ______
  Scale Y: ______
  Offset X: ______ m
  Offset Y: ______ m
```

### Vision Calibration Log

```
Date: ____________
Camera Model: ____________
Nominal FOV: ____________°

Azimuth Measurements:
| Position | Expected Angle | Detected Angle | Pixel X | Error |
|----------|----------------|----------------|---------|-------|
| C2       | 0°             |                |         |       |
| L30-2    | -30°           |                |         |       |
| R30-2    | +30°           |                |         |       |
| L60-2    | -60°           |                |         |       |
| R60-2    | +60°           |                |         |       |

Calculated FOV: ______°
Azimuth Offset: ______°
```

---

## Appendix B: Quick Reference Card

```
┌─────────────────────────────────────────────────────────────────┐
│                    LD2450 Quick Reference                        │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  UART: 256000 baud, 8N1                                         │
│  Frame header: 0xAA 0xFF 0x03 0x00                              │
│  Frame tail:   0x55 0xCC                                        │
│                                                                  │
│  Target data: 8 bytes × 3 targets                               │
│    Bytes 0-1: X position (int16, mm)                            │
│    Bytes 2-3: Y position (int16, mm)                            │
│    Bytes 4-5: Speed (int16, cm/s)                               │
│    Bytes 6-7: Reserved                                          │
│                                                                  │
│  Coordinate system:                                              │
│    +X = right, +Y = forward, origin at sensor                   │
│                                                                  │
│  Range: 0.2m - 6.0m                                             │
│  FOV: ±60° horizontal                                           │
│  Update rate: ~10 Hz                                            │
│  Max targets: 3                                                  │
│                                                                  │
│  Power: 5V, ~200mA                                              │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

---

## See Also

- [RADAR_INTEGRATION.md](RADAR_INTEGRATION.md) - Protocol details and code
- [CAMERA_HIL_INTEGRATION.md](CAMERA_HIL_INTEGRATION.md) - Camera setup guide
- [ROOMPLAN_INTEGRATION_TESTING.md](ROOMPLAN_INTEGRATION_TESTING.md) - RoomPlan testing
- [ROOMPLAN_API.md](ROOMPLAN_API.md) - Coordinate transform API
- [FUSION_ENGINE.md](FUSION_ENGINE.md) - Sensor fusion algorithms
