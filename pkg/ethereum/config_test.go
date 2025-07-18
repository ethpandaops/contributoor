package ethereum

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
		errorMsg    string
	}{
		{
			name: "valid config with beacon node address",
			config: &Config{
				BeaconNodeAddress: "http://localhost:5052",
			},
			expectError: false,
		},
		{
			name: "valid config with all fields",
			config: &Config{
				BeaconNodeAddress: "http://localhost:5052",
				BeaconNodeHeaders: map[string]string{
					"Authorization": "Bearer token",
					"X-Custom":      "value",
				},
				NetworkOverride: "mainnet",
			},
			expectError: false,
		},
		{
			name: "invalid config - missing beacon node address",
			config: &Config{
				BeaconNodeAddress: "",
			},
			expectError: true,
			errorMsg:    "beaconNodeAddress is required",
		},
		{
			name: "valid config with https address",
			config: &Config{
				BeaconNodeAddress: "https://beacon.example.com:5052",
			},
			expectError: false,
		},
		{
			name: "valid config with empty headers",
			config: &Config{
				BeaconNodeAddress: "http://localhost:5052",
				BeaconNodeHeaders: map[string]string{},
			},
			expectError: false,
		},
		{
			name: "valid config with nil headers",
			config: &Config{
				BeaconNodeAddress: "http://localhost:5052",
				BeaconNodeHeaders: nil,
			},
			expectError: false,
		},
		{
			name: "valid config with network override",
			config: &Config{
				BeaconNodeAddress: "http://localhost:5052",
				NetworkOverride:   "sepolia",
			},
			expectError: false,
		},
		{
			name: "valid config without network override",
			config: &Config{
				BeaconNodeAddress: "http://localhost:5052",
				NetworkOverride:   "",
			},
			expectError: false,
		},
		{
			name: "valid config with subnet check enabled and valid max subnets",
			config: &Config{
				BeaconNodeAddress: "http://localhost:5052",
				AttestationSubnetConfig: SubnetConfig{
					Enabled:    true,
					MaxSubnets: 2,
				},
			},
			expectError: false,
		},
		{
			name: "valid config with subnet check at boundary (0)",
			config: &Config{
				BeaconNodeAddress: "http://localhost:5052",
				AttestationSubnetConfig: SubnetConfig{
					Enabled:    true,
					MaxSubnets: 0,
				},
			},
			expectError: false,
		},
		{
			name: "valid config with subnet check at boundary (64)",
			config: &Config{
				BeaconNodeAddress: "http://localhost:5052",
				AttestationSubnetConfig: SubnetConfig{
					Enabled:    true,
					MaxSubnets: 64,
				},
			},
			expectError: false,
		},
		{
			name: "invalid config - subnet check with negative max subnets",
			config: &Config{
				BeaconNodeAddress: "http://localhost:5052",
				AttestationSubnetConfig: SubnetConfig{
					Enabled:    true,
					MaxSubnets: -1,
				},
			},
			expectError: true,
			errorMsg:    "subnetCheck.maxSubnets must be between 0 and 64 (inclusive)",
		},
		{
			name: "invalid config - subnet check with max subnets > 64",
			config: &Config{
				BeaconNodeAddress: "http://localhost:5052",
				AttestationSubnetConfig: SubnetConfig{
					Enabled:    true,
					MaxSubnets: 65,
				},
			},
			expectError: true,
			errorMsg:    "subnetCheck.maxSubnets must be between 0 and 64 (inclusive)",
		},
		{
			name: "valid config - subnet check disabled with invalid max subnets (should not validate)",
			config: &Config{
				BeaconNodeAddress: "http://localhost:5052",
				AttestationSubnetConfig: SubnetConfig{
					Enabled:    false,
					MaxSubnets: -1,
				},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.expectError {
				require.Error(t, err)
				assert.Equal(t, tt.errorMsg, err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestConfig_Fields(t *testing.T) {
	t.Run("verify struct fields", func(t *testing.T) {
		config := &Config{
			BeaconNodeAddress: "http://localhost:5052",
			BeaconNodeHeaders: map[string]string{
				"Authorization": "Bearer token123",
				"X-API-Key":     "key123",
			},
			NetworkOverride: "holesky",
		}

		// Verify all fields are set correctly
		assert.Equal(t, "http://localhost:5052", config.BeaconNodeAddress)
		assert.Equal(t, "Bearer token123", config.BeaconNodeHeaders["Authorization"])
		assert.Equal(t, "key123", config.BeaconNodeHeaders["X-API-Key"])
		assert.Equal(t, "holesky", config.NetworkOverride)
	})

	t.Run("modify headers after creation", func(t *testing.T) {
		config := &Config{
			BeaconNodeAddress: "http://localhost:5052",
			BeaconNodeHeaders: map[string]string{
				"Initial": "value",
			},
		}

		// Add new header
		config.BeaconNodeHeaders["New-Header"] = "new-value"
		assert.Equal(t, "new-value", config.BeaconNodeHeaders["New-Header"])

		// Modify existing header
		config.BeaconNodeHeaders["Initial"] = "modified"
		assert.Equal(t, "modified", config.BeaconNodeHeaders["Initial"])

		// Delete header
		delete(config.BeaconNodeHeaders, "Initial")
		_, exists := config.BeaconNodeHeaders["Initial"]
		assert.False(t, exists)
	})
}

func TestConfig_EdgeCases(t *testing.T) {
	t.Run("config with whitespace beacon address", func(t *testing.T) {
		config := &Config{
			BeaconNodeAddress: "   http://localhost:5052   ",
		}
		// Validate doesn't trim whitespace, it just checks for empty string
		assert.NoError(t, config.Validate())
	})

	t.Run("config with only spaces in beacon address", func(t *testing.T) {
		config := &Config{
			BeaconNodeAddress: "   ",
		}
		// Spaces are not considered empty by the current validation
		assert.NoError(t, config.Validate())
	})

	t.Run("config with large headers map", func(t *testing.T) {
		headers := make(map[string]string)
		for i := 0; i < 100; i++ {
			headers[string(rune('A'+i))] = string(rune('a' + i))
		}

		config := &Config{
			BeaconNodeAddress: "http://localhost:5052",
			BeaconNodeHeaders: headers,
		}

		assert.NoError(t, config.Validate())
		assert.Len(t, config.BeaconNodeHeaders, 100)
	})

	t.Run("zero value config", func(t *testing.T) {
		var config Config
		err := config.Validate()
		assert.Error(t, err)
		assert.Equal(t, "beaconNodeAddress is required", err.Error())
	})

	t.Run("nil config pointer", func(t *testing.T) {
		var config *Config
		assert.Nil(t, config)
		// Can't call Validate on nil pointer
	})
}
