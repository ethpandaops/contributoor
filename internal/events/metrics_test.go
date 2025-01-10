package events

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

func TestNewMetrics(t *testing.T) {
	prometheus.DefaultRegisterer = prometheus.NewRegistry()

	metrics := NewMetrics("test1")
	assert.NotNil(t, metrics)
	assert.NotNil(t, metrics.decoratedEventTotal)
}

func TestMetrics_AddDecoratedEvent(t *testing.T) {
	prometheus.DefaultRegisterer = prometheus.NewRegistry()

	metrics := NewMetrics("test2")
	metrics.AddDecoratedEvent(1, "event_type_1", "network_1")
	metrics.AddDecoratedEvent(2, "event_type_1", "network_1")
	metrics.AddDecoratedEvent(3, "event_type_2", "network_2")

	expected := `# HELP test2_decorated_event_total Total number of decorated events received
# TYPE test2_decorated_event_total counter
test2_decorated_event_total{network_id="network_1",type="event_type_1"} 3
test2_decorated_event_total{network_id="network_2",type="event_type_2"} 3
`

	assert.NoError(t, testutil.CollectAndCompare(
		metrics.decoratedEventTotal,
		strings.NewReader(expected),
	))
}

func TestMetrics_AddDecoratedEvent_MultipleNetworks(t *testing.T) {
	prometheus.DefaultRegisterer = prometheus.NewRegistry()

	metrics := NewMetrics("test3")
	metrics.AddDecoratedEvent(1, "event_type_1", "network_1")
	metrics.AddDecoratedEvent(2, "event_type_1", "network_2")
	metrics.AddDecoratedEvent(3, "event_type_1", "network_3")

	expected := `# HELP test3_decorated_event_total Total number of decorated events received
# TYPE test3_decorated_event_total counter
test3_decorated_event_total{network_id="network_1",type="event_type_1"} 1
test3_decorated_event_total{network_id="network_2",type="event_type_1"} 2
test3_decorated_event_total{network_id="network_3",type="event_type_1"} 3
`

	assert.NoError(t, testutil.CollectAndCompare(
		metrics.decoratedEventTotal,
		strings.NewReader(expected),
	))
}

func TestMetrics_AddDecoratedEvent_MultipleTypes(t *testing.T) {
	prometheus.DefaultRegisterer = prometheus.NewRegistry()

	metrics := NewMetrics("test4")
	metrics.AddDecoratedEvent(1, "event_type_1", "network_1")
	metrics.AddDecoratedEvent(1, "event_type_2", "network_1")
	metrics.AddDecoratedEvent(1, "event_type_3", "network_1")

	expected := `# HELP test4_decorated_event_total Total number of decorated events received
# TYPE test4_decorated_event_total counter
test4_decorated_event_total{network_id="network_1",type="event_type_1"} 1
test4_decorated_event_total{network_id="network_1",type="event_type_2"} 1
test4_decorated_event_total{network_id="network_1",type="event_type_3"} 1
`

	assert.NoError(t, testutil.CollectAndCompare(
		metrics.decoratedEventTotal,
		strings.NewReader(expected),
	))
}
