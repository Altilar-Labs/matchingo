# Matchingo - gRPC Server and Client Test Documentation

This document provides test instructions and examples for running the Matchingo gRPC server and client components. It includes step-by-step guides for testing different order book scenarios and troubleshooting common issues.

## Prerequisites

- Go 1.21 or later
- Make
- Protocol Buffers compiler (protoc)

## Building the Project

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

## Step-by-Step Functional Test

Follow these instructions precisely to run a complete functional test of the system:

### 1. Ensure No Existing Server Is Running

First, make sure no orderbook server is already running:

```bash
# Find and kill any existing server processes
pkill -f orderbook-server
```

### 2. Start the Server

Start the server in the background:

```bash
# Make sure you're in the project root directory
cd /path/to/matchingo

# Start the server and send it to background
./bin/orderbook-server &

# Wait a moment for server to initialize
sleep 2

# Note the process ID if you need to stop it later
echo "Server PID: $!"
```

### 3. Basic Order Book Testing

Let's create and interact with the order book:

```bash
# Create a sell limit order
./bin/orderbook-client create-order default SELL LIMIT 0.5 100.0 sell1

# Check the order book state
./bin/orderbook-client get-state default

# Get details of the order we just created
./bin/orderbook-client get-order default sell1

# Create a buy limit order at a lower price (shouldn't match)
./bin/orderbook-client create-order default BUY LIMIT 0.4 95.0 buy1

# Check the order book state again (should show both orders)
./bin/orderbook-client get-state default
```

### 4. Trade Matching

Now let's create matching orders:

```bash
# Create a buy order that matches with the sell order
./bin/orderbook-client create-order default BUY LIMIT 0.3 100.0 buy2

# Check the order book state (sell1 should be partially filled)
./bin/orderbook-client get-state default

# Create a market buy order to match remainder of sell1
./bin/orderbook-client create-order default BUY MARKET 0.2 0.0 buy3

# Check the order book state (should only have buy1 left)
./bin/orderbook-client get-state default
```

### 5. Stop-Limit Order Testing

Test stop-limit orders:

```bash
# Create a buy stop-limit order (triggers when price >= 105.0)
./bin/orderbook-client create-order default BUY STOP_LIMIT 1.0 100.0 stoplimit1 105.0

# Check the order (should be open but not visible in order book)
./bin/orderbook-client get-order default stoplimit1
./bin/orderbook-client get-state default

# Create a sell order at a price above the stop price
./bin/orderbook-client create-order default SELL LIMIT 0.1 110.0 sell2

# This should activate the stop-limit order
./bin/orderbook-client get-order default stoplimit1
./bin/orderbook-client get-state default
```

### 6. Order Cancellation

Test cancelling orders:

```bash
# Create another order
./bin/orderbook-client create-order default SELL LIMIT 2.0 120.0 sell3

# Verify it exists
./bin/orderbook-client get-state default

# Cancel the order
./bin/orderbook-client cancel-order default sell3

# Verify it's gone
./bin/orderbook-client get-state default
```

### 7. Creating Multiple Order Books

```bash
# Create another order book
./bin/orderbook-client create-book crypto

# List all order books
./bin/orderbook-client list-books

# Create orders in the new book
./bin/orderbook-client create-order crypto BUY LIMIT 1.0 50.0 crypto-buy1
./bin/orderbook-client create-order crypto SELL LIMIT 1.0 55.0 crypto-sell1

# Check the new order book state
./bin/orderbook-client get-state crypto
```

### 8. Cleanup

When you're done testing, clean up:

```bash
# Kill the server
pkill -f orderbook-server
```

## Troubleshooting

### Server Already Running

If you see this error: "listen tcp :50051: bind: address already in use"

```bash
# Find processes using port 50051
lsof -i :50051

# Kill the process (replace PID with the actual process ID)
kill -9 PID
```

### Client Can't Connect

If the client commands fail with connection issues:

1. Make sure the server is running:
```bash
ps aux | grep orderbook-server
```

2. Check the server logs for any errors

3. Try restarting the server:
```bash
pkill -f orderbook-server
./bin/orderbook-server &
```

### Order Creation Errors

If you get "order already exists" errors, use a different order ID or delete the existing order:

```bash
./bin/orderbook-client cancel-order default order_id
```

## Additional Commands Reference

### Create Order with Different Types

```bash
# Limit order
./bin/orderbook-client create-order default BUY LIMIT 1.0 100.0 limit1

# Market order
./bin/orderbook-client create-order default SELL MARKET 1.0 0.0 market1

# Stop-limit order (stop price of 105.0, limit price of 100.0)
./bin/orderbook-client create-order default BUY STOP_LIMIT 1.0 100.0 stop1 105.0
```

### Order Book Management

```bash
# Create book
./bin/orderbook-client create-book mybook

# Get book
./bin/orderbook-client get-book mybook

# List books
./bin/orderbook-client list-books

# Delete book
./bin/orderbook-client delete-book mybook
```

### Order Management

```bash
# Get order
./bin/orderbook-client get-order default order1

# Cancel order
./bin/orderbook-client cancel-order default order1

# Get order book state
./bin/orderbook-client get-state default
``` 