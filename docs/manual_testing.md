# Manual Testing Guide for Matchingo

This document provides step-by-step instructions for manually testing the Matchingo trading system.

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
docker-compose up -d
```

This starts:
- Redis (port 6379)
- Zookeeper (port 2181)
- Kafka (port 9092)
- Matchingo server (ports 50051 for gRPC, 8080 for HTTP)

### 2. Build the Components

Build the orderbook client and market maker:

```bash
# Build the order book client
go build -o bin/orderbook-client cmd/client/main.go

# Build the market maker
go build -o bin/marketmaker cmd/marketmaker/main.go
```

## Core Order Book Testing

### Create and Manage Order Books

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
./bin/orderbook-client create-order default BUY LIMIT 10.0 100.0 order1 0x1111111111111111111111111111111111111111
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
          Price|       Quantity|         Orders|         Address|Side
---------------|---------------|---------------|---------------|----
---------------|---------------|---------------|---------------|----
        100.000|         10.000|              1|0x1111111111111|BID
```

### Order Matching Scenarios

#### 6. Partial Order Match

Create a sell limit order that partially matches with the existing buy order:

```bash
./bin/orderbook-client create-order default SELL LIMIT 5.0 100.0 order2 0x2222222222222222222222222222222222222222
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
          Price|       Quantity|         Orders|         Address|Side
---------------|---------------|---------------|---------------|----
---------------|---------------|---------------|---------------|----
        100.000|          5.000|              1|0x1111111111111|BID
```

#### 9. Full Order Match

Create a market sell order to complete the match:

```bash
./bin/orderbook-client create-order default SELL MARKET 5.0 0.0 order3 0x3333333333333333333333333333333333333333
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
          Price|       Quantity|         Orders|         Address|Side
---------------|---------------|---------------|---------------|----
---------------|---------------|---------------|---------------|----
```

## Advanced Order Types

### Stop Limit Orders

#### 1. Create a Stop Limit Order

Create a stop limit order that will trigger when the market price reaches a specific level:

```bash
./bin/orderbook-client create-order default BUY STOP_LIMIT 5.0 95.0 order4 0x4444444444444444444444444444444444444444 --stop-price=90.0
```

#### 2. Trigger the Stop Order

Create a sell order at the trigger price:

```bash
./bin/orderbook-client create-order default SELL LIMIT 1.0 90.0 order5 0x5555555555555555555555555555555555555555
```

The stop order should be activated and converted to a limit order.

### Fill-or-Kill (FOK) Orders

#### 1. Create a Maker Order

Create a limit order on the buy side:

```bash
./bin/orderbook-client create-order default BUY LIMIT 10.0 100.0 order6 0x6666666666666666666666666666666666666666
```

#### 2. Test FOK Behavior

Create a FOK order that should be fully matched or canceled:

```bash
./bin/orderbook-client create-order default SELL LIMIT 15.0 100.0 order7 0x7777777777777777777777777777777777777777 --time-in-force=FOK
```

This order should be canceled as there's only 10.0 quantity available.

## Market Maker Testing

The market maker automatically creates and maintains a series of buy and sell orders at different price levels around a reference price.

### 1. Start a New Order Book

Create a dedicated order book for market maker testing:

```bash
./bin/orderbook-client create-book --name=market-maker-test --backend=memory
```

### 2. Configure the Market Maker

Create a configuration file for the market maker:

```bash
cat > market_maker_config.env << EOF
# gRPC connection settings
MATCHINGO_GRPC_ADDR=localhost:50051
REQUEST_TIMEOUT_MS=5000

# Market settings
MARKET_SYMBOL=market-maker-test
EXTERNAL_SYMBOL=BTCUSDT
PRICE_SOURCE_URL=https://api.binance.com

# Market making parameters
NUM_LEVELS=3
BASE_SPREAD_PERCENT=0.2
PRICE_STEP_PERCENT=0.1
ORDER_SIZE=1.0
UPDATE_INTERVAL_MS=5000
MARKET_MAKER_ID=mm-01

# HTTP client settings
HTTP_TIMEOUT_MS=5000
MAX_RETRIES=3
EOF
```

### 3. Start the Market Maker

Run the market maker using the configuration file:

```bash
export $(cat market_maker_config.env | xargs) && ./bin/marketmaker
```

You should see log output as the market maker starts up, connects to the price source, and begins placing orders.

### 4. Verify Order Placement

In a separate terminal, check the order book state:

