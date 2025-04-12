package server_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/erain9/matchingo/pkg/api/proto"
	"github.com/erain9/matchingo/pkg/core"
	"github.com/erain9/matchingo/pkg/messaging"
	"github.com/erain9/matchingo/pkg/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

const bufSizeV2 = 1024 * 1024

// setupIntegrationTestV2 starts an in-process gRPC server with a mock message sender.
// Uses a separate listener to avoid conflicts with other tests.
func setupIntegrationTestV2(tb testing.TB) (proto.OrderBookServiceClient, *messaging.MockMessageSender, func()) {
	tb.Helper()

	lisV2 := bufconn.Listen(bufSizeV2)
	grpcServer := grpc.NewServer()

	// Create and inject mock sender factory
	mockSenderV2 := messaging.NewMockMessageSender()
	core.SetMessageSenderFactory(func() messaging.MessageSender { return mockSenderV2 })

	// Create manager (will now use the factory set above when creating order books)
	manager := server.NewOrderBookManager()

	orderBookService := server.NewGRPCOrderBookService(manager)
	proto.RegisterOrderBookServiceServer(grpcServer, orderBookService)

	go func() {
		if err := grpcServer.Serve(lisV2); err != nil {
			// Ignore benign errors on close
			if err != grpc.ErrServerStopped && err.Error() != "listener closed" { // Check specific error string for bufconn
				tb.Logf("V2 Server exited with error: %v", err)
			}
		}
	}()

	// Create client connection
	ctx, cancelCtx := context.WithTimeout(context.Background(), 5*time.Second)
	conn, err := grpc.DialContext(ctx,
		"bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lisV2.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	cancelCtx() // Cancel timeout context once connected or failed
	require.NoError(tb, err, "Failed to dial bufnet for V2")

	client := proto.NewOrderBookServiceClient(conn)

	// Teardown function
	teardown := func() {
		core.SetMessageSenderFactory(nil) // Reset sender factory to default
		require.NoError(tb, conn.Close())
		grpcServer.Stop()
		_ = lisV2.Close() // Ignore error on close, might already be closed
	}

	return client, mockSenderV2, teardown
}

// --- Test Cases --- //

// TestIntegrationV2_BasicLimitOrder verifies creating a book, adding a limit order,
// and checking the state and mock sender.
func TestIntegrationV2_BasicLimitOrder(t *testing.T) {
	client, mockSender, teardown := setupIntegrationTestV2(t)
	defer teardown()

	mockSender.ClearSentMessages() // Ensure clean state for this test

	ctx := context.Background()
	bookName := "integ-test-book-v2-1"
	orderID := "limit-order-v2-1"

	// 1. Create Order Book
	createBookReq := &proto.CreateOrderBookRequest{
		Name:        bookName,
		BackendType: proto.BackendType_MEMORY,
	}
	_, err := client.CreateOrderBook(ctx, createBookReq)
	require.NoError(t, err, "CreateOrderBook failed")

	// 2. Create Limit Order (will rest)
	createOrderReq := &proto.CreateOrderRequest{
		OrderBookName: bookName,
		OrderId:       orderID,
		Side:          proto.OrderSide_BUY,
		Quantity:      "10.0",
		Price:         "95.0",
		OrderType:     proto.OrderType_LIMIT,
		TimeInForce:   proto.TimeInForce_GTC,
	}
	orderResp, err := client.CreateOrder(ctx, createOrderReq)
	require.NoError(t, err, "CreateOrder failed")
	assert.Equal(t, orderID, orderResp.OrderId)
	assert.Equal(t, proto.OrderStatus_OPEN, orderResp.Status) // Order should be open

	// 3. Verify Order Book State
	stateReq := &proto.GetOrderBookStateRequest{Name: bookName}
	stateResp, err := client.GetOrderBookState(ctx, stateReq)
	require.NoError(t, err, "GetOrderBookState failed")
	require.Len(t, stateResp.Bids, 1, "Expected 1 bid level")
	assert.Equal(t, "95.000", stateResp.Bids[0].Price)
	assert.Equal(t, "10.000", stateResp.Bids[0].TotalQuantity)
	assert.Equal(t, int32(1), stateResp.Bids[0].OrderCount)
	assert.Empty(t, stateResp.Asks, "Expected no asks")

	// 4. Verify Mock Sender (should have received message for the stored order)
	sentMessages := mockSender.GetSentMessages()
	require.Len(t, sentMessages, 1, "Expected 1 message sent to Kafka")
	msg := sentMessages[0]
	assert.Equal(t, orderID, msg.OrderID)
	assert.True(t, msg.Stored, "Expected Stored flag to be true")
	// The message includes the taker order itself, but no other trades when just stored
	require.Len(t, msg.Trades, 1, "Expected 1 trade entry (taker) for resting order")
	takerTrade := msg.Trades[0]
	assert.Equal(t, orderID, takerTrade.OrderID)
	assert.Equal(t, "10.000", msg.Quantity)     // Original quantity
	assert.Equal(t, "10.000", msg.RemainingQty) // Remaining quantity
	assert.Equal(t, "0.000", msg.ExecutedQty)   // Executed quantity
}

// TestIntegrationV2_LimitOrderMatch verifies order matching and Kafka messages.
func TestIntegrationV2_LimitOrderMatch(t *testing.T) {
	client, mockSender, teardown := setupIntegrationTestV2(t)
	defer teardown()

	mockSender.ClearSentMessages()
	ctx := context.Background()
	bookName := "integ-test-book-v2-match"
	sellOrderID := "sell-match-1"
	buyOrderID := "buy-match-1"

	// 1. Create Book
	_, err := client.CreateOrderBook(ctx, &proto.CreateOrderBookRequest{Name: bookName, BackendType: proto.BackendType_MEMORY})
	require.NoError(t, err)

	// 2. Create initial Sell Limit Order (Maker)
	_, err = client.CreateOrder(ctx, &proto.CreateOrderRequest{
		OrderBookName: bookName,
		OrderId:       sellOrderID,
		Side:          proto.OrderSide_SELL,
		Quantity:      "5.0",
		Price:         "100.0",
		OrderType:     proto.OrderType_LIMIT,
		TimeInForce:   proto.TimeInForce_GTC,
	})
	require.NoError(t, err)
	mockSender.ClearSentMessages() // Clear message from initial order storage

	// 3. Create matching Buy Limit Order (Taker)
	buyResp, err := client.CreateOrder(ctx, &proto.CreateOrderRequest{
		OrderBookName: bookName,
		OrderId:       buyOrderID,
		Side:          proto.OrderSide_BUY,
		Quantity:      "3.0",   // Partial match
		Price:         "100.0", // Exact price match
		OrderType:     proto.OrderType_LIMIT,
		TimeInForce:   proto.TimeInForce_GTC,
	})
	require.NoError(t, err)
	assert.Equal(t, proto.OrderStatus_FILLED, buyResp.Status) // Buy order fully filled

	// 4. Verify Order Book State (Sell order should be partially filled)
	stateResp, err := client.GetOrderBookState(ctx, &proto.GetOrderBookStateRequest{Name: bookName})
	require.NoError(t, err)
	assert.Empty(t, stateResp.Bids, "Expected no bids after match")
	require.Len(t, stateResp.Asks, 1, "Expected 1 ask level remaining")
	assert.Equal(t, "100.000", stateResp.Asks[0].Price)
	assert.Equal(t, "2.000", stateResp.Asks[0].TotalQuantity) // 5.0 - 3.0 = 2.0
	assert.Equal(t, int32(1), stateResp.Asks[0].OrderCount)

	// 5. Verify Mock Sender (should have message for the taker order execution)
	sentMessages := mockSender.GetSentMessages()
	require.Len(t, sentMessages, 1, "Expected 1 message sent for the taker execution")
	msg := sentMessages[0]
	assert.Equal(t, buyOrderID, msg.OrderID) // Message relates to the taker order
	assert.False(t, msg.Stored, "Taker order was not stored")
	assert.Equal(t, "3.000", msg.Quantity)     // Original taker quantity
	assert.Equal(t, "0.000", msg.RemainingQty) // Taker fully filled
	assert.Equal(t, "3.000", msg.ExecutedQty)

	// Verify trade details within the message
	require.Len(t, msg.Trades, 2, "Expected 2 trade entries: taker and maker")
	takerTrade := msg.Trades[0]
	makerTrade := msg.Trades[1]
	assert.Equal(t, buyOrderID, takerTrade.OrderID)
	assert.Equal(t, sellOrderID, makerTrade.OrderID)
	assert.Equal(t, "3.000", makerTrade.Quantity) // Matched quantity
	assert.Equal(t, "100.000", makerTrade.Price)  // Matched price
}

// TestIntegrationV2_MarketOrderMatch verifies market order matching and Kafka messages.
func TestIntegrationV2_MarketOrderMatch(t *testing.T) {
	client, mockSender, teardown := setupIntegrationTestV2(t)
	defer teardown()

	mockSender.ClearSentMessages()
	ctx := context.Background()
	bookName := "integ-test-book-v2-market"
	sellOrderID := "sell-market-match-1"
	buyOrderID := "buy-market-match-1"

	// 1. Create Book
	_, err := client.CreateOrderBook(ctx, &proto.CreateOrderBookRequest{Name: bookName, BackendType: proto.BackendType_MEMORY})
	require.NoError(t, err)

	// 2. Create initial Sell Limit Order (Maker)
	_, err = client.CreateOrder(ctx, &proto.CreateOrderRequest{
		OrderBookName: bookName,
		OrderId:       sellOrderID,
		Side:          proto.OrderSide_SELL,
		Quantity:      "5.0",
		Price:         "100.0",
		OrderType:     proto.OrderType_LIMIT,
		TimeInForce:   proto.TimeInForce_GTC,
	})
	require.NoError(t, err)
	mockSender.ClearSentMessages() // Clear message from initial order storage

	// 3. Create matching Buy Market Order (Taker)
	buyResp, err := client.CreateOrder(ctx, &proto.CreateOrderRequest{
		OrderBookName: bookName,
		OrderId:       buyOrderID,
		Side:          proto.OrderSide_BUY,
		Quantity:      "3.0", // Partial match against sell order
		Price:         "0.0", // Market orders have price 0
		OrderType:     proto.OrderType_MARKET,
		// TIF usually ignored for Market, defaults might apply
	})
	require.NoError(t, err)
	// Market orders are immediately filled or killed, not left open.
	// Assuming FILLED status is returned even if partially filled against liquidity.
	assert.Equal(t, proto.OrderStatus_FILLED, buyResp.Status)

	// 4. Verify Order Book State (Sell order should be partially filled)
	stateResp, err := client.GetOrderBookState(ctx, &proto.GetOrderBookStateRequest{Name: bookName})
	require.NoError(t, err)
	assert.Empty(t, stateResp.Bids, "Expected no bids after market order")
	require.Len(t, stateResp.Asks, 1, "Expected 1 ask level remaining")
	assert.Equal(t, "100.000", stateResp.Asks[0].Price)
	assert.Equal(t, "2.000", stateResp.Asks[0].TotalQuantity) // 5.0 - 3.0 = 2.0
	assert.Equal(t, int32(1), stateResp.Asks[0].OrderCount)

	// 5. Verify Mock Sender (should have message for the taker market order execution)
	sentMessages := mockSender.GetSentMessages()
	require.Len(t, sentMessages, 1, "Expected 1 message sent for the market taker execution")
	msg := sentMessages[0]
	assert.Equal(t, buyOrderID, msg.OrderID) // Message relates to the taker order
	assert.False(t, msg.Stored, "Market order was not stored")
	assert.Equal(t, "3.000", msg.Quantity)     // Original taker quantity
	assert.Equal(t, "0.000", msg.RemainingQty) // Market orders always fully processed or canceled (0 left)
	assert.Equal(t, "3.000", msg.ExecutedQty)

	// Verify trade details within the message
	require.Len(t, msg.Trades, 2, "Expected 2 trade entries: taker and maker")
	takerTrade := msg.Trades[0]
	makerTrade := msg.Trades[1]
	assert.Equal(t, buyOrderID, takerTrade.OrderID)
	assert.Equal(t, sellOrderID, makerTrade.OrderID)
	assert.Equal(t, "3.000", makerTrade.Quantity) // Matched quantity
	assert.Equal(t, "100.000", makerTrade.Price)  // Matched price (maker's price)
}

// TestIntegrationV2_CancelOrder verifies canceling an order and checks Kafka messages.
func TestIntegrationV2_CancelOrder(t *testing.T) {
	client, mockSender, teardown := setupIntegrationTestV2(t)
	defer teardown()

	mockSender.ClearSentMessages()
	ctx := context.Background()
	bookName := "integ-test-book-v2-cancel"
	orderID := "cancel-order-1"

	// 1. Create Book
	_, err := client.CreateOrderBook(ctx, &proto.CreateOrderBookRequest{Name: bookName, BackendType: proto.BackendType_MEMORY})
	require.NoError(t, err)

	// 2. Create a resting Limit Order
	_, err = client.CreateOrder(ctx, &proto.CreateOrderRequest{
		OrderBookName: bookName,
		OrderId:       orderID,
		Side:          proto.OrderSide_BUY,
		Quantity:      "5.0",
		Price:         "99.0",
		OrderType:     proto.OrderType_LIMIT,
		TimeInForce:   proto.TimeInForce_GTC,
	})
	require.NoError(t, err)
	mockSender.ClearSentMessages() // Clear message from initial order storage

	// 3. Verify order exists via GetOrder
	getResp, err := client.GetOrder(ctx, &proto.GetOrderRequest{OrderBookName: bookName, OrderId: orderID})
	require.NoError(t, err, "GetOrder before cancel failed")
	require.Equal(t, proto.OrderStatus_OPEN, getResp.Status)

	// 4. Cancel the order
	_, err = client.CancelOrder(ctx, &proto.CancelOrderRequest{OrderBookName: bookName, OrderId: orderID})
	require.NoError(t, err, "CancelOrder failed")

	// 5. Verify order is gone via GetOrder (expect NotFound)
	_, err = client.GetOrder(ctx, &proto.GetOrderRequest{OrderBookName: bookName, OrderId: orderID})
	require.Error(t, err, "Expected error getting canceled order")
	if st, ok := status.FromError(err); !ok || st.Code() != codes.NotFound {
		t.Errorf("Expected gRPC code NotFound getting canceled order, got %v (error: %v)", st.Code(), err)
	}

	// 6. Verify Order Book State (should be empty)
	stateResp, err := client.GetOrderBookState(ctx, &proto.GetOrderBookStateRequest{Name: bookName})
	require.NoError(t, err)
	assert.Empty(t, stateResp.Bids, "Expected no bids after cancel")
	assert.Empty(t, stateResp.Asks, "Expected no asks")

	// 7. Verify Mock Sender (Cancellation itself might not send a message in current core logic)
	sentMessages := mockSender.GetSentMessages()
	// TODO: Adjust assertion based on whether core.CancelOrder is modified to send Kafka messages.
	// For now, assume it does NOT send a message directly upon cancellation.
	assert.Empty(t, sentMessages, "Expected no Kafka message sent directly from CancelOrder")
}

// TestIntegrationV2_IOC_FOK verifies ImmediateOrCancel and FillOrKill TIF logic.
func TestIntegrationV2_IOC_FOK(t *testing.T) {
	client, mockSender, teardown := setupIntegrationTestV2(t)
	defer teardown()

	ctx := context.Background()
	bookName := "integ-test-book-v2-tif"

	// 1. Create Book
	_, err := client.CreateOrderBook(ctx, &proto.CreateOrderBookRequest{Name: bookName, BackendType: proto.BackendType_MEMORY})
	require.NoError(t, err)

	// 2. Add initial liquidity (Sell 5 @ 100)
	_, err = client.CreateOrder(ctx, &proto.CreateOrderRequest{
		OrderBookName: bookName,
		OrderId:       "sell-liq-1",
		Side:          proto.OrderSide_SELL,
		Quantity:      "5.0",
		Price:         "100.0",
		OrderType:     proto.OrderType_LIMIT,
		TimeInForce:   proto.TimeInForce_GTC,
	})
	require.NoError(t, err)

	// --- IOC Test ---
	t.Run("IOC_PartialFill", func(t *testing.T) {
		mockSender.ClearSentMessages()
		buyOrderID := "buy-ioc-1"

		// Create IOC Buy Order for 10.0 (more than available)
		buyResp, err := client.CreateOrder(ctx, &proto.CreateOrderRequest{
			OrderBookName: bookName,
			OrderId:       buyOrderID,
			Side:          proto.OrderSide_BUY,
			Quantity:      "10.0",
			Price:         "100.0", // Match price
			OrderType:     proto.OrderType_LIMIT,
			TimeInForce:   proto.TimeInForce_IOC,
		})
		require.NoError(t, err)
		assert.Equal(t, proto.OrderStatus_FILLED, buyResp.Status) // IOC orders that fill anything are considered FILLED

		// Verify book state (Asks should be empty now)
		stateResp, err := client.GetOrderBookState(ctx, &proto.GetOrderBookStateRequest{Name: bookName})
		require.NoError(t, err)
		assert.Empty(t, stateResp.Asks, "IOC: Expected asks to be empty after partial fill")
		assert.Empty(t, stateResp.Bids, "IOC: Expected bids to remain empty")

		// Verify Mock Sender
		sentMessages := mockSender.GetSentMessages()
		require.Len(t, sentMessages, 1, "IOC: Expected 1 message")
		msg := sentMessages[0]
		assert.Equal(t, buyOrderID, msg.OrderID)
		assert.Equal(t, "10.000", msg.Quantity)    // Original Qty
		assert.Equal(t, "5.000", msg.ExecutedQty)  // Filled Qty
		assert.Equal(t, "5.000", msg.RemainingQty) // Remaining Qty (which was canceled by IOC)
		assert.False(t, msg.Stored)
		require.Len(t, msg.Trades, 2, "IOC: Expected 2 trades (taker+maker)")
		assert.Equal(t, "sell-liq-1", msg.Trades[1].OrderID)
		assert.Equal(t, "5.000", msg.Trades[1].Quantity)
	})

	// --- FOK Test ---
	t.Run("FOK_Fail", func(t *testing.T) {
		// Re-add liquidity (Sell 5 @ 101) - use different ID and price
		_, err = client.CreateOrder(ctx, &proto.CreateOrderRequest{
			OrderBookName: bookName,
			OrderId:       "sell-liq-2",
			Side:          proto.OrderSide_SELL,
			Quantity:      "5.0",
			Price:         "101.0",
			OrderType:     proto.OrderType_LIMIT,
			TimeInForce:   proto.TimeInForce_GTC,
		})
		require.NoError(t, err)
		mockSender.ClearSentMessages()
		buyOrderID := "buy-fok-1"

		// Create FOK Buy Order for 10.0 (more than available at 101)
		buyResp, err := client.CreateOrder(ctx, &proto.CreateOrderRequest{
			OrderBookName: bookName,
			OrderId:       buyOrderID,
			Side:          proto.OrderSide_BUY,
			Quantity:      "10.0",
			Price:         "101.0", // Match price
			OrderType:     proto.OrderType_LIMIT,
			TimeInForce:   proto.TimeInForce_FOK,
		})
		require.NoError(t, err)
		// FOK orders that cannot be fully filled are typically just CANCELED
		assert.Equal(t, proto.OrderStatus_CANCELED, buyResp.Status)

		// Verify book state (Asks should be unchanged)
		stateResp, err := client.GetOrderBookState(ctx, &proto.GetOrderBookStateRequest{Name: bookName})
		require.NoError(t, err)
		require.Len(t, stateResp.Asks, 1, "FOK: Expected asks to be unchanged")
		assert.Equal(t, "101.000", stateResp.Asks[0].Price)
		assert.Equal(t, "5.000", stateResp.Asks[0].TotalQuantity)

		// Verify Mock Sender
		sentMessages := mockSender.GetSentMessages()
		require.Len(t, sentMessages, 1, "FOK: Expected 1 message")
		msg := sentMessages[0]
		assert.Equal(t, buyOrderID, msg.OrderID)
		assert.Equal(t, "10.000", msg.Quantity)     // Original Qty
		assert.Equal(t, "0.000", msg.ExecutedQty)   // Filled Qty = 0
		assert.Equal(t, "10.000", msg.RemainingQty) // Remaining Qty (which was canceled by FOK)
		assert.False(t, msg.Stored)
		assert.Len(t, msg.Trades, 1, "FOK: Expected only taker trade entry") // Only taker, no maker matches
	})

	t.Run("FOK_Success", func(t *testing.T) {
		mockSender.ClearSentMessages()
		buyOrderID := "buy-fok-2"

		// Create FOK Buy Order for 5.0 (exactly available at 101)
		buyResp, err := client.CreateOrder(ctx, &proto.CreateOrderRequest{
			OrderBookName: bookName,
			OrderId:       buyOrderID,
			Side:          proto.OrderSide_BUY,
			Quantity:      "5.0",
			Price:         "101.0", // Match price
			OrderType:     proto.OrderType_LIMIT,
			TimeInForce:   proto.TimeInForce_FOK,
		})
		require.NoError(t, err)
		assert.Equal(t, proto.OrderStatus_FILLED, buyResp.Status)

		// Verify book state (Asks should be empty)
		stateResp, err := client.GetOrderBookState(ctx, &proto.GetOrderBookStateRequest{Name: bookName})
		require.NoError(t, err)
		assert.Empty(t, stateResp.Asks, "FOK Success: Expected asks to be empty")

		// Verify Mock Sender
		sentMessages := mockSender.GetSentMessages()
		require.Len(t, sentMessages, 1, "FOK Success: Expected 1 message")
		msg := sentMessages[0]
		assert.Equal(t, buyOrderID, msg.OrderID)
		assert.Equal(t, "5.000", msg.Quantity)     // Original Qty
		assert.Equal(t, "5.000", msg.ExecutedQty)  // Filled Qty
		assert.Equal(t, "0.000", msg.RemainingQty) // Remaining Qty
		assert.False(t, msg.Stored)
		assert.Len(t, msg.Trades, 2, "FOK Success: Expected taker and maker trade entries")
		assert.Equal(t, "sell-liq-2", msg.Trades[1].OrderID)
		assert.Equal(t, "5.000", msg.Trades[1].Quantity)
	})
}

// TestIntegrationV2_StopLimit verifies stop-limit order placement, activation, and matching.
func TestIntegrationV2_StopLimit(t *testing.T) {
	client, mockSender, teardown := setupIntegrationTestV2(t)
	defer teardown()

	mockSender.ClearSentMessages()
	ctx := context.Background()
	bookName := "integ-test-book-v2-stop"
	stopBuyID := "stop-buy-1"
	triggerSellID := "trigger-sell-1"
	fillSellID := "fill-sell-1"

	// 1. Create Book
	_, err := client.CreateOrderBook(ctx, &proto.CreateOrderBookRequest{Name: bookName, BackendType: proto.BackendType_MEMORY})
	require.NoError(t, err)

	// 2. Place Buy Stop-Limit Order (Stop = 105, Limit = 104)
	stopResp, err := client.CreateOrder(ctx, &proto.CreateOrderRequest{
		OrderBookName: bookName,
		OrderId:       stopBuyID,
		Side:          proto.OrderSide_BUY,
		Quantity:      "10.0",
		Price:         "104.0", // Limit price
		OrderType:     proto.OrderType_STOP_LIMIT,
		StopPrice:     "105.0", // Stop trigger price
		// TimeInForce:   proto.TimeInForce_GTC, // Implicit GTC for stop?
	})
	require.NoError(t, err, "Failed to place stop order")
	// Stop orders are accepted but not immediately open
	// The closest status in proto is PENDING.
	assert.Equal(t, proto.OrderStatus_PENDING, stopResp.Status)

	// Verify state (stop order shouldn't be on book)
	stateResp1, err := client.GetOrderBookState(ctx, &proto.GetOrderBookStateRequest{Name: bookName})
	require.NoError(t, err)
	assert.Empty(t, stateResp1.Bids, "Stop order should not be on bids yet")
	assert.Empty(t, stateResp1.Asks, "Stop order should not be on asks yet")

	// Verify Kafka message for stop order storage
	sentStopMsgs := mockSender.GetSentMessages()
	require.Len(t, sentStopMsgs, 1, "Expected 1 message for stop order placement")
	stopMsg := sentStopMsgs[0]
	assert.Equal(t, stopBuyID, stopMsg.OrderID)
	assert.True(t, stopMsg.Stored, "Stop order message should indicate stored")
	assert.Equal(t, "10.000", stopMsg.Quantity)
	assert.Equal(t, "10.000", stopMsg.RemainingQty)
	assert.Equal(t, "0.000", stopMsg.ExecutedQty)
	mockSender.ClearSentMessages()

	// 3. Place a Sell Order to Trigger the Stop (Sell @ 105)
	_, err = client.CreateOrder(ctx, &proto.CreateOrderRequest{
		OrderBookName: bookName,
		OrderId:       triggerSellID,
		Side:          proto.OrderSide_SELL,
		Quantity:      "1.0", // Small quantity just to trigger
		Price:         "105.0",
		OrderType:     proto.OrderType_LIMIT,
		TimeInForce:   proto.TimeInForce_GTC,
	})
	require.NoError(t, err, "Failed to place trigger sell order")

	// Verify state (trigger sell should be on asks, stop should now be on bids)
	// Need a small delay or check mechanism for stop activation if it's async,
	// but assuming it's synchronous within Process for now.
	stateResp2, err := client.GetOrderBookState(ctx, &proto.GetOrderBookStateRequest{Name: bookName})
	require.NoError(t, err)
	require.Len(t, stateResp2.Asks, 1, "Expected trigger sell on asks")
	assert.Equal(t, "105.000", stateResp2.Asks[0].Price)
	require.Len(t, stateResp2.Bids, 1, "Expected activated stop order on bids")
	assert.Equal(t, "104.000", stateResp2.Bids[0].Price) // Limit price of the stop order
	assert.Equal(t, "10.000", stateResp2.Bids[0].TotalQuantity)

	// Verify Kafka messages (trigger sell store + stop activation?)
	sentTriggerMsgs := mockSender.GetSentMessages()
	// TODO: Verify exact messages. Expect trigger sell storage. Maybe stop activation?
	// For now, check there's at least one message.
	require.GreaterOrEqual(t, len(sentTriggerMsgs), 1, "Expected messages for trigger sell process")
	mockSender.ClearSentMessages()

	// 4. Place another Sell Order to Match the Activated Stop Order (Sell @ 104)
	fillResp, err := client.CreateOrder(ctx, &proto.CreateOrderRequest{
		OrderBookName: bookName,
		OrderId:       fillSellID,
		Side:          proto.OrderSide_SELL,
		Quantity:      "7.0", // Partial fill of the activated stop order
		Price:         "104.0",
		OrderType:     proto.OrderType_LIMIT,
		TimeInForce:   proto.TimeInForce_GTC,
	})
	require.NoError(t, err, "Failed to place fill sell order")
	assert.Equal(t, proto.OrderStatus_FILLED, fillResp.Status)

	// Verify state (stop order partially filled)
	stateResp3, err := client.GetOrderBookState(ctx, &proto.GetOrderBookStateRequest{Name: bookName})
	require.NoError(t, err)
	require.Len(t, stateResp3.Asks, 1, "Expected trigger sell still on asks") // Trigger sell untouched
	assert.Equal(t, "105.000", stateResp3.Asks[0].Price)
	require.Len(t, stateResp3.Bids, 1, "Expected partially filled stop order on bids")
	assert.Equal(t, "104.000", stateResp3.Bids[0].Price)
	assert.Equal(t, "3.000", stateResp3.Bids[0].TotalQuantity) // 10.0 - 7.0 = 3.0

	// Verify Kafka message for the fill sell order
	sentFillMsgs := mockSender.GetSentMessages()
	require.Len(t, sentFillMsgs, 1, "Expected 1 message for fill sell execution")
	fillMsg := sentFillMsgs[0]
	assert.Equal(t, fillSellID, fillMsg.OrderID)
	assert.False(t, fillMsg.Stored)
	assert.Equal(t, "7.000", fillMsg.ExecutedQty)
	assert.Equal(t, "0.000", fillMsg.RemainingQty)
	require.Len(t, fillMsg.Trades, 2)
	assert.Equal(t, stopBuyID, fillMsg.Trades[1].OrderID) // Matched against stop order
	assert.Equal(t, "7.000", fillMsg.Trades[1].Quantity)
	assert.Equal(t, "104.000", fillMsg.Trades[1].Price)
}

// TODO: Add more integration tests V2:
