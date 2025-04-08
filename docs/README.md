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

### 1. Start the Server

First, make sure you have built the project:

```bash
make clean && make build
```

Then start the server:

```bash
./bin/orderbook-server
```

You should see output indicating the server has started successfully:

```
INFO Starting gRPC server on port 50051...
```

### 2. Create an Order Book

Before you can create any orders, you must first create an order book:

```bash
./bin/orderbook-client create-book default --backend=memory
```

Expected output:

```
INFO Created order book "default"
```

### 3. Create Orders

Now you can create orders in the order book:

```bash
# Create a sell limit order
./bin/orderbook-client create-order default SELL LIMIT 0.5 100.0 sell1

# Create a buy limit order
./bin/orderbook-client create-order default BUY LIMIT 1.0 95.0 buy1
```

Expected output for the first command:

```
INFO Created order sell1 in order book default
```

### 4. View Order Book State

Check the current state of the order book:

```bash
./bin/orderbook-client get-state default
```

Expected output:

```
+----------------- Order Book: default -----------------+
|                BIDS                |       ASKS       |
+--------------------+---------------+------------------+
| Price    | Quantity | Orders | Price    | Quantity | Orders |
+----------+----------+--------+----------+----------+--------+
| 95.00    | 1.0000   | 1      | 100.00   | 0.5000   | 1      |
+----------+----------+--------+----------+----------+--------+
```

### 5. Create a Market Order

Create a market order that will match against existing limit orders:

```bash
./bin/orderbook-client create-order default BUY MARKET 0.3 0.0 buy2
```

Expected output:

```
INFO Created order buy2 in order book default
INFO Order matched! Taker: buy2, Maker: sell1, Quantity: 0.3, Price: 100.0
```

### 6. Check Order Book State Again

Check how the order book state has changed:

```bash
./bin/orderbook-client get-state default
```

Expected output:

```
+----------------- Order Book: default -----------------+
|                BIDS                |       ASKS       |
+--------------------+---------------+------------------+
| Price    | Quantity | Orders | Price    | Quantity | Orders |
+----------+----------+--------+----------+----------+--------+
| 95.00    | 1.0000   | 1      | 100.00   | 0.2000   | 1      |
+----------+----------+--------+----------+----------+--------+
```

Notice the ASKS quantity has been reduced by the matched amount (0.3).

### 7. Create a Stop Limit Order

Create a stop limit order that will become active when the stop price is reached:

```bash
./bin/orderbook-client create-order default BUY STOP_LIMIT 2.0 102.0 buy3 --stop-price=101.0
```

Expected output:

```
INFO Created order buy3 in order book default
```

### 8. Cancel an Order

Cancel an existing order:

```bash
./bin/orderbook-client cancel-order default buy1
```

Expected output:

```
INFO Cancelled order buy1 in order book default
```

### 9. List All Order Books

List all available order books:

```bash
./bin/orderbook-client list-books
```

Expected output:

```
INFO Order books: [default]
```

### 10. Create Another Order Book

Create another order book with a different name:

```bash
./bin/orderbook-client create-book BTCUSD --backend=memory
```

Expected output:

```
INFO Created order book "BTCUSD"
```

### 11. Test with Redis Backend

If you have Redis running, you can test with the Redis backend:

```bash
# Create an order book with Redis backend
./bin/orderbook-client create-book ETHUSD --backend=redis

# Add orders to the Redis-backed order book
./bin/orderbook-client create-order ETHUSD BUY LIMIT 2.0 1800.0 eth_buy1
./bin/orderbook-client create-order ETHUSD SELL LIMIT 1.5 1850.0 eth_sell1

# Check the state
./bin/orderbook-client get-state ETHUSD
```

### 12. Test Different Time-in-Force Options

Test immediate-or-cancel (IOC) order:

```bash
./bin/orderbook-client create-order default BUY LIMIT 0.1 90.0 ioc_order --tif=IOC
```

Test fill-or-kill (FOK) order:

```bash
./bin/orderbook-client create-order default SELL LIMIT 2.0 105.0 fok_order --tif=FOK
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