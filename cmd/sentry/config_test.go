package main

import (
	"encoding/base64"
	"fmt"
	"os"
	"testing"

	"github.com/ethpandaops/contributoor/pkg/config/v1"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
)

func TestApplyConfigOverridesFromFlags(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		validate func(*testing.T, *config.Config)
	}{
		{
			name: "network override",
			args: []string{"--network", "sepolia"},
			validate: func(t *testing.T, cfg *config.Config) {
				t.Helper()
				assert.Equal(t, "sepolia", cfg.NetworkName)
			},
		},
		{
			name: "beacon node address override",
			args: []string{"--beacon-node-address", "http://localhost:5052"},
			validate: func(t *testing.T, cfg *config.Config) {
				t.Helper()
				assert.Equal(t, "http://localhost:5052", cfg.BeaconNodeAddress)
			},
		},
		{
			name: "metrics address override",
			args: []string{"--metrics-address", "localhost:9091"},
			validate: func(t *testing.T, cfg *config.Config) {
				t.Helper()
				assert.Equal(t, "localhost:9091", cfg.MetricsAddress)
			},
		},
		{
			name: "health check address override",
			args: []string{"--health-check-address", "localhost:9191"},
			validate: func(t *testing.T, cfg *config.Config) {
				t.Helper()
				assert.Equal(t, "localhost:9191", cfg.HealthCheckAddress)
			},
		},
		{
			name: "log level override",
			args: []string{"--log-level", "debug"},
			validate: func(t *testing.T, cfg *config.Config) {
				t.Helper()
				assert.Equal(t, "debug", cfg.LogLevel)
			},
		},
		{
			name: "output server address override",
			args: []string{"--output-server-address", "localhost:8080"},
			validate: func(t *testing.T, cfg *config.Config) {
				t.Helper()
				require.NotNil(t, cfg.OutputServer)
				assert.Equal(t, "localhost:8080", cfg.OutputServer.Address)
			},
		},
		{
			name: "output server credentials override",
			args: []string{"--username", "user", "--password", "pass"},
			validate: func(t *testing.T, cfg *config.Config) {
				t.Helper()
				require.NotNil(t, cfg.OutputServer)
				expected := base64.StdEncoding.EncodeToString([]byte("user:pass"))
				assert.Equal(t, expected, cfg.OutputServer.Credentials)
			},
		},
		{
			name: "output server tls override",
			args: []string{"--output-server-tls", "true"},
			validate: func(t *testing.T, cfg *config.Config) {
				t.Helper()
				require.NotNil(t, cfg.OutputServer)
				assert.True(t, cfg.OutputServer.Tls)
			},
		},
		{
			name: "contributoor directory override",
			args: []string{"--contributoor-directory", "/tmp/contributoor"},
			validate: func(t *testing.T, cfg *config.Config) {
				t.Helper()
				assert.Equal(t, "/tmp/contributoor", cfg.ContributoorDirectory)
			},
		},
		{
			name: "multiple overrides",
			args: []string{
				"--network", "sepolia",
				"--beacon-node-address", "http://localhost:5052",
				"--metrics-address", "localhost:9091",
				"--log-level", "debug",
			},
			validate: func(t *testing.T, cfg *config.Config) {
				t.Helper()
				assert.Equal(t, "sepolia", cfg.NetworkName)
				assert.Equal(t, "http://localhost:5052", cfg.BeaconNodeAddress)
				assert.Equal(t, "localhost:9091", cfg.MetricsAddress)
				assert.Equal(t, "debug", cfg.LogLevel)
			},
		},
		{
			name: "output server credentials override with special chars",
			args: []string{"--username", "user", "--password", "pass!@#$%^&*()"},
			validate: func(t *testing.T, cfg *config.Config) {
				t.Helper()
				require.NotNil(t, cfg.OutputServer)
				expected := base64.StdEncoding.EncodeToString([]byte("user:pass!@#$%^&*()"))
				assert.Equal(t, expected, cfg.OutputServer.Credentials)

				// Verify it's valid base64 and decodes back correctly
				decoded, err := base64.StdEncoding.DecodeString(cfg.OutputServer.Credentials)
				require.NoError(t, err)
				assert.Equal(t, "user:pass!@#$%^&*()", string(decoded))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := cli.NewApp()
			app.Flags = []cli.Flag{
				&cli.StringFlag{Name: "network"},
				&cli.StringFlag{Name: "beacon-node-address"},
				&cli.StringFlag{Name: "metrics-address"},
				&cli.StringFlag{Name: "health-check-address"},
				&cli.StringFlag{Name: "log-level"},
				&cli.StringFlag{Name: "output-server-address"},
				&cli.StringFlag{Name: "username"},
				&cli.StringFlag{Name: "password"},
				&cli.StringFlag{Name: "output-server-tls"},
				&cli.StringFlag{Name: "contributoor-directory"},
			}

			// Create a base config
			cfg := &config.Config{}

			app.Action = func(c *cli.Context) error {
				return applyConfigOverridesFromFlags(cfg, c)
			}

			err := app.Run(append([]string{"contributoor"}, tt.args...))
			require.NoError(t, err)

			tt.validate(t, cfg)
		})
	}
}

