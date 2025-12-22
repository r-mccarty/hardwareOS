package radar

import (
	"math"
	"testing"
)

func TestTargetCalculatePolar(t *testing.T) {
	tests := []struct {
		name            string
		x, y            int16
		expectedRange   float64
		expectedAzimuth float64
		tolerance       float64
	}{
		{
			name:            "target directly ahead",
			x:               0,
			y:               2000, // 2 meters forward
			expectedRange:   2.0,
			expectedAzimuth: 0.0,
			tolerance:       0.001,
		},
		{
			name:            "target to the right",
			x:               1000, // 1 meter right
			y:               0,
			expectedRange:   1.0,
			expectedAzimuth: 90.0,
			tolerance:       0.001,
		},
		{
			name:            "target to the left",
			x:               -1000, // 1 meter left
			y:               0,
			expectedRange:   1.0,
			expectedAzimuth: -90.0,
			tolerance:       0.001,
		},
		{
			name:            "target at 45 degrees right",
			x:               1000, // 1 meter right
			y:               1000, // 1 meter forward
			expectedRange:   math.Sqrt(2),
			expectedAzimuth: 45.0,
			tolerance:       0.001,
		},
		{
			name:            "target at 3-4-5 triangle",
			x:               3000, // 3 meters right
			y:               4000, // 4 meters forward
			expectedRange:   5.0,
			expectedAzimuth: 36.87, // atan(3/4) in degrees
			tolerance:       0.01,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := Target{X: tt.x, Y: tt.y}
			target.CalculatePolar()

			if math.Abs(target.Range-tt.expectedRange) > tt.tolerance {
				t.Errorf("Range: got %v, want %v (tolerance %v)",
					target.Range, tt.expectedRange, tt.tolerance)
			}

			if math.Abs(target.Azimuth-tt.expectedAzimuth) > tt.tolerance {
				t.Errorf("Azimuth: got %v, want %v (tolerance %v)",
					target.Azimuth, tt.expectedAzimuth, tt.tolerance)
			}

			// Verify Distance is set in mm
			expectedDistMm := uint16(tt.expectedRange * 1000)
			if target.Distance != expectedDistMm {
				t.Errorf("Distance: got %v mm, want %v mm",
					target.Distance, expectedDistMm)
			}
		})
	}
}

func TestFrameValidTargets(t *testing.T) {
	frame := Frame{
		Targets: [maxTargets]Target{
			{X: 1000, Y: 2000, Valid: true},
			{X: 0, Y: 0, Valid: false},
			{X: -500, Y: 3000, Valid: true},
		},
	}

	valid := frame.ValidTargets()

	if len(valid) != 2 {
		t.Errorf("expected 2 valid targets, got %d", len(valid))
	}

	if valid[0].X != 1000 || valid[0].Y != 2000 {
		t.Errorf("first valid target incorrect: got (%d, %d)", valid[0].X, valid[0].Y)
	}

	if valid[1].X != -500 || valid[1].Y != 3000 {
		t.Errorf("second valid target incorrect: got (%d, %d)", valid[1].X, valid[1].Y)
	}
}

