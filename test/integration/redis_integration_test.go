package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/erain9/matchingo/pkg/api/proto"
	testutil "github.com/erain9/matchingo/test/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRedisIntegration_BasicFlow verifies basic Redis backend operations
func TestRedisIntegration_BasicFlow(t *testing.T) {
	// Use our new Docker utility to automatically start Redis and Kafka
	testutil.RunIntegrationTest(t, func(redisAddr, kafkaAddr string) {
		client, teardown := setupRealIntegrationTest(t, redisAddr, kafkaAddr)
		defer teardown()

		// Use a unique book name with timestamp to avoid conflicts with previous test runs
		bookName := fmt.Sprintf("redis-test-book-%d", time.Now().UnixNano())
		ctx := context.Background()

		// Create order book with Redis backend using the Docker container address
		createResp, err := client.CreateOrderBook(ctx, &proto.CreateOrderBookRequest{
			Name:        bookName,
			BackendType: proto.BackendType_REDIS,
			Options:     map[string]string{"redis_addr": redisAddr},
		})
		require.NoError(t, err, "CreateOrderBook (Redis) failed")
		assert.Equal(t, bookName, createResp.Name)

		// Create a buy order
		orderResp, err := client.CreateOrder(ctx, &proto.CreateOrderRequest{
			OrderBookName: bookName,
			OrderId:       "redis-test-buy-1",
			Side:          proto.OrderSide_BUY,
			OrderType:     proto.OrderType_LIMIT,
			Quantity:      "1.5",
			Price:         "100.0",
		})
		require.NoError(t, err, "Failed to create order")
		assert.Equal(t, proto.OrderStatus_OPEN, orderResp.Status)

		// Create a sell order (should match)
		sellResp, err := client.CreateOrder(ctx, &proto.CreateOrderRequest{
			OrderBookName: bookName,
			OrderId:       "redis-test-sell-1",
			Side:          proto.OrderSide_SELL,
			OrderType:     proto.OrderType_LIMIT,
			Quantity:      "1.0",
			Price:         "100.0",
		})
		require.NoError(t, err, "Failed to create sell order")
		assert.Equal(t, proto.OrderStatus_FILLED, sellResp.Status)

		// Verify state
		stateResp, err := client.GetOrderBookState(ctx, &proto.GetOrderBookStateRequest{
			Name: bookName,
		})
		require.NoError(t, err)

		// After matching, the buy order should be partially filled and still on the book
		require.Len(t, stateResp.Bids, 1, "Expected 1 bid level")
		assert.Equal(t, "100.000", stateResp.Bids[0].Price)
		assert.Equal(t, "0.500", stateResp.Bids[0].TotalQuantity) // 1.5 - 1.0 = 0.5

		// Ask side should be empty
		assert.Empty(t, stateResp.Asks, "Expected no asks")
	})
}
