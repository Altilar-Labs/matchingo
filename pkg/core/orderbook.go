package core

import (
	"fmt"
	"strings"
	"sync"

	"github.com/erain9/matchingo/pkg/db/queue"
	"github.com/erain9/matchingo/pkg/messaging"
	"github.com/nikolaydubina/fpdecimal"
)

// --- Message Sender Factory ---

var (
	messageSenderFactory func() messaging.MessageSender
	factoryMu            sync.Mutex
)

// defaultMessageSenderFactory returns the default Kafka sender.
func defaultMessageSenderFactory() messaging.MessageSender {
	return &queue.QueueMessageSender{}
}

// SetMessageSenderFactory allows overriding the default message sender, primarily for testing.
// Pass nil to reset to the default Kafka sender.
func SetMessageSenderFactory(factory func() messaging.MessageSender) {
	factoryMu.Lock()
	defer factoryMu.Unlock()
	if factory == nil {
		messageSenderFactory = defaultMessageSenderFactory
	} else {
		messageSenderFactory = factory
	}
}

// getMessageSender returns the currently configured message sender.
func getMessageSender() messaging.MessageSender {
	factoryMu.Lock()
	defer factoryMu.Unlock()
	// Initialize with default if not set
	if messageSenderFactory == nil {
		messageSenderFactory = defaultMessageSenderFactory
	}
	return messageSenderFactory()
}

// --- OrderBook ---

