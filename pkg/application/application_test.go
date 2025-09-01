package application

import (
	"context"
	"testing"
	"time"

	"github.com/ethpandaops/contributoor/pkg/config/v1"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		opts    Options
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid configuration",
			opts: Options{
				Config: config.NewDefaultConfig(),
				Logger: logrus.New(),
			},
			wantErr: false,
		},
		{
			name:    "missing configuration",
			opts:    Options{},
			wantErr: true,
			errMsg:  "configuration is required",
		},
		{
			name: "nil logger creates default",
			opts: Options{
				Config: config.NewDefaultConfig(),
			},
			wantErr: false,
		},
		{
			name: "debug mode enabled",
			opts: Options{
				Config: config.NewDefaultConfig(),
				Debug:  true,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app, err := New(tt.opts)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				assert.Nil(t, app)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, app)
				assert.NotNil(t, app.config)
				assert.NotNil(t, app.log)
				assert.NotNil(t, app.servers)
				assert.NotNil(t, app.beaconNodes)
				assert.Equal(t, tt.opts.Debug, app.debug)
			}
		})
	}
}

func TestApplicationGetters(t *testing.T) {
	cfg := config.NewDefaultConfig()
	cfg.BeaconNodeAddress = defaultBeaconNodeAddress

	app, err := New(Options{
		Config: cfg,
	})
	require.NoError(t, err)

	// Test Config getter
	assert.Equal(t, cfg, app.Config())

	// Test BeaconNodes getter (should be empty before start)
	assert.NotNil(t, app.BeaconNodes())
	assert.Empty(t, app.BeaconNodes())

	// Test Metrics getter for non-existent beacon
	assert.Nil(t, app.Metrics("non-existent"))

	// Test IsHealthy (should be false before start)
	assert.False(t, app.IsHealthy())

	// Test Logger getter
	assert.NotNil(t, app.Logger())

	// Test Servers getter
	assert.NotNil(t, app.Servers())

	// Test ClockDrift getter (should be nil before start)
	assert.Nil(t, app.ClockDrift())
}

func TestHealthStatus(t *testing.T) {
	cfg := config.NewDefaultConfig()
	cfg.BeaconNodeAddress = defaultBeaconNodeAddress

	app, err := New(Options{
		Config: cfg,
	})
	require.NoError(t, err)

	// Get health status before start
	status := app.GetHealthStatus()
	assert.False(t, status.Healthy)
	assert.Empty(t, status.BeaconNodes)
}

func TestApplicationWithTestOptions(t *testing.T) {
	// Test using the test helper
	app := NewTestApplication(t, TestOptions{
		BeaconAddresses: []string{defaultBeaconNodeAddress, "http://localhost:5053"},
		Debug:           true,
	})

	// Verify configuration was set correctly
	assert.Contains(t, app.Config().BeaconNodeAddress, defaultBeaconNodeAddress)
	assert.Contains(t, app.Config().BeaconNodeAddress, "http://localhost:5053")
	assert.True(t, app.debug)
	assert.Empty(t, app.Config().MetricsAddress)
	assert.Empty(t, app.Config().PprofAddress)
	assert.Empty(t, app.Config().HealthCheckAddress)
}

func TestWaitForHealthy(t *testing.T) {
	cfg := config.NewDefaultConfig()
	cfg.BeaconNodeAddress = defaultBeaconNodeAddress

	app, err := New(Options{
		Config: cfg,
		Debug:  true,
	})
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Should timeout when not started
	healthy := app.WaitForHealthy(ctx, 100*time.Millisecond)
	assert.False(t, healthy)

	// Test context cancellation
	cancel()

	healthy = app.WaitForHealthy(ctx, 1*time.Second)

	assert.False(t, healthy)
}

func TestGetBeaconTraceIDs(t *testing.T) {
	cfg := config.NewDefaultConfig()
	cfg.BeaconNodeAddress = defaultBeaconNodeAddress

	app, err := New(Options{
		Config: cfg,
	})
	require.NoError(t, err)

	// Should be empty before initialization
	ids := app.GetBeaconTraceIDs()
	assert.Empty(t, ids)
}

func TestGetFirstHealthyBeacon(t *testing.T) {
	cfg := config.NewDefaultConfig()
	cfg.BeaconNodeAddress = defaultBeaconNodeAddress

	app, err := New(Options{
		Config: cfg,
	})
	require.NoError(t, err)

	// Should return empty string when no healthy beacons
	id := app.GetFirstHealthyBeacon()
	assert.Empty(t, id)
}
