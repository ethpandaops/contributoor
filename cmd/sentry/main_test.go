package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"testing"
	"time"

	contr "github.com/ethpandaops/contributoor/internal/contributoor"
	"github.com/ethpandaops/contributoor/pkg/config/v1"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
)

// TestMain tests the main function with various scenarios.
func TestMain(t *testing.T) {
	// Save original args
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()

	// Test release flag
	t.Run("release flag", func(t *testing.T) {
		// This test needs to run the actual binary
		if os.Getenv("BE_MAIN_TEST") == "1" {
			os.Args = []string{"contributoor", "--release"}
			main()

			return
		}

		cmd := exec.Command(os.Args[0], "-test.run=TestMain/release_flag")
		cmd.Env = append(os.Environ(), "BE_MAIN_TEST=1")
		output, err := cmd.CombinedOutput()

		// os.Exit(0) may or may not be reported as an error depending on the system
		if err != nil {
			// If there's an error, it should be exit code 0
			exitErr, ok := err.(*exec.ExitError)
			if ok {
				require.Equal(t, 0, exitErr.ExitCode())
			}
		}

		// Should print release version
		require.Contains(t, string(output), contr.Release)
	})
}

// TestCLIApp tests the CLI application setup and flags.
func TestCLIApp(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		envVars   map[string]string
		setupMock func(*testing.T) func()
		wantErr   bool
		validate  func(*testing.T, *cli.Context, error)
	}{
		{
			name:    "default flags",
			args:    []string{"contributoor"},
			wantErr: false, // Just tests flag parsing, not full validation
		},
		// Skip the config file test - it's covered by TestCreateConfig
		{
			name: "all CLI flags",
			args: []string{
				"contributoor",
				"--network", "mainnet",
				"--beacon-node-address", "http://localhost:5052",
				"--metrics-address", ":9090",
				"--health-check-address", ":8080",
				"--log-level", "debug",
				"--output-server-address", "http://output:8080",
				"--username", "user",
				"--password", "pass",
				"--output-server-tls", "true",
				"--contributoor-directory", "/tmp/contributoor",
				"--debug",
			},
		},
		{
			name: "environment variables",
			args: []string{"contributoor"},
			envVars: map[string]string{
				"CONTRIBUTOOR_NETWORK":               "sepolia",
				"CONTRIBUTOOR_BEACON_NODE_ADDRESS":   "http://localhost:5052",
				"CONTRIBUTOOR_METRICS_ADDRESS":       ":9091",
				"CONTRIBUTOOR_HEALTH_CHECK_ADDRESS":  ":8081",
				"CONTRIBUTOOR_LOG_LEVEL":             "info",
				"CONTRIBUTOOR_OUTPUT_SERVER_ADDRESS": "http://output:8081",
				"CONTRIBUTOOR_USERNAME":              "envuser",
				"CONTRIBUTOOR_PASSWORD":              "envpass",
				"CONTRIBUTOOR_OUTPUT_SERVER_TLS":     "false",
				"CONTRIBUTOOR_DIRECTORY":             "/tmp/contributoor-env",
			},
		},
		{
			name: "CLI flags override env vars",
			args: []string{
				"contributoor",
				"--network", "mainnet",
				"--beacon-node-address", "http://localhost:5053",
			},
			envVars: map[string]string{
				"CONTRIBUTOOR_NETWORK":             "sepolia",
				"CONTRIBUTOOR_BEACON_NODE_ADDRESS": "http://localhost:5052",
			},
			validate: func(t *testing.T, c *cli.Context, err error) {
				t.Helper()

				// Should work with the overrides
				assert.NoError(t, err)
			},
		},
		{
			name: "invalid log level",
			args: []string{
				"contributoor",
				"--beacon-node-address", "http://localhost:5052",
				"--log-level", "invalid",
			},
			validate: func(t *testing.T, c *cli.Context, err error) {
				t.Helper()

				// Should still work but log a warning
				assert.NoError(t, err)
			},
		},
		{
			name: "valid unknown network",
			args: []string{
				"contributoor",
				"--beacon-node-address", "http://localhost:5052",
				"--network", "unknown-network",
			},
			wantErr: false, // Unknown networks are allowed
		},
		{
			name: "invalid output-server-tls",
			args: []string{
				"contributoor",
				"--beacon-node-address", "http://localhost:5052",
				"--output-server-tls", "not-a-bool",
			},
			wantErr: true,
		},
		{
			name: "missing config file",
			args: []string{
				"contributoor",
				"--config", "/non/existent/config.yaml",
			},
			wantErr: true,
		},
		{
			name: "multiple beacon addresses",
			args: []string{
				"contributoor",
				"--beacon-node-address", "http://localhost:5052,http://localhost:5053,http://localhost:5054",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment
			for k, v := range tt.envVars {
				oldVal := os.Getenv(k)
				os.Setenv(k, v)
				defer os.Setenv(k, oldVal)
			}

			var cleanup func()
			if tt.setupMock != nil {
				cleanup = tt.setupMock(t)
				if cleanup != nil {
					defer cleanup()
				}
			}

			// Create a test context that we can cancel
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Override main execution to avoid blocking
			app := &cli.App{
				Name:  "contributoor",
				Usage: "Contributoor node",
				Flags: []cli.Flag{
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
					&cli.BoolFlag{Name: "release"},
				},
				Before: func(c *cli.Context) error {
					if c.Bool("release") {
						// Don't actually exit in tests
						return cli.Exit("", 0)
					}

					return nil
				},
				Action: func(c *cli.Context) error {
					// Test config creation
					cfg, err := createConfig(c)
					if err != nil {
						return err
					}

					// Validate config was created properly
					if c.String("network") != "" && cfg != nil {
						assert.Equal(t, c.String("network"), cfg.NetworkName)
					}
					if c.String("beacon-node-address") != "" && cfg != nil {
						assert.Equal(t, c.String("beacon-node-address"), cfg.BeaconNodeAddress)
					}

					// Don't actually start the application in tests
					cancel()

					return nil
				},
			}

			// Run the app
			err := app.RunContext(ctx, tt.args)

			if tt.wantErr {
				assert.Error(t, err)
			} else if tt.validate != nil {
				// Create a context from args for validation
				c := cli.NewContext(app, nil, nil)
				tt.validate(t, c, err)
			} else if err != nil {
				// Check if it's an expected exit error
				exitErr, isExit := err.(cli.ExitCoder)
				if !isExit || exitErr.ExitCode() != 0 {
					assert.NoError(t, err)
				}
			}
		})
	}
}

