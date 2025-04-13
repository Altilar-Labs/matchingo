package redis

import (
	"context"
	"fmt"
	"testing"

	"github.com/erain9/matchingo/pkg/core"
	"github.com/nikolaydubina/fpdecimal"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestRedis initializes a Redis client for testing.
// It assumes Redis is running on localhost:6379.
// Flushes the DB before returning the client.
func setupTestRedis(t *testing.T) *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   0, // Use default DB
	})
	_, err := client.Ping(context.Background()).Result()
	if err != nil {
		t.Skipf("Skipping Redis tests: Cannot connect to Redis (%v)", err)
	}
	err = client.FlushDB(context.Background()).Err()
	if err != nil {
		t.Fatalf("Failed to flush Redis DB: %v", err)
	}
	return client
}

func TestNewRedisBackend(t *testing.T) {
	client := setupTestRedis(t)
	prefix := "test:newredis:"
	backend := NewRedisBackend(client, prefix)

	assert.NotNil(t, backend)
	assert.Equal(t, client, backend.client)
	assert.Equal(t, fmt.Sprintf("%s:order", prefix), backend.orderKey)
	assert.Equal(t, fmt.Sprintf("%s:bids", prefix), backend.bidsKey)
	assert.Equal(t, fmt.Sprintf("%s:asks", prefix), backend.asksKey)
	assert.Equal(t, fmt.Sprintf("%s:stop:buy", prefix), backend.stopBuyKey)
	assert.Equal(t, fmt.Sprintf("%s:stop:sell", prefix), backend.stopSellKey)
	assert.Equal(t, fmt.Sprintf("%s:oco", prefix), backend.ocoKey)
}

func TestRedisBackend_StoreGetUpdateDeleteOrder(t *testing.T) {
	client := setupTestRedis(t)
	backend := NewRedisBackend(client, "test:orders:")

	// Create test order
	order, err := core.NewLimitOrder("test1", core.Buy, fpdecimal.FromFloat(1.0), fpdecimal.FromFloat(100.0), core.GTC, "")
	require.NoError(t, err)

	// Test storing order
	err = backend.StoreOrder(order)
	assert.NoError(t, err)

	// Test getting order
	stored := backend.GetOrder(order.ID())
	assert.NotNil(t, stored)
	assert.Equal(t, order.ID(), stored.ID())

	// Test updating order
	order.SetQuantity(fpdecimal.FromFloat(2.0))
	err = backend.UpdateOrder(order)
	assert.NoError(t, err)

	// Verify update
	updated := backend.GetOrder(order.ID())
	assert.NotNil(t, updated)
	assert.Equal(t, fpdecimal.FromFloat(2.0), updated.Quantity())

	// Test deleting order
	backend.DeleteOrder(order.ID())
	deleted := backend.GetOrder(order.ID())
	assert.Nil(t, deleted)
}

func TestRedisBackend_AppendAndRemoveFromSide(t *testing.T) {
	client := setupTestRedis(t)
	backend := NewRedisBackend(client, "test:sides:")

	// Create test order
	order, err := core.NewLimitOrder("test1", core.Buy, fpdecimal.FromFloat(1.0), fpdecimal.FromFloat(100.0), core.GTC, "")
	require.NoError(t, err)

	// Test appending to side
	backend.AppendToSide(core.Buy, order)

	// Verify order was added
	priceKey := fmt.Sprintf("%s:%s", backend.bidsKey, order.Price().String())
	exists, err := client.SIsMember(context.Background(), priceKey, order.ID()).Result()
	assert.NoError(t, err)
	assert.True(t, exists)

	// Test removing from side
	removed := backend.RemoveFromSide(core.Buy, order)
	assert.True(t, removed)

	// Verify order was removed
	exists, err = client.SIsMember(context.Background(), priceKey, order.ID()).Result()
	assert.NoError(t, err)
	assert.False(t, exists)
}

