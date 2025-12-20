# RoomPlan API

The RS-1 accepts room configuration data from iPhone apps using Apple's RoomPlan framework. This enables accurate sensor-to-room coordinate transformation for WorldState output.

## Overview

```
iPhone (RoomPlan) → JSON Export → POST /api/setup/roomplan → Config Storage → Fusion Transform
```

The RoomPlan API:
- Receives room dimensions and sensor placement from iPhone
- Stores the 4x4 transformation matrix for coordinate conversion
- Updates the fusion engine in real-time

---

## API Endpoints

### Upload Room Configuration

**POST /api/setup/roomplan**

Upload room dimensions and sensor pose matrix.

#### Request

```http
POST /api/setup/roomplan HTTP/1.1
Content-Type: application/json
Authorization: Bearer <token>

{
  "room_width": 5.2,
  "room_height": 4.8,
  "sensor_pose": {
    "m": [
      0.866, 0.0, -0.5, 0.0,
      0.0, 1.0, 0.0, 0.0,
      0.5, 0.0, 0.866, 0.0,
      2.6, 0.0, 2.4, 1.0
    ]
  },
  "room_polygon": [
    {"x": 0.0, "y": 0.0},
    {"x": 5.2, "y": 0.0},
    {"x": 5.2, "y": 4.8},
    {"x": 0.0, "y": 4.8}
  ],
  "obstacles": [
    {
      "type": "furniture",
      "polygon": [
        {"x": 1.0, "y": 2.0},
        {"x": 2.0, "y": 2.0},
        {"x": 2.0, "y": 3.0},
        {"x": 1.0, "y": 3.0}
      ]
    }
  ]
}
```

#### Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `room_width` | float | Yes | Room width in meters |
| `room_height` | float | Yes | Room height (depth) in meters |
| `sensor_pose` | object | Yes | 4x4 transformation matrix |
| `room_polygon` | array | No | Room boundary vertices |
| `obstacles` | array | No | Furniture/obstacles to exclude |

#### Response

```json
{
  "success": true,
  "message": "Room configuration saved",
  "transform_applied": true
}
```

### Get Room Configuration

**GET /api/setup/roomplan**

Retrieve the current room configuration.

#### Response

```json
{
  "room_width": 5.2,
  "room_height": 4.8,
  "sensor_pose": {
    "m": [0.866, 0.0, -0.5, 0.0, 0.0, 1.0, 0.0, 0.0, 0.5, 0.0, 0.866, 0.0, 2.6, 0.0, 2.4, 1.0]
  },
  "room_polygon": [...],
  "configured": true,
  "last_updated": "2024-01-15T10:30:00Z"
}
```

---

## Sensor Pose Matrix

The `sensor_pose` is a 4x4 homogeneous transformation matrix in column-major order that transforms coordinates from sensor space to room space.

### Matrix Layout

```
[m0  m4  m8  m12]   [Rx  Ux  Fx  Tx]
[m1  m5  m9  m13] = [Ry  Uy  Fy  Ty]
[m2  m6  m10 m14]   [Rz  Uz  Fz  Tz]
[m3  m7  m11 m15]   [0   0   0   1 ]

R = Right vector (sensor X axis in room coords)
U = Up vector (sensor Y axis in room coords)
F = Forward vector (sensor Z axis in room coords)
T = Translation (sensor position in room coords)
```

### Example: Sensor on South Wall, Facing North

Sensor at (2.6m, 2.4m) on the wall, facing into the room (+Y direction):

```json
{
  "m": [
    1.0, 0.0, 0.0, 0.0,
    0.0, 1.0, 0.0, 0.0,
    0.0, 0.0, 1.0, 0.0,
    2.6, 0.0, 2.4, 1.0
  ]
}
```

### Example: Sensor on East Wall, Facing West

Sensor rotated 90° counter-clockwise:

```json
{
  "m": [
    0.0, 0.0, -1.0, 0.0,
    0.0, 1.0, 0.0, 0.0,
    1.0, 0.0, 0.0, 0.0,
    5.0, 0.0, 2.4, 1.0
  ]
}
```

---

## Go Implementation

### `roomplan.go`

