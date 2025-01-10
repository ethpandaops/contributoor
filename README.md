# contributoor

Contributoor is a lightweight service that runs alongside an Ethereum consensus client and collects data via the client's APIs. It's a streamlined, user-friendly extraction of the `sentry` service from [ethpandaops/xatu](https://github.com/ethpandaops/xatu).

## Installation

See the 📦[contributoor-installer](https://github.com/ethpandaops/contributoor-installer) repository for supported platforms and installation instructions.

## Getting Started

Once installed, you can manage the Contributoor service using these commands:

```bash
contributoor start    # Start the service
contributoor stop     # Stop the service
contributoor status   # Check service status
contributoor restart  # Restart the service
contributoor config   # View/edit configuration
contributoor update   # Update the service to the latest version
```

## Configuration

The [contributoor-installer](https://github.com/ethpandaops/contributoor-installer) will generate a `config.yaml` file for you.

```yaml
# The address of your beacon node's HTTP API.
beaconNodeAddress: http://127.0.0.1:64692

# The address of your metrics server (defaults to :9090).
metricsAddress: ":9090"

# The log level (debug, info, warn, error).
logLevel: info

# The network name (NETWORK_NAME_MAINNET, NETWORK_NAME_SEPOLIA, NETWORK_NAME_HOLESKY).
networkName: NETWORK_NAME_MAINNET

# The output server configuration (credentials are base64 encoded and required if a pandaops server is used).
outputServer:
    address: https://xatu.primary.production.platform.ethpandaops.io
    credentials: YWRtaW46YWRtaW4=

# The contributoor version to use.
version: 0.0.8

# The directory where contributoor stores its configuration and data.
contributoorDirectory: /Users/username/.contributoor

# The method to run contributoor (RUN_METHOD_DOCKER, RUN_METHOD_BINARY, RUN_METHOD_SYSTEMD).
```

If you encounter configuration issues, you can:

1. Compare your config with the example above
2. Remove the config file and re-run `contributoor install` to generate a fresh one
3. Check the debug logs for detailed error messages

## Development

### Running Locally

To run Contributoor in development mode:

```bash
go run ./cmd/sentry/main.go --config /path/to/.contributoor/config.yaml --debug true
```

The `config.yaml` would have been generated for you by the installer.

### Code Generation

Generate protocol buffers and other generated code:

```bash
go generate ./...
make proto
```

### Testing

Run tests with race detection, coverage reporting, and view the coverage report:

```bash
go test -race -failfast -cover -coverpkg=./... -coverprofile=coverage.out ./... && go tool cover -html=coverage.out
```

## Contributing

Contributoor is part of EthPandaOps' suite of tools for Ethereum network operations. Contributions are welcome! Please check our [GitHub repository](https://github.com/ethpandaops) for more information.
