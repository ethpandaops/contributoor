package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
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
beaconNodeAddress: http://localhost:5052
contributoorDirectory: /tmp/contributoor
runMethod: RUN_METHOD_DOCKER
`,
			expectError: false,
		},
		{
			name: "missing required field",
			config: `version: 0.0.2
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
				require.Equal(t, "", cfg.NetworkName)
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

func TestParseAddress(t *testing.T) {
	tests := []struct {
		name         string
		address      string
		defaultHost  string
		defaultPort  string
		expectedHost string
		expectedPort string
	}{
		{
			name:         "empty address returns defaults",
			address:      "",
			defaultHost:  "127.0.0.1",
			defaultPort:  "9090",
			expectedHost: "127.0.0.1",
			expectedPort: "9090",
		},
		{
			name:         "port only returns default host",
			address:      ":8080",
			defaultHost:  "127.0.0.1",
			defaultPort:  "9090",
			expectedHost: "127.0.0.1",
			expectedPort: "8080",
		},
		{
			name:         "full address",
			address:      "localhost:8080",
			defaultHost:  "127.0.0.1",
			defaultPort:  "9090",
			expectedHost: "localhost",
			expectedPort: "8080",
		},
		{
			name:         "http url",
			address:      "http://localhost:8080",
			defaultHost:  "127.0.0.1",
			defaultPort:  "9090",
			expectedHost: "localhost",
			expectedPort: "8080",
		},
		{
			name:         "https url",
			address:      "https://example.com:8080",
			defaultHost:  "127.0.0.1",
			defaultPort:  "9090",
			expectedHost: "example.com",
			expectedPort: "8080",
		},
		{
			name:         "invalid address returns defaults",
			address:      "not:a:valid:address",
			defaultHost:  "127.0.0.1",
			defaultPort:  "9090",
			expectedHost: "127.0.0.1",
			expectedPort: "9090",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, port := ParseAddress(tt.address, tt.defaultHost, tt.defaultPort)
			assert.Equal(t, tt.expectedHost, host)
			assert.Equal(t, tt.expectedPort, port)
		})
	}
}

func TestConfig_GetMetricsHostPort(t *testing.T) {
	tests := []struct {
		name         string
		config       *Config
		expectedHost string
		expectedPort string
	}{
		{
			name:         "empty address returns empty strings",
			config:       &Config{MetricsAddress: ""},
			expectedHost: "",
			expectedPort: "",
		},
		{
			name:         "port only",
			config:       &Config{MetricsAddress: ":8080"},
			expectedHost: defaultMetricsHost,
			expectedPort: "8080",
		},
		{
			name:         "full address",
			config:       &Config{MetricsAddress: "localhost:8080"},
			expectedHost: "localhost",
			expectedPort: "8080",
		},
		{
			name:         "http url",
			config:       &Config{MetricsAddress: "http://localhost:8080"},
			expectedHost: "localhost",
			expectedPort: "8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, port := tt.config.GetMetricsHostPort()
			assert.Equal(t, tt.expectedHost, host)
			assert.Equal(t, tt.expectedPort, port)
		})
	}
}

func TestConfig_GetPprofHostPort(t *testing.T) {
	tests := []struct {
		name         string
		config       *Config
		expectedHost string
		expectedPort string
	}{
		{
			name:         "empty address returns empty strings",
			config:       &Config{PprofAddress: ""},
			expectedHost: "",
			expectedPort: "",
		},
		{
			name:         "port only",
			config:       &Config{PprofAddress: ":8080"},
			expectedHost: defaultPprofHost,
			expectedPort: "8080",
		},
		{
			name:         "full address",
			config:       &Config{PprofAddress: "localhost:8080"},
			expectedHost: "localhost",
			expectedPort: "8080",
		},
		{
			name:         "http url",
			config:       &Config{PprofAddress: "http://localhost:8080"},
			expectedHost: "localhost",
			expectedPort: "8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, port := tt.config.GetPprofHostPort()
			assert.Equal(t, tt.expectedHost, host)
			assert.Equal(t, tt.expectedPort, port)
		})
	}
}
