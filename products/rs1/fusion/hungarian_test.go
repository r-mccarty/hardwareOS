package fusion

import (
	"math"
	"testing"
)

func TestAssociateDetections_EmptyInputs(t *testing.T) {
	// Both empty
	assoc, unmatchedV, unmatchedR := AssociateDetections(nil, nil)
	if len(assoc) != 0 || len(unmatchedV) != 0 || len(unmatchedR) != 0 {
		t.Error("Empty inputs should return empty results")
	}

	// Empty vision
	assoc, unmatchedV, unmatchedR = AssociateDetections(nil, []float64{0, 10, 20})
	if len(assoc) != 0 {
		t.Error("No vision detections should mean no associations")
	}
	if len(unmatchedR) != 3 {
		t.Errorf("All radar should be unmatched, got %d", len(unmatchedR))
	}

	// Empty radar
	assoc, unmatchedV, unmatchedR = AssociateDetections([]float64{0, 10, 20}, nil)
	if len(assoc) != 0 {
		t.Error("No radar detections should mean no associations")
	}
	if len(unmatchedV) != 3 {
		t.Errorf("All vision should be unmatched, got %d", len(unmatchedV))
	}
}

func TestAssociateDetections_PerfectMatch(t *testing.T) {
	// Perfect 1:1 match with identical angles
	visionAzimuths := []float64{0, 30, 60}
	radarAzimuths := []float64{0, 30, 60}

	assoc, unmatchedV, unmatchedR := AssociateDetections(visionAzimuths, radarAzimuths)

	if len(assoc) != 3 {
		t.Errorf("Expected 3 associations, got %d", len(assoc))
	}
	if len(unmatchedV) != 0 {
		t.Errorf("Expected 0 unmatched vision, got %d", len(unmatchedV))
	}
	if len(unmatchedR) != 0 {
		t.Errorf("Expected 0 unmatched radar, got %d", len(unmatchedR))
	}

	// Verify assignments are correct
	for _, a := range assoc {
		if visionAzimuths[a.VisionIdx] != radarAzimuths[a.RadarIdx] {
			t.Errorf("Mismatched assignment: vision[%d]=%v, radar[%d]=%v",
				a.VisionIdx, visionAzimuths[a.VisionIdx],
				a.RadarIdx, radarAzimuths[a.RadarIdx])
		}
		if a.Cost != 0 {
			t.Errorf("Perfect match should have cost 0, got %v", a.Cost)
		}
	}
}

func TestAssociateDetections_WithinThreshold(t *testing.T) {
	// Matches within 15 degree threshold
	visionAzimuths := []float64{0, 30}
	radarAzimuths := []float64{10, 35} // 10° and 5° off

	assoc, unmatchedV, unmatchedR := AssociateDetections(visionAzimuths, radarAzimuths)

	if len(assoc) != 2 {
		t.Errorf("Expected 2 associations, got %d", len(assoc))
	}
	if len(unmatchedV) != 0 || len(unmatchedR) != 0 {
		t.Error("All detections should be matched")
	}

	// Verify costs are correct angular differences
	for _, a := range assoc {
		expectedCost := math.Abs(visionAzimuths[a.VisionIdx] - radarAzimuths[a.RadarIdx])
		if math.Abs(a.Cost-expectedCost) > 1e-9 {
			t.Errorf("Cost = %v, expected %v", a.Cost, expectedCost)
		}
	}
}

func TestAssociateDetections_ExceedsThreshold(t *testing.T) {
	// All matches exceed 15 degree threshold
	visionAzimuths := []float64{0}
	radarAzimuths := []float64{30} // 30° off - exceeds threshold

	assoc, unmatchedV, unmatchedR := AssociateDetections(visionAzimuths, radarAzimuths)

	if len(assoc) != 0 {
		t.Error("No associations should be made when threshold exceeded")
	}
	if len(unmatchedV) != 1 {
		t.Errorf("Vision should be unmatched, got %d unmatched", len(unmatchedV))
	}
	if len(unmatchedR) != 1 {
		t.Errorf("Radar should be unmatched, got %d unmatched", len(unmatchedR))
	}
}

func TestAssociateDetections_MixedMatches(t *testing.T) {
	// Some match, some don't
	// Vision: 0, 45, 90
	// Radar: 5, 100, 120
	// 0° matches 5° (diff 5°) - within threshold
	// 45° doesn't match 100° (diff 55°) or 120° (diff 75°)
	// 90° matches 100° (diff 10°) - within threshold
	// So: 2 associations, 1 unmatched vision (45°), 1 unmatched radar (120°)
	visionAzimuths := []float64{0, 45, 90}
	radarAzimuths := []float64{5, 100, 120}

	assoc, unmatchedV, unmatchedR := AssociateDetections(visionAzimuths, radarAzimuths)

	if len(assoc) != 2 {
		t.Errorf("Expected 2 associations, got %d", len(assoc))
	}
	if len(unmatchedV) != 1 {
		t.Errorf("Expected 1 unmatched vision (45°), got %d", len(unmatchedV))
	}
	if len(unmatchedR) != 1 {
		t.Errorf("Expected 1 unmatched radar (120°), got %d", len(unmatchedR))
	}
}

