# RoomPlan Integration Testing Plan

Hardware-in-the-loop testing scheme for RS-1 RoomPlan integration using iPhone LiDAR devices without a custom React Native app.

## Overview

This document outlines methods to capture and test real Apple RoomPlan data against the RS-1 system before the production React Native app is available. The approach uses third-party apps, Apple sample code, and structured test fixtures.

## Hardware Requirements

### iPhone with LiDAR

RoomPlan requires a LiDAR-equipped device running iOS 16+:

| Device | LiDAR | RoomPlan Support |
|--------|-------|------------------|
| iPhone 12 Pro / Pro Max | Yes | iOS 16+ |
| iPhone 13 Pro / Pro Max | Yes | iOS 16+ |
| iPhone 14 Pro / Pro Max | Yes | iOS 16+ |
| iPhone 15 Pro / Pro Max | Yes | iOS 17+ (enhanced) |
| iPhone 16 Pro / Pro Max | Yes | iOS 18+ (latest) |
| iPad Pro (2020+) | Yes | iOS 16+ |

**Recommended**: iPhone 15 Pro Max or newer for iOS 17+ enhancements (multi-room scanning, improved object detection, custom AR session support).

---

## Testing Approaches (No Custom App Required)

### Approach 1: Third-Party RoomPlan Apps (Fastest Setup)

Several App Store apps export CapturedRoom data in usable formats:

