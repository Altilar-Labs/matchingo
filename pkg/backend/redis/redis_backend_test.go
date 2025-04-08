package redis

import (
	"context"
	"testing"

	"github.com/erain9/matchingo/pkg/core"
	"github.com/nikolaydubina/fpdecimal"
	"github.com/redis/go-redis/v9"
)

// skipRedisTestIfNoServer will skip the test if Redis is not available
func skipRedisTestIfNoServer(t *testing.T) *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})

	_, err := client.Ping(context.Background()).Result()
	if err != nil {
		t.Skipf("Skipping Redis tests, Redis not available: %v", err)
		return nil
	}

	return client
}

func TestNewRedisBackend(t *testing.T) {
	client := skipRedisTestIfNoServer(t)
	if client == nil {
		return
	}
	defer client.Close()

	backend := NewRedisBackend(client, "test")
	if backend == nil {
		t.Fatal("Expected non-nil backend")
	}

	// Check that keys are properly formatted
	if backend.orderKey != "test:order" {
		t.Errorf("Expected orderKey 'test:order', got %s", backend.orderKey)
	}

	if backend.bidsKey != "test:bids" {
		t.Errorf("Expected bidsKey 'test:bids', got %s", backend.bidsKey)
	}

	if backend.asksKey != "test:asks" {
		t.Errorf("Expected asksKey 'test:asks', got %s", backend.asksKey)
	}

	if backend.stopBuyKey != "test:stop:buy" {
		t.Errorf("Expected stopBuyKey 'test:stop:buy', got %s", backend.stopBuyKey)
	}

	if backend.stopSellKey != "test:stop:sell" {
		t.Errorf("Expected stopSellKey 'test:stop:sell', got %s", backend.stopSellKey)
	}

	if backend.ocoKey != "test:oco" {
		t.Errorf("Expected ocoKey 'test:oco', got %s", backend.ocoKey)
	}
}

func TestRedisBackend_BasicOperations(t *testing.T) {
	client := skipRedisTestIfNoServer(t)
	if client == nil {
		return
	}
	defer client.Close()

	// Flush the database to start fresh
	ctx := context.Background()
	client.FlushDB(ctx)

	backend := NewRedisBackend(client, "test")

	// Test storing and retrieving an order
	orderID := "test-123"
	price := fpdecimal.FromFloat(100.0)
	quantity := fpdecimal.FromFloat(10.0)
	order := core.NewLimitOrder(orderID, core.Buy, quantity, price, core.GTC, "")

	// Initially should be nil
	initialOrder := backend.GetOrder(orderID)
	if initialOrder != nil {
		t.Errorf("Expected nil order before storage, got %v", initialOrder)
	}

	// Store order
	err := backend.StoreOrder(order)
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
		t.Errorf("Expected nil order after deletion, got %v", deletedOrder)
	}
}

func TestRedisBackend_AppendToSide(t *testing.T) {
	client := skipRedisTestIfNoServer(t)
	if client == nil {
		return
	}
	defer client.Close()

	// Flush the database to start fresh
	ctx := context.Background()
	client.FlushDB(ctx)

	backend := NewRedisBackend(client, "test")

	// Create a buy order
	buyOrderID := "buy-123"
	buyPrice := fpdecimal.FromFloat(100.0)
	buyQuantity := fpdecimal.FromFloat(10.0)
	buyOrder := core.NewLimitOrder(buyOrderID, core.Buy, buyQuantity, buyPrice, core.GTC, "")

	// Store order
	_ = backend.StoreOrder(buyOrder)

	// Append to side
	backend.AppendToSide(core.Buy, buyOrder)

	// Create a sell order
	sellOrderID := "sell-123"
	sellPrice := fpdecimal.FromFloat(110.0)
	sellQuantity := fpdecimal.FromFloat(10.0)
	sellOrder := core.NewLimitOrder(sellOrderID, core.Sell, sellQuantity, sellPrice, core.GTC, "")

	// Store order
	_ = backend.StoreOrder(sellOrder)

	// Append to side
	backend.AppendToSide(core.Sell, sellOrder)
}

func TestRedisBackend_GetSides(t *testing.T) {
	client := skipRedisTestIfNoServer(t)
	if client == nil {
		return
	}
	defer client.Close()

	backend := NewRedisBackend(client, "test")

	// Check that GetBids, GetAsks, and GetStopBook return non-nil values
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

func TestRedisSide_String(t *testing.T) {
	client := skipRedisTestIfNoServer(t)
	if client == nil {
		return
	}
	defer client.Close()

	redisBackend := NewRedisBackend(client, "test")

	side := &RedisSide{
		backend: redisBackend,
		sideKey: "test:side",
		reverse: false,
	}

	// An empty side should return an empty string since no orders found
	str := side.String()
	if str != "" {
		t.Errorf("Expected empty string for empty side, got %q", str)
	}
}

func TestRedisStopBook_String(t *testing.T) {
	client := skipRedisTestIfNoServer(t)
	if client == nil {
		return
	}
	defer client.Close()

	redisBackend := NewRedisBackend(client, "test")

	stopBook := &RedisStopBook{
		backend:     redisBackend,
		stopBuyKey:  "test:stop:buy",
		stopSellKey: "test:stop:sell",
	}

	// An empty stop book should return the header text with a newline
	str := stopBook.String()
	expected := "Buy Stop Orders:\nSell Stop Orders:"
	if str != expected {
		t.Errorf("Expected %q for empty stop book, got %q", expected, str)
	}
}
