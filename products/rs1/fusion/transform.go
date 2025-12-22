// Package fusion implements sensor fusion for combining camera and radar data.
package fusion

import "math"

// Position represents a 2D position in room coordinates (meters)
type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// DistanceTo calculates the Euclidean distance to another position
func (p Position) DistanceTo(other Position) float64 {
	dx := p.X - other.X
	dy := p.Y - other.Y
	return math.Sqrt(dx*dx + dy*dy)
}

// Azimuth returns the angle in degrees from positive Y axis (0=forward, positive=clockwise)
func (p Position) Azimuth() float64 {
	// atan2 returns angle from positive X axis, we want from positive Y
	// Also converting to degrees and making clockwise positive
	rad := math.Atan2(p.X, p.Y)
	deg := rad * 180.0 / math.Pi
	return deg
}

// TransformMatrix is a 4x4 homogeneous transformation matrix in column-major order.
// Used to transform positions from sensor coordinates to room coordinates.
//
// Column-major layout:
//
//	[m0  m4  m8  m12]   [Rx  Ux  Fx  Tx]
//	[m1  m5  m9  m13] = [Ry  Uy  Fy  Ty]
//	[m2  m6  m10 m14]   [Rz  Uz  Fz  Tz]
//	[m3  m7  m11 m15]   [0   0   0   1 ]
type TransformMatrix struct {
	M [16]float64 `json:"m"`
}

// NewIdentityTransform creates an identity transformation matrix
func NewIdentityTransform() *TransformMatrix {
	return &TransformMatrix{
		M: [16]float64{
			1, 0, 0, 0,
			0, 1, 0, 0,
			0, 0, 1, 0,
			0, 0, 0, 1,
		},
	}
}

// Apply transforms a position from sensor coordinates to room coordinates.
// This performs a 2D transformation assuming Z=0 in the XY plane.
func (t *TransformMatrix) Apply(sensor Position) Position {
	// Column-major multiplication for 2D case (assuming z=0, w=1)
	// x' = m0*x + m4*y + m12
	// y' = m1*x + m5*y + m13
	return Position{
		X: t.M[0]*sensor.X + t.M[4]*sensor.Y + t.M[12],
		Y: t.M[1]*sensor.X + t.M[5]*sensor.Y + t.M[13],
	}
}

// IsValid checks if the transform matrix is valid (last row should be [0,0,0,1])
func (t *TransformMatrix) IsValid() bool {
	return t.M[3] == 0 && t.M[7] == 0 && t.M[11] == 0 && t.M[15] == 1
}

// Translation returns the translation component of the matrix
func (t *TransformMatrix) Translation() Position {
	return Position{X: t.M[12], Y: t.M[13]}
}
