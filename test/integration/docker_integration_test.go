package integration

import (
	"context"
	"testing"
	"time"

	"github.com/erain9/matchingo/pkg/api/proto"
	testutil "github.com/erain9/matchingo/test/utils"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDockerIntegration_FullFlow verifies a complete flow using Docker-provided Redis and Kafka
func TestDockerIntegration_FullFlow(t *testing.T) {
	testutil.RunIntegrationTest(t, func(redisAddr, kafkaAddr string) {
		// Create a Redis client to clean up any existing state
		redisClient := redis.NewClient(&redis.Options{
			Addr: redisAddr,
		})
		defer redisClient.Close()

		// Clean up any existing state
		err := redisClient.FlushAll(context.Background()).Err()
		require.NoError(t, err, "Failed to clean Redis state")

		// Create a real integration test setup with the Docker-provided services
		ctx := context.Background()

		// Set up gRPC client with real dependencies
		client, teardown := setupRealIntegrationTest(t, redisAddr, kafkaAddr)
		defer teardown()

		// Create a new order book
		bookName := "docker-integration-test"
		createResp, err := client.CreateOrderBook(ctx, &proto.CreateOrderBookRequest{
			Name:        bookName,
			BackendType: proto.BackendType_REDIS,
		})
		require.NoError(t, err, "Failed to create order book")
		assert.Equal(t, bookName, createResp.Name)

		// Create a buy limit order
		buyOrderID := "docker-buy-1"
		buyResp, err := client.CreateOrder(ctx, &proto.CreateOrderRequest{
			OrderBookName: bookName,
			OrderId:       buyOrderID,
			Side:          proto.OrderSide_BUY,
			OrderType:     proto.OrderType_LIMIT,
			Quantity:      "2.0",
			Price:         "100.0",
			TimeInForce:   proto.TimeInForce_GTC,
		})
		require.NoError(t, err, "Failed to create buy order")
		assert.Equal(t, proto.OrderStatus_OPEN, buyResp.Status)

		// Create a sell stop order which should be triggered immediately since there's no last price
		stopOrderID := "docker-stop-1"
		_, err = client.CreateOrder(ctx, &proto.CreateOrderRequest{
			OrderBookName: bookName,
			OrderId:       stopOrderID,
			Side:          proto.OrderSide_SELL,
			OrderType:     proto.OrderType_STOP_LIMIT,
			Quantity:      "1.0",
			Price:         "99.0",  // Limit price
			StopPrice:     "100.0", // Trigger price
			TimeInForce:   proto.TimeInForce_GTC,
		})
		require.NoError(t, err, "Failed to create stop order")

		// Allow some time for processing
		time.Sleep(200 * time.Millisecond)

		// Verify state - the stop order should have been triggered and matched
		stateResp, err := client.GetOrderBookState(ctx, &proto.GetOrderBookStateRequest{
			Name: bookName,
		})
		require.NoError(t, err, "Failed to get order book state")

		// The buy order should be partially filled (1.0 out of 2.0)
		require.Len(t, stateResp.Bids, 1, "Expected 1 bid level")
		assert.Equal(t, "100.000", stateResp.Bids[0].Price)
		assert.Equal(t, "1.000", stateResp.Bids[0].TotalQuantity) // 2.0 - 1.0 = 1.0

		// Create a market sell order to match the remaining buy
		marketOrderID := "docker-market-1"
		marketResp, err := client.CreateOrder(ctx, &proto.CreateOrderRequest{
			OrderBookName: bookName,
			OrderId:       marketOrderID,
			Side:          proto.OrderSide_SELL,
			OrderType:     proto.OrderType_MARKET,
			Quantity:      "1.0",
		})
		require.NoError(t, err, "Failed to create market order")
		assert.Equal(t, proto.OrderStatus_FILLED, marketResp.Status)

		// Verify final state - all orders should be matched
		finalStateResp, err := client.GetOrderBookState(ctx, &proto.GetOrderBookStateRequest{
			Name: bookName,
		})
		require.NoError(t, err, "Failed to get final order book state")

		// Book should be empty
		assert.Empty(t, finalStateResp.Bids, "Expected no bids")
		assert.Empty(t, finalStateResp.Asks, "Expected no asks")

		// Clean up - Delete the order book
		_, err = client.DeleteOrderBook(ctx, &proto.DeleteOrderBookRequest{
			Name: bookName,
		})
		require.NoError(t, err, "Failed to delete order book")
	})
}
