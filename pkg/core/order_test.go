package core

import (
	"encoding/json"
	"testing"

	"github.com/nikolaydubina/fpdecimal"
)

func TestSideString(t *testing.T) {
	tests := []struct {
		name string
		side Side
		want string
	}{
		{"Buy", Buy, "BUY"},
		{"Sell", Sell, "SELL"},
		{"Invalid", Side(999), "UNKNOWN"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.side.String(); got != tt.want {
				t.Errorf("Side.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewMarketOrder(t *testing.T) {
	orderID := "test-123"
	quantity := fpdecimal.FromFloat(10.5)

	order := NewMarketOrder(orderID, Buy, quantity)

	if order.ID() != orderID {
		t.Errorf("Expected ID %s, got %s", orderID, order.ID())
	}

	if order.Side() != Buy {
		t.Errorf("Expected Side Buy, got %v", order.Side())
	}

	if !order.Quantity().Equal(quantity) {
		t.Errorf("Expected Quantity %v, got %v", quantity, order.Quantity())
	}

	if !order.OriginalQty().Equal(quantity) {
		t.Errorf("Expected OriginalQty %v, got %v", quantity, order.OriginalQty())
	}

	if !order.Price().Equal(fpdecimal.Zero) {
		t.Errorf("Expected Price 0, got %v", order.Price())
	}

	if order.IsCanceled() {
		t.Error("Expected order not to be canceled")
	}

	if order.IsQuote() {
		t.Error("Expected order not to be quote")
	}

	if !order.IsMarketOrder() {
		t.Error("Expected IsMarketOrder to be true")
	}

	if order.IsLimitOrder() {
		t.Error("Expected IsLimitOrder to be false")
	}

	if order.IsStopOrder() {
		t.Error("Expected IsStopOrder to be false")
	}
}

func TestNewMarketQuoteOrder(t *testing.T) {
	orderID := "test-123"
	quantity := fpdecimal.FromFloat(10.5)

	order := NewMarketQuoteOrder(orderID, Buy, quantity)

	if !order.IsQuote() {
		t.Error("Expected order to be quote")
	}

	if !order.IsMarketOrder() {
		t.Error("Expected IsMarketOrder to be true")
	}
}

func TestNewLimitOrder(t *testing.T) {
	orderID := "test-123"
	quantity := fpdecimal.FromFloat(10.5)
	price := fpdecimal.FromFloat(100.0)

	order := NewLimitOrder(orderID, Sell, quantity, price, GTC, "")

	if order.ID() != orderID {
		t.Errorf("Expected ID %s, got %s", orderID, order.ID())
	}

	if order.Side() != Sell {
		t.Errorf("Expected Side Sell, got %v", order.Side())
	}

	if !order.Quantity().Equal(quantity) {
		t.Errorf("Expected Quantity %v, got %v", quantity, order.Quantity())
	}

	if !order.Price().Equal(price) {
		t.Errorf("Expected Price %v, got %v", price, order.Price())
	}

	if !order.IsLimitOrder() {
		t.Error("Expected IsLimitOrder to be true")
	}

	if order.TIF() != GTC {
		t.Errorf("Expected TIF GTC, got %v", order.TIF())
	}
}

func TestNewStopLimitOrder(t *testing.T) {
	orderID := "test-123"
	quantity := fpdecimal.FromFloat(10.5)
	price := fpdecimal.FromFloat(100.0)
	stopPrice := fpdecimal.FromFloat(105.0)

	order := NewStopLimitOrder(orderID, Sell, quantity, price, stopPrice, "")

	if !order.IsStopOrder() {
		t.Error("Expected IsStopOrder to be true")
	}

	if !order.StopPrice().Equal(stopPrice) {
		t.Errorf("Expected StopPrice %v, got %v", stopPrice, order.StopPrice())
	}
}

func TestOrderJSON(t *testing.T) {
	orderID := "test-123"
	quantity := fpdecimal.FromFloat(10.5)
	price := fpdecimal.FromFloat(100.0)

	order := NewLimitOrder(orderID, Buy, quantity, price, GTC, "oco-456")

	// Test Marshal
	data, err := json.Marshal(order)
	if err != nil {
		t.Fatalf("Failed to marshal order: %v", err)
	}

	// Test Unmarshal
	var newOrder Order
	err = json.Unmarshal(data, &newOrder)
	if err != nil {
		t.Fatalf("Failed to unmarshal order: %v", err)
	}

	if newOrder.ID() != orderID {
		t.Errorf("Expected ID %s, got %s", orderID, newOrder.ID())
	}

	if newOrder.Side() != Buy {
		t.Errorf("Expected Side Buy, got %v", newOrder.Side())
	}

	if !newOrder.Quantity().Equal(quantity) {
		t.Errorf("Expected Quantity %v, got %v", quantity, newOrder.Quantity())
	}

	if !newOrder.Price().Equal(price) {
		t.Errorf("Expected Price %v, got %v", price, newOrder.Price())
	}

	if newOrder.OCO() != "oco-456" {
		t.Errorf("Expected OCO oco-456, got %v", newOrder.OCO())
	}
}

func TestOrderSettersAndGetters(t *testing.T) {
	orderID := "test-123"
	quantity := fpdecimal.FromFloat(10.5)
	price := fpdecimal.FromFloat(100.0)

	order := NewLimitOrder(orderID, Buy, quantity, price, GTC, "")

	// Test cancel
	if order.IsCanceled() {
		t.Error("Order should not be canceled initially")
	}

	order.Cancel()

	if !order.IsCanceled() {
		t.Error("Order should be canceled after Cancel() call")
	}

	// Test quantity modification
	newQuantity := fpdecimal.FromFloat(5.0)
	order.SetQuantity(newQuantity)

	if !order.Quantity().Equal(newQuantity) {
		t.Errorf("Expected Quantity %v after SetQuantity, got %v", newQuantity, order.Quantity())
	}

	// Test decrease quantity
	decrease := fpdecimal.FromFloat(2.0)
	expectedAfterDecrease := newQuantity.Sub(decrease)
	order.DecreaseQuantity(decrease)

	if !order.Quantity().Equal(expectedAfterDecrease) {
		t.Errorf("Expected Quantity %v after DecreaseQuantity, got %v", expectedAfterDecrease, order.Quantity())
	}

	// Test role
	if order.Role() != TAKER {
		t.Errorf("Expected default Role TAKER, got %v", order.Role())
	}

	order.SetMaker()

	if order.Role() != MAKER {
		t.Errorf("Expected Role MAKER after SetMaker(), got %v", order.Role())
	}

	order.SetTaker()

	if order.Role() != TAKER {
		t.Errorf("Expected Role TAKER after SetTaker(), got %v", order.Role())
	}
}

func TestActivateStopOrder(t *testing.T) {
	orderID := "test-123"
	quantity := fpdecimal.FromFloat(10.5)
	price := fpdecimal.FromFloat(100.0)
	stopPrice := fpdecimal.FromFloat(105.0)

	order := NewStopLimitOrder(orderID, Sell, quantity, price, stopPrice, "")

	if !order.IsStopOrder() {
		t.Error("Order should be a stop order initially")
	}

	order.ActivateStopOrder()

	if order.IsStopOrder() {
		t.Error("Order should not be a stop order after activation")
	}

	if !order.IsLimitOrder() {
		t.Error("Order should be a limit order after activation")
	}

	if !order.StopPrice().Equal(fpdecimal.Zero) {
		t.Errorf("Expected stop price to be 0 after activation, got %v", order.StopPrice())
	}
}

func TestToSimple(t *testing.T) {
	orderID := "test-123"
	quantity := fpdecimal.FromFloat(10.5)
	price := fpdecimal.FromFloat(100.0)

	order := NewLimitOrder(orderID, Buy, quantity, price, GTC, "")
	order.SetMaker()

	simple := order.ToSimple()

	if simple.OrderID != orderID {
		t.Errorf("Expected simple.OrderID %s, got %s", orderID, simple.OrderID)
	}

	if simple.Role != MAKER {
		t.Errorf("Expected simple.Role MAKER, got %v", simple.Role)
	}

	if !simple.Price.Equal(price) {
		t.Errorf("Expected simple.Price %v, got %v", price, simple.Price)
	}

	if !simple.Quantity.Equal(quantity) {
		t.Errorf("Expected simple.Quantity %v, got %v", quantity, simple.Quantity)
	}

	if simple.IsQuote {
		t.Error("Expected simple.IsQuote to be false")
	}
}
