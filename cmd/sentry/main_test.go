package main

import (
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"encoding/base64"

	"github.com/ethpandaops/contributoor/pkg/config/v1"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
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
				assert.Equal(t, config.NetworkName_NETWORK_NAME_SEPOLIA, cfg.NetworkName)
			},
		},
		{
			name: "beacon node address override",
			args: []string{"--beacon-node-address", "http://localhost:5052"},
			validate: func(t *testing.T, cfg *config.Config) {
				assert.Equal(t, "http://localhost:5052", cfg.BeaconNodeAddress)
			},
		},
		{
			name: "metrics address override",
			args: []string{"--metrics-address", "localhost:9091"},
			validate: func(t *testing.T, cfg *config.Config) {
				assert.Equal(t, "localhost:9091", cfg.MetricsAddress)
			},
		},
		{
			name: "log level override",
			args: []string{"--log-level", "debug"},
			validate: func(t *testing.T, cfg *config.Config) {
				assert.Equal(t, "debug", cfg.LogLevel)
			},
		},
		{
			name: "output server address override",
			args: []string{"--output-server-address", "localhost:8080"},
			validate: func(t *testing.T, cfg *config.Config) {
				require.NotNil(t, cfg.OutputServer)
				assert.Equal(t, "localhost:8080", cfg.OutputServer.Address)
			},
		},
		{
			name: "output server credentials override",
			args: []string{"--username", "user", "--password", "pass"},
			validate: func(t *testing.T, cfg *config.Config) {
				require.NotNil(t, cfg.OutputServer)
				expected := base64.StdEncoding.EncodeToString([]byte("user:pass"))
				assert.Equal(t, expected, cfg.OutputServer.Credentials)
			},
		},
		{
			name: "output server tls override",
			args: []string{"--output-server-tls", "true"},
			validate: func(t *testing.T, cfg *config.Config) {
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
				assert.Equal(t, config.NetworkName_NETWORK_NAME_SEPOLIA, cfg.NetworkName)
				assert.Equal(t, "http://localhost:5052", cfg.BeaconNodeAddress)
				assert.Equal(t, "localhost:9091", cfg.MetricsAddress)
				assert.Equal(t, "debug", cfg.LogLevel)
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
