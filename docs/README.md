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

## Running the Server

1. Start the gRPC server:
```bash
./bin/orderbook-server
```

The server will:
- Start on port 50051
- Create a default order book
- Enable gRPC reflection for tools like grpcurl

## Running the Client

The client supports several commands for interacting with the order book. Here are some examples:

### Create a Sell Order
```bash
./bin/orderbook-client create-order default SELL LIMIT 0.5 100.0 sell1
```
This creates a sell order with:
- Book: default
- Side: SELL
- Type: LIMIT
- Quantity: 0.5
- Price: 100.0
- Order ID: sell1

### Create a Buy Order
```bash
./bin/orderbook-client create-order default BUY LIMIT 0.5 100.0 buy1
```
This creates a buy order with:
- Book: default
- Side: BUY
- Type: LIMIT
- Quantity: 0.5
- Price: 100.0
- Order ID: buy1

### Check Order Book State
```bash
./bin/orderbook-client get-state default
```
This shows the current state of the order book, including all bids and asks.

### Get Specific Order Details
```bash
./bin/orderbook-client get-order default buy1
```
This shows details for a specific order.

### Cancel an Order
```bash
./bin/orderbook-client cancel-order default buy1
```
This cancels the specified order.

## Common Issues

1. **Port Already in Use**
   If you see the error "address already in use", it means another instance of the server is running. You can:
   - Find and kill the existing process: `lsof -i :50051`
   - Or use a different port by modifying the server configuration

2. **Order Already Exists**
   If you get an "order exists" error, try:
   - Using a different order ID
   - Canceling the existing order first
   - Checking the order book state to see existing orders

3. **Connection Issues**
   If the client can't connect to the server:
   - Make sure the server is running
   - Check that you're using the correct port
   - Verify network connectivity

## Additional Commands

### List All Order Books
```bash
./bin/orderbook-client list-books
```

### Create a New Order Book
```bash
./bin/orderbook-client create-book mybook
```

### Delete an Order Book
```bash
./bin/orderbook-client delete-book mybook
```

## Debugging

The server logs are helpful for debugging. You can see:
- Order creation and processing
- Matching engine activity
- Error messages and warnings

## Testing Different Scenarios

1. **Market Orders**
```bash
./bin/orderbook-client create-order default BUY MARKET 1.0 0.0 market1
```

2. **Different Time-in-Force**
```bash
./bin/orderbook-client create-order default SELL LIMIT 1.0 100.0 ioc1 --time-in-force=IOC
```

3. **Multiple Orders**
```bash
# Create multiple sell orders
./bin/orderbook-client create-order default SELL LIMIT 0.5 100.0 sell1
./bin/orderbook-client create-order default SELL LIMIT 0.5 101.0 sell2

# Create a buy order that matches
./bin/orderbook-client create-order default BUY LIMIT 1.0 101.0 buy1
``` 