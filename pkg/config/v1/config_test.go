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

func TestConfig_GetHealthCheckHostPort(t *testing.T) {
	tests := []struct {
		name         string
		config       *Config
		expectedHost string
		expectedPort string
	}{
		{
			name:         "empty address returns empty strings",
			config:       &Config{HealthCheckAddress: ""},
			expectedHost: "",
			expectedPort: "",
		},
		{
			name:         "port only",
			config:       &Config{HealthCheckAddress: ":8080"},
			expectedHost: defaultHealthCheckHost,
			expectedPort: "8080",
		},
		{
			name:         "full address",
			config:       &Config{HealthCheckAddress: "localhost:8080"},
			expectedHost: "localhost",
			expectedPort: "8080",
		},
		{
			name:         "http url",
			config:       &Config{HealthCheckAddress: "http://localhost:8080"},
			expectedHost: "localhost",
			expectedPort: "8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, port := tt.config.GetHealthCheckHostPort()
			assert.Equal(t, tt.expectedHost, host)
			assert.Equal(t, tt.expectedPort, port)
		})
	}
}

func TestConfig_IsRunMethodSystemd(t *testing.T) {
	tests := []struct {
		name     string
		config   *Config
		envVars  map[string]string
		expected bool
	}{
		{
			name:     "explicit systemd run method",
			config:   &Config{RunMethod: RunMethod_RUN_METHOD_SYSTEMD},
			envVars:  map[string]string{},
			expected: true,
		},
		{
			name:     "docker run method with no env vars",
			config:   &Config{RunMethod: RunMethod_RUN_METHOD_DOCKER},
			envVars:  map[string]string{},
			expected: false,
		},
		{
			name:     "binary run method with no env vars",
			config:   &Config{RunMethod: RunMethod_RUN_METHOD_BINARY},
			envVars:  map[string]string{},
			expected: false,
		},
		{
			name:   "docker run method with INVOCATION_ID",
			config: &Config{RunMethod: RunMethod_RUN_METHOD_DOCKER},
			envVars: map[string]string{
				"INVOCATION_ID": "12345",
			},
			expected: true,
		},
		{
			name:   "docker run method with JOURNAL_STREAM",
			config: &Config{RunMethod: RunMethod_RUN_METHOD_DOCKER},
			envVars: map[string]string{
				"JOURNAL_STREAM": "8:23456",
			},
			expected: true,
		},
		{
			name:   "docker run method with NOTIFY_SOCKET",
			config: &Config{RunMethod: RunMethod_RUN_METHOD_DOCKER},
			envVars: map[string]string{
				"NOTIFY_SOCKET": "/run/systemd/notify",
			},
			expected: true,
		},
		{
			name:   "docker run method with multiple systemd vars",
			config: &Config{RunMethod: RunMethod_RUN_METHOD_DOCKER},
			envVars: map[string]string{
				"INVOCATION_ID":  "12345",
				"JOURNAL_STREAM": "8:23456",
				"NOTIFY_SOCKET":  "/run/systemd/notify",
			},
			expected: true,
		},
		{
			name:   "systemd run method with systemd vars",
			config: &Config{RunMethod: RunMethod_RUN_METHOD_SYSTEMD},
			envVars: map[string]string{
				"INVOCATION_ID": "12345",
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear all potential systemd env vars first
			os.Unsetenv("INVOCATION_ID")
			os.Unsetenv("JOURNAL_STREAM")
			os.Unsetenv("NOTIFY_SOCKET")

			// Set test env vars
			for k, v := range tt.envVars {
				os.Setenv(k, v)

				defer os.Unsetenv(k)
			}

			result := tt.config.IsRunMethodSystemd()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestConfig_SetNetwork(t *testing.T) {
	tests := []struct {
		name          string
		network       string
		expectError   bool
		expectedValue string
	}{
		{
			name:          "valid network",
			network:       "mainnet",
			expectError:   false,
			expectedValue: "mainnet",
		},
		{
			name:        "empty network",
			network:     "",
			expectError: true,
		},
		{
			name:          "network with special chars",
			network:       "test-network-1",
			expectError:   false,
			expectedValue: "test-network-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Config{}
			err := c.SetNetwork(tt.network)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedValue, c.NetworkName)
			}
		})
	}
}

func TestConfig_SetBeaconNodeAddress(t *testing.T) {
	tests := []struct {
		name          string
		address       string
		expectedValue string
	}{
		{
			name:          "valid address",
			address:       "http://localhost:5052",
			expectedValue: "http://localhost:5052",
		},
		{
			name:          "empty address does nothing",
			address:       "",
			expectedValue: "",
		},
		{
			name:          "https address",
			address:       "https://beacon.example.com:5052",
			expectedValue: "https://beacon.example.com:5052",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Config{}
			c.SetBeaconNodeAddress(tt.address)
			assert.Equal(t, tt.expectedValue, c.BeaconNodeAddress)
		})
	}
}

