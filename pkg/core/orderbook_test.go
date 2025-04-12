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

func TestFOKLimitOrder(t *testing.T) {
	// ... existing TestFOKLimitOrder code ...
}

// TestPriceTimePriority verifies that orders are matched based on price first, then time.
func TestPriceTimePriority(t *testing.T) {
	backend := newMockBackend()
	book := NewOrderBook(backend)

	// Setup sell side with multiple price levels and orders
	// Sell orders: sell1 (10 @ 100), sell2 (5 @ 100), sell3 (10 @ 105)
	sellOrders := []*Order{
		NewLimitOrder("sell1", Sell, fpdecimal.FromInt(10), fpdecimal.FromInt(100), GTC, ""),
		NewLimitOrder("sell2", Sell, fpdecimal.FromInt(5), fpdecimal.FromInt(100), GTC, ""), // Same price as sell1, but added later
		NewLimitOrder("sell3", Sell, fpdecimal.FromInt(10), fpdecimal.FromInt(105), GTC, ""),
	}
	for _, order := range sellOrders {
		_, err := book.Process(order)
		if err != nil {
			t.Fatalf("Failed to process setup order %s: %v", order.ID(), err)
		}
	}

	// Create a buy order that should match sell1 and part of sell2
	buyQty := fpdecimal.FromInt(12)
	buyOrder := NewLimitOrder("buy1", Buy, buyQty, fpdecimal.FromInt(100), GTC, "") // Price matches sell1 & sell2

	// Process the buy order
	done, err := book.Process(buyOrder)
	if err != nil {
		t.Fatalf("Expected no error when processing buy order, got %v", err)
	}

	// --- Verification ---

	// 1. Total processed quantity should match buyQty (as enough liquidity exists at 100)
	expectedProcessed := buyQty
	if !done.Processed.Equal(expectedProcessed) {
		t.Errorf("Expected processed quantity to be %s, got %s", expectedProcessed, done.Processed)
	}
	if !done.Left.Equal(fpdecimal.Zero) {
		t.Errorf("Expected left quantity to be zero, got %s", done.Left)
	}

	// 2. Check number of trades (should include the taker order + 2 maker matches)
	expectedTradeCount := 3
	if len(done.Trades) != expectedTradeCount {
		t.Fatalf("Expected %d trades (taker + makers), got %d", expectedTradeCount, len(done.Trades))
	}

	// 3. Verify sell1 match details (should be the first match due to time priority)
	trade1 := done.Trades[1] // First maker match
	expectedSell1Fill := fpdecimal.FromInt(10)
	if trade1.OrderID != "sell1" || !trade1.Quantity.Equal(expectedSell1Fill) || !trade1.Price.Equal(fpdecimal.FromInt(100)) {
		t.Errorf("Trade 1 (Maker Match) details incorrect. Expected MakerID: sell1, Qty: %s, Price: 100. Got: %+v", expectedSell1Fill, trade1)
	}
	// Verify sell1 is fully filled in the backend
	sell1 := backend.GetOrder("sell1")
	if sell1 != nil && !sell1.Quantity().Equal(fpdecimal.Zero) {
		t.Errorf("Expected sell1 to be fully filled in backend (qty 0), got %s", sell1.Quantity())
	}

	// 4. Verify sell2 match details (should be the second match)
	trade2 := done.Trades[2]                  // Second maker match
	expectedSell2Fill := fpdecimal.FromInt(2) // Taker needed 12, sell1 provided 10, sell2 provides remaining 2
	if trade2.OrderID != "sell2" || !trade2.Quantity.Equal(expectedSell2Fill) || !trade2.Price.Equal(fpdecimal.FromInt(100)) {
		t.Errorf("Trade 2 (Maker Match) details incorrect. Expected MakerID: sell2, Qty: %s, Price: 100. Got: %+v", expectedSell2Fill, trade2)
	}
	// Verify sell2 is partially filled in the backend
	sell2 := backend.GetOrder("sell2")
	expectedSell2Remaining := fpdecimal.FromInt(5).Sub(expectedSell2Fill) // 5 initial - 2 matched
	if sell2 == nil {
		t.Errorf("Expected sell2 to still exist in backend")
	} else if !sell2.Quantity().Equal(expectedSell2Remaining) {
		t.Errorf("Expected sell2 remaining quantity in backend to be %s, got %s", expectedSell2Remaining, sell2.Quantity())
	}

	// 5. Verify sell3 (higher price) is untouched
	sell3 := backend.GetOrder("sell3")
	if sell3 == nil {
		t.Errorf("Expected sell3 to still exist")
	} else if !sell3.Quantity().Equal(fpdecimal.FromInt(10)) {
		t.Errorf("Expected sell3 quantity to be unchanged (10), got %s", sell3.Quantity())
	}

	// 6. Verify buy order is fully processed (check done object)
	if !done.Left.Equal(fpdecimal.Zero) {
		t.Errorf("Expected done.Left to be zero for the taker order, got %s", done.Left)
	}
	// Check the taker order entry in trades
	takerTrade := done.Trades[0]
	if takerTrade.OrderID != "buy1" || !takerTrade.Quantity.Equal(done.Processed) {
		t.Errorf("Taker trade entry incorrect. Expected ID: buy1, Qty: %s. Got: %+v", done.Processed, takerTrade)
	}
}

