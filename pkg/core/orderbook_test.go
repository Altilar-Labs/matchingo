package core

import (
	"sort"
	"testing"

	"github.com/nikolaydubina/fpdecimal"
)

// mockBackend implements the OrderBookBackend interface for testing with enhanced functionality
type mockBackend struct {
	orders    map[string]*Order
	sellSide  mockOrderSide
	buySide   mockOrderSide
	stopBooks []*Order
}

func newMockBackend() *mockBackend {
	return &mockBackend{
		orders:    make(map[string]*Order),
		sellSide:  mockOrderSide{orders: make(map[string]fpdecimalOrders)},
		buySide:   mockOrderSide{orders: make(map[string]fpdecimalOrders)},
		stopBooks: make([]*Order, 0),
	}
}

func (m *mockBackend) GetOrder(orderID string) *Order {
	return m.orders[orderID]
}

func (m *mockBackend) StoreOrder(order *Order) error {
	m.orders[order.ID()] = order
	return nil
}

func (m *mockBackend) UpdateOrder(order *Order) error {
	m.orders[order.ID()] = order
	return nil
}

func (m *mockBackend) DeleteOrder(orderID string) {
	delete(m.orders, orderID)
}

func (m *mockBackend) AppendToSide(side Side, order *Order) {
	if side == Buy {
		m.buySide.appendOrder(order)
	} else {
		m.sellSide.appendOrder(order)
	}
}

func (m *mockBackend) RemoveFromSide(side Side, order *Order) bool {
	if side == Buy {
		return m.buySide.removeOrder(order)
	}
	return m.sellSide.removeOrder(order)
}

func (m *mockBackend) AppendToStopBook(order *Order) {
	m.stopBooks = append(m.stopBooks, order)
}

func (m *mockBackend) RemoveFromStopBook(order *Order) bool {
	for i, o := range m.stopBooks {
		if o.ID() == order.ID() {
			m.stopBooks = append(m.stopBooks[:i], m.stopBooks[i+1:]...)
			return true
		}
	}
	return false
}

func (m *mockBackend) CheckOCO(orderID string) string {
	return ""
}

func (m *mockBackend) GetBids() interface{} {
	return &m.buySide
}

func (m *mockBackend) GetAsks() interface{} {
	return &m.sellSide
}

func (m *mockBackend) GetStopBook() interface{} {
	return m.stopBooks
}

// fpdecimalOrders is a map from order ID to Order
type fpdecimalOrders map[string]*Order

// mockOrderSide is a mock implementation of the OrderSide interface for testing
type mockOrderSide struct {
	orders map[string]fpdecimalOrders
}

func (m *mockOrderSide) appendOrder(order *Order) {
	price := order.Price().String()
	if _, exists := m.orders[price]; !exists {
		m.orders[price] = make(fpdecimalOrders)
	}
	m.orders[price][order.ID()] = order
}

func (m *mockOrderSide) removeOrder(order *Order) bool {
	price := order.Price().String()
	if _, exists := m.orders[price]; !exists {
		return false
	}
	if _, exists := m.orders[price][order.ID()]; !exists {
		return false
	}
	delete(m.orders[price], order.ID())
	if len(m.orders[price]) == 0 {
		delete(m.orders, price)
	}
	return true
}

// Prices returns all prices in the order side
func (m *mockOrderSide) Prices() []fpdecimal.Decimal {
	prices := make([]fpdecimal.Decimal, 0, len(m.orders))
	for priceStr := range m.orders {
		price, _ := fpdecimal.FromString(priceStr)
		prices = append(prices, price)
	}

	// Simulate the behavior of memory backend
	// For buy orders (bids), return prices in descending order (highest first)
	// For sell orders (asks), return prices in ascending order (lowest first)
	sort.Slice(prices, func(i, j int) bool {
		// Detect if this is a buy or sell side by checking the first order
		if len(prices) > 0 {
			for _, orders := range m.orders {
				for _, order := range orders {
					if order.Side() == Buy {
						// For buy orders, sort descending (highest first)
						return prices[i].GreaterThan(prices[j])
					} else {
						// For sell orders, sort ascending (lowest first)
						return prices[i].LessThan(prices[j])
					}
				}
				break
			}
		}
		// Default to ascending order if we can't determine
		return prices[i].LessThan(prices[j])
	})

	return prices
}

func (m *mockOrderSide) Orders(price fpdecimal.Decimal) []*Order {
	priceStr := price.String()
	if _, exists := m.orders[priceStr]; !exists {
		return []*Order{}
	}

	orders := make([]*Order, 0, len(m.orders[priceStr]))
	for _, order := range m.orders[priceStr] {
		orders = append(orders, order)
	}

	return orders
}