func TestConfig_SetMetricsAddress(t *testing.T) {
	tests := []struct {
		name          string
		address       string
		expectedValue string
	}{
		{
			name:          "valid address",
			address:       ":9090",
			expectedValue: ":9090",
		},
		{
			name:          "empty address does nothing",
			address:       "",
			expectedValue: "",
		},
		{
			name:          "full address",
			address:       "localhost:9090",
			expectedValue: "localhost:9090",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Config{}
			c.SetMetricsAddress(tt.address)
			assert.Equal(t, tt.expectedValue, c.MetricsAddress)
		})
	}
}

func TestConfig_SetHealthCheckAddress(t *testing.T) {
	tests := []struct {
		name          string
		address       string
		expectedValue string
	}{
		{
			name:          "valid address",
			address:       ":9191",
			expectedValue: ":9191",
		},
		{
			name:          "empty address does nothing",
			address:       "",
			expectedValue: "",
		},
		{
			name:          "full address",
			address:       "localhost:9191",
			expectedValue: "localhost:9191",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Config{}
			c.SetHealthCheckAddress(tt.address)
			assert.Equal(t, tt.expectedValue, c.HealthCheckAddress)
		})
	}
}

func TestConfig_SetLogLevel(t *testing.T) {
	tests := []struct {
		name          string
		level         string
		expectedValue string
	}{
		{
			name:          "debug level",
			level:         "debug",
			expectedValue: "debug",
		},
		{
			name:          "info level",
			level:         "info",
			expectedValue: "info",
		},
		{
			name:          "empty level does nothing",
			level:         "",
			expectedValue: "",
		},
		{
			name:          "warn level",
			level:         "warn",
			expectedValue: "warn",
		},
		{
			name:          "error level",
			level:         "error",
			expectedValue: "error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Config{}
			c.SetLogLevel(tt.level)
			assert.Equal(t, tt.expectedValue, c.LogLevel)
		})
	}
}

func TestConfig_SetOutputServerAddress(t *testing.T) {
	tests := []struct {
		name          string
		address       string
		expectedValue string
	}{
		{
			name:          "valid address",
			address:       "xatu.example.com:443",
			expectedValue: "xatu.example.com:443",
		},
		{
			name:          "empty address does nothing",
			address:       "",
			expectedValue: "",
		},
		{
			name:          "creates OutputServer if nil test addr",
			address:       "server.example.com:8080",
			expectedValue: "server.example.com:8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Config{}

			if tt.name != "creates OutputServer if nil test addr" {
				c.OutputServer = &OutputServer{}
			}

			c.SetOutputServerAddress(tt.address)

			if tt.address == "" {
				if c.OutputServer != nil {
					assert.Equal(t, tt.expectedValue, c.OutputServer.Address)
				}
			} else {
				assert.NotNil(t, c.OutputServer)
				assert.Equal(t, tt.expectedValue, c.OutputServer.Address)
			}
		})
	}
}

func TestConfig_SetOutputServerCredentials(t *testing.T) {
	tests := []struct {
		name          string
		creds         string
		expectedValue string
	}{
		{
			name:          "valid credentials",
			creds:         "username:password",
			expectedValue: "username:password",
		},
		{
			name:          "empty credentials does nothing",
			creds:         "",
			expectedValue: "",
		},
		{
			name:          "creates OutputServer if nil test creds",
			creds:         "api-key-12345",
			expectedValue: "api-key-12345",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Config{}

			if tt.name != "creates OutputServer if nil test creds" {
				c.OutputServer = &OutputServer{}
			}

			c.SetOutputServerCredentials(tt.creds)

			if tt.creds == "" {
				if c.OutputServer != nil {
					assert.Equal(t, tt.expectedValue, c.OutputServer.Credentials)
				}
			} else {
				assert.NotNil(t, c.OutputServer)
				assert.Equal(t, tt.expectedValue, c.OutputServer.Credentials)
			}
		})
	}
}

