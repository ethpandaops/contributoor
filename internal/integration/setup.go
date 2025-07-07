package integration

import (
	"context"
	"testing"
	"time"

	"github.com/ethpandaops/contributoor/pkg/application"
	"github.com/ethpandaops/contributoor/pkg/config/v1"
	"github.com/ethpandaops/ethereum-package-go"
	"github.com/ethpandaops/ethereum-package-go/pkg/network"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

// StartTestNetwork creates a test Ethereum network using ethereum-package-go.
// Returns the network instance, beacon node URL, and cleanup function.
func StartTestNetwork(t *testing.T, ctx context.Context) (net network.Network, beaconURL string, cleanup func()) {
	t.Helper()

	// Start a minimal network.
	t.Log("Starting test network with ethereum-package-go...")

	net, err := ethereum.Run(
		ctx,
		ethereum.Minimal(),
		ethereum.WithChainID(12345),
		ethereum.WithWaitForGenesis(), // Wait for genesis before returning
		ethereum.WithDockerCacheParams(true, "docker.ethquokkaops.io"),
	)
	require.NoError(t, err, "Failed to start test network")

	// Get beacon node URL from the consensus client
	consensusClients := net.ConsensusClients()
	allCLs := consensusClients.All()
	require.NotEmpty(t, allCLs, "No consensus clients found in test network")

	// Use the first consensus client's beacon URL
	beaconURL = allCLs[0].BeaconAPIURL()
	require.NotEmpty(t, beaconURL, "Beacon API URL is empty")

	t.Logf("Test network started successfully")
	t.Logf("Network name: %s", net.Name())
	t.Logf("Chain ID: %d", net.ChainID())
	t.Logf("Beacon URL: %s", beaconURL)

	// Log all clients for debugging
	for _, cl := range allCLs {
		t.Logf(
			"Consensus client: %s (type: %s), beacon: %s",
			cl.Name(), cl.Type(), cl.BeaconAPIURL(),
		)
	}

	execClients := net.ExecutionClients()
	for _, el := range execClients.All() {
		t.Logf(
			"Execution client: %s (type: %s), RPC: %s",
			el.Name(), el.Type(), el.RPCURL(),
		)
	}

	cleanup = func() {
		t.Log("Cleaning up test network...")

		if cerr := net.Cleanup(ctx); err != nil {
			t.Logf("Warning: Failed to cleanup network: %v", cerr)
		}
	}

	return net, beaconURL, cleanup
}

// StartContributoor starts a Contributoor instance with the given configuration.
// Returns the contributoor instance and a cleanup function.
func StartContributoor(t *testing.T, ctx context.Context, cfg *config.Config, debugMode bool) (*application.Application, func()) {
	t.Helper()

	t.Log("Starting Contributoor with test configuration...")

	// Create logger
	log := logrus.New()

	if cfg.LogLevel != "" {
		level, err := logrus.ParseLevel(cfg.LogLevel)
		if err == nil {
			log.SetLevel(level)
		} else {
			log.SetLevel(logrus.DebugLevel)
		}
	}

	// Create application using the public API
	app, err := application.New(application.Options{
		Config: cfg,
		Logger: log.WithField("module", "contributoor"),
		Debug:  debugMode,
	})
	require.NoError(t, err, "Failed to create application")

	// Start the application
	err = app.Start(ctx)
	require.NoError(t, err, "Failed to start application")

	// Create cleanup function
	cleanup := func() {
		t.Log("Cleaning up contributoor...")

		stopCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)

		defer cancel()

		if err := app.Stop(stopCtx); err != nil {
			t.Logf("Failed to stop application: %v", err)
		}
	}

	return app, cleanup
}
