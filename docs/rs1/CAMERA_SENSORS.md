# RS-1 Camera Sensor Guide

This document covers camera sensor options for the RS-1 vision pipeline, including alternate sensors for development and hardware-in-the-loop (HIL) testing.

## Production Sensor: SC3336

The RS-1 production design uses the SmartSens SC3336 3MP MIPI sensor.

| Specification | Value |
|---------------|-------|
| Resolution | 2304 x 1296 (3MP) |
| Pixel Size | ~2.5 µm |
| Optical Format | 1/2.8" |
| Interface | MIPI CSI-2, 2 lanes |
| Frame Rate | 30 FPS |
| FOV | ~120° horizontal (with wide-angle lens) |
| Low-Light | Excellent (DSI-2 tech, PixGain dual-gain) |

### Why SC3336?

- Large pixels (~2.5µm) provide excellent low-light sensitivity for occupancy detection
- SmartSens DSI-2 technology with PixGain for day/night transitions
- Native support in Rockchip RV1106 ISP with IQ tuning files
- Wide-angle lens option matches RS-1's 120° FOV requirement

---

## Alternate MIPI CSI Sensors

For development boards (Luckfox Pico Pro Max) when SC3336 is unavailable:

| Sensor | Resolution | Pixel Size | Optical Format | Low-Light | RV1106 Support | Notes |
|--------|------------|------------|----------------|-----------|----------------|-------|
| **MIS5001** | 5MP | ~1.6 µm | 1/2.7" | Good | Native | Recommended alternative |
| **IMX415** | 8.4MP (4K) | 1.45 µm | 1/2.8" | Excellent | Native | Sony STARVIS BSI |
| **SC3336** | 3MP | ~2.5 µm | 1/2.8" | Excellent | Native | Production sensor |
| IMX219 | 8MP | 1.12 µm | 1/4" | Poor | Community | Requires driver work |
| OV5647 | 5MP | 1.4 µm | 1/4" | Fair | Community | Requires driver work |

### Recommended: MIS5001

The MIS5001 is the best drop-in alternative:
- Officially supported by Luckfox with native IQ files
- Wide-angle lens version available (~120° FOV)
- Higher resolution than SC3336 (can downsample)
- Available from Waveshare and Luckfox directly

### Premium Option: IMX415

Sony's IMX415 with STARVIS technology has the best absolute low-light performance:
- Backside illumination (BSI) captures more photons despite smaller pixels
- 0.4 lux minimum illumination
- 100dB HDR with DOL technology
- Overkill resolution (4K) but excellent for testing

---

## Luckfox Pico Pro Max Connector

**Important**: The Luckfox Pico Pro Max uses a non-standard **20-pin** MIPI CSI connector.

| Connector Type | Pin Count | Pitch | Compatible Boards |
|----------------|-----------|-------|-------------------|
| Raspberry Pi Standard | 15-pin | 1.0mm | RPi 3/4, most SBCs |
| Raspberry Pi Zero/5 | 22-pin | 0.5mm | RPi Zero, RPi 5, CM4 |
| **Luckfox Pro/Max** | **20-pin** | 0.5mm | Luckfox only |

Standard Raspberry Pi cameras (15-pin or 22-pin) will **not** work without custom adapter cables.

### Officially Compatible Cameras

Only these cameras work out-of-box with Luckfox Pico Pro Max:
- SC3336 3MP Camera (A/B)
- MIS5001 5MP Camera (A/B)
- IMX415 8MP Camera (Luckfox version)

### Sourcing

| Source | Shipping to USA | Time |
|--------|-----------------|------|
| Waveshare (DHL Express) | ~$20 | 3-7 days |
| Luckfox Official | ~$15 | 2-3 weeks |
| AliExpress | ~$5 | 2-3 weeks |
| Amazon | Varies | Often out of stock |

---

## USB Camera Workaround for HIL Testing

When MIPI CSI cameras are unavailable, USB UVC cameras can be used for initial hardware-in-the-loop testing.

### Recommended USB Camera

