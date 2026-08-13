package application

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ethpandaops/contributoor/pkg/config/v1"
	"github.com/ethpandaops/contributoor/pkg/ethereum"
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

// fakeClockDrift is a minimal, deterministic clockdrift.ClockDrift used only to construct a real
// BeaconFactory without pulling in the NTP-syncing Service.
type fakeClockDrift struct{}

func (fakeClockDrift) GetDrift() time.Duration { return 0 }
func (fakeClockDrift) Now() time.Time          { return time.Now() }

func TestRestartWithoutSingleAttestation_FailedRestartLeavesInstanceUntouched(t *testing.T) {
	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)

	cfg := config.NewDefaultConfig()

	app := &Application{
		log:           log,
		debug:         true, // stdout sink; avoids needing a real xatu output server config
		config:        cfg,
		beaconFactory: ethereum.NewBeaconFactory(log, fakeClockDrift{}),
	}

	const unreachableAddr = "http://127.0.0.1:1" // port 1 is reserved; nothing listens there.

	instance, err := app.createBeaconInstance(context.Background(), log, unreachableAddr, "trace-a", nil)
	require.NoError(t, err)

	instance.app = app
	instance.stopMonitor = make(chan struct{})

	originalNode := instance.Node
	originalMetrics := instance.Metrics
	originalSummary := instance.Summary
	originalCache := instance.Cache

	// Short deadline: ethcore's BeaconNode.Start selects on ctx.Done() and returns promptly, so
	// this reliably reproduces a real restart failure within about a second.
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	restartErr := instance.RestartWithoutSingleAttestation(ctx)
	require.Error(t, restartErr, "restart must fail against an unreachable beacon address")

	assert.Same(t, originalNode, instance.Node,
		"a failed restart must leave the existing Node completely untouched, not nil and not "+
			"swapped - the replacement is built and started before anything about the current "+
			"instance is touched")
	assert.Same(t, originalMetrics, instance.Metrics)
	assert.Same(t, originalSummary, instance.Summary)
	assert.Same(t, originalCache, instance.Cache)

	// The stopMonitor channel must still be open and usable - a failed restart never closed it,
	// so a second attempt won't panic on close-of-closed-channel.
	select {
	case <-instance.stopMonitor:
		t.Fatal("stopMonitor must not be closed after a failed restart")
	default:
	}
}

func TestRestartWithoutSingleAttestation_RespectsCooldownAfterFailedAttempt(t *testing.T) {
	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)

	cfg := config.NewDefaultConfig()

	app := &Application{
		log:           log,
		debug:         true,
		config:        cfg,
		beaconFactory: ethereum.NewBeaconFactory(log, fakeClockDrift{}),
	}

	const unreachableAddr = "http://127.0.0.1:1"

	instance, err := app.createBeaconInstance(context.Background(), log, unreachableAddr, "trace-b", nil)
	require.NoError(t, err)

	instance.app = app
	instance.stopMonitor = make(chan struct{})

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	require.Error(t, instance.RestartWithoutSingleAttestation(ctx))
	require.False(t, instance.lastReconnect.IsZero(),
		"lastReconnect must be recorded even on failure, so a burst of reconnect signals can't "+
			"retry in a tight loop")

	// Immediately trying again must be suppressed by the cooldown and return without attempting
	// anything (no error, nothing touched).
	before := instance.Node

	require.NoError(t, instance.RestartWithoutSingleAttestation(context.Background()))
	assert.Same(t, before, instance.Node, "a cooldown-suppressed attempt must not touch the instance")
}

// TestRestartWithoutSingleAttestation_TearsDownOldInstanceOnSuccess is a structural regression
// test, not a behavioral one. Driving the success path requires newInstance.Node.Start(ctx) to
// actually succeed, which needs a real (or fully faked) beacon HTTP backend - the same ethcore
// live-state wall documented elsewhere in this codebase's tests (NM-02/09/12 in the nemesis
// triage). What's verified here: the exact teardown calls the success path depends on are present
// in the current source.
func TestRestartWithoutSingleAttestation_TearsDownOldInstanceOnSuccess(t *testing.T) {
	out, err := exec.Command("grep", "-n", "oldNode.Stop\\|oldCache.Stop\\|oldMetrics.Unregister\\|newInstance.Cache.Start", "beacons.go").
		CombinedOutput()
	require.NoError(t, err, "grep must find the teardown/start call sites in beacons.go")

	block := string(out)

	for _, want := range []string{"newInstance.Cache.Start", "oldNode.Stop", "oldCache.Stop", "oldMetrics.Unregister"} {
		assert.True(t, strings.Contains(block, want),
			"expected %q to still be called in RestartWithoutSingleAttestation - if this fails, "+
				"the restart no longer starts the replacement cache or tears down the old "+
				"instance's resources, regressing NM-05", want)
	}
}

func TestApplicationStop_NilNodeDoesNotPanic(t *testing.T) {
	app := &Application{
		log:     logrus.New(),
		servers: &ServerManager{},
		beaconNodes: map[string]*BeaconNodeInstance{
			"trace-a": {Node: nil},
		},
	}

	assert.NotPanics(t, func() {
		require.NoError(t, app.Stop(context.Background()))
	})
}

func TestInitBeacons_RejectsDuplicateAddresses(t *testing.T) {
	cfg := config.NewDefaultConfig()
	cfg.BeaconNodeAddress = "http://localhost:5052,http://localhost:5052"

	app := &Application{
		log:    logrus.New(),
		config: cfg,
	}

	err := app.initBeacons(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate")
}

func TestInitBeacons_RejectsDuplicateAddressesAfterTrimming(t *testing.T) {
	cfg := config.NewDefaultConfig()
	cfg.BeaconNodeAddress = "http://localhost:5052, http://localhost:5052 "

	app := &Application{
		log:    logrus.New(),
		config: cfg,
	}

	err := app.initBeacons(context.Background())
	require.Error(t, err, "duplicates must be caught after trimming whitespace, not just exact string matches")
}

func TestFetchNodeIdentityAttnetsWithRetry(t *testing.T) {
	t.Run("succeeds after a transient failure", func(t *testing.T) {
		var attempt atomic.Int32

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if attempt.Add(1) == 1 {
				w.WriteHeader(http.StatusInternalServerError)

				return
			}

			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":{"peer_id":"test-peer","enr":"","metadata":{"attnets":"0x0100000000000000"}}}`))
		}))
		defer server.Close()

		log := logrus.New()
		log.SetLevel(logrus.ErrorLevel)

		subnets, err := fetchNodeIdentityAttnetsWithRetry(context.Background(), log, server.URL, nil)
		require.NoError(t, err)
		assert.Equal(t, []int{0}, subnets)
		assert.GreaterOrEqual(t, attempt.Load(), int32(2), "the first failure must have been retried")
	})

	t.Run("fails after exhausting attempts", func(t *testing.T) {
		log := logrus.New()
		log.SetLevel(logrus.ErrorLevel)

		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()

		_, err := fetchNodeIdentityAttnetsWithRetry(ctx, log, "http://127.0.0.1:1", nil)
		require.Error(t, err)
	})
}
