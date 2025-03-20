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
			name:           "within threshold (positive direction)",
			slotA:          100,
			slotB:          120, // 20 slot difference
			expectTooLarge: false,
		},
		{
			name:           "within threshold (negative direction)",
			slotA:          120,
			slotB:          100, // 20 slot difference
			expectTooLarge: false,
		},
		{
			name:           "at threshold",
			slotA:          100,
			slotB:          132, // exactly 32 slot difference
			expectTooLarge: false,
		},
		{
			name:           "beyond threshold (positive direction)",
			slotA:          100,
			slotB:          140, // 40 slot difference, > MaxReasonableSlotDifference (32)
			expectTooLarge: true,
		},
		{
			name:           "beyond threshold (negative direction)",
			slotA:          140,
			slotB:          100, // 40 slot difference, > MaxReasonableSlotDifference (32)
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
