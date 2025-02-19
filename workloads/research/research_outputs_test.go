package research

import (
	"math"
	"testing"

	"github.com/allora-network/allora-simulator/types"
)

func TestExperienceFactor(t *testing.T) {
	// Create test config
	config := &types.ResearchConfig{
		BaseExperienceFactor: 0.5,
		ExperienceGrowth:     -0.03,
	}

	tests := []struct {
		name     string
		age      int
		expected float64
	}{
		{
			name:     "Age 0",
			age:      0,
			expected: 1.0, // 0.5 * (1 + e^0) = 0.5 * 2 = 1.0
		},
		{
			name:     "Age 50",
			age:      50,
			expected: 0.6116, // 0.5 * (1 + e^(-0.03 * 50))
		},
		{
			name:     "Age 100",
			age:      100,
			expected: 0.5249, // 0.5 * (1 + e^(-0.03 * 100))
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := experienceFactor(config, tt.age)
			// Use approx equal due to floating point arithmetic
			if !almostEqual(result, tt.expected, 0.0001) {
				t.Errorf("experienceFactor(%d) = %v, want %v", tt.age, result, tt.expected)
			}
		})
	}
}

// Helper function for floating point comparison
func almostEqual(a, b, tolerance float64) bool {
	return math.Abs(a-b) <= tolerance
}
