package application

import (
	"fmt"
	"net/http"

	"github.com/ethpandaops/contributoor/pkg/ethereum"
)

// handleHealthCheck handles the /healthz endpoint.
// Returns 200 OK if at least one beacon node is healthy, 503 otherwise.
func (a *Application) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	// Check if at least one beacon is healthy
	for traceID, instance := range a.beaconNodes {
		if node, ok := instance.Node.(*ethereum.BeaconNode); ok && node.IsHealthy() {
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

		if node, ok := instance.Node.(*ethereum.BeaconNode); ok {
			beaconHealth.Connected = true
			beaconHealth.Healthy = node.IsHealthy()

			if beaconHealth.Healthy {
				status.Healthy = true
			}
		}

		status.BeaconNodes[traceID] = beaconHealth
	}

	return status
}
