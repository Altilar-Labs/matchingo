package marketmaker

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	pb "github.com/erain9/matchingo/pkg/api/proto"
	"github.com/erain9/matchingo/pkg/core"
)

// MarketMaker represents the market making service
type MarketMaker struct {
	cfg          *Config
	logger       *slog.Logger
	orderPlacer  OrderPlacer
	priceFetcher PriceFetcher
	strategy     MarketMakerStrategy
	activeOrders sync.Map // map[string]bool - tracks active order IDs
	stopCh       chan struct{}
	wg           sync.WaitGroup
	address      string // Market maker's address
}

// NewMarketMaker creates a new market maker service
func NewMarketMaker(cfg *Config, logger *slog.Logger, orderPlacer OrderPlacer, priceFetcher PriceFetcher, strategy MarketMakerStrategy) (*MarketMaker, error) {
	// Generate a unique address for this market maker
	address, err := core.GenerateFakeERC20Address()
	if err != nil {
		return nil, fmt.Errorf("failed to generate market maker address: %w", err)
	}

	return &MarketMaker{
		cfg:          cfg,
		logger:       logger.With("component", "MarketMaker"),
		orderPlacer:  orderPlacer,
		priceFetcher: priceFetcher,
		strategy:     strategy,
		stopCh:       make(chan struct{}),
		address:      address,
	}, nil
}

// Start begins the market making process
func (m *MarketMaker) Start(ctx context.Context) error {
	m.logger.Info("Starting market maker service",
		"market_symbol", m.cfg.MarketSymbol,
		"update_interval", m.cfg.UpdateInterval)

	// Start the main loop in a goroutine
	m.wg.Add(1)
	go m.run(ctx)

	return nil
}

// Stop gracefully shuts down the market maker
func (m *MarketMaker) Stop(ctx context.Context) error {
	m.logger.Info("Stopping market maker service")

	// Signal the main loop to stop
	close(m.stopCh)

	// Wait for the main loop to finish with timeout
	done := make(chan struct{})
	go func() {
		m.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		m.logger.Info("Market maker stopped successfully")
	case <-ctx.Done():
		return fmt.Errorf("timeout waiting for market maker to stop: %w", ctx.Err())
	}

	// Cancel all active orders
	if err := m.cancelAllOrders(ctx); err != nil {
		m.logger.Error("Failed to cancel all orders during shutdown", "error", err)
		return fmt.Errorf("failed to cancel orders during shutdown: %w", err)
	}

	return nil
}

// run is the main market making loop
func (m *MarketMaker) run(ctx context.Context) {
	defer m.wg.Done()

	ticker := time.NewTicker(m.cfg.UpdateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			m.logger.Info("Context cancelled, stopping market maker loop")
			return
		case <-m.stopCh:
			m.logger.Info("Stop signal received, stopping market maker loop")
			return
		case <-ticker.C:
			if err := m.updateOrders(ctx); err != nil {
				m.logger.Error("Failed to update orders", "error", err)
				// Continue running despite errors
			}
		}
	}
}

// updateOrders performs a single iteration of the market making process
func (m *MarketMaker) updateOrders(ctx context.Context) error {
	// Fetch current price
	price, err := m.priceFetcher.FetchPrice(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch price: %w", err)
	}

	// Calculate new orders
	orders, err := m.strategy.CalculateOrders(ctx, price, m.address)
	if err != nil {
		return fmt.Errorf("failed to calculate orders: %w", err)
	}

	// Cancel existing orders
	if err := m.cancelAllOrders(ctx); err != nil {
		return fmt.Errorf("failed to cancel existing orders: %w", err)
	}

	// Place new orders
	for _, order := range orders {
		// Add market maker's address to the order
		order.UserAddress = m.address

		resp, err := m.orderPlacer.CreateOrder(ctx, order)
		if err != nil {
			m.logger.Error("Failed to place order",
				"order_id", order.OrderId,
				"side", order.Side,
				"price", order.Price,
				"user_address", m.address,
				"error", err)
			continue
		}

		// Track the new order
		m.activeOrders.Store(order.OrderId, true)

		m.logger.Debug("Successfully placed order",
			"order_id", resp.OrderId,
			"side", order.Side,
			"price", order.Price,
			"user_address", m.address)
	}

	return nil
}

// cancelAllOrders cancels all tracked active orders
func (m *MarketMaker) cancelAllOrders(ctx context.Context) error {
	var lastErr error
	m.activeOrders.Range(func(key, _ interface{}) bool {
		orderID := key.(string)
		req := &pb.CancelOrderRequest{
			OrderBookName: m.cfg.MarketSymbol,
			OrderId:       orderID,
		}

		_, err := m.orderPlacer.CancelOrder(ctx, req)
		if err != nil {
			m.logger.Error("Failed to cancel order",
				"order_id", orderID,
				"error", err)
			lastErr = err
			// Continue canceling other orders even if one fails
			return true
		}

		m.activeOrders.Delete(orderID)
		m.logger.Debug("Successfully cancelled order", "order_id", orderID)
		return true
	})

	return lastErr
}
