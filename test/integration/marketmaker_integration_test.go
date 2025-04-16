package integration

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/erain9/matchingo/pkg/api/proto"
	"github.com/erain9/matchingo/pkg/marketmaker"
	testutil "github.com/erain9/matchingo/test/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/emptypb"
)

// MockPriceFetcher implements the marketmaker.PriceFetcher interface for testing
type MockPriceFetcher struct {
	prices []float64
	index  int
}

func NewMockPriceFetcher(prices []float64) *MockPriceFetcher {
	return &MockPriceFetcher{
		prices: prices,
		index:  0,
	}
}

func (m *MockPriceFetcher) FetchPrice(ctx context.Context) (float64, error) {
	if m.index >= len(m.prices) {
		m.index = 0 // wrap around if we've gone through all prices
	}
	price := m.prices[m.index]
	m.index++
	return price, nil
}

func (m *MockPriceFetcher) Close() error {
	return nil
}

// TestMarketMakerIntegration tests the integration between the market maker and the order book service
func TestMarketMakerIntegration(t *testing.T) {
	// Setup integration test with in-memory bufconn
	client, mockSender, teardown := setupIntegrationTestV2(t)
	defer teardown()

	mockSender.ClearSentMessages() // Ensure clean state

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Setup logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	// Create a test order book
	bookName := "mm-test-book"
	_, err := client.CreateOrderBook(ctx, &proto.CreateOrderBookRequest{
		Name:        bookName,
		BackendType: proto.BackendType_MEMORY,
	})
	require.NoError(t, err, "Failed to create order book")

	// Create the market maker configuration
	mmConfig := &marketmaker.Config{
		MatchingoGRPCAddr: "bufnet", // Will be overridden in the test
		RequestTimeout:    5 * time.Second,
		MarketSymbol:      bookName,
		ExternalSymbol:    "BTCUSDT",
		PriceSourceURL:    "https://api.binance.com",
		NumLevels:         3,
		BaseSpreadPercent: 0.1,
		PriceStepPercent:  0.05,
		OrderSize:         "1.0",
		UpdateInterval:    1 * time.Second,
		MarketMakerID:     "test-mm-01",
		HTTPTimeout:       5 * time.Second,
		MaxRetries:        3,
	}

	// Create a mock price fetcher that returns predictable prices
	mockPriceFetcher := NewMockPriceFetcher([]float64{10000.0, 10050.0, 10100.0})

	// Create a direct client order placer using our test client
	orderPlacer := &testOrderPlacer{
		client: client,
		logger: logger,
	}

	// Create a market maker strategy
	strategy := marketmaker.NewLayeredSymmetricQuoting(mmConfig, logger)

	// Create the market maker with our test components
	mm, err := marketmaker.NewMarketMaker(mmConfig, logger, orderPlacer, mockPriceFetcher, strategy)
	require.NoError(t, err, "Failed to create market maker")

	// Run subtests
	t.Run("MarketMakerLifecycle", func(t *testing.T) {
		// Start the market maker
		err = mm.Start(ctx)
		require.NoError(t, err, "Failed to start market maker")

		// Wait for at least one update cycle
		time.Sleep(2 * time.Second)

		// Get the order book state
		stateResp, err := client.GetOrderBookState(ctx, &proto.GetOrderBookStateRequest{
			Name: bookName,
		})
		require.NoError(t, err, "Failed to get order book state")

		// Verify orders were placed
		t.Logf("Order book state: %d bid levels, %d ask levels", len(stateResp.Bids), len(stateResp.Asks))

		// There should be orders on both sides
		assert.Greater(t, len(stateResp.Bids), 0, "Expected at least one bid")
		assert.Greater(t, len(stateResp.Asks), 0, "Expected at least one ask")

		// Verify bid/ask ordering
		if len(stateResp.Bids) > 1 {
			for i := 1; i < len(stateResp.Bids); i++ {
				assert.Less(t, stateResp.Bids[i].Price, stateResp.Bids[i-1].Price, "Bids should be in descending price order")
			}
		}
		if len(stateResp.Asks) > 1 {
			for i := 1; i < len(stateResp.Asks); i++ {
				assert.Greater(t, stateResp.Asks[i].Price, stateResp.Asks[i-1].Price, "Asks should be in ascending price order")
			}
		}

		// Verify the total order counts at each level
		for _, level := range stateResp.Bids {
			assert.Greater(t, level.OrderCount, int32(0), "Expected at least one order at bid price level %s", level.Price)
		}
		for _, level := range stateResp.Asks {
			assert.Greater(t, level.OrderCount, int32(0), "Expected at least one order at ask price level %s", level.Price)
		}

		// Wait for another update cycle to test order replacement
		time.Sleep(2 * time.Second)

		// Get the order book state again
		stateResp2, err := client.GetOrderBookState(ctx, &proto.GetOrderBookStateRequest{
			Name: bookName,
		})
		require.NoError(t, err, "Failed to get order book state after update")

		// Verify orders were updated
		t.Logf("Updated order book state: %d bid levels, %d ask levels", len(stateResp2.Bids), len(stateResp2.Asks))

		// Check that there are still orders on both sides
		assert.Greater(t, len(stateResp2.Bids), 0, "Expected at least one bid after update")
		assert.Greater(t, len(stateResp2.Asks), 0, "Expected at least one ask after update")

		// Stop the market maker
		err = mm.Stop(ctx)
		require.NoError(t, err, "Failed to stop market maker")

		// Wait for cancellations to process
		time.Sleep(1 * time.Second)

		// Verify all orders were cancelled
		stateResp3, err := client.GetOrderBookState(ctx, &proto.GetOrderBookStateRequest{
			Name: bookName,
		})
		require.NoError(t, err, "Failed to get order book state after stopping")

		// After stopping, there should be no orders left
		orderCount := 0
		for _, level := range stateResp3.Bids {
			orderCount += int(level.OrderCount)
		}
		for _, level := range stateResp3.Asks {
			orderCount += int(level.OrderCount)
		}

		assert.Equal(t, 0, orderCount, "Expected all market maker orders to be cancelled")
	})
}

