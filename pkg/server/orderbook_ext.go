package server

import (
	"github.com/erain9/matchingo/pkg/core"
)

// OrderBookExt provides extension methods for OrderBook
type OrderBookExt struct {
	*core.OrderBook
}

// GetBackend returns the backend of the order book
func GetOrderBookBackend(book *core.OrderBook) core.OrderBookBackend {
	// We can't directly access the unexported backend field
	// Instead, we'll use a trick: we'll ask for bids and asks from the book's methods
	// This is a bit inefficient but it works

	// Create a minimal partial implementation of OrderBookBackend
	return &orderBookBackendGetter{book: book}
}

// orderBookBackendGetter provides minimal implementation to get bids and asks
type orderBookBackendGetter struct {
	book *core.OrderBook
}

func (g *orderBookBackendGetter) GetBids() interface{} {
	// Since we can't directly access the backend, we have to use indirect ways
	// We know internally that getOppositeOrders with Sell gets Bids
	// This is brittle and depends on OrderBook implementation, but it's the best we can do
	// without modifying the core package
	return g.book.GetBids()
}

func (g *orderBookBackendGetter) GetAsks() interface{} {
	// Similar to GetBids, we'd need to handle this indirectly
	return g.book.GetAsks()
}

// Unused OrderBookBackend methods - we only need GetBids and GetAsks
func (g *orderBookBackendGetter) GetOrder(orderID string) *core.Order                   { return nil }
func (g *orderBookBackendGetter) StoreOrder(order *core.Order) error                    { return nil }
func (g *orderBookBackendGetter) UpdateOrder(order *core.Order) error                   { return nil }
func (g *orderBookBackendGetter) DeleteOrder(orderID string)                            {}
func (g *orderBookBackendGetter) AppendToSide(side core.Side, order *core.Order)        {}
func (g *orderBookBackendGetter) RemoveFromSide(side core.Side, order *core.Order) bool { return false }
func (g *orderBookBackendGetter) AppendToStopBook(order *core.Order)                    {}
func (g *orderBookBackendGetter) RemoveFromStopBook(order *core.Order) bool             { return false }
func (g *orderBookBackendGetter) CheckOCO(orderID string) string                        { return "" }
func (g *orderBookBackendGetter) GetStopBook() interface{}                              { return nil }