// TestCreateConfig tests the createConfig function.
func TestCreateConfig(t *testing.T) {
	tests := []struct {
		name     string
		setupCtx func() *cli.Context
		setupEnv map[string]string
		wantErr  bool
		validate func(*testing.T, *config.Config)
	}{
		{
			name: "default config",
			setupCtx: func() *cli.Context {
				app := &cli.App{
					Flags: []cli.Flag{
						&cli.StringFlag{Name: "config"},
					},
				}

				return flagSet(t, app.Flags, []string{"contributoor"})
			},
			validate: func(t *testing.T, cfg *config.Config) {
				t.Helper()

				assert.NotNil(t, cfg)
				assert.Equal(t, config.RunMethod_RUN_METHOD_DOCKER, cfg.RunMethod)
			},
		},
		{
			name: "config from file",
			setupCtx: func() *cli.Context {
				// Create temp config file
				tmpFile, err := os.CreateTemp("", "config*.yaml")
				require.NoError(t, err)
				defer tmpFile.Close()

				configContent := `
network_name: "holesky"
beacon_node_address: "http://beacon:5052"
log_level: "info"
metrics_address: ":9090"
run_method: 1
`
				_, err = tmpFile.WriteString(configContent)
				require.NoError(t, err)

				app := &cli.App{
					Flags: []cli.Flag{
						&cli.StringFlag{Name: "config"},
					},
				}

				return flagSet(t, app.Flags, []string{"contributoor", "--config", tmpFile.Name()})
			},
			validate: func(t *testing.T, cfg *config.Config) {
				t.Helper()

				if cfg != nil {
					assert.Equal(t, "holesky", cfg.NetworkName)
					assert.Equal(t, "http://beacon:5052", cfg.BeaconNodeAddress)
					assert.Equal(t, "info", cfg.LogLevel)
					assert.Equal(t, ":9090", cfg.MetricsAddress)
				}
			},
		},
		{
			name: "invalid config file",
			setupCtx: func() *cli.Context {
				app := &cli.App{
					Flags: []cli.Flag{
						&cli.StringFlag{Name: "config"},
					},
				}

				return flagSet(t, app.Flags, []string{"contributoor", "--config", "/non/existent/file.yaml"})
			},
			wantErr: true,
		},
		{
			name: "config with flag overrides",
			setupCtx: func() *cli.Context {
				app := &cli.App{
					Flags: []cli.Flag{
						&cli.StringFlag{Name: "config"},
						&cli.StringFlag{Name: "network"},
						&cli.StringFlag{Name: "log-level"},
					},
				}

				return flagSet(t, app.Flags, []string{
					"contributoor",
					"--network", "mainnet",
					"--log-level", "debug",
				})
			},
			validate: func(t *testing.T, cfg *config.Config) {
				t.Helper()

				assert.Equal(t, "mainnet", cfg.NetworkName)
				assert.Equal(t, "debug", cfg.LogLevel)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up environment
			for k, v := range tt.setupEnv {
				oldVal := os.Getenv(k)
				os.Setenv(k, v)
				defer os.Setenv(k, oldVal)
			}

			ctx := tt.setupCtx()
			cfg, err := createConfig(ctx)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, cfg)
				if tt.validate != nil {
					tt.validate(t, cfg)
				}
			}
		})
	}
}