```go
package kvm

import (
    "net/http"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/jetkvm/kvm/internal/fusion"
)

// RoomPlanRequest represents the incoming room configuration
type RoomPlanRequest struct {
    RoomWidth   float64           `json:"room_width" binding:"required,gt=0"`
    RoomHeight  float64           `json:"room_height" binding:"required,gt=0"`
    SensorPose  TransformMatrix   `json:"sensor_pose" binding:"required"`
    RoomPolygon []Point2D         `json:"room_polygon"`
    Obstacles   []Obstacle        `json:"obstacles"`
}

// TransformMatrix is a 4x4 matrix in column-major order
type TransformMatrix struct {
    M [16]float64 `json:"m" binding:"required,len=16"`
}

// Point2D represents a 2D coordinate
type Point2D struct {
    X float64 `json:"x"`
    Y float64 `json:"y"`
}

// Obstacle represents a region to exclude from tracking
type Obstacle struct {
    Type    string    `json:"type"`
    Polygon []Point2D `json:"polygon"`
}

// RoomConfig stored in config.go
type RoomConfig struct {
    Width       float64         `json:"width"`
    Height      float64         `json:"height"`
    SensorPose  TransformMatrix `json:"sensor_pose"`
    RoomPolygon []Point2D       `json:"room_polygon,omitempty"`
    Obstacles   []Obstacle      `json:"obstacles,omitempty"`
    UpdatedAt   time.Time       `json:"updated_at"`
}

func handleRoomPlanUpload(c *gin.Context) {
    var req RoomPlanRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{
            "success": false,
            "error":   err.Error(),
        })
        return
    }

    // Validate matrix (should be invertible)
    if !isValidTransform(req.SensorPose) {
        c.JSON(http.StatusBadRequest, gin.H{
            "success": false,
            "error":   "Invalid transformation matrix",
        })
        return
    }

    // Update config
    config.RoomConfig = &RoomConfig{
        Width:       req.RoomWidth,
        Height:      req.RoomHeight,
        SensorPose:  req.SensorPose,
        RoomPolygon: req.RoomPolygon,
        Obstacles:   req.Obstacles,
        UpdatedAt:   time.Now(),
    }

    if err := SaveConfig(); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{
            "success": false,
            "error":   "Failed to save configuration",
        })
        return
    }

    // Update fusion engine transform
    fusionTransform := &fusion.TransformMatrix{M: req.SensorPose.M}
    fusionEngine.SetRoomTransform(fusionTransform)

    logger.Info().
        Float64("width", req.RoomWidth).
        Float64("height", req.RoomHeight).
        Msg("Room configuration updated")

    c.JSON(http.StatusOK, gin.H{
        "success":           true,
        "message":           "Room configuration saved",
        "transform_applied": true,
    })
}

func handleGetRoomPlan(c *gin.Context) {
    if config.RoomConfig == nil {
        c.JSON(http.StatusOK, gin.H{
            "configured": false,
        })
        return
    }

    c.JSON(http.StatusOK, gin.H{
        "room_width":   config.RoomConfig.Width,
        "room_height":  config.RoomConfig.Height,
        "sensor_pose":  config.RoomConfig.SensorPose,
        "room_polygon": config.RoomConfig.RoomPolygon,
        "obstacles":    config.RoomConfig.Obstacles,
        "configured":   true,
        "last_updated": config.RoomConfig.UpdatedAt,
    })
}

func isValidTransform(t TransformMatrix) bool {
    // Check that the matrix is not degenerate
    // Simple check: last row should be [0,0,0,1]
    return t.M[3] == 0 && t.M[7] == 0 && t.M[11] == 0 && t.M[15] == 1
}
```

### Route Registration (`web.go`)

```go
func setupRoutes(r *gin.Engine) {
    // ... existing routes ...

    protected := r.Group("/")
    protected.Use(authMiddleware())
    {
        // RoomPlan API
        protected.POST("/api/setup/roomplan", handleRoomPlanUpload)
        protected.GET("/api/setup/roomplan", handleGetRoomPlan)
    }
}
```

---

## iPhone Integration

### Swift RoomPlan Export

