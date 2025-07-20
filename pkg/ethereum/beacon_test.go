package ethereum

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsSlotDifferenceTooLarge(t *testing.T) {
	tests := []struct {
		name           string
		slotA          uint64
		slotB          uint64
		expectTooLarge bool
	}{
		{
			name:           "identical slots",
			slotA:          100,
			slotB:          100,
			expectTooLarge: false,
		},
		{
			name:           "small difference (positive direction)",
			slotA:          100,
			slotB:          1000, // 900 slot difference
			expectTooLarge: false,
		},
		{
			name:           "small difference (negative direction)",
			slotA:          1000,
			slotB:          100, // 900 slot difference
			expectTooLarge: false,
		},
		{
			name:           "at threshold",
			slotA:          100,
			slotB:          10100, // exactly 10000 slot difference
			expectTooLarge: false,
		},
		{
			name:           "beyond threshold (positive direction)",
			slotA:          100,
			slotB:          12000, // 11900 slot difference, > MaxReasonableSlotDifference (10000)
			expectTooLarge: true,
		},
		{
			name:           "beyond threshold (negative direction)",
			slotA:          12000,
			slotB:          100, // 11900 slot difference, > MaxReasonableSlotDifference (10000)
			expectTooLarge: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isSlotDifferenceTooLarge(tt.slotA, tt.slotB)
			assert.Equal(t, tt.expectTooLarge, result)
		})
	}
}

func TestBeaconWrapper_IsActiveSubnet(t *testing.T) {
	tests := []struct {
		name          string
		activeSubnets []int
		subnetID      uint64
		expected      bool
	}{
		{
			name:          "empty active subnets denies all",
			activeSubnets: []int{},
			subnetID:      5,
			expected:      false,
		},
		{
			name:          "subnet in active list",
			activeSubnets: []int{2, 5, 10},
			subnetID:      5,
			expected:      true,
		},
		{
			name:          "subnet not in active list",
			activeSubnets: []int{2, 5, 10},
			subnetID:      7,
			expected:      false,
		},
		{
			name:          "zero subnet in active list",
			activeSubnets: []int{0, 5, 10},
			subnetID:      0,
			expected:      true,
		},
		{
			name:          "max subnet ID",
			activeSubnets: []int{63},
			subnetID:      63,
			expected:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &BeaconWrapper{
				activeSubnets: tt.activeSubnets,
			}
			assert.Equal(t, tt.expected, w.IsActiveSubnet(tt.subnetID))
		})
	}
}

func TestCalculateSubnetID(t *testing.T) {
	tests := []struct {
		committeeIndex uint64
		expectedSubnet uint64
	}{
		{0, 0},
		{1, 1},
		{63, 63},
		{64, 0},
		{65, 1},
		{127, 63},
		{128, 0},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			subnetID := tt.committeeIndex % 64
			assert.Equal(t, tt.expectedSubnet, subnetID)
		})
	}
}