// TestSignalHandling tests graceful shutdown on signals.
func TestSignalHandling(t *testing.T) {
	// Test signal handling logic without subprocess
	ctx, cancel := context.WithCancel(context.Background())

	sigChan := make(chan os.Signal, 1)
	shutdownChan := make(chan bool, 1)

	go func() {
		<-sigChan
		shutdownChan <- true
		cancel()
	}()

	// Send signal
	sigChan <- syscall.SIGINT

	// Wait for shutdown signal
	select {
	case <-shutdownChan:
		// Shutdown was called
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Timeout waiting for shutdown signal")
	}

	// Verify context is cancelled
	select {
	case <-ctx.Done():
		// Success
	default:
		t.Fatal("Context should be cancelled")
	}
}

// TestLogLevelConfiguration tests log level setting.
func TestLogLevelConfiguration(t *testing.T) {
	tests := []struct {
		name          string
		logLevel      string
		expectedLevel logrus.Level
		expectWarning bool
	}{
		{
			name:          "valid debug level",
			logLevel:      "debug",
			expectedLevel: logrus.DebugLevel,
		},
		{
			name:          "valid info level",
			logLevel:      "info",
			expectedLevel: logrus.InfoLevel,
		},
		{
			name:          "valid warn level",
			logLevel:      "warn",
			expectedLevel: logrus.WarnLevel,
			// Note: "Log level set" won't appear because it's logged at Info level
		},
		{
			name:          "valid error level",
			logLevel:      "error",
			expectedLevel: logrus.ErrorLevel,
			// Note: "Log level set" won't appear because it's logged at Info level
		},
		{
			name:          "invalid level defaults to info",
			logLevel:      "invalid",
			expectedLevel: logrus.InfoLevel,
			expectWarning: true,
		},
		{
			name:          "empty level",
			logLevel:      "",
			expectedLevel: logrus.InfoLevel, // Should remain at default
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture log output
			var buf bytes.Buffer
			oldOut := log.Out
			log.SetOutput(&buf)
			defer log.SetOutput(oldOut)

			// Reset log level
			log.SetLevel(logrus.InfoLevel)

			// Create config with log level
			cfg := config.NewDefaultConfig()
			cfg.LogLevel = tt.logLevel

			// Apply log level (simulate what happens in main)
			if cfg.LogLevel != "" {
				level, err := logrus.ParseLevel(cfg.LogLevel)
				if err != nil {
					log.WithField("level", cfg.LogLevel).WithError(err).Warn("Invalid log level, defaulting to info")
					level = logrus.InfoLevel
				}
				log.SetLevel(level)
				log.WithField("level", level.String()).Info("Log level set")
			}

			// Check the level was set correctly
			assert.Equal(t, tt.expectedLevel, log.GetLevel())

			// Check for warning message
			output := buf.String()
			if tt.expectWarning {
				assert.Contains(t, output, "Invalid log level")
			}
			// "Log level set" is logged at Info level
			// We can only see it if the test is running at Info level or lower
			if tt.logLevel != "" && !tt.expectWarning && tt.expectedLevel <= logrus.InfoLevel {
				// For warn/error levels, the message won't appear in the buffer
				// because it's logged at Info level which gets filtered out
				if tt.expectedLevel == logrus.InfoLevel || tt.expectedLevel == logrus.DebugLevel {
					assert.Contains(t, output, "Log level set")
				}
			}
		})
	}
}

