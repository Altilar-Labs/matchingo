package core

import (
	"fmt"
	"testing"

	// No core import needed here
	// "github.com/erain9/matchingo/pkg/core" // Removed import
	"github.com/nikolaydubina/fpdecimal"
	"github.com/stretchr/testify/require"
)

// Helper function for benchmarks
func benchmarkMatchLimitOrder(b *testing.B, backend OrderBookBackend, numOrders int) {
	book := NewOrderBook(backend)
	// Pre-fill the book
	for i := 0; i < numOrders; i++ {
		price := fpdecimal.FromInt(int64(10000 + i))
		qty := fpdecimal.FromInt(1)
		o, err := NewLimitOrder(fmt.Sprintf("setup-sell-%d", i), Sell, qty, price, GTC, "")
		require.NoError(b, err)
		_, err = book.Process(o)
		require.NoError(b, err)
	}

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		id := fmt.Sprintf("bench-buy-%d", n)
		price := fpdecimal.FromInt(10000) // Match the lowest sell price
		qty := fpdecimal.FromInt(1)
		order, err := NewLimitOrder(id, Buy, qty, price, GTC, "")
		require.NoError(b, err)
		_, err = book.Process(order)
		require.NoError(b, err)
	}
}

func benchmarkMatchMarketOrder(b *testing.B, backend OrderBookBackend, numOrders int) {
	book := NewOrderBook(backend)
	// Pre-fill the book
	for i := 0; i < numOrders; i++ {
		price := fpdecimal.FromInt(int64(10000 + i))
		qty := fpdecimal.FromInt(1)
		o, err := NewLimitOrder(fmt.Sprintf("setup-sell-%d", i), Sell, qty, price, GTC, "")
		require.NoError(b, err)
		_, err = book.Process(o)
		require.NoError(b, err)
	}

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		id := fmt.Sprintf("bench-buy-%d", n)
		qty := fpdecimal.FromInt(1)
		order, err := NewMarketOrder(id, Buy, qty)
		require.NoError(b, err)
		_, err = book.Process(order)
		require.NoError(b, err)
	}
}

func benchmarkAddLimitOrder(b *testing.B, backend OrderBookBackend, numOrders int) {
	book := NewOrderBook(backend)
	// Pre-fill if needed (e.g., to test adding to a non-empty book)
	// For add benchmark, maybe start empty?

	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		id := fmt.Sprintf("bench-add-%d", n)
		price := fpdecimal.FromInt(int64(10000 + n)) // Unique price to ensure addition
		qty := fpdecimal.FromInt(1)
		order, err := NewLimitOrder(id, Buy, qty, price, GTC, "")
		require.NoError(b, err)
		_, err = book.Process(order)
		require.NoError(b, err)
	}
}

func benchmarkCancelLimitOrder(b *testing.B, backend OrderBookBackend, numOrders int) {
	book := NewOrderBook(backend)
	orderIDs := make([]string, numOrders)
	// Pre-fill the book
	for i := 0; i < numOrders; i++ {
		id := fmt.Sprintf("setup-cancel-%d", i)
		orderIDs[i] = id
		price := fpdecimal.FromInt(int64(10000 + i))
		qty := fpdecimal.FromInt(1)
		o, err := NewLimitOrder(id, Sell, qty, price, GTC, "")
		require.NoError(b, err)
		_, err = book.Process(o)
		require.NoError(b, err)
	}

	idx := 0
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		cancelID := orderIDs[idx%numOrders]
		book.CancelOrder(cancelID)
		idx++

		// Re-add an order to keep the book size roughly constant
		if n < numOrders { // Only re-add if we have space (simplified logic)
			reAddID := fmt.Sprintf("re-add-%d", n)
			price := fpdecimal.FromInt(int64(20000 + n))
			qty := fpdecimal.FromInt(1)
			order, err := NewLimitOrder(reAddID, Sell, qty, price, GTC, "")
			require.NoError(b, err)
			_, err = book.Process(order)
			require.NoError(b, err)
		}
	}
}

