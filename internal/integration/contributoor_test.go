package integration

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/ethpandaops/contributoor/pkg/config/v1"
	"github.com/ethpandaops/ethereum-package-go"
	"github.com/ethpandaops/ethereum-package-go/pkg/client"
	ethconfig "github.com/ethpandaops/ethereum-package-go/pkg/config"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

// TestContributoor_AllClients tests Contributoor against all consensus clients simultaneously.
func TestContributoor_AllClients(t *testing.T) {
	// Skip if not running integration tests
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()

	// Start a single network with ALL consensus clients
	t.Log("Starting test network with all consensus clients...")

	// Create participants with validators
	// Need ~1400 validators per node (8400 total) for 2+ committees per slot. It's still not enough to completely
	// test subnet mismatch functionality, but anymore is impractical.
	participants := []ethconfig.ParticipantConfig{
		{ELType: client.Geth, CLType: client.Lighthouse, Count: 1, ValidatorCount: 1400},
		{ELType: client.Geth, CLType: client.Teku, Count: 1, ValidatorCount: 1400},
		{ELType: client.Geth, CLType: client.Prysm, Count: 1, ValidatorCount: 1400},
		{ELType: client.Geth, CLType: client.Nimbus, Count: 1, ValidatorCount: 1400},
		{ELType: client.Geth, CLType: client.Lodestar, Count: 1, ValidatorCount: 1400},
		{ELType: client.Geth, CLType: client.Grandine, Count: 1, ValidatorCount: 1400},
	}

	net, err := ethereum.Run(
		ctx,
		ethereum.WithParticipants(participants),
		ethereum.WithChainID(12345),
		ethereum.WithWaitForGenesis(),
		ethereum.WithDockerCacheParams(true, "docker.ethquokkaops.io"),
	)
	require.NoError(t, err, "Failed to start test network")

	// Cleanup network when done
	defer func() {
		t.Log("Cleaning up test network...")

		if cerr := net.Cleanup(ctx); cerr != nil {
			t.Logf("Warning: Failed to cleanup network: %v", cerr)
		}
	}()

	// Get all consensus clients
	consensusClients := net.ConsensusClients()
	allCLs := consensusClients.All()
	require.NotEmpty(t, allCLs, "No consensus clients found in test network")

	// Collect all beacon node URLs
	beaconURLs := make([]string, 0)
	clientTypes := make(map[string]string) // URL -> client type mapping

	for _, cl := range allCLs {
		beaconURL := cl.BeaconAPIURL()
		beaconURLs = append(beaconURLs, beaconURL)
		clientTypes[beaconURL] = string(cl.Type())

		t.Logf("Found consensus client: %s (type: %s), beacon: %s",
			cl.Name(), cl.Type(), beaconURL)
	}

	// Join all beacon URLs with commas
	allBeaconAddresses := strings.Join(beaconURLs, ",")
	t.Logf("Combined beacon node addresses: %s", allBeaconAddresses)

	// Create contributoor config with ALL beacon nodes
	cfg := config.NewDefaultConfig()
	cfg.BeaconNodeAddress = allBeaconAddresses // Multiple addresses!
	cfg.NetworkName = "testnet"
	cfg.LogLevel = "debug"
	cfg.MetricsAddress = ":19090"
	cfg.HealthCheckAddress = ":19091"
	cfg.ContributoorDirectory = t.TempDir()
	cfg.RunMethod = config.RunMethod_RUN_METHOD_BINARY
	cfg.LogLevel = logrus.DebugLevel.String()
	cfg.AttestationSubnetCheck = &config.AttestationSubnetCheck{
		Enabled:    true,
		MaxSubnets: 2,
	}

	// Start Contributoor connected to ALL beacon nodes
	contributoor, contribCleanup := StartContributoor(t, ctx, cfg, true)
	defer contribCleanup()

	// Wait for events to be collected from all clients
	t.Log("Waiting for beacon chain events from all consensus clients...")
	time.Sleep(45 * time.Second) // Give more time for all clients to produce events

	// Query metrics
	resp, err := http.Get("http://localhost:19090/metrics")
	require.NoError(t, err, "Failed to query metrics")

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "Failed to read metrics response")

	metrics := string(body)

	// Verify we have beacon nodes for each client
	beaconNodes := contributoor.BeaconNodes()
	t.Logf("Contributoor connected to %d beacon nodes", len(beaconNodes))
	require.Equal(t, len(allCLs), len(beaconNodes),
		"Expected %d beacon nodes but got %d", len(allCLs), len(beaconNodes))

	// Check that we got events from multiple beacon nodes
	// Look for trace IDs in metrics
	traceIDsWithEvents := make(map[string]int)

	lines := strings.Split(metrics, "\n")
	for _, line := range lines {
		// Look for decorated event metrics with trace IDs
		if strings.Contains(line, "decorated_event_total") && !strings.HasPrefix(line, "#") && strings.Contains(line, "} ") {
			// Extract trace ID from metric name
			// Format: contributoor_<traceID>_decorated_event_total{...} <count>
			if strings.HasPrefix(line, "contributoor_") {
				parts := strings.Split(line, "_")
				if len(parts) >= 3 {
					traceID := parts[1]

					// Get count
					fields := strings.Fields(line)
					if len(fields) >= 2 {
						count := fields[len(fields)-1]
						if count != "0" {
							// Remove trailing newline if present
							count = strings.TrimSuffix(count, "\n")
							countInt := 0

							if _, serr := fmt.Sscanf(count, "%d", &countInt); serr == nil && countInt > 0 {
								traceIDsWithEvents[traceID] += countInt
							}
						}
					}
				}
			}
		}
	}

	t.Logf("Found events from %d beacon nodes", len(traceIDsWithEvents))

	for traceID, count := range traceIDsWithEvents {
		t.Logf("Trace ID %s: %d events", traceID, count)
	}

	// We should have events from multiple beacon nodes
	require.GreaterOrEqual(t, len(traceIDsWithEvents), 2,
		"Expected events from at least 2 beacon nodes, got %d", len(traceIDsWithEvents))

	// Check health endpoint
	healthResp, err := http.Get("http://localhost:19091/healthz")
	require.NoError(t, err, "Failed to query health endpoint")

	defer healthResp.Body.Close()

	require.Equal(t, http.StatusOK, healthResp.StatusCode, "Expected healthy status")

	healthBody, err := io.ReadAll(healthResp.Body)
	require.NoError(t, err, "Failed to read health response")

	healthStr := string(healthBody)
	t.Logf("Health check response: %s\n", healthStr)

	// Health endpoint returns OK if at least one beacon is healthy
	require.Contains(t, healthStr, "OK", "Expected OK health status")
	require.Contains(t, healthStr, "is healthy", "Expected healthy beacon message")

	// Verify all beacon nodes are actually healthy via GetHealthStatus
	healthStatus := contributoor.GetHealthStatus()
	require.True(t, healthStatus.Healthy, "Expected overall healthy status")
	require.Len(t, healthStatus.BeaconNodes, len(beaconNodes),
		"Expected health status for all %d beacon nodes", len(beaconNodes))

	// Count healthy nodes
	healthyCount := 0

	for traceID, health := range healthStatus.BeaconNodes {
		t.Logf("Beacon %s: connected=%v, healthy=%v, address=%s",
			traceID, health.Connected, health.Healthy, health.Address)

		if health.Healthy {
			healthyCount++
		}
	}

	require.GreaterOrEqual(t, healthyCount, len(beaconNodes)-1,
		"Expected at least %d healthy beacon nodes", len(beaconNodes)-1)

	t.Log("Multi-client integration test completed successfully!")
	t.Logf("Successfully collected events from %d different beacon nodes", len(traceIDsWithEvents))
}
