package ethereum

import (
	"errors"
)

const (
	// Max number of subnets acceptable to monitor.
	defaultMaxSubnets = 2
	// Number of slots to track to determine a mismatch.
	defaultMismatchDetectionWindow = 2
	// Number of times a detection window mismatch needs to trigger before reconnection.
	defaultMismatchThreshold = 1
	// Number of seconds post reconnection before another reconnection can potentially occur.
	defaultMismatchCooldownSeconds = 300 // 5 minutes.
	// Number of additional events on subnets other than the advetised allowed before mismatch is triggered.
	subnetHighWaterMark = 5
)

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
}

// SubnetConfig controls subnet-based subscription filtering and mismatch detection.
type SubnetConfig struct {
	// Enabled controls whether to check subnet participation at startup.
	Enabled bool `yaml:"enabled"`
	// MaxSubnets is the maximum number of subnets the node participates in.
	MaxSubnets int `yaml:"maxSubnets"`
	// MismatchDetectionWindow is the number of slots to track for subnet activity.
	MismatchDetectionWindow int `yaml:"mismatchDetectionWindow"`
	// MismatchThreshold is the number of mismatches required before triggering reconnection.
	MismatchThreshold int `yaml:"mismatchThreshold"`
	// MismatchCooldownSeconds is the cooldown period between reconnections in seconds.
	MismatchCooldownSeconds int `yaml:"mismatchCooldownSeconds"`
	// SubnetHighWaterMark allows temporary participation in additional subnets
	// without triggering a restart. This accommodates validators temporarily
	// joining subnets to submit attestations while protecting privacy.
	SubnetHighWaterMark int `yaml:"subnetHighWaterMark"`
}

// NewDefaultConfig returns a new config with default values.
func NewDefaultConfig() *Config {
	return &Config{
		AttestationSubnetConfig: SubnetConfig{
			Enabled:                 false,
			MaxSubnets:              defaultMaxSubnets,
			MismatchDetectionWindow: defaultMismatchDetectionWindow,
			MismatchThreshold:       defaultMismatchThreshold,
			MismatchCooldownSeconds: defaultMismatchCooldownSeconds,
			SubnetHighWaterMark:     subnetHighWaterMark,
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

	return nil
}
