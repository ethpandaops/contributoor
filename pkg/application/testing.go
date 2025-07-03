package application

import (
	"context"
	"testing"
	"time"

	"github.com/ethpandaops/contributoor/pkg/config/v1"
	"github.com/ethpandaops/contributoor/pkg/ethereum"
	"github.com/sirupsen/logrus"
)

const defaultBeaconNodeAddress = "http://localhost:5052"

// TestOptions provides convenient options for creating Applications in tests.
type TestOptions struct {
	// T is the testing context, used for cleanup registration
	T *testing.T

	// Config to use. If nil, a minimal test config will be created
	Config *config.Config

	// Logger to use. If nil, a test logger will be created
	Logger logrus.FieldLogger

	// Debug mode
	Debug bool

	// Custom beacon addresses. If empty, uses localhost:5052
	BeaconAddresses []string
}

// NewTestApplication creates an Application configured for testing.
// It automatically registers cleanup functions with testing.T.
func NewTestApplication(t *testing.T, opts TestOptions) *Application {
	t.Helper()

	// Create default test config if not provided
	if opts.Config == nil {
		cfg := config.NewDefaultConfig()

		// Set test-friendly defaults
		if len(opts.BeaconAddresses) > 0 {
			cfg.BeaconNodeAddress = opts.BeaconAddresses[0]
			for i := 1; i < len(opts.BeaconAddresses); i++ {
				cfg.BeaconNodeAddress += "," + opts.BeaconAddresses[i]
			}
		} else {
			cfg.BeaconNodeAddress = defaultBeaconNodeAddress
		}

		cfg.MetricsAddress = ""     // Disable metrics by default in tests
		cfg.PprofAddress = ""       // Disable pprof by default in tests
		cfg.HealthCheckAddress = "" // Disable health check by default in tests
		cfg.NetworkName = "testnet"
		cfg.LogLevel = "debug"
		cfg.ContributoorDirectory = t.TempDir()
		cfg.RunMethod = config.RunMethod_RUN_METHOD_BINARY

		opts.Config = cfg
	}

	// Create test logger if not provided
	if opts.Logger == nil {
		logger := logrus.New()
		logger.SetLevel(logrus.DebugLevel)
		opts.Logger = logger.WithField("test", t.Name())
	}

	// Create application
	app, err := New(Options{
		Config: opts.Config,
		Logger: opts.Logger,
		Debug:  opts.Debug,
	})
	if err != nil {
		t.Fatalf("Failed to create test application: %v", err)
	}

	// Register cleanup
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := app.Stop(ctx); err != nil {
			t.Logf("Failed to stop application during cleanup: %v", err)
		}
	})

	return app
}

// WaitForHealthy waits for at least one beacon node to become healthy.
// Returns true if healthy within the timeout, false otherwise.
func (a *Application) WaitForHealthy(ctx context.Context, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(100 * time.Millisecond)

	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return false
		case <-ticker.C:
			if a.IsHealthy() {
				return true
			}

			if time.Now().After(deadline) {
				return false
			}
		}
	}
}

// GetBeaconTraceIDs returns a slice of all beacon trace IDs.
// Useful for tests that need to verify specific beacons.
func (a *Application) GetBeaconTraceIDs() []string {
	ids := make([]string, 0, len(a.beaconNodes))
	for id := range a.beaconNodes {
		ids = append(ids, id)
	}

	return ids
}

// GetFirstHealthyBeacon returns the trace ID of the first healthy beacon found.
// Returns empty string if no healthy beacons exist.
func (a *Application) GetFirstHealthyBeacon() string {
	for traceID, instance := range a.beaconNodes {
		if node, ok := instance.Node.(*ethereum.BeaconWrapper); ok && node.IsHealthy() {
			return traceID
		}
	}

	return ""
}
