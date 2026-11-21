package events

import "github.com/prometheus/client_golang/prometheus"

// Metrics is the metrics for the events package.
type Metrics struct {
	decoratedEventTotal *prometheus.CounterVec
}

// NewMetrics creates a new Metrics instance.
func NewMetrics(namespace string) *Metrics {
	m := &Metrics{
		decoratedEventTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "decorated_event_total",
			Help:      "Total number of decorated events received",
		}, []string{"type", "network_id"}),
	}

	prometheus.MustRegister(m.decoratedEventTotal)

	return m
}

// AddDecoratedEvent adds a decorated event to the metrics.
func (m *Metrics) AddDecoratedEvent(count int, eventType, networkID string) {
	m.decoratedEventTotal.WithLabelValues(eventType, networkID).Add(float64(count))
}

// Unregister removes this instance's collector from the default Prometheus registry. Call this
// when replacing a Metrics instance (e.g. on beacon restart) so the old, now-dead series doesn't
// stay registered and flat-line forever under its old namespace.
func (m *Metrics) Unregister() {
	prometheus.Unregister(m.decoratedEventTotal)
}