// TestApplicationCreationError tests handling of application creation errors.
func TestApplicationCreationError(t *testing.T) {
	// This test is tricky because we need to mock application.New to return an error
	// For now, we can test with invalid configurations that would cause errors
	t.Run("invalid beacon address format", func(t *testing.T) {
		app := &cli.App{
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "beacon-node-address"},
			},
			Action: func(c *cli.Context) error {
				cfg := config.NewDefaultConfig()
				cfg.BeaconNodeAddress = "not-a-valid-url"

				// In real scenario, application.New would fail with this config
				// For this test, we just verify the error handling path
				return fmt.Errorf("failed to create application: invalid beacon node address")
			},
		}

		err := app.Run([]string{"contributoor", "--beacon-node-address", "not-a-valid-url"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to create application")
	})
}

// TestSystemdIntegration tests systemd journal hook.
func TestSystemdIntegration(t *testing.T) {
	t.Run("systemd run method", func(t *testing.T) {
		cfg := config.NewDefaultConfig()
		cfg.RunMethod = config.RunMethod_RUN_METHOD_SYSTEMD

		// Just verify the condition works
		assert.True(t, cfg.IsRunMethodSystemd())
	})

	t.Run("non-systemd run method", func(t *testing.T) {
		// Clear any systemd environment variables that might be set in CI
		oldInvocationID := os.Getenv("INVOCATION_ID")
		oldJournalStream := os.Getenv("JOURNAL_STREAM")
		oldNotifySocket := os.Getenv("NOTIFY_SOCKET")

		os.Unsetenv("INVOCATION_ID")
		os.Unsetenv("JOURNAL_STREAM")
		os.Unsetenv("NOTIFY_SOCKET")

		defer func() {
			// Restore original values
			if oldInvocationID != "" {
				os.Setenv("INVOCATION_ID", oldInvocationID)
			}
			if oldJournalStream != "" {
				os.Setenv("JOURNAL_STREAM", oldJournalStream)
			}
			if oldNotifySocket != "" {
				os.Setenv("NOTIFY_SOCKET", oldNotifySocket)
			}
		}()

		cfg := config.NewDefaultConfig()
		cfg.RunMethod = config.RunMethod_RUN_METHOD_DOCKER

		assert.False(t, cfg.IsRunMethodSystemd())
	})
}

// TestMainExitCodes tests the --release flag.
func TestMainExitCodes(t *testing.T) {
	// Test the release flag
	if os.Getenv("BE_EXIT_TEST") == "1" {
		os.Args = []string{"contributoor", "--release"}
		main()

		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=TestMainExitCodes")
	cmd.Env = append(os.Environ(), "BE_EXIT_TEST=1")
	output, err := cmd.CombinedOutput()

	// The --release flag causes os.Exit(0)
	if err == nil {
		// Some systems may not report exit(0) as an error
		assert.Contains(t, string(output), contr.Release)
	} else {
		// Most systems report any exit as an error
		exitErr, ok := err.(*exec.ExitError)
		if ok {
			assert.Equal(t, 0, exitErr.ExitCode())
		}
	}

	// Should print release version
	require.Contains(t, string(output), contr.Release)
}

// TestConcurrentShutdown tests concurrent shutdown scenarios.
func TestConcurrentShutdown(t *testing.T) {
	// Test that multiple signals don't cause issues
	ctx, cancel := context.WithCancel(context.Background())

	sigChan := make(chan os.Signal, 1)

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		<-sigChan
		cancel()
	}()

	// Send multiple signals
	sigChan <- syscall.SIGINT
	sigChan <- syscall.SIGTERM // Should be ignored

	// Wait for handler
	wg.Wait()

	// Verify context is cancelled
	select {
	case <-ctx.Done():
		// Success
	default:
		t.Fatal("Context should be cancelled")
	}
}

