package server

import (
	"context"
	"testing"

	"github.com/erain9/matchingo/pkg/api/proto"
	pkgotel "github.com/erain9/matchingo/pkg/otel"
	"github.com/nikolaydubina/fpdecimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestGRPCOrderBookService(t *testing.T) {
	// Initialize OpenTelemetry for testing
	tp := trace.NewTracerProvider()
	otel.SetTracerProvider(tp)

	// Register a test tracer
	testTracer := tp.Tracer("test-tracer")

	// Ensure we reset OpenTelemetry after the test
	defer func() {
		pkgotel.ResetForTesting()
		tp.Shutdown(context.Background())
	}()

	// Initialize test tracers
	err := pkgotel.InitForTesting(testTracer)
	require.NoError(t, err, "Failed to initialize OpenTelemetry for testing")

	// Create a test context with a valid span
	ctx := context.Background()
	ctx, span := testTracer.Start(ctx, "test")
	defer span.End()

	// Create a test manager
	manager := NewOrderBookManager()
	defer manager.Close()

	// Create gRPC service
	service := NewGRPCOrderBookService(manager)

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

	// Test creating a duplicate order book
	t.Run("CreateOrderBook_Duplicate", func(t *testing.T) {
		req := &proto.CreateOrderBookRequest{
			Name:        "test-book", // Already exists from previous test
			BackendType: proto.BackendType_MEMORY,
		}

		_, err := service.CreateOrderBook(ctx, req)
		if err == nil {
			t.Fatal("Expected an error when creating a duplicate order book, got nil")
		}

		// Check for gRPC status code
		if st, ok := status.FromError(err); !ok || st.Code() != codes.AlreadyExists {
			t.Errorf("Expected gRPC code AlreadyExists, got %v (error: %v)", st.Code(), err)
		}
	})

	// Test getting a non-existent order book
	t.Run("GetOrderBook_NotFound", func(t *testing.T) {
		req := &proto.GetOrderBookRequest{
			Name: "non-existent-book",
		}

		_, err := service.GetOrderBook(ctx, req)
		if err == nil {
			t.Fatal("Expected an error when getting a non-existent order book, got nil")
		}

		// Check for gRPC status code
		if st, ok := status.FromError(err); !ok || st.Code() != codes.NotFound {
			t.Errorf("Expected gRPC code NotFound, got %v (error: %v)", st.Code(), err)
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

	// Test creating an order with invalid quantity
	t.Run("CreateOrder_InvalidQuantity", func(t *testing.T) {
		req := &proto.CreateOrderRequest{
			OrderBookName: "test-book",
			OrderId:       "invalid-qty-order",
			Side:          proto.OrderSide_BUY,
			Quantity:      "not-a-number", // Invalid decimal string
			Price:         "100.0",
			OrderType:     proto.OrderType_LIMIT,
			TimeInForce:   proto.TimeInForce_GTC,
		}

		_, err := service.CreateOrder(ctx, req)
		if err == nil {
			t.Fatal("Expected an error for invalid quantity, got nil")
		}

		// Check for gRPC status code
		if st, ok := status.FromError(err); !ok || st.Code() != codes.InvalidArgument {
			t.Errorf("Expected gRPC code InvalidArgument for quantity, got %v (error: %v)", st.Code(), err)
		}
	})

	// Test creating an order with invalid price
	t.Run("CreateOrder_InvalidPrice", func(t *testing.T) {
		req := &proto.CreateOrderRequest{
			OrderBookName: "test-book",
			OrderId:       "invalid-price-order",
			Side:          proto.OrderSide_BUY,
			Quantity:      "1.0",
			Price:         "not-a-price", // Invalid decimal string
			OrderType:     proto.OrderType_LIMIT,
			TimeInForce:   proto.TimeInForce_GTC,
		}

		_, err := service.CreateOrder(ctx, req)
		if err == nil {
			t.Fatal("Expected an error for invalid price, got nil")
		}

		// Check for gRPC status code
		if st, ok := status.FromError(err); !ok || st.Code() != codes.InvalidArgument {
			t.Errorf("Expected gRPC code InvalidArgument for price, got %v (error: %v)", st.Code(), err)
		}
	})

	// Test creating order with invalid quantity (zero - should be caught by core panic, but gRPC might return InvalidArgument)
	t.Run("CreateOrder_ZeroQuantity", func(t *testing.T) {
		// This might ideally be InvalidArgument, but core panics. Test for robustness.
		req := &proto.CreateOrderRequest{
			OrderBookName: "test-book",
			OrderId:       "zero-qty-order",
			Side:          proto.OrderSide_BUY,
			Quantity:      "0.0", // Invalid
			Price:         "100.0",
			OrderType:     proto.OrderType_LIMIT,
		}
		_, err := service.CreateOrder(ctx, req)
		assert.Error(t, err, "Expected an error for zero quantity")
		// We might get Internal if the panic isn't caught, or InvalidArgument if parsing fails first
		if st, ok := status.FromError(err); ok {
			assert.Condition(t, func() bool { return st.Code() == codes.InvalidArgument || st.Code() == codes.Internal }, "Expected InvalidArgument or Internal, got %v", st.Code())
		} else {
			t.Errorf("Expected gRPC status error, got %T: %v", err, err)
		}
	})

	// Test creating order with duplicate ID
	t.Run("CreateOrder_DuplicateID", func(t *testing.T) {
		// First, ensure an order exists
		firstReq := &proto.CreateOrderRequest{
			OrderBookName: "test-book",
			OrderId:       "duplicate-id-test",
			Side:          proto.OrderSide_BUY,
			Quantity:      "1.0",
			Price:         "101.0",
			OrderType:     proto.OrderType_LIMIT,
		}
		_, err := service.CreateOrder(ctx, firstReq)
		require.NoError(t, err, "Setup for duplicate ID test failed")

		// Attempt to create another with the same ID
		dupReq := &proto.CreateOrderRequest{
			OrderBookName: "test-book",
			OrderId:       "duplicate-id-test", // Same ID
			Side:          proto.OrderSide_SELL,
			Quantity:      "1.0",
			Price:         "102.0",
			OrderType:     proto.OrderType_LIMIT,
		}
		_, err = service.CreateOrder(ctx, dupReq)
		require.Error(t, err, "Expected an error for duplicate order ID")
		if st, ok := status.FromError(err); !ok || st.Code() != codes.AlreadyExists {
			t.Errorf("Expected gRPC code AlreadyExists, got %v (error: %v)", st.Code(), err)
		}
	})

	// Test GetOrder for non-existent order
	t.Run("GetOrder_NotFound", func(t *testing.T) {
		req := &proto.GetOrderRequest{
			OrderBookName: "test-book",
			OrderId:       "non-existent-order-id",
		}
		_, err := service.GetOrder(ctx, req)
		require.Error(t, err, "Expected an error getting non-existent order")
		if st, ok := status.FromError(err); !ok || st.Code() != codes.NotFound {
			t.Errorf("Expected gRPC code NotFound, got %v (error: %v)", st.Code(), err)
		}
	})

	// Test CancelOrder for non-existent order
	t.Run("CancelOrder_NotFound", func(t *testing.T) {
		req := &proto.CancelOrderRequest{
			OrderBookName: "test-book",
			OrderId:       "non-existent-order-id-cancel",
		}
		_, err := service.CancelOrder(ctx, req)
		require.Error(t, err, "Expected an error canceling non-existent order")
		if st, ok := status.FromError(err); !ok || st.Code() != codes.NotFound {
			t.Errorf("Expected gRPC code NotFound, got %v (error: %v)", st.Code(), err)
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

		// Verify there are bids in the book from previous test cases
		if len(resp.Bids) == 0 {
			t.Errorf("Expected at least one bid, got none")
		}

		// We expect 0 asks since we haven't created any sell orders
		if len(resp.Asks) != 0 {
			t.Errorf("Expected 0 asks, got %d", len(resp.Asks))
		}

		// Verify the bid has proper format
		if resp.Bids[0].Price == "" {
			t.Errorf("Expected non-empty bid price, got empty string")
		}

		if resp.Bids[0].TotalQuantity == "" {
			t.Errorf("Expected non-empty bid quantity, got empty string")
		}

		if resp.Bids[0].OrderCount < 1 {
			t.Errorf("Expected bid order count to be at least 1, got %d", resp.Bids[0].OrderCount)
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

	// Test DeleteOrderBook for non-existent book
	t.Run("DeleteOrderBook_NotFound", func(t *testing.T) {
		req := &proto.DeleteOrderBookRequest{
			Name: "non-existent-book-delete",
		}
		_, err := service.DeleteOrderBook(ctx, req)
		require.Error(t, err, "Expected an error deleting non-existent book")
		if st, ok := status.FromError(err); !ok || st.Code() != codes.NotFound {
			t.Errorf("Expected gRPC code NotFound, got %v (error: %v)", st.Code(), err)
		}
	})

	// Test CancelOrder for an already processed (e.g., filled/canceled) order - Expect NotFound
	t.Run("CancelOrder_AlreadyProcessed", func(t *testing.T) {
		// Setup: Create and cancel an order first
		setupOrderReq := &proto.CreateOrderRequest{
			OrderBookName: "test-book",
			OrderId:       "already-processed-order",
			Side:          proto.OrderSide_BUY,
			Quantity:      "1.0", Price: "1.0", OrderType: proto.OrderType_LIMIT,
		}
		_, err := service.CreateOrder(ctx, setupOrderReq)
		require.NoError(t, err, "Setup: CreateOrder failed")
		_, err = service.CancelOrder(ctx, &proto.CancelOrderRequest{OrderBookName: "test-book", OrderId: "already-processed-order"})
		require.NoError(t, err, "Setup: CancelOrder failed")

		// Attempt to cancel again
		req := &proto.CancelOrderRequest{
			OrderBookName: "test-book",
			OrderId:       "already-processed-order",
		}
		_, err = service.CancelOrder(ctx, req)
		require.Error(t, err, "Expected an error canceling already processed order")
		if st, ok := status.FromError(err); !ok || st.Code() != codes.NotFound {
			t.Errorf("Expected gRPC code NotFound for already processed order, got %v (error: %v)", st.Code(), err)
		}
	})

	// Test creating order with invalid type (enum value out of range)
	t.Run("CreateOrder_InvalidType", func(t *testing.T) {
		req := &proto.CreateOrderRequest{
			OrderBookName: "test-book",
			OrderId:       "invalid-type-order",
			Side:          proto.OrderSide_BUY,
			Quantity:      "1.0",
			Price:         "100.0",
			OrderType:     proto.OrderType(999), // Invalid enum value
		}
		_, err := service.CreateOrder(ctx, req)
		require.Error(t, err, "Expected an error for invalid order type")
		if st, ok := status.FromError(err); !ok || st.Code() != codes.InvalidArgument {
			t.Errorf("Expected gRPC code InvalidArgument for type, got %v (error: %v)", st.Code(), err)
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