func TestParseFrameData(t *testing.T) {
	// Create a mock LD2450 with nil port (we won't use it)
	r := &LD2450{}

	// Construct a valid frame data payload (24 bytes of target data)
	// Target 1: X=1000 (0x03E8), Y=2000 (0x07D0), Speed=50 (0x0032)
	// Target 2: X=0, Y=0, Speed=0 (invalid)
	// Target 3: X=-500 (0xFE0C), Y=3000 (0x0BB8), Speed=-100 (0xFF9C)
	data := []byte{
		// Target 1
		0xE8, 0x03, // X = 1000 (little-endian)
		0xD0, 0x07, // Y = 2000
		0x32, 0x00, // Speed = 50
		0x00, 0x00, // Reserved
		// Target 2 (invalid - all zeros)
		0x00, 0x00, // X = 0
		0x00, 0x00, // Y = 0
		0x00, 0x00, // Speed = 0
		0x00, 0x00, // Reserved
		// Target 3
		0x0C, 0xFE, // X = -500 (little-endian two's complement)
		0xB8, 0x0B, // Y = 3000
		0x9C, 0xFF, // Speed = -100
		0x00, 0x00, // Reserved
	}

	frame := r.parseFrame(data)

	// Verify Target 1
	if !frame.Targets[0].Valid {
		t.Error("Target 1 should be valid")
	}
	if frame.Targets[0].X != 1000 {
		t.Errorf("Target 1 X: got %d, want 1000", frame.Targets[0].X)
	}
	if frame.Targets[0].Y != 2000 {
		t.Errorf("Target 1 Y: got %d, want 2000", frame.Targets[0].Y)
	}
	if frame.Targets[0].Speed != 50 {
		t.Errorf("Target 1 Speed: got %d, want 50", frame.Targets[0].Speed)
	}

	// Verify polar coordinates are calculated
	if math.Abs(frame.Targets[0].Range-2.236) > 0.01 {
		t.Errorf("Target 1 Range: got %v, want ~2.236", frame.Targets[0].Range)
	}

	// Verify Target 2 is invalid
	if frame.Targets[1].Valid {
		t.Error("Target 2 should be invalid")
	}

	// Verify Target 3
	if !frame.Targets[2].Valid {
		t.Error("Target 3 should be valid")
	}
	if frame.Targets[2].X != -500 {
		t.Errorf("Target 3 X: got %d, want -500", frame.Targets[2].X)
	}
	if frame.Targets[2].Y != 3000 {
		t.Errorf("Target 3 Y: got %d, want 3000", frame.Targets[2].Y)
	}
	if frame.Targets[2].Speed != -100 {
		t.Errorf("Target 3 Speed: got %d, want -100", frame.Targets[2].Speed)
	}
}

func TestParseData_FindsValidFrame(t *testing.T) {
	r := &LD2450{
		frameCh: make(chan Frame, 10),
	}

	// Construct a complete frame with header and tail
	// Header: 0xAA 0xFF 0x03 0x00
	// 24 bytes of target data
	// Tail: 0x55 0xCC
	data := make([]byte, 30)

	// Header
	data[0] = headerByte0
	data[1] = headerByte1
	data[2] = headerByte2
	data[3] = headerByte3

	// Target 1: X=1500, Y=2500
	data[4] = 0xDC  // 1500 & 0xFF
	data[5] = 0x05  // 1500 >> 8
	data[6] = 0xC4  // 2500 & 0xFF
	data[7] = 0x09  // 2500 >> 8
	data[8] = 0x00  // Speed low
	data[9] = 0x00  // Speed high
	data[10] = 0x00 // Reserved
	data[11] = 0x00 // Reserved

	// Targets 2 and 3: zeros (invalid)
	// bytes 12-27 already zero

	// Tail
	data[28] = tailByte0
	data[29] = tailByte1

	r.parseData(data)

	// Check if frame was received
	select {
	case frame := <-r.frameCh:
		if !frame.Targets[0].Valid {
			t.Error("Expected target 0 to be valid")
		}
		if frame.Targets[0].X != 1500 {
			t.Errorf("Target X: got %d, want 1500", frame.Targets[0].X)
		}
		if frame.Targets[0].Y != 2500 {
			t.Errorf("Target Y: got %d, want 2500", frame.Targets[0].Y)
		}
	default:
		t.Error("Expected frame to be parsed and sent to channel")
	}
}

func TestParseData_IgnoresInvalidTail(t *testing.T) {
	r := &LD2450{
		frameCh: make(chan Frame, 10),
	}

	// Frame with valid header but invalid tail
	data := make([]byte, 30)
	data[0] = headerByte0
	data[1] = headerByte1
	data[2] = headerByte2
	data[3] = headerByte3
	// Some target data
	data[4] = 0xE8
	data[5] = 0x03
	// Wrong tail
	data[28] = 0x00
	data[29] = 0x00

	r.parseData(data)

	// Should not receive any frame
	select {
	case <-r.frameCh:
		t.Error("Should not parse frame with invalid tail")
	default:
		// Expected - no frame parsed
	}
}

func BenchmarkParseFrame(b *testing.B) {
	r := &LD2450{}
	data := []byte{
		0xE8, 0x03, 0xD0, 0x07, 0x32, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x0C, 0xFE, 0xB8, 0x0B, 0x9C, 0xFF, 0x00, 0x00,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.parseFrame(data)
	}
}

func BenchmarkCalculatePolar(b *testing.B) {
	target := Target{X: 3000, Y: 4000}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		target.CalculatePolar()
	}
}
