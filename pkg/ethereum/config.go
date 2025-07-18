package ethereum

import (
	"errors"
)

// Config defines the configuration for the Ethereum beacon node.
type Config struct {
	// The address of the Beacon node to connect to
	BeaconNodeAddress string `yaml:"beaconNodeAddress"`
	// BeaconNodeHeaders is a map of headers to send to the beacon node.
	BeaconNodeHeaders map[string]string `yaml:"beaconNodeHeaders"`
	// NetworkOverride is an optional network name to use instead of what's reported by the beacon node
	NetworkOverride string `yaml:"networkOverride,omitempty"`
	// SubnetCheck controls subnet-based subscription filtering
	SubnetCheck SubnetCheckConfig `yaml:"subnetCheck"`
}

// SubnetCheckConfig controls subnet-based subscription filtering.
type SubnetCheckConfig struct {
	// Enabled controls whether to check subnet participation at startup
	Enabled bool `yaml:"enabled"`
	// MaxSubnets is the maximum number of subnets to allow for single_attestation subscription
	MaxSubnets int `yaml:"maxSubnets"`
}

// NewDefaultConfig returns a new config with default values.
func NewDefaultConfig() *Config {
	return &Config{
		SubnetCheck: SubnetCheckConfig{
			Enabled:    true,
			MaxSubnets: 2,
		},
	}
}

// Validate checks the configuration for the beacon node.
func (c *Config) Validate() error {
	if c.BeaconNodeAddress == "" {
		return errors.New("beaconNodeAddress is required")
	}

	if c.SubnetCheck.Enabled && c.SubnetCheck.MaxSubnets < 0 {
		return errors.New("subnetCheck.maxSubnets must be >= 0 when enabled")
	}

	return nil
}