**Innomaker 720P USB 2.0 UVC Camera with 120° DFOV**
- Resolution: 720P (1280x720)
- FOV: 120° diagonal (close to RS-1's 120° horizontal spec)
- Interface: USB 2.0 UVC
- Price: ~$17
- Availability: Amazon Prime (next-day delivery)

### USB Camera Limitations

| Feature | MIPI CSI (SC3336) | USB UVC |
|---------|-------------------|---------|
| ISP Processing | Full (3A, LDCH, HDR) | None |
| Lens Correction | Hardware LDCH | Software only |
| Low-Light | Excellent | Basic |
| Latency | ~5ms | ~30-50ms |
| NPU Pipeline | Full | Compatible |
| WebRTC Streaming | Hardware H.265 | Software encode |

### What Works with USB Camera

- V4L2 frame capture
- NPU inference (YOLOv8)
- Fusion engine integration
- WebRTC streaming (software encode)
- Basic occupancy detection

### What Doesn't Work

- Hardware ISP (LDCH lens correction disabled)
- Accurate azimuth calculation (no cylindrical projection)
- Optimal low-light performance
- H.265 hardware encoding

### USB Camera Setup

1. **Connect USB camera to Luckfox**
   ```bash
   # Verify camera is detected
   v4l2-ctl --list-devices
   # Should show /dev/video0 or /dev/video1
   ```

2. **Test capture**
   ```bash
   v4l2-ctl --device=/dev/video0 \
     --set-fmt-video=width=1280,height=720,pixelformat=YUYV \
     --stream-mmap --stream-count=10
   ```

3. **Configure vision pipeline**

   Update FOV to match camera:
   ```c
   // targets/rv1106/native/cgo/vision_pipeline.c
   g_pipeline.config.horizontal_fov_deg = 120.0f;  // Adjust to actual lens
   ```

4. **Disable LDCH** (not available for USB cameras):
   ```c
   // Skip ISP initialization for USB path
   g_pipeline.config.ldch_enabled = false;
   ```

### Device Path Configuration

| Camera Type | Device Path | Notes |
|-------------|-------------|-------|
| MIPI CSI (SC3336) | `/dev/video0` (sensor), `/dev/video1` (ISP) | Default config |
| USB UVC | `/dev/video0` or `/dev/video1` | Check `v4l2-ctl --list-devices` |

---

## FOV Configuration

The vision pipeline calculates azimuth angles based on configured FOV:

```c
// targets/rv1106/native/cgo/npu.c
g_npu.config.horizontal_fov_deg = 120.0f;  // Must match actual lens
g_npu.config.vertical_fov_deg = 90.0f;
```

### FOV by Camera/Lens

| Camera | Lens | Horizontal FOV | Config Value |
|--------|------|----------------|--------------|
| SC3336 (A) | Standard | ~98° | 98.0 |
| SC3336 (B) | Wide-angle | ~120° | 120.0 |
| MIS5001 (A) | Wide-angle | ~120° | 120.0 |
| IMX415 | Stock M12 | ~95° | 95.0 |
| Innomaker USB | 120° DFOV | ~100° H | 100.0 |

**Note**: DFOV (diagonal FOV) ≠ horizontal FOV. For 16:9 sensors, horizontal FOV ≈ DFOV × 0.87.

---

## Pixel Size and Low-Light Comparison

Larger pixels capture more light, improving low-light performance:

```
Best Low-Light ────────────────────────────────────── Worst

SC3336 (~2.5µm) > IMX415* (1.45µm) > MIS5001 (~1.6µm) > OV5647 (1.4µm) > IMX219 (1.12µm)
     │                │
     │                └─ *Sony STARVIS BSI compensates for smaller pixels
     └─ Largest pixels, best for occupancy detection
```

### Minimum Illumination

| Sensor | Technology | Min Lux (color) | Notes |
|--------|------------|-----------------|-------|
| IMX415 | STARVIS BSI | 0.4 lux | Best absolute low-light |
| SC3336 | DSI-2 + PixGain | ~1 lux | Excellent day/night transition |
| MIS5001 | Standard | ~3 lux | Good for indoor use |
| OV5647 | OmniBSI | ~10 lux | 2x2 binning helps |
| IMX219 | Standard | ~10 lux | Smallest pixels, struggles |

---

## HIL Testing Strategy

### Phase 1: Immediate (USB Camera)

1. Order Innomaker 720P USB camera (Amazon Prime, next-day)
2. Configure for USB V4L2 input
3. Validate NPU → Fusion → WorldState pipeline
4. Test WebRTC streaming (software encode)

### Phase 2: Short-term (MIPI CSI)

1. Order MIS5001 from Waveshare (DHL Express, 3-7 days)
2. Switch to MIPI CSI pipeline
3. Enable ISP with LDCH correction
4. Validate full hardware acceleration

### Phase 3: Production

1. Source SC3336 when available
2. Final IQ tuning for production lens
3. Calibrate LDCH mesh for specific optics

---

## Configuration Changes for Alternate Sensors

### Camera Resolution

```c
// targets/rv1106/native/cgo/camera.h
#define CAMERA_SENSOR_WIDTH  2304   // Adjust for sensor
#define CAMERA_SENSOR_HEIGHT 1296   // Adjust for sensor
#define CAMERA_SENSOR_FPS    30
```

### ISP IQ Files

Each sensor requires matching IQ (Image Quality) tuning files:

```
/etc/iqfiles/
├── sc3336_CMK-OT2117-PC1_30IRC-F16.json   # SC3336 tuning
├── mis5001_*.json                          # MIS5001 tuning
└── imx415_*.json                           # IMX415 tuning
```

IQ files are provided by Luckfox/Rockchip for officially supported sensors.

---

## References

- [Luckfox CSI Camera Wiki](https://wiki.luckfox.com/Luckfox-Pico/CSI-Camera/)
- [Waveshare Luckfox Cameras](https://www.waveshare.com/product/luckfox/cameras.htm)
- [SmartSens SC3336 Product Page](https://www.gophotonics.com/products/cmos-image-sensors/smartsens-technology/21-1025-sc3336)
- [Sony IMX415 STARVIS](https://www.sony-semicon.com/files/62/pdf/p-12_IMX415-AAQR_AAMR_Flyer.pdf)

---

## See Also

- [VISION_PIPELINE.md](VISION_PIPELINE.md) - Full pipeline architecture
- [FUSION_ENGINE.md](FUSION_ENGINE.md) - How vision integrates with radar
- [ROOMPLAN_INTEGRATION_TESTING.md](ROOMPLAN_INTEGRATION_TESTING.md) - HIL testing methodology
