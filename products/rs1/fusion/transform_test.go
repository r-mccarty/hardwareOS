package fusion

import (
	"math"
	"testing"
)

func TestPosition_DistanceTo(t *testing.T) {
	tests := []struct {
		name     string
		p1       Position
		p2       Position
		expected float64
	}{
		{
			name:     "same point",
			p1:       Position{X: 0, Y: 0},
			p2:       Position{X: 0, Y: 0},
			expected: 0,
		},
		{
			name:     "unit distance X",
			p1:       Position{X: 0, Y: 0},
			p2:       Position{X: 1, Y: 0},
			expected: 1,
		},
		{
			name:     "unit distance Y",
			p1:       Position{X: 0, Y: 0},
			p2:       Position{X: 0, Y: 1},
			expected: 1,
		},
		{
			name:     "3-4-5 triangle",
			p1:       Position{X: 0, Y: 0},
			p2:       Position{X: 3, Y: 4},
			expected: 5,
		},
		{
			name:     "negative coordinates",
			p1:       Position{X: -1, Y: -1},
			p2:       Position{X: 2, Y: 3},
			expected: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.p1.DistanceTo(tt.p2)
			if math.Abs(result-tt.expected) > 1e-9 {
				t.Errorf("DistanceTo() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestPosition_Azimuth(t *testing.T) {
	tests := []struct {
		name     string
		p        Position
		expected float64
	}{
		{
			name:     "forward (positive Y)",
			p:        Position{X: 0, Y: 1},
			expected: 0,
		},
		{
			name:     "right (positive X)",
			p:        Position{X: 1, Y: 0},
			expected: 90,
		},
		{
			name:     "left (negative X)",
			p:        Position{X: -1, Y: 0},
			expected: -90,
		},
		{
			name:     "45 degrees right",
			p:        Position{X: 1, Y: 1},
			expected: 45,
		},
		{
			name:     "45 degrees left",
			p:        Position{X: -1, Y: 1},
			expected: -45,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.p.Azimuth()
			if math.Abs(result-tt.expected) > 1e-9 {
				t.Errorf("Azimuth() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestNewIdentityTransform(t *testing.T) {
	tf := NewIdentityTransform()

	// Check identity matrix
	expected := [16]float64{
		1, 0, 0, 0,
		0, 1, 0, 0,
		0, 0, 1, 0,
		0, 0, 0, 1,
	}

	for i, v := range expected {
		if tf.M[i] != v {
			t.Errorf("M[%d] = %v, expected %v", i, tf.M[i], v)
		}
	}
}

func TestTransformMatrix_Apply(t *testing.T) {
	tests := []struct {
		name     string
		tf       *TransformMatrix
		input    Position
		expected Position
	}{
		{
			name:     "identity transform",
			tf:       NewIdentityTransform(),
			input:    Position{X: 3, Y: 4},
			expected: Position{X: 3, Y: 4},
		},
		{
			name: "translation only",
			tf: &TransformMatrix{
				M: [16]float64{
					1, 0, 0, 0,
					0, 1, 0, 0,
					0, 0, 1, 0,
					2, 3, 0, 1, // Translate by (2, 3)
				},
			},
			input:    Position{X: 1, Y: 1},
			expected: Position{X: 3, Y: 4},
		},
		{
			name: "90 degree rotation",
			tf: &TransformMatrix{
				M: [16]float64{
					0, 1, 0, 0, // cos(90)=0, sin(90)=1
					-1, 0, 0, 0, // -sin(90)=-1, cos(90)=0
					0, 0, 1, 0,
					0, 0, 0, 1,
				},
			},
			input:    Position{X: 1, Y: 0},
			expected: Position{X: 0, Y: 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.tf.Apply(tt.input)
			if math.Abs(result.X-tt.expected.X) > 1e-9 || math.Abs(result.Y-tt.expected.Y) > 1e-9 {
				t.Errorf("Apply(%v) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestTransformMatrix_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		tf       *TransformMatrix
		expected bool
	}{
		{
			name:     "identity is valid",
			tf:       NewIdentityTransform(),
			expected: true,
		},
		{
			name: "invalid last row",
			tf: &TransformMatrix{
				M: [16]float64{
					1, 0, 0, 1, // m[3] should be 0
					0, 1, 0, 0,
					0, 0, 1, 0,
					0, 0, 0, 1,
				},
			},
			expected: false,
		},
		{
			name: "valid with rotation and translation",
			tf: &TransformMatrix{
				M: [16]float64{
					0.866, 0.5, 0, 0,
					-0.5, 0.866, 0, 0,
					0, 0, 1, 0,
					2.5, 3.0, 0, 1,
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.tf.IsValid()
			if result != tt.expected {
				t.Errorf("IsValid() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestTransformMatrix_Translation(t *testing.T) {
	tf := &TransformMatrix{
		M: [16]float64{
			1, 0, 0, 0,
			0, 1, 0, 0,
			0, 0, 1, 0,
			5.5, 7.2, 0, 1,
		},
	}

	trans := tf.Translation()
	if trans.X != 5.5 || trans.Y != 7.2 {
		t.Errorf("Translation() = %v, expected {5.5, 7.2}", trans)
	}
}