// OrderBook implements standard matching algorithm
type OrderBook struct {
	backend        OrderBookBackend
	lastTradePrice fpdecimal.Decimal
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
	remaining := quantity

	// For buy orders, check sell side (asks)
	// For sell orders, check buy side (bids)
	if side == Buy {
		asks := ob.backend.GetAsks()
		if askSide, isOrderSide := asks.(interface {
			Prices() []fpdecimal.Decimal
			Orders(price fpdecimal.Decimal) []*Order
		}); isOrderSide {
			prices := askSide.Prices()
			for _, p := range prices {
				orders := askSide.Orders(p)
				for _, order := range orders {
					orderQty := order.Quantity()
					if remaining.LessThanOrEqual(orderQty) {
						// This order can fill the entire remaining quantity
						price = price.Add(p.Mul(remaining))
						remaining = fpdecimal.Zero
						break
					} else {
						// This order can only partially fill the remaining quantity
						price = price.Add(p.Mul(orderQty))
						remaining = remaining.Sub(orderQty)
					}
				}
				if remaining.Equal(fpdecimal.Zero) {
					break
				}
			}
		}
	} else {
		bids := ob.backend.GetBids()
		if bidSide, isOrderSide := bids.(interface {
			Prices() []fpdecimal.Decimal
			Orders(price fpdecimal.Decimal) []*Order
		}); isOrderSide {
			prices := bidSide.Prices()
			for _, p := range prices {
				orders := bidSide.Orders(p)
				for _, order := range orders {
					orderQty := order.Quantity()
					if remaining.LessThanOrEqual(orderQty) {
						// This order can fill the entire remaining quantity
						price = price.Add(p.Mul(remaining))
						remaining = fpdecimal.Zero
						break
					} else {
						// This order can only partially fill the remaining quantity
						price = price.Add(p.Mul(orderQty))
						remaining = remaining.Sub(orderQty)
					}
				}
				if remaining.Equal(fpdecimal.Zero) {
					break
				}
			}
		}
	}

	if !remaining.Equal(fpdecimal.Zero) {
		return fpdecimal.Zero, ErrInsufficientQuantity
	}

	return price, nil
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

func (ob *OrderBook) processMarketOrder(marketOrder *Order) (*Done, error) {
	quantity := marketOrder.Quantity()

	if quantity.LessThanOrEqual(fpdecimal.Zero) {
		return nil, ErrInvalidQuantity
	}

	// Store the order first
	err := ob.backend.StoreOrder(marketOrder)
	if err != nil {
		return nil, err
	}

	done := newDone(marketOrder)
	remainingQty := quantity

	// Set market order as taker
	marketOrder.SetTaker()

	// Process against the opposite side of the book
	oppositeOrders := ob.getOppositeOrders(marketOrder.Side())
	if ordersInterface, isOrderSideInterface := oppositeOrders.(interface {
		Prices() []fpdecimal.Decimal
		Orders(price fpdecimal.Decimal) []*Order
	}); isOrderSideInterface {
		prices := ordersInterface.Prices()

		// Process each price level
		for _, price := range prices {
			if remainingQty.Equal(fpdecimal.Zero) {
				break
			}

			// Get all orders at this price level
			orders := ordersInterface.Orders(price)

			// Process each order at this price level
			for _, order := range orders {
				if remainingQty.Equal(fpdecimal.Zero) {
					break
				}

				// Determine the execution quantity
				executionQty := min(remainingQty, order.Quantity())
				remainingQty = remainingQty.Sub(executionQty)

				// Update the matched order's quantity
				order.DecreaseQuantity(executionQty)

				// Record the trade in the done object
				done.appendOrder(order, executionQty, price)

				// Update the order in the backend
				err = ob.backend.UpdateOrder(order)
				if err != nil {
					return nil, err
				}

				// If order is fully executed, remove it from the book
				if order.Quantity().Equal(fpdecimal.Zero) {
					ob.deleteOrder(order)
				}

				// Update the last trade price to the current execution price
				ob.lastTradePrice = price
			}
		}
	}

	// Calculate processed quantity
	processedQty := quantity.Sub(remainingQty)

	// Check if the market order was partially filled and is IOC
	if remainingQty.GreaterThan(fpdecimal.Zero) && marketOrder.TIF() == IOC {
		// For IOC market orders with remaining quantity, cancel the rest
		done.appendCanceled(marketOrder)
		done.Left = remainingQty
		done.Processed = processedQty

		// Add the taker order to trades with the processed quantity
		if processedQty.GreaterThan(fpdecimal.Zero) {
			done.appendOrder(marketOrder, processedQty, ob.lastTradePrice)
		} else {
			// If no matches found, still add the taker to trades with zero quantity
			done.appendOrder(marketOrder, fpdecimal.Zero, fpdecimal.Zero)
		}

		// Delete the market order
		ob.backend.DeleteOrder(marketOrder.ID())

		// Send to Kafka if any trades occurred
		if processedQty.GreaterThan(fpdecimal.Zero) {
			// Trigger stop orders with the last trade price
			ob.checkStopOrderTrigger(ob.lastTradePrice)
			sendToKafka(done)
		}

		return done, nil
	}

	// For fully filled orders or non-IOC orders
	if processedQty.GreaterThan(fpdecimal.Zero) {
		done.appendOrder(marketOrder, processedQty, ob.lastTradePrice)
	} else {
		// For market orders with no matches, still add the taker to trades with zero quantity
		done.appendOrder(marketOrder, fpdecimal.Zero, fpdecimal.Zero)
	}

	done.Processed = processedQty

	// For market orders, we don't keep any remaining quantity in the book
	if processedQty.GreaterThan(fpdecimal.Zero) {
		done.Left = fpdecimal.Zero
	} else {
		done.Left = remainingQty
	}

	// Delete the market order
	ob.backend.DeleteOrder(marketOrder.ID())

	// Update last trade price and check stop orders if any trades occurred
	if done.Processed.GreaterThan(fpdecimal.Zero) && len(done.Trades) > 1 {
		// Trigger stop orders with the last trade price (already set during order processing)
		ob.checkStopOrderTrigger(ob.lastTradePrice)
	}

	// Send to Kafka
	sendToKafka(done)

	return done, nil
}

func (ob *OrderBook) processLimitOrder(limitOrder *Order) (*Done, error) {
	if limitOrder.IsMarketOrder() {
		return nil, ErrInvalidArgument
	}

	// Check for duplicate order
	if existing := ob.backend.GetOrder(limitOrder.ID()); existing != nil {
		return nil, ErrOrderExists
	}

	// Store the order first
	err := ob.backend.StoreOrder(limitOrder)
	if err != nil {
		return nil, err
	}

	done := newDone(limitOrder)
	oppositeOrders := ob.getOppositeOrders(limitOrder.Side())
	if oppositeOrders == nil {
		if limitOrder.TIF() == IOC || limitOrder.TIF() == FOK {
			done.appendCanceled(limitOrder)
			ob.backend.DeleteOrder(limitOrder.ID())
			done.Left = limitOrder.Quantity()
			done.Processed = fpdecimal.Zero
			// Add the taker to trades with zero quantity
			done.appendOrder(limitOrder, fpdecimal.Zero, limitOrder.Price())
			return done, nil
		}
		ob.backend.AppendToSide(limitOrder.Side(), limitOrder)
		// Append to done to indicate the order is now resting on the book
		done.appendOrder(limitOrder, fpdecimal.Zero, limitOrder.Price())
		return done, nil
	}

	// For FOK orders, we need to check if the order can be fully filled
	// before making any changes to the order book
	if limitOrder.TIF() == FOK {
		// First, calculate if we can fill the entire order
		executionQty := limitOrder.Quantity()
		availableQty := fpdecimal.Zero
		canFillCompletely := false

		if ordersInterface, ok := oppositeOrders.(interface {
			Prices() []fpdecimal.Decimal
			Orders(price fpdecimal.Decimal) []*Order
		}); ok {
			prices := ordersInterface.Prices()
			for _, price := range prices {
				if !ob.matchPrice(limitOrder.Side(), limitOrder.Price(), price) {
					break
				}

				orders := ordersInterface.Orders(price)
				for _, oppositeOrder := range orders {
					availableQty = availableQty.Add(oppositeOrder.Quantity())
					if availableQty.GreaterThanOrEqual(executionQty) {
						canFillCompletely = true
						break
					}
				}
				if canFillCompletely {
					break
				}
			}
		}

		if !canFillCompletely {
			// Cannot fill the FOK order completely, cancel it
			done.appendCanceled(limitOrder)
			ob.backend.DeleteOrder(limitOrder.ID())
			done.Left = executionQty
			done.Processed = fpdecimal.Zero
			done.Stored = false
			// Add the taker to trades with zero quantity
			done.appendOrder(limitOrder, fpdecimal.Zero, limitOrder.Price())
			// Send Kafka message for canceled FOK order
			sendToKafka(done)
			return done, nil
		}
		// If we can fill completely, proceed with normal execution below
	}

	executionQty := limitOrder.Quantity()
	processedQty := fpdecimal.Zero

	if ordersInterface, ok := oppositeOrders.(interface {
		Prices() []fpdecimal.Decimal
		Orders(price fpdecimal.Decimal) []*Order
	}); ok {
		prices := ordersInterface.Prices()
		for _, price := range prices {
			if !ob.matchPrice(limitOrder.Side(), limitOrder.Price(), price) {
				break
			}

			orders := ordersInterface.Orders(price)
			for _, oppositeOrder := range orders {
				availableQty := oppositeOrder.Quantity()
				matchQty := availableQty
				if executionQty.Sub(processedQty).LessThan(availableQty) {
					matchQty = executionQty.Sub(processedQty)
				}

				if matchQty.Equal(fpdecimal.Zero) {
					break
				}

				done.appendOrder(oppositeOrder, matchQty, oppositeOrder.Price()) // Record trade for the matched maker order
				processedQty = processedQty.Add(matchQty)

				// Update the opposite order's quantity
				oppositeOrder.SetQuantity(availableQty.Sub(matchQty))
				if oppositeOrder.Quantity().Equal(fpdecimal.Zero) {
					ob.backend.RemoveFromSide(oppositeOrder.Side(), oppositeOrder)
					ob.backend.DeleteOrder(oppositeOrder.ID())
				} else {
					ob.backend.UpdateOrder(oppositeOrder)
				}

				if processedQty.Equal(executionQty) {
					break
				}
			}
			if processedQty.Equal(executionQty) {
				break
			}
		}
	}

	leftQty := executionQty.Sub(processedQty)

	// This section is now only for IOC orders, as FOK orders are handled above
	if !leftQty.Equal(fpdecimal.Zero) {
		if limitOrder.TIF() == IOC {
			done.appendCanceled(limitOrder)
			ob.backend.DeleteOrder(limitOrder.ID())
			done.Left = leftQty
			done.Processed = processedQty
			done.Stored = false

			// Add the taker to trades with processed quantity
			done.appendOrder(limitOrder, processedQty, limitOrder.Price())

			// Update last trade price for stop orders if a trade occurred
			if processedQty.GreaterThan(fpdecimal.Zero) {
				ob.lastTradePrice = limitOrder.Price()
				ob.checkStopOrderTrigger(ob.lastTradePrice)
				sendToKafka(done)
			}

			return done, nil
		}
		// For GTC or other TIFs that allow resting orders:
		limitOrder.SetQuantity(leftQty)
		ob.backend.UpdateOrder(limitOrder) // Update the order with the new quantity
		ob.backend.AppendToSide(limitOrder.Side(), limitOrder)
		// Append to done to indicate the order is now resting on the book with remaining qty
		done.appendOrder(limitOrder, processedQty, limitOrder.Price())
		done.Stored = true
	} else {
		// Order fully filled
		ob.backend.DeleteOrder(limitOrder.ID())
		done.Stored = false
		// Ensure taker order is properly recorded with full quantity
		done.appendOrder(limitOrder, processedQty, limitOrder.Price())
	}

	done.Left = leftQty
	done.Processed = processedQty

	// Update last trade price for stop orders if a trade occurred
	if processedQty.GreaterThan(fpdecimal.Zero) {
		// Use the price of the last matched maker order as the trade price?
		// Or the taker's limit price? Let's use the taker's price for now.
		ob.lastTradePrice = limitOrder.Price()
		ob.checkStopOrderTrigger(ob.lastTradePrice)
		sendToKafka(done)
	}

	return done, nil
}

func (ob *OrderBook) processStopOrder(stopOrder *Order) (*Done, error) {
	if !stopOrder.IsStopOrder() {
		return nil, ErrInvalidArgument
	}

	// Check for duplicate order
	if existing := ob.backend.GetOrder(stopOrder.ID()); existing != nil {
		return nil, ErrOrderExists
	}

	done := newDone(stopOrder)

	// Store the stop order first
	err := ob.backend.StoreOrder(stopOrder)
	if err != nil {
		return nil, err
	}

	// Check if stop price is triggered
	isTriggerConditionMet := false
	if !ob.lastTradePrice.Equal(fpdecimal.Zero) {
		if (stopOrder.Side() == Buy && ob.lastTradePrice.GreaterThanOrEqual(stopOrder.StopPrice())) ||
			(stopOrder.Side() == Sell && ob.lastTradePrice.LessThanOrEqual(stopOrder.StopPrice())) {
			isTriggerConditionMet = true
		}
	}

	if isTriggerConditionMet {
		// Convert to limit order
		limitOrder, err := NewLimitOrder(
			stopOrder.ID(),
			stopOrder.Side(),
			stopOrder.Quantity(),
			stopOrder.Price(),
			stopOrder.TIF(),
			stopOrder.OCO(),
		)
		if err != nil {
			ob.backend.DeleteOrder(stopOrder.ID())
			return nil, err
		}

		// Remove from stop book before triggering (it could be not there yet, but that's fine)
		ob.backend.RemoveFromStopBook(stopOrder)

		// Update order record to reflect conversion from stop to limit
		err = ob.backend.StoreOrder(limitOrder)
		if err != nil {
			return nil, err
		}

		// Store that the order was activated
		done.appendActivated(stopOrder)

		// Process as limit order
		limitDone, err := ob.processLimitOrder(limitOrder)
		if err != nil {
			return done, err
		}

		// Copy relevant fields from limitDone to our done
		done.Trades = limitDone.Trades
		done.Processed = limitDone.Processed
		done.Left = limitDone.Left
		done.Stored = limitDone.Stored

		// Send done message to Kafka
		sendToKafka(done)
		return done, nil
	}

	// Add to stop book if not triggered
	ob.backend.AppendToStopBook(stopOrder)
	done.Stored = true

	// Record that we stored the order but didn't execute any quantity
	done.appendOrder(stopOrder, fpdecimal.Zero, stopOrder.Price())

	// Send the message about storing the stop order
	sendToKafka(done)
	return done, nil
}

// Helper function to check if a stop order should be triggered
func (ob *OrderBook) checkStopOrderTrigger(lastPrice fpdecimal.Decimal) {
	// Update the last trade price
	ob.lastTradePrice = lastPrice
	stopBook := ob.backend.GetStopBook()

	// First try the BuyOrders/SellOrders interface
	if stopBookInterface, ok := stopBook.(interface {
		BuyOrders() []*Order
		SellOrders() []*Order
	}); ok {
		// Process all buy stop orders
		buyStops := stopBookInterface.BuyOrders()
		for _, order := range buyStops {
			if lastPrice.GreaterThanOrEqual(order.StopPrice()) {
				ob.triggerStopOrder(order)
			}
		}

		// Process all sell stop orders
		sellStops := stopBookInterface.SellOrders()
		for _, order := range sellStops {
			if lastPrice.LessThanOrEqual(order.StopPrice()) {
				ob.triggerStopOrder(order)
			}
		}
		return // Successfully processed using the side-based interface
	}

	// Fallback to using the price-based interface
	if stopBookInterface, ok := stopBook.(interface {
		Orders(price fpdecimal.Decimal) []*Order
		Prices() []fpdecimal.Decimal
	}); ok {
		prices := stopBookInterface.Prices()
		for _, price := range prices {
			orders := stopBookInterface.Orders(price)
			for _, order := range orders {
				triggered := false
				if order.Side() == Buy && lastPrice.GreaterThanOrEqual(order.StopPrice()) {
					triggered = true
				} else if order.Side() == Sell && lastPrice.LessThanOrEqual(order.StopPrice()) {
					triggered = true
				}

				if triggered {
					ob.triggerStopOrder(order)
				}
			}
		}
	}
}

// Helper to trigger a stop order
func (ob *OrderBook) triggerStopOrder(order *Order) {
	// Remove the stop order from the stop book
	ob.backend.RemoveFromStopBook(order)

	// Convert to a limit order
	limitOrder, err := NewLimitOrder(
		order.ID(),
		order.Side(),
		order.Quantity(),
		order.Price(),
		order.TIF(),
		order.OCO(),
	)
	if err != nil {
		// Log error but continue processing
		fmt.Printf("Error converting stop order to limit order: %v\n", err)
		return
	}

	// Update the order record in the backend
	err = ob.backend.StoreOrder(limitOrder)
	if err != nil {
		fmt.Printf("Error storing converted limit order: %v\n", err)
		return
	}

	// Create a done object to track the activation
	done := newDone(order)
	done.appendActivated(order)

	// Process the newly activated limit order
	limitDone, processErr := ob.processLimitOrder(limitOrder)
	if processErr != nil {
		fmt.Printf("Error processing activated limit order: %v\n", processErr)
		// Still send activation message to Kafka
		sendToKafka(done)
		return
	}

	// Merge the results from the limit order processing
	done.Trades = limitDone.Trades
	done.Left = limitDone.Left
	done.Processed = limitDone.Processed
	done.Stored = limitDone.Stored

	// Send the complete message to Kafka
	sendToKafka(done)
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
		return true
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

// Helper functions for the matching engine

// getOppositeOrders returns the orders from the opposite side of the book
func (ob *OrderBook) getOppositeOrders(side Side) interface{} {
	if side == Buy {
		return ob.backend.GetAsks() // For buy orders, get sell orders
	}
	return ob.backend.GetBids() // For sell orders, get buy orders
}

// oppositeOrder returns the opposite side
func oppositeOrder(side Side) Side {
	if side == Buy {
		return Sell
	}
	return Buy
}

// min returns the minimum of two decimals
func min(a, b fpdecimal.Decimal) fpdecimal.Decimal {
	if a.LessThan(b) {
		return a
	}
	return b
}

// matchPrice checks if the order price matches with the book price
func (ob *OrderBook) matchPrice(side Side, orderPrice, bookPrice fpdecimal.Decimal) bool {
	if side == Buy {
		return orderPrice.GreaterThanOrEqual(bookPrice)
	}
	return orderPrice.LessThanOrEqual(bookPrice)
}

// GetBids returns the bid side of the order book
func (ob *OrderBook) GetBids() interface{} {
	return ob.backend.GetBids()
}

// GetAsks returns the ask side of the order book
func (ob *OrderBook) GetAsks() interface{} {
	return ob.backend.GetAsks()
}

// Implement convertTrades function
func convertTrades(trades []TradeOrder) []messaging.Trade {
	converted := make([]messaging.Trade, len(trades))
	for i, trade := range trades {
		role := "MAKER"
		if trade.Role == TAKER {
			role = "TAKER"
		}

		// Format decimal values consistently with 3 decimal places
		formatDecimal := func(d fpdecimal.Decimal) string {
			// Ensure decimal is formatted with 3 decimal places
			val := d.String()
			parts := strings.Split(val, ".")
			if len(parts) == 1 {
				// No decimal part, add .000
				return val + ".000"
			} else if len(parts[1]) < 3 {
				// Fewer than 3 decimal places, add zeroes
				return val + strings.Repeat("0", 3-len(parts[1]))
			}
			return val
		}

		converted[i] = messaging.Trade{
			OrderID:  trade.OrderID,
			Role:     role,
			Price:    formatDecimal(trade.Price),
			Quantity: formatDecimal(trade.Quantity),
			IsQuote:  trade.IsQuote,
		}
	}
	return converted
}

// sendToKafka sends the execution result using the configured message sender.
func sendToKafka(done *Done) {
	// Convert core.Done to messaging.DoneMessage
	msg := done.ToMessagingDoneMessage() // Method is defined on Done in types.go
	if msg == nil {
		// TODO: Use proper logger
		fmt.Println("Error: Failed to convert Done to MessagingDoneMessage (nil result)")
		return
	}

	// Get sender using the factory
	sender := getMessageSender()
	err := sender.SendDoneMessage(msg)
	if err != nil {
		// TODO: Use a proper logger passed down or configured globally
		fmt.Printf("Error sending message via configured sender: %v\n", err)
	}
}