func TestOrderBookCreation(t *testing.T) {
	backend := newMockBackend()
	book := NewOrderBook(backend)

	if book == nil {
		t.Error("Expected OrderBook to be created, got nil")
	}
}

func TestMarketOrderExecution(t *testing.T) {
	backend := newMockBackend()
	book := NewOrderBook(backend)

	// Create a sell limit order first
	sellOrderID := "sell-1"
	sellPrice := fpdecimal.FromFloat(10.0)
	sellQty := fpdecimal.FromFloat(5.0)
	sellOrder := NewLimitOrder(sellOrderID, Sell, sellQty, sellPrice, GTC, "")

	// Process the sell order
	_, err := book.Process(sellOrder)
	if err != nil {
		t.Errorf("Expected no error when processing sell order, got %v", err)
	}

	// Create a buy market order
	buyOrderID := "buy-1"
	buyQty := fpdecimal.FromFloat(2.0)
	buyOrder := NewMarketOrder(buyOrderID, Buy, buyQty)

	// Process the buy order
	done, err := book.Process(buyOrder)
	if err != nil {
		t.Errorf("Expected no error when processing buy order, got %v", err)
	}

	// Verify the results
	if done.Processed.Equal(fpdecimal.Zero) {
		t.Error("Expected the order to be partially processed, got zero processed quantity")
	}

	// Check if trades were recorded
	if len(done.Trades) == 0 {
		t.Error("Expected trades to be recorded, got none")
	}

	// Verify the remaining quantity of the sell order
	remainingSellQty := sellQty.Sub(buyQty)
	updatedSellOrder := backend.GetOrder(sellOrderID)
	if updatedSellOrder == nil {
		t.Error("Expected sell order to still exist, got nil")
	} else if !updatedSellOrder.Quantity().Equal(remainingSellQty) {
		t.Errorf("Expected remaining sell quantity to be %s, got %s", remainingSellQty.String(), updatedSellOrder.Quantity().String())
	}
}

func TestLimitOrderMatching(t *testing.T) {
	backend := newMockBackend()
	book := NewOrderBook(backend)

	// Create a sell limit order
	sellOrderID := "sell-1"
	sellPrice := fpdecimal.FromFloat(10.0)
	sellQty := fpdecimal.FromFloat(5.0)
	sellOrder := NewLimitOrder(sellOrderID, Sell, sellQty, sellPrice, GTC, "")

	// Process the sell order
	_, err := book.Process(sellOrder)
	if err != nil {
		t.Errorf("Expected no error when processing sell order, got %v", err)
	}

	// Create a buy limit order that matches
	buyOrderID := "buy-1"
	buyPrice := fpdecimal.FromFloat(10.0) // Exact match
	buyQty := fpdecimal.FromFloat(3.0)
	buyOrder := NewLimitOrder(buyOrderID, Buy, buyQty, buyPrice, GTC, "")

	// Process the buy order
	done, err := book.Process(buyOrder)
	if err != nil {
		t.Errorf("Expected no error when processing buy order, got %v", err)
	}

	// Verify the results
	if done.Processed.Equal(fpdecimal.Zero) {
		t.Error("Expected the order to be processed, got zero processed quantity")
	}

	// Check if trades were recorded
	if len(done.Trades) == 0 {
		t.Error("Expected trades to be recorded, got none")
	}

	// Verify the remaining quantity of the sell order
	remainingSellQty := sellQty.Sub(buyQty)
	updatedSellOrder := backend.GetOrder(sellOrderID)
	if updatedSellOrder == nil {
		t.Error("Expected sell order to still exist, got nil")
	} else if !updatedSellOrder.Quantity().Equal(remainingSellQty) {
		t.Errorf("Expected remaining sell quantity to be %s, got %s", remainingSellQty.String(), updatedSellOrder.Quantity().String())
	}
}

