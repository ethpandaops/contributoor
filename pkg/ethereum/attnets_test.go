package ethereum

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAttnetsBitmask(t *testing.T) {
	tests := []struct {
		name          string
		attnets       string
		expectedCount int
		expectError   bool
	}{
		{
			name:          "empty string",
			attnets:       "",
			expectedCount: 0,
			expectError:   false,
		},
		{
			name:          "all zeros (no active subnets)",
			attnets:       "0x0000000000000000",
			expectedCount: 0,
			expectError:   false,
		},
		{
			name:          "all zeros without 0x prefix",
			attnets:       "0000000000000000",
			expectedCount: 0,
			expectError:   false,
		},
		{
			name:          "single subnet active (bit 0)",
			attnets:       "0x0100000000000000",
			expectedCount: 1,
			expectError:   false,
		},
		{
			name:          "single subnet active (bit 7)",
			attnets:       "0x8000000000000000",
			expectedCount: 1,
			expectError:   false,
		},
		{
			name:          "two subnets active",
			attnets:       "0x0300000000000000",
			expectedCount: 2,
			expectError:   false,
		},
		{
			name:          "multiple subnets active",
			attnets:       "0xFF00000000000000",
			expectedCount: 8,
			expectError:   false,
		},
		{
			name:          "all subnets active (64 bits)",
			attnets:       "0xFFFFFFFFFFFFFFFF",
			expectedCount: 64,
			expectError:   false,
		},
		{
			name:          "malformed hex string",
			attnets:       "0xGGGG",
			expectedCount: -1,
			expectError:   true,
		},
		{
			name:          "odd length hex string",
			attnets:       "0x123",
			expectedCount: -1,
			expectError:   true,
		},
		{
			name:          "typical real-world example (3 subnets)",
			attnets:       "0x0007000000000000",
			expectedCount: 3,
			expectError:   false,
		},
		{
			name:          "subnet pattern spread across bytes",
			attnets:       "0x0101010101010101",
			expectedCount: 8,
			expectError:   false,
		},
		{
			name:          "longer than 8 bytes (should still work)",
			attnets:       "0xFFFFFFFFFFFFFFFFFF",
			expectedCount: 64,
			expectError:   false,
		},
		{
			name:          "shorter than 8 bytes",
			attnets:       "0xFF",
			expectedCount: 8,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count, err := ParseAttnetsBitmask(tt.attnets)

			if tt.expectError {
				require.Error(t, err)
				assert.Equal(t, -1, count)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedCount, count, "Expected %d active subnets, got %d", tt.expectedCount, count)
			}
		})
	}
}

func TestParseAttnetsBitmask_EdgeCases(t *testing.T) {
	t.Run("verify bit positions", func(t *testing.T) {
		// Test that we correctly identify which bit is set
		testCases := []struct {
			attnets  string
			expected int
		}{
			{"0x01", 1}, // bit 0
			{"0x02", 1}, // bit 1
			{"0x04", 1}, // bit 2
			{"0x08", 1}, // bit 3
			{"0x10", 1}, // bit 4
			{"0x20", 1}, // bit 5
			{"0x40", 1}, // bit 6
			{"0x80", 1}, // bit 7
		}

		for _, tc := range testCases {
			count, err := ParseAttnetsBitmask(tc.attnets)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, count, "For attnets %s", tc.attnets)
		}
	})

	t.Run("verify max subnets cap", func(t *testing.T) {
		// Even with more than 64 bits set, should cap at 64
		count, err := ParseAttnetsBitmask("0xFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF")
		require.NoError(t, err)
		assert.Equal(t, 64, count)
	})
}
