// Package fusion implements sensor fusion for combining camera and radar data.
package fusion

import (
	"time"

	"gonum.org/v1/gonum/mat"
)

// KalmanFilter implements a 2D Kalman filter for tracking objects.
// State vector: [px, py, vx, vy]^T
// Measurement vector: [px, py]^T
type KalmanFilter struct {
	// State vector [px, py, vx, vy]
	x *mat.VecDense

	// State covariance matrix (4x4)
	P *mat.Dense

	// Process noise covariance (4x4)
	Q *mat.Dense

	// Measurement noise covariance (2x2)
	R *mat.Dense

	// Measurement matrix H (2x4) - observes position only
	H *mat.Dense

	// Last update time for dt calculation
	lastUpdate time.Time

	// Track lifecycle
	missedFrames int
	maxMissed    int
}

// NewKalmanFilter creates a new Kalman filter initialized at position (x, y)
// with zero initial velocity.
func NewKalmanFilter(x, y float64) *KalmanFilter {
	kf := &KalmanFilter{
		// Initial state: position at (x,y), velocity = 0
		x: mat.NewVecDense(4, []float64{x, y, 0, 0}),

		// Initial covariance - higher uncertainty for velocity
		// P = diag(1, 1, 10, 10)
		P: mat.NewDense(4, 4, []float64{
			1, 0, 0, 0,
			0, 1, 0, 0,
			0, 0, 10, 0,
			0, 0, 0, 10,
		}),

		// Process noise
		// Q = diag(0.1, 0.1, 1.0, 1.0)
		Q: mat.NewDense(4, 4, []float64{
			0.1, 0, 0, 0,
			0, 0.1, 0, 0,
			0, 0, 1.0, 0,
			0, 0, 0, 1.0,
		}),

		// Measurement noise
		// R = diag(0.5, 0.5)
		R: mat.NewDense(2, 2, []float64{
			0.5, 0,
			0, 0.5,
		}),

		// Measurement matrix - observes only position
		// H = [[1, 0, 0, 0], [0, 1, 0, 0]]
		H: mat.NewDense(2, 4, []float64{
			1, 0, 0, 0,
			0, 1, 0, 0,
		}),

		lastUpdate:   time.Now(),
		missedFrames: 0,
		maxMissed:    5, // Delete track after 5 missed frames
	}

	return kf
}

// Predict advances the state estimate to the current time.
// Uses constant velocity motion model.
func (kf *KalmanFilter) Predict(now time.Time) {
	dt := now.Sub(kf.lastUpdate).Seconds()
	if dt <= 0 {
		return
	}
	kf.lastUpdate = now

	// State transition matrix F
	// F = [[1, 0, dt, 0 ],
	//      [0, 1, 0,  dt],
	//      [0, 0, 1,  0 ],
	//      [0, 0, 0,  1 ]]
	F := mat.NewDense(4, 4, []float64{
		1, 0, dt, 0,
		0, 1, 0, dt,
		0, 0, 1, 0,
		0, 0, 0, 1,
	})

	// x' = F * x
	newX := mat.NewVecDense(4, nil)
	newX.MulVec(F, kf.x)
	kf.x = newX

	// P' = F * P * F^T + Q
	var FP mat.Dense
	FP.Mul(F, kf.P)

	var FPFT mat.Dense
	FPFT.Mul(&FP, F.T())

	kf.P.Add(&FPFT, kf.Q)
}

// Update incorporates a measurement into the state estimate.
func (kf *KalmanFilter) Update(measX, measY float64) {
	kf.missedFrames = 0

	// Measurement vector z
	z := mat.NewVecDense(2, []float64{measX, measY})

	// Innovation: y = z - H * x
	var Hx mat.VecDense
	Hx.MulVec(kf.H, kf.x)

	y := mat.NewVecDense(2, nil)
	y.SubVec(z, &Hx)

	// Innovation covariance: S = H * P * H^T + R
	var HP mat.Dense
	HP.Mul(kf.H, kf.P)

	var HPHT mat.Dense
	HPHT.Mul(&HP, kf.H.T())

	S := mat.NewDense(2, 2, nil)
	S.Add(&HPHT, kf.R)

	// Kalman gain: K = P * H^T * S^-1
	var PHT mat.Dense
	PHT.Mul(kf.P, kf.H.T())

	var SInv mat.Dense
	err := SInv.Inverse(S)
	if err != nil {
		// If S is singular, skip update
		return
	}

	K := mat.NewDense(4, 2, nil)
	K.Mul(&PHT, &SInv)

	// Update state: x = x + K * y
	var Ky mat.VecDense
	Ky.MulVec(K, y)
	kf.x.AddVec(kf.x, &Ky)

	// Update covariance: P = (I - K*H) * P
	I := mat.NewDense(4, 4, []float64{
		1, 0, 0, 0,
		0, 1, 0, 0,
		0, 0, 1, 0,
		0, 0, 0, 1,
	})

	var KH mat.Dense
	KH.Mul(K, kf.H)

	var IminusKH mat.Dense
	IminusKH.Sub(I, &KH)

	var newP mat.Dense
	newP.Mul(&IminusKH, kf.P)
	kf.P = &newP
}

// State returns the current state estimate (x, y, vx, vy)
func (kf *KalmanFilter) State() (x, y, vx, vy float64) {
	return kf.x.AtVec(0), kf.x.AtVec(1), kf.x.AtVec(2), kf.x.AtVec(3)
}

// Position returns the current position estimate
func (kf *KalmanFilter) Position() Position {
	return Position{X: kf.x.AtVec(0), Y: kf.x.AtVec(1)}
}

// Velocity returns the current velocity estimate
func (kf *KalmanFilter) Velocity() (vx, vy float64) {
	return kf.x.AtVec(2), kf.x.AtVec(3)
}

// Speed returns the scalar speed (magnitude of velocity)
func (kf *KalmanFilter) Speed() float64 {
	vx, vy := kf.Velocity()
	return vx*vx + vy*vy // Return squared to avoid sqrt, caller can sqrt if needed
}

// MarkMissed marks this track as having missed a detection frame
func (kf *KalmanFilter) MarkMissed() {
	kf.missedFrames++
}

// ShouldDelete returns true if the track has missed too many frames
func (kf *KalmanFilter) ShouldDelete() bool {
	return kf.missedFrames >= kf.maxMissed
}

// MissedFrames returns the number of consecutive missed frames
func (kf *KalmanFilter) MissedFrames() int {
	return kf.missedFrames
}
