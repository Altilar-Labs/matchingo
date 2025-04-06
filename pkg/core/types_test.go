package core

import (
	"encoding/json"
	"testing"

	"github.com/nikolaydubina/fpdecimal"
)

func TestTradeOrder_MarshalJSON(t *testing.T) {
	// Create a trade order
	tradeOrder := &TradeOrder{
		OrderID:  "test-123",
		Role:     MAKER,
		Price:    fpdecimal.FromFloat(100.0),
		IsQuote:  false,
		Quantity: fpdecimal.FromFloat(10.0),
	}

	// Marshal to JSON
	data, err := tradeOrder.MarshalJSON()
	if err != nil {
		t.Fatalf("Failed to marshal TradeOrder: %v", err)
	}

	// Parse the JSON to verify
	var jsonMap map[string]interface{}
	err = json.Unmarshal(data, &jsonMap)
	if err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	// Check that fields are present and correct
	if jsonMap["orderID"] != "test-123" {
		t.Errorf("Expected orderID test-123, got %v", jsonMap["orderID"])
	}

	if jsonMap["role"] != "MAKER" {
		t.Errorf("Expected role MAKER, got %v", jsonMap["role"])
	}

	if price, ok := jsonMap["price"].(string); !ok || price != "100.000" {
		t.Errorf("Expected price 100.000, got %v", jsonMap["price"])
	}

	if jsonMap["isQuote"] != false {
		t.Errorf("Expected isQuote false, got %v", jsonMap["isQuote"])
	}

	if quantity, ok := jsonMap["quantity"].(string); !ok || quantity != "10.000" {
		t.Errorf("Expected quantity 10.000, got %v", jsonMap["quantity"])
	}
}

func TestDone_GetTradeOrder(t *testing.T) {
	// Create an order
	orderID := "test-123"
	price := fpdecimal.FromFloat(100.0)
	quantity := fpdecimal.FromFloat(10.0)
	order := NewLimitOrder(orderID, Buy, quantity, price, GTC, "")

	// Create a Done object
	done := newDone(order)

	// Initially, there should be no trades
	tradeOrder := done.GetTradeOrder(orderID)
	if tradeOrder != nil {
		t.Errorf("Expected nil trade order initially, got %v", tradeOrder)
	}

	// Append an order as a trade
	matchOrderID := "match-123"
	matchOrder := NewLimitOrder(matchOrderID, Sell, quantity, price, GTC, "")
	matchQuantity := fpdecimal.FromFloat(5.0)
	done.appendOrder(matchOrder, matchQuantity, price)

	// Now we should have trades
	tradeOrder = done.GetTradeOrder(matchOrderID)
	if tradeOrder == nil {
		t.Error("Expected non-nil trade order after appending")
	} else {
		if tradeOrder.OrderID != matchOrderID {
			t.Errorf("Expected OrderID %s, got %s", matchOrderID, tradeOrder.OrderID)
		}

		if !tradeOrder.Quantity.Equal(matchQuantity) {
			t.Errorf("Expected Quantity %v, got %v", matchQuantity, tradeOrder.Quantity)
		}
	}
}

func TestDone_TradesToSlice(t *testing.T) {
	// Create an order
	orderID := "test-123"
	price := fpdecimal.FromFloat(100.0)
	quantity := fpdecimal.FromFloat(10.0)
	order := NewLimitOrder(orderID, Buy, quantity, price, GTC, "")

	// Create a Done object
	done := newDone(order)

	// Initially, there should be no trades
	trades := done.tradesToSlice()
	if len(trades) != 0 {
		t.Errorf("Expected empty trades slice initially, got %d trades", len(trades))
	}

	// Append an order as a trade
	matchOrderID := "match-123"
	matchOrder := NewLimitOrder(matchOrderID, Sell, quantity, price, GTC, "")
	matchQuantity := fpdecimal.FromFloat(5.0)
	done.appendOrder(matchOrder, matchQuantity, price)

	// Now we should have trades
	trades = done.tradesToSlice()
	if len(trades) != 2 { // One for the original order and one for the matched order
		t.Errorf("Expected 2 trades, got %d", len(trades))
	}
}

