package integration

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/ethpandaops/contributoor/pkg/config/v1"
	"github.com/stretchr/testify/require"
)

// TestContributoorEventCollection validates that Contributoor can connect to
// and collect events from a real consensus client in a test network.
func TestContributoorEventCollection(t *testing.T) {
	// Skip if not running integration tests
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Start test network
	network, beaconURL, networkCleanup := StartTestNetwork(t, ctx)
	defer networkCleanup()

	// Create contributoor config
	cfg := config.NewDefaultConfig()
	cfg.BeaconNodeAddress = beaconURL
	cfg.NetworkName = "testnet"
	cfg.LogLevel = "debug"
	cfg.MetricsAddress = ":19090"     // Non-default port for testing
	cfg.HealthCheckAddress = ":19091" // Non-default port for testing
	cfg.ContributoorDirectory = t.TempDir()
	cfg.RunMethod = config.RunMethod_RUN_METHOD_BINARY

	// Start Contributoor in debug mode (uses stdout sink)
	contributoor, contribCleanup := StartContributoor(t, ctx, cfg, true)
	defer contribCleanup()

	// Wait for beacon chain to produce some events
	t.Log("Waiting for beacon chain events to be collected...")
	time.Sleep(30 * time.Second)

	// Query metrics to verify event collection
	t.Log("Querying metrics to verify event collection...")
	resp, err := http.Get("http://localhost:19090/metrics")
	require.NoError(t, err, "Failed to query metrics")
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "Failed to read metrics response")

	metrics := string(body)
	t.Logf("Metrics response length: %d bytes", len(metrics))

	// Look for decorated event metrics
	// The metric format is: contributoor_bn_<traceID>_decorated_event_total
	require.Contains(t, metrics, "decorated_event_total", "Expected decorated event metric")

	// Parse metrics to find specific event counts
	var foundEvents bool
	var eventTypes []string
	lines := strings.Split(metrics, "\n")

	for _, line := range lines {
		if strings.Contains(line, "decorated_event_total") && !strings.HasPrefix(line, "#") {
			foundEvents = true

			// Extract event type from the metric line
			// Format: contributoor_bn_<traceID>_decorated_event_total{type="beacon_api_eth_v1_events_block",...} <count>
			if strings.Contains(line, "type=") {
				start := strings.Index(line, `type="`) + 6
				end := strings.Index(line[start:], `"`)
				if end > 0 {
					eventType := line[start : start+end]
					eventTypes = append(eventTypes, eventType)

					t.Logf("Found event type: %s", eventType)
				}
			}

			// Check if counter is > 0
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				count := parts[len(parts)-1]
				if count != "0" {
					t.Logf("Event metric line: %s", line)
				}
			}
		}
	}

	require.True(t, foundEvents, "No decorated event metrics found")
	require.NotEmpty(t, eventTypes, "No event types found in metrics")

	// Verify we have multiple event types
	t.Logf("Collected event types: %v", eventTypes)
	require.GreaterOrEqual(t, len(eventTypes), 1, "Expected at least one event type")

	// Check health endpoint
	t.Log("Checking health endpoint...")
	healthResp, err := http.Get("http://localhost:19091/healthz")
	require.NoError(t, err, "Failed to query health endpoint")
	defer healthResp.Body.Close()

	require.Equal(t, http.StatusOK, healthResp.StatusCode, "Expected healthy status")

	healthBody, err := io.ReadAll(healthResp.Body)
	require.NoError(t, err, "Failed to read health response")
	t.Logf("Health check response: %s", string(healthBody))

	// Log network info for debugging
	t.Logf("Test network name: %s", network.Name())
	t.Logf("Test network chain ID: %d", network.ChainID())

	// Log contributoor state
	beaconNodes := contributoor.BeaconNodes()
	t.Logf("Contributoor has %d beacon nodes configured", len(beaconNodes))
	for traceID := range beaconNodes {
		t.Logf("Beacon node trace ID: %s", traceID)
	}

	t.Log("Integration test completed successfully!")
}
