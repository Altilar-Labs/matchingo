package memory

import (
	"fmt"
	"testing"

	"github.com/erain9/matchingo/pkg/core"
	"github.com/nikolaydubina/fpdecimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMemoryBackend(t *testing.T) {
	backend := NewMemoryBackend()
	assert.NotNil(t, backend)
	assert.NotNil(t, backend.orders)
	assert.NotNil(t, backend.bids)
	assert.NotNil(t, backend.asks)
	assert.NotNil(t, backend.stopBook)
	assert.NotNil(t, backend.ocoMapping)
}

func TestMemoryBackend_OrderOperations(t *testing.T) {
	backend := NewMemoryBackend()

	// Create a test order
	orderID := "test-123"
	price := fpdecimal.FromFloat(100.0)
	quantity := fpdecimal.FromFloat(10.0)

	order, err := core.NewLimitOrder(orderID, core.Buy, quantity, price, core.GTC, "", "test_user")
	require.NoError(t, err)

	// Test StoreOrder
	err = backend.StoreOrder(order)
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
	buyOrder, err := core.NewLimitOrder(buyOrderID, core.Buy, quantity, buyPrice, core.GTC, "", "test_user")
	require.NoError(t, err)

	// Create a sell order
	sellOrderID := "sell-123"
	sellPrice := fpdecimal.FromFloat(102.0)
	sellOrder, err := core.NewLimitOrder(sellOrderID, core.Sell, quantity, sellPrice, core.GTC, "", "test_user")
	require.NoError(t, err)

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
	order, err := core.NewLimitOrder(orderID, core.Buy, quantity, price, core.GTC, "", "test_user")
	require.NoError(t, err)

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

	order1, err := core.NewLimitOrder(order1ID, core.Buy, quantity, price, core.GTC, order2ID, "test_user")
	require.NoError(t, err)
	order2, err := core.NewLimitOrder(order2ID, core.Sell, quantity, price, core.GTC, order1ID, "test_user")
	require.NoError(t, err)

	// Store orders
	_ = backend.StoreOrder(order1)
	_ = backend.StoreOrder(order2)

	// Check OCO mapping for order1 - this will remove the mapping
	ocoID := backend.CheckOCO(order1ID)
	if ocoID != order2ID {
		t.Errorf("Expected OCO ID %s for order1, got %s", order2ID, ocoID)
	}

	// The OCO mappings should still exist after checking
	ocoID = backend.CheckOCO(order2ID)
	if ocoID != order1ID {
		t.Errorf("Expected OCO ID %s for order2, got %s", order1ID, ocoID)
	}

	// Delete order1 and verify its OCO mapping is cleared
	backend.DeleteOrder(order1ID)
	ocoID = backend.CheckOCO(order2ID)
	if ocoID != "" {
		t.Errorf("Expected empty OCO ID after deleting order1, got %s", ocoID)
	}

	// Test with a new pair of orders to check the other direction
	order3ID := "order-3"
	order4ID := "order-4"
	order3, err := core.NewLimitOrder(order3ID, core.Buy, quantity, price, core.GTC, order4ID, "test_user")
	require.NoError(t, err)
	order4, err := core.NewLimitOrder(order4ID, core.Sell, quantity, price, core.GTC, order3ID, "test_user")
	require.NoError(t, err)

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

	order, err := core.NewStopLimitOrder(orderID, core.Buy, quantity, price, stopPrice, "", "test_user")
	require.NoError(t, err)

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
	q := NewOrderQueue(price)
	assert.NotNil(t, q)
	assert.True(t, q.priceDecm.Equal(price))
	assert.Equal(t, price.String(), q.priceStr)
	assert.NotNil(t, q.orders)
}

func TestOrderSide(t *testing.T) {
	os := &OrderSide{
		orderID: make(map[string]*OrderQueue),
	}
	assert.NotNil(t, os)
	assert.NotNil(t, os.orderID)
}

