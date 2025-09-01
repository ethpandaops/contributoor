package application

import (
	"context"
	"testing"

	"github.com/ethpandaops/contributoor/pkg/config/v1"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateBeaconTraceIDs(t *testing.T) {
	tests := []struct {
		name      string
		addresses []string
		wantErr   bool
	}{
		{
			name:      "single address",
			addresses: []string{"http://localhost:5052"},
			wantErr:   false,
		},
		{
			name:      "multiple addresses",
			addresses: []string{"http://localhost:5052", "http://localhost:5053", "http://localhost:5054"},
			wantErr:   false,
		},
		{
			name:      "duplicate addresses get unique IDs",
			addresses: []string{"http://localhost:5052", "http://localhost:5052"},
			wantErr:   false,
		},
		{
			name:      "empty addresses",
			addresses: []string{},
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ids, err := generateBeaconTraceIDs(tt.addresses)

			if tt.wantErr {
				require.Error(t, err)
				assert.Nil(t, ids)
			} else {
				require.NoError(t, err)
				assert.Len(t, ids, len(tt.addresses))

				// Verify all IDs are unique
				uniqueIDs := make(map[string]bool)

				for _, id := range ids {
					assert.NotEmpty(t, id)
					assert.False(t, uniqueIDs[id], "Found duplicate ID: %s", id)
					uniqueIDs[id] = true
				}
			}
		})
	}
}

func TestInitCache(t *testing.T) {
	app := &Application{
		log: logrus.New(),
	}

	cache, err := app.initCache()
	require.NoError(t, err)
	assert.NotNil(t, cache)
}

func TestInitMetrics(t *testing.T) {
	app := &Application{
		log: logrus.New(),
	}

	tests := []struct {
		name    string
		traceID string
	}{
		{
			name:    "simple trace ID",
			traceID: "test123",
		},
		{
			name:    "trace ID with dashes",
			traceID: "test-123-abc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metrics, err := app.initMetrics(tt.traceID)
			require.NoError(t, err)
			assert.NotNil(t, metrics)
		})
	}
}

func TestInitSummary(t *testing.T) {
	app := &Application{
		log: logrus.New(),
	}

	log := logrus.New().WithField("test", "true")
	summary, err := app.initSummary(log, "test-trace-id")
	require.NoError(t, err)
	assert.NotNil(t, summary)
}

func TestInitSinks(t *testing.T) {
	tests := []struct {
		name      string
		debugMode bool
		expectErr bool
	}{
		{
			name:      "debug mode creates stdout sink",
			debugMode: true,
			expectErr: false,
		},
		{
			name:      "production mode creates xatu sink",
			debugMode: false,
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.NewDefaultConfig()
			cfg.SetOutputServerAddress("localhost:8080")

			app := &Application{
				config: cfg,
				log:    logrus.New(),
				debug:  tt.debugMode,
			}

			ctx := context.Background()
			log := logrus.New().WithField("test", "true")

			sinks, err := app.initSinks(ctx, log, "test-trace-id")

			if tt.expectErr {
				require.Error(t, err)
				assert.Nil(t, sinks)
			} else {
				require.NoError(t, err)
				assert.NotEmpty(t, sinks)
				assert.Len(t, sinks, 1)

				// Clean up
				for _, sink := range sinks {
					_ = sink.Stop(ctx)
				}
			}
		})
	}
}
