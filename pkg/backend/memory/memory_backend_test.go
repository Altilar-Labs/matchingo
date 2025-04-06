package memory

import (
	"fmt"
	"testing"

	"github.com/nikolaydubina/fpdecimal"
	"github.com/erain9/matchingo/pkg/core"
)

func TestNewMemoryBackend(t *testing.T) {
	backend := NewMemoryBackend()

	// Verify that backend was initialized correctly
	if backend.orders == nil {
		t.Error("Expected non-nil orders map")
	}

	if backend.bids == nil {
		t.Error("Expected non-nil bids")
	}

	if backend.asks == nil {
		t.Error("Expected non-nil asks")
	}

	if backend.stopBook == nil {
		t.Error("Expected non-nil stopBook")
	}

	if backend.ocoMapping == nil {
		t.Error("Expected non-nil ocoMapping")
	}
}

func TestMemoryBackend_OrderOperations(t *testing.T) {
	backend := NewMemoryBackend()

	// Create a test order
	orderID := "test-123"
	price := fpdecimal.FromFloat(100.0)
	quantity := fpdecimal.FromFloat(10.0)

	order := core.NewLimitOrder(orderID, core.Buy, quantity, price, core.GTC, "")

	// Test StoreOrder
	err := backend.StoreOrder(order)
	if err != nil {
		t.Errorf("StoreOrder returned an error: %v", err)
	}

	// Test GetOrder
	retrievedOrder := backend.GetOrder(orderID)
	if retrievedOrder == nil {
		t.Error("GetOrder returned nil")
	} else if retrievedOrder.ID() != orderID {
		t.Errorf("Expected order ID %s, got %s", orderID, retrievedOrder.ID())
	}

	// Test UpdateOrder
	order.SetTaker()
	err = backend.UpdateOrder(order)
	if err != nil {
		t.Errorf("UpdateOrder returned an error: %v", err)
	}

	// Verify update
	updatedOrder := backend.GetOrder(orderID)
	if updatedOrder.Role() != core.TAKER {
		t.Errorf("Expected role %s, got %s", core.TAKER, updatedOrder.Role())
	}

	// Test DeleteOrder
	backend.DeleteOrder(orderID)
	deletedOrder := backend.GetOrder(orderID)
	if deletedOrder != nil {
		t.Error("Expected nil after deletion, but order still exists")
	}
}

func TestMemoryBackend_AppendToSide(t *testing.T) {
	backend := NewMemoryBackend()

	// Create a buy order
	buyOrderID := "buy-123"
	buyPrice := fpdecimal.FromFloat(100.0)
	quantity := fpdecimal.FromFloat(10.0)
	buyOrder := core.NewLimitOrder(buyOrderID, core.Buy, quantity, buyPrice, core.GTC, "")

	// Create a sell order
	sellOrderID := "sell-123"
	sellPrice := fpdecimal.FromFloat(102.0)
	sellOrder := core.NewLimitOrder(sellOrderID, core.Sell, quantity, sellPrice, core.GTC, "")

	// Store orders
	_ = backend.StoreOrder(buyOrder)
	_ = backend.StoreOrder(sellOrder)

	// Add to sides
	backend.AppendToSide(core.Buy, buyOrder)
	backend.AppendToSide(core.Sell, sellOrder)
}

func TestMemoryBackend_RemoveFromSide(t *testing.T) {
	backend := NewMemoryBackend()

	// Create a buy order
	orderID := "buy-123"
	price := fpdecimal.FromFloat(100.0)
	quantity := fpdecimal.FromFloat(10.0)
	order := core.NewLimitOrder(orderID, core.Buy, quantity, price, core.GTC, "")

	// Store order
	_ = backend.StoreOrder(order)

	// Add to side
	backend.AppendToSide(core.Buy, order)

	// Remove from side
	removed := backend.RemoveFromSide(core.Buy, order)
	if !removed {
		t.Error("Expected RemoveFromSide to return true")
	}

	// Try to remove again (should fail)
	removed = backend.RemoveFromSide(core.Buy, order)
	if removed {
		t.Error("Expected RemoveFromSide to return false when order not found")
	}
}

