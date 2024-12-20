package config

// DisplayName returns the display name of the network.
func (n NetworkName) DisplayName() string {
	switch n {
	case NetworkName_NETWORK_NAME_MAINNET:
		return "Mainnet"
	case NetworkName_NETWORK_NAME_SEPOLIA:
		return "Sepolia"
	case NetworkName_NETWORK_NAME_HOLESKY:
		return "Holesky"
	default:
		return "Unknown"
	}
}

// DisplayName returns the display name of the run method.
func (r RunMethod) DisplayName() string {
	switch r {
	case RunMethod_RUN_METHOD_DOCKER:
		return "Docker"
	case RunMethod_RUN_METHOD_SYSTEMD:
		return "Systemd"
	case RunMethod_RUN_METHOD_BINARY:
		return "Binary"
	default:
		return "Unknown"
	}
}