```swift
import RoomPlan
import simd

struct RS1RoomConfig: Codable {
    let roomWidth: Double
    let roomHeight: Double
    let sensorPose: SensorPose
    let roomPolygon: [Point2D]

    enum CodingKeys: String, CodingKey {
        case roomWidth = "room_width"
        case roomHeight = "room_height"
        case sensorPose = "sensor_pose"
        case roomPolygon = "room_polygon"
    }
}

struct SensorPose: Codable {
    let m: [Double]
}

struct Point2D: Codable {
    let x: Double
    let y: Double
}

class RoomPlanExporter {
    func exportForRS1(capturedRoom: CapturedRoom,
                      sensorPosition: simd_float3,
                      sensorRotation: simd_quatf) -> RS1RoomConfig {

        // Calculate room bounds
        let bounds = calculateRoomBounds(capturedRoom)

        // Convert sensor pose to 4x4 matrix
        let rotationMatrix = simd_float4x4(sensorRotation)
        var poseMatrix = rotationMatrix
        poseMatrix.columns.3 = simd_float4(sensorPosition, 1.0)

        // Extract column-major array
        let m: [Double] = [
            Double(poseMatrix.columns.0.x), Double(poseMatrix.columns.0.y),
            Double(poseMatrix.columns.0.z), Double(poseMatrix.columns.0.w),
            Double(poseMatrix.columns.1.x), Double(poseMatrix.columns.1.y),
            Double(poseMatrix.columns.1.z), Double(poseMatrix.columns.1.w),
            Double(poseMatrix.columns.2.x), Double(poseMatrix.columns.2.y),
            Double(poseMatrix.columns.2.z), Double(poseMatrix.columns.2.w),
            Double(poseMatrix.columns.3.x), Double(poseMatrix.columns.3.y),
            Double(poseMatrix.columns.3.z), Double(poseMatrix.columns.3.w),
        ]

        // Extract room polygon
        let polygon = extractFloorPolygon(capturedRoom)

        return RS1RoomConfig(
            roomWidth: bounds.width,
            roomHeight: bounds.height,
            sensorPose: SensorPose(m: m),
            roomPolygon: polygon
        )
    }

    func uploadToRS1(config: RS1RoomConfig, deviceURL: URL) async throws {
        var request = URLRequest(url: deviceURL.appendingPathComponent("api/setup/roomplan"))
        request.httpMethod = "POST"
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        request.httpBody = try JSONEncoder().encode(config)

        let (_, response) = try await URLSession.shared.data(for: request)

        guard let httpResponse = response as? HTTPURLResponse,
              httpResponse.statusCode == 200 else {
            throw RS1Error.uploadFailed
        }
    }
}
```

---

## Coordinate Transformation

### Sensor to Room Coordinates

Given a point `P_sensor` in sensor coordinates, the room coordinate `P_room` is:

```
P_room = T_pose * P_sensor

where T_pose is the sensor_pose matrix
```

### Example Calculation

Sensor at (2.6, 0, 2.4) facing +Y, detecting object at sensor-local (1.0, 2.0):

```
T_pose = [1 0 0 2.6]    P_sensor = [1.0]
         [0 1 0 0  ]               [2.0]
         [0 0 1 2.4]               [0  ]
         [0 0 0 1  ]               [1  ]

P_room = T_pose * P_sensor
       = [1*1.0 + 0*2.0 + 0*0 + 2.6*1] = [3.6]
         [0*1.0 + 1*2.0 + 0*0 + 0*1  ]   [2.0]
         [0*1.0 + 0*2.0 + 1*0 + 2.4*1]   [2.4]
         [0 + 0 + 0 + 1              ]   [1  ]

Object is at room coordinates (3.6, 2.0)
```

---

## Detection Zones (Optional)

Configure areas of interest for triggering alerts:

### POST /api/detection-zones

```json
{
  "zones": [
    {
      "id": "entry",
      "name": "Entry Zone",
      "polygon": [
        {"x": 0.0, "y": 0.0},
        {"x": 2.0, "y": 0.0},
        {"x": 2.0, "y": 1.5},
        {"x": 0.0, "y": 1.5}
      ],
      "trigger_on_enter": true,
      "trigger_on_exit": false
    }
  ]
}
```

---

## Calibration Workflow

1. **Scan Room**: Use iPhone with RoomPlan to capture room geometry
2. **Place Sensor**: Position RS-1 on wall with known orientation
3. **Mark Sensor Position**: In iPhone app, tap on sensor location
4. **Export Configuration**: App calculates transform and uploads
5. **Verify Tracking**: Check WorldState coordinates match expected positions

---

## See Also

- [RS1_ARCHITECTURE.md](RS1_ARCHITECTURE.md) - System overview
- [WORLDSTATE_PROTOCOL.md](WORLDSTATE_PROTOCOL.md) - Output coordinate format
- [FUSION_ENGINE.md](FUSION_ENGINE.md) - How transforms are applied
