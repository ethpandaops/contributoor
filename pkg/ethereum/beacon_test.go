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