func TestCompleteOrderExecution(t *testing.T) {
	backend := newMockBackend()
	book := NewOrderBook(backend)

	// Create multiple sell limit orders
	sell1 := NewLimitOrder("sell-1", Sell, fpdecimal.FromFloat(3.0), fpdecimal.FromFloat(10.0), GTC, "")
	sell2 := NewLimitOrder("sell-2", Sell, fpdecimal.FromFloat(2.0), fpdecimal.FromFloat(11.0), GTC, "")

	// Process sell orders
	book.Process(sell1)
	book.Process(sell2)

	// Create a buy limit order that matches both sells completely
	buy := NewLimitOrder("buy-1", Buy, fpdecimal.FromFloat(5.0), fpdecimal.FromFloat(11.0), GTC, "")

	// Process the buy order
	done, err := book.Process(buy)
	if err != nil {
		t.Errorf("Expected no error when processing buy order, got %v", err)
	}

	// Verify that the buy order was fully executed (processed == original qty)
	if !done.Processed.Equal(buy.Quantity()) {
		t.Errorf("Expected buy order to be fully processed. Got processed: %s, original: %s",
			done.Processed.String(), buy.Quantity().String())
	}

	// Verify that the sell orders were removed from the book
	if backend.GetOrder("sell-1") != nil {
		t.Error("Expected sell-1 to be removed from the book")
	}

	if backend.GetOrder("sell-2") != nil {
		t.Error("Expected sell-2 to be removed from the book")
	}

	// Verify that trades were recorded
	if len(done.Trades) < 3 { // At least the buy order itself and 2 matches
		t.Errorf("Expected at least 3 trade entries, got %d", len(done.Trades))
	}
}

func TestPartialFillAndBookInsertion(t *testing.T) {
	backend := newMockBackend()
	book := NewOrderBook(backend)

	// Create a small sell limit order
	sell := NewLimitOrder("sell-1", Sell, fpdecimal.FromFloat(2.0), fpdecimal.FromFloat(10.0), GTC, "")

	// Process sell order
	book.Process(sell)

	// Create a larger buy limit order
	buy := NewLimitOrder("buy-1", Buy, fpdecimal.FromFloat(5.0), fpdecimal.FromFloat(10.0), GTC, "")

	// Process the buy order - should partially fill and be inserted into the book
	done, err := book.Process(buy)
	if err != nil {
		t.Errorf("Expected no error when processing buy order, got %v", err)
	}

	// Verify that the buy order was partially filled
	expectedProcessed := fpdecimal.FromFloat(2.0) // Matches the sell quantity
	if !done.Processed.Equal(expectedProcessed) {
		t.Errorf("Expected processed quantity to be %s, got %s",
			expectedProcessed.String(), done.Processed.String())
	}

	// Verify remaining quantity
	expectedRemaining := fpdecimal.FromFloat(3.0) // 5 original - 2 processed
	if !done.Left.Equal(expectedRemaining) {
		t.Errorf("Expected remaining quantity to be %s, got %s",
			expectedRemaining.String(), done.Left.String())
	}

	// Verify the buy order was added to the book
	if done.Stored != true {
		t.Error("Expected the buy order to be stored in the book")
	}

	// Verify the order in the backend has the correct quantity
	storedBuy := backend.GetOrder("buy-1")
	if storedBuy == nil {
		t.Error("Expected buy order to be in the backend, got nil")
	} else if !storedBuy.Quantity().Equal(expectedRemaining) {
		t.Errorf("Expected stored buy quantity to be %s, got %s",
			expectedRemaining.String(), storedBuy.Quantity().String())
	}

	// Verify the sell order was removed
	if backend.GetOrder("sell-1") != nil {
		t.Error("Expected sell order to be removed from the book")
	}
}

func TestPriceTimeOrderPriority(t *testing.T) {
	backend := newMockBackend()
	book := NewOrderBook(backend)

	// Create multiple sell limit orders at different prices
	sell1 := NewLimitOrder("sell-1", Sell, fpdecimal.FromFloat(2.0), fpdecimal.FromFloat(10.0), GTC, "")
	sell2 := NewLimitOrder("sell-2", Sell, fpdecimal.FromFloat(2.0), fpdecimal.FromFloat(11.0), GTC, "")
	sell3 := NewLimitOrder("sell-3", Sell, fpdecimal.FromFloat(2.0), fpdecimal.FromFloat(9.5), GTC, "")

	// Process sell orders
	book.Process(sell1)
	book.Process(sell2)
	book.Process(sell3)

	// Create a buy market order to match against the best prices
	buy := NewMarketOrder("buy-1", Buy, fpdecimal.FromFloat(3.0))

	// Process the buy order
	done, err := book.Process(buy)
	if err != nil {
		t.Errorf("Expected no error when processing buy order, got %v", err)
	}

	// The order should match first with sell3 (9.5) then with sell1 (10.0)
	if len(done.Trades) < 2 {
		t.Errorf("Expected at least 2 trades, got %d", len(done.Trades))
	}

	// Check that the trades are ordered by price (best price first)
	// Note: The first entry is the buy order itself
	var foundSell3, foundSell1 bool
	for _, trade := range done.Trades {
		if trade.OrderID == "sell-3" {
			foundSell3 = true
		}
		if trade.OrderID == "sell-1" {
			foundSell1 = true
			// Make sure this comes after sell3 which had better price
			if !foundSell3 {
				t.Error("Expected sell-3 (better price) to match before sell-1")
			}
		}
	}

	if !foundSell3 {
		t.Error("Expected sell-3 to match, but it didn't")
	}

	if !foundSell1 {
		t.Error("Expected sell-1 to match, but it didn't")
	}

	// Verify sell2 (worst price) is still in the book
	if backend.GetOrder("sell-2") == nil {
		t.Error("Expected sell-2 to remain in the book")
	}
}