func TestAppendAndRemoveOrder(t *testing.T) {
	backend := NewMemoryBackend()
	price := fpdecimal.FromFloat(100.0)
	qty := fpdecimal.FromFloat(1.0)
	order, err := core.NewLimitOrder("order-1", core.Buy, qty, price, core.GTC, "", "test_user")
	require.NoError(t, err)

	backend.AppendToSide(core.Buy, order)

	// Verify order is retrievable via GetBids interface
	bidsInterface := backend.GetBids()
	bids, ok := bidsInterface.(*OrderSide)
	require.True(t, ok, "GetBids should return *OrderSide")
	ordersAtPrice := bids.Orders(price)
	assert.Len(t, ordersAtPrice, 1, "Expected 1 order at the price level")
	assert.Equal(t, order.ID(), ordersAtPrice[0].ID(), "Order ID mismatch")

	// Verify Prices returns the correct price
	prices := bids.Prices()
	assert.Len(t, prices, 1, "Expected 1 price level")
	assert.True(t, prices[0].Equal(price), "Expected price %s, got %s", price, prices[0])

	// Remove the order
	removed := backend.RemoveFromSide(core.Buy, order)
	assert.True(t, removed, "Expected RemoveFromSide to return true")

	// Verify order is removed
	ordersAtPriceAfterRemove := bids.Orders(price)
	assert.Len(t, ordersAtPriceAfterRemove, 0, "Expected 0 orders after removal")
	pricesAfterRemove := bids.Prices()
	assert.Len(t, pricesAfterRemove, 0, "Expected 0 price levels after removal")
}

func TestOrderSide_RemoveNonExistent(t *testing.T) {
	backend := NewMemoryBackend()
	price := fpdecimal.FromFloat(100.0)
	qty := fpdecimal.FromFloat(1.0)
	order1, err := core.NewLimitOrder("order-1", core.Buy, qty, price, core.GTC, "", "test_user")
	require.NoError(t, err)
	order2, err := core.NewLimitOrder("order-2", core.Buy, qty, price, core.GTC, "", "test_user") // Different order
	require.NoError(t, err)

	backend.AppendToSide(core.Buy, order1)

	// Try removing order2 (which wasn't added)
	removed := backend.RemoveFromSide(core.Buy, order2)
	assert.False(t, removed, "Expected RemoveFromSide to return false for non-existent order")

	// Verify order1 is still there
	bidsInterface := backend.GetBids()
	bids, ok := bidsInterface.(*OrderSide)
	require.True(t, ok)
	ordersAtPrice := bids.Orders(price)
	assert.Len(t, ordersAtPrice, 1, "Expected order-1 to still be present")
	assert.Equal(t, "order-1", ordersAtPrice[0].ID())
}

func TestPriceSorting(t *testing.T) {
	backend := NewMemoryBackend()

	// Add orders at different prices
	order100, err := core.NewLimitOrder("order-100", core.Sell, fpdecimal.FromInt(1), fpdecimal.FromInt(100), core.GTC, "", "test_user")
	require.NoError(t, err)
	order105, err := core.NewLimitOrder("order-105", core.Sell, fpdecimal.FromInt(1), fpdecimal.FromInt(105), core.GTC, "", "test_user")
	require.NoError(t, err)
	order95, err := core.NewLimitOrder("order-95", core.Sell, fpdecimal.FromInt(1), fpdecimal.FromInt(95), core.GTC, "", "test_user")
	require.NoError(t, err)

	backend.AppendToSide(core.Sell, order100)
	backend.AppendToSide(core.Sell, order105)
	backend.AppendToSide(core.Sell, order95)

	// Get asks and verify price sorting (ascending for asks)
	asksInterface := backend.GetAsks()
	asks, ok := asksInterface.(*OrderSide)
	require.True(t, ok)
	prices := asks.Prices()

	require.Len(t, prices, 3)
	assert.True(t, prices[0].Equal(fpdecimal.FromInt(95)), "Expected first price 95, got %s", prices[0])
	assert.True(t, prices[1].Equal(fpdecimal.FromInt(100)), "Expected second price 100, got %s", prices[1])
	assert.True(t, prices[2].Equal(fpdecimal.FromInt(105)), "Expected third price 105, got %s", prices[2])

	// Add buy orders
	buyOrder100, err := core.NewLimitOrder("buy-100", core.Buy, fpdecimal.FromInt(1), fpdecimal.FromInt(100), core.GTC, "", "test_user")
	require.NoError(t, err)
	buyOrder95, err := core.NewLimitOrder("buy-95", core.Buy, fpdecimal.FromInt(1), fpdecimal.FromInt(95), core.GTC, "", "test_user")
	require.NoError(t, err)

	backend.AppendToSide(core.Buy, buyOrder100)
	backend.AppendToSide(core.Buy, buyOrder95)

	// Get bids and verify price sorting (descending for bids)
	bidsInterface := backend.GetBids()
	bids, ok := bidsInterface.(*OrderSide)
	require.True(t, ok)
	bidPrices := bids.Prices()

	require.Len(t, bidPrices, 2)
	assert.True(t, bidPrices[0].Equal(fpdecimal.FromInt(100)), "Expected first bid price 100, got %s", bidPrices[0])
	assert.True(t, bidPrices[1].Equal(fpdecimal.FromInt(95)), "Expected second bid price 95, got %s", bidPrices[1])
}

