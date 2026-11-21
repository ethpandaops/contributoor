package application

import (
	"fmt"
	"net/http"

	"github.com/ethpandaops/contributoor/pkg/ethereum"
)

// handleHealthCheck handles the /healthz endpoint.
// Returns 200 OK if at least one beacon node is healthy, 503 otherwise.
//
// This reflects beacon connectivity only. It does not know whether events are actually reaching
// the output server - a sink can be failing to deliver while the beacon connection itself stays
// healthy. See GetHealthStatus for per-beacon failed event counts if that visibility is needed.
func (a *Application) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	// Check if at least one beacon is healthy
	for traceID, instance := range a.beaconNodes {
		if node, ok := instance.Node.(*ethereum.BeaconWrapper); ok && node.IsHealthy() {
			w.WriteHeader(http.StatusOK)

			fmt.Fprintf(w, "OK - beacon %s is healthy", traceID)

			return
		}
	}

	// No healthy beacons found
	w.WriteHeader(http.StatusServiceUnavailable)
	fmt.Fprint(w, "No healthy beacons")
}

// HealthStatus represents the health status of the application.
type HealthStatus struct {
	Healthy     bool                    `json:"healthy"`
	BeaconNodes map[string]BeaconHealth `json:"beacon_nodes"` //nolint:tagliatelle // upstream definition.
}

// BeaconHealth represents the health status of a single beacon node.
type BeaconHealth struct {
	Connected bool   `json:"connected"`
	Healthy   bool   `json:"healthy"`
	Address   string `json:"address"`
	// FailedEvents is the number of events that failed to reach a sink during the current summary
	// window (the same window Summary logs and resets on, by default every 10 seconds) - it is not
	// a lifetime total. A nonzero value means something is wrong right now; it is not suitable for
	// alerting math without accounting for the reset.
	FailedEvents uint64 `json:"failed_events"` //nolint:tagliatelle // matches beacon_nodes' snake_case convention above.
}

// GetHealthStatus returns detailed health information about the application.
func (a *Application) GetHealthStatus() HealthStatus {
	status := HealthStatus{
		Healthy:     false,
		BeaconNodes: make(map[string]BeaconHealth),
	}

	for traceID, instance := range a.beaconNodes {
		beaconHealth := BeaconHealth{
			Address: instance.Address,
		}

		if node, ok := instance.Node.(*ethereum.BeaconWrapper); ok {
			beaconHealth.Connected = true
			beaconHealth.Healthy = node.IsHealthy()

			if beaconHealth.Healthy {
				status.Healthy = true
			}
		}

		if instance.Summary != nil {
			beaconHealth.FailedEvents = instance.Summary.GetFailedEvents()
		}

		status.BeaconNodes[traceID] = beaconHealth
	}

	return status
}
