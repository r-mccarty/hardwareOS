// Package fusion implements sensor fusion for combining camera and radar data.
package fusion

import (
	"math"
)

// Association represents a matched pair of detections
type Association struct {
	VisionIdx int     // Index in vision detections array
	RadarIdx  int     // Index in radar detections array
	Cost      float64 // Association cost (angular difference in degrees)
}

// MaxAssociationAngle is the maximum angular difference (degrees) for valid association
const MaxAssociationAngle = 15.0

// NoMatchCost is the cost assigned to prevent invalid matches
const NoMatchCost = 1e6

// AssociateDetections performs optimal assignment between vision and radar detections
// using angular difference as the cost metric.
//
// Parameters:
//   - visionAzimuths: azimuth angles (degrees) of vision detections
//   - radarAzimuths: azimuth angles (degrees) of radar detections
//
// Returns:
//   - associations: matched pairs with their costs
//   - unmatchedVision: indices of vision detections with no radar match
//   - unmatchedRadar: indices of radar detections with no vision match
func AssociateDetections(visionAzimuths, radarAzimuths []float64) (
	associations []Association,
	unmatchedVision []int,
	unmatchedRadar []int,
) {
	nVision := len(visionAzimuths)
	nRadar := len(radarAzimuths)

	// Handle empty cases
	if nVision == 0 {
		for i := 0; i < nRadar; i++ {
			unmatchedRadar = append(unmatchedRadar, i)
		}
		return
	}
	if nRadar == 0 {
		for i := 0; i < nVision; i++ {
			unmatchedVision = append(unmatchedVision, i)
		}
		return
	}

	// Build cost matrix
	// Rows = vision detections, Cols = radar detections
	costMatrix := make([][]float64, nVision)
	for i := range costMatrix {
		costMatrix[i] = make([]float64, nRadar)
		for j := range costMatrix[i] {
			angularDiff := absAngleDiff(visionAzimuths[i], radarAzimuths[j])
			if angularDiff > MaxAssociationAngle {
				costMatrix[i][j] = NoMatchCost
			} else {
				costMatrix[i][j] = angularDiff
			}
		}
	}

	// Solve assignment problem using Hungarian algorithm
	assignment := hungarianSolve(costMatrix)

	// Process results
	matchedVision := make(map[int]bool)
	matchedRadar := make(map[int]bool)

	for visionIdx, radarIdx := range assignment {
		if radarIdx >= 0 && radarIdx < nRadar {
			cost := costMatrix[visionIdx][radarIdx]
			if cost < NoMatchCost {
				associations = append(associations, Association{
					VisionIdx: visionIdx,
					RadarIdx:  radarIdx,
					Cost:      cost,
				})
				matchedVision[visionIdx] = true
				matchedRadar[radarIdx] = true
			}
		}
	}

	// Collect unmatched
	for i := 0; i < nVision; i++ {
		if !matchedVision[i] {
			unmatchedVision = append(unmatchedVision, i)
		}
	}
	for i := 0; i < nRadar; i++ {
		if !matchedRadar[i] {
			unmatchedRadar = append(unmatchedRadar, i)
		}
	}

	return
}

// absAngleDiff computes the absolute angular difference in degrees,
// handling wraparound at 360 degrees.
func absAngleDiff(a, b float64) float64 {
	diff := math.Abs(a - b)
	if diff > 180 {
		diff = 360 - diff
	}
	return diff
}