func TestConfig_SetOutputServerTLS(t *testing.T) {
	tests := []struct {
		name          string
		useTLS        bool
		expectedValue bool
	}{
		{
			name:          "enable TLS",
			useTLS:        true,
			expectedValue: true,
		},
		{
			name:          "disable TLS",
			useTLS:        false,
			expectedValue: false,
		},
		{
			name:          "creates OutputServer if nil test tls",
			useTLS:        true,
			expectedValue: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Config{}

			if tt.name != "creates OutputServer if nil test tls" {
				c.OutputServer = &OutputServer{}
			}

			c.SetOutputServerTLS(tt.useTLS)

			assert.NotNil(t, c.OutputServer)
			assert.Equal(t, tt.expectedValue, c.OutputServer.Tls)
		})
	}
}

func TestConfig_SetContributoorDirectory(t *testing.T) {
	tests := []struct {
		name          string
		dir           string
		expectedValue string
	}{
		{
			name:          "valid directory",
			dir:           "/tmp/contributoor",
			expectedValue: "/tmp/contributoor",
		},
		{
			name:          "home directory",
			dir:           "~/.contributoor",
			expectedValue: "~/.contributoor",
		},
		{
			name:          "empty directory does nothing",
			dir:           "",
			expectedValue: "",
		},
		{
			name:          "relative directory",
			dir:           "./data/contributoor",
			expectedValue: "./data/contributoor",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Config{}
			c.SetContributoorDirectory(tt.dir)
			assert.Equal(t, tt.expectedValue, c.ContributoorDirectory)
		})
	}
}

func TestNetworkName_DisplayName(t *testing.T) {
	tests := []struct {
		name         string
		network      NetworkName
		expectedName string
	}{
		{
			name:         "mainnet",
			network:      NetworkName_NETWORK_NAME_MAINNET,
			expectedName: "Mainnet",
		},
		{
			name:         "sepolia",
			network:      NetworkName_NETWORK_NAME_SEPOLIA,
			expectedName: "Sepolia",
		},
		{
			name:         "holesky",
			network:      NetworkName_NETWORK_NAME_HOLESKY,
			expectedName: "Holesky",
		},
		{
			name:         "hoodi",
			network:      NetworkName_NETWORK_NAME_HOODI,
			expectedName: "Hoodi",
		},
		{
			name:         "unspecified",
			network:      NetworkName_NETWORK_NAME_UNSPECIFIED,
			expectedName: "Unknown",
		},
		{
			name:         "invalid value",
			network:      NetworkName(999),
			expectedName: "Unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.network.DisplayName()
			assert.Equal(t, tt.expectedName, result)
		})
	}
}

func TestRunMethod_DisplayName(t *testing.T) {
	tests := []struct {
		name         string
		method       RunMethod
		expectedName string
	}{
		{
			name:         "docker",
			method:       RunMethod_RUN_METHOD_DOCKER,
			expectedName: "Docker",
		},
		{
			name:         "systemd",
			method:       RunMethod_RUN_METHOD_SYSTEMD,
			expectedName: "Systemd",
		},
		{
			name:         "binary",
			method:       RunMethod_RUN_METHOD_BINARY,
			expectedName: "Binary",
		},
		{
			name:         "unspecified",
			method:       RunMethod_RUN_METHOD_UNSPECIFIED,
			expectedName: "Unknown",
		},
		{
			name:         "invalid value",
			method:       RunMethod(999),
			expectedName: "Unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.method.DisplayName()
			assert.Equal(t, tt.expectedName, result)
		})
	}
}

func TestNewDefaultConfig(t *testing.T) {
	cfg := NewDefaultConfig()

	// Verify all default values
	assert.NotNil(t, cfg)
	assert.Equal(t, "info", cfg.LogLevel)
	assert.Equal(t, "", cfg.Version)
	assert.Equal(t, "~/.contributoor", cfg.ContributoorDirectory)
	assert.Equal(t, RunMethod_RUN_METHOD_DOCKER, cfg.RunMethod)
	assert.Equal(t, "", cfg.NetworkName)
	assert.Equal(t, "http://localhost:5052", cfg.BeaconNodeAddress)
	assert.Equal(t, "", cfg.MetricsAddress)
	assert.Equal(t, "", cfg.PprofAddress)

	// Verify OutputServer defaults
	assert.NotNil(t, cfg.OutputServer)
	assert.Equal(t, "xatu.primary.production.platform.ethpandaops.io:443", cfg.OutputServer.Address)
	assert.Equal(t, "", cfg.OutputServer.Credentials)
	assert.Equal(t, true, cfg.OutputServer.Tls)
}
