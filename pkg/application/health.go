package application

import (
	"encoding/json"
	"net/http"

	"github.com/ethpandaops/contributoor/pkg/ethereum"
)

// handleHealthCheck handles the /healthz endpoint.
// Returns 200 OK if at least one beacon node is healthy, 503 otherwise.
// Response is in JSON format including network information.
func (a *Application) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	status := a.GetHealthStatus()

	w.Header().Set("Content-Type", "application/json")

	if status.Healthy {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	if err := json.NewEncoder(w).Encode(status); err != nil {
		a.log.WithError(err).Error("Failed to encode health status")
	}
}

// HealthStatus represents the health status of the application.
type HealthStatus struct {
	Healthy     bool                    `json:"healthy"`
	BeaconNodes map[string]BeaconHealth `json:"beacon_nodes"` //nolint:tagliatelle // upstream definition.
}

// BeaconHealth represents the health status of a single beacon node.
type BeaconHealth struct {
	Address string `json:"address"`
	Network string `json:"network,omitempty"`
	ChainID uint64 `json:"chain_id,omitempty"` //nolint:tagliatelle // standard naming
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
			// Check if node is healthy (has metadata)
			if node.IsHealthy() {
				status.Healthy = true
			}

			// Get network information from the metadata
			if metadata := node.Metadata(); metadata != nil {
				if network := metadata.GetNetwork(); network != nil && network.Name != "none" {
					beaconHealth.Network = string(network.Name)
					beaconHealth.ChainID = network.ID
				}
			}
		}

		status.BeaconNodes[traceID] = beaconHealth
	}

	return status
}
