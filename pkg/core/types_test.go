package core

import (
	"encoding/json"
	"testing"

	"github.com/nikolaydubina/fpdecimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTradeOrder_MarshalJSON(t *testing.T) {
	trade := TradeOrder{
		OrderID:  "test-123",
		Role:     MAKER,
		IsQuote:  false,
		Price:    fpdecimal.FromFloat(100.5),
		Quantity: fpdecimal.FromFloat(10.0),
	}

	jsonData, err := json.Marshal(&trade)
	if err != nil {
		t.Fatalf("Failed to marshal TradeOrder: %v", err)
	}

	expected := `{"orderID":"test-123","role":"MAKER","isQuote":false,"price":"100.500","quantity":"10.000"}`
	if string(jsonData) != expected {
		t.Errorf("Expected JSON %s, got %s", expected, string(jsonData))
	}
}

func TestNewDone(t *testing.T) {
	orderID := "test-123"
	quantity := fpdecimal.FromFloat(10.0)
	price := fpdecimal.FromFloat(100.0)
	order, err := NewLimitOrder(orderID, Buy, quantity, price, GTC, "")
	require.NoError(t, err)

	done := newDone(order)

	if done == nil {
		t.Fatal("Expected non-nil Done object")
	}

	if done.Order == nil || done.Order.ID() != orderID {
		t.Errorf("Expected Done.Order to be the input order")
	}

	if len(done.Trades) != 0 {
		t.Error("Expected empty Trades slice initially")
	}

	if len(done.Canceled) != 0 {
		t.Error("Expected empty Canceled slice initially")
	}

	if len(done.Activated) != 0 {
		t.Error("Expected empty Activated slice initially")
	}

	if !done.Quantity.Equal(quantity) {
		t.Errorf("Expected Done.Quantity %v, got %v", quantity, done.Quantity)
	}

	if !done.Left.Equal(fpdecimal.Zero) {
		t.Errorf("Expected Done.Left 0, got %v", done.Left)
	}

	if !done.Processed.Equal(fpdecimal.Zero) {
		t.Errorf("Expected Done.Processed 0, got %v", done.Processed)
	}
}

func TestDone_GetTradeOrder(t *testing.T) {
	// Create an order
	orderID := "test-123"
	price := fpdecimal.FromFloat(100.0)
	quantity := fpdecimal.FromFloat(10.0)
	order, err := NewLimitOrder(orderID, Buy, quantity, price, GTC, "")
	require.NoError(t, err)

	// Create a Done object
	done := newDone(order)

	// Initially, there should be no trades
	tradeOrder := done.GetTradeOrder(orderID)
	if tradeOrder != nil {
		t.Errorf("Expected nil trade order initially, got %v", tradeOrder)
	}

	// Append an order as a trade
	matchOrderID := "match-123"
	matchOrder, err := NewLimitOrder(matchOrderID, Sell, quantity, price, GTC, "")
	require.NoError(t, err)
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
	order, err := NewLimitOrder(orderID, Buy, quantity, price, GTC, "")
	require.NoError(t, err)

	// Create a Done object
	done := newDone(order)

	// Initially, there should be no trades
	trades := done.tradesToSlice()
	if len(trades) != 0 {
		t.Errorf("Expected empty trades slice initially, got %d trades", len(trades))
	}

	// Append an order as a trade
	matchOrderID := "match-123"
	matchOrder, err := NewLimitOrder(matchOrderID, Sell, quantity, price, GTC, "")
	require.NoError(t, err)
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
	order, err := NewLimitOrder(orderID, Buy, quantity, price, GTC, "")
	require.NoError(t, err)

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
	canceledOrder, err := NewLimitOrder("cancel-123", Sell, quantity, price, GTC, "")
	require.NoError(t, err)
	done.appendCanceled(canceledOrder)

	if len(done.Canceled) != 1 {
		t.Errorf("Expected 1 canceled order, got %d", len(done.Canceled))
	}

	if done.Canceled[0].ID() != "cancel-123" {
		t.Errorf("Expected canceled ID cancel-123, got %s", done.Canceled[0].ID())
	}

	// Append an activated order
	activatedOrder, err := NewLimitOrder("activate-123", Sell, quantity, price, GTC, "")
	require.NoError(t, err)
	done.appendActivated(activatedOrder)

	if len(done.Activated) != 1 {
		t.Errorf("Expected 1 activated order, got %d", len(done.Activated))
	}

	if done.Activated[0].ID() != "activate-123" {
		t.Errorf("Expected activated ID activate-123, got %s", done.Activated[0].ID())
	}
}

