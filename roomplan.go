package kvm

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jetkvm/kvm/products/rs1/fusion"
)

// RoomPlanRequest is the request body for uploading room configuration
type RoomPlanRequest struct {
	RoomWidth   float64     `json:"room_width" binding:"required,gt=0"`
	RoomHeight  float64     `json:"room_height" binding:"required,gt=0"`
	SensorPose  [16]float64 `json:"sensor_pose" binding:"required"`
	RoomPolygon []Point2D   `json:"room_polygon"`
	Obstacles   []Obstacle  `json:"obstacles"`
}

// Point2D represents a 2D point
type Point2D struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// Obstacle represents an obstacle in the room
type Obstacle struct {
	Type    string    `json:"type"`
	Polygon []Point2D `json:"polygon"`
}

// RoomPlanResponse is the response for room plan operations
type RoomPlanResponse struct {
	Success          bool        `json:"success"`
	Message          string      `json:"message,omitempty"`
	TransformApplied bool        `json:"transform_applied,omitempty"`
	RoomWidth        float64     `json:"room_width,omitempty"`
	RoomHeight       float64     `json:"room_height,omitempty"`
	SensorPose       [16]float64 `json:"sensor_pose,omitempty"`
	RoomPolygon      []Point2D   `json:"room_polygon,omitempty"`
	Obstacles        []Obstacle  `json:"obstacles,omitempty"`
	Configured       bool        `json:"configured"`
	LastUpdated      string      `json:"last_updated,omitempty"`
}

// RoomConfig stores the room configuration
type RoomConfig struct {
	Width       float64     `json:"width"`
	Height      float64     `json:"height"`
	SensorPose  [16]float64 `json:"sensor_pose"`
	RoomPolygon []Point2D   `json:"room_polygon,omitempty"`
	Obstacles   []Obstacle  `json:"obstacles,omitempty"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

// handleRoomPlanUpload handles POST /api/setup/roomplan
func handleRoomPlanUpload(c *gin.Context) {
	var req RoomPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, RoomPlanResponse{
			Success: false,
			Message: "Invalid request: " + err.Error(),
		})
		return
	}

	// Validate transform matrix
	if !isValidTransformMatrix(req.SensorPose) {
		c.JSON(http.StatusBadRequest, RoomPlanResponse{
			Success: false,
			Message: "Invalid transform matrix: last row must be [0, 0, 0, 1]",
		})
		return
	}

	// Store configuration
	roomConfig := &RoomConfig{
		Width:       req.RoomWidth,
		Height:      req.RoomHeight,
		SensorPose:  req.SensorPose,
		RoomPolygon: req.RoomPolygon,
		Obstacles:   req.Obstacles,
		UpdatedAt:   time.Now(),
	}

	// Save to config
	config.RoomConfig = roomConfig
	if err := SaveConfig(); err != nil {
		c.JSON(http.StatusInternalServerError, RoomPlanResponse{
			Success: false,
			Message: "Failed to save configuration: " + err.Error(),
		})
		return
	}

	// Apply transform to fusion engine
	transformApplied := applyRoomTransform(req.SensorPose)

	logger.Info().
		Float64("width", req.RoomWidth).
		Float64("height", req.RoomHeight).
		Bool("transform_applied", transformApplied).
		Msg("room plan configuration saved")

	c.JSON(http.StatusOK, RoomPlanResponse{
		Success:          true,
		Message:          "Room configuration saved",
		TransformApplied: transformApplied,
		Configured:       true,
	})
}

// handleGetRoomPlan handles GET /api/setup/roomplan
func handleGetRoomPlan(c *gin.Context) {
	if config.RoomConfig == nil {
		c.JSON(http.StatusOK, RoomPlanResponse{
			Success:    true,
			Configured: false,
			Message:    "Room plan not configured",
		})
		return
	}

	c.JSON(http.StatusOK, RoomPlanResponse{
		Success:     true,
		Configured:  true,
		RoomWidth:   config.RoomConfig.Width,
		RoomHeight:  config.RoomConfig.Height,
		SensorPose:  config.RoomConfig.SensorPose,
		RoomPolygon: config.RoomConfig.RoomPolygon,
		Obstacles:   config.RoomConfig.Obstacles,
		LastUpdated: config.RoomConfig.UpdatedAt.Format(time.RFC3339),
	})
}

// isValidTransformMatrix validates that the matrix has proper form
// Last row must be [0, 0, 0, 1] for a valid homogeneous transform
func isValidTransformMatrix(m [16]float64) bool {
	// Column-major layout: last row is m[3], m[7], m[11], m[15]
	return m[3] == 0 && m[7] == 0 && m[11] == 0 && m[15] == 1
}

// applyRoomTransform applies the sensor pose transform to the fusion engine
// Returns true if successfully applied
func applyRoomTransform(pose [16]float64) bool {
	if rs1WorldStateGetter == nil {
		// RS-1 product not initialized
		return false
	}

	// Create transform matrix and apply to fusion engine
	transform := &fusion.TransformMatrix{M: pose}

	// Get fusion engine and set transform
	// This will be set up when RS-1 product is initialized
	if rs1FusionEngineSetter != nil {
		rs1FusionEngineSetter(transform)
		return true
	}

	return false
}

// rs1FusionEngineSetter is called to set the transform on the fusion engine
var rs1FusionEngineSetter func(*fusion.TransformMatrix)

// SetRS1FusionEngineSetter sets the function to update fusion engine transform
func SetRS1FusionEngineSetter(setter func(*fusion.TransformMatrix)) {
	rs1FusionEngineSetter = setter
}
