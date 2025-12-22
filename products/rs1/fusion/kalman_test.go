package fusion

import (
	"math"
	"testing"
	"time"
)

func TestNewKalmanFilter(t *testing.T) {
	kf := NewKalmanFilter(1.0, 2.0)

	x, y, vx, vy := kf.State()
	if x != 1.0 || y != 2.0 {
		t.Errorf("Initial position = (%v, %v), expected (1.0, 2.0)", x, y)
	}
	if vx != 0.0 || vy != 0.0 {
		t.Errorf("Initial velocity = (%v, %v), expected (0.0, 0.0)", vx, vy)
	}
}

func TestKalmanFilter_Position(t *testing.T) {
	kf := NewKalmanFilter(3.5, 4.5)

	pos := kf.Position()
	if pos.X != 3.5 || pos.Y != 4.5 {
		t.Errorf("Position() = %v, expected {3.5, 4.5}", pos)
	}
}

func TestKalmanFilter_Velocity(t *testing.T) {
	kf := NewKalmanFilter(0, 0)

	vx, vy := kf.Velocity()
	if vx != 0.0 || vy != 0.0 {
		t.Errorf("Velocity() = (%v, %v), expected (0.0, 0.0)", vx, vy)
	}
}

func TestKalmanFilter_Predict(t *testing.T) {
	kf := NewKalmanFilter(0, 0)

	// Manually set velocity by updating the state
	// Update with measurement at (1, 0) to establish velocity
	kf.Update(1.0, 0.0)

	// Predict with a 1 second time step
	futureTime := kf.lastUpdate.Add(1 * time.Second)
	kf.Predict(futureTime)

	// Position should have moved based on velocity estimate
	x, y, _, _ := kf.State()

	// The exact values depend on the Kalman math, but position should change
	if x == 0.0 {
		t.Error("Predict should have updated X position")
	}
	t.Logf("After predict: x=%v, y=%v", x, y)
}

func TestKalmanFilter_Update(t *testing.T) {
	kf := NewKalmanFilter(0, 0)

	// Update with measurement at (2, 3)
	kf.Update(2.0, 3.0)

	x, y, _, _ := kf.State()

	// Position should move towards measurement
	if x <= 0 || y <= 0 {
		t.Errorf("Update should move position towards measurement, got (%v, %v)", x, y)
	}

	// With multiple updates converging to same point
	for i := 0; i < 10; i++ {
		kf.Update(5.0, 5.0)
	}

	x, y, _, _ = kf.State()
	if math.Abs(x-5.0) > 0.5 || math.Abs(y-5.0) > 0.5 {
		t.Errorf("Position should converge to (5.0, 5.0), got (%v, %v)", x, y)
	}
}

func TestKalmanFilter_VelocityEstimation(t *testing.T) {
	kf := NewKalmanFilter(0, 0)

	// Simulate moving object: update with increasing positions
	baseTime := time.Now()
	kf.lastUpdate = baseTime

	// Move along X axis at 1 m/s
	for i := 1; i <= 10; i++ {
		kf.Predict(baseTime.Add(time.Duration(i) * 100 * time.Millisecond))
		kf.Update(float64(i)*0.1, 0) // Move 0.1m every 100ms = 1 m/s
	}

	vx, vy := kf.Velocity()

	// Velocity X should be positive (moving right)
	if vx <= 0 {
		t.Errorf("VelocityX should be positive for rightward motion, got %v", vx)
	}

	// Velocity Y should be near zero
	if math.Abs(vy) > 0.5 {
		t.Errorf("VelocityY should be near zero, got %v", vy)
	}

	t.Logf("Estimated velocity: vx=%v, vy=%v", vx, vy)
}

func TestKalmanFilter_MissedFrames(t *testing.T) {
	kf := NewKalmanFilter(0, 0)

	if kf.MissedFrames() != 0 {
		t.Error("Initial missed frames should be 0")
	}

	kf.MarkMissed()
	if kf.MissedFrames() != 1 {
		t.Errorf("MissedFrames() = %d, expected 1", kf.MissedFrames())
	}

	kf.MarkMissed()
	kf.MarkMissed()
	if kf.MissedFrames() != 3 {
		t.Errorf("MissedFrames() = %d, expected 3", kf.MissedFrames())
	}

	// Update should reset missed frames
	kf.Update(1, 1)
	if kf.MissedFrames() != 0 {
		t.Error("Update should reset missed frames to 0")
	}
}

func TestKalmanFilter_ShouldDelete(t *testing.T) {
	kf := NewKalmanFilter(0, 0)

	if kf.ShouldDelete() {
		t.Error("New filter should not be marked for deletion")
	}

	// Miss enough frames to trigger deletion
	for i := 0; i < 5; i++ {
		kf.MarkMissed()
	}

	if !kf.ShouldDelete() {
		t.Error("Filter should be marked for deletion after 5 missed frames")
	}
}

func TestKalmanFilter_PredictZeroDt(t *testing.T) {
	kf := NewKalmanFilter(1, 2)

	// Predict with same time should not change state
	now := kf.lastUpdate
	kf.Predict(now)

	x, y, _, _ := kf.State()
	if x != 1.0 || y != 2.0 {
		t.Errorf("Predict with zero dt should not change position, got (%v, %v)", x, y)
	}
}

func TestKalmanFilter_Speed(t *testing.T) {
	kf := NewKalmanFilter(0, 0)

	// Initial speed should be 0
	if kf.Speed() != 0 {
		t.Errorf("Initial speed should be 0, got %v", kf.Speed())
	}
}
