package server_test

import (
	"context"
	"testing"
	"time"

	"github.com/erain9/matchingo/pkg/api/proto"
	"github.com/erain9/matchingo/pkg/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMarketOrder_IOC verifies that IOC market orders are properly handled with partial fills
func TestMarketOrder_IOC(t *testing.T) {
	testutil.RunIntegrationTest(t, func(redisAddr, kafkaAddr string) {
		// Setup client
		client, teardown := setupRealIntegrationTest(t, redisAddr, kafkaAddr)
		defer teardown()

		ctx := context.Background()
		bookName := "market-order-test-book"

		// 1. Create Order Book using Redis backend
		_, err := client.CreateOrderBook(ctx, &proto.CreateOrderBookRequest{
			Name:        bookName,
			BackendType: proto.BackendType_REDIS,
		})
		require.NoError(t, err, "Failed to create order book")

		// 2. Place limit sell orders on the book at different price levels
		_, err = client.CreateOrder(ctx, &proto.CreateOrderRequest{
			OrderBookName: bookName,
			OrderId:       "limit-sell-1",
			Side:          proto.OrderSide_SELL,
			OrderType:     proto.OrderType_LIMIT,
			Quantity:      "10",
			Price:         "100",
			TimeInForce:   proto.TimeInForce_GTC,
		})
		require.NoError(t, err, "Failed to create limit sell 1")

		_, err = client.CreateOrder(ctx, &proto.CreateOrderRequest{
			OrderBookName: bookName,
			OrderId:       "limit-sell-2",
			Side:          proto.OrderSide_SELL,
			OrderType:     proto.OrderType_LIMIT,
			Quantity:      "15",
			Price:         "102",
			TimeInForce:   proto.TimeInForce_GTC,
		})
		require.NoError(t, err, "Failed to create limit sell 2")

		_, err = client.CreateOrder(ctx, &proto.CreateOrderRequest{
			OrderBookName: bookName,
			OrderId:       "limit-sell-3",
			Side:          proto.OrderSide_SELL,
			OrderType:     proto.OrderType_LIMIT,
			Quantity:      "5",
			Price:         "105",
			TimeInForce:   proto.TimeInForce_GTC,
		})
		require.NoError(t, err, "Failed to create limit sell 3")

		// Verify the book state - should have 3 sell orders
		stateResp, err := client.GetOrderBookState(ctx, &proto.GetOrderBookStateRequest{
			Name: bookName,
		})
		require.NoError(t, err, "Failed to get order book state")
		require.Len(t, stateResp.Asks, 3, "Expected 3 ask levels")
		assert.Empty(t, stateResp.Bids, "Expected no bids")

		// 3. Place a market buy order with IOC that should partially execute
		marketResp, err := client.CreateOrder(ctx, &proto.CreateOrderRequest{
			OrderBookName: bookName,
			OrderId:       "market-buy-1",
			Side:          proto.OrderSide_BUY,
			OrderType:     proto.OrderType_MARKET,
			Quantity:      "12", // Should match 10 from first level and 2 from second
			TimeInForce:   proto.TimeInForce_IOC,
		})
		require.NoError(t, err, "Failed to create market buy order")
		assert.Equal(t, proto.OrderStatus_FILLED, marketResp.Status)

		// Give some time for processing
		time.Sleep(200 * time.Millisecond)

		// 4. Check the book state after market order
		stateResp, err = client.GetOrderBookState(ctx, &proto.GetOrderBookStateRequest{
			Name: bookName,
		})
		require.NoError(t, err, "Failed to get updated order book state")

		// First sell level should be gone, second level partially filled
		require.Len(t, stateResp.Asks, 2, "Expected 2 ask levels")
		assert.Equal(t, "102.000", stateResp.Asks[0].Price)
		assert.Equal(t, "13.000", stateResp.Asks[0].TotalQuantity) // 15 - 2 = 13
		assert.Equal(t, "105.000", stateResp.Asks[1].Price)
		assert.Equal(t, "5.000", stateResp.Asks[1].TotalQuantity)

		// 5. Place another market buy order with IOC that exceeds available quantity
		marketResp2, err := client.CreateOrder(ctx, &proto.CreateOrderRequest{
			OrderBookName: bookName,
			OrderId:       "market-buy-2",
			Side:          proto.OrderSide_BUY,
			OrderType:     proto.OrderType_MARKET,
			Quantity:      "30", // More than available (18 units)
			TimeInForce:   proto.TimeInForce_IOC,
		})
		require.NoError(t, err, "Failed to create second market buy order")

		// Order should be partially filled and not resting on the book (IOC)
		assert.Equal(t, proto.OrderStatus_PARTIALLY_FILLED, marketResp2.Status)

		// Wait for processing
		time.Sleep(200 * time.Millisecond)

		// 6. Check final book state - should be empty
		finalStateResp, err := client.GetOrderBookState(ctx, &proto.GetOrderBookStateRequest{
			Name: bookName,
		})
		require.NoError(t, err, "Failed to get final order book state")
		assert.Empty(t, finalStateResp.Asks, "Expected no asks")
		assert.Empty(t, finalStateResp.Bids, "Expected no bids")

		// 7. Clean up
		_, err = client.DeleteOrderBook(ctx, &proto.DeleteOrderBookRequest{
			Name: bookName,
		})
		require.NoError(t, err, "Failed to delete order book")
	})
}
