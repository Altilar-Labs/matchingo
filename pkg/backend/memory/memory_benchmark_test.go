package memory

import (
	"fmt"
	"testing"

	"github.com/nikolaydubina/fpdecimal"
	"github.com/erain9/matchingo/pkg/core"
)

func BenchmarkMemoryBackend_StoreOrder(b *testing.B) {
	backend := NewMemoryBackend()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		orderID := fmt.Sprintf("order-%d", i)
		price := fpdecimal.FromFloat(100.0)
		quantity := fpdecimal.FromFloat(10.0)
		order := core.NewLimitOrder(orderID, core.Buy, quantity, price, core.GTC, "")
		_ = backend.StoreOrder(order)
	}
}

func BenchmarkMemoryBackend_GetOrder(b *testing.B) {
	backend := NewMemoryBackend()

	// Store some orders first
	numOrders := 1000
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

func BenchmarkMemoryBackend_AppendToSide(b *testing.B) {
	backend := NewMemoryBackend()

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

func BenchmarkMemoryBackend_RemoveFromSide(b *testing.B) {
	backend := NewMemoryBackend()

	// Store and add to side some orders first
	numOrders := 100 // Reduced for faster execution
	orders := make([]*core.Order, numOrders)

	for i := 0; i < numOrders; i++ {
		orderID := fmt.Sprintf("order-%d", i)
		price := fpdecimal.FromFloat(float64(100 + (i % 100)))
		quantity := fpdecimal.FromFloat(10.0)
		order := core.NewLimitOrder(orderID, core.Buy, quantity, price, core.GTC, "")
		_ = backend.StoreOrder(order)
		backend.AppendToSide(core.Buy, order)
		orders[i] = order
	}

	// Pre-reset timer to exclude setup time
	b.ResetTimer()

	// Only run up to b.N iterations
	for i := 0; i < b.N; i++ {
		// Reset orders after we've gone through all of them
		if i%numOrders == 0 && i > 0 {
			b.StopTimer()
			// Re-add all orders to the side
			for j := 0; j < numOrders; j++ {
				backend.AppendToSide(core.Buy, orders[j])
			}
			b.StartTimer()
		}

		// Get order index, making sure we don't go out of bounds
		index := i % numOrders
		backend.RemoveFromSide(core.Buy, orders[index])
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

func BenchmarkOrderBook_LargeOrderBook_Memory(b *testing.B) {
	backend := NewMemoryBackend()
	book := core.NewOrderBook(backend)

	// Create a large order book with many price levels (reduced for faster execution)
	for i := 0; i < 200; i++ {
		// Add buy orders
		buyOrderID := fmt.Sprintf("buy-order-%d", i)
		buyPrice := fpdecimal.FromFloat(float64(90 - (i % 90)))
		buyQuantity := fpdecimal.FromFloat(10.0)
		buyOrder := core.NewLimitOrder(buyOrderID, core.Buy, buyQuantity, buyPrice, core.GTC, "")
		_, _ = book.Process(buyOrder)

		// Add sell orders
		sellOrderID := fmt.Sprintf("sell-order-%d", i)
		sellPrice := fpdecimal.FromFloat(float64(110 + (i % 90)))
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