// TestMultiLevelMatching verifies that a taker order correctly matches across multiple price levels.
func TestMultiLevelMatching(t *testing.T) {
	backend := newMockBackend()
	book := NewOrderBook(backend)

	// Setup sell side with multiple price levels
	// sell1: 5 @ 100
	// sell2: 5 @ 101
	// sell3: 5 @ 102
	sellOrders := []*Order{
		NewLimitOrder("sell1", Sell, fpdecimal.FromInt(5), fpdecimal.FromInt(100), GTC, ""),
		NewLimitOrder("sell2", Sell, fpdecimal.FromInt(5), fpdecimal.FromInt(101), GTC, ""),
		NewLimitOrder("sell3", Sell, fpdecimal.FromInt(5), fpdecimal.FromInt(102), GTC, ""),
	}
	for _, order := range sellOrders {
		_, err := book.Process(order)
		if err != nil {
			t.Fatalf("Failed to process setup order %s: %v", order.ID(), err)
		}
	}

	// Create a buy order that consumes sell1 and sell2 completely, and part of sell3
	buyQty := fpdecimal.FromInt(12)                                                 // Needs 12 total
	buyOrder := NewLimitOrder("buy1", Buy, buyQty, fpdecimal.FromInt(102), GTC, "") // Limit price allows matching up to 102

	// Process the buy order
	done, err := book.Process(buyOrder)
	if err != nil {
		t.Fatalf("Expected no error when processing buy order, got %v", err)
	}

	// --- Verification ---

	// 1. Verify total processed quantity
	expectedProcessed := buyQty
	if !done.Processed.Equal(expectedProcessed) {
		t.Errorf("Expected processed quantity %s, got %s", expectedProcessed, done.Processed)
	}
	if !done.Left.Equal(fpdecimal.Zero) {
		t.Errorf("Expected left quantity zero, got %s", done.Left)
	}

	// 2. Verify number of trades (taker + 3 makers)
	expectedTradeCount := 4
	if len(done.Trades) != expectedTradeCount {
		t.Fatalf("Expected %d trades, got %d", expectedTradeCount, len(done.Trades))
	}

	// 3. Verify matches occurred correctly across levels
	// Match 1: sell1 @ 100 for 5 qty
	trade1 := done.Trades[1]
	if trade1.OrderID != "sell1" || !trade1.Quantity.Equal(fpdecimal.FromInt(5)) || !trade1.Price.Equal(fpdecimal.FromInt(100)) {
		t.Errorf("Trade 1 (sell1) details incorrect. Got: %+v", trade1)
	}
	// Match 2: sell2 @ 101 for 5 qty
	trade2 := done.Trades[2]
	if trade2.OrderID != "sell2" || !trade2.Quantity.Equal(fpdecimal.FromInt(5)) || !trade2.Price.Equal(fpdecimal.FromInt(101)) {
		t.Errorf("Trade 2 (sell2) details incorrect. Got: %+v", trade2)
	}
	// Match 3: sell3 @ 102 for 2 qty (12 total - 5 - 5)
	trade3 := done.Trades[3]
	if trade3.OrderID != "sell3" || !trade3.Quantity.Equal(fpdecimal.FromInt(2)) || !trade3.Price.Equal(fpdecimal.FromInt(102)) {
		t.Errorf("Trade 3 (sell3) details incorrect. Got: %+v", trade3)
	}

	// 4. Verify backend state
	// sell1 and sell2 should be gone
	if backend.GetOrder("sell1") != nil {
		t.Error("Expected sell1 to be removed from backend")
	}
	if backend.GetOrder("sell2") != nil {
		t.Error("Expected sell2 to be removed from backend")
	}
	// sell3 should have remaining quantity
	sell3 := backend.GetOrder("sell3")
	expectedSell3Remaining := fpdecimal.FromInt(5).Sub(fpdecimal.FromInt(2))
	if sell3 == nil {
		t.Error("Expected sell3 to exist in backend")
	} else if !sell3.Quantity().Equal(expectedSell3Remaining) {
		t.Errorf("Expected sell3 remaining qty %s, got %s", expectedSell3Remaining, sell3.Quantity())
	}

	// 5. Verify taker order status in done object
	takerTrade := done.Trades[0]
	if takerTrade.OrderID != "buy1" || !takerTrade.Quantity.Equal(done.Processed) {
		t.Errorf("Taker trade entry incorrect. Got: %+v", takerTrade)
	}
}

