# Manual Testing Guide for Matchingo

This document provides step-by-step instructions for manually testing the Matchingo trading system using Docker containers and the command-line client.

## Prerequisites

- Go 1.21+ installed
- Docker and Docker Compose installed
- Git (to clone the repository if needed)

## Setting Up the Test Environment

### 1. Start Docker Services

The project includes a Docker Compose file that starts Redis, Kafka with Zookeeper, and the Matchingo server. Start all services with:

```bash
# Navigate to the project root
cd /path/to/matchingo

# Start all services
PATH="/Applications/Docker.app/Contents/Resources/bin:$PATH" docker-compose up -d
```

If Docker is properly configured in your PATH, you can simply use:

```bash
docker-compose up -d
```

This starts:
- Redis (port 6379)
- Zookeeper (port 2181)
- Kafka (port 9092)
- Matchingo server (ports 50051 for gRPC, 8080 for HTTP)

### 2. Build the Client

Build the orderbook client to interact with the server:

```bash
go build -o bin/orderbook-client cmd/client/main.go
```

## Test Scenarios

### Basic Order Book Operations

#### 1. Create an Order Book

First, create an order book with a memory backend:

```bash
./bin/orderbook-client create-book --name=default --backend=memory
```

Expected output:
```
INFO Created order book backend=MEMORY created_at=<timestamp> name=default
```

#### 2. List Order Books

Verify the order book was created:

```bash
./bin/orderbook-client list-books
```

Expected output should include the "default" order book.

### Basic Order Operations

#### 3. Create a Buy Limit Order

Create a buy limit order for 10 units at price 100.0:

```bash
./bin/orderbook-client create-order default BUY LIMIT 10.0 100.0 order1
```

Expected output:
```
INFO Created order filled_quantity=0 order_id=order1 remaining_quantity=10.000 status=OPEN
INFO Fill index=1 price=100.000 quantity=0 timestamp=<timestamp>
```

#### 4. Verify the Order

Get order details:

```bash
./bin/orderbook-client get-order default order1
```

Expected output:
```
INFO Retrieved order book=default created_at=<timestamp> filled_quantity=0 order_id=order1 price=100.000 quantity=10.000 remaining_quantity=10.000 side=BUY status=OPEN time_in_force=GTC type=LIMIT updated_at=<timestamp>
```

#### 5. Check Order Book State

View the current state of the order book:

```bash
./bin/orderbook-client get-state default
```

Expected output should show a buy order at price 100.0 with quantity 10.0:
```
 Price|Quantity|Orders|Side
---------------|---------------|---------------|----
---------------|---------------|---------------|----
        100.000|         10.000|              1|BID
```

### Order Matching Scenarios

#### 6. Partial Order Match

Create a sell limit order that partially matches with the existing buy order:

```bash
./bin/orderbook-client create-order default SELL LIMIT 5.0 100.0 order2
```

Expected output:
```
INFO Created order filled_quantity=5.000 order_id=order2 remaining_quantity=0 status=FILLED
INFO Fill index=1 price=100.000 quantity=5.000 timestamp=<timestamp>
```

#### 7. Verify Partially Filled Order

Check the status of the first order:

```bash
./bin/orderbook-client get-order default order1
```

Expected output should show it's partially filled:
```
INFO Retrieved order book=default created_at=<timestamp> filled_quantity=5.000 order_id=order1 price=100.000 quantity=10.000 remaining_quantity=5.000 side=BUY status=PARTIALLY_FILLED time_in_force=GTC type=LIMIT updated_at=<timestamp>
```

#### 8. Check Updated Order Book State

Check the order book state again:

```bash
./bin/orderbook-client get-state default
```

Expected output should show the reduced quantity:
```
 Price|Quantity|Orders|Side
---------------|---------------|---------------|----
---------------|---------------|---------------|----
        100.000|          5.000|              1|BID
```

#### 9. Full Order Match

Create a market sell order to complete the match:

```bash
./bin/orderbook-client create-order default SELL MARKET 5.0 0.0 order3
```

Expected output:
```
INFO Created order filled_quantity=5.000 order_id=order3 remaining_quantity=0 status=FILLED
INFO Fill index=1 price=100.000 quantity=5.000 timestamp=<timestamp>
```

#### 10. Verify Empty Order Book

Check that the order book is now empty:

```bash
./bin/orderbook-client get-state default
```

Expected output should show an empty order book:
```
 Price|Quantity|Orders|Side
---------------|---------------|---------------|----
---------------|---------------|---------------|----
```

### Advanced Order Types

#### 11. Stop Limit Orders

Create a stop limit order that will trigger when the market price reaches a specific level:

```bash
./bin/orderbook-client create-order default BUY STOP_LIMIT 5.0 95.0 order4 --stop-price=90.0
```

Create a sell order at the trigger price:

```bash
./bin/orderbook-client create-order default SELL LIMIT 1.0 90.0 order5
```

The stop order should be activated and converted to a limit order.

#### 12. Fill-or-Kill (FOK) Orders

Create a limit order on the buy side:

```bash
./bin/orderbook-client create-order default BUY LIMIT 10.0 100.0 order6
```

Create a FOK order that should be fully matched or canceled:

```bash
./bin/orderbook-client create-order default SELL LIMIT 15.0 100.0 order7 --time-in-force=FOK
```

This order should be canceled as there's only 10.0 quantity available.

## Cleanup

When done testing, stop the Docker containers:

```bash
PATH="/Applications/Docker.app/Contents/Resources/bin:$PATH" docker-compose down
```

Or if Docker is in your PATH:

```bash
docker-compose down
```

## Troubleshooting

### Docker Issues

- **Connection refused**: Ensure Docker is running and all services have started properly.
- **Port conflicts**: If you have other services using ports 6379, 9092, 2181, 50051, or 8080, modify the docker-compose.yml file to use different ports.

### Client Issues

- **Go version**: Ensure you're using Go 1.21 or later.
- **Order book not found**: Make sure you've created the order book before attempting to place orders.
- **Server not responding**: Check if the Matchingo server container is running.

### Viewing Server Logs

To check server logs:

```bash
PATH="/Applications/Docker.app/Contents/Resources/bin:$PATH" docker-compose logs -f server
```

This shows real-time logs from the server, helpful for debugging issues. 