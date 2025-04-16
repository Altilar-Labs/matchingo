package redis

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/erain9/matchingo/pkg/core"
	"github.com/nikolaydubina/fpdecimal"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

const benchSize = 10000

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

func benchmarkAppendToSide(b *testing.B, backend *RedisBackend, side core.Side) {
	orders := make([]*core.Order, b.N)
	for i := 0; i < b.N; i++ {
		price := fpdecimal.FromInt(int64(10000 + i))
		qty := fpdecimal.FromInt(1)
		order, err := core.NewLimitOrder(fmt.Sprintf("order-%d", i), side, qty, price, core.GTC, "", "test_user")
		require.NoError(b, err)
		orders[i] = order
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		backend.AppendToSide(side, orders[i])
	}
}

func BenchmarkAppendToSide_Bids(b *testing.B) {
	client := skipIfNoRedis(b)
	if client == nil {
		return
	}
	defer client.Close()

	// Flush the database to start fresh
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client.FlushDB(ctx)

	backend := NewRedisBackend(client, "bench:bids:")
	benchmarkAppendToSide(b, backend, core.Buy)
}

func BenchmarkAppendToSide_Asks(b *testing.B) {
	client := skipIfNoRedis(b)
	if client == nil {
		return
	}
	defer client.Close()

	// Flush the database to start fresh
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client.FlushDB(ctx)

	backend := NewRedisBackend(client, "bench:asks:")
	benchmarkAppendToSide(b, backend, core.Sell)
}

func benchmarkRemoveFromSide(b *testing.B, backend *RedisBackend, side core.Side) {
	orders := make([]*core.Order, benchSize)
	for i := 0; i < benchSize; i++ {
		price := fpdecimal.FromInt(int64(10000 + i))
		qty := fpdecimal.FromInt(1)
		order, err := core.NewLimitOrder(fmt.Sprintf("order-%d", i), side, qty, price, core.GTC, "", "test_user")
		require.NoError(b, err)
		orders[i] = order
		backend.AppendToSide(side, order)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		backend.RemoveFromSide(side, orders[i%benchSize])
	}
}

func BenchmarkRemoveFromSide_Bids(b *testing.B) {
	client := skipIfNoRedis(b)
	if client == nil {
		return
	}
	defer client.Close()

	// Flush the database to start fresh
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client.FlushDB(ctx)

	backend := NewRedisBackend(client, "bench:bids:")
	benchmarkRemoveFromSide(b, backend, core.Buy)
}

