package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"encoding/base64"

	"github.com/ethpandaops/contributoor/internal/events"
	"github.com/ethpandaops/contributoor/pkg/config/v1"
	"github.com/ethpandaops/contributoor/pkg/ethereum/mock"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
	"go.uber.org/mock/gomock"
)

func TestStartMetricsServer(t *testing.T) {
	tests := []struct {
		name         string
		metricsAddr  string
		expectServer bool
		expectError  bool
	}{
		{
			name:         "empty address skips server",
			metricsAddr:  "",
			expectServer: false,
		},
		{
			name:         "valid address starts server",
			metricsAddr:  "localhost:9090",
			expectServer: true,
		},
		{
			name:         "invalid address errors",
			metricsAddr:  "256.256.256.256:99999",
			expectServer: false,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &contributoor{
				log: logrus.New(),
				config: &config.Config{
					MetricsAddress: tt.metricsAddr,
				},
			}

			err := s.startMetricsServer()
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tt.expectServer {
				require.NotNil(t, s.metricsServer)

				waitForServer(t, s.metricsServer.Addr)

				client := http.Client{Timeout: time.Second}

				resp, err := client.Get(fmt.Sprintf("http://%s/metrics", s.metricsServer.Addr))
				require.NoError(t, err)
				assert.Equal(t, http.StatusOK, resp.StatusCode)

				resp.Body.Close()
				_ = s.metricsServer.Close()
			} else {
				assert.Nil(t, s.metricsServer)
			}
		})
	}
}

