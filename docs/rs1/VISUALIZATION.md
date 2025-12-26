# RS-1 3D Visualization

This document describes the Tesla FSD-inspired 3D visualization system for the RS-1 occupancy tracking sensor.

## Overview

The RS-1 visualization provides a real-time 3D view of tracked occupants in a room, inspired by Tesla's Full Self-Driving (FSD) visualization aesthetic. It renders:

- **Tracked occupants** with motion trails, prediction cones, and confidence indicators
- **Room geometry** from Apple RoomPlan LiDAR scans (walls, doors, windows, furniture)
- **Sensor information** including radar sweep effects and FOV visualization
- **HUD overlay** with occupancy count, tracking source breakdown, and diagnostics

## Technology Stack

| Component | Technology | Purpose |
|-----------|------------|---------|
| 3D Engine | Three.js v0.182 | WebGL rendering |
| React Bindings | @react-three/fiber v9 | React integration |
| Helpers | @react-three/drei v10 | Camera controls, Grid, Line |
| Post-processing | @react-three/postprocessing | Bloom, Vignette effects |
| State Management | Zustand | WorldState and RoomPlan stores |

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         Scene.tsx                                │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────────┐  │
│  │EffectComposer│  │   Lighting   │  │   CameraController   │  │
│  │ - Bloom      │  │ - Ambient    │  │   (OrbitControls)    │  │
│  │ - Vignette   │  │ - Directional│  └──────────────────────┘  │
│  └──────────────┘  └──────────────┘                             │
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                    Room Geometry                            │ │
│  │  ┌─────────────┐  ┌──────────────────┐  ┌───────────────┐ │ │
│  │  │  RoomFloor  │  │CapturedRoomRender│  │ RoomBoundary  │ │ │
│  │  │ - Grid      │  │ - Wall3D         │  │ (fallback)    │ │ │
│  │  │ - RadarSweep│  │ - Door3D         │  └───────────────┘ │ │
│  │  │ - PulseRings│  │ - Window3D       │                     │ │
│  │  └─────────────┘  │ - Furniture3D    │                     │ │
│  │                   └──────────────────┘                     │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │                   OccupantManager                           │ │
│  │  ┌─────────────────────────────────────────────────────┐   │ │
│  │  │              HumanSilhouette (per occupant)          │   │ │
│  │  │  - Motion trail          - Uncertainty halo          │   │ │
│  │  │  - Prediction cone       - Scanning line             │   │ │
│  │  │  - Body + Head mesh      - Confidence bar            │   │ │
│  │  │  - Heading arrow         - Velocity vector           │   │ │
│  │  └─────────────────────────────────────────────────────┘   │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                  │
│  ┌─────────────────┐                                            │
│  │ SensorIndicator │                                            │
│  │ - Position      │                                            │
│  │ - FOV cone      │                                            │
│  └─────────────────┘                                            │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│                      StatsOverlay (HTML)                         │
│  - Occupant count + confidence bar                               │
│  - Connection status + frame rate                                │
│  - Tracking source breakdown (fused/radar/vision)                │
│  - Room dimensions + 3D scan badge                               │
│  - Legend (tracking sources, indicators, room geometry)          │
└─────────────────────────────────────────────────────────────────┘
```

## Component Reference

### Scene (`Scene.tsx`)

Main orchestrator component that sets up the 3D canvas and post-processing.

**Key Features:**
- Deep dark background (`#030308`) with fog for depth
- Blue-tinted directional lighting for futuristic aesthetic
- Bloom post-processing for glow effects on emissive materials
- Vignette for cinematic edge darkening
- Corner accent brackets (CSS overlay)

**Props:**
| Prop | Type | Description |
|------|------|-------------|
| `deviceUrl` | `string?` | Device API URL for live data |
| `demo` | `boolean` | Enable demo mode with simulated data |

### HumanSilhouette (`Occupants/HumanSilhouette.tsx`)

Renders a single tracked occupant with Tesla FSD-style visual indicators.

**Visual Elements:**

| Element | Description |
|---------|-------------|
| **Motion Trail** | Line connecting historical positions (last 20 points, 100ms intervals) |
| **Prediction Cone** | Amber dashed line + ring showing projected position (0.75s ahead) |
| **Uncertainty Halo** | Pulsing ring that grows with lower confidence (red below 50%) |
| **Scanning Line** | Rotating line effect (Tesla FSD signature) |
| **Base Ring** | Solid ring at feet indicating tracking source color |
| **Pulse Ring** | Animated expanding ring for visual emphasis |
| **Body Mesh** | Semi-transparent capsule with wireframe overlay |
| **Head Mesh** | Sphere with emissive glow |
| **Heading Arrow** | Direction indicator with cone head (shown when speed > 0.08 m/s) |
| **Velocity Vector** | Dashed white line from torso (shown when speed > 0.15 m/s) |
| **Confidence Bar** | Vertical bar showing tracking confidence (color-coded) |

**Color Coding:**
| Source | Color | Hex |
|--------|-------|-----|
| Fused (Radar + Vision) | Cyan | `#06b6d4` |
| Radar only | Blue | `#3b82f6` |
| Vision only | Green | `#22c55e` |
| Prediction | Amber | `#f59e0b` |
| Low confidence | Red | `#ef4444` |

### RoomFloor (`Room/RoomFloor.tsx`)

Grid floor with Tesla FSD-style radar sweep effect.

**Visual Elements:**

