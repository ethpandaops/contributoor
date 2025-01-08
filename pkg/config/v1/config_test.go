package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewConfigFromPath(t *testing.T) {
	tests := []struct {
		name        string
		config      string
		expectError bool
	}{
		{
			name: "valid config",
			config: `version: 0.0.2
networkName: NETWORK_NAME_MAINNET
beaconNodeAddress: http://localhost:5052
contributoorDirectory: /tmp/contributoor
runMethod: RUN_METHOD_DOCKER
`,
			expectError: false,
		},
		{
			name: "invalid network name",
			config: `version: 0.0.2
networkName: INVALID_NETWORK
beaconNodeAddress: http://localhost:5052
contributoorDirectory: /tmp/contributoor
runMethod: RUN_METHOD_DOCKER
`,
			expectError: true,
		},
		{
			name: "invalid network name",
			config: `version: 0.0.2
networkName: INVALID_NETWORK
beaconNodeAddress: http://localhost:5052
contributoorDirectory: /tmp/contributoor
runMethod: RUN_METHOD_DOCKER
`,
			expectError: true,
		},
		{
			name: "missing required field",
			config: `version: 0.0.2
networkName: NETWORK_NAME_MAINNET
`,
			expectError: true,
		},
		{
			name:        "invalid yaml",
			config:      `{[invalid yaml`,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a temporary file
			tmpFile := filepath.Join(t.TempDir(), "config.yaml")
			err := os.WriteFile(tmpFile, []byte(tt.config), 0o600)
			require.NoError(t, err)

			// Test the config loading
			cfg, err := NewConfigFromPath(tmpFile)
			if tt.expectError {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			require.NotNil(t, cfg)

			// Assert we have valid config.
			if !tt.expectError {
				require.Equal(t, "0.0.2", cfg.Version)
				require.Equal(t, NetworkName_NETWORK_NAME_MAINNET, cfg.NetworkName)
				require.Equal(t, "http://localhost:5052", cfg.BeaconNodeAddress)
				require.Equal(t, "/tmp/contributoor", cfg.ContributoorDirectory)
			}
		})
	}
}

func TestNewConfigFromPath_NonExistentFile(t *testing.T) {
	_, err := NewConfigFromPath("non_existent_file.yaml")
	require.Error(t, err)
}
