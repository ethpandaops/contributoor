package application

import "errors"

// Common errors returned by the application package.
var (
	// ErrConfigRequired is returned when no configuration is provided.
	ErrConfigRequired = errors.New("configuration is required")

	// ErrNoBeaconNodes is returned when no beacon nodes are configured.
	ErrNoBeaconNodes = errors.New("no beacon nodes configured")

	// ErrAllBeaconsFailed is returned when all beacon nodes fail to connect.
	ErrAllBeaconsFailed = errors.New("all beacon nodes failed to connect")
)
