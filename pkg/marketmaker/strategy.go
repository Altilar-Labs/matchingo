package marketmaker

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"time"

	pb "github.com/erain9/matchingo/pkg/api/proto"
)

// MarketMakerStrategy defines the interface for market making strategies
type MarketMakerStrategy interface {
	// CalculateOrders calculates the orders to be placed based on the current price
	CalculateOrders(ctx context.Context, currentPrice float64) ([]*pb.CreateOrderRequest, error)
}

// LayeredSymmetricQuoting implements a symmetric market making strategy with multiple price levels
type LayeredSymmetricQuoting struct {
	cfg    *Config
	logger *slog.Logger
}

// NewLayeredSymmetricQuoting creates a new LayeredSymmetricQuoting strategy
func NewLayeredSymmetricQuoting(cfg *Config, logger *slog.Logger) MarketMakerStrategy {
	return &LayeredSymmetricQuoting{
		cfg:    cfg,
		logger: logger.With("component", "LayeredSymmetricQuoting"),
	}
}

// CalculateOrders implements MarketMakerStrategy
func (s *LayeredSymmetricQuoting) CalculateOrders(ctx context.Context, currentPrice float64) ([]*pb.CreateOrderRequest, error) {
	baseHalfSpread := currentPrice * (s.cfg.BaseSpreadPercent / 2 / 100)
	priceStep := currentPrice * (s.cfg.PriceStepPercent / 100)

	// Pre-allocate slice for all orders (buy and sell orders for each level)
	orders := make([]*pb.CreateOrderRequest, 0, s.cfg.NumLevels*2)

	timestamp := time.Now().UnixNano()

	for i := 1; i <= s.cfg.NumLevels; i++ {
		// Calculate bid and ask prices for this level
		bidPrice := currentPrice - baseHalfSpread - float64(i-1)*priceStep
		askPrice := currentPrice + baseHalfSpread + float64(i-1)*priceStep

		// Format prices with appropriate precision (8 decimal places for crypto)
		bidPriceStr := strconv.FormatFloat(math.Round(bidPrice*1e8)/1e8, 'f', 8, 64)
		askPriceStr := strconv.FormatFloat(math.Round(askPrice*1e8)/1e8, 'f', 8, 64)

		// Create buy order for this level
		buyOrder := &pb.CreateOrderRequest{
			OrderBookName: s.cfg.MarketSymbol,
			OrderId:       fmt.Sprintf("%s-buy-%d-%d", s.cfg.MarketMakerID, i, timestamp),
			Side:          pb.Side_BUY,
			OrderType:     pb.OrderType_LIMIT,
			Quantity:      s.cfg.OrderSize,
			Price:         bidPriceStr,
			TimeInForce:   pb.TimeInForce_GTC,
		}
		orders = append(orders, buyOrder)

		// Create sell order for this level
		sellOrder := &pb.CreateOrderRequest{
			OrderBookName: s.cfg.MarketSymbol,
			OrderId:       fmt.Sprintf("%s-sell-%d-%d", s.cfg.MarketMakerID, i, timestamp),
			Side:          pb.Side_SELL,
			OrderType:     pb.OrderType_LIMIT,
			Quantity:      s.cfg.OrderSize,
			Price:         askPriceStr,
			TimeInForce:   pb.TimeInForce_GTC,
		}
		orders = append(orders, sellOrder)

		s.logger.Debug("Calculated order pair",
			"level", i,
			"bid_price", bidPriceStr,
			"ask_price", askPriceStr,
			"quantity", s.cfg.OrderSize)
	}

	return orders, nil
}
