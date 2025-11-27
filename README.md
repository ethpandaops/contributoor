# contributoor

Contributoor is a lightweight service that runs alongside an Ethereum consensus client and collects data via the client's APIs. It's a streamlined, user-friendly extraction of the `sentry` service from [ethpandaops/xatu](https://github.com/ethpandaops/xatu).

[![Go Reference](https://pkg.go.dev/badge/github.com/ethpandaops/contributoor.svg)](https://pkg.go.dev/github.com/ethpandaops/contributoor)
[![codecov](https://codecov.io/gh/ethpandaops/contributoor/graph/badge.svg?token=40GPSFLS2W)](https://codecov.io/gh/ethpandaops/contributoor)
[![Go Report Card](https://goreportcard.com/badge/github.com/ethpandaops/contributoor)](https://goreportcard.com/report/github.com/ethpandaops/contributoor)
[![License](https://img.shields.io/github/license/ethpandaops/contributoor)](LICENSE)

## 📦 Installation

See the [contributoor-installer](https://github.com/ethpandaops/contributoor-installer) repository for supported platforms and installation instructions.

## 🚀 Getting Started

Once installed, you can manage the Contributoor service using these commands:

```bash
contributoor start    # Start the service
contributoor stop     # Stop the service
contributoor status   # Check service status
contributoor restart  # Restart the service
contributoor config   # View/edit configuration
contributoor update   # Update the service to the latest version
contributoor logs     # Show logs
```

## ⚙️ Configuration

The [contributoor-installer](https://github.com/ethpandaops/contributoor-installer) will generate a `config.yaml` file for you.

If you encounter configuration issues, you can:

1. Compare your config with the example below
2. Remove the config file and re-run `contributoor install` to generate a fresh one
3. Check the debug logs for detailed error messages

<details>
  <summary>📄 View Example Config</summary>

```yaml
# The address of your beacon node's HTTP API.
beaconNodeAddress: http://127.0.0.1:64692

# The address to serve metrics on (optional, disabled if empty).
metricsAddress: ":9090"

# The address to serve a health check on (optional, disabled if empty).
healthCheckAddress: ":9191"

# The log level (debug, info, warn, error).
logLevel: info

# Specifies a network name override. This is only used when connecting to testnets where
# the beacon node reports a generic network name like "testnet". For known networks
# (mainnet, sepolia, holesky, hoodi, etc.), the network is always derived automatically from
# the beacon node's configuration.
networkName: pectra-devnet-6

# The output server configuration (credentials are base64 encoded and required if a pandaops server is used).
outputServer:
    address: xatu.primary.production.platform.ethpandaops.io:443
    credentials: <base64-encoded-value>
    tls: true

# The contributoor version to use.
version: 0.0.8

# The directory where contributoor stores its configuration and data.
contributoorDirectory: /Users/username/.contributoor

# The method to run contributoor (RUN_METHOD_DOCKER, RUN_METHOD_BINARY, RUN_METHOD_SYSTEMD).
runMethod: RUN_METHOD_DOCKER
```
</details>

<details>
  <summary>Available CLI Flags</summary>

All configuration options can be overridden via CLI flags:

```bash
--config string                                    # Config file path
--debug                                            # Enable debug mode
--network string                                   # Ethereum network name (mainnet, sepolia, holesky)
--beacon-node-address string                       # Comma-separated addresses of beacon node APIs (e.g. http://localhost:5052,http://localhost:5053)
--metrics-address string                           # Address of the metrics server (e.g. :9091)
--health-check-address string                      # Address of the health check server (e.g. :9191)
--log-level string                                 # Log level (debug, info, warn, error)
--username string                                  # Username for the output server
--password string                                  # Password for the output server
--output-server-address string                     # Address of the output server (e.g. xatu.primary.production.platform.ethpandaops.io:443)
--output-server-tls string                         # Enable TLS for the output server (true/false)
--contributoor-directory string                    # Directory where contributoor stores configuration and data
--attestation-subnet-check-enabled                 # Enable attestation subnet checking for single_attestation topic filtering
--attestation-subnet-max-subnets int               # Maximum number of subnets before single_attestation topic is disabled (0-64)
--attestation-subnet-mismatch-detection-window int # Number of slots to track for subnet activity
--attestation-subnet-mismatch-threshold int        # Number of mismatches required before triggering reconnection
--attestation-subnet-mismatch-cooldown-seconds int # Cooldown period between reconnections in seconds
--attestation-subnet-high-water-mark int           # Number of additional temporary subnets allowed without triggering a restart
--release                                          # Print release and exit
```

Example with multiple flags:
```bash
go run ./cmd/sentry/main.go \
  --config ./config.yaml \
  --debug true \
  --network sepolia \
  --beacon-node-address http://localhost:5052 \
  --metrics-address localhost:9091 \
  --log-level debug
```
</details>

## 🔨 Development

<details>
  <summary>Running Locally</summary>

To run Contributoor in development mode:

```bash
go run ./cmd/sentry --config /path/to/.contributoor/config.yaml --debug true
```

The `config.yaml` would have been generated for you by the installer.
</details>


<details>
  <summary>Code Generation</summary>

Generate protocol buffers and other generated code:

```bash
go generate ./...
make proto
```
</details>

<details>
  <summary>Testing</summary>

Run tests with race detection, coverage reporting, and view the coverage report:

```bash
go test -race -failfast -cover -coverpkg=./... -coverprofile=coverage.out ./... && go tool cover -html=coverage.out
```

</details>

## 📊 Share Your Node Data

Help improve Ethereum by contributing your node's data to our research. Data is published openly in a privacy-focused manner and used for research and analysis. Let us know if you're interested by completing this [form](https://ethpandaops.io/contribute-data/).

## 🤝 Contributing Code

Contributoor is part of EthPandaOps' suite of tools for Ethereum network operations. Contributions are welcome! Please check our [GitHub repository](https://github.com/ethpandaops) for more information.
