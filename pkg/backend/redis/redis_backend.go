package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/erain9/matchingo/pkg/core"
	"github.com/nikolaydubina/fpdecimal"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// RedisOptions represents configuration options for Redis connection
type RedisOptions struct {
	Addr     string
	Password string
	DB       int
}

var defaultOptions = &RedisOptions{
	Addr:     "localhost:6379",
	Password: "",
	DB:       0,
}

// SetDefaultRedisOptions sets the default options for Redis connections
func SetDefaultRedisOptions(options *RedisOptions) {
	defaultOptions = options
}

// GetRedisClient creates a new Redis client using the default options
func GetRedisClient() *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:     defaultOptions.Addr,
		Password: defaultOptions.Password,
		DB:       defaultOptions.DB,
	})
}

// RedisBackend implements OrderBookBackend interface with Redis storage
type RedisBackend struct {
	sync.RWMutex
	client      *redis.Client
	ctx         context.Context
	orderPrefix string
	bidsKey     string
	asksKey     string
	stopBuyKey  string
	stopSellKey string
	ocoKey      string
	logger      *zap.Logger
}

// NewRedisBackend creates a new instance of RedisBackend
func NewRedisBackend(client *redis.Client, orderPrefix string, logger *zap.Logger) *RedisBackend {
	return &RedisBackend{
		client:      client,
		ctx:         context.Background(),
		orderPrefix: orderPrefix,
		bidsKey:     fmt.Sprintf("%s:bids", orderPrefix),
		asksKey:     fmt.Sprintf("%s:asks", orderPrefix),
		stopBuyKey:  fmt.Sprintf("%s:stop:buy", orderPrefix),
		stopSellKey: fmt.Sprintf("%s:stop:sell", orderPrefix),
		ocoKey:      fmt.Sprintf("%s:oco", orderPrefix),
		logger:      logger,
	}
}

// GetOrder retrieves an order from Redis by its ID
func (b *RedisBackend) GetOrder(orderID string) *core.Order {
	b.RLock()
	defer b.RUnlock()

	key := b.getOrderKey(orderID)
	data, err := b.client.Get(b.ctx, key).Bytes()
	if err != nil {
		if err != redis.Nil {
			b.logger.Error("failed to get order",
				zap.String("orderID", orderID),
				zap.Error(err))
		}
		return nil
	}

	var order core.Order
	if err := json.Unmarshal(data, &order); err != nil {
		b.logger.Error("failed to unmarshal order",
			zap.String("orderID", orderID),
			zap.Error(err))
		return nil
	}

	return &order
}

// StoreOrder stores an order in Redis
func (b *RedisBackend) StoreOrder(order *core.Order) error {
	// Check if order exists
	key := b.getOrderKey(order.ID())
	exists, err := b.client.Exists(b.ctx, key).Result()
	if err != nil {
		return err
	}
	if exists > 0 {
		return core.ErrOrderExists
	}

	// Serialize order
	data, err := json.Marshal(order)
	if err != nil {
		return err
	}

	// Store order
	err = b.client.Set(b.ctx, key, data, 0).Err()
	if err != nil {
		return err
	}

	// Store OCO mapping if exists
	if oco := order.OCO(); oco != "" {
		pipe := b.client.Pipeline()
		pipe.HSet(b.ctx, b.ocoKey, order.ID(), oco)
		pipe.HSet(b.ctx, b.ocoKey, oco, order.ID())
		_, err = pipe.Exec(b.ctx)
		if err != nil {
			return err
		}
	}

	return nil
}

// UpdateOrder updates an existing order in Redis
func (b *RedisBackend) UpdateOrder(order *core.Order) error {
	// Check if order exists
	key := b.getOrderKey(order.ID())
	exists, err := b.client.Exists(b.ctx, key).Result()
	if err != nil {
		return err
	}
	if exists == 0 {
		return core.ErrNonexistentOrder
	}

	// Serialize order
	data, err := json.Marshal(order)
	if err != nil {
		return err
	}

	// Update order
	return b.client.Set(b.ctx, key, data, 0).Err()
}

// DeleteOrder deletes an order from Redis
func (b *RedisBackend) DeleteOrder(orderID string) {
	// Get order to check OCO
	order := b.GetOrder(orderID)
	if order == nil {
		return
	}

	// Clean up OCO references
	if oco := order.OCO(); oco != "" {
		pipe := b.client.Pipeline()
		pipe.HDel(b.ctx, b.ocoKey, orderID)
		pipe.HDel(b.ctx, b.ocoKey, oco)
		pipe.Exec(b.ctx)
	}

	// Delete order
	key := b.getOrderKey(orderID)
	b.client.Del(b.ctx, key)
}