```bash
./bin/orderbook-client get-state market-maker-test
```

You should see multiple bid and ask levels with orders placed by the market maker. For example:

```
 Price|Quantity|Orders|Side
---------------|---------------|---------------|----
         9980.0|          1.000|              1|BID
         9990.0|          1.000|              1|BID
        10000.0|          1.000|              1|BID
---------------|---------------|---------------|----
        10020.0|          1.000|              1|ASK
        10030.0|          1.000|              1|ASK
        10040.0|          1.000|              1|ASK
```

### 5. Observe Automatic Updates

The market maker periodically refreshes the orders based on current market prices. Watch the order book state change over time:

```bash
# Run this command repeatedly to see changes
./bin/orderbook-client get-state market-maker-test
```

You should notice the prices shifting as the reference price changes.

### 6. Test Order Matching with Market Maker

Create an order that matches with one of the market maker's orders:

```bash
# Find a price level from the current order book state
./bin/orderbook-client get-state market-maker-test

# Create a matching order at one of the ask prices (replace <price> and <address>)
./bin/orderbook-client create-order market-maker-test BUY LIMIT 1.0 <price> test-order1 0x8888888888888888888888888888888888888888
```

Observe that the market maker will replace the filled order on the next update cycle.

### 7. Stop the Market Maker

Stop the market maker by pressing Ctrl+C in its terminal. The market maker should gracefully shut down and cancel all of its outstanding orders.

Verify that all market maker orders are gone:

```bash
./bin/orderbook-client get-state market-maker-test
```

The order book should now be empty or contain only orders you placed manually.

## Monitoring Messages and Logs

### Monitoring Kafka Messages

The Matchingo system sends messages to Kafka whenever orders are processed, matched, or their state changes.

#### 1. Consume Messages from Kafka Topic

To view messages sent to Kafka in real-time:

```bash
# Using kafka-console-consumer
kafka-console-consumer --bootstrap-server localhost:9092 --topic test-msg-queue --from-beginning
```

#### 2. Message Patterns to Watch For

- **New Limit Order**: Message shows `Stored: true` with zero executed quantity
- **Filled Order**: Message shows `ExecutedQty` equal to the order quantity and `RemainingQty` of zero
- **Partial Fill**: Message shows non-zero values for both `ExecutedQty` and `RemainingQty`
- **Canceled Order**: The order ID appears in the `Canceled` array
- **Activated Stop Order**: The order ID appears in the `Activated` array

### Viewing Server Logs

To check server logs:

```bash
docker-compose logs -f server
```

This shows real-time logs from the server, helpful for debugging issues.

## Clean Up

When done testing, stop the Docker containers:

```bash
docker-compose down
```

## Production Deployment on Ubuntu

For production deployments on Ubuntu, follow these instructions to set up the Matchingo server and market maker.

### Server Deployment

#### 1. Install Prerequisites

Update the system and install required packages:

```bash
sudo apt update
sudo apt upgrade -y
sudo apt install -y git make build-essential
```

#### 2. Install Go 1.21+

```bash
wget https://go.dev/dl/go1.21.0.linux-amd64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.21.0.linux-amd64.tar.gz
rm go1.21.0.linux-amd64.tar.gz

# Add Go to PATH
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.profile
echo 'export PATH=$PATH:$HOME/go/bin' >> ~/.profile
source ~/.profile

# Verify installation
go version
```

#### 3. Clone and Build the Application

```bash
mkdir -p /opt/matchingo
cd /opt/matchingo
git clone https://github.com/erain9/matchingo.git .

# Build the server
go build -o bin/matchingo-server cmd/server/main.go

# Build the client
go build -o bin/orderbook-client cmd/client/main.go

# Build the market maker
go build -o bin/marketmaker cmd/marketmaker/main.go
```

#### 4. Install and Configure Dependencies

For production, install Redis and Kafka:

```bash
# Install Redis
sudo apt install -y redis-server
sudo systemctl enable redis-server
sudo systemctl start redis-server

# Install Kafka
sudo apt install -y default-jre
wget https://downloads.apache.org/kafka/3.6.0/kafka_2.13-3.6.0.tgz
tar -xzf kafka_2.13-3.6.0.tgz
sudo mv kafka_2.13-3.6.0 /opt/kafka
rm kafka_2.13-3.6.0.tgz
```

Create systemd service files for Zookeeper and Kafka (see deployment_ubuntu.md for details).