func TestMarketOrderFullExecution(t *testing.T) {
	backend := newMockBackend()
	book := NewOrderBook(backend)

	// Create multiple sell limit orders with enough quantity
	sell1 := NewLimitOrder("sell-1", Sell, fpdecimal.FromFloat(5.0), fpdecimal.FromFloat(10.0), GTC, "")
	sell2 := NewLimitOrder("sell-2", Sell, fpdecimal.FromFloat(5.0), fpdecimal.FromFloat(11.0), GTC, "")

	// Process sell orders
	book.Process(sell1)
	book.Process(sell2)

	// Create a buy market order
	buy := NewMarketOrder("buy-1", Buy, fpdecimal.FromFloat(7.0))

	// Process the buy order
	done, err := book.Process(buy)
	if err != nil {
		t.Errorf("Expected no error when processing buy order, got %v", err)
	}

	// Verify the buy order was fully executed
	if !done.Left.Equal(fpdecimal.Zero) {
		t.Errorf("Expected market order to be fully executed, got remaining: %s", done.Left.String())
	}

	// Verify that sell1 was fully executed
	if backend.GetOrder("sell-1") != nil {
		t.Error("Expected sell-1 to be removed from the book")
	}

	// Verify that sell2 was partially executed
	sell2Updated := backend.GetOrder("sell-2")
	if sell2Updated == nil {
		t.Error("Expected sell-2 to remain in the book")
	} else {
		expectedQty := fpdecimal.FromFloat(3.0) // original 5 - remaining 2
		if !sell2Updated.Quantity().Equal(expectedQty) {
			t.Errorf("Expected sell-2 to have %s quantity left, got %s",
				expectedQty.String(), sell2Updated.Quantity().String())
		}
	}
}

func TestCalculateMarketPrice(t *testing.T) {
	backend := newMockBackend()
	book := NewOrderBook(backend)

	// Add some sell orders
	book.Process(NewLimitOrder("sell-1", Sell, fpdecimal.FromFloat(3.0), fpdecimal.FromFloat(10.0), GTC, ""))
	book.Process(NewLimitOrder("sell-2", Sell, fpdecimal.FromFloat(2.0), fpdecimal.FromFloat(11.0), GTC, ""))

	// Calculate market price for buy order
	price, err := book.CalculateMarketPrice(Buy, fpdecimal.FromFloat(4.0))

	if err != nil {
		t.Errorf("Expected market price calculation to succeed, got error: %v", err)
	}

	// Expected price: (3 * 10) + (1 * 11) = 41
	expectedPrice := fpdecimal.FromFloat(41.0)
	if !price.Equal(expectedPrice) {
		t.Errorf("Expected market price to be %s, got %s", expectedPrice.String(), price.String())
	}

	// Now test insufficient quantity
	_, err = book.CalculateMarketPrice(Buy, fpdecimal.FromFloat(10.0))
	if err != ErrInsufficientQuantity {
		t.Errorf("Expected ErrInsufficientQuantity, got %v", err)
	}
}

func BenchmarkOrderMatching(b *testing.B) {
	backend := newMockBackend()
	book := NewOrderBook(backend)

	// Add a bunch of sell orders at different price levels
	for i := 0; i < 100; i++ {
		price := 10.0 + float64(i)*0.1
		book.Process(NewLimitOrder(
			"sell-"+string(rune(i)),
			Sell,
			fpdecimal.FromFloat(1.0),
			fpdecimal.FromFloat(price),
			GTC,
			""))
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Create and process a buy market order
		buy := NewMarketOrder("buy-bench", Buy, fpdecimal.FromFloat(5.0))
		book.Process(buy)

		// Refill the book for the next iteration
		for j := 0; j < 5; j++ {
			price := 10.0 + float64(j)*0.1
			book.Process(NewLimitOrder(
				"sell-refill-"+string(rune(j)),
				Sell,
				fpdecimal.FromFloat(1.0),
				fpdecimal.FromFloat(price),
				GTC,
				""))
		}
	}
}