func TestStartPProfServer(t *testing.T) {
	tests := []struct {
		name         string
		pprofAddr    string
		expectServer bool
		expectError  bool
	}{
		{
			name:         "empty address skips server",
			pprofAddr:    "",
			expectServer: false,
		},
		{
			name:         "valid address starts server",
			pprofAddr:    "localhost:6060",
			expectServer: true,
		},
		{
			name:         "invalid address errors",
			pprofAddr:    "256.256.256.256:99999",
			expectServer: false,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &contributoor{
				log: logrus.New(),
				config: &config.Config{
					PprofAddress: tt.pprofAddr,
				},
			}

			err := s.startPProfServer()
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tt.expectServer {
				require.NotNil(t, s.pprofServer)

				waitForServer(t, s.pprofServer.Addr)

				client := http.Client{Timeout: time.Second}

				resp, err := client.Get(fmt.Sprintf("http://%s/debug/pprof/", s.pprofServer.Addr))
				require.NoError(t, err)
				assert.Equal(t, http.StatusOK, resp.StatusCode)

				resp.Body.Close()
				_ = s.pprofServer.Close()
			} else {
				assert.Nil(t, s.pprofServer)
			}
		})
	}
}

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
				assert.Equal(t, config.NetworkName_NETWORK_NAME_SEPOLIA, cfg.NetworkName)
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
			name: "multiple overrides",
			args: []string{
				"--network", "sepolia",
				"--beacon-node-address", "http://localhost:5052",
				"--metrics-address", "localhost:9091",
				"--log-level", "debug",
			},
			validate: func(t *testing.T, cfg *config.Config) {
				t.Helper()
				assert.Equal(t, config.NetworkName_NETWORK_NAME_SEPOLIA, cfg.NetworkName)
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
			setter:        func(c *config.Config, v string) { c.SetNetwork(v) },
			getter:        func(c *config.Config) string { return strings.ToLower(c.NetworkName.DisplayName()) },
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

func TestGenerateBeaconTraceIDs(t *testing.T) {
	tests := []struct {
		name      string
		addresses []string
		want      []string
	}{
		{
			name:      "single address",
			addresses: []string{"http://localhost:5052"},
			want:      []string{"bn_3d25b"},
		},
		{
			name:      "multiple addresses",
			addresses: []string{"http://localhost:5052", "http://localhost:5053"},
			want:      []string{"bn_3d25b", "bn_0e06e"},
		},
		{
			name: "long addresses",
			addresses: []string{
				"http://very-long-domain-name-spiders-snakes-elephants-and-tigers.com:5052",
				"http://another-very-long-domain-name-caterpillars-ants-slugs-and-beetles.com:5053",
			},
			want: []string{"bn_0c9e2", "bn_27f0a"},
		},
		{
			name:      "empty addresses",
			addresses: []string{"", ""},
			want:      []string{"bn_e3b0c", "bn_e3b0c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := generateBeaconTraceIDs(tt.addresses)
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestInitBeacons(t *testing.T) {
	prometheus.DefaultRegisterer = prometheus.NewRegistry()

	tests := []struct {
		name           string
		addresses      string
		expectedCount  int
		expectedError  bool
		expectedErrMsg string
	}{
		{
			name:          "single address",
			addresses:     "http://localhost:5052",
			expectedCount: 1,
		},
		{
			name:          "multiple addresses",
			addresses:     "http://localhost:5052,http://localhost:5053",
			expectedCount: 2,
		},
		{
			name:          "addresses with whitespace",
			addresses:     " http://localhost:5052 , http://localhost:5053 ",
			expectedCount: 2,
		},
		{
			name:          "empty address",
			addresses:     "",
			expectedCount: 1, // Empty string splits to [""]
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prometheus.DefaultRegisterer = prometheus.NewRegistry()

			cfg := config.NewDefaultConfig()
			cfg.BeaconNodeAddress = tt.addresses
			cfg.NetworkName = config.NetworkName_NETWORK_NAME_MAINNET
			cfg.OutputServer = &config.OutputServer{
				Address: "http://localhost:8080",
				Tls:     false,
			}

			s := &contributoor{
				log:    logrus.New(),
				config: cfg,
				debug:  true,
			}

			err := s.initBeacons(context.Background())
			if tt.expectedError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErrMsg)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectedCount, len(s.beaconNodes))

			// Verify each beacon node has unique trace ID.
			traceIDs := make(map[string]bool)
			for traceID := range s.beaconNodes {
				assert.False(t, traceIDs[traceID], "duplicate trace ID found: %s", traceID)
				traceIDs[traceID] = true
			}
		})
	}
}

func TestConnectBeacons(t *testing.T) {
	tests := []struct {
		name           string
		nodeCount      int
		setupNodes     func(*testing.T, map[string]*beaconNodeInstance)
		expectedError  bool
		errorContains  string
		expectedHealth int
	}{
		{
			name:           "single node success",
			nodeCount:      1,
			expectedHealth: 1,
			setupNodes: func(t *testing.T, nodes map[string]*beaconNodeInstance) {
				t.Helper()

				ctrl := gomock.NewController(t)
				for _, instance := range nodes {
					instance.node = setupMockBeaconNode(ctrl, "success")
				}
			},
		},
		{
			name:           "multiple nodes all succeed",
			nodeCount:      3,
			expectedHealth: 3,
			setupNodes: func(t *testing.T, nodes map[string]*beaconNodeInstance) {
				t.Helper()

				ctrl := gomock.NewController(t)
				for _, instance := range nodes {
					instance.node = setupMockBeaconNode(ctrl, "success")
				}
			},
		},
		{
			name:          "single node fails",
			nodeCount:     1,
			expectedError: true,
			errorContains: "all beacons failed to connect",
			setupNodes: func(t *testing.T, nodes map[string]*beaconNodeInstance) {
				t.Helper()

				ctrl := gomock.NewController(t)
				for _, instance := range nodes {
					instance.node = setupMockBeaconNode(ctrl, "fail")
				}
			},
		},
		{
			name:           "some nodes succeed some fail",
			nodeCount:      3,
			expectedHealth: 1,
			setupNodes: func(t *testing.T, nodes map[string]*beaconNodeInstance) {
				t.Helper()

				var (
					ctrl      = gomock.NewController(t)
					behaviors = []string{"success", "fail", "fail"}
					i         = 0
				)

				for _, instance := range nodes {
					instance.node = setupMockBeaconNode(ctrl, behaviors[i])
					i++
				}
			},
		},
		{
			name:          "all nodes fail",
			nodeCount:     3,
			expectedError: true,
			errorContains: "all beacons failed to connect",
			setupNodes: func(t *testing.T, nodes map[string]*beaconNodeInstance) {
				t.Helper()

				ctrl := gomock.NewController(t)
				for _, instance := range nodes {
					instance.node = setupMockBeaconNode(ctrl, "fail")
				}
			},
		},
		{
			name:          "timeout with no healthy nodes",
			nodeCount:     2,
			expectedError: true,
			errorContains: context.DeadlineExceeded.Error(),
			setupNodes: func(t *testing.T, nodes map[string]*beaconNodeInstance) {
				t.Helper()

				ctrl := gomock.NewController(t)
				for _, instance := range nodes {
					instance.node = setupMockBeaconNode(ctrl, "hang")
				}
			},
		},
		{
			name:           "timeout with some healthy nodes continues in background",
			nodeCount:      3,
			expectedHealth: 1,
			expectedError:  false,
			setupNodes: func(t *testing.T, nodes map[string]*beaconNodeInstance) {
				t.Helper()

				var (
					ctrl      = gomock.NewController(t)
					behaviors = []string{"success", "hang", "hang"}
					i         = 0
				)

				for _, instance := range nodes {
					instance.node = setupMockBeaconNode(ctrl, behaviors[i])
					i++
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prometheus.DefaultRegisterer = prometheus.NewRegistry()

			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()

			s := &contributoor{
				log:         logrus.New(),
				beaconNodes: make(map[string]*beaconNodeInstance),
			}

			for i := 0; i < tt.nodeCount; i++ {
				traceID := fmt.Sprintf("test_node_%d", i)
				s.beaconNodes[traceID] = &beaconNodeInstance{
					metrics: events.NewMetrics(fmt.Sprintf("test_metrics_%d", i)),
					cache:   events.NewDuplicateCache(),
					summary: events.NewSummary(s.log, traceID, time.Second),
				}
			}

			tt.setupNodes(t, s.beaconNodes)

			err := s.connectBeacons(ctx)

			if tt.expectedError {
				require.Error(t, err)

				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}

				return
			}

			require.NoError(t, err)

			// Count how many nodes actually became healthy.
			healthyCount := 0

			for _, instance := range s.beaconNodes {
				if mockNode, ok := instance.node.(*mock.MockBeaconNodeAPI); ok && mockNode != nil {
					// For mock nodes, we consider them healthy if they were set up with "success"
					// see 'setupMockBeaconNode'.
					if ch, _ := mockNode.Start(ctx); ch != nil {
						select {
						case <-ch:
							healthyCount++
						default:
						}
					}
				}
			}

			assert.Equal(t, tt.expectedHealth, healthyCount, "unexpected number of healthy nodes")
		})
	}
}

func waitForServer(t *testing.T, addr string) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)

	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			conn.Close()

			return
		}

		time.Sleep(100 * time.Millisecond)
	}

	t.Fatalf("Server at %s did not start within deadline", addr)
}

func setupMockBeaconNode(ctrl *gomock.Controller, behavior string) *mock.MockBeaconNodeAPI {
	mockNode := mock.NewMockBeaconNodeAPI(ctrl)

	switch behavior {
	case "success":
		ready := make(chan struct{})
		close(ready)
		mockNode.EXPECT().Start(gomock.Any()).Return(ready, nil).AnyTimes()
	case "fail":
		mockNode.EXPECT().Start(gomock.Any()).Return(nil, fmt.Errorf("mock node failure")).AnyTimes()
	case "hang":
		mockNode.EXPECT().Start(gomock.Any()).Return(make(chan struct{}), nil).AnyTimes()
	}

	return mockNode
}
