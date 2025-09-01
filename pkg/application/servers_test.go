package application

import (
	"context"
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
		name           string
		metricsAddress string
		expectStart    bool
	}{
		{
			name:           "metrics server enabled",
			metricsAddress: "127.0.0.1:19999",
			expectStart:    true,
		},
		{
			name:           "metrics server disabled",
			metricsAddress: "",
			expectStart:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.NewDefaultConfig()
			cfg.MetricsAddress = tt.metricsAddress

			app := &Application{
				config:  cfg,
				log:     logrus.New(),
				servers: &ServerManager{},
			}

			err := app.startMetricsServer()
			require.NoError(t, err)

			if tt.expectStart {
				assert.NotNil(t, app.servers.metricsServer)

				// Give server time to start
				time.Sleep(100 * time.Millisecond)

				// Test that metrics endpoint is accessible
				resp, err := http.Get("http://" + tt.metricsAddress + "/metrics")
				require.NoError(t, err)

				defer resp.Body.Close()

				assert.Equal(t, http.StatusOK, resp.StatusCode)

				// Clean up
				ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
				defer cancel()

				app.stopServers(ctx)
			} else {
				assert.Nil(t, app.servers.metricsServer)
			}
		})
	}
}

func TestStartHealthCheckServer(t *testing.T) {
	tests := []struct {
		name               string
		healthCheckAddress string
		expectStart        bool
	}{
		{
			name:               "health check server enabled",
			healthCheckAddress: "127.0.0.1:19998",
			expectStart:        true,
		},
		{
			name:               "health check server disabled",
			healthCheckAddress: "",
			expectStart:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.NewDefaultConfig()
			cfg.HealthCheckAddress = tt.healthCheckAddress

			app := &Application{
				config:      cfg,
				log:         logrus.New(),
				servers:     &ServerManager{},
				beaconNodes: make(map[string]*BeaconNodeInstance),
			}

			err := app.startHealthCheckServer()
			require.NoError(t, err)

			if tt.expectStart {
				assert.NotNil(t, app.servers.healthCheckServer)

				// Give server time to start
				time.Sleep(100 * time.Millisecond)

				// Test that health endpoint is accessible
				resp, err := http.Get("http://" + tt.healthCheckAddress + "/healthz")
				require.NoError(t, err)

				defer resp.Body.Close()

				// Should return 503 since no beacons are healthy
				assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)

				// Clean up
				ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
				defer cancel()

				app.stopServers(ctx)
			} else {
				assert.Nil(t, app.servers.healthCheckServer)
			}
		})
	}
}

func TestStopServers(t *testing.T) {
	cfg := config.NewDefaultConfig()
	cfg.MetricsAddress = "127.0.0.1:19997"
	cfg.PprofAddress = "127.0.0.1:19996"
	cfg.HealthCheckAddress = "127.0.0.1:19995"

	app := &Application{
		config:      cfg,
		log:         logrus.New(),
		servers:     &ServerManager{},
		beaconNodes: make(map[string]*BeaconNodeInstance),
	}

	// Start all servers
	require.NoError(t, app.startMetricsServer())
	require.NoError(t, app.startPProfServer())
	require.NoError(t, app.startHealthCheckServer())

	// Give servers time to start
	time.Sleep(100 * time.Millisecond)

	// Verify all servers are running
	assert.NotNil(t, app.servers.metricsServer)
	assert.NotNil(t, app.servers.pprofServer)
	assert.NotNil(t, app.servers.healthCheckServer)

	// Stop all servers
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	app.stopServers(ctx)

	// Give servers time to stop
	time.Sleep(100 * time.Millisecond)

	// Verify servers are no longer accessible
	resp, err := http.Get("http://" + cfg.MetricsAddress + "/metrics")
	if err == nil {
		resp.Body.Close()
	}

	assert.Error(t, err)

	resp2, err := http.Get("http://" + cfg.HealthCheckAddress + "/healthz")
	if err == nil {
		resp2.Body.Close()
	}

	assert.Error(t, err)
}