func TestConfigOverridePrecedence(t *testing.T) {
	tests := []struct {
		name          string
		configValue   string
		envValue      string
		cliValue      string
		expectedValue string
		envVar        string
		cliFlag       string
		setter        func(*config.Config, string)
		getter        func(*config.Config) string
	}{
		{
			name:          "CLI overrides env and config - network",
			configValue:   "mainnet",
			envValue:      "sepolia",
			cliValue:      "holesky",
			expectedValue: "holesky",
			envVar:        "CONTRIBUTOOR_NETWORK",
			cliFlag:       "network",
			setter: func(c *config.Config, v string) {
				if err := c.SetNetwork(v); err != nil {
					t.Fatalf("failed to set network: %v", err)
				}
			},
			getter: func(c *config.Config) string { return c.NetworkName },
		},
		{
			name:          "Env overrides config but not CLI - beacon node",
			configValue:   "http://localhost:5052",
			envValue:      "http://beacon:5052",
			cliValue:      "",
			expectedValue: "http://beacon:5052",
			envVar:        "CONTRIBUTOOR_BEACON_NODE_ADDRESS",
			cliFlag:       "beacon-node-address",
			setter:        func(c *config.Config, v string) { c.SetBeaconNodeAddress(v) },
			getter:        func(c *config.Config) string { return c.BeaconNodeAddress },
		},
		{
			name:          "Config value preserved when no overrides",
			configValue:   ":9090",
			envValue:      "",
			cliValue:      "",
			expectedValue: ":9090",
			envVar:        "CONTRIBUTOOR_METRICS_ADDRESS",
			cliFlag:       "metrics-address",
			setter:        func(c *config.Config, v string) { c.SetMetricsAddress(v) },
			getter:        func(c *config.Config) string { return c.MetricsAddress },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup initial config
			cfg := config.NewDefaultConfig()
			tt.setter(cfg, tt.configValue)

			// Set env var if provided
			if tt.envValue != "" {
				os.Setenv(tt.envVar, tt.envValue)
				defer os.Unsetenv(tt.envVar)
			}

			// Create CLI app with all flags
			app := cli.NewApp()
			app.Flags = []cli.Flag{
				&cli.StringFlag{Name: "network"},
				&cli.StringFlag{Name: "beacon-node-address"},
				&cli.StringFlag{Name: "metrics-address"},
				&cli.StringFlag{Name: "health-check-address"},
				&cli.StringFlag{Name: "log-level"},
				&cli.StringFlag{Name: "output-server-address"},
				&cli.StringFlag{Name: "username"},
				&cli.StringFlag{Name: "password"},
				&cli.StringFlag{Name: "output-server-tls"},
			}

			// Set up action to apply config
			app.Action = func(c *cli.Context) error {
				return applyConfigOverridesFromFlags(cfg, c)
			}

			// Build args
			args := []string{"app"}
			if tt.cliValue != "" {
				args = append(args, fmt.Sprintf("--%s", tt.cliFlag), tt.cliValue)
			}

			// Run app with args
			err := app.Run(args)
			require.NoError(t, err)

			// Verify final value
			assert.Equal(t, tt.expectedValue, tt.getter(cfg))
		})
	}
}

