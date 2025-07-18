package main

import (
	"encoding/base64"
	"fmt"
	"os"
	"strconv"

	"github.com/ethpandaops/contributoor/pkg/config/v1"
	"github.com/pkg/errors"
	"github.com/urfave/cli/v2"
)

// applyConfigOverridesFromFlags applies CLI flags to the config if they are set.
func applyConfigOverridesFromFlags(cfg *config.Config, c *cli.Context) error {
	// Apply environment variables first, then override with CLI flags if set
	if network := os.Getenv("CONTRIBUTOOR_NETWORK"); network != "" {
		log.Infof("Setting network from env to %s", network)

		if err := cfg.SetNetwork(network); err != nil {
			return errors.Wrap(err, "failed to set network from env")
		}
	}

	if c.String("network") != "" {
		log.Infof("Overriding network from CLI to %s", c.String("network"))

		if err := cfg.SetNetwork(c.String("network")); err != nil {
			return errors.Wrap(err, "failed to set network from cli")
		}
	}

	if addr := os.Getenv("CONTRIBUTOOR_BEACON_NODE_ADDRESS"); addr != "" {
		log.Infof("Setting beacon node address from env")
		cfg.SetBeaconNodeAddress(addr)
	}

	if c.String("beacon-node-address") != "" {
		log.Infof("Overriding beacon node address from CLI")
		cfg.SetBeaconNodeAddress(c.String("beacon-node-address"))
	}

	if addr := os.Getenv("CONTRIBUTOOR_METRICS_ADDRESS"); addr != "" {
		log.Infof("Setting metrics address from env to %s", addr)
		cfg.SetMetricsAddress(addr)
	}

	if c.String("metrics-address") != "" {
		log.Infof("Overriding metrics address from CLI to %s", c.String("metrics-address"))
		cfg.SetMetricsAddress(c.String("metrics-address"))
	}

	if addr := os.Getenv("CONTRIBUTOOR_HEALTH_CHECK_ADDRESS"); addr != "" {
		log.Infof("Setting health check address from env to %s", addr)
		cfg.SetHealthCheckAddress(addr)
	}

	if c.String("health-check-address") != "" {
		log.Infof("Overriding health check address from CLI to %s", c.String("health-check-address"))
		cfg.SetHealthCheckAddress(c.String("health-check-address"))
	}

	if level := os.Getenv("CONTRIBUTOOR_LOG_LEVEL"); level != "" {
		log.Infof("Setting log level from env to %s", level)
		cfg.SetLogLevel(level)
	}

	if c.String("log-level") != "" {
		log.Infof("Overriding log level from CLI to %s", c.String("log-level"))
		cfg.SetLogLevel(c.String("log-level"))
	}

	if addr := os.Getenv("CONTRIBUTOOR_OUTPUT_SERVER_ADDRESS"); addr != "" {
		log.Infof("Setting output server address from env")
		cfg.SetOutputServerAddress(addr)
	}

	if c.String("output-server-address") != "" {
		log.Infof("Overriding output server address from CLI")
		cfg.SetOutputServerAddress(c.String("output-server-address"))
	}

	// Handle credentials from env
	var (
		username = os.Getenv("CONTRIBUTOOR_USERNAME")
		password = os.Getenv("CONTRIBUTOOR_PASSWORD")
	)

	if username != "" || password != "" {
		log.Infof("Setting output server credentials from env")
		cfg.SetOutputServerCredentials(
			base64.StdEncoding.EncodeToString(
				[]byte(fmt.Sprintf("%s:%s", username, password)),
			),
		)
	}

	// CLI flags override env vars for credentials
	if c.String("username") != "" || c.String("password") != "" {
		log.Infof("Overriding output server credentials from CLI")
		cfg.SetOutputServerCredentials(
			base64.StdEncoding.EncodeToString(
				[]byte(fmt.Sprintf("%s:%s", c.String("username"), c.String("password"))),
			),
		)
	}

	if tls := os.Getenv("CONTRIBUTOOR_OUTPUT_SERVER_TLS"); tls != "" {
		log.Infof("Setting output server tls from env to %s", tls)

		tlsBool, err := strconv.ParseBool(tls)
		if err != nil {
			return errors.Wrap(err, "failed to parse output server tls env var")
		}

		cfg.SetOutputServerTLS(tlsBool)
	}

	if c.String("output-server-tls") != "" {
		log.Infof("Overriding output server tls from CLI to %s", c.String("output-server-tls"))

		tls, err := strconv.ParseBool(c.String("output-server-tls"))
		if err != nil {
			return errors.Wrap(err, "failed to parse output server tls flag")
		}

		cfg.SetOutputServerTLS(tls)
	}

	// Handle contributoor directory from env
	if dir := os.Getenv("CONTRIBUTOOR_DIRECTORY"); dir != "" {
		log.Infof("Setting contributoor directory from env to %s", dir)
		cfg.ContributoorDirectory = dir
	}

	// CLI flag overrides env var
	if c.String("contributoor-directory") != "" {
		log.Infof("Overriding contributoor directory from CLI to %s", c.String("contributoor-directory"))
		cfg.ContributoorDirectory = c.String("contributoor-directory")
	}

	// Handle attestation subnet check configuration from env
	if enabled := os.Getenv("CONTRIBUTOOR_ATTESTATION_SUBNET_CHECK_ENABLED"); enabled != "" {
		log.Infof("Setting attestation subnet check enabled from env to %s", enabled)

		enabledBool, err := strconv.ParseBool(enabled)
		if err != nil {
			return errors.Wrap(err, "failed to parse attestation subnet check enabled env var")
		}

		if cfg.AttestationSubnetCheck == nil {
			cfg.AttestationSubnetCheck = &config.AttestationSubnetCheck{}
		}

		cfg.AttestationSubnetCheck.Enabled = enabledBool
		cfg.AttestationSubnetCheck.MaxSubnets = 2
	}

	if maxSubnets := os.Getenv("CONTRIBUTOOR_ATTESTATION_SUBNET_MAX_SUBNETS"); maxSubnets != "" {
		log.Infof("Setting attestation subnet max subnets from env to %s", maxSubnets)

		maxSubnetsInt, err := strconv.ParseUint(maxSubnets, 10, 32)
		if err != nil {
			return errors.Wrap(err, "failed to parse attestation subnet max subnets env var")
		}

		if cfg.AttestationSubnetCheck == nil {
			cfg.AttestationSubnetCheck = &config.AttestationSubnetCheck{}
		}

		cfg.AttestationSubnetCheck.MaxSubnets = uint32(maxSubnetsInt)
	}

	// CLI flag overrides env var
	if c.Bool("attestation-subnet-check-enabled") {
		log.Infof("Setting attestation subnet check enabled from CLI to true")

		if cfg.AttestationSubnetCheck == nil {
			cfg.AttestationSubnetCheck = &config.AttestationSubnetCheck{}
		}

		cfg.AttestationSubnetCheck.Enabled = true
	}

	if c.Int("attestation-subnet-max-subnets") >= 0 { // -1 means not set
		log.Infof("Setting attestation subnet max subnets from CLI to %d", c.Int("attestation-subnet-max-subnets"))

		if cfg.AttestationSubnetCheck == nil {
			cfg.AttestationSubnetCheck = &config.AttestationSubnetCheck{}
		}

		cfg.AttestationSubnetCheck.MaxSubnets = uint32(c.Int("attestation-subnet-max-subnets")) //nolint:gosec // conversion fine.
	}

	return nil
}