// hungarianSolve implements the Hungarian algorithm for the assignment problem.
// Returns an array where result[i] = j means row i is assigned to column j.
// Returns -1 for unassigned rows.
func hungarianSolve(costMatrix [][]float64) []int {
	nRows := len(costMatrix)
	if nRows == 0 {
		return nil
	}
	nCols := len(costMatrix[0])
	if nCols == 0 {
		return make([]int, nRows)
	}

	// Pad to square matrix if needed
	n := nRows
	if nCols > n {
		n = nCols
	}

	// Create padded square matrix
	matrix := make([][]float64, n)
	for i := range matrix {
		matrix[i] = make([]float64, n)
		for j := range matrix[i] {
			if i < nRows && j < nCols {
				matrix[i][j] = costMatrix[i][j]
			} else {
				matrix[i][j] = 0 // Dummy assignments have zero cost
			}
		}
	}

	// Step 1: Subtract row minimums
	for i := 0; i < n; i++ {
		minVal := matrix[i][0]
		for j := 1; j < n; j++ {
			if matrix[i][j] < minVal {
				minVal = matrix[i][j]
			}
		}
		for j := 0; j < n; j++ {
			matrix[i][j] -= minVal
		}
	}

	// Step 2: Subtract column minimums
	for j := 0; j < n; j++ {
		minVal := matrix[0][j]
		for i := 1; i < n; i++ {
			if matrix[i][j] < minVal {
				minVal = matrix[i][j]
			}
		}
		for i := 0; i < n; i++ {
			matrix[i][j] -= minVal
		}
	}

	// Assignment arrays
	rowAssignment := make([]int, n)
	colAssignment := make([]int, n)
	for i := range rowAssignment {
		rowAssignment[i] = -1
	}
	for i := range colAssignment {
		colAssignment[i] = -1
	}

	// Try to find initial assignments using zeros
	for i := 0; i < n; i++ {
		for j := 0; j < n; j++ {
			if matrix[i][j] == 0 && rowAssignment[i] == -1 && colAssignment[j] == -1 {
				rowAssignment[i] = j
				colAssignment[j] = i
			}
		}
	}

	// Main loop: augment until all rows are assigned
	for {
		// Count assignments
		numAssigned := 0
		for i := 0; i < n; i++ {
			if rowAssignment[i] != -1 {
				numAssigned++
			}
		}
		if numAssigned == n {
			break
		}

		// Find augmenting path using BFS
		rowCovered := make([]bool, n)
		colCovered := make([]bool, n)
		parent := make([]int, n)
		for i := range parent {
			parent[i] = -1
		}

		// Mark covered columns
		for i := 0; i < n; i++ {
			if rowAssignment[i] != -1 {
				colCovered[rowAssignment[i]] = true
			}
		}

		// Find uncovered zero and try to augment
		foundAugment := false
		for !foundAugment {
			// Find minimum uncovered element
			minVal := math.MaxFloat64
			for i := 0; i < n; i++ {
				if rowCovered[i] {
					continue
				}
				for j := 0; j < n; j++ {
					if colCovered[j] {
						continue
					}
					if matrix[i][j] < minVal {
						minVal = matrix[i][j]
					}
				}
			}

			// Subtract from uncovered rows, add to covered columns
			for i := 0; i < n; i++ {
				for j := 0; j < n; j++ {
					if !rowCovered[i] && !colCovered[j] {
						matrix[i][j] -= minVal
					} else if rowCovered[i] && colCovered[j] {
						matrix[i][j] += minVal
					}
				}
			}

			// Try to find augmenting path
			for startRow := 0; startRow < n; startRow++ {
				if rowAssignment[startRow] != -1 {
					continue
				}

				// BFS from unassigned row
				queue := []int{startRow}
				visited := make([]bool, n)
				visited[startRow] = true
				for i := range parent {
					parent[i] = -1
				}

				for len(queue) > 0 && !foundAugment {
					row := queue[0]
					queue = queue[1:]

					for col := 0; col < n; col++ {
						if matrix[row][col] != 0 {
							continue
						}

						if colAssignment[col] == -1 {
							// Found augmenting path - trace back and flip
							curCol := col
							curRow := row
							for curRow != -1 {
								prevCol := rowAssignment[curRow]
								rowAssignment[curRow] = curCol
								colAssignment[curCol] = curRow
								curCol = prevCol
								if curCol != -1 {
									curRow = parent[curRow]
								} else {
									curRow = -1
								}
							}
							foundAugment = true
							break
						}

						// Column is assigned, continue through assignment
						nextRow := colAssignment[col]
						if !visited[nextRow] {
							visited[nextRow] = true
							parent[nextRow] = row
							queue = append(queue, nextRow)
						}
					}
				}

				if foundAugment {
					break
				}
			}
		}
	}

	// Return only the original row assignments
	result := make([]int, nRows)
	for i := 0; i < nRows; i++ {
		if rowAssignment[i] < nCols {
			result[i] = rowAssignment[i]
		} else {
			result[i] = -1 // Assigned to dummy column
		}
	}

	return result
}