// TestMarketOrderNoLiquidity verifies market order behavior when the opposite side is empty.
func TestMarketOrderNoLiquidity(t *testing.T) {
	backend := newMockBackend()
	book := NewOrderBook(backend)

	// Attempt to create a buy market order when there are no sell orders
	buyOrderID := "buy-market-no-liq"
	buyQty := fpdecimal.FromFloat(5.0)
	buyOrder := NewMarketOrder(buyOrderID, Buy, buyQty)

	// Process the buy market order
	done, err := book.Process(buyOrder)

	// --- Verification ---

	// 1. Expect no error, as market orders don't typically fail outright, they just don't fill.
	if err != nil {
		t.Errorf("Expected no error processing market order with no liquidity, got %v", err)
	}
	if done == nil {
		t.Fatalf("Expected a non-nil done object, got nil")
	}

	// 2. Expect zero processed quantity
	if !done.Processed.Equal(fpdecimal.Zero) {
		t.Errorf("Expected zero processed quantity, got %s", done.Processed)
	}

	// 3. Expect the original quantity left (or the quantity field to reflect original)
	if !done.Left.Equal(buyQty) {
		t.Errorf("Expected left quantity %s, got %s", buyQty, done.Left)
	}
	if !done.Quantity.Equal(buyQty) {
		t.Errorf("Expected original quantity %s, got %s", buyQty, done.Quantity)
	}

	// 4. Expect no trades
	// The first entry is always the taker order itself, so expect only that.
	if len(done.Trades) != 1 {
		t.Errorf("Expected only 1 trade entry (the taker itself), got %d", len(done.Trades))
	}

	// 5. Verify the order was not stored in the backend (market orders don't rest)
	if backend.GetOrder(buyOrderID) != nil {
		t.Error("Expected the market order not to be stored in the backend")
	}
}

