[![Go Report Card](https://goreportcard.com/badge/github.com/erain9/matchingo)](https://goreportcard.com/report/github.com/erain9/matchingo)
![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/erain9/matchingo)
![GitHub](https://img.shields.io/github/license/erain9/matchingo)

# Matchingo

A high-performance order book matching engine written in Go.

## Features

- In-memory and Redis-backed order book implementations
- Support for multiple order types (LIMIT, MARKET)
- Support for different time-in-force options (GTC, IOC, FOK)
- gRPC API for order book operations
- Comprehensive logging and monitoring
- High-performance matching engine

## Project Structure

```
.
├── cmd/                    # Command-line applications
│   ├── client/            # gRPC client implementation
│   └── server/            # gRPC server implementation
├── pkg/                   # Reusable packages
│   ├── api/              # Protocol buffer definitions and gRPC services
│   ├── backend/          # Backend implementations (memory, Redis)
│   ├── core/             # Core order book logic
│   ├── logging/          # Logging utilities
│   └── server/           # Server-side gRPC service implementation
├── docs/                  # Documentation
├── Makefile              # Build and development tasks
└── README.md             # This file
```

## Prerequisites

- Go 1.21 or later
- Make
- Protocol Buffers compiler (protoc)

## Building

1. Clone the repository:
```bash
git clone https://github.com/erain9/matchingo.git
cd matchingo
```

2. Build the project:
```bash
make clean && make build-all
```

This will:
- Clean any previous builds
- Generate protobuf code
- Build both server and client binaries
- Place the binaries in the `./bin/` directory

## Running

### Server

Start the gRPC server:
```bash
./bin/orderbook-server
```

The server will:
- Start on port 50051
- Create a default order book
- Enable gRPC reflection for tools like grpcurl

### Client

The client supports several commands for interacting with the order book. See [docs/README.md](docs/README.md) for detailed usage instructions.

## Development

### Code Generation

To generate protobuf code:
```bash
make proto
```

### Testing

To run tests:
```bash
make test
```

### Linting

To run linters:
```bash
make lint
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Commit your changes
4. Push to the branch
5. Create a Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE.md](LICENSE.md) file for details.

## Acknowledgments

* Original concept inspired by [gonevo/matchingo](https://github.com/gonevo/matchingo)
* Refactored and enhanced by [erain9](https://github.com/erain9)

## Matching Engine

The `matchingo` library includes a high-performance matching engine that follows price-time priority rules for matching orders. The matching engine supports:

- Market orders: Execute immediately at the best available price
- Limit orders: Execute at a specified price or better
- Stop orders: Become active when a specified price is reached

### Key Features of the Matching Engine

- **Price-Time Priority**: Orders are matched based on price first, then time of arrival
- **Efficient Matching Algorithm**: O(1) lookup for price levels, O(n) for order processing within a price level
- **Partial Fills**: Orders can be partially filled, with the remaining quantity staying in the book
- **Trade Recording**: All trades are recorded in the `Done` object returned from order processing

### Example Usage

To see the matching engine in action, run the enhanced example:

```bash
go run ./cmd/examples/matching/enhanced_example/main.go
```

This example demonstrates:
- Adding orders to the book
- Matching orders at the same price level
- Matching orders across multiple price levels
- Market order execution
- Partial fills and order book updates

## gRPC Service

Matchingo now includes a gRPC service for managing multiple order books. The service provides a comprehensive API for creating and managing order books, as well as executing trades.

### Features

- Create, get, list, and delete order books
- Create, get, and cancel orders
- Get order book state (depth, price levels)
- Support for multiple backend types (memory, Redis)
- Comprehensive logging with request IDs and structured logs

### Building and Running

To generate the protobuf files:

```bash
make proto
```

To build the server and client:

```bash
make build-all
```

To run the server:

```bash
./bin/orderbook-server
```

The server accepts the following flags:
- `--port`: The port to listen on (default: 50051)
- `--log-level`: The log level (debug, info, warn, error) (default: info)
- `--pretty`: Enable pretty logging (default: false)

### Using the Client

The client provides a simple command-line interface for interacting with the server. Here are some examples:

Create an order book:

```bash
./bin/orderbook-client create-book --name=btcusd --backend=memory
```

Create an order:

```bash
./bin/orderbook-client create-order --book=btcusd --id=order1 --side=buy --type=limit --qty=1.0 --price=50000.0
```

List all order books:

```bash
./bin/orderbook-client list-books
```

Get order book state:

```bash
./bin/orderbook-client get-state --book=btcusd --depth=5
```

Run the client without arguments to see all available commands:

```bash
./bin/orderbook-client
```