| Element | Description |
|---------|-------------|
| **Primary Grid** | 0.5m cells, blue lines (`#2563eb` sections, `#1a365d` cells) |
| **Secondary Grid** | Finer 0.25m grid for depth perception |
| **Radar Sweep** | GLSL shader rotating sweep effect from sensor position |
| **Pulse Rings** | Expanding rings emanating from sensor (3s cycle) |
| **Distance Rings** | Static rings at 2m, 4m, 6m from sensor |
| **Sensor Crosshair** | Center indicator at sensor location |

**Radar Sweep Shader:**
```glsl
// Calculates angle from center, creates rotating sweep with fade trail
float sweepPos = fract(time * 0.15);  // Rotation speed
float intensity = 1.0 - (dist / sweepWidth);  // Fade at edges
float trailIntensity = (1.0 - behindSweep / fadeLength) * 0.5;  // Trail behind sweep
```

### StatsOverlay (`UI/StatsOverlay.tsx`)

HTML overlay providing HUD-style information display.

**Sections:**

| Position | Content |
|----------|---------|
| Top-left | Large occupant count with confidence bar, corner brackets |
| Top-center | Tracking source breakdown pill (fused/radar/vision counts) |
| Top-right | Connection status (LIVE/STALE/OFFLINE), frame rate, frame number |
| Bottom-left | Room dimensions, 3D SCAN badge if CapturedRoom available |
| Bottom-center | OPTICWORKS RS-1 branding |
| Bottom-right | Legend (tracking sources, indicators, room geometry) |

## Data Flow

```
Device (30Hz)                Browser
┌──────────┐                ┌─────────────────────────────────────┐
│  Radar   │──┐             │                                     │
│ LD2450   │  │             │  useWorldState()                    │
└──────────┘  │  Fusion     │       ↓                             │
              ├───Engine────│→ WebRTC DataChannel "worldstate"    │
┌──────────┐  │             │       ↓                             │
│  Vision  │──┘             │  useWorldStateStore()               │
│  YOLOv8  │                │       ↓                             │
└──────────┘                │  OccupantManager                    │
                            │       ↓                             │
┌──────────┐                │  HumanSilhouette[] (one per object) │
│ RoomPlan │────REST API────│→ useRoomPlan() → useRoomPlanStore() │
│  Config  │                │       ↓                             │
└──────────┘                │  CapturedRoomRenderer / RoomFloor   │
                            └─────────────────────────────────────┘
```

## Demo Mode

Access the visualization demo at `/demo` route. Demo mode:

1. Skips device API calls
2. Uses `useWorldStateDemo()` hook to generate simulated occupants
3. Loads sample CapturedRoom geometry
4. Animates occupants with random movement patterns

**Demo Data Generation (`useWorldStateDemo.ts`):**
- 3 simulated occupants with different tracking sources
- Random walking patterns within room bounds
- Varying confidence levels
- 30Hz update rate matching real device

## Performance Considerations

| Optimization | Implementation |
|--------------|----------------|
| Mesh memoization | `useMemo` for geometry and materials |
| Lazy state init | `useState(() => ...)` for Three.js objects |
| Ref-based animation | `useFrame` updates refs, not state |
| Suspense boundaries | Async geometry loading |
| Fog culling | Objects fade at distance |
| Responsive DPR | `dpr={[1, 2]}` adapts to device |

## File Structure

```
ui/src/components/visualization/
├── Scene.tsx                 # Main scene orchestrator
├── Camera/
│   └── CameraController.tsx  # OrbitControls wrapper
├── Occupants/
│   ├── OccupantManager.tsx   # Renders all tracked objects
│   └── HumanSilhouette.tsx   # Individual occupant visualization
├── Room/
│   ├── RoomFloor.tsx         # Grid + radar sweep
│   ├── RoomBoundary.tsx      # Simple 2D boundary fallback
│   ├── CapturedRoomRenderer.tsx  # Apple RoomPlan 3D geometry
│   ├── Wall3D.tsx            # Wall surface rendering
│   ├── Door3D.tsx            # Door opening rendering
│   ├── Window3D.tsx          # Window rendering
│   └── Furniture3D.tsx       # Furniture bounding boxes
├── Sensor/
│   └── SensorIndicator.tsx   # Sensor position + FOV
└── UI/
    └── StatsOverlay.tsx      # HUD overlay
```

## Customization

### Changing Colors

Edit the `COLORS` constant in `HumanSilhouette.tsx`:

```typescript
const COLORS = {
  fused: "#06b6d4",      // Cyan
  radarOnly: "#3b82f6",  // Blue
  visionOnly: "#22c55e", // Green
  prediction: "#f59e0b", // Amber
  uncertainty: "#ef4444", // Red
};
```

### Adjusting Motion Trail

```typescript
const TRAIL_LENGTH = 20;           // Number of historical points
const TRAIL_UPDATE_INTERVAL = 100; // ms between captures
```

### Adjusting Prediction

```typescript
const PREDICTION_TIME = 0.75; // Seconds to project ahead
```

### Bloom Intensity

In `Scene.tsx`:
```tsx
<Bloom
  luminanceThreshold={0.2}  // Lower = more glow
  intensity={0.8}           // Glow strength
  radius={0.8}              // Glow spread
/>
```

## Related Documentation

- [WorldState Protocol](./WORLDSTATE_PROTOCOL.md) - Data format for tracked objects
- [RoomPlan API](./ROOMPLAN_API.md) - Room configuration and CapturedRoom data
- [Fusion Engine](./FUSION_ENGINE.md) - How radar + vision data is combined
