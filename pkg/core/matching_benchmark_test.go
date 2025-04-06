package core

import (
	"fmt"
	"testing"

	"github.com/nikolaydubina/fpdecimal"
)

// BenchmarkMarketOrderMatching tests the performance of market order matching
func BenchmarkMarketOrderMatching(b *testing.B) {
	// Create a new order book with an in-memory backend
	backend := newMockBackend()
	book := NewOrderBook(backend)

	// Prepare the order book with sell orders at different price levels
	// We'll create a realistic orderbook with orders at various price points
	for i := 0; i < 100; i++ {
		sellID := fmt.Sprintf("sell-%d", i)
		// Create price points from 100.00 to 110.00
		price := fpdecimal.FromFloat(100.0 + float64(i)*0.1)
		// Create varying quantity sizes
		quantity := fpdecimal.FromFloat(1.0 + float64(i%5))

		sellOrder := NewLimitOrder(sellID, Sell, quantity, price, GTC, "")
		_, _ = book.Process(sellOrder)
	}

	// Reset the timer to not include setup time
	b.ResetTimer()

	// Run the benchmark
	for i := 0; i < b.N; i++ {
		// Create a buy market order
		buyID := fmt.Sprintf("buy-market-%d", i)
		buyQty := fpdecimal.FromFloat(3.0) // Small enough to not deplete the book

		buyOrder := NewMarketOrder(buyID, Buy, buyQty)
		_, _ = book.Process(buyOrder)

		// We don't need to restore the book since we're not depleting it
	}
}

// BenchmarkLimitOrderMatching tests the performance of limit order matching
func BenchmarkLimitOrderMatching(b *testing.B) {
	// Create a new order book with an in-memory backend
	backend := newMockBackend()
	book := NewOrderBook(backend)

	// Prepare the order book with sell orders at different price levels
	for i := 0; i < 100; i++ {
		sellID := fmt.Sprintf("sell-%d", i)
		price := fpdecimal.FromFloat(100.0 + float64(i)*0.1)
		quantity := fpdecimal.FromFloat(1.0 + float64(i%5))

		sellOrder := NewLimitOrder(sellID, Sell, quantity, price, GTC, "")
		_, _ = book.Process(sellOrder)
	}

	// Reset the timer to not include setup time
	b.ResetTimer()

	// Run the benchmark
	for i := 0; i < b.N; i++ {
		// Create a buy limit order that will match with some sell orders
		buyID := fmt.Sprintf("buy-limit-%d", i)
		buyPrice := fpdecimal.FromFloat(100.5) // Will match with some sells
		buyQty := fpdecimal.FromFloat(2.0)

		buyOrder := NewLimitOrder(buyID, Buy, buyQty, buyPrice, GTC, "")
		_, _ = book.Process(buyOrder)
	}
}

// BenchmarkMultiLevelMatching tests matching across multiple price levels
func BenchmarkMultiLevelMatching(b *testing.B) {
	// Create a new order book with an in-memory backend
	backend := newMockBackend()
	book := NewOrderBook(backend)

	// Prepare the order book with sell orders at different price levels
	// Create a more realistic order book with dense price levels
	for i := 0; i < 50; i++ {
		// Create multiple orders at each price level
		for j := 0; j < 5; j++ {
			sellID := fmt.Sprintf("sell-%d-%d", i, j)
			price := fpdecimal.FromFloat(100.0 + float64(i)*0.1)
			quantity := fpdecimal.FromFloat(1.0)

			sellOrder := NewLimitOrder(sellID, Sell, quantity, price, GTC, "")
			_, _ = book.Process(sellOrder)
		}
	}

	// Reset the timer to not include setup time
	b.ResetTimer()

	// Run the benchmark
	for i := 0; i < b.N; i++ {
		// Create a buy order that will match across multiple price levels
		buyID := fmt.Sprintf("buy-multi-%d", i)
		buyPrice := fpdecimal.FromFloat(102.0) // Will match with many sells
		buyQty := fpdecimal.FromFloat(10.0)    // Will consume multiple levels

		buyOrder := NewLimitOrder(buyID, Buy, buyQty, buyPrice, GTC, "")
		_, _ = book.Process(buyOrder)

		// Restore the book for the next iteration
		if i < b.N-1 {
			b.StopTimer()
			// Add back the orders that were consumed
			for j := 0; j < 10; j++ {
				restoreID := fmt.Sprintf("restore-%d-%d", i, j)
				price := fpdecimal.FromFloat(100.0 + float64(j)*0.1)
				quantity := fpdecimal.FromFloat(1.0)

				restoreOrder := NewLimitOrder(restoreID, Sell, quantity, price, GTC, "")
				_, _ = book.Process(restoreOrder)
			}
			b.StartTimer()
		}
	}
}

