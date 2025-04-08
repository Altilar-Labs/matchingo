package redis

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/erain9/matchingo/pkg/core"
	"github.com/nikolaydubina/fpdecimal"
	"github.com/redis/go-redis/v9"
)

// skipIfNoRedis will skip the test if Redis is not available
func skipIfNoRedis(t *testing.B) *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	_, err := client.Ping(ctx).Result()
	if err != nil {
		t.Skipf("Skipping Redis tests, Redis not available: %v", err)
		return nil
	}

	return client
}

func BenchmarkRedisBackend_StoreOrder(b *testing.B) {
	client := skipIfNoRedis(b)
	if client == nil {
		return
	}
	defer client.Close()

	// Flush the database to start fresh
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client.FlushDB(ctx)

	backend := NewRedisBackend(client, "benchmark")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		orderID := fmt.Sprintf("order-%d", i)
		price := fpdecimal.FromFloat(100.0)
		quantity := fpdecimal.FromFloat(10.0)
		order := core.NewLimitOrder(orderID, core.Buy, quantity, price, core.GTC, "")
		_ = backend.StoreOrder(order)
	}
}

func BenchmarkRedisBackend_GetOrder(b *testing.B) {
	client := skipIfNoRedis(b)
	if client == nil {
		return
	}
	defer client.Close()

	// Flush the database to start fresh
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client.FlushDB(ctx)

	backend := NewRedisBackend(client, "benchmark")

	// Store some orders first
	numOrders := 100 // Using fewer orders for Redis to avoid timeout
	orderIDs := make([]string, numOrders)

	for i := 0; i < numOrders; i++ {
		orderID := fmt.Sprintf("order-%d", i)
		orderIDs[i] = orderID
		price := fpdecimal.FromFloat(100.0)
		quantity := fpdecimal.FromFloat(10.0)
		order := core.NewLimitOrder(orderID, core.Buy, quantity, price, core.GTC, "")
		_ = backend.StoreOrder(order)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		index := i % numOrders
		_ = backend.GetOrder(orderIDs[index])
	}
}

func BenchmarkRedisBackend_AppendToSide(b *testing.B) {
	client := skipIfNoRedis(b)
	if client == nil {
		return
	}
	defer client.Close()

	// Flush the database to start fresh
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client.FlushDB(ctx)

	backend := NewRedisBackend(client, "benchmark")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		orderID := fmt.Sprintf("order-%d", i)
		// Use different prices to test the sorting performance
		price := fpdecimal.FromFloat(float64(100 + (i % 100)))
		quantity := fpdecimal.FromFloat(10.0)
		order := core.NewLimitOrder(orderID, core.Buy, quantity, price, core.GTC, "")
		_ = backend.StoreOrder(order)
		backend.AppendToSide(core.Buy, order)
	}
}

func BenchmarkOrderBook_Process_Redis(b *testing.B) {
	client := skipIfNoRedis(b)
	if client == nil {
		return
	}
	defer client.Close()

	// Flush the database to start fresh
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client.FlushDB(ctx)

	backend := NewRedisBackend(client, "benchmark")
	book := core.NewOrderBook(backend)

	// Create sell orders to match against (fewer for Redis)
	for i := 0; i < 20; i++ {
		orderID := fmt.Sprintf("sell-order-%d", i)
		price := fpdecimal.FromFloat(float64(100 + i))
		quantity := fpdecimal.FromFloat(10.0)
		order := core.NewLimitOrder(orderID, core.Sell, quantity, price, core.GTC, "")
		_, _ = book.Process(order)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		orderID := fmt.Sprintf("buy-order-%d", i)
		price := fpdecimal.FromFloat(100.0) // Set price to match lowest sell order
		quantity := fpdecimal.FromFloat(1.0)
		order := core.NewLimitOrder(orderID, core.Buy, quantity, price, core.GTC, "")
		_, _ = book.Process(order)
	}
}

func BenchmarkOrderBook_SmallOrderBook_Redis(b *testing.B) {
	client := skipIfNoRedis(b)
	if client == nil {
		return
	}
	defer client.Close()

	// Flush the database to start fresh
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client.FlushDB(ctx)

	backend := NewRedisBackend(client, "benchmark")
	book := core.NewOrderBook(backend)

	// Create a smaller order book with fewer price levels (for Redis)
	for i := 0; i < 20; i++ { // Reduced from 50 to 20 for faster setup
		// Add buy orders
		buyOrderID := fmt.Sprintf("buy-order-%d", i)
		buyPrice := fpdecimal.FromFloat(float64(90 - (i % 10)))
		buyQuantity := fpdecimal.FromFloat(10.0)
		buyOrder := core.NewLimitOrder(buyOrderID, core.Buy, buyQuantity, buyPrice, core.GTC, "")
		_, _ = book.Process(buyOrder)

		// Add sell orders
		sellOrderID := fmt.Sprintf("sell-order-%d", i)
		sellPrice := fpdecimal.FromFloat(float64(110 + (i % 10)))
		sellQuantity := fpdecimal.FromFloat(10.0)
		sellOrder := core.NewLimitOrder(sellOrderID, core.Sell, sellQuantity, sellPrice, core.GTC, "")
		_, _ = book.Process(sellOrder)
	}

	b.ResetTimer()
	// Benchmark a market order that crosses the book
	for i := 0; i < b.N; i++ {
		orderID := fmt.Sprintf("market-order-%d", i)
		quantity := fpdecimal.FromFloat(5.0)

		// Alternate between buy and sell market orders
		side := core.Buy
		if i%2 == 0 {
			side = core.Sell
		}

		order := core.NewMarketOrder(orderID, side, quantity)
		_, _ = book.Process(order)
	}
}
