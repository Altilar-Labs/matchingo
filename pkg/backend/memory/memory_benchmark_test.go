package memory

import (
	"fmt"
	"testing"

	"github.com/erain9/matchingo/pkg/core"
	"github.com/nikolaydubina/fpdecimal"
	"github.com/stretchr/testify/require"
)

const benchSize = 10000

func benchmarkAppendToSide(b *testing.B, backend *MemoryBackend, side core.Side) {
	orders := make([]*core.Order, b.N)
	for i := 0; i < b.N; i++ {
		price := fpdecimal.FromInt(int64(10000 + i))
		qty := fpdecimal.FromInt(1)
		// Fix: Assign both return values and check error
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
	backend := NewMemoryBackend()
	benchmarkAppendToSide(b, backend, core.Buy)
}

func BenchmarkAppendToSide_Asks(b *testing.B) {
	backend := NewMemoryBackend()
	benchmarkAppendToSide(b, backend, core.Sell)
}

func benchmarkRemoveFromSide(b *testing.B, backend *MemoryBackend, side core.Side) {
	orders := make([]*core.Order, benchSize)
	for i := 0; i < benchSize; i++ {
		price := fpdecimal.FromInt(int64(10000 + i))
		qty := fpdecimal.FromInt(1)
		// Fix: Assign both return values and check error
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
	backend := NewMemoryBackend()
	benchmarkRemoveFromSide(b, backend, core.Buy)
}

func BenchmarkRemoveFromSide_Asks(b *testing.B) {
	backend := NewMemoryBackend()
	benchmarkRemoveFromSide(b, backend, core.Sell)
}

func BenchmarkMemoryBackend_StoreOrder(b *testing.B) {
	backend := NewMemoryBackend()
	orders := make([]*core.Order, b.N)
	for i := 0; i < b.N; i++ {
		orderID := fmt.Sprintf("order-%d", i)
		price := fpdecimal.FromFloat(float64(100 + i))
		quantity := fpdecimal.FromFloat(10.0)
		// Fix: Assign both return values and check error
		order, err := core.NewLimitOrder(orderID, core.Buy, quantity, price, core.GTC, "", "test_user")
		require.NoError(b, err)
		orders[i] = order
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = backend.StoreOrder(orders[i])
	}
}

func BenchmarkMemoryBackend_GetOrder(b *testing.B) {
	backend := NewMemoryBackend()
	orderIDs := make([]string, benchSize)
	for i := 0; i < benchSize; i++ {
		orderID := fmt.Sprintf("order-%d", i)
		orderIDs[i] = orderID
		price := fpdecimal.FromFloat(float64(100 + i))
		quantity := fpdecimal.FromFloat(10.0)
		// Fix: Assign both return values and check error
		order, err := core.NewLimitOrder(orderID, core.Buy, quantity, price, core.GTC, "", "test_user")
		require.NoError(b, err)
		_ = backend.StoreOrder(order)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = backend.GetOrder(orderIDs[i%benchSize])
	}
}

func BenchmarkMemoryBackend_UpdateOrder(b *testing.B) {
	backend := NewMemoryBackend()
	orders := make([]*core.Order, benchSize)
	for i := 0; i < benchSize; i++ {
		orderID := fmt.Sprintf("order-%d", i)
		price := fpdecimal.FromFloat(float64(100 + i))
		quantity := fpdecimal.FromFloat(10.0)
		// Fix: Assign both return values and check error
		order, err := core.NewLimitOrder(orderID, core.Buy, quantity, price, core.GTC, "", "test_user")
		require.NoError(b, err)
		orders[i] = order
		_ = backend.StoreOrder(order)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		order := orders[i%benchSize]
		// Modify quantity slightly for update
		order.SetQuantity(order.Quantity().Add(fpdecimal.FromFloat(0.1)))
		_ = backend.UpdateOrder(order)
	}
}

func BenchmarkMemoryBackend_DeleteOrder(b *testing.B) {
	backend := NewMemoryBackend()
	orderIDs := make([]string, benchSize)
	for i := 0; i < benchSize; i++ {
		orderID := fmt.Sprintf("order-%d", i)
		orderIDs[i] = orderID
		price := fpdecimal.FromFloat(float64(100 + i))
		quantity := fpdecimal.FromFloat(10.0)
		// Fix: Assign both return values and check error
		order, err := core.NewLimitOrder(orderID, core.Buy, quantity, price, core.GTC, "", "test_user")
		require.NoError(b, err)
		_ = backend.StoreOrder(order)
	}

	b.ResetTimer()
	// To avoid deleting the same order multiple times in the benchmark loop,
	// we'll delete and potentially re-add within the loop, or pre-populate more orders.
	// Simple approach for now: assume b.N <= benchSize
	for i := 0; i < b.N; i++ {
		backend.DeleteOrder(orderIDs[i])
	}
}

func BenchmarkOrderBook_Process_Memory(b *testing.B) {
	backend := NewMemoryBackend()
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

func BenchmarkOrderBook_LargeOrderBook_Memory(b *testing.B) {
	backend := NewMemoryBackend()
	book := core.NewOrderBook(backend)

	// Create a large order book with many price levels (reduced for faster execution)
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