#### 5. Create and Configure Server Service

Create a systemd service for the Matchingo server:

```bash
sudo nano /etc/systemd/system/matchingo.service
```

Add the appropriate service configuration (see deployment_ubuntu.md for details).

```bash
sudo systemctl daemon-reload
sudo systemctl enable matchingo.service
sudo systemctl start matchingo.service
```

### Market Maker Deployment

#### 1. Create Market Maker Configuration

Create a configuration file for the market maker:

```bash
mkdir -p /opt/matchingo/config
nano /opt/matchingo/config/market_maker.env
```

Add the following content (adjust as needed):

```bash
# gRPC connection settings
MATCHINGO_GRPC_ADDR=localhost:50051
REQUEST_TIMEOUT_MS=5000

# Market settings
MARKET_SYMBOL=BTC-USD
EXTERNAL_SYMBOL=BTCUSDT
PRICE_SOURCE_URL=https://api.binance.com

# Market making parameters
NUM_LEVELS=5
BASE_SPREAD_PERCENT=0.5
PRICE_STEP_PERCENT=0.2
ORDER_SIZE=0.01
UPDATE_INTERVAL_MS=10000
MARKET_MAKER_ID=mm-prod-01

# HTTP client settings
HTTP_TIMEOUT_MS=5000
MAX_RETRIES=3
```

#### 2. Create Market Maker Service

Create a systemd service for the market maker:

```bash
sudo nano /etc/systemd/system/marketmaker.service
```

Add the following content:

```
[Unit]
Description=Matchingo Market Maker
After=network.target matchingo.service
Requires=matchingo.service

[Service]
User=ubuntu
Group=ubuntu
WorkingDirectory=/opt/matchingo
EnvironmentFile=/opt/matchingo/config/market_maker.env
ExecStart=/opt/matchingo/bin/marketmaker
Restart=on-failure
RestartSec=5

# Optional security enhancements
PrivateTmp=true
ProtectSystem=full
NoNewPrivileges=true

[Install]
WantedBy=multi-user.target
```

#### 3. Start the Market Maker Service

```bash
sudo systemctl daemon-reload
sudo systemctl enable marketmaker.service
sudo systemctl start marketmaker.service
```

#### 4. Monitor Market Maker Logs

```bash
sudo journalctl -u marketmaker.service -f
```

### Testing the Production Deployment

#### 1. Create an Order Book

Use the client to create an order book:

```bash
/opt/matchingo/bin/orderbook-client create-book --name=BTC-USD --backend=redis
```

#### 2. Monitor the Market Maker Activity

Check the order book state:

```bash
/opt/matchingo/bin/orderbook-client get-state BTC-USD
```

You should see the orders placed by the market maker at various price levels.

#### 3. Monitor Server and Market Maker Logs

```bash
# Server logs
sudo journalctl -u matchingo.service -f

# Market maker logs
sudo journalctl -u marketmaker.service -f
```

#### 4. Test Order Placement and Matching

Create orders that match with market maker orders:

```bash
/opt/matchingo/bin/orderbook-client create-order BTC-USD BUY LIMIT 0.01 <price> order-test-1 0x9999999999999999999999999999999999999999
```

Replace `<price>` with one of the ask prices from the order book state.

## Troubleshooting

### Connection Issues

- **Connection refused**: Ensure Docker is running and all services have started properly.
- **Port conflicts**: If you have other services using ports 6379, 9092, 2181, 50051, or 8080, modify the docker-compose.yml file to use different ports.

### Market Maker Issues

- **Price API errors**: If the market maker cannot fetch prices, check your internet connection and the `PRICE_SOURCE_URL` configuration.
- **Order placement failures**: Verify the server is running and the gRPC address is correct.
- **No orders appearing**: Check the market maker logs for errors and verify the correct order book name is being used.
- **User Address Required**: The `create-order` command now requires a user address as the last argument.

### Production Deployment Issues

- **Service won't start**: Check logs with `sudo journalctl -u matchingo.service -e` or `sudo journalctl -u marketmaker.service -e`.
- **Redis or Kafka connectivity**: Ensure services are running with `sudo systemctl status redis-server` and `sudo systemctl status kafka`.
- **Port availability**: Check that required ports are open with `sudo ss -tulpn | grep -E '9092|6379|50051|8080'`.
- **Firewall issues**: Configure UFW to allow required ports: `sudo ufw allow 50051/tcp` and `sudo ufw allow 8080/tcp`.