func TestRedisBackend_AppendToSide_MultipleOrdersSamePrice(t *testing.T) {
	client := setupTestRedis(t)
	backend := NewRedisBackend(client, "test:multisameprice:")
	price := fpdecimal.FromFloat(100.0)
	qty := fpdecimal.FromFloat(1.0)

	// Create orders
	order1, err := core.NewLimitOrder("order-1", core.Buy, qty, price, core.GTC, "")
	require.NoError(t, err)
	order2, err := core.NewLimitOrder("order-2", core.Buy, qty, price, core.GTC, "")
	require.NoError(t, err)

	// Store orders
	require.NoError(t, backend.StoreOrder(order1))
	require.NoError(t, backend.StoreOrder(order2))

	// Append both to side
	backend.AppendToSide(core.Buy, order1)
	backend.AppendToSide(core.Buy, order2)

	// Verify presence
	ctx := context.Background()
	priceKey := fmt.Sprintf("%s:%s", backend.bidsKey, price.String())
	exists, err := client.SIsMember(ctx, priceKey, "order-1").Result()
	require.NoError(t, err)
	assert.True(t, exists, "Expected order-1 to exist")

	exists, err = client.SIsMember(ctx, priceKey, "order-2").Result()
	require.NoError(t, err)
	assert.True(t, exists, "Expected order-2 to exist")

	// Remove one order
	removed := backend.RemoveFromSide(core.Buy, order1)
	assert.True(t, removed)

	// Verify state after removal
	exists, err = client.SIsMember(ctx, priceKey, "order-1").Result()
	require.NoError(t, err)
	assert.False(t, exists, "Expected order-1 to be removed")

	exists, err = client.SIsMember(ctx, priceKey, "order-2").Result()
	require.NoError(t, err)
	assert.True(t, exists, "Expected order-2 to still exist")

	// Verify price level still exists in ZSET
	count, err := client.ZCard(ctx, backend.bidsKey).Result()
	require.NoError(t, err)
	assert.Equal(t, int64(1), count, "Price level should still exist in ZSET")

	// Remove the second order
	removed = backend.RemoveFromSide(core.Buy, order2)
	assert.True(t, removed)

	// Verify price level is removed from ZSET when list is empty
	countAfter, err := client.ZCard(ctx, backend.bidsKey).Result()
	require.NoError(t, err)
	assert.Equal(t, int64(0), countAfter, "Price level should be removed from ZSET")
}

func TestRedisBackend_GetComponents(t *testing.T) {
	client := setupTestRedis(t)
	backend := NewRedisBackend(client, "test")

	bids := backend.GetBids()
	if bids == nil {
		t.Error("Expected non-nil bids")
	}

	asks := backend.GetAsks()
	if asks == nil {
		t.Error("Expected non-nil asks")
	}

	stopBook := backend.GetStopBook()
	if stopBook == nil {
		t.Error("Expected non-nil stopBook")
	}
}

func TestRedisBackend_BasicOperations(t *testing.T) {
	client := setupTestRedis(t)
	backend := NewRedisBackend(client, "test")

	// Test storing and retrieving an order
	orderID := "test-123"
	price := fpdecimal.FromFloat(100.0)
	quantity := fpdecimal.FromFloat(10.0)
	order, err := core.NewLimitOrder(orderID, core.Buy, quantity, price, core.GTC, "")
	require.NoError(t, err)

	// Initially should be nil
	initialOrder := backend.GetOrder(orderID)
	if initialOrder != nil {
		t.Errorf("Expected nil order before storage, got %v", initialOrder)
	}

	// Store order
	err = backend.StoreOrder(order)
	if err != nil {
		t.Fatalf("Failed to store order: %v", err)
	}

	// Retrieve order
	storedOrder := backend.GetOrder(orderID)
	if storedOrder == nil {
		t.Fatal("Expected non-nil order after storage")
	}

	if storedOrder.ID() != orderID {
		t.Errorf("Expected ID %s, got %s", orderID, storedOrder.ID())
	}

	// Delete order
	backend.DeleteOrder(orderID)

	// Verify order is deleted
	deletedOrder := backend.GetOrder(orderID)
	if deletedOrder != nil {
		t.Error("Expected nil order after deletion")
	}
}

func TestRedisBackend_StopBookInterface(t *testing.T) {
	client := setupTestRedis(t)
	defer client.Close()

	backend := NewRedisBackend(client, "test:testrstopbook")

	// 1. Create some stop orders
	buyStopOrder1, err := core.NewStopLimitOrder("stop-buy-1", core.Buy, fpdecimal.FromFloat(5.0), fpdecimal.FromFloat(100.0), fpdecimal.FromFloat(105.0), "")
	require.NoError(t, err)
	buyStopOrder2, err := core.NewStopLimitOrder("stop-buy-2", core.Buy, fpdecimal.FromFloat(3.0), fpdecimal.FromFloat(99.0), fpdecimal.FromFloat(104.0), "")
	require.NoError(t, err)
	sellStopOrder1, err := core.NewStopLimitOrder("stop-sell-1", core.Sell, fpdecimal.FromFloat(4.0), fpdecimal.FromFloat(110.0), fpdecimal.FromFloat(100.0), "")
	require.NoError(t, err)
	sellStopOrder2, err := core.NewStopLimitOrder("stop-sell-2", core.Sell, fpdecimal.FromFloat(2.0), fpdecimal.FromFloat(112.0), fpdecimal.FromFloat(101.0), "")
	require.NoError(t, err)

	// 2. Store the orders
	err = backend.StoreOrder(buyStopOrder1)
	require.NoError(t, err)
	err = backend.StoreOrder(buyStopOrder2)
	require.NoError(t, err)
	err = backend.StoreOrder(sellStopOrder1)
	require.NoError(t, err)
	err = backend.StoreOrder(sellStopOrder2)
	require.NoError(t, err)

	// 3. Add to stop book
	backend.AppendToStopBook(buyStopOrder1)
	backend.AppendToStopBook(buyStopOrder2)
	backend.AppendToStopBook(sellStopOrder1)
	backend.AppendToStopBook(sellStopOrder2)

	// 4. Get the stop book and test the interface
	stopBook := backend.GetStopBook().(*RedisStopBook)

	// Test BuyOrders()
	buyOrders := stopBook.BuyOrders()
	require.Len(t, buyOrders, 2, "Should have 2 buy stop orders")

	// Orders should be returned, but the order might vary
	buyOrderIDs := make([]string, len(buyOrders))
	for i, order := range buyOrders {
		buyOrderIDs[i] = order.ID()
	}
	assert.Contains(t, buyOrderIDs, "stop-buy-1")
	assert.Contains(t, buyOrderIDs, "stop-buy-2")

	// Test SellOrders()
	sellOrders := stopBook.SellOrders()
	require.Len(t, sellOrders, 2, "Should have 2 sell stop orders")

	// Orders should be returned, but the order might vary
	sellOrderIDs := make([]string, len(sellOrders))
	for i, order := range sellOrders {
		sellOrderIDs[i] = order.ID()
	}
	assert.Contains(t, sellOrderIDs, "stop-sell-1")
	assert.Contains(t, sellOrderIDs, "stop-sell-2")

	// Test Prices()
	prices := stopBook.Prices()
	assert.Len(t, prices, 4, "Should have 4 unique price levels")

	// Convert to string for easier comparison
	priceStrings := make([]string, len(prices))
	for i, price := range prices {
		priceStrings[i] = price.String()
	}
	assert.Contains(t, priceStrings, "104.000")
	assert.Contains(t, priceStrings, "105.000")
	assert.Contains(t, priceStrings, "100.000")
	assert.Contains(t, priceStrings, "101.000")

	// Test Orders() at specific price
	buyOrders105 := stopBook.Orders(fpdecimal.FromFloat(105.0))
	require.Len(t, buyOrders105, 1, "Should have 1 buy order at price 105")
	assert.Equal(t, "stop-buy-1", buyOrders105[0].ID())

	sellOrders100 := stopBook.Orders(fpdecimal.FromFloat(100.0))
	require.Len(t, sellOrders100, 1, "Should have 1 sell order at price 100")
	assert.Equal(t, "stop-sell-1", sellOrders100[0].ID())
}
