package application

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ethpandaops/contributoor/internal/events"
	"github.com/ethpandaops/contributoor/pkg/ethereum"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// traceIDSeq keeps each call to newTestBeaconWrapper on a unique Prometheus metrics namespace -
// the underlying beacon library registers metrics on construction, and the default registry
// panics on a duplicate registration.
var traceIDSeq atomic.Uint64

// newTestBeaconWrapper builds a real *ethereum.BeaconWrapper via the same production factory
// beacons.go uses, without any network I/O (CreateBeacon only constructs objects; it does not
// connect). A freshly constructed wrapper has never been healthy, matching the state a beacon
// starts in before its first successful connection.
func newTestBeaconWrapper(t *testing.T) *ethereum.BeaconWrapper {
	t.Helper()

	log := logrus.New()
	log.SetLevel(logrus.ErrorLevel)

	factory := ethereum.NewBeaconFactory(log, nil)

	topicManager := ethereum.NewTopicManager(log, &ethereum.TopicConfig{
		AllTopics:   ethereum.GetDefaultAllTopics(),
		OptInTopics: ethereum.GetOptInTopics(),
	})

	wrapper, err := factory.CreateBeacon(context.Background(), &ethereum.BeaconOptions{
		TraceID:      fmt.Sprintf("test-trace-%d", traceIDSeq.Add(1)),
		Config:       ethereum.NewDefaultConfig(),
		TopicManager: topicManager,
	})
	require.NoError(t, err)

	return wrapper
}

func TestHandleHealthCheck(t *testing.T) {
	t.Run("no beacon nodes returns 503", func(t *testing.T) {
		app := &Application{beaconNodes: map[string]*BeaconNodeInstance{}}

		rec := httptest.NewRecorder()
		app.handleHealthCheck(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	})

	t.Run("node not a BeaconWrapper returns 503", func(t *testing.T) {
		app := &Application{beaconNodes: map[string]*BeaconNodeInstance{
			"trace-a": {Node: nil},
		}}

		rec := httptest.NewRecorder()
		app.handleHealthCheck(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	})

	t.Run("freshly created wrapper has never been healthy, returns 503", func(t *testing.T) {
		wrapper := newTestBeaconWrapper(t)

		app := &Application{beaconNodes: map[string]*BeaconNodeInstance{
			"trace-a": {Node: wrapper},
		}}

		rec := httptest.NewRecorder()
		app.handleHealthCheck(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	})

	t.Run("healthy wrapper returns 200", func(t *testing.T) {
		wrapper := newTestBeaconWrapper(t)

		// IsHealthy is now the wrapper's own live-tracked field (fixed in this change); flipping
		// it directly here stands in for what a real connection-succeeded event does.
		ethereum.SetHealthyForTesting(wrapper, true)

		app := &Application{beaconNodes: map[string]*BeaconNodeInstance{
			"trace-a": {Node: wrapper},
		}}

		rec := httptest.NewRecorder()
		app.handleHealthCheck(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))

		assert.Equal(t, http.StatusOK, rec.Code)
	})
}

func TestGetHealthStatus(t *testing.T) {
	t.Run("surfaces failed events from the instance summary", func(t *testing.T) {
		summary := events.NewSummary(logrus.New(), "trace-a", time.Hour)
		summary.AddFailedEvents(3)

		app := &Application{beaconNodes: map[string]*BeaconNodeInstance{
			"trace-a": {Address: "http://localhost:5052", Summary: summary},
		}}

		status := app.GetHealthStatus()

		require.Contains(t, status.BeaconNodes, "trace-a")
		assert.Equal(t, uint64(3), status.BeaconNodes["trace-a"].FailedEvents)
	})

	t.Run("nil summary does not panic and reports zero", func(t *testing.T) {
		app := &Application{beaconNodes: map[string]*BeaconNodeInstance{
			"trace-a": {Address: "http://localhost:5052", Summary: nil},
		}}

		status := app.GetHealthStatus()

		require.Contains(t, status.BeaconNodes, "trace-a")
		assert.Equal(t, uint64(0), status.BeaconNodes["trace-a"].FailedEvents)
	})
}