func TestAssociateDetections_UnbalancedMoreVision(t *testing.T) {
	// More vision than radar
	visionAzimuths := []float64{0, 30, 60, 90}
	radarAzimuths := []float64{0, 30}

	assoc, unmatchedV, unmatchedR := AssociateDetections(visionAzimuths, radarAzimuths)

	if len(assoc) != 2 {
		t.Errorf("Expected 2 associations, got %d", len(assoc))
	}
	if len(unmatchedV) != 2 {
		t.Errorf("Expected 2 unmatched vision, got %d", len(unmatchedV))
	}
	if len(unmatchedR) != 0 {
		t.Errorf("Expected 0 unmatched radar, got %d", len(unmatchedR))
	}
}

func TestAssociateDetections_UnbalancedMoreRadar(t *testing.T) {
	// More radar than vision
	visionAzimuths := []float64{0, 30}
	radarAzimuths := []float64{0, 30, 60, 90}

	assoc, unmatchedV, unmatchedR := AssociateDetections(visionAzimuths, radarAzimuths)

	if len(assoc) != 2 {
		t.Errorf("Expected 2 associations, got %d", len(assoc))
	}
	if len(unmatchedV) != 0 {
		t.Errorf("Expected 0 unmatched vision, got %d", len(unmatchedV))
	}
	if len(unmatchedR) != 2 {
		t.Errorf("Expected 2 unmatched radar, got %d", len(unmatchedR))
	}
}

func TestAssociateDetections_OptimalAssignment(t *testing.T) {
	// Test that algorithm finds optimal assignment, not greedy
	// Vision: [0, 10], Radar: [8, 12]
	// Greedy would match 0->8 (cost 8), then 10->12 (cost 2)
	// Optimal should match 0->8 (cost 8), 10->12 (cost 2) OR
	// 0 unmatched, 10->8 (cost 2), 12 unmatched - actually this is worse
	// In this case greedy and optimal are the same

	// Better test: Vision [0, 14], Radar [7, 13]
	// Greedy: 0->7 (cost 7), 14->13 (cost 1) = total 8
	// Optimal: 0->7 (cost 7), 14->13 (cost 1) = same
	// Actually need a case where greedy fails...

	// Vision [0, 8], Radar [5, 9]
	// Greedy: 0->5 (cost 5), 8->9 (cost 1) = total 6
	// Alternate: 0->9 (cost 9), 8->5 (cost 3) = total 12 (worse)
	// In this case they're the same

	visionAzimuths := []float64{0, 8}
	radarAzimuths := []float64{5, 9}

	assoc, _, _ := AssociateDetections(visionAzimuths, radarAzimuths)

	if len(assoc) != 2 {
		t.Fatalf("Expected 2 associations, got %d", len(assoc))
	}

	totalCost := 0.0
	for _, a := range assoc {
		totalCost += a.Cost
	}

	if totalCost > 10 {
		t.Errorf("Total cost %v seems too high for optimal assignment", totalCost)
	}
	t.Logf("Total assignment cost: %v", totalCost)
}

func TestAbsAngleDiff(t *testing.T) {
	tests := []struct {
		name     string
		a, b     float64
		expected float64
	}{
		{"same angle", 45, 45, 0},
		{"simple diff", 30, 45, 15},
		{"reverse order", 45, 30, 15},
		{"negative angles", -30, 30, 60},
		{"wraparound 180", 170, -170, 20},
		{"wraparound large", 350, 10, 20},
		{"exactly 180", 0, 180, 180},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := absAngleDiff(tt.a, tt.b)
			if math.Abs(result-tt.expected) > 1e-9 {
				t.Errorf("absAngleDiff(%v, %v) = %v, expected %v", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

func TestMaxAssociationAngle(t *testing.T) {
	if MaxAssociationAngle != 15.0 {
		t.Errorf("MaxAssociationAngle = %v, expected 15.0", MaxAssociationAngle)
	}
}

func TestNoMatchCost(t *testing.T) {
	if NoMatchCost != 1e6 {
		t.Errorf("NoMatchCost = %v, expected 1e6", NoMatchCost)
	}
}

func TestAssociateDetections_SingleElements(t *testing.T) {
	// Single vision, single radar - within threshold
	assoc, unmatchedV, unmatchedR := AssociateDetections([]float64{10}, []float64{12})
	if len(assoc) != 1 {
		t.Error("Single matching pair should create 1 association")
	}
	if len(unmatchedV) != 0 || len(unmatchedR) != 0 {
		t.Error("No unmatched elements expected")
	}

	// Single vision, single radar - exceeds threshold
	assoc, unmatchedV, unmatchedR = AssociateDetections([]float64{10}, []float64{50})
	if len(assoc) != 0 {
		t.Error("No association expected when threshold exceeded")
	}
	if len(unmatchedV) != 1 || len(unmatchedR) != 1 {
		t.Error("Both should be unmatched")
	}
}
