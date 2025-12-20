package rs1

import (
	"sync"
	"time"
)

// WorldState represents the current room occupancy state
type WorldState struct {
	mu        sync.RWMutex
	occupants []Occupant
	zones     map[string]ZoneState
	timestamp time.Time
	running   bool
	stopCh    chan struct{}
}

// Occupant represents a detected person in the room
type Occupant struct {
	ID         string    `json:"id"`
	X          float64   `json:"x"`           // Position X in meters
	Y          float64   `json:"y"`           // Position Y in meters
	Confidence float64   `json:"confidence"`  // Detection confidence 0-1
	Source     string    `json:"source"`      // "camera", "radar", or "fusion"
	Velocity   *Velocity `json:"velocity,omitempty"` // Movement velocity
	LastSeen   time.Time `json:"last_seen"`
}

// Velocity represents movement speed and direction
type Velocity struct {
	X     float64 `json:"x"`     // X velocity m/s
	Y     float64 `json:"y"`     // Y velocity m/s
	Speed float64 `json:"speed"` // Scalar speed m/s
}

// ZoneState represents the state of a detection zone
type ZoneState struct {
	ZoneID       string    `json:"zone_id"`
	Occupied     bool      `json:"occupied"`
	OccupantIDs  []string  `json:"occupant_ids"`
	LastChanged  time.Time `json:"last_changed"`
	DwellTime    float64   `json:"dwell_time_sec"` // Time occupied in seconds
}

// WorldStateSnapshot is a point-in-time snapshot for streaming
type WorldStateSnapshot struct {
	Timestamp    time.Time            `json:"timestamp"`
	Occupants    []Occupant           `json:"occupants"`
	OccupantCount int                 `json:"occupant_count"`
	Zones        map[string]ZoneState `json:"zones"`
}

// NewWorldState creates a new WorldState instance
func NewWorldState() *WorldState {
	return &WorldState{
		occupants: make([]Occupant, 0),
		zones:     make(map[string]ZoneState),
		stopCh:    make(chan struct{}),
	}
}

// Start begins world state updates
func (w *WorldState) Start() {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return
	}
	w.running = true
	w.stopCh = make(chan struct{})
	w.mu.Unlock()

	go w.updateLoop()
	logger.Info().Msg("world state started")
}

// Stop halts world state updates
func (w *WorldState) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return
	}
	w.running = false
	close(w.stopCh)
	logger.Info().Msg("world state stopped")
}

// updateLoop periodically updates the world state
func (w *WorldState) updateLoop() {
	ticker := time.NewTicker(100 * time.Millisecond) // 10Hz update rate
	defer ticker.Stop()

	for {
		select {
		case <-w.stopCh:
			return
		case <-ticker.C:
			w.update()
		}
	}
}

// update processes new sensor data and updates state
func (w *WorldState) update() {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.timestamp = time.Now()
	// TODO: Process fusion engine output and update occupants/zones
}

// GetSnapshot returns the current world state snapshot
func (w *WorldState) GetSnapshot() WorldStateSnapshot {
	w.mu.RLock()
	defer w.mu.RUnlock()

	// Deep copy occupants
	occupants := make([]Occupant, len(w.occupants))
	copy(occupants, w.occupants)

	// Deep copy zones
	zones := make(map[string]ZoneState, len(w.zones))
	for k, v := range w.zones {
		zones[k] = v
	}

	return WorldStateSnapshot{
		Timestamp:     w.timestamp,
		Occupants:     occupants,
		OccupantCount: len(occupants),
		Zones:         zones,
	}
}

// UpdateOccupants updates the occupant list from sensor data
func (w *WorldState) UpdateOccupants(occupants []Occupant) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.occupants = occupants
	w.timestamp = time.Now()
}

// GetOccupantCount returns the current number of detected occupants
func (w *WorldState) GetOccupantCount() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return len(w.occupants)
}