func TestMemoryBackend_OCOOperations(t *testing.T) {
	backend := NewMemoryBackend()

	// Create two orders with OCO relationship
	order1ID := "order-1"
	order2ID := "order-2"
	price := fpdecimal.FromFloat(100.0)
	quantity := fpdecimal.FromFloat(10.0)

	order1 := core.NewLimitOrder(order1ID, core.Buy, quantity, price, core.GTC, order2ID)
	order2 := core.NewLimitOrder(order2ID, core.Sell, quantity, price, core.GTC, order1ID)

	// Store orders
	_ = backend.StoreOrder(order1)
	_ = backend.StoreOrder(order2)

	// Check OCO mapping for order1 - this will remove the mapping
	ocoID := backend.CheckOCO(order1ID)
	if ocoID != order2ID {
		t.Errorf("Expected OCO ID %s for order1, got %s", order2ID, ocoID)
	}

	// The OCO mappings should be removed after first call, so a second call should return empty
	ocoID = backend.CheckOCO(order2ID)
	if ocoID != "" {
		t.Errorf("Expected empty OCO ID after first check, got %s", ocoID)
	}

	// Test with a new pair of orders to check the other direction
	order3ID := "order-3"
	order4ID := "order-4"
	order3 := core.NewLimitOrder(order3ID, core.Buy, quantity, price, core.GTC, order4ID)
	order4 := core.NewLimitOrder(order4ID, core.Sell, quantity, price, core.GTC, order3ID)

	_ = backend.StoreOrder(order3)
	_ = backend.StoreOrder(order4)

	// Check OCO mapping for order4 first this time
	ocoID = backend.CheckOCO(order4ID)
	if ocoID != order3ID {
		t.Errorf("Expected OCO ID %s for order4, got %s", order3ID, ocoID)
	}
}

func TestMemoryBackend_StopBookOperations(t *testing.T) {
	backend := NewMemoryBackend()

	// Create a stop order
	orderID := "stop-123"
	price := fpdecimal.FromFloat(100.0)
	stopPrice := fpdecimal.FromFloat(105.0)
	quantity := fpdecimal.FromFloat(10.0)

	order := core.NewStopLimitOrder(orderID, core.Buy, quantity, price, stopPrice, "")

	// Store order
	_ = backend.StoreOrder(order)

	// Append to stop book
	backend.AppendToStopBook(order)

	// Remove from stop book
	removed := backend.RemoveFromStopBook(order)
	if !removed {
		t.Error("Expected RemoveFromStopBook to return true")
	}

	// Try to remove again (should fail)
	removed = backend.RemoveFromStopBook(order)
	if removed {
		t.Error("Expected RemoveFromStopBook to return false when order not found")
	}
}

func TestMemoryBackend_GetSides(t *testing.T) {
	backend := NewMemoryBackend()

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

func TestOrderQueue(t *testing.T) {
	price := fpdecimal.FromFloat(100.0)
	queue := NewOrderQueue(price)

	if queue == nil {
		t.Fatal("Expected non-nil queue")
	}

	if queue.priceStr != price.String() {
		t.Errorf("Expected price string %s, got %s", price.String(), queue.priceStr)
	}

	if !queue.priceDecm.Equal(price) {
		t.Errorf("Expected price %v, got %v", price, queue.priceDecm)
	}

	if queue.orders == nil {
		t.Error("Expected non-nil orders map")
	}
}

func TestOrderSide_String(t *testing.T) {
	side := &OrderSide{
		orderID: make(map[string]*OrderQueue),
	}

	// An empty side should still return a valid string
	str := side.String()
	if str != "" {
		t.Errorf("Expected empty string for empty side, got %q", str)
	}

	// Add a queue to test non-empty side
	price := fpdecimal.FromFloat(100.0)
	queue := NewOrderQueue(price)
	side.orderID[price.String()] = queue
	side.head = queue
	side.tail = queue

	// A non-empty side should return a non-empty string
	str = side.String()
	if str == "" {
		t.Error("Expected non-empty string for non-empty side")
	}

	expectedStr := fmt.Sprintf("\n%s -> orders: %d", price.String(), 0)
	if str != expectedStr {
		t.Errorf("Expected %q, got %q", expectedStr, str)
	}
}

func TestStopBook_String(t *testing.T) {
	stopBook := &StopBook{
		buy: &OrderSide{
			orderID: make(map[string]*OrderQueue),
		},
		sell: &OrderSide{
			orderID: make(map[string]*OrderQueue),
		},
	}

	// An empty stop book should still return a valid string
	str := stopBook.String()
	if str == "" {
		t.Error("Expected non-empty string representation")
	}

	expected := "Buy Stop Orders:\nSell Stop Orders:"
	if str != expected {
		t.Errorf("Expected %q, got %q", expected, str)
	}
}
