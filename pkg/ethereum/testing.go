package ethereum

// SetHealthyForTesting sets a BeaconWrapper's live connection health state directly, standing in
// for the real connection-succeeded/connection-lost events that drive it in production. Intended
// for use by other packages' tests that need a BeaconWrapper in a known health state without
// connecting to a real beacon node.
func SetHealthyForTesting(w *BeaconWrapper, healthy bool) {
	w.isHealthy.Store(healthy)
}
