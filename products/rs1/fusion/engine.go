// Package fusion implements sensor fusion for combining camera and radar data.
package fusion

import (
	"fmt"
	"math"
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

// Azimuth returns the azimuth angle of this detection in degrees
func (d Detection) Azimuth() float64 {
	return Position{X: d.X, Y: d.Y}.Azimuth()
}

// Track represents a tracked object with Kalman filter state
type Track struct {
	ID               string
	Filter           *KalmanFilter
	ClassID          int // -1=unknown, 0=person, 1=vehicle, 2=animal
	CameraConfidence float64
	RadarConfidence  float64
	IsRadarOnly      bool
	IsVisionOnly     bool
	FirstSeen        time.Time
}

// FusedObject represents a tracked object after sensor fusion
type FusedObject struct {
	ID               string    `json:"id"`
	X                float64   `json:"x"`
	Y                float64   `json:"y"`
	VelocityX        float64   `json:"velocity_x"`
	VelocityY        float64   `json:"velocity_y"`
	Confidence       float64   `json:"confidence"`
	CameraConfidence float64   `json:"camera_confidence"`
	RadarConfidence  float64   `json:"radar_confidence"`
	IsRadarOnly      bool      `json:"is_radar_only"`
	IsVisionOnly     bool      `json:"is_vision_only"`
	LastSeen         time.Time `json:"last_seen"`
	FirstSeen        time.Time `json:"first_seen"`
}

// Engine performs sensor fusion between camera and radar detections
type Engine struct {
	mu            sync.RWMutex
	tracks        map[string]*Track
	running       bool
	stopCh        chan struct{}
	cameraInputCh chan []Detection
	radarInputCh  chan []Detection
	outputCh      chan []FusedObject

	// Configuration
	transform       *TransformMatrix
	timeoutDuration time.Duration

	// Track ID counter
	nextTrackID int
}

// NewEngine creates a new sensor fusion engine
func NewEngine() *Engine {
	return &Engine{
		tracks:          make(map[string]*Track),
		cameraInputCh:   make(chan []Detection, 10),
		radarInputCh:    make(chan []Detection, 10),
		outputCh:        make(chan []FusedObject, 10),
		stopCh:          make(chan struct{}),
		transform:       NewIdentityTransform(),
		timeoutDuration: 2 * time.Second,
		nextTrackID:     1,
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

// SetRoomTransform sets the sensor-to-room coordinate transformation
func (e *Engine) SetRoomTransform(t *TransformMatrix) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.transform = t
	logger.Info().Msg("room transform updated")
}

// GetObjects returns current tracked objects
func (e *Engine) GetObjects() []FusedObject {
	e.mu.RLock()
	defer e.mu.RUnlock()

	objects := make([]FusedObject, 0, len(e.tracks))
	for _, track := range e.tracks {
		x, y, vx, vy := track.Filter.State()

		// Apply room transform if set
		if e.transform != nil {
			pos := e.transform.Apply(Position{X: x, Y: y})
			x, y = pos.X, pos.Y
		}

		confidence := track.CameraConfidence
		if track.RadarConfidence > confidence {
			confidence = track.RadarConfidence
		}
		if track.CameraConfidence > 0 && track.RadarConfidence > 0 {
			// Both sensors see it - higher confidence
			confidence = (track.CameraConfidence + track.RadarConfidence) / 2
			confidence = math.Min(confidence*1.2, 1.0)
		}

		objects = append(objects, FusedObject{
			ID:               track.ID,
			X:                x,
			Y:                y,
			VelocityX:        vx,
			VelocityY:        vy,
			Confidence:       confidence,
			CameraConfidence: track.CameraConfidence,
			RadarConfidence:  track.RadarConfidence,
			IsRadarOnly:      track.IsRadarOnly,
			IsVisionOnly:     track.IsVisionOnly,
			LastSeen:         track.Filter.lastUpdate,
			FirstSeen:        track.FirstSeen,
		})
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
			e.predictAndCleanup()
			e.emitOutput()
		}
	}
}

// processCameraDetections integrates camera detections using Hungarian algorithm
func (e *Engine) processCameraDetections(detections []Detection) {
	e.mu.Lock()
	defer e.mu.Unlock()

	now := time.Now()

	// Get current track azimuths for association
	trackIDs := make([]string, 0, len(e.tracks))
	trackAzimuths := make([]float64, 0, len(e.tracks))
	for id, track := range e.tracks {
		x, y, _, _ := track.Filter.State()
		trackIDs = append(trackIDs, id)
		trackAzimuths = append(trackAzimuths, Position{X: x, Y: y}.Azimuth())
	}

	// Get detection azimuths
	detAzimuths := make([]float64, len(detections))
	for i, det := range detections {
		detAzimuths[i] = det.Azimuth()
	}

	// Associate detections with tracks
	associations, unmatchedDet, _ := AssociateDetections(detAzimuths, trackAzimuths)

	// Update matched tracks with camera data
	for _, assoc := range associations {
		det := detections[assoc.VisionIdx]
		trackID := trackIDs[assoc.RadarIdx]
		track := e.tracks[trackID]

		track.Filter.Predict(now)
		track.Filter.Update(det.X, det.Y)
		track.CameraConfidence = det.Confidence
		track.IsVisionOnly = track.RadarConfidence == 0
		track.IsRadarOnly = false
	}

	// Create new tracks for unmatched detections
	for _, detIdx := range unmatchedDet {
		det := detections[detIdx]
		e.createTrack(det, true, false)
	}
}

// processRadarDetections integrates radar detections using Hungarian algorithm
func (e *Engine) processRadarDetections(detections []Detection) {
	e.mu.Lock()
	defer e.mu.Unlock()

	now := time.Now()

	// Get current track azimuths for association
	trackIDs := make([]string, 0, len(e.tracks))
	trackAzimuths := make([]float64, 0, len(e.tracks))
	for id, track := range e.tracks {
		x, y, _, _ := track.Filter.State()
		trackIDs = append(trackIDs, id)
		trackAzimuths = append(trackAzimuths, Position{X: x, Y: y}.Azimuth())
	}

	// Get detection azimuths
	detAzimuths := make([]float64, len(detections))
	for i, det := range detections {
		detAzimuths[i] = det.Azimuth()
	}

	// Associate detections with tracks
	associations, unmatchedDet, _ := AssociateDetections(detAzimuths, trackAzimuths)

	// Update matched tracks with radar data
	for _, assoc := range associations {
		det := detections[assoc.VisionIdx]
		trackID := trackIDs[assoc.RadarIdx]
		track := e.tracks[trackID]

		track.Filter.Predict(now)
		track.Filter.Update(det.X, det.Y)
		track.RadarConfidence = det.Confidence
		track.IsRadarOnly = track.CameraConfidence == 0
		track.IsVisionOnly = false
	}

	// Create new tracks for unmatched detections
	for _, detIdx := range unmatchedDet {
		det := detections[detIdx]
		e.createTrack(det, false, true)
	}
}

// createTrack creates a new track from a detection
func (e *Engine) createTrack(det Detection, isVision, isRadar bool) {
	id := fmt.Sprintf("track_%d", e.nextTrackID)
	e.nextTrackID++

	track := &Track{
		ID:        id,
		Filter:    NewKalmanFilter(det.X, det.Y),
		ClassID:   -1, // Unknown
		FirstSeen: det.Timestamp,
	}

	if isVision {
		track.CameraConfidence = det.Confidence
		track.IsVisionOnly = true
	}
	if isRadar {
		track.RadarConfidence = det.Confidence
		track.IsRadarOnly = true
	}

	e.tracks[id] = track
	logger.Debug().
		Str("id", id).
		Float64("x", det.X).
		Float64("y", det.Y).
		Msg("created new track")
}

// predictAndCleanup advances all Kalman filters and removes stale tracks
func (e *Engine) predictAndCleanup() {
	e.mu.Lock()
	defer e.mu.Unlock()

	now := time.Now()
	toDelete := make([]string, 0)

	for id, track := range e.tracks {
		// Predict to current time
		track.Filter.Predict(now)

		// Mark as missed if not updated recently
		if now.Sub(track.Filter.lastUpdate) > 200*time.Millisecond {
			track.Filter.MarkMissed()
		}

		// Check if track should be deleted
		if track.Filter.ShouldDelete() {
			toDelete = append(toDelete, id)
		}

		// Also delete very old tracks
		if now.Sub(track.Filter.lastUpdate) > e.timeoutDuration {
			toDelete = append(toDelete, id)
		}

		// Decay confidence over time
		decay := 0.95
		track.CameraConfidence *= decay
		track.RadarConfidence *= decay
	}

	for _, id := range toDelete {
		delete(e.tracks, id)
		logger.Debug().Str("id", id).Msg("deleted stale track")
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
