package ethereum

import (
	"errors"

	configv1 "github.com/ethpandaops/contributoor/pkg/config/v1"
)

const defaultMaxSubnets = 2

// Config defines the configuration for the Ethereum beacon node.
type Config struct {
	// The address of the Beacon node to connect to.
	BeaconNodeAddress string `yaml:"beaconNodeAddress"`
	// BeaconNodeHeaders is a map of headers to send to the beacon node.
	BeaconNodeHeaders map[string]string `yaml:"beaconNodeHeaders"`
	// NetworkOverride is an optional network name to use instead of what's reported by the beacon node.
	NetworkOverride string `yaml:"networkOverride,omitempty"`
	// AttestationSubnetConfig controls attestation subnet-based subscription filtering.
	AttestationSubnetConfig SubnetConfig `yaml:"attestationSubnet"`
	// SubnetMismatchDetection controls subnet mismatch detection and reconnection.
	SubnetMismatchDetection *configv1.SubnetMismatchDetection `yaml:"subnetMismatchDetection"`
}

// SubnetConfig controls subnet-based subscription filtering.
type SubnetConfig struct {
	// Enabled controls whether to check subnet participation at startup.
	Enabled bool `yaml:"enabled"`
	// MaxSubnets is the maximum number of subnets the node participates in.
	MaxSubnets int `yaml:"maxSubnets"`
}

// NewDefaultConfig returns a new config with default values.
func NewDefaultConfig() *Config {
	return &Config{
		AttestationSubnetConfig: SubnetConfig{
			Enabled:    false,
			MaxSubnets: defaultMaxSubnets,
		},
		SubnetMismatchDetection: &configv1.SubnetMismatchDetection{
			Enabled:           false,
			DetectionWindow:   32,
			MismatchThreshold: 3,
			CooldownSeconds:   300,
		},
	}
}

// Validate checks the configuration for the beacon node.
func (c *Config) Validate() error {
	if c.BeaconNodeAddress == "" {
		return errors.New("beaconNodeAddress is required")
	}

	if c.AttestationSubnetConfig.Enabled {
		if c.AttestationSubnetConfig.MaxSubnets < 0 || c.AttestationSubnetConfig.MaxSubnets > 64 {
			return errors.New("attestationSubnet.maxSubnets must be between 0 and 64 (inclusive)")
		}
	}

	if c.SubnetMismatchDetection != nil && c.SubnetMismatchDetection.Enabled {
		if c.SubnetMismatchDetection.DetectionWindow < 1 || c.SubnetMismatchDetection.DetectionWindow > 64 {
			return errors.New("subnetMismatchDetection.detectionWindow must be between 1 and 64 (inclusive)")
		}

		if c.SubnetMismatchDetection.MismatchThreshold < 1 {
			return errors.New("subnetMismatchDetection.mismatchThreshold must be at least 1")
		}

		if c.SubnetMismatchDetection.CooldownSeconds < 1 {
			return errors.New("subnetMismatchDetection.cooldownSeconds must be at least 1")
		}
	}

	return nil
}