// BenchmarkHighFrequencyMatching tests high frequency order matching
func BenchmarkHighFrequencyMatching(b *testing.B) {
	// Create a new order book with an in-memory backend
	backend := newMockBackend()
	book := NewOrderBook(backend)

	// Prepare the order book with both buy and sell orders
	// Adding buy orders at lower prices
	for i := 0; i < 50; i++ {
		buyID := fmt.Sprintf("buy-book-%d", i)
		price := fpdecimal.FromFloat(99.0 - float64(i)*0.1)
		quantity := fpdecimal.FromFloat(1.0)

		buyOrder := NewLimitOrder(buyID, Buy, quantity, price, GTC, "")
		_, _ = book.Process(buyOrder)
	}

	// Adding sell orders at higher prices
	for i := 0; i < 50; i++ {
		sellID := fmt.Sprintf("sell-book-%d", i)
		price := fpdecimal.FromFloat(101.0 + float64(i)*0.1)
		quantity := fpdecimal.FromFloat(1.0)

		sellOrder := NewLimitOrder(sellID, Sell, quantity, price, GTC, "")
		_, _ = book.Process(sellOrder)
	}

	// Reset the timer to not include setup time
	b.ResetTimer()

	// Run the benchmark - alternating between buy and sell orders
	for i := 0; i < b.N; i++ {
		if i%2 == 0 {
			// Create a buy order at market price
			buyID := fmt.Sprintf("buy-hf-%d", i)
			buyQty := fpdecimal.FromFloat(1.0)

			buyOrder := NewMarketOrder(buyID, Buy, buyQty)
			_, _ = book.Process(buyOrder)

			// Restore a sell order
			if i < b.N-1 {
				b.StopTimer()
				restoreID := fmt.Sprintf("restore-sell-%d", i)
				price := fpdecimal.FromFloat(101.0)
				quantity := fpdecimal.FromFloat(1.0)

				restoreOrder := NewLimitOrder(restoreID, Sell, quantity, price, GTC, "")
				_, _ = book.Process(restoreOrder)
				b.StartTimer()
			}
		} else {
			// Create a sell order at market price
			sellID := fmt.Sprintf("sell-hf-%d", i)
			sellQty := fpdecimal.FromFloat(1.0)

			sellOrder := NewMarketOrder(sellID, Sell, sellQty)
			_, _ = book.Process(sellOrder)

			// Restore a buy order
			if i < b.N-1 {
				b.StopTimer()
				restoreID := fmt.Sprintf("restore-buy-%d", i)
				price := fpdecimal.FromFloat(99.0)
				quantity := fpdecimal.FromFloat(1.0)

				restoreOrder := NewLimitOrder(restoreID, Buy, quantity, price, GTC, "")
				_, _ = book.Process(restoreOrder)
				b.StartTimer()
			}
		}
	}
}

// BenchmarkHeavyLoad tests order book under heavy load
func BenchmarkHeavyLoad(b *testing.B) {
	// Create a new order book with an in-memory backend
	backend := newMockBackend()
	book := NewOrderBook(backend)

	// Prepare a large order book with many price levels and orders
	for i := 0; i < 500; i++ {
		// Add sell orders
		sellID := fmt.Sprintf("sell-load-%d", i)
		sellPrice := fpdecimal.FromFloat(100.0 + float64(i%100)*0.1)
		sellQty := fpdecimal.FromFloat(1.0 + float64(i%10))

		sellOrder := NewLimitOrder(sellID, Sell, sellQty, sellPrice, GTC, "")
		_, _ = book.Process(sellOrder)

		// Add buy orders
		buyID := fmt.Sprintf("buy-load-%d", i)
		buyPrice := fpdecimal.FromFloat(99.0 - float64(i%100)*0.1)
		buyQty := fpdecimal.FromFloat(1.0 + float64(i%10))

		buyOrder := NewLimitOrder(buyID, Buy, buyQty, buyPrice, GTC, "")
		_, _ = book.Process(buyOrder)
	}

	// Reset the timer to not include setup time
	b.ResetTimer()

	// Run the benchmark - large market orders
	for i := 0; i < b.N; i++ {
		// Alternate between buy and sell
		side := Buy
		if i%2 == 1 {
			side = Sell
		}

		// Create a large order that will match with many orders
		orderID := fmt.Sprintf("heavy-load-%d", i)
		quantity := fpdecimal.FromFloat(50.0) // Match with many orders

		order := NewMarketOrder(orderID, side, quantity)
		_, _ = book.Process(order)

		// Restore the book
		if i < b.N-1 {
			b.StopTimer()

			// Add back a bunch of orders on the side that was just depleted
			restoreSide := Sell
			restoreBasePrice := 100.0
			if side == Sell {
				restoreSide = Buy
				restoreBasePrice = 99.0
			}

			for j := 0; j < 50; j++ {
				restoreID := fmt.Sprintf("restore-heavy-%d-%d", i, j)
				var price fpdecimal.Decimal
				if restoreSide == Sell {
					price = fpdecimal.FromFloat(restoreBasePrice + float64(j%10)*0.1)
				} else {
					price = fpdecimal.FromFloat(restoreBasePrice - float64(j%10)*0.1)
				}
				quantity := fpdecimal.FromFloat(1.0 + float64(j%5))

				restoreOrder := NewLimitOrder(restoreID, restoreSide, quantity, price, GTC, "")
				_, _ = book.Process(restoreOrder)
			}

			b.StartTimer()
		}
	}
}
