// Package fusion implements sensor fusion for combining camera and radar data.
package fusion

import (
	"sync"
	"time"

	"github.com/jetkvm/kvm/platform/logging"
)

var logger = logging.GetSubsystemLogger("fusion")

// DetectionSource indicates where a detection originated
type DetectionSource string

const (
	SourceCamera DetectionSource = "camera"
	SourceRadar  DetectionSource = "radar"
	SourceFusion DetectionSource = "fusion"
)

// Detection represents a single detection from any source
type Detection struct {
	ID         string          `json:"id"`
	X          float64         `json:"x"`          // Position X in meters
	Y          float64         `json:"y"`          // Position Y in meters
	Confidence float64         `json:"confidence"` // 0-1 confidence
	Source     DetectionSource `json:"source"`
	Timestamp  time.Time       `json:"timestamp"`
}

// FusedObject represents a tracked object after sensor fusion
type FusedObject struct {
	ID            string          `json:"id"`
	X             float64         `json:"x"`
	Y             float64         `json:"y"`
	VelocityX     float64         `json:"velocity_x"`
	VelocityY     float64         `json:"velocity_y"`
	Confidence    float64         `json:"confidence"`
	CameraConfidence float64      `json:"camera_confidence"`
	RadarConfidence  float64      `json:"radar_confidence"`
	LastSeen      time.Time       `json:"last_seen"`
	FirstSeen     time.Time       `json:"first_seen"`
}

// Engine performs sensor fusion between camera and radar detections
type Engine struct {
	mu             sync.RWMutex
	objects        map[string]*FusedObject
	running        bool
	stopCh         chan struct{}
	cameraInputCh  chan []Detection
	radarInputCh   chan []Detection
	outputCh       chan []FusedObject

	// Configuration
	associationThreshold float64 // Max distance to associate detections (meters)
	timeoutDuration      time.Duration
}

// NewEngine creates a new sensor fusion engine
func NewEngine() *Engine {
	return &Engine{
		objects:              make(map[string]*FusedObject),
		cameraInputCh:        make(chan []Detection, 10),
		radarInputCh:         make(chan []Detection, 10),
		outputCh:             make(chan []FusedObject, 10),
		stopCh:               make(chan struct{}),
		associationThreshold: 0.5, // 50cm
		timeoutDuration:      2 * time.Second,
	}
}

// Start begins the fusion processing loop
func (e *Engine) Start() {
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return
	}
	e.running = true
	e.stopCh = make(chan struct{})
	e.mu.Unlock()

	go e.processLoop()
	logger.Info().Msg("fusion engine started")
}

// Stop halts the fusion engine
func (e *Engine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.running {
		return
	}
	e.running = false
	close(e.stopCh)
	logger.Info().Msg("fusion engine stopped")
}

// SubmitCameraDetections adds camera detections to be fused
func (e *Engine) SubmitCameraDetections(detections []Detection) {
	select {
	case e.cameraInputCh <- detections:
	default:
		// Channel full, drop
	}
}

// SubmitRadarDetections adds radar detections to be fused
func (e *Engine) SubmitRadarDetections(detections []Detection) {
	select {
	case e.radarInputCh <- detections:
	default:
		// Channel full, drop
	}
}

// Output returns a channel of fused object updates
func (e *Engine) Output() <-chan []FusedObject {
	return e.outputCh
}

// GetObjects returns current tracked objects
func (e *Engine) GetObjects() []FusedObject {
	e.mu.RLock()
	defer e.mu.RUnlock()

	objects := make([]FusedObject, 0, len(e.objects))
	for _, obj := range e.objects {
		objects = append(objects, *obj)
	}
	return objects
}

// processLoop handles incoming detections and performs fusion
func (e *Engine) processLoop() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-e.stopCh:
			return

		case detections := <-e.cameraInputCh:
			e.processCameraDetections(detections)

		case detections := <-e.radarInputCh:
			e.processRadarDetections(detections)

		case <-ticker.C:
			e.cleanupStaleObjects()
			e.emitOutput()
		}
	}
}

// processCameraDetections integrates camera detections
func (e *Engine) processCameraDetections(detections []Detection) {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, det := range detections {
		obj := e.findNearestObject(det.X, det.Y)
		if obj != nil {
			// Update existing object with camera data
			e.updateObjectWithCamera(obj, det)
		} else {
			// Create new object
			e.createObject(det)
		}
	}
}

// processRadarDetections integrates radar detections
func (e *Engine) processRadarDetections(detections []Detection) {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, det := range detections {
		obj := e.findNearestObject(det.X, det.Y)
		if obj != nil {
			// Update existing object with radar data
			e.updateObjectWithRadar(obj, det)
		} else {
			// Create new object
			e.createObject(det)
		}
	}
}

// findNearestObject finds the closest tracked object within threshold
func (e *Engine) findNearestObject(x, y float64) *FusedObject {
	var nearest *FusedObject
	minDist := e.associationThreshold

	for _, obj := range e.objects {
		dx := obj.X - x
		dy := obj.Y - y
		dist := dx*dx + dy*dy // squared distance
		if dist < minDist*minDist {
			minDist = dist
			nearest = obj
		}
	}
	return nearest
}

// updateObjectWithCamera updates object position with camera detection
func (e *Engine) updateObjectWithCamera(obj *FusedObject, det Detection) {
	// Simple weighted average - camera more accurate for position
	alpha := 0.7
	obj.X = alpha*det.X + (1-alpha)*obj.X
	obj.Y = alpha*det.Y + (1-alpha)*obj.Y
	obj.CameraConfidence = det.Confidence
	obj.LastSeen = det.Timestamp
	obj.Confidence = (obj.CameraConfidence + obj.RadarConfidence) / 2
}

// updateObjectWithRadar updates object with radar detection
func (e *Engine) updateObjectWithRadar(obj *FusedObject, det Detection) {
	// Radar less accurate but provides velocity
	alpha := 0.3
	obj.X = alpha*det.X + (1-alpha)*obj.X
	obj.Y = alpha*det.Y + (1-alpha)*obj.Y
	obj.RadarConfidence = det.Confidence
	obj.LastSeen = det.Timestamp
	obj.Confidence = (obj.CameraConfidence + obj.RadarConfidence) / 2
}

// createObject creates a new tracked object from a detection
func (e *Engine) createObject(det Detection) {
	obj := &FusedObject{
		ID:        det.ID,
		X:         det.X,
		Y:         det.Y,
		Confidence: det.Confidence,
		FirstSeen: det.Timestamp,
		LastSeen:  det.Timestamp,
	}

	if det.Source == SourceCamera {
		obj.CameraConfidence = det.Confidence
	} else {
		obj.RadarConfidence = det.Confidence
	}

	e.objects[obj.ID] = obj
}

// cleanupStaleObjects removes objects not seen recently
func (e *Engine) cleanupStaleObjects() {
	e.mu.Lock()
	defer e.mu.Unlock()

	now := time.Now()
	for id, obj := range e.objects {
		if now.Sub(obj.LastSeen) > e.timeoutDuration {
			delete(e.objects, id)
		}
	}
}

// emitOutput sends current objects to output channel
func (e *Engine) emitOutput() {
	objects := e.GetObjects()
	select {
	case e.outputCh <- objects:
	default:
		// Channel full
	}
}
