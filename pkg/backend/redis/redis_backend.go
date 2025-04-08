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
)

// RedisBackend implements OrderBookBackend interface with Redis storage
type RedisBackend struct {
	sync.RWMutex
	client      *redis.Client
	ctx         context.Context
	orderKey    string
	bidsKey     string
	asksKey     string
	stopBuyKey  string
	stopSellKey string
	ocoKey      string
}

// NewRedisBackend creates a new instance of RedisBackend
func NewRedisBackend(client *redis.Client, orderPrefix string) *RedisBackend {
	return &RedisBackend{
		client:      client,
		ctx:         context.Background(),
		orderKey:    fmt.Sprintf("%s:order", orderPrefix),
		bidsKey:     fmt.Sprintf("%s:bids", orderPrefix),
		asksKey:     fmt.Sprintf("%s:asks", orderPrefix),
		stopBuyKey:  fmt.Sprintf("%s:stop:buy", orderPrefix),
		stopSellKey: fmt.Sprintf("%s:stop:sell", orderPrefix),
		ocoKey:      fmt.Sprintf("%s:oco", orderPrefix),
	}
}

// GetOrder retrieves an order by ID from Redis
func (b *RedisBackend) GetOrder(orderID string) *core.Order {
	key := fmt.Sprintf("%s:%s", b.orderKey, orderID)
	val, err := b.client.Get(b.ctx, key).Result()
	if err != nil {
		return nil
	}

	var order core.Order
	if err := json.Unmarshal([]byte(val), &order); err != nil {
		return nil
	}

	return &order
}

// StoreOrder stores an order in Redis
func (b *RedisBackend) StoreOrder(order *core.Order) error {
	// Check if order exists
	key := fmt.Sprintf("%s:%s", b.orderKey, order.ID())
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
	key := fmt.Sprintf("%s:%s", b.orderKey, order.ID())
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
	key := fmt.Sprintf("%s:%s", b.orderKey, orderID)
	b.client.Del(b.ctx, key)
}

// AppendToSide adds an order to the specified side in Redis
func (b *RedisBackend) AppendToSide(side core.Side, order *core.Order) {
	if order.IsMarketOrder() {
		return
	}

	var sideKey string
	if side == core.Buy {
		sideKey = b.bidsKey
	} else {
		sideKey = b.asksKey
	}

	// Store order price -> ID mapping
	priceKey := fmt.Sprintf("%s:%s", sideKey, order.Price().String())
	b.client.SAdd(b.ctx, priceKey, order.ID())

	// Store price in the sorted set
	var score float64
	if side == core.Buy {
		// For buy orders, higher prices should be processed first
		score = -parseFloat(order.Price())
	} else {
		// For sell orders, lower prices should be processed first
		score = parseFloat(order.Price())
	}

	b.client.ZAdd(b.ctx, sideKey, redis.Z{
		Score:  score,
		Member: order.Price().String(),
	})
}

// RemoveFromSide removes an order from the specified side in Redis
func (b *RedisBackend) RemoveFromSide(side core.Side, order *core.Order) bool {
	if order.IsMarketOrder() {
		return false
	}

	var sideKey string
	if side == core.Buy {
		sideKey = b.bidsKey
	} else {
		sideKey = b.asksKey
	}

	priceKey := fmt.Sprintf("%s:%s", sideKey, order.Price().String())
	removed, err := b.client.SRem(b.ctx, priceKey, order.ID()).Result()
	if err != nil || removed == 0 {
		return false
	}

	// Check if there are other orders at this price
	count, err := b.client.SCard(b.ctx, priceKey).Result()
	if err != nil || count > 0 {
		return true
	}

	// If no more orders at this price, remove the price from the sorted set
	b.client.ZRem(b.ctx, sideKey, order.Price().String())
	b.client.Del(b.ctx, priceKey)

	return true
}

// AppendToStopBook adds a stop order to the stop book in Redis
func (b *RedisBackend) AppendToStopBook(order *core.Order) {
	if !order.IsStopOrder() {
		return
	}

	var stopKey string
	if order.Side() == core.Buy {
		stopKey = b.stopBuyKey
	} else {
		stopKey = b.stopSellKey
	}

	// Store order stop price -> ID mapping
	priceKey := fmt.Sprintf("%s:%s", stopKey, order.StopPrice().String())
	b.client.SAdd(b.ctx, priceKey, order.ID())

	// Store price in the sorted set
	var score float64
	if order.Side() == core.Buy {
		// For buy stop orders, lower prices should be processed first
		score = parseFloat(order.StopPrice())
	} else {
		// For sell stop orders, higher prices should be processed first
		score = -parseFloat(order.StopPrice())
	}

	b.client.ZAdd(b.ctx, stopKey, redis.Z{
		Score:  score,
		Member: order.StopPrice().String(),
	})
}

// RemoveFromStopBook removes a stop order from the stop book in Redis
func (b *RedisBackend) RemoveFromStopBook(order *core.Order) bool {
	if !order.IsStopOrder() {
		return false
	}

	var stopKey string
	if order.Side() == core.Buy {
		stopKey = b.stopBuyKey
	} else {
		stopKey = b.stopSellKey
	}

	priceKey := fmt.Sprintf("%s:%s", stopKey, order.StopPrice().String())
	removed, err := b.client.SRem(b.ctx, priceKey, order.ID()).Result()
	if err != nil || removed == 0 {
		return false
	}

	// Check if there are other orders at this price
	count, err := b.client.SCard(b.ctx, priceKey).Result()
	if err != nil || count > 0 {
		return true
	}

	// If no more orders at this price, remove the price from the sorted set
	b.client.ZRem(b.ctx, stopKey, order.StopPrice().String())
	b.client.Del(b.ctx, priceKey)

	return true
}

// CheckOCO checks and cancels any OCO (One Cancels Other) orders in Redis
func (b *RedisBackend) CheckOCO(orderID string) string {
	ocoID, err := b.client.HGet(b.ctx, b.ocoKey, orderID).Result()
	if err != nil {
		return ""
	}

	// Clean up mappings
	pipe := b.client.Pipeline()
	pipe.HDel(b.ctx, b.ocoKey, orderID)
	pipe.HDel(b.ctx, b.ocoKey, ocoID)
	pipe.Exec(b.ctx)

	return ocoID
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
			rs.backend.orderKey+":"+orderID).Result()
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
				rsb.backend.orderKey+":"+orderID).Result()
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
				rsb.backend.orderKey+":"+orderID).Result()
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

// parseFloat converts a decimal to float64 for Redis score
func parseFloat(d fpdecimal.Decimal) float64 {
	str := d.String()
	f, _ := strconv.ParseFloat(str, 64)
	return f
}
