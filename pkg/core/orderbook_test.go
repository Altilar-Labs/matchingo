package core

import (
	"testing"

	"github.com/nikolaydubina/fpdecimal"
)

// mockBackend implements the OrderBookBackend interface for testing
type mockBackend struct{}

func (m *mockBackend) GetOrder(orderID string) *Order              { return nil }
func (m *mockBackend) StoreOrder(order *Order) error               { return nil }
func (m *mockBackend) UpdateOrder(order *Order) error              { return nil }
func (m *mockBackend) DeleteOrder(orderID string)                  {}
func (m *mockBackend) AppendToSide(side Side, order *Order)        {}
func (m *mockBackend) RemoveFromSide(side Side, order *Order) bool { return true }
func (m *mockBackend) AppendToStopBook(order *Order)               {}
func (m *mockBackend) RemoveFromStopBook(order *Order) bool        { return true }
func (m *mockBackend) CheckOCO(orderID string) string              { return "" }
func (m *mockBackend) GetBids() interface{}                        { return nil }
func (m *mockBackend) GetAsks() interface{}                        { return nil }
func (m *mockBackend) GetStopBook() interface{}                    { return nil }

func TestOrderBookCreation(t *testing.T) {
	backend := &mockBackend{}
	book := NewOrderBook(backend)

	if book == nil {
		t.Error("Expected OrderBook to be created, got nil")
	}
}

func TestOrderProcessing(t *testing.T) {
	backend := &mockBackend{}
	book := NewOrderBook(backend)

	// Create a new limit order
	orderID := "test-order-1"
	price := fpdecimal.FromFloat(10.0)
	quantity := fpdecimal.FromFloat(5.0)
	order := NewLimitOrder(orderID, Buy, quantity, price, GTC, "")

	// Process the order
	_, err := book.Process(order)
	if err != nil {
		t.Errorf("Expected no error when processing order, got %v", err)
	}
}