func TestDone_CancelAndActivation(t *testing.T) {
	// Create an order
	orderID := "test-123"
	price := fpdecimal.FromFloat(100.0)
	quantity := fpdecimal.FromFloat(10.0)
	order := NewLimitOrder(orderID, Buy, quantity, price, GTC, "")

	// Create a Done object
	done := newDone(order)

	// Initially, there should be no canceled or activated orders
	if len(done.Canceled) != 0 {
		t.Errorf("Expected empty canceled list initially, got %d", len(done.Canceled))
	}

	if len(done.Activated) != 0 {
		t.Errorf("Expected empty activated list initially, got %d", len(done.Activated))
	}

	// Append a canceled order
	canceledOrder := NewLimitOrder("cancel-123", Sell, quantity, price, GTC, "")
	done.appendCanceled(canceledOrder)

	if len(done.Canceled) != 1 {
		t.Errorf("Expected 1 canceled order, got %d", len(done.Canceled))
	}

	if done.Canceled[0] != "cancel-123" {
		t.Errorf("Expected canceled ID cancel-123, got %s", done.Canceled[0])
	}

	// Append an activated order
	activatedOrder := NewLimitOrder("activate-123", Sell, quantity, price, GTC, "")
	done.appendActivated(activatedOrder)

	if len(done.Activated) != 1 {
		t.Errorf("Expected 1 activated order, got %d", len(done.Activated))
	}

	if done.Activated[0] != "activate-123" {
		t.Errorf("Expected activated ID activate-123, got %s", done.Activated[0])
	}
}

func TestDone_SetLeftQuantity(t *testing.T) {
	// Create an order
	orderID := "test-123"
	price := fpdecimal.FromFloat(100.0)
	quantity := fpdecimal.FromFloat(10.0)
	order := NewLimitOrder(orderID, Buy, quantity, price, GTC, "")

	// Create a Done object
	done := newDone(order)

	// Initially, left quantity should be zero and processed should be zero
	if !done.Left.Equal(fpdecimal.Zero) {
		t.Errorf("Expected Left to be 0 initially, got %v", done.Left)
	}

	if !done.Processed.Equal(fpdecimal.Zero) {
		t.Errorf("Expected Processed to be 0 initially, got %v", done.Processed)
	}

	// Set left quantity
	leftQuantity := fpdecimal.FromFloat(2.0)
	done.setLeftQuantity(&leftQuantity)

	if !done.Left.Equal(leftQuantity) {
		t.Errorf("Expected Left to be %v after setLeftQuantity, got %v", leftQuantity, done.Left)
	}

	expectedProcessed := quantity.Sub(leftQuantity)
	if !done.Processed.Equal(expectedProcessed) {
		t.Errorf("Expected Processed to be %v after setLeftQuantity, got %v", expectedProcessed, done.Processed)
	}
}

func TestDone_MarshalJSON(t *testing.T) {
	// Create an order
	orderID := "test-123"
	price := fpdecimal.FromFloat(100.0)
	quantity := fpdecimal.FromFloat(10.0)
	order := NewLimitOrder(orderID, Buy, quantity, price, GTC, "")
	order.SetMaker()

	// Create a Done object
	done := newDone(order)

	// Add some data to the Done object
	cancelID := "cancel-123"
	activateID := "activate-123"
	done.appendCanceled(&Order{id: cancelID})
	done.appendActivated(&Order{id: activateID})

	leftQuantity := fpdecimal.FromFloat(2.0)
	done.setLeftQuantity(&leftQuantity)

	done.Stored = true

	// Marshal to JSON
	jsonData, err := done.MarshalJSON()
	if err != nil {
		t.Fatalf("Failed to marshal Done to JSON: %v", err)
	}

	// Unmarshal to a map for easier inspection
	var jsonMap map[string]interface{}
	if err := json.Unmarshal(jsonData, &jsonMap); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	// Check fields
	if order, ok := jsonMap["order"].(map[string]interface{}); !ok || order["orderID"] != "test-123" {
		t.Errorf("Expected order ID test-123, got %v", jsonMap["order"])
	}

	if left, ok := jsonMap["left"].(string); !ok || left != "2.000" {
		t.Errorf("Expected left 2.000, got %v", jsonMap["left"])
	}

	if processed, ok := jsonMap["processed"].(string); !ok || processed != "8.000" {
		t.Errorf("Expected processed 8.000, got %v", jsonMap["processed"])
	}

	cancels := jsonMap["canceled"].([]interface{})
	if len(cancels) != 1 || cancels[0] != cancelID {
		t.Errorf("Expected 1 canceled order with ID %s, got %v", cancelID, cancels)
	}

	activates := jsonMap["activated"].([]interface{})
	if len(activates) != 1 || activates[0] != activateID {
		t.Errorf("Expected 1 activated order with ID %s, got %v", activateID, activates)
	}
}
