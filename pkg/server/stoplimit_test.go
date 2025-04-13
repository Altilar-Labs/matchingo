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

// TestStopLimit_OrderTriggering verifies that stop limit orders are properly triggered
func TestStopLimit_OrderTriggering(t *testing.T) {
	testutil.RunIntegrationTest(t, func(redisAddr, kafkaAddr string) {
		// Setup client
		client, teardown := setupRealIntegrationTest(t, redisAddr, kafkaAddr)
		defer teardown()

		ctx := context.Background()
		bookName := "stoplimit-test-book"

		// 1. Create Order Book using Redis backend
		_, err := client.CreateOrderBook(ctx, &proto.CreateOrderBookRequest{
			Name:        bookName,
			BackendType: proto.BackendType_REDIS,
		})
		require.NoError(t, err, "Failed to create order book")

		// 2. Place a buy STOP LIMIT order (100 @ $50, stop price $55)
		// This should not be triggered because there's no last trade price yet
		buyStopOrderID := "buy-stop-1"
		_, err = client.CreateOrder(ctx, &proto.CreateOrderRequest{
			OrderBookName: bookName,
			OrderId:       buyStopOrderID,
			Side:          proto.OrderSide_BUY,
			OrderType:     proto.OrderType_STOP_LIMIT,
			Quantity:      "100",
			Price:         "50", // Limit price
			StopPrice:     "55", // Trigger when price rises to or above $55
			TimeInForce:   proto.TimeInForce_GTC,
		})
		require.NoError(t, err, "Failed to create buy stop order")

		// 3. Place a sell STOP LIMIT order (100 @ $70, stop price $65)
		// This should not be triggered because there's no last trade price yet
		sellStopOrderID := "sell-stop-1"
		_, err = client.CreateOrder(ctx, &proto.CreateOrderRequest{
			OrderBookName: bookName,
			OrderId:       sellStopOrderID,
			Side:          proto.OrderSide_SELL,
			OrderType:     proto.OrderType_STOP_LIMIT,
			Quantity:      "100",
			Price:         "70", // Limit price
			StopPrice:     "65", // Trigger when price falls to or below $65
			TimeInForce:   proto.TimeInForce_GTC,
		})
		require.NoError(t, err, "Failed to create sell stop order")

		// Verify the book state - no visible orders yet because they're stop orders
		stateResp, err := client.GetOrderBookState(ctx, &proto.GetOrderBookStateRequest{
			Name: bookName,
		})
		require.NoError(t, err, "Failed to get order book state")
		assert.Empty(t, stateResp.Bids, "Expected no visible bids (stop order)")
		assert.Empty(t, stateResp.Asks, "Expected no visible asks (stop order)")

		// 4. Create a regular trade to set the last trade price to $60
		// This should trigger the buy stop order but not the sell stop order
		_, err = client.CreateOrder(ctx, &proto.CreateOrderRequest{
			OrderBookName: bookName,
			OrderId:       "regular-buy-1",
			Side:          proto.OrderSide_BUY,
			OrderType:     proto.OrderType_LIMIT,
			Quantity:      "10",
			Price:         "60",
			TimeInForce:   proto.TimeInForce_GTC,
		})
		require.NoError(t, err, "Failed to create regular buy")

		_, err = client.CreateOrder(ctx, &proto.CreateOrderRequest{
			OrderBookName: bookName,
			OrderId:       "regular-sell-1",
			Side:          proto.OrderSide_SELL,
			OrderType:     proto.OrderType_LIMIT,
			Quantity:      "10",
			Price:         "60",
			TimeInForce:   proto.TimeInForce_GTC,
		})
		require.NoError(t, err, "Failed to create regular sell")

		// Give some time for the system to process and trigger the stop order
		time.Sleep(200 * time.Millisecond)

		// 5. Check the book state again - the buy stop should be visible now as a limit order
		stateResp, err = client.GetOrderBookState(ctx, &proto.GetOrderBookStateRequest{
			Name: bookName,
		})
		require.NoError(t, err, "Failed to get updated order book state")
		require.Len(t, stateResp.Bids, 1, "Expected 1 bid level (triggered stop order)")
		assert.Equal(t, "50.000", stateResp.Bids[0].Price)
		assert.Equal(t, "100.000", stateResp.Bids[0].TotalQuantity)
		assert.Empty(t, stateResp.Asks, "Expected no asks (sell stop not triggered)")

		// 6. Now create a trade at $62 which still shouldn't trigger the sell stop
		_, err = client.CreateOrder(ctx, &proto.CreateOrderRequest{
			OrderBookName: bookName,
			OrderId:       "regular-buy-2",
			Side:          proto.OrderSide_BUY,
			OrderType:     proto.OrderType_LIMIT,
			Quantity:      "5",
			Price:         "62",
			TimeInForce:   proto.TimeInForce_GTC,
		})
		require.NoError(t, err, "Failed to create second regular buy")

		_, err = client.CreateOrder(ctx, &proto.CreateOrderRequest{
			OrderBookName: bookName,
			OrderId:       "regular-sell-2",
			Side:          proto.OrderSide_SELL,
			OrderType:     proto.OrderType_LIMIT,
			Quantity:      "5",
			Price:         "62",
			TimeInForce:   proto.TimeInForce_GTC,
		})
		require.NoError(t, err, "Failed to create second regular sell")

		// Wait for processing
		time.Sleep(200 * time.Millisecond)

		// 7. Book state should still have only the buy stop as a limit order
		stateResp, err = client.GetOrderBookState(ctx, &proto.GetOrderBookStateRequest{
			Name: bookName,
		})
		require.NoError(t, err, "Failed to get order book state after second trade")
		require.Len(t, stateResp.Bids, 1, "Expected 1 bid level (triggered stop order)")
		assert.Empty(t, stateResp.Asks, "Expected no asks (sell stop still not triggered)")

		// 8. Finally, create a trade at $63 which should trigger the sell stop
		_, err = client.CreateOrder(ctx, &proto.CreateOrderRequest{
			OrderBookName: bookName,
			OrderId:       "regular-buy-3",
			Side:          proto.OrderSide_BUY,
			OrderType:     proto.OrderType_LIMIT,
			Quantity:      "5",
			Price:         "63",
			TimeInForce:   proto.TimeInForce_GTC,
		})
		require.NoError(t, err, "Failed to create third regular buy")

		_, err = client.CreateOrder(ctx, &proto.CreateOrderRequest{
			OrderBookName: bookName,
			OrderId:       "regular-sell-3",
			Side:          proto.OrderSide_SELL,
			OrderType:     proto.OrderType_LIMIT,
			Quantity:      "5",
			Price:         "63",
			TimeInForce:   proto.TimeInForce_GTC,
		})
		require.NoError(t, err, "Failed to create third regular sell")

		// Wait for processing
		time.Sleep(200 * time.Millisecond)

		// 9. Final book state should have both buy and sell orders
		stateResp, err = client.GetOrderBookState(ctx, &proto.GetOrderBookStateRequest{
			Name: bookName,
		})
		require.NoError(t, err, "Failed to get final order book state")
		require.Len(t, stateResp.Bids, 1, "Expected 1 bid level")
		assert.Equal(t, "50.000", stateResp.Bids[0].Price)

		require.Len(t, stateResp.Asks, 1, "Expected 1 ask level (triggered sell stop)")
		assert.Equal(t, "70.000", stateResp.Asks[0].Price)
		assert.Equal(t, "100.000", stateResp.Asks[0].TotalQuantity)

		// Clean up
		_, err = client.DeleteOrderBook(ctx, &proto.DeleteOrderBookRequest{
			Name: bookName,
		})
		require.NoError(t, err, "Failed to delete order book")
	})
}