#### **Mappedin Scan** (Recommended)
- **App Store**: [Mappedin Scan](https://apps.apple.com/app/mappedin-scan/id6449945802)
- **Export Formats**: PDF, SVG, PNG, USDZ, **GeoJSON**
- **Use Case**: GeoJSON export provides structured room geometry that can be converted to our CapturedRoom format
- **Workflow**:
  1. Scan room with Mappedin Scan
  2. Export as GeoJSON
  3. Convert to CapturedRoom JSON using converter script (see below)
  4. POST to RS-1 `/api/setup/roomplan`

#### **RoomPlan - Interior 3D Scanner** (by Heliogram Labs)
- **App Store**: [RoomPlan App](https://apps.apple.com/us/app/roomplan-interior-3d-scanner/id1658425563)
- **Export Formats**: USDZ, USD, OBJ (with subscription)
- **Use Case**: Direct USDZ export; requires metadata extraction
- **Limitation**: No direct JSON export; need to parse USDZ metadata

#### **AI Room Plan Scanner**
- **App Store**: [AI Room Plan Scanner](https://apps.apple.com/us/app/ai-room-plan-scanner/id6741044530)
- **Export Formats**: USDZ, OBJ, STL, DAE
- **Use Case**: Multiple format options for debugging

### Approach 2: Apple Sample App (Best for Development)

Apple provides a sample Xcode project that exports CapturedRoom as JSON directly.

#### Setup Steps

1. **Download Apple Sample Code**
   ```bash
   # Clone the WWDC23 sample (includes JSON export)
   git clone https://github.com/gromb57/ios-wwdc23__ProvidingCustomModelsForCapturedRoomsAndStructureExports.git
   cd ios-wwdc23__ProvidingCustomModelsForCapturedRoomsAndStructureExports
   ```

2. **Build and Run**
   - Open in Xcode 15+
   - Select your LiDAR-equipped device
   - Build and run (requires Apple Developer account for device deployment)

3. **Export CapturedRoom JSON**
   - Scan a room using the app
   - Use the "Export" function
   - Select JSON format
   - AirDrop or save to Files

4. **Use with RS-1**
   ```bash
   # Transfer JSON to development machine
   # Use test script to POST to RS-1
   ./scripts/test_roomplan.sh <device-ip> room_scan.json
   ```

### Approach 3: Custom Minimal Test App (Most Control)

Create a minimal Swift app specifically for RS-1 testing:

```swift
// RS1RoomPlanTest/ContentView.swift
import SwiftUI
import RoomPlan

struct ContentView: View {
    @State private var capturedRoom: CapturedRoom?
    @State private var showScanner = false
    @State private var rs1Host = "192.168.1.100"

    var body: some View {
        VStack(spacing: 20) {
            TextField("RS-1 IP Address", text: $rs1Host)
                .textFieldStyle(.roundedBorder)
                .padding()

            Button("Scan Room") {
                showScanner = true
            }
            .buttonStyle(.borderedProminent)

            if capturedRoom != nil {
                Button("Upload to RS-1") {
                    Task { await uploadToRS1() }
                }
                .buttonStyle(.bordered)

                Button("Export JSON") {
                    exportJSON()
                }
            }
        }
        .sheet(isPresented: $showScanner) {
            RoomCaptureViewWrapper(capturedRoom: $capturedRoom)
        }
    }

    func uploadToRS1() async {
        guard let room = capturedRoom else { return }

        // Encode CapturedRoom to JSON
        let encoder = JSONEncoder()
        encoder.outputFormatting = .prettyPrinted

        guard let jsonData = try? encoder.encode(room) else { return }

        // Create RS-1 config from CapturedRoom
        let config = createRS1Config(from: room)
        guard let configData = try? encoder.encode(config) else { return }

        // Upload to RS-1
        var request = URLRequest(url: URL(string: "http://\(rs1Host)/api/setup/roomplan")!)
        request.httpMethod = "POST"
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        request.httpBody = configData

        do {
            let (_, response) = try await URLSession.shared.data(for: request)
            if let httpResponse = response as? HTTPURLResponse {
                print("Upload status: \(httpResponse.statusCode)")
            }
        } catch {
            print("Upload failed: \(error)")
        }
    }

    func exportJSON() {
        guard let room = capturedRoom else { return }
        let encoder = JSONEncoder()
        encoder.outputFormatting = .prettyPrinted

        if let data = try? encoder.encode(room),
           let jsonString = String(data: data, encoding: .utf8) {
            // Save to Files app
            let url = FileManager.default.temporaryDirectory
                .appendingPathComponent("captured_room.json")
            try? jsonString.write(to: url, atomically: true, encoding: .utf8)

            // Share via activity controller
            let activityVC = UIActivityViewController(
                activityItems: [url],
                applicationActivities: nil
            )
            // Present activity controller...
        }
    }

    func createRS1Config(from room: CapturedRoom) -> RS1RoomConfig {
        // Calculate room bounds from walls
        var minX = Double.infinity, maxX = -Double.infinity
        var minZ = Double.infinity, maxZ = -Double.infinity

        for wall in room.walls {
            let pos = wall.transform.columns.3
            let halfWidth = wall.dimensions.x / 2
            let halfDepth = wall.dimensions.z / 2

            minX = min(minX, Double(pos.x) - Double(halfWidth))
            maxX = max(maxX, Double(pos.x) + Double(halfWidth))
            minZ = min(minZ, Double(pos.z) - Double(halfDepth))
            maxZ = max(maxZ, Double(pos.z) + Double(halfDepth))
        }

        let width = maxX - minX
        let depth = maxZ - minZ

        // Default sensor pose (identity + centered)
        let sensorPose: [Double] = [
            1, 0, 0, 0,
            0, 1, 0, 0,
            0, 0, 1, 0,
            width/2, 0, depth/2, 1
        ]

        return RS1RoomConfig(
            roomWidth: width,
            roomHeight: depth,
            sensorPose: SensorPose(m: sensorPose),
            roomPolygon: [],
            capturedRoom: room  // Include full CapturedRoom for 3D viz
        )
    }
}

struct RoomCaptureViewWrapper: UIViewControllerRepresentable {
    @Binding var capturedRoom: CapturedRoom?

    func makeUIViewController(context: Context) -> RoomCaptureViewController {
        let vc = RoomCaptureViewController()
        vc.delegate = context.coordinator
        return vc
    }

    func updateUIViewController(_ uiViewController: RoomCaptureViewController, context: Context) {}

    func makeCoordinator() -> Coordinator {
        Coordinator(self)
    }

    class Coordinator: NSObject, RoomCaptureViewDelegate {
        var parent: RoomCaptureViewWrapper

        init(_ parent: RoomCaptureViewWrapper) {
            self.parent = parent
        }

        func captureView(shouldPresent roomDataForProcessing: CapturedRoomData, error: Error?) -> Bool {
            return true
        }

        func captureView(didPresent processedResult: CapturedRoom, error: Error?) {
            parent.capturedRoom = processedResult
        }
    }
}
```

---

## Hardware-in-the-Loop Test Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                        Test Orchestration Host                          │
│                     (macOS with Xcode / Linux CI)                       │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────────────────┐  │
│  │  Test Runner │    │   Fixture    │    │    Results Collector     │  │
│  │   (pytest/   │    │   Library    │    │   (JSON reports,         │  │
│  │    go test)  │    │  (JSON/USDZ) │    │    screenshots)          │  │
│  └──────┬───────┘    └──────┬───────┘    └────────────┬─────────────┘  │
│         │                   │                         │                 │
└─────────┼───────────────────┼─────────────────────────┼─────────────────┘
          │                   │                         │
          │ HTTP POST         │ Read fixtures           │ Capture results
          │ /api/setup/       │                         │
          ▼                   ▼                         ▼
┌─────────────────────────────────────────────────────────────────────────┐
│                          RS-1 Device (DUT)                              │
│                        192.168.1.x / mDNS                               │
├─────────────────────────────────────────────────────────────────────────┤
│  RoomPlan API ──► Config Storage ──► Fusion Engine ──► WorldState       │
│                                                                         │
│  Radar Input ───────────────────────► Fusion ─────────► WebRTC Output   │
└─────────────────────────────────────────────────────────────────────────┘
          ▲
          │ WebRTC DataChannel
          │ (worldstate stream)
          │
┌─────────┴───────────────────────────────────────────────────────────────┐
│                        Test Verification Client                         │
│                   (Browser / Headless Chrome / Go client)               │
├─────────────────────────────────────────────────────────────────────────┤
│  - Subscribes to WorldState stream                                      │
│  - Validates coordinate transformations                                 │
│  - Captures 3D visualization screenshots                                │
│  - Reports tracking accuracy vs ground truth                            │
└─────────────────────────────────────────────────────────────────────────┘
          ▲
          │ Live scan input
          │
┌─────────┴───────────────────────────────────────────────────────────────┐
│                     iPhone (LiDAR Device)                               │
│                    iPhone 15 Pro Max / iOS 17+                          │
├─────────────────────────────────────────────────────────────────────────┤
│  - Runs test app or third-party scanner                                 │
│  - Captures room geometry                                               │
│  - Exports CapturedRoom JSON                                            │
│  - Optional: Direct HTTP POST to RS-1                                   │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## Test Fixture Library

### Directory Structure

```
testdata/
└── roomplan/
    ├── fixtures/
    │   ├── simple_rectangle.json      # 4x5m empty room
    │   ├── l_shaped_room.json          # L-shaped floor plan
    │   ├── living_room_furnished.json  # Room with furniture
    │   ├── office_cubicles.json        # Complex office layout
    │   ├── multi_room_house.json       # iOS 17+ StructureBuilder
    │   └── real_scans/                 # Actual iPhone captures
    │       ├── conference_room_a.json
    │       ├── lobby.json
    │       └── test_lab.json
    ├── expected/
    │   ├── simple_rectangle_bounds.json
    │   └── ...
    └── invalid/
        ├── missing_walls.json
        ├── invalid_transform.json
        └── empty_room.json
```

### Sample Test Fixture

```json
{
  "identifier": "test-room-001",
  "surfaces": [
    {
      "identifier": "wall-north",
      "category": "wall",
      "dimensions": [5.0, 2.4, 0.15],
      "transform": [
        1, 0, 0, 0,
        0, 1, 0, 0,
        0, 0, 1, 0,
        2.5, 1.2, 4.0, 1
      ],
      "confidence": "high"
    },
    {
      "identifier": "wall-south",
      "category": "wall",
      "dimensions": [5.0, 2.4, 0.15],
      "transform": [
        1, 0, 0, 0,
        0, 1, 0, 0,
        0, 0, 1, 0,
        2.5, 1.2, 0.0, 1
      ],
      "confidence": "high"
    },
    {
      "identifier": "wall-east",
      "category": "wall",
      "dimensions": [4.0, 2.4, 0.15],
      "transform": [
        0, 0, 1, 0,
        0, 1, 0, 0,
        -1, 0, 0, 0,
        5.0, 1.2, 2.0, 1
      ],
      "confidence": "high"
    },
    {
      "identifier": "wall-west",
      "category": "wall",
      "dimensions": [4.0, 2.4, 0.15],
      "transform": [
        0, 0, 1, 0,
        0, 1, 0, 0,
        -1, 0, 0, 0,
        0.0, 1.2, 2.0, 1
      ],
      "confidence": "high"
    },
    {
      "identifier": "door-main",
      "category": "door",
      "dimensions": [0.9, 2.1, 0.1],
      "transform": [
        1, 0, 0, 0,
        0, 1, 0, 0,
        0, 0, 1, 0,
        1.0, 1.05, 0.0, 1
      ],
      "confidence": "medium",
      "parent": "wall-south"
    }
  ],
  "objects": [
    {
      "identifier": "table-1",
      "category": "table",
      "dimensions": [1.5, 0.75, 0.9],
      "transform": [
        1, 0, 0, 0,
        0, 1, 0, 0,
        0, 0, 1, 0,
        2.5, 0.375, 2.0, 1
      ],
      "confidence": "high"
    },
    {
      "identifier": "chair-1",
      "category": "chair",
      "dimensions": [0.5, 0.9, 0.5],
      "transform": [
        1, 0, 0, 0,
        0, 1, 0, 0,
        0, 0, 1, 0,
        2.5, 0.45, 1.2, 1
      ],
      "confidence": "medium"
    }
  ],
  "floors": [
    {
      "identifier": "floor-main",
      "polygonCorners": [
        [0, 0, 0],
        [5, 0, 0],
        [5, 0, 4],
        [0, 0, 4]
      ]
    }
  ]
}
```

---

## Test Scripts

### `scripts/test_roomplan.sh`

```bash
#!/bin/bash
# Upload RoomPlan fixture to RS-1 and verify

DEVICE_IP="${1:-192.168.1.100}"
FIXTURE="${2:-testdata/roomplan/fixtures/simple_rectangle.json}"

echo "Uploading $FIXTURE to RS-1 at $DEVICE_IP..."

# Parse CapturedRoom and create RS-1 config
CONFIG=$(cat "$FIXTURE" | jq '{
  room_width: (.surfaces | map(select(.category == "wall")) |
               [.[].transform[12]] | (max - min) +
               ([.[].dimensions[0]] | max)),
  room_height: (.surfaces | map(select(.category == "wall")) |
                [.[].transform[14]] | (max - min) +
                ([.[].dimensions[2]] | max)),
  sensor_pose: {
    m: [1,0,0,0, 0,1,0,0, 0,0,1,0, 2.5,0,2,1]
  },
  captured_room: .
}')

# Upload to RS-1
RESPONSE=$(curl -s -X POST "http://$DEVICE_IP/api/setup/roomplan" \
  -H "Content-Type: application/json" \
  -d "$CONFIG")

echo "Response: $RESPONSE"

# Verify configuration was applied
VERIFY=$(curl -s "http://$DEVICE_IP/api/setup/roomplan")
echo "Verification: $VERIFY"
```

### Go Integration Test

```go
// roomplan_integration_test.go
//go:build integration

package integration

import (
    "encoding/json"
    "net/http"
    "os"
    "testing"
    "time"
)

func TestRoomPlanUpload(t *testing.T) {
    deviceIP := os.Getenv("RS1_DEVICE_IP")
    if deviceIP == "" {
        t.Skip("RS1_DEVICE_IP not set")
    }

    fixtures := []string{
        "testdata/roomplan/fixtures/simple_rectangle.json",
        "testdata/roomplan/fixtures/l_shaped_room.json",
        "testdata/roomplan/fixtures/living_room_furnished.json",
    }

    for _, fixture := range fixtures {
        t.Run(fixture, func(t *testing.T) {
            data, err := os.ReadFile(fixture)
            if err != nil {
                t.Fatalf("Failed to read fixture: %v", err)
            }

            // Create RS-1 config from CapturedRoom
            config := createRS1Config(data)

            // Upload
            resp, err := http.Post(
                "http://"+deviceIP+"/api/setup/roomplan",
                "application/json",
                bytes.NewReader(config),
            )
            if err != nil {
                t.Fatalf("Upload failed: %v", err)
            }
            defer resp.Body.Close()

            if resp.StatusCode != 200 {
                t.Errorf("Expected 200, got %d", resp.StatusCode)
            }

            // Verify
            time.Sleep(100 * time.Millisecond)
            verifyResp, _ := http.Get("http://" + deviceIP + "/api/setup/roomplan")
            // ... validate response
        })
    }
}

func TestWorldStateTransforms(t *testing.T) {
    // Test that WorldState coordinates are correctly transformed
    // after RoomPlan upload
}
```

---

## Test Workflow: Manual Hardware-in-the-Loop

### Phase 1: Capture Test Fixtures (One-time)

1. **Setup**
   - Install Apple RoomPlan sample app or Mappedin Scan on iPhone
   - Identify 3-5 representative test rooms

2. **Capture**
   - Scan each room following RoomPlan best practices:
     - Lighting > 50 lux
     - < 5 minute scan duration
     - Cover all walls systematically
   - Export as JSON/USDZ

3. **Archive**
   - Transfer to `testdata/roomplan/real_scans/`
   - Document room characteristics (size, furniture, conditions)

### Phase 2: Automated Regression Tests

```bash
# Run full RoomPlan integration suite
RS1_DEVICE_IP=192.168.1.100 go test -tags integration ./tests/roomplan/...

# Or via make
make test_roomplan DEVICE_IP=192.168.1.100
```

### Phase 3: Visual Verification

1. Open RS-1 web UI at `http://<device-ip>`
2. Navigate to 3D visualization
3. Verify:
   - Walls render correctly
   - Furniture positions match scan
   - Occupant tracking uses room coordinates

---

## Live iPhone Testing Workflow

For real-time testing during development:

```
┌────────────────┐     ┌────────────────┐     ┌────────────────┐
│   iPhone       │     │    RS-1        │     │  Dev Machine   │
│   (Scanner)    │     │    Device      │     │  (Browser)     │
└───────┬────────┘     └───────┬────────┘     └───────┬────────┘
        │                      │                      │
        │  1. Scan room        │                      │
        │ ──────────────►      │                      │
        │                      │                      │
        │  2. POST /api/setup/roomplan               │
        │ ─────────────────────►                      │
        │                      │                      │
        │                      │  3. Observe 3D viz  │
        │                      │ ◄────────────────────
        │                      │                      │
        │  4. Walk through room│                      │
        │ ──────────────────────►                     │
        │                      │                      │
        │                      │  5. Verify tracking │
        │                      │ ◄────────────────────
        │                      │                      │
```

---

## Validation Checklist

### RoomPlan Upload Validation

- [ ] Room dimensions extracted correctly from walls
- [ ] Sensor pose matrix is valid (last row = [0,0,0,1])
- [ ] CapturedRoom JSON stored and retrievable via GET
- [ ] Fusion engine receives transform update
- [ ] Config persists across device restart

### 3D Visualization Validation

- [ ] Walls render with correct dimensions
- [ ] Doors/windows appear in correct positions
- [ ] Furniture objects placed correctly
- [ ] Floor grid aligns with room bounds
- [ ] Sensor indicator shows correct position

### WorldState Validation

- [ ] Detected occupants use room coordinates
- [ ] Coordinates match expected transform output
- [ ] Tracking continues after RoomPlan update

---

## Troubleshooting

### Common Issues

| Issue | Cause | Resolution |
|-------|-------|------------|
| "Invalid transformation matrix" | Last row not [0,0,0,1] | Check matrix format is column-major |
| Empty walls array | Incomplete scan | Re-scan with better lighting |
| 3D viz shows nothing | CapturedRoom not in response | Verify `captured_room` field in POST |
| Coordinates inverted | X/Z axis swap | Check coordinate system conversion |

### Debug Logging

```bash
# Enable RoomPlan trace logging on RS-1
LOG_TRACE_SCOPES="roomplan,fusion" ./opticworks-rs1_app
```

---

## References

- [Apple RoomPlan Documentation](https://developer.apple.com/documentation/roomplan/)
- [WWDC22: Create parametric 3D room scans](https://developer.apple.com/videos/play/wwdc2022/10127/)
- [WWDC23: Explore enhancements to RoomPlan](https://developer.apple.com/videos/play/wwdc2023/10192/)
- [GitHub: Apple WWDC23 Sample Code](https://github.com/gromb57/ios-wwdc23__ProvidingCustomModelsForCapturedRoomsAndStructureExports)
- [Mappedin Scan App](https://www.mappedin.com/resources/blog/in-the-spotlight-mappedin-ios-app-for-free-scan-to-floor-plan/)

---

## See Also

- [ROOMPLAN_API.md](ROOMPLAN_API.md) - API specification
- [../platform/TESTING.md](../platform/TESTING.md) - General testing methodology
- [WORLDSTATE_PROTOCOL.md](WORLDSTATE_PROTOCOL.md) - Output coordinate format
