package main

import (
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/ethpandaops/contributoor/pkg/config/v1"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