func TestCredentialsPrecedence(t *testing.T) {
	tests := []struct {
		name          string
		envUser       string
		envPass       string
		cliUser       string
		cliPass       string
		expectedCreds string
	}{
		{
			name:          "CLI credentials override env",
			envUser:       "env_user",
			envPass:       "env_pass",
			cliUser:       "cli_user",
			cliPass:       "cli_pass",
			expectedCreds: "cli_user:cli_pass",
		},
		{
			name:          "Env credentials used when no CLI",
			envUser:       "env_user",
			envPass:       "env_pass",
			cliUser:       "",
			cliPass:       "",
			expectedCreds: "env_user:env_pass",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.NewDefaultConfig()

			// Set env vars if provided
			if tt.envUser != "" {
				os.Setenv("CONTRIBUTOOR_USERNAME", tt.envUser)
				defer os.Unsetenv("CONTRIBUTOOR_USERNAME")
			}
			if tt.envPass != "" {
				os.Setenv("CONTRIBUTOOR_PASSWORD", tt.envPass)
				defer os.Unsetenv("CONTRIBUTOOR_PASSWORD")
			}

			// Create CLI app with all flags
			app := cli.NewApp()
			app.Flags = []cli.Flag{
				&cli.StringFlag{Name: "username"},
				&cli.StringFlag{Name: "password"},
			}

			// Set up action to apply config
			app.Action = func(c *cli.Context) error {
				return applyConfigOverridesFromFlags(cfg, c)
			}

			// Build args
			args := []string{"app"}
			if tt.cliUser != "" {
				args = append(args, "--username", tt.cliUser)
			}
			if tt.cliPass != "" {
				args = append(args, "--password", tt.cliPass)
			}

			// Run app with args
			err := app.Run(args)
			require.NoError(t, err)

			// Decode and verify credentials
			require.NotNil(t, cfg.OutputServer)
			decoded, err := base64.StdEncoding.DecodeString(cfg.OutputServer.Credentials)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedCreds, string(decoded))
		})
	}
}

func TestCreateConfigLogLevel(t *testing.T) {
	// Save the original log level to restore after test
	originalLevel := log.GetLevel()
	defer log.SetLevel(originalLevel)

	tests := []struct {
		name             string
		args             []string
		envLogLevel      string
		expectedLevel    logrus.Level
		expectedCfgLevel string
		expectLogOutput  bool
	}{
		{
			name:             "default log level info",
			args:             []string{},
			expectedLevel:    logrus.InfoLevel,
			expectedCfgLevel: "info",
		},
		{
			name:             "CLI flag sets debug level",
			args:             []string{"--log-level", "debug"},
			expectedLevel:    logrus.DebugLevel,
			expectedCfgLevel: "debug",
		},
		{
			name:             "CLI flag sets warn level",
			args:             []string{"--log-level", "warn"},
			expectedLevel:    logrus.WarnLevel,
			expectedCfgLevel: "warn",
		},
		{
			name:             "CLI flag sets error level",
			args:             []string{"--log-level", "error"},
			expectedLevel:    logrus.ErrorLevel,
			expectedCfgLevel: "error",
		},
		{
			name:             "env var sets debug level",
			envLogLevel:      "debug",
			expectedLevel:    logrus.DebugLevel,
			expectedCfgLevel: "debug",
		},
		{
			name:             "CLI overrides env var",
			args:             []string{"--log-level", "error"},
			envLogLevel:      "debug",
			expectedLevel:    logrus.ErrorLevel,
			expectedCfgLevel: "error",
		},
		{
			name:             "invalid log level defaults to info",
			args:             []string{"--log-level", "invalid"},
			expectedLevel:    logrus.InfoLevel,
			expectedCfgLevel: "invalid",
			expectLogOutput:  true,
		},
		{
			name:             "empty log level keeps default",
			args:             []string{"--log-level", ""},
			expectedLevel:    logrus.InfoLevel,
			expectedCfgLevel: "info",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset log level before each test
			log.SetLevel(logrus.InfoLevel)

			// Set env var if needed
			if tt.envLogLevel != "" {
				os.Setenv("CONTRIBUTOOR_LOG_LEVEL", tt.envLogLevel)
				defer os.Unsetenv("CONTRIBUTOOR_LOG_LEVEL")
			}

			// Create CLI app
			app := cli.NewApp()
			app.Flags = []cli.Flag{
				&cli.StringFlag{Name: "config"},
				&cli.BoolFlag{Name: "debug"},
				&cli.StringFlag{Name: "network"},
				&cli.StringFlag{Name: "beacon-node-address"},
				&cli.StringFlag{Name: "metrics-address"},
				&cli.StringFlag{Name: "health-check-address"},
				&cli.StringFlag{Name: "log-level"},
				&cli.StringFlag{Name: "output-server-address"},
				&cli.StringFlag{Name: "username"},
				&cli.StringFlag{Name: "password"},
				&cli.StringFlag{Name: "output-server-tls"},
				&cli.StringFlag{Name: "contributoor-directory"},
			}

			var createdConfig *config.Config
			app.Action = func(c *cli.Context) error {
				cfg, err := createConfig(c)
				if err != nil {
					return err
				}
				createdConfig = cfg

				// Apply log level as it would be in main
				if cfg.LogLevel != "" {
					level, lerr := logrus.ParseLevel(cfg.LogLevel)
					if lerr != nil {
						log.WithField("level", cfg.LogLevel).WithError(lerr).Warn("Invalid log level, defaulting to info")

						level = logrus.InfoLevel
					}

					log.SetLevel(level)
				}

				return nil
			}

			// Run app with args
			err := app.Run(append([]string{"contributoor"}, tt.args...))
			require.NoError(t, err)
			require.NotNil(t, createdConfig)

			// Verify the log level was set correctly
			assert.Equal(t, tt.expectedLevel, log.GetLevel(), "log level mismatch")
			assert.Equal(t, tt.expectedCfgLevel, createdConfig.LogLevel, "config log level mismatch")
		})
	}
}