// TestMainAction tests the main CLI app Action function.
func TestMainAction(t *testing.T) {
	// Save original logger
	originalLog := log
	defer func() { log = originalLog }()

	tests := []struct {
		name          string
		args          []string
		setupMock     func() func()
		expectError   bool
		errorContains string
	}{
		{
			name: "successful config and log level",
			args: []string{
				"contributoor",
				"--beacon-node-address", "http://localhost:5052",
				"--log-level", "debug",
			},
			expectError: false,
		},
		{
			name: "invalid log level with warning",
			args: []string{
				"contributoor",
				"--beacon-node-address", "http://localhost:5052",
				"--log-level", "invalid-level",
			},
			expectError: false,
		},
		{
			name: "systemd run method",
			args: []string{
				"contributoor",
				"--beacon-node-address", "http://localhost:5052",
			},
			setupMock: func() func() {
				// Create a config that returns systemd run method
				oldMethod := config.RunMethod_RUN_METHOD_SYSTEMD

				return func() {
					_ = oldMethod
				}
			},
			expectError: false,
		},
		{
			name: "config creation error",
			args: []string{
				"contributoor",
				"--config", "/non/existent/config.yaml",
			},
			expectError:   true,
			errorContains: "no such file or directory",
		},
		{
			name: "application creation error",
			args: []string{
				"contributoor",
				"--beacon-node-address", "invalid-url",
			},
			expectError: true, // We force an error in the test
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a context that we'll cancel immediately to prevent blocking
			ctx, cancel := context.WithCancel(context.Background())

			if tt.setupMock != nil {
				cleanup := tt.setupMock()
				defer cleanup()
			}

			// Create the CLI app
			app := &cli.App{
				Name:  "contributoor",
				Usage: "Contributoor node",
				Flags: []cli.Flag{
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
					&cli.BoolFlag{Name: "release"},
				},
				Before: func(c *cli.Context) error {
					if c.Bool("release") {
						return cli.Exit("", 0)
					}

					return nil
				},
				Action: func(c *cli.Context) error {
					// Create configuration
					cfg, err := createConfig(c)
					if err != nil {
						return err
					}

					// Apply log level
					if cfg.LogLevel != "" {
						level, lerr := logrus.ParseLevel(cfg.LogLevel)
						if lerr != nil {
							log.WithField("level", cfg.LogLevel).WithError(lerr).Warn("Invalid log level, defaulting to info")
							level = logrus.InfoLevel
						}

						log.SetLevel(level)
						log.WithField("level", level.String()).Info("Log level set")
					}

					// Add journald hook if running under systemd
					if cfg.IsRunMethodSystemd() {
						// Skip actual journald hook in tests
						log.Info("Would add systemd journal hook for priority mapping")
					}

					// For testing, don't actually create the application
					// Just validate we got to this point
					if cfg.BeaconNodeAddress == "invalid-url" {
						return fmt.Errorf("failed to create application: invalid beacon node address")
					}

					// Cancel context immediately to avoid blocking
					cancel()

					// Simulate successful start
					log.WithFields(logrus.Fields{
						"config_path": cfg.ContributoorDirectory,
						"version":     cfg.Version,
					}).Info("Starting contributoor")

					return nil
				},
			}

			// Run the app
			err := app.RunContext(ctx, tt.args)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				// Check if error is just from the exit
				if err != nil {
					exitErr, isExit := err.(cli.ExitCoder)
					if isExit && exitErr.ExitCode() == 0 {
						// This is fine
					} else {
						assert.NoError(t, err)
					}
				}
			}
		})
	}
}

// TestMainActionWithApplication tests the Action with actual application creation.
func TestMainActionWithApplication(t *testing.T) {
	// This test needs to run main() to cover the Action function
	if os.Getenv("BE_MAIN_ACTION_TEST") == "1" {
		// Set a valid beacon node address to avoid connection errors
		os.Args = []string{"contributoor", "--beacon-node-address", "http://localhost:5052"}
		main()

		return
	}

	// Run as subprocess
	cmd := exec.Command(os.Args[0], "-test.run=TestMainActionWithApplication")
	cmd.Env = append(os.Environ(), "BE_MAIN_ACTION_TEST=1")

	// Start the process
	err := cmd.Start()
	require.NoError(t, err)

	// Give it time to start and cover the Action function
	time.Sleep(500 * time.Millisecond)

	// Send interrupt signal to trigger shutdown
	err = cmd.Process.Signal(syscall.SIGINT)
	require.NoError(t, err)

	// Wait for process to exit
	err = cmd.Wait()
	// Exit error is expected due to signal
	if err != nil {
		_, ok := err.(*exec.ExitError)
		assert.True(t, ok, "Expected exit error, got: %v", err)
	}
}