func TestStopBook(t *testing.T) {
	backend := NewMemoryBackend()
	stopPrice := fpdecimal.FromFloat(105.0)
	limitPrice := fpdecimal.FromFloat(100.0)
	qty := fpdecimal.FromFloat(1.0)
	stopOrder, err := core.NewStopLimitOrder("stop-1", core.Buy, qty, limitPrice, stopPrice, "", "test_user")
	require.NoError(t, err)

	backend.AppendToStopBook(stopOrder)

	// Verify order is in stop book
	stopBookInterface := backend.GetStopBook()
	stopBook, ok := stopBookInterface.(*StopBook)
	require.True(t, ok)
	stopOrders := stopBook.buy.Orders(stopPrice) // Check the 'buy' side specifically
	assert.Len(t, stopOrders, 1)
	assert.Equal(t, "stop-1", stopOrders[0].ID())

	// Remove the order
	removed := backend.RemoveFromStopBook(stopOrder)
	assert.True(t, removed)

	// Verify order is removed
	stopOrdersAfterRemove := stopBook.buy.Orders(stopPrice) // Check the 'buy' side again
	assert.Len(t, stopOrdersAfterRemove, 0)
}

func TestStoreGetUpdateDeleteOrder(t *testing.T) {
	backend := NewMemoryBackend()
	price := fpdecimal.FromFloat(100.0)
	qty := fpdecimal.FromFloat(1.0)
	order, err := core.NewLimitOrder("order-crud", core.Buy, qty, price, core.GTC, "", "test_user")
	require.NoError(t, err)

	// Store
	err = backend.StoreOrder(order)
	assert.NoError(t, err)

	// Get
	retrievedOrder := backend.GetOrder("order-crud")
	require.NotNil(t, retrievedOrder)
	assert.Equal(t, order.ID(), retrievedOrder.ID())
	assert.True(t, order.Quantity().Equal(retrievedOrder.Quantity()))

	// Update
	newQty := fpdecimal.FromFloat(0.5)
	order.SetQuantity(newQty)
	err = backend.UpdateOrder(order)
	assert.NoError(t, err)

	// Get again to verify update
	updatedOrder := backend.GetOrder("order-crud")
	require.NotNil(t, updatedOrder)
	assert.True(t, newQty.Equal(updatedOrder.Quantity()), "Expected updated quantity %s, got %s", newQty, updatedOrder.Quantity())

	// Delete
	backend.DeleteOrder("order-crud")

	// Get again to verify deletion
	deletedOrder := backend.GetOrder("order-crud")
	assert.Nil(t, deletedOrder, "Expected order to be nil after deletion")
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

func TestMemoryBackend_StopOrderHandling(t *testing.T) {
	backend := NewMemoryBackend()

	// Create a stop order
	stopPrice, _ := fpdecimal.FromString("100.0")
	limitPrice, _ := fpdecimal.FromString("101.0")
	qty, _ := fpdecimal.FromString("1.0")
	stopOrder, err := core.NewStopLimitOrder("stop1", core.Buy, qty, limitPrice, stopPrice, "", "test_user")
	assert.NoError(t, err)

	// Test storing stop order
	err = backend.StoreOrder(stopOrder)
	assert.NoError(t, err)
	assert.Equal(t, stopOrder, backend.GetOrder("stop1"))

	// Test adding to stop book
	backend.AppendToStopBook(stopOrder)
	stopBook := backend.GetStopBook()
	stopBookInterface := stopBook.(interface {
		Orders(price fpdecimal.Decimal) []*core.Order
		Prices() []fpdecimal.Decimal
	})
	assert.Contains(t, stopBookInterface.Prices(), stopPrice)
	assert.Contains(t, stopBookInterface.Orders(stopPrice), stopOrder)

	// Test removing from stop book
	backend.RemoveFromStopBook(stopOrder)
	assert.Empty(t, stopBookInterface.Orders(stopPrice))

	// Test deleting stop order
	backend.DeleteOrder("stop1")
	assert.Nil(t, backend.GetOrder("stop1"))
}

func TestMemoryBackend_OrderBookOperations(t *testing.T) {
	backend := NewMemoryBackend()

	// Create test orders
	qty1, _ := fpdecimal.FromString("1.0")
	price1, _ := fpdecimal.FromString("100.0")
	buyOrder1, err := core.NewLimitOrder("buy1", core.Buy, qty1, price1, core.GTC, "", "test_user")
	assert.NoError(t, err)

	qty2, _ := fpdecimal.FromString("2.0")
	price2, _ := fpdecimal.FromString("99.0")
	buyOrder2, err := core.NewLimitOrder("buy2", core.Buy, qty2, price2, core.GTC, "", "test_user")
	assert.NoError(t, err)

	qty3, _ := fpdecimal.FromString("1.5")
	price3, _ := fpdecimal.FromString("101.0")
	sellOrder1, err := core.NewLimitOrder("sell1", core.Sell, qty3, price3, core.GTC, "", "test_user")
	assert.NoError(t, err)

	// Test storing orders
	assert.NoError(t, backend.StoreOrder(buyOrder1))
	assert.NoError(t, backend.StoreOrder(buyOrder2))
	assert.NoError(t, backend.StoreOrder(sellOrder1))

	// Test adding to order book sides
	backend.AppendToSide(core.Buy, buyOrder1)
	backend.AppendToSide(core.Buy, buyOrder2)
	backend.AppendToSide(core.Sell, sellOrder1)

	// Verify bids
	bids := backend.GetBids()
	bidsInterface := bids.(interface {
		Orders(price fpdecimal.Decimal) []*core.Order
		Prices() []fpdecimal.Decimal
	})
	assert.Contains(t, bidsInterface.Orders(price1), buyOrder1)
	assert.Contains(t, bidsInterface.Orders(price2), buyOrder2)

	// Verify asks
	asks := backend.GetAsks()
	asksInterface := asks.(interface {
		Orders(price fpdecimal.Decimal) []*core.Order
		Prices() []fpdecimal.Decimal
	})
	assert.Contains(t, asksInterface.Orders(price3), sellOrder1)

	// Test removing orders
	removed := backend.RemoveFromSide(core.Buy, buyOrder1)
	assert.True(t, removed)
	assert.NotContains(t, bidsInterface.Orders(price1), buyOrder1)

	// Test removing non-existent order
	removed = backend.RemoveFromSide(core.Buy, buyOrder1)
	assert.False(t, removed)
}

func TestMemoryBackend_OrderUpdates(t *testing.T) {
	backend := NewMemoryBackend()

	// Create and store an order
	qty, _ := fpdecimal.FromString("1.0")
	price, _ := fpdecimal.FromString("100.0")
	order, err := core.NewLimitOrder("test1", core.Buy, qty, price, core.GTC, "", "test_user")
	assert.NoError(t, err)
	assert.NoError(t, backend.StoreOrder(order))

	// Update order quantity
	newQty, _ := fpdecimal.FromString("2.0")
	order.SetQuantity(newQty)
	assert.NoError(t, backend.UpdateOrder(order))

	// Verify update
	updated := backend.GetOrder("test1")
	assert.Equal(t, newQty, updated.Quantity())
}

func TestMemoryBackend_OCOOrders(t *testing.T) {
	backend := NewMemoryBackend()

	// Create OCO orders
	qty1, _ := fpdecimal.FromString("1.0")
	price1, _ := fpdecimal.FromString("100.0")
	order1, err := core.NewLimitOrder("oco1", core.Buy, qty1, price1, core.GTC, "oco2", "test_user")
	assert.NoError(t, err)

	qty2, _ := fpdecimal.FromString("1.0")
	price2, _ := fpdecimal.FromString("110.0")
	order2, err := core.NewLimitOrder("oco2", core.Sell, qty2, price2, core.GTC, "oco1", "test_user")
	assert.NoError(t, err)

	// Store orders
	assert.NoError(t, backend.StoreOrder(order1))
	assert.NoError(t, backend.StoreOrder(order2))

	// Check OCO relationship
	assert.Equal(t, "oco2", backend.CheckOCO("oco1"))
	assert.Equal(t, "oco1", backend.CheckOCO("oco2"))

	// Delete one order and verify OCO is cleared
	backend.DeleteOrder("oco1")
	assert.Empty(t, backend.CheckOCO("oco2"))
}