func BenchmarkRemoveFromSide_Asks(b *testing.B) {
	client := skipIfNoRedis(b)
	if client == nil {
		return
	}
	defer client.Close()

	// Flush the database to start fresh
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client.FlushDB(ctx)

	backend := NewRedisBackend(client, "bench:asks:")
	benchmarkRemoveFromSide(b, backend, core.Sell)
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

	backend := NewRedisBackend(client, "bench:store:")
	orders := make([]*core.Order, b.N)
	for i := 0; i < b.N; i++ {
		orderID := fmt.Sprintf("order-%d", i)
		price := fpdecimal.FromFloat(float64(100 + i))
		quantity := fpdecimal.FromFloat(10.0)
		order, err := core.NewLimitOrder(orderID, core.Buy, quantity, price, core.GTC, "", "test_user")
		require.NoError(b, err)
		orders[i] = order
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = backend.StoreOrder(orders[i])
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

	backend := NewRedisBackend(client, "bench:get:")
	orderIDs := make([]string, benchSize)
	for i := 0; i < benchSize; i++ {
		orderID := fmt.Sprintf("order-%d", i)
		orderIDs[i] = orderID
		price := fpdecimal.FromFloat(float64(100 + i))
		quantity := fpdecimal.FromFloat(10.0)
		order, err := core.NewLimitOrder(orderID, core.Buy, quantity, price, core.GTC, "", "test_user")
		require.NoError(b, err)
		_ = backend.StoreOrder(order)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = backend.GetOrder(orderIDs[i%benchSize])
	}
}

func BenchmarkRedisBackend_UpdateOrder(b *testing.B) {
	client := skipIfNoRedis(b)
	if client == nil {
		return
	}
	defer client.Close()

	// Flush the database to start fresh
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client.FlushDB(ctx)

	backend := NewRedisBackend(client, "bench:update:")
	orders := make([]*core.Order, benchSize)
	for i := 0; i < benchSize; i++ {
		orderID := fmt.Sprintf("order-%d", i)
		price := fpdecimal.FromFloat(float64(100 + i))
		quantity := fpdecimal.FromFloat(10.0)
		order, err := core.NewLimitOrder(orderID, core.Buy, quantity, price, core.GTC, "", "test_user")
		require.NoError(b, err)
		orders[i] = order
		_ = backend.StoreOrder(order)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		order := orders[i%benchSize]
		order.SetQuantity(order.Quantity().Add(fpdecimal.FromFloat(0.1)))
		_ = backend.UpdateOrder(order)
	}
}

func BenchmarkRedisBackend_DeleteOrder(b *testing.B) {
	client := skipIfNoRedis(b)
	if client == nil {
		return
	}
	defer client.Close()

	// Flush the database to start fresh
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client.FlushDB(ctx)

	backend := NewRedisBackend(client, "bench:delete:")
	orders := make([]*core.Order, benchSize)
	for i := 0; i < benchSize; i++ {
		orderID := fmt.Sprintf("order-%d", i)
		price := fpdecimal.FromFloat(float64(100 + i))
		quantity := fpdecimal.FromFloat(10.0)
		order, err := core.NewLimitOrder(orderID, core.Buy, quantity, price, core.GTC, "", "test_user")
		require.NoError(b, err)
		orders[i] = order
		_ = backend.StoreOrder(order)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		backend.DeleteOrder(orders[i%benchSize].ID())
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

	backend := NewRedisBackend(client, "bench:process:")
	book := core.NewOrderBook(backend)

	// Create sell orders to match against
	for i := 0; i < 100; i++ {
		orderID := fmt.Sprintf("sell-order-%d", i)
		price := fpdecimal.FromFloat(float64(100 + i))
		quantity := fpdecimal.FromFloat(10.0)
		order, err := core.NewLimitOrder(orderID, core.Sell, quantity, price, core.GTC, "", "test_user")
		require.NoError(b, err)
		_, err = book.Process(order)
		require.NoError(b, err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		orderID := fmt.Sprintf("buy-order-%d", i)
		quantity := fpdecimal.FromFloat(1.0)
		order, err := core.NewMarketOrder(orderID, core.Buy, quantity, "test_user")
		require.NoError(b, err)
		_, err = book.Process(order)
		require.NoError(b, err)
	}
}

func BenchmarkOrderBook_LargeOrderBook_Redis(b *testing.B) {
	client := skipIfNoRedis(b)
	if client == nil {
		return
	}
	defer client.Close()

	// Flush the database to start fresh
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client.FlushDB(ctx)

	backend := NewRedisBackend(client, "bench:large:")
	book := core.NewOrderBook(backend)

	// Create a large order book with many price levels
	for i := 0; i < 200; i++ {
		// Add buy orders
		buyOrderID := fmt.Sprintf("buy-order-%d", i)
		buyPrice := fpdecimal.FromFloat(float64(90 - (i % 90)))
		buyQuantity := fpdecimal.FromFloat(10.0)
		buyOrder, err := core.NewLimitOrder(buyOrderID, core.Buy, buyQuantity, buyPrice, core.GTC, "", "test_user")
		require.NoError(b, err)
		_, err = book.Process(buyOrder)
		require.NoError(b, err)

		// Add sell orders
		sellOrderID := fmt.Sprintf("sell-order-%d", i)
		sellPrice := fpdecimal.FromFloat(float64(110 + (i % 90)))
		sellQuantity := fpdecimal.FromFloat(10.0)
		sellOrder, err := core.NewLimitOrder(sellOrderID, core.Sell, sellQuantity, sellPrice, core.GTC, "", "test_user")
		require.NoError(b, err)
		_, err = book.Process(sellOrder)
		require.NoError(b, err)
	}

	b.ResetTimer()
	// Benchmark a market order that crosses the book
	for i := 0; i < b.N; i++ {
		orderID := fmt.Sprintf("market-order-%d", i)
		quantity := fpdecimal.FromFloat(5.0)
		order, err := core.NewMarketOrder(orderID, core.Buy, quantity, "test_user")
		require.NoError(b, err)
		_, err = book.Process(order)
		require.NoError(b, err)
	}
}
