package marketmaker

import (
	"context"
	"log/slog"
	"os"
	"strconv"
	"testing"

	pb "github.com/erain9/matchingo/pkg/api/proto"
)

func TestMarketMakerStrategy(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	config := &Config{
		MarketSymbol:      "BTC-USDT",
		NumLevels:         3,
		BaseSpreadPercent: 0.1,    // 0.1%
		PriceStepPercent:  0.05,   // 0.05%
		OrderSize:         "0.01", // 0.01 BTC
		MarketMakerID:     "test-mm",
	}

	strategy := NewLayeredSymmetricQuoting(config, logger)

	// Test case 1: Verify basic order creation
	t.Run("Basic order creation", func(t *testing.T) {
		ctx := context.Background()
		orders, err := strategy.CalculateOrders(ctx, 50000.0, "test-mm")
		if err != nil {
			t.Fatalf("CalculateOrders failed: %v", err)
		}

		if len(orders) != 6 {
			t.Errorf("Expected 6 orders (3 bids + 3 asks), got %d", len(orders))
		}

		// Verify first bid and ask
		if orders[0].Side != pb.OrderSide_BUY { // BUY
			t.Errorf("Expected first order to be a buy order")
		}
		if orders[1].Side != pb.OrderSide_SELL { // SELL
			t.Errorf("Expected second order to be a sell order")
		}

		// Verify order types and time in force
		for _, order := range orders {
			if order.OrderType != pb.OrderType_LIMIT { // LIMIT
				t.Errorf("Expected LIMIT order type")
			}
			if order.TimeInForce != pb.TimeInForce_GTC { // GTC
				t.Errorf("Expected GTC time in force")
			}
		}
	})

	// Test case 2: Verify order price spacing
	t.Run("Order price spacing", func(t *testing.T) {
		ctx := context.Background()
		orders, err := strategy.CalculateOrders(ctx, 50000.0, "test-mm")
		if err != nil {
			t.Fatalf("CalculateOrders failed: %v", err)
		}

		// Extract bid prices
		var bidPrices []float64
		for i := 0; i < len(orders); i += 2 {
			price := parseFloat(t, orders[i].Price)
			bidPrices = append(bidPrices, price)
		}

		// With the current implementation, each level has a larger step,
		// so we expect the prices to decrease at an increasing rate
		if len(bidPrices) > 1 {
			// Simply verify that the prices are decreasing
			// and they are in the correct order from highest to lowest
			for i := 1; i < len(bidPrices); i++ {
				if bidPrices[i-1] <= bidPrices[i] {
					t.Errorf("Expected decreasing prices, but price at level %d (%f) is not less than price at level %d (%f)",
						i, bidPrices[i], i-1, bidPrices[i-1])
				}
			}
		}
	})
}

func parseFloat(t *testing.T, s string) float64 {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		t.Fatalf("Failed to parse float: %v", err)
	}
	return f
}
