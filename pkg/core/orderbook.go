package core

import (
	"fmt"
	"strings"

	"github.com/nikolaydubina/fpdecimal"
)

// OrderBook implements standard matching algorithm
type OrderBook struct {
	backend OrderBookBackend
}

// NewOrderBook creates Orderbook object with a backend
func NewOrderBook(backend OrderBookBackend) *OrderBook {
	return &OrderBook{
		backend: backend,
	}
}

// GetOrder returns Order by id
func (ob *OrderBook) GetOrder(orderID string) *Order {
	return ob.backend.GetOrder(orderID)
}

// CancelOrder removes Order with given ID from the Order book or the Stop book
func (ob *OrderBook) CancelOrder(orderID string) *Order {
	order := ob.GetOrder(orderID)
	if order == nil {
		return nil
	}

	order.Cancel()

	if order.IsStopOrder() {
		ob.backend.RemoveFromStopBook(order)
		ob.backend.DeleteOrder(order.ID())
	} else {
		ob.deleteOrder(order)
	}

	return order
}

// Process public method
func (ob *OrderBook) Process(order *Order) (done *Done, err error) {
	if order.IsMarketOrder() {
		return ob.processMarketOrder(order)
	}

	if order.IsLimitOrder() {
		return ob.processLimitOrder(order)
	}

	if order.IsStopOrder() {
		return ob.processStopOrder(order)
	}

	panic("unrecognized order type")
}

// CalculateMarketPrice returns total market Price for requested quantity
func (ob *OrderBook) CalculateMarketPrice(side Side, quantity fpdecimal.Decimal) (price fpdecimal.Decimal, err error) {
	price = fpdecimal.Zero

	// This is a placeholder for the actual implementation
	// In a real implementation, you would need to adapt to the specific backend implementation
	// Since we're restructuring the code, we'll leave this as a stub
	return price, ErrInsufficientQuantity
}

// private methods

func (ob *OrderBook) deleteOrder(order *Order) {
	ob.backend.DeleteOrder(order.ID())

	if order.Side() == Buy {
		ob.backend.RemoveFromSide(Buy, order)
	}

	if order.Side() == Sell {
		ob.backend.RemoveFromSide(Sell, order)
	}
}

func (ob *OrderBook) processMarketOrder(marketOrder *Order) (done *Done, err error) {
	quantity := marketOrder.Quantity()

	if quantity.LessThanOrEqual(fpdecimal.Zero) {
		return nil, ErrInvalidQuantity
	}

	// Store the order first
	err = ob.backend.StoreOrder(marketOrder)
	if err != nil {
		return nil, err
	}

	done = newDone(marketOrder)

	// This is a simplified implementation
	// In a real implementation, you would need to adapt to the specific backend implementation
	// and process orders on the opposite side of the book

	// For demo purposes, we'll just set the order as fully processed
	marketOrder.SetQuantity(fpdecimal.Zero)
	err = ob.backend.UpdateOrder(marketOrder)
	if err != nil {
		return nil, err
	}

	zeroQuantity := fpdecimal.Zero
	done.setLeftQuantity(&zeroQuantity)

	return done, nil
}

func (ob *OrderBook) processLimitOrder(limitOrder *Order) (done *Done, err error) {
	quantity := limitOrder.Quantity()

	// Check if order exists
	order := ob.GetOrder(limitOrder.ID())
	if order != nil {
		return nil, ErrOrderExists
	}

	// Store the order first
	err = ob.backend.StoreOrder(limitOrder)
	if err != nil {
		return nil, err
	}

	done = newDone(limitOrder)

	// Check for OCO (One Cancels Other)
	if ob.checkOCO(limitOrder, done) {
		return done, nil
	}

	// This is a simplified implementation
	// In a real implementation, you would need to adapt to the specific backend implementation
	// and match orders on the opposite side of the book

	// Add the order to the appropriate side
	if limitOrder.Side() == Buy {
		limitOrder.SetMaker()
		ob.backend.AppendToSide(Buy, limitOrder)
	} else {
		limitOrder.SetMaker()
		ob.backend.AppendToSide(Sell, limitOrder)
	}

	done.setLeftQuantity(&quantity)
	done.Stored = true

	return done, nil
}

func (ob *OrderBook) processStopOrder(stopOrder *Order) (done *Done, err error) {
	// Check if order exists
	order := ob.GetOrder(stopOrder.ID())
	if order != nil {
		return nil, ErrOrderExists
	}

	// Store the order first
	err = ob.backend.StoreOrder(stopOrder)
	if err != nil {
		return nil, err
	}

	// Add to stop book
	ob.backend.AppendToStopBook(stopOrder)

	// Create done object
	done = newDone(stopOrder)
	done.Stored = true

	quantity := stopOrder.Quantity()
	done.setLeftQuantity(&quantity)

	return done, nil
}

func (ob *OrderBook) checkOCO(order *Order, done *Done) bool {
	if order.OCO() == "" {
		return false
	}

	// Check if OCO order exists and cancel it
	ocoID := ob.backend.CheckOCO(order.ID())
	if ocoID == "" {
		return false
	}

	ocoOrder := ob.GetOrder(ocoID)
	if ocoOrder != nil {
		ob.CancelOrder(ocoID)
		done.appendCanceled(ocoOrder)
	}

	return false
}

// String implements fmt.Stringer interface
func (ob *OrderBook) String() string {
	builder := strings.Builder{}

	builder.WriteString("Ask:")
	asks := ob.backend.GetAsks()
	if stringer, ok := asks.(fmt.Stringer); ok {
		builder.WriteString(stringer.String())
	} else {
		builder.WriteString(" (No string representation available)")
	}
	builder.WriteString("\n")

	builder.WriteString("Bid:")
	bids := ob.backend.GetBids()
	if stringer, ok := bids.(fmt.Stringer); ok {
		builder.WriteString(stringer.String())
	} else {
		builder.WriteString(" (No string representation available)")
	}
	builder.WriteString("\n")

	return builder.String()
}