func TestDone_SetLeftQuantity(t *testing.T) {
	// Create an order
	orderID := "test-123"
	price := fpdecimal.FromFloat(100.0)
	quantity := fpdecimal.FromFloat(10.0)
	order, err := NewLimitOrder(orderID, Buy, quantity, price, GTC, "")
	require.NoError(t, err)

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
	done.SetLeftQuantity(&leftQuantity)

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
	order, err := NewLimitOrder(orderID, Buy, quantity, price, GTC, "")
	require.NoError(t, err)
	order.SetMaker()

	// Create a Done object
	done := newDone(order)

	// Add some data to the Done object
	cancelID := "cancel-123"
	activateID := "activate-123"
	canceledOrder, err := NewLimitOrder(cancelID, Sell, quantity, price, GTC, "")
	require.NoError(t, err)
	activatedOrder, err := NewLimitOrder(activateID, Sell, quantity, price, GTC, "")
	require.NoError(t, err)
	done.appendCanceled(canceledOrder)
	done.appendActivated(activatedOrder)

	leftQuantity := fpdecimal.FromFloat(2.0)
	done.SetLeftQuantity(&leftQuantity)

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
	if orderJSON, ok := jsonMap["order"].(map[string]interface{}); !ok || orderJSON["orderID"] != "test-123" {
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

func TestDone_ToMessagingDoneMessage(t *testing.T) {
	// Create an order
	orderID := "test-123"
	quantity := fpdecimal.FromFloat(10.0)
	price := fpdecimal.FromFloat(100.0)
	order, err := NewLimitOrder(orderID, Buy, quantity, price, GTC, "")
	require.NoError(t, err)

	// Create a Done object
	done := newDone(order)

	// Add some data
	leftQty := fpdecimal.FromFloat(3.0)
	done.SetLeftQuantity(&leftQty)
	done.Stored = true
	done.appendCanceled(&Order{id: "cancel-1"})
	done.appendActivated(&Order{id: "activate-1"})

	// Add a trade
	matchOrder, err := NewLimitOrder("match-1", Sell, fpdecimal.FromFloat(7.0), price, GTC, "")
	require.NoError(t, err)
	done.appendOrder(matchOrder, fpdecimal.FromFloat(7.0), price)

	// Convert to messaging format
	msg := done.ToMessagingDoneMessage()
	require.NotNil(t, msg)

	// Assertions
	assert.Equal(t, orderID, msg.OrderID)
	assert.Equal(t, "7.000", msg.ExecutedQty)
	assert.Equal(t, "3.000", msg.RemainingQty)
	assert.Equal(t, []string{"cancel-1"}, msg.Canceled)
	assert.Equal(t, []string{"activate-1"}, msg.Activated)
	assert.True(t, msg.Stored)
	assert.Equal(t, "10.000", msg.Quantity)
	assert.Equal(t, "7.000", msg.Processed)
	assert.Equal(t, "3.000", msg.Left)
	require.Len(t, msg.Trades, 2)
	assert.Equal(t, orderID, msg.Trades[0].OrderID)
	assert.Equal(t, "7.000", msg.Trades[0].Quantity)
	assert.Equal(t, "match-1", msg.Trades[1].OrderID)
	assert.Equal(t, "7.000", msg.Trades[1].Quantity)
}