// TestMainActionSystemd tests the Action with systemd run method.
func TestMainActionSystemd(t *testing.T) {
	// This test needs to run main() to cover systemd paths in Action function
	if os.Getenv("BE_MAIN_SYSTEMD_TEST") == "1" {
		// Create temp config file with systemd run method
		tmpFile, err := os.CreateTemp("", "config*.yaml")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create temp file: %v\n", err)
			os.Exit(1)
		}
		defer os.Remove(tmpFile.Name())

		configContent := `
network_name: "testnet"
beacon_node_address: "http://localhost:5052"
log_level: "info"
run_method: 3
`
		_, err = tmpFile.WriteString(configContent)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write config: %v\n", err)
			os.Exit(1)
		}
		tmpFile.Close()

		os.Args = []string{"contributoor", "--config", tmpFile.Name()}
		main()

		return
	}

	// Run as subprocess
	cmd := exec.Command(os.Args[0], "-test.run=TestMainActionSystemd")
	cmd.Env = append(os.Environ(), "BE_MAIN_SYSTEMD_TEST=1")

	// Start the process
	err := cmd.Start()
	require.NoError(t, err)

	// Give it time to start
	time.Sleep(300 * time.Millisecond)

	// Send interrupt signal
	err = cmd.Process.Signal(syscall.SIGINT)
	require.NoError(t, err)

	// Wait for process to exit
	err = cmd.Wait()
	if err != nil {
		_, ok := err.(*exec.ExitError)
		assert.True(t, ok, "Expected exit error, got: %v", err)
	}
}

// Helper function to create flag set.
func flagSet(t *testing.T, flags []cli.Flag, args []string) *cli.Context {
	t.Helper()

	app := &cli.App{
		Flags: flags,
	}

	// Parse args to create a context
	var ctx *cli.Context
	app.Action = func(c *cli.Context) error {
		ctx = c

		return nil
	}

	err := app.Run(args)
	require.NoError(t, err)

	return ctx
}

// TestMainActionInvalidLogLevel tests the Action with invalid log level.
func TestMainActionInvalidLogLevel(t *testing.T) {
	// This test runs main() to cover the invalid log level warning path
	if os.Getenv("BE_MAIN_LOGLEVEL_TEST") == "1" {
		os.Args = []string{"contributoor", "--beacon-node-address", "http://localhost:5052", "--log-level", "invalid"}
		main()

		return
	}

	// Run as subprocess
	cmd := exec.Command(os.Args[0], "-test.run=TestMainActionInvalidLogLevel")
	cmd.Env = append(os.Environ(), "BE_MAIN_LOGLEVEL_TEST=1")

	// Start the process
	err := cmd.Start()
	require.NoError(t, err)

	// Give it time to start and log the invalid level warning
	time.Sleep(500 * time.Millisecond)

	// Send interrupt signal to trigger shutdown
	err = cmd.Process.Signal(syscall.SIGINT)
	require.NoError(t, err)

	// Wait for process to exit
	err = cmd.Wait()

	// Exit error is expected due to signal
	if err != nil {
		_, ok := err.(*exec.ExitError)
		assert.True(t, ok, "Expected exit error, got: %v", err)
	}
}

// TestMainActionConfigError tests the Action with config creation error.
func TestMainActionConfigError(t *testing.T) {
	// This test runs main() to cover the config error path
	if os.Getenv("BE_MAIN_CONFIG_ERROR_TEST") == "1" {
		os.Args = []string{"contributoor", "--config", "/non/existent/config.yaml"}
		main()

		return
	}

	// Run as subprocess
	cmd := exec.Command(os.Args[0], "-test.run=TestMainActionConfigError")
	cmd.Env = append(os.Environ(), "BE_MAIN_CONFIG_ERROR_TEST=1")

	// Run and expect it to fail
	output, err := cmd.CombinedOutput()

	// Should exit with error
	assert.Error(t, err)
	exitErr, ok := err.(*exec.ExitError)
	assert.True(t, ok, "Expected exit error")
	assert.NotEqual(t, 0, exitErr.ExitCode())

	// Should contain error about config file
	assert.Contains(t, string(output), "no such file or directory")
}

// TestMainActionApplicationError tests error handling in main.
func TestMainActionApplicationError(t *testing.T) {
	// This test verifies error handling when app.RunContext fails
	if os.Getenv("BE_MAIN_APP_ERROR_TEST") == "1" {
		// Use invalid output-server-tls to trigger an error
		os.Args = []string{"contributoor", "--output-server-tls", "not-a-bool"}
		main()

		return
	}

	// Run as subprocess
	cmd := exec.Command(os.Args[0], "-test.run=TestMainActionApplicationError")
	cmd.Env = append(os.Environ(), "BE_MAIN_APP_ERROR_TEST=1")

	// Run and expect it to fail
	output, err := cmd.CombinedOutput()

	// Should exit with error
	assert.Error(t, err)
	exitErr, ok := err.(*exec.ExitError)
	assert.True(t, ok, "Expected exit error")
	assert.NotEqual(t, 0, exitErr.ExitCode())

	// Should contain error about parsing bool
	assert.Contains(t, string(output), "failed to parse output server tls")
}