// AppendToSide adds an order to the specified side of the order book
func (b *RedisBackend) AppendToSide(side core.Side, order *core.Order) {
	b.Lock()
	defer b.Unlock()

	pipe := b.client.Pipeline()
	sideKey := b.getSideKey(side)
	priceKey := fmt.Sprintf("%s:%s", sideKey, order.Price().String())

	// Add to sorted set with score as price
	pipe.ZAdd(b.ctx, sideKey, redis.Z{
		Score:  order.Price().Float64(),
		Member: order.Price().String(),
	})

	// Add order ID to the set at this price level
	pipe.SAdd(b.ctx, priceKey, order.ID())

	// Execute pipeline
	if _, err := pipe.Exec(b.ctx); err != nil {
		b.logger.Error("failed to execute pipeline",
			zap.String("order_id", order.ID()),
			zap.Error(err))
		return
	}
}

// RemoveFromSide removes an order from the specified side of the order book
func (b *RedisBackend) RemoveFromSide(side core.Side, order *core.Order) bool {
	b.Lock()
	defer b.Unlock()

	pipe := b.client.Pipeline()
	sideKey := b.getSideKey(side)
	priceKey := fmt.Sprintf("%s:%s", sideKey, order.Price().String())

	// Remove order from price level set
	pipe.SRem(b.ctx, priceKey, order.ID())

	// Check if price level is empty after removal
	pipe.SCard(b.ctx, priceKey).Result()

	// Execute pipeline
	cmders, err := pipe.Exec(b.ctx)
	if err != nil {
		b.logger.Error("failed to execute pipeline",
			zap.String("orderID", order.ID()),
			zap.String("side", side.String()),
			zap.Error(err))
		return false
	}

	// If price level is empty, remove it from sorted set and delete the set
	if cmders[1].(*redis.IntCmd).Val() == 0 {
		pipe.ZRem(b.ctx, sideKey, order.Price().String())
		pipe.Del(b.ctx, priceKey)
		if _, err := pipe.Exec(b.ctx); err != nil {
			b.logger.Error("failed to clean up empty price level",
				zap.String("orderID", order.ID()),
				zap.Error(err))
		}
	}

	return true
}

// AppendToStopBook adds a stop order to the stop book
func (b *RedisBackend) AppendToStopBook(order *core.Order) {
	b.Lock()
	defer b.Unlock()

	// Determine which stop book to use
	var stopKey string
	if order.Side() == core.Buy {
		stopKey = b.stopBuyKey
	} else {
		stopKey = b.stopSellKey
	}

	pipe := b.client.Pipeline()
	priceKey := fmt.Sprintf("%s:%s", stopKey, order.StopPrice().String())

	// Add to sorted set with score as stop price
	pipe.ZAdd(b.ctx, stopKey, redis.Z{
		Score:  order.StopPrice().Float64(),
		Member: order.StopPrice().String(),
	})

	// Add order ID to the set at this price level
	pipe.SAdd(b.ctx, priceKey, order.ID())

	// Execute pipeline
	if _, err := pipe.Exec(b.ctx); err != nil {
		b.logger.Error("failed to execute pipeline",
			zap.String("order_id", order.ID()),
			zap.Error(err))
	}
}

// RemoveFromStopBook removes a stop order from the stop book
func (b *RedisBackend) RemoveFromStopBook(order *core.Order) bool {
	b.Lock()
	defer b.Unlock()

	// Determine which stop book to use
	var stopKey string
	if order.Side() == core.Buy {
		stopKey = b.stopBuyKey
	} else {
		stopKey = b.stopSellKey
	}

	pipe := b.client.Pipeline()
	priceKey := fmt.Sprintf("%s:%s", stopKey, order.StopPrice().String())

	// Remove order from price level set
	pipe.SRem(b.ctx, priceKey, order.ID())

	// Check if price level is empty after removal
	pipe.SCard(b.ctx, priceKey).Result()

	// Execute pipeline
	cmders, err := pipe.Exec(b.ctx)
	if err != nil {
		b.logger.Error("failed to execute pipeline",
			zap.String("orderID", order.ID()),
			zap.Error(err))
		return false
	}

	// If price level is empty, remove it from sorted set and delete the set
	if cmders[1].(*redis.IntCmd).Val() == 0 {
		pipe.ZRem(b.ctx, stopKey, order.StopPrice().String())
		pipe.Del(b.ctx, priceKey)
		if _, err := pipe.Exec(b.ctx); err != nil {
			b.logger.Error("failed to clean up empty price level",
				zap.String("orderID", order.ID()),
				zap.Error(err))
		}
	}

	return true
}

