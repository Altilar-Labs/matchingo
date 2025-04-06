[![Go Report Card](https://goreportcard.com/badge/github.com/erain9/matchingo)](https://goreportcard.com/report/github.com/erain9/matchingo)
![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/erain9/matchingo)
![GitHub](https://img.shields.io/github/license/erain9/matchingo)

# Matchingo - Go Order Matching Engine

> **Note**: This project is inspired by the original [gonevo/matchingo](https://github.com/gonevo/matchingo) package but has undergone significant rewrites, reorganization, and improvements. The API, implementation details, and overall architecture have been substantially modified to enhance performance, testability, and maintainability.

Matchingo is a powerful and flexible order matching engine written in Go. It's designed to be both fast and adaptable, with support for both in-memory and Redis-based storage backends.

## Features

- High-performance order matching for financial markets
- Support for various order types (Market, Limit, Stop, OCO)
- Multiple time-in-force options (GTC, IOC, FOK)
- Pluggable backend system with implementations for:
  - In-memory storage (for single instance deployments)
  - Redis storage (for distributed deployments)
- Comprehensive test suite with high code coverage
- Decimal precision using fpdecimal for accurate financial calculations

## Project Structure

The project follows standard Go project layout conventions:

```
matchingo/
├── pkg/                # Library code
│   ├── core/           # Core domain logic
│   ├── backend/        # Backend implementations
│       ├── memory/     # In-memory backend
│       └── redis/      # Redis backend
├── cmd/                # Command-line applications
│   └── examples/       # Example applications
│       ├── basic/      # Basic in-memory example
│       └── redis/      # Redis-backed example
```

## Getting Started

### Prerequisites

- Go 1.22 or higher
- For Redis backend: Redis server

### Building

To build the examples:

```bash
make build
```

This creates executable files in the `build/` directory.

### Running Examples

To run the basic in-memory example:

```bash
make demo-memory
```

To run the Redis-backed example (requires a Redis server running on localhost:6379):

```bash
make demo-redis
```

### Running Tests

To run tests:

```bash
make test
```

For verbose test output:

```bash
make test-v
```

### Running Benchmarks

To run all benchmarks:

```bash
make bench
```

To run only in-memory backend benchmarks:

```bash
make bench-memory
```

To run only Redis backend benchmarks (requires Redis running):

```bash
make bench-redis
```

## Performance Benchmarks

Matchingo's performance has been optimized for both memory and Redis backends. Below are sample benchmark results showing operations per second, memory allocations, and operation latency.

### Memory Backend Performance

The in-memory backend provides extremely low latency and high throughput, making it suitable for high-performance single-instance deployments.

```
BenchmarkMemoryBackend_StoreOrder-16            10284558               291.7 ns/op           254 B/op          3 allocs/op
BenchmarkMemoryBackend_GetOrder-16              237962809               10.00 ns/op            0 B/op          0 allocs/op
BenchmarkMemoryBackend_AppendToSide-16           5421882               507.1 ns/op           325 B/op          4 allocs/op
BenchmarkMemoryBackend_RemoveFromSide-16        25432212                93.15 ns/op            8 B/op          1 allocs/op
BenchmarkOrderBook_Process_Memory-16             4345374               541.8 ns/op           500 B/op          5 allocs/op
BenchmarkOrderBook_LargeOrderBook_Memory-16      7782270               381.1 ns/op           400 B/op          4 allocs/op
```

Key performance highlights:
- Order storage: ~10M ops/sec (~292 ns/op)
- Order retrieval: ~238M ops/sec (~10 ns/op)
- Order processing: ~4.3M ops/sec (~542 ns/op)

### Redis Backend Performance

The Redis backend sacrifices some raw performance for distributed capabilities, making it suitable for scalable multi-instance deployments. These benchmarks were performed with Redis 7.2.7 running natively on the local machine.

```
BenchmarkRedisBackend_StoreOrder-16                15501            153813 ns/op            1275 B/op         25 allocs/op
BenchmarkRedisBackend_GetOrder-16                  30518             78344 ns/op            1377 B/op         25 allocs/op
BenchmarkRedisBackend_AppendToSide-16               7520            311517 ns/op            1877 B/op         46 allocs/op
BenchmarkOrderBook_Process_Redis-16                 6055            390175 ns/op            2252 B/op         56 allocs/op
BenchmarkOrderBook_SmallOrderBook_Redis-16          7612            309645 ns/op            2500 B/op         46 allocs/op
```

Key performance highlights:
- Order storage: ~6.5K ops/sec (~154 μs/op)
- Order retrieval: ~12.8K ops/sec (~78 μs/op)
- Order processing: ~2.6K ops/sec (~390 μs/op)

Redis's own internal benchmark shows excellent raw performance:
```
SET: 694,500 requests per second
GET: 757,636 requests per second
```

The difference between Redis's raw performance and our benchmarks is due to the additional processing required for order serialization, Redis command composition, and network communication.

### Performance Comparison

|  Operation   | Memory Backend | Redis Backend | Difference |
|--------------|---------------|---------------|------------|
| Store Order  | 292 ns        | 154 μs        | ~527x      |
| Get Order    | 10 ns         | 78 μs         | ~7800x     |
| Process Order| 542 ns        | 390 μs        | ~720x      |

The memory backend is significantly faster (up to ~7800x for certain operations), but the Redis backend offers distributed capabilities that may be necessary for certain applications. The performance difference is expected given the nature of in-memory operations versus network communication with a separate process.

## Usage

### Creating an Order Book

Using the in-memory backend:

```go
import (
    "github.com/erain9/matchingo/pkg/backend/memory"
    "github.com/erain9/matchingo/pkg/core"
)

// Create a new in-memory backend
backend := memory.NewMemoryBackend()

// Create an order book with the backend
book := core.NewOrderBook(backend)
```

Using the Redis backend:

```go
import (
    "context"
    "github.com/redis/go-redis/v9"
    redisbackend "github.com/erain9/matchingo/pkg/backend/redis"
    "github.com/erain9/matchingo/pkg/core"
)

// Connect to Redis
client := redis.NewClient(&redis.Options{
    Addr: "localhost:6379",
})

// Create a Redis backend with a prefix
backend := redisbackend.NewRedisBackend(client, "myorderbook")

// Create an order book with the Redis backend
book := core.NewOrderBook(backend)
```

### Working with Orders

Creating and processing orders:

```go
import (
    "github.com/nikolaydubina/fpdecimal"
    "github.com/erain9/matchingo/pkg/core"
)

// Create a limit sell order
sellOrderID := "sell_123"
sellPrice := fpdecimal.FromFloat(10.0)
sellQuantity := fpdecimal.FromFloat(10.0)
sellOrder := core.NewLimitOrder(sellOrderID, core.Sell, sellQuantity, sellPrice, core.GTC, "")

// Process the sell order
sellDone, err := book.Process(sellOrder)
if err != nil {
    // Handle error
}

// Create a limit buy order
buyOrderID := "buy_123"
buyPrice := fpdecimal.FromFloat(10.0)
buyQuantity := fpdecimal.FromFloat(5.0)
buyOrder := core.NewLimitOrder(buyOrderID, core.Buy, buyQuantity, buyPrice, core.GTC, "")

// Process the buy order
buyDone, err := book.Process(buyOrder)
if err != nil {
    // Handle error
}
```

## License

This project is licensed under the MIT License - see the LICENSE file for details.

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