func benchmarkGetOrder(b *testing.B, backend OrderBookBackend, numOrders int) {
	book := NewOrderBook(backend)
	orderIDs := make([]string, numOrders)
	// Pre-fill the book
	for i := 0; i < numOrders; i++ {
		id := fmt.Sprintf("setup-get-%d", i)
		orderIDs[i] = id
		price := fpdecimal.FromInt(int64(10000 + i))
		qty := fpdecimal.FromInt(1)
		o, err := NewLimitOrder(id, Sell, qty, price, GTC, "")
		require.NoError(b, err)
		_, err = book.Process(o)
		require.NoError(b, err)
	}

	idx := 0
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		getID := orderIDs[idx%numOrders]
		_ = book.GetOrder(getID)
		idx++
	}
}

// --- Benchmark Execution --- (Add more backends as needed)

const (
	benchOrdersSmall  = 100
	benchOrdersMedium = 1000
	benchOrdersLarge  = 10000
)

// Remove getMockBackend helper - benchmarks should use newMockBackend directly
/*
func getMockBackend() OrderBookBackend {
	panic("Need access to core's mock backend constructor for benchmarks in core_test package")
	// return newMockBackend() // Use directly if package is core
}
*/

func BenchmarkMemory_MatchLimitOrder_Small(b *testing.B) {
	benchmarkMatchLimitOrder(b, newMockBackend(), benchOrdersSmall)
}
func BenchmarkMemory_MatchLimitOrder_Medium(b *testing.B) {
	benchmarkMatchLimitOrder(b, newMockBackend(), benchOrdersMedium)
}
func BenchmarkMemory_MatchLimitOrder_Large(b *testing.B) {
	benchmarkMatchLimitOrder(b, newMockBackend(), benchOrdersLarge)
}

func BenchmarkMemory_MatchMarketOrder_Small(b *testing.B) {
	benchmarkMatchMarketOrder(b, newMockBackend(), benchOrdersSmall)
}
func BenchmarkMemory_MatchMarketOrder_Medium(b *testing.B) {
	benchmarkMatchMarketOrder(b, newMockBackend(), benchOrdersMedium)
}
func BenchmarkMemory_MatchMarketOrder_Large(b *testing.B) {
	benchmarkMatchMarketOrder(b, newMockBackend(), benchOrdersLarge)
}

func BenchmarkMemory_AddLimitOrder_Small(b *testing.B) {
	benchmarkAddLimitOrder(b, newMockBackend(), benchOrdersSmall)
}
func BenchmarkMemory_AddLimitOrder_Medium(b *testing.B) {
	benchmarkAddLimitOrder(b, newMockBackend(), benchOrdersMedium)
}
func BenchmarkMemory_AddLimitOrder_Large(b *testing.B) {
	benchmarkAddLimitOrder(b, newMockBackend(), benchOrdersLarge)
}

func BenchmarkMemory_CancelLimitOrder_Small(b *testing.B) {
	benchmarkCancelLimitOrder(b, newMockBackend(), benchOrdersSmall)
}
func BenchmarkMemory_CancelLimitOrder_Medium(b *testing.B) {
	benchmarkCancelLimitOrder(b, newMockBackend(), benchOrdersMedium)
}
func BenchmarkMemory_CancelLimitOrder_Large(b *testing.B) {
	benchmarkCancelLimitOrder(b, newMockBackend(), benchOrdersLarge)
}

func BenchmarkMemory_GetOrder_Small(b *testing.B) {
	benchmarkGetOrder(b, newMockBackend(), benchOrdersSmall)
}
func BenchmarkMemory_GetOrder_Medium(b *testing.B) {
	benchmarkGetOrder(b, newMockBackend(), benchOrdersMedium)
}
func BenchmarkMemory_GetOrder_Large(b *testing.B) {
	benchmarkGetOrder(b, newMockBackend(), benchOrdersLarge)
}