// CheckOCO checks and returns any OCO (One Cancels Other) orders in Redis
func (b *RedisBackend) CheckOCO(orderID string) string {
	order := b.GetOrder(orderID)
	if order == nil {
		return ""
	}

	// Return the OCO order ID
	return order.OCO()
}

// GetBids returns the bid side of the order book for iteration
func (b *RedisBackend) GetBids() interface{} {
	return &RedisSide{
		backend: b,
		sideKey: b.bidsKey,
		reverse: true, // Bids are stored with negative scores
	}
}

// GetAsks returns the ask side of the order book for iteration
func (b *RedisBackend) GetAsks() interface{} {
	return &RedisSide{
		backend: b,
		sideKey: b.asksKey,
		reverse: false,
	}
}

// GetStopBook returns the stop book for iteration
func (b *RedisBackend) GetStopBook() interface{} {
	return &RedisStopBook{
		backend:     b,
		stopBuyKey:  b.stopBuyKey,
		stopSellKey: b.stopSellKey,
	}
}

// Helper functions and types for Redis iteration

// RedisSide represents one side (bid/ask) of the Redis order book
type RedisSide struct {
	backend *RedisBackend
	sideKey string
	reverse bool // If true, prices are stored with negative scores
}

// String implements fmt.Stringer interface
func (rs *RedisSide) String() string {
	sb := strings.Builder{}

	// Get all members from the sorted set
	var members []string
	var err error

	if rs.reverse {
		// For bids (highest first)
		members, err = rs.backend.client.ZRevRange(rs.backend.ctx, rs.sideKey, 0, -1).Result()
	} else {
		// For asks (lowest first)
		members, err = rs.backend.client.ZRange(rs.backend.ctx, rs.sideKey, 0, -1).Result()
	}

	if err != nil {
		return fmt.Sprintf("Error fetching data: %v", err)
	}

	// Process each member
	for _, orderID := range members {
		orderData, err := rs.backend.client.Get(rs.backend.ctx,
			rs.backend.getOrderKey(orderID)).Result()
		if err != nil {
			continue
		}

		var orderMap map[string]interface{}
		if err := json.Unmarshal([]byte(orderData), &orderMap); err != nil {
			continue
		}

		price, ok := orderMap["price"].(string)
		if !ok {
			continue
		}

		sb.WriteString(fmt.Sprintf("\n%s -> orders: 1", price))
	}

	return sb.String()
}

// Prices returns all prices in the order side
func (rs *RedisSide) Prices() []fpdecimal.Decimal {
	var members []string
	var err error

	if rs.reverse {
		// For bids (highest first)
		members, err = rs.backend.client.ZRevRange(rs.backend.ctx, rs.sideKey, 0, -1).Result()
	} else {
		// For asks (lowest first)
		members, err = rs.backend.client.ZRange(rs.backend.ctx, rs.sideKey, 0, -1).Result()
	}

	if err != nil {
		return []fpdecimal.Decimal{}
	}

	prices := make([]fpdecimal.Decimal, 0, len(members))
	for _, priceStr := range members {
		// Convert string to float64 first, then to fpdecimal
		f, err := strconv.ParseFloat(priceStr, 64)
		if err != nil {
			continue
		}
		prices = append(prices, fpdecimal.FromFloat(f))
	}

	return prices
}

// Orders returns all orders at a given price level
func (rs *RedisSide) Orders(price fpdecimal.Decimal) []*core.Order {
	priceKey := fmt.Sprintf("%s:%s", rs.sideKey, price.String())

	// Get all order IDs at this price level
	orderIDs, err := rs.backend.client.SMembers(rs.backend.ctx, priceKey).Result()
	if err != nil {
		return []*core.Order{}
	}

	orders := make([]*core.Order, 0, len(orderIDs))
	for _, orderID := range orderIDs {
		order := rs.backend.GetOrder(orderID)
		if order != nil {
			orders = append(orders, order)
		}
	}

	return orders
}