// testOrderPlacer implements the marketmaker.OrderPlacer interface using a gRPC client
type testOrderPlacer struct {
	client proto.OrderBookServiceClient
	logger *slog.Logger
}

func (p *testOrderPlacer) CreateOrder(ctx context.Context, req *proto.CreateOrderRequest) (*proto.OrderResponse, error) {
	p.logger.Debug("Sending CreateOrder request",
		"order_book", req.OrderBookName,
		"order_id", req.OrderId,
		"side", req.Side,
		"type", req.OrderType,
		"qty", req.Quantity,
		"price", req.Price)

	return p.client.CreateOrder(ctx, req)
}

func (p *testOrderPlacer) CancelOrder(ctx context.Context, req *proto.CancelOrderRequest) (*emptypb.Empty, error) {
	p.logger.Debug("Sending CancelOrder request",
		"order_book", req.OrderBookName,
		"order_id", req.OrderId)

	return p.client.CancelOrder(ctx, req)
}

func (p *testOrderPlacer) Close() error {
	// No need to close anything since we're using the test client
	return nil
}

// TestMarketMakerWithRealDependencies tests the market maker integration with real Redis and Kafka
func TestMarketMakerWithRealDependencies(t *testing.T) {
	testutil.RunIntegrationTest(t, func(redisAddr, kafkaAddr string) {
		// Setup client with real dependencies
		client, teardown := setupRealIntegrationTest(t, redisAddr, kafkaAddr)
		defer teardown()

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Setup logger
		logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}))

		// Create a test order book
		bookName := "mm-redis-test"
		_, err := client.CreateOrderBook(ctx, &proto.CreateOrderBookRequest{
			Name:        bookName,
			BackendType: proto.BackendType_REDIS, // Use Redis backend
		})
		require.NoError(t, err, "Failed to create order book")

		// Create the market maker configuration
		mmConfig := &marketmaker.Config{
			MatchingoGRPCAddr: "bufnet", // Will be overridden by the test client
			RequestTimeout:    5 * time.Second,
			MarketSymbol:      bookName,
			ExternalSymbol:    "BTCUSDT",
			PriceSourceURL:    "https://api.binance.com",
			NumLevels:         2,
			BaseSpreadPercent: 0.2,
			PriceStepPercent:  0.1,
			OrderSize:         "1.0",
			UpdateInterval:    1 * time.Second,
			MarketMakerID:     "test-mm-redis",
			HTTPTimeout:       5 * time.Second,
			MaxRetries:        3,
		}

		// Create a fixed price fetcher for deterministic testing
		mockPriceFetcher := NewMockPriceFetcher([]float64{10000.0})

		// Create a test order placer
		orderPlacer := &testOrderPlacer{
			client: client,
			logger: logger,
		}

		// Create a market maker strategy
		strategy := marketmaker.NewLayeredSymmetricQuoting(mmConfig, logger)

		// Create the market maker
		mm, err := marketmaker.NewMarketMaker(mmConfig, logger, orderPlacer, mockPriceFetcher, strategy)
		require.NoError(t, err, "Failed to create market maker")

		// Start the market maker
		err = mm.Start(ctx)
		require.NoError(t, err, "Failed to start market maker")

		// Wait for orders to be placed
		time.Sleep(2 * time.Second)

		// Get the order book state
		stateResp, err := client.GetOrderBookState(ctx, &proto.GetOrderBookStateRequest{
			Name: bookName,
		})
		require.NoError(t, err, "Failed to get order book state")

		// Verify orders were placed
		assert.Greater(t, len(stateResp.Bids), 0, "Expected at least one bid level")
		assert.Greater(t, len(stateResp.Asks), 0, "Expected at least one ask level")

		// Calculate expected number of orders
		expectedOrderCount := mmConfig.NumLevels * 2 // bid and ask sides
		actualOrderCount := 0
		for _, level := range stateResp.Bids {
			actualOrderCount += int(level.OrderCount)
		}
		for _, level := range stateResp.Asks {
			actualOrderCount += int(level.OrderCount)
		}

		assert.Equal(t, expectedOrderCount, actualOrderCount, "Should have placed the expected number of orders")

		// Stop the market maker
		err = mm.Stop(ctx)
		require.NoError(t, err, "Failed to stop market maker")

		// Wait for cancellation
		time.Sleep(1 * time.Second)

		// Verify all orders were cancelled
		stateResp2, err := client.GetOrderBookState(ctx, &proto.GetOrderBookStateRequest{
			Name: bookName,
		})
		require.NoError(t, err, "Failed to get order book state after stopping")

		orderCount := 0
		for _, level := range stateResp2.Bids {
			orderCount += int(level.OrderCount)
		}
		for _, level := range stateResp2.Asks {
			orderCount += int(level.OrderCount)
		}

		assert.Equal(t, 0, orderCount, "Expected all market maker orders to be cancelled")
	})
}
