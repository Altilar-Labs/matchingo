# Matchingo Documentation

This document provides step-by-step instructions for testing and using the Matchingo order book system.

## Important Note

**Before creating any orders, you MUST first create an order book!** 
If you get an error like `order book default not found`, it means you tried to use an order book that doesn't exist yet.

Example:
```bash
# First create the order book
./bin/orderbook-client create-book default --backend=memory

# Then you can create orders
./bin/orderbook-client create-order default SELL LIMIT 0.5 100.0 sell1
```

## Step-by-Step Testing Guide

Follow these steps to test the Matchingo order book system:

### Prerequisites

*   Go 1.21+
*   Make
*   Docker and Docker Compose (for Redis backend integration tests)

### 1. Build

First, ensure the project is built:

```bash
make clean && make build
```

### 2. Start the Server

For manual testing or running unit/integration tests that don't manage dependencies:

```bash
./bin/orderbook-server
```

### 3. Running Tests

*   **Run all unit and basic integration tests (Memory Backend):**
    ```bash
    make test
    # or
    go test -v -race -coverprofile=coverage.out ./...
    ```

*   **Run V2 Integration Tests (Memory Backend, incl. Kafka message checks):**
    This runs the tests defined in `pkg/server/integration_v2_test.go`.
    ```bash
    go test -v -race ./pkg/server/... -run IntegrationV2
    ```

*   **Run Redis Integration Tests (Requires Docker):**
    This target automatically starts Redis in Docker (via `docker-compose.yml` on host port 6380), runs the specific Redis tests in `pkg/server/redis_integration_test.go`, and stops Redis.
    ```bash
    make test-redis
    # Manual equivalent:
    # make test-deps-up
    # go test -v -race ./pkg/server/... -run RedisIntegration
    # make test-deps-down
    ```

### 4. Manual CLI Testing Steps (Memory Backend)

*Ensure the server is running (`./bin/orderbook-server`)*

1.  **Create Order Book:**
    ```bash
    ./bin/orderbook-client create-book default --backend=memory
    ```

2.  **Create Orders:**
    ```bash
    # Sell Limit
    ./bin/orderbook-client create-order default SELL LIMIT 0.5 100.0 sell1
    # Buy Limit
    ./bin/orderbook-client create-order default BUY LIMIT 1.0 95.0 buy1
    ```

3.  **View State:**
    ```bash
    ./bin/orderbook-client get-state default
    ```

4.  **Create Matching Market Order:**
    ```bash
    ./bin/orderbook-client create-order default BUY MARKET 0.3 0.0 buy2
    ```

5.  **View State Again:**
    ```bash
    ./bin/orderbook-client get-state default
    ```

6.  **Cancel Order:**
    ```bash
    ./bin/orderbook-client cancel-order default buy1
    ```

7.  **List Books:**
    ```bash
    ./bin/orderbook-client list-books
    ```

### 5. Testing with Redis (Manual)

1.  Start the test Redis container:
    ```bash
    make test-deps-up
    ```
2.  Start the server (it will connect to default localhost:50051, ensure no conflicts)
    ```bash
    ./bin/orderbook-server
    ```
3.  Use the client. Note: `create-book` CLI doesn't currently support passing Redis options, so it relies on server defaults or prior creation via integration tests.
    ```bash
    # Example (assuming server defaults to Redis or book exists)
    ./bin/orderbook-client create-book redis-book # Might need modification
    ./bin/orderbook-client create-order redis-book BUY LIMIT 1.0 100.0 rbuy1
    ./bin/orderbook-client get-state redis-book
    ```
4.  Stop the test Redis container:
    ```bash
    make test-deps-down
    ```

## Troubleshooting

### Common Errors

1. **Order book not found**:
   ```
   CreateOrder failed error="rpc error: code = NotFound desc = order book default not found"
   ```
   **Solution**: Create the order book first with `create-book` command before creating orders.

2. **Duplicate order ID**:
   ```
   CreateOrder failed error="rpc error: code = AlreadyExists desc = order with ID sell1 already exists"
   ```
   **Solution**: Use a unique order ID for each order within the same order book.

3. **Invalid order parameters**:
   ```
   CreateOrder failed error="rpc error: code = InvalidArgument desc = invalid order parameters"
   ```
   **Solution**: Ensure all order parameters are valid (e.g., positive quantity, valid price for limit orders).

4. **Server not running**:
   ```
   Failed to connect to server error="context deadline exceeded"
   ```
   **Solution**: Ensure the server is running with `./bin/orderbook-server`.

## Command Reference

### Server Commands

```bash
# Start server
./bin/orderbook-server

# Start server with custom port
./bin/orderbook-server --port=50052

# Start server with debug logging
./bin/orderbook-server --log-level=debug

# Start server with pretty logging
./bin/orderbook-server --pretty
```

### Client Commands

```bash
# Create order book
./bin/orderbook-client create-book <name> [--backend=memory|redis]

# Create order
./bin/orderbook-client create-order <book> <side> <type> <quantity> <price> <id> [--stop-price=<price>] [--tif=GTC|IOC|FOK]

# Cancel order
./bin/orderbook-client cancel-order <book> <order-id>

# Get order book state
./bin/orderbook-client get-state <book>

# List order books
./bin/orderbook-client list-books

# Get specific order
./bin/orderbook-client get-order <book> <order-id>
```

### Order Parameters

- **Side**: `BUY` or `SELL`
- **Type**: `MARKET`, `LIMIT`, or `STOP_LIMIT`
- **Quantity**: Order quantity (positive number)
- **Price**: Order price (0.0 for market orders)
- **ID**: Unique order identifier
- **Stop Price**: Trigger price for stop-limit orders
- **TIF** (Time-in-Force): `GTC` (Good Till Canceled, default), `IOC` (Immediate or Cancel), or `FOK` (Fill or Kill)

## Additional Information

For more detailed information, refer to:
- API Documentation: See [api.md](api.md)
- Development Guide: See [development.md](development.md)
- Testing Guide: See [testing.md](testing.md) 