// RedisStopBook represents the Redis stop book
type RedisStopBook struct {
	backend     *RedisBackend
	stopBuyKey  string
	stopSellKey string
}

// String implements fmt.Stringer interface
func (rsb *RedisStopBook) String() string {
	sb := strings.Builder{}

	// Buy stop orders
	sb.WriteString("Buy Stop Orders:")
	buyMembers, err := rsb.backend.client.ZRange(rsb.backend.ctx, rsb.stopBuyKey, 0, -1).Result()
	if err == nil {
		for _, orderID := range buyMembers {
			orderData, err := rsb.backend.client.Get(rsb.backend.ctx,
				rsb.backend.getOrderKey(orderID)).Result()
			if err != nil {
				continue
			}

			var orderMap map[string]interface{}
			if err := json.Unmarshal([]byte(orderData), &orderMap); err != nil {
				continue
			}

			stopPrice, ok := orderMap["stopPrice"].(string)
			if !ok {
				continue
			}

			sb.WriteString(fmt.Sprintf("\n%s -> orders: 1", stopPrice))
		}
	}

	// Sell stop orders
	sb.WriteString("\nSell Stop Orders:")
	sellMembers, err := rsb.backend.client.ZRange(rsb.backend.ctx, rsb.stopSellKey, 0, -1).Result()
	if err == nil {
		for _, orderID := range sellMembers {
			orderData, err := rsb.backend.client.Get(rsb.backend.ctx,
				rsb.backend.getOrderKey(orderID)).Result()
			if err != nil {
				continue
			}

			var orderMap map[string]interface{}
			if err := json.Unmarshal([]byte(orderData), &orderMap); err != nil {
				continue
			}

			stopPrice, ok := orderMap["stopPrice"].(string)
			if !ok {
				continue
			}

			sb.WriteString(fmt.Sprintf("\n%s -> orders: 1", stopPrice))
		}
	}

	return sb.String()
}

// Prices returns all unique prices from both buy and sell sides
func (rsb *RedisStopBook) Prices() []fpdecimal.Decimal {
	// Get prices from buy side
	buyPrices := make([]fpdecimal.Decimal, 0)
	buyMembers, err := rsb.backend.client.ZRange(rsb.backend.ctx, rsb.stopBuyKey, 0, -1).Result()
	if err == nil {
		for _, priceStr := range buyMembers {
			f, err := strconv.ParseFloat(priceStr, 64)
			if err != nil {
				continue
			}
			buyPrices = append(buyPrices, fpdecimal.FromFloat(f))
		}
	}

	// Get prices from sell side
	sellPrices := make([]fpdecimal.Decimal, 0)
	sellMembers, err := rsb.backend.client.ZRange(rsb.backend.ctx, rsb.stopSellKey, 0, -1).Result()
	if err == nil {
		for _, priceStr := range sellMembers {
			f, err := strconv.ParseFloat(priceStr, 64)
			if err != nil {
				continue
			}
			sellPrices = append(sellPrices, fpdecimal.FromFloat(f))
		}
	}

	// Create a map to deduplicate prices
	priceMap := make(map[string]fpdecimal.Decimal)
	for _, price := range buyPrices {
		priceMap[price.String()] = price
	}
	for _, price := range sellPrices {
		priceMap[price.String()] = price
	}

	// Convert map back to slice
	prices := make([]fpdecimal.Decimal, 0, len(priceMap))
	for _, price := range priceMap {
		prices = append(prices, price)
	}

	return prices
}

// Orders returns all orders at a given price level for both buy and sell sides
func (rsb *RedisStopBook) Orders(price fpdecimal.Decimal) []*core.Order {
	// Get buy orders at this price
	buyPriceKey := fmt.Sprintf("%s:%s", rsb.stopBuyKey, price.String())
	buyOrderIDs, err := rsb.backend.client.SMembers(rsb.backend.ctx, buyPriceKey).Result()
	if err != nil {
		buyOrderIDs = []string{}
	}

	// Get sell orders at this price
	sellPriceKey := fmt.Sprintf("%s:%s", rsb.stopSellKey, price.String())
	sellOrderIDs, err := rsb.backend.client.SMembers(rsb.backend.ctx, sellPriceKey).Result()
	if err != nil {
		sellOrderIDs = []string{}
	}

	// Combine order IDs
	allOrderIDs := append(buyOrderIDs, sellOrderIDs...)
	orders := make([]*core.Order, 0, len(allOrderIDs))

	// Get each order
	for _, orderID := range allOrderIDs {
		order := rsb.backend.GetOrder(orderID)
		if order != nil {
			orders = append(orders, order)
		}
	}

	return orders
}

