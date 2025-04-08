package server

import (
	"context"
	"testing"

	"github.com/erain9/matchingo/pkg/api/proto"
	"github.com/nikolaydubina/fpdecimal"
)

func TestGRPCOrderBookService(t *testing.T) {
	// Create a test manager
	manager := NewOrderBookManager()
	defer manager.Close()

	// Create gRPC service
	service := NewGRPCOrderBookService(manager)

	// Create a test context
	ctx := context.Background()

	// Test creating an order book
	t.Run("CreateOrderBook", func(t *testing.T) {
		req := &proto.CreateOrderBookRequest{
			Name:        "test-book",
			BackendType: proto.BackendType_MEMORY,
		}

		resp, err := service.CreateOrderBook(ctx, req)
		if err != nil {
			t.Fatalf("Failed to create order book: %v", err)
		}

		if resp.Name != "test-book" {
			t.Errorf("Expected name 'test-book', got '%s'", resp.Name)
		}

		if resp.BackendType != proto.BackendType_MEMORY {
			t.Errorf("Expected backend type MEMORY, got %s", resp.BackendType)
		}

		if resp.OrderCount != 0 {
			t.Errorf("Expected order count 0, got %d", resp.OrderCount)
		}

		if resp.CreatedAt == nil {
			t.Error("Created timestamp should not be nil")
		}
	})

	// Test getting an order book
	t.Run("GetOrderBook", func(t *testing.T) {
		req := &proto.GetOrderBookRequest{
			Name: "test-book",
		}

		resp, err := service.GetOrderBook(ctx, req)
		if err != nil {
			t.Fatalf("Failed to get order book: %v", err)
		}

		if resp.Name != "test-book" {
			t.Errorf("Expected name 'test-book', got '%s'", resp.Name)
		}

		if resp.BackendType != proto.BackendType_MEMORY {
			t.Errorf("Expected backend type MEMORY, got %s", resp.BackendType)
		}
	})

	// Test listing order books
	t.Run("ListOrderBooks", func(t *testing.T) {
		req := &proto.ListOrderBooksRequest{
			Limit:  10,
			Offset: 0,
		}

		resp, err := service.ListOrderBooks(ctx, req)
		if err != nil {
			t.Fatalf("Failed to list order books: %v", err)
		}

		if resp.Total < 1 {
			t.Errorf("Expected at least 1 order book, got %d", resp.Total)
		}

		if len(resp.OrderBooks) < 1 {
			t.Errorf("Expected at least 1 order book in response, got %d", len(resp.OrderBooks))
		}

		// Check if our test-book is in the list
		foundTestBook := false
		for _, book := range resp.OrderBooks {
			if book.Name == "test-book" {
				foundTestBook = true
				break
			}
		}

		if !foundTestBook {
			t.Error("Could not find 'test-book' in the list of order books")
		}
	})

	// Test creating an order
	t.Run("CreateOrder", func(t *testing.T) {
		req := &proto.CreateOrderRequest{
			OrderBookName: "test-book",
			OrderId:       "test-order",
			Side:          proto.OrderSide_BUY,
			Quantity:      "1.0",
			Price:         "100.0",
			OrderType:     proto.OrderType_LIMIT,
			TimeInForce:   proto.TimeInForce_GTC,
		}

		resp, err := service.CreateOrder(ctx, req)
		if err != nil {
			t.Fatalf("Failed to create order: %v", err)
		}

		if resp.OrderId != "test-order" {
			t.Errorf("Expected order ID 'test-order', got '%s'", resp.OrderId)
		}

		if resp.OrderBookName != "test-book" {
			t.Errorf("Expected order book name 'test-book', got '%s'", resp.OrderBookName)
		}

		if resp.Side != proto.OrderSide_BUY {
			t.Errorf("Expected side BUY, got %s", resp.Side)
		}

		if resp.Quantity != "1.0" {
			t.Errorf("Expected quantity '1.0', got '%s'", resp.Quantity)
		}

		if resp.Price != "100.0" {
			t.Errorf("Expected price '100.0', got '%s'", resp.Price)
		}

		if resp.Status != proto.OrderStatus_OPEN {
			t.Errorf("Expected status OPEN, got %s", resp.Status)
		}
	})

	// Test getting an order
	t.Run("GetOrder", func(t *testing.T) {
		req := &proto.GetOrderRequest{
			OrderBookName: "test-book",
			OrderId:       "test-order",
		}

		resp, err := service.GetOrder(ctx, req)
		if err != nil {
			t.Fatalf("Failed to get order: %v", err)
		}

		if resp.OrderId != "test-order" {
			t.Errorf("Expected order ID 'test-order', got '%s'", resp.OrderId)
		}

		if resp.OrderBookName != "test-book" {
			t.Errorf("Expected order book name 'test-book', got '%s'", resp.OrderBookName)
		}

		if resp.Side != proto.OrderSide_BUY {
			t.Errorf("Expected side BUY, got %s", resp.Side)
		}

		// We don't check the exact quantity as it might be formatted differently (e.g., "1.0" vs "1.000")
		qty, err := fpdecimal.FromString(resp.Quantity)
		if err != nil {
			t.Errorf("Invalid quantity format: %s", resp.Quantity)
		}
		expectedQty, _ := fpdecimal.FromString("1.0")
		if !qty.Equal(expectedQty) {
			t.Errorf("Expected quantity equal to 1.0, got '%s'", resp.Quantity)
		}
	})

	// Test getting order book state
	t.Run("GetOrderBookState", func(t *testing.T) {
		req := &proto.GetOrderBookStateRequest{
			Name:  "test-book",
			Depth: 10,
		}

		resp, err := service.GetOrderBookState(ctx, req)
		if err != nil {
			t.Fatalf("Failed to get order book state: %v", err)
		}

		if resp.Name != "test-book" {
			t.Errorf("Expected name 'test-book', got '%s'", resp.Name)
		}

		// We expect 1 bid since we created a buy order earlier
		if len(resp.Bids) != 1 {
			t.Errorf("Expected 1 bid, got %d", len(resp.Bids))
		}

		// We expect 0 asks since we haven't created any sell orders
		if len(resp.Asks) != 0 {
			t.Errorf("Expected 0 asks, got %d", len(resp.Asks))
		}

		// Verify the bid details
		if resp.Bids[0].Price != "100.000" {
			t.Errorf("Expected bid price '100.000', got '%s'", resp.Bids[0].Price)
		}

		if resp.Bids[0].TotalQuantity != "1.000" {
			t.Errorf("Expected bid quantity '1.000', got '%s'", resp.Bids[0].TotalQuantity)
		}

		if resp.Bids[0].OrderCount != 1 {
			t.Errorf("Expected bid order count 1, got %d", resp.Bids[0].OrderCount)
		}
	})

	// Test canceling an order
	t.Run("CancelOrder", func(t *testing.T) {
		req := &proto.CancelOrderRequest{
			OrderBookName: "test-book",
			OrderId:       "test-order",
		}

		_, err := service.CancelOrder(ctx, req)
		if err != nil {
			t.Fatalf("Failed to cancel order: %v", err)
		}

		// Verify the order was canceled by trying to get it
		getReq := &proto.GetOrderRequest{
			OrderBookName: "test-book",
			OrderId:       "test-order",
		}

		// In our implementation, canceled orders are removed entirely
		// so we expect an error here
		_, err = service.GetOrder(ctx, getReq)
		if err == nil {
			t.Fatal("Expected error when getting canceled order, got none")
		}
	})

	// Test deleting an order book (do this last)
	t.Run("DeleteOrderBook", func(t *testing.T) {
		req := &proto.DeleteOrderBookRequest{
			Name: "test-book",
		}

		_, err := service.DeleteOrderBook(ctx, req)
		if err != nil {
			t.Fatalf("Failed to delete order book: %v", err)
		}

		// Verify the order book was deleted by trying to get it
		getReq := &proto.GetOrderBookRequest{
			Name: "test-book",
		}

		_, err = service.GetOrderBook(ctx, getReq)
		if err == nil {
			t.Fatal("Expected error when getting deleted order book, got none")
		}
	})
}
