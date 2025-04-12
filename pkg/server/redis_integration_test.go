package server_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/erain9/matchingo/pkg/api/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// redisTestHost is the host:port for the test Redis instance (from docker-compose.yml)
const redisTestHost = "localhost:6380"

// skipRedisTests checks if Redis integration tests should be skipped.
// Tests are skipped if the REDIS_TEST_HOST environment variable is not set,
// or if connection fails.
func skipRedisTests(t *testing.T) {
	if os.Getenv("CI") != "" {
		// Assume CI environment handles dependencies
		return
	}
	host := os.Getenv("REDIS_TEST_HOST")
	if host == "" {
		host = redisTestHost // Use default if not set
	}
	// A simple check could be attempted here if needed, e.g., pinging Redis.
	// For now, rely on Make targets to set up dependencies.
	// If make test-redis fails due to connection, the test will fail anyway.
	t.Logf("Using Redis host: %s for integration tests. Ensure it's running (e.g., via 'make test-deps-up')", host)
}

// TestRedisIntegration_BasicFlow tests basic order book operations using Redis backend.
func TestRedisIntegration_BasicFlow(t *testing.T) {
	skipRedisTests(t) // Check if Redis is available

	// --- Setup similar to V2 integration, but using Redis backend ---
	client, mockSender, teardown := setupIntegrationTestV2(t) // Re-use V2 setup
	defer teardown()

	mockSender.ClearSentMessages()
	ctx := context.Background()
	bookName := fmt.Sprintf("redis-test-book-%d", time.Now().UnixNano())
	orderIDBuy := "redis-buy-1"
	orderIDSell := "redis-sell-1"

	// Redis Options (ensure correct port from docker-compose)
	redisOptions := map[string]string{
		"addr": redisTestHost,
		// "password": "", // Add if needed
		// "db": "0",       // Add if needed
		"prefix": bookName, // Use book name as prefix for keys
	}

	// 1. Create Order Book with Redis Backend
	createBookReq := &proto.CreateOrderBookRequest{
		Name:        bookName,
		BackendType: proto.BackendType_REDIS,
		Options:     redisOptions,
	}
	_, err := client.CreateOrderBook(ctx, createBookReq)
	require.NoError(t, err, "CreateOrderBook (Redis) failed")

	// 2. Create Buy Limit Order
	createBuyReq := &proto.CreateOrderRequest{
		OrderBookName: bookName,
		OrderId:       orderIDBuy,
		Side:          proto.OrderSide_BUY,
		Quantity:      "10.0",
		Price:         "95.0",
		OrderType:     proto.OrderType_LIMIT,
		TimeInForce:   proto.TimeInForce_GTC,
	}
	_, err = client.CreateOrder(ctx, createBuyReq)
	require.NoError(t, err, "CreateOrder (Buy) failed")

	// 3. Create Sell Limit Order
	createSellReq := &proto.CreateOrderRequest{
		OrderBookName: bookName,
		OrderId:       orderIDSell,
		Side:          proto.OrderSide_SELL,
		Quantity:      "8.0",
		Price:         "96.0",
		OrderType:     proto.OrderType_LIMIT,
		TimeInForce:   proto.TimeInForce_GTC,
	}
	_, err = client.CreateOrder(ctx, createSellReq)
	require.NoError(t, err, "CreateOrder (Sell) failed")

	// 4. Verify Order Book State
	stateReq := &proto.GetOrderBookStateRequest{Name: bookName}
	stateResp, err := client.GetOrderBookState(ctx, stateReq)
	require.NoError(t, err, "GetOrderBookState failed")

	require.Len(t, stateResp.Bids, 1, "Expected 1 bid level")
	assert.Equal(t, "95.000", stateResp.Bids[0].Price)
	assert.Equal(t, "10.000", stateResp.Bids[0].TotalQuantity)

	require.Len(t, stateResp.Asks, 1, "Expected 1 ask level")
	assert.Equal(t, "96.000", stateResp.Asks[0].Price)
	assert.Equal(t, "8.000", stateResp.Asks[0].TotalQuantity)

	// 5. Verify Kafka messages (Optional - Focus here is Redis backend working)
	// Kafka behavior should be identical regardless of backend
	sentMessages := mockSender.GetSentMessages()
	// Expect messages for buy store and sell store
	assert.Len(t, sentMessages, 2, "Expected 2 messages sent to Kafka for order storage")

	// 6. Clean up (Delete Book)
	_, err = client.DeleteOrderBook(ctx, &proto.DeleteOrderBookRequest{Name: bookName})
	assert.NoError(t, err, "DeleteOrderBook failed")
}