// BuyOrders returns all buy stop orders
func (rsb *RedisStopBook) BuyOrders() []*core.Order {
	var allOrders []*core.Order

	// Get all buy stop price levels
	members, err := rsb.backend.client.ZRange(rsb.backend.ctx, rsb.stopBuyKey, 0, -1).Result()
	if err != nil {
		return allOrders
	}

	// Get orders at each price level
	for _, priceStr := range members {
		priceKey := fmt.Sprintf("%s:%s", rsb.stopBuyKey, priceStr)
		orderIDs, err := rsb.backend.client.SMembers(rsb.backend.ctx, priceKey).Result()
		if err != nil {
			continue
		}

		for _, orderID := range orderIDs {
			order := rsb.backend.GetOrder(orderID)
			if order != nil {
				allOrders = append(allOrders, order)
			}
		}
	}

	return allOrders
}

// SellOrders returns all sell stop orders
func (rsb *RedisStopBook) SellOrders() []*core.Order {
	var allOrders []*core.Order

	// Get all sell stop price levels
	members, err := rsb.backend.client.ZRange(rsb.backend.ctx, rsb.stopSellKey, 0, -1).Result()
	if err != nil {
		return allOrders
	}

	// Get orders at each price level
	for _, priceStr := range members {
		priceKey := fmt.Sprintf("%s:%s", rsb.stopSellKey, priceStr)
		orderIDs, err := rsb.backend.client.SMembers(rsb.backend.ctx, priceKey).Result()
		if err != nil {
			continue
		}

		for _, orderID := range orderIDs {
			order := rsb.backend.GetOrder(orderID)
			if order != nil {
				allOrders = append(allOrders, order)
			}
		}
	}

	return allOrders
}

// parseFloat converts a decimal to float64 for Redis score
func parseFloat(d fpdecimal.Decimal) float64 {
	str := d.String()
	f, _ := strconv.ParseFloat(str, 64)
	return f
}

// Helper methods for key generation
func (b *RedisBackend) getSideKey(side core.Side) string {
	if side == core.Buy {
		return b.bidsKey
	}
	return b.asksKey
}

func (b *RedisBackend) getOrderKey(orderID string) string {
	return fmt.Sprintf("order:%s", orderID)
}

// Close closes the Redis client and cleans up resources
func (b *RedisBackend) Close() error {
	b.Lock()
	defer b.Unlock()
	return b.client.Close()
}

// WithContext returns a new RedisBackend with the given context
func (b *RedisBackend) WithContext(ctx context.Context) *RedisBackend {
	if ctx == nil {
		ctx = context.Background()
	}
	clone := *b
	clone.ctx = ctx
	return &clone
}

func (rsb *RedisStopBook) RemoveStopOrder(symbol string, orderID string) error {
	order := rsb.backend.GetOrder(orderID)
	if order == nil {
		return fmt.Errorf("order not found: %s", orderID)
	}
	// ... existing code ...
	return nil
}

func (rsb *RedisStopBook) TriggerStopOrder(symbol string, orderID string) error {
	order := rsb.backend.GetOrder(orderID)
	if order == nil {
		return fmt.Errorf("order not found: %s", orderID)
	}
	// ... existing code ...
	return nil
}

func (rsb *RedisStopBook) RemoveFromStopBook(symbol string, orderID string) error {
	order := rsb.backend.GetOrder(orderID)
	if order == nil {
		return fmt.Errorf("order not found: %s", orderID)
	}
	// ... existing code ...
	return nil
}

func (rsb *RedisStopBook) GetStopOrders(symbol string) ([]*core.Order, error) {
	// ... existing code ...
	orderIDs, err := rsb.backend.client.SMembers(rsb.backend.ctx, rsb.stopBuyKey).Result()
	if err != nil {
		return nil, err
	}
	// ... existing code ...
	for _, orderID := range orderIDs {
		order := rsb.backend.GetOrder(orderID)
		if order == nil {
			continue
		}
		// ... existing code ...
	}
	// ... existing code ...
	return nil, nil
}