func TestApplyConfigOverridesFromFlagsErrors(t *testing.T) {
	tests := []struct {
		name          string
		envVars       map[string]string
		args          []string
		expectError   bool
		errorContains string
	}{
		{
			name: "invalid output server tls from env",
			envVars: map[string]string{
				"CONTRIBUTOOR_OUTPUT_SERVER_TLS": "not-a-bool",
			},
			expectError:   true,
			errorContains: "failed to parse output server tls env var",
		},
		{
			name:          "invalid output server tls from CLI",
			args:          []string{"--output-server-tls", "not-a-bool"},
			expectError:   true,
			errorContains: "failed to parse output server tls flag",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set env vars
			for k, v := range tt.envVars {
				os.Setenv(k, v)
				defer os.Unsetenv(k)
			}

			app := cli.NewApp()
			app.Flags = []cli.Flag{
				&cli.StringFlag{Name: "network"},
				&cli.StringFlag{Name: "output-server-tls"},
			}

			cfg := config.NewDefaultConfig()

			app.Action = func(c *cli.Context) error {
				return applyConfigOverridesFromFlags(cfg, c)
			}

			args := append([]string{"app"}, tt.args...)
			err := app.Run(args)

			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCreateConfigFromPath(t *testing.T) {
	tests := []struct {
		name          string
		configPath    string
		expectError   bool
		errorContains string
	}{
		{
			name:          "invalid config path",
			configPath:    "/non/existent/config.yaml",
			expectError:   true,
			errorContains: "no such file or directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := cli.NewApp()
			app.Flags = []cli.Flag{
				&cli.StringFlag{Name: "config"},
				&cli.BoolFlag{Name: "debug"},
			}

			app.Action = func(c *cli.Context) error {
				_, err := createConfig(c)
				return err
			}

			args := []string{"contributoor"}
			if tt.configPath != "" {
				args = append(args, "--config", tt.configPath)
			}

			err := app.Run(args)
			if tt.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorContains)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestDebugFlag(t *testing.T) {
	app := cli.NewApp()
	app.Flags = []cli.Flag{
		&cli.StringFlag{Name: "config"},
		&cli.BoolFlag{Name: "debug"},
		&cli.StringFlag{Name: "beacon-node-address"},
	}

	var debugMode bool
	app.Action = func(c *cli.Context) error {
		debugMode = c.Bool("debug")

		return nil
	}

	err := app.Run([]string{"contributoor", "--debug", "--beacon-node-address", "http://localhost:5052"})

	require.NoError(t, err)
	assert.True(t, debugMode)
}

func TestContributoorDirectoryOverride(t *testing.T) {
	tests := []struct {
		name          string
		envValue      string
		cliValue      string
		expectedValue string
	}{
		{
			name:          "CLI overrides env",
			envValue:      "/env/contributoor",
			cliValue:      "/cli/contributoor",
			expectedValue: "/cli/contributoor",
		},
		{
			name:          "Env used when no CLI",
			envValue:      "/env/contributoor",
			cliValue:      "",
			expectedValue: "/env/contributoor",
		},
		{
			name:          "Default when no overrides",
			envValue:      "",
			cliValue:      "",
			expectedValue: "", // Config default would apply
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.NewDefaultConfig()
			originalDir := cfg.ContributoorDirectory

			// Set env var if provided
			if tt.envValue != "" {
				os.Setenv("CONTRIBUTOOR_DIRECTORY", tt.envValue)
				defer os.Unsetenv("CONTRIBUTOOR_DIRECTORY")
			}

			// Create CLI app
			app := cli.NewApp()
			app.Flags = []cli.Flag{
				&cli.StringFlag{Name: "contributoor-directory"},
			}

			app.Action = func(c *cli.Context) error {
				return applyConfigOverridesFromFlags(cfg, c)
			}

			// Build args
			args := []string{"app"}
			if tt.cliValue != "" {
				args = append(args, "--contributoor-directory", tt.cliValue)
			}

			// Run app
			err := app.Run(args)
			require.NoError(t, err)

			// Verify the value
			if tt.expectedValue != "" {
				assert.Equal(t, tt.expectedValue, cfg.ContributoorDirectory)
			} else {
				// Should keep original default
				assert.Equal(t, originalDir, cfg.ContributoorDirectory)
			}
		})
	}
}