// TestCancelPendingStopOrder verifies that a non-activated stop order can be canceled.
func TestCancelPendingStopOrder(t *testing.T) {
	backend := newMockBackend()
	book := NewOrderBook(backend)

	// Place a stop-limit order
	stopOrderID := "stop-cancel-test"
	stopPrice := fpdecimal.FromInt(105)
	limitPrice := fpdecimal.FromInt(104)
	qty := fpdecimal.FromInt(10)
	stopOrder := NewStopLimitOrder(stopOrderID, Buy, qty, limitPrice, stopPrice, "")

	// Process the stop order (places it in the stop book)
	_, err := book.Process(stopOrder)
	if err != nil {
		t.Fatalf("Failed to process stop order: %v", err)
	}

	// Verify it's in the backend store and stop book (mock)
	if backend.GetOrder(stopOrderID) == nil {
		t.Fatalf("Expected stop order %s to be stored in backend", stopOrderID)
	}
	stopBook, ok := backend.GetStopBook().([]*Order)
	if !ok || len(stopBook) != 1 || stopBook[0].ID() != stopOrderID {
		t.Fatalf("Expected stop order %s to be in the stop book", stopOrderID)
	}

	// Cancel the pending stop order
	canceledOrder := book.CancelOrder(stopOrderID)

	// --- Verification ---

	// 1. Verify the returned order indicates cancellation
	if canceledOrder == nil {
		t.Fatalf("Expected CancelOrder to return the canceled order, got nil")
	}
	if !canceledOrder.IsCanceled() {
		t.Errorf("Expected returned order to be marked as canceled")
	}
	if canceledOrder.ID() != stopOrderID {
		t.Errorf("Expected canceled order ID %s, got %s", stopOrderID, canceledOrder.ID())
	}

	// 2. Verify the order is removed from the backend store
	if backend.GetOrder(stopOrderID) != nil {
		t.Errorf("Expected stop order %s to be removed from the backend store after cancellation", stopOrderID)
	}

	// 3. Verify the order is removed from the stop book (mock)
	stopBookAfterCancel, ok := backend.GetStopBook().([]*Order)
	if !ok || len(stopBookAfterCancel) != 0 {
		t.Errorf("Expected stop book to be empty after cancellation, got %d orders", len(stopBookAfterCancel))
	}

	// 4. Verify the order is not on the main bid/ask sides
	bids, _ := backend.GetBids().(*mockOrderSide)
	numBids := 0
	for _, orders := range bids.orders {
		numBids += len(orders)
	}
	if numBids != 0 {
		t.Errorf("Expected bid side to be empty, got %d orders", numBids)
	}
}

// TestDuplicateOrderID verifies that processing an order with an existing ID fails.
func TestDuplicateOrderID(t *testing.T) {
	backend := newMockBackend()
	book := NewOrderBook(backend)

	orderID := "duplicate-test-id"
	price := fpdecimal.FromInt(100)
	qty := fpdecimal.FromInt(10)

	// Process the first order
	order1 := NewLimitOrder(orderID, Buy, qty, price, GTC, "")
	_, err := book.Process(order1)
	if err != nil {
		t.Fatalf("Processing first order failed unexpectedly: %v", err)
	}

	// Verify it was stored
	if backend.GetOrder(orderID) == nil {
		t.Fatalf("First order was not stored in the backend")
	}

	// Process a second order with the same ID
	order2 := NewLimitOrder(orderID, Sell, qty, price.Add(fpdecimal.FromInt(1)), GTC, "") // Different details but same ID
	done, err := book.Process(order2)

	// --- Verification ---

	// 1. Expect ErrOrderExists
	if err == nil {
		t.Errorf("Expected an error when processing duplicate order ID, got nil")
	} else if err != ErrOrderExists {
		t.Errorf("Expected ErrOrderExists, got %v", err)
	}

	// 2. Expect nil done object
	if done != nil {
		t.Errorf("Expected nil done object on error, got %+v", done)
	}

	// 3. Verify the original order is still in the backend, unchanged
	originalOrder := backend.GetOrder(orderID)
	if originalOrder == nil {
		t.Errorf("Original order disappeared from backend")
	} else if originalOrder.Side() != Buy || !originalOrder.Quantity().Equal(qty) {
		t.Errorf("Original order was modified. Expected Side=Buy, Qty=%s. Got Side=%s, Qty=%s",
			qty, originalOrder.Side(), originalOrder.Quantity())
	}
}
