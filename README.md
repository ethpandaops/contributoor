# Contributoor

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
