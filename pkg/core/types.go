package core

import (
	"encoding/json"
	"strings"

	"github.com/erain9/matchingo/pkg/messaging"
	"github.com/nikolaydubina/fpdecimal"
)

// TradeOrder structure
type TradeOrder struct {
	OrderID  string
	Role     Role
	Price    fpdecimal.Decimal
	IsQuote  bool
	Quantity fpdecimal.Decimal
}

// MarshalJSON implements Marshaler interface
func (t *TradeOrder) MarshalJSON() ([]byte, error) {
	customStruct := struct {
		OrderID  string `json:"orderID"`
		Role     Role   `json:"role"`
		IsQuote  bool   `json:"isQuote"`
		Price    string `json:"price"`
		Quantity string `json:"quantity"`
	}{
		OrderID:  t.OrderID,
		Role:     t.Role,
		IsQuote:  t.IsQuote,
		Price:    t.Price.String(),
		Quantity: t.Quantity.String(),
	}
	return json.Marshal(customStruct)
}

// Done contains information about the order execution result
type Done struct {
	// Initial order processed
	Order *Order
	// Original quantity of the order
	Quantity fpdecimal.Decimal
	// Trades executed, including the taker order if applicable
	Trades []TradeOrder
	// Orders canceled, e.g., due to TIF or OCO
	Canceled []*Order
	// Activations, e.g., stop orders converted to limit
	Activated []*Order
	// Remaining quantity left for the initial order
	Left fpdecimal.Decimal
	// Total quantity processed for the initial order
	Processed fpdecimal.Decimal
	// Whether the order was stored in the book (e.g., partial fill GTC)
	Stored bool
}

// newDone creates a new Done object for the given order
func newDone(order *Order) *Done {
	return &Done{
		Order:     order,
		Quantity:  order.OriginalQty(),
		Trades:    make([]TradeOrder, 0),
		Canceled:  make([]*Order, 0),
		Activated: make([]*Order, 0),
		Left:      fpdecimal.Zero,
		Processed: fpdecimal.Zero,
		Stored:    false,
	}
}

// GetTradeOrder returns TradeOrder by id
func (d *Done) GetTradeOrder(id string) *TradeOrder {
	for _, t := range d.Trades {
		if t.OrderID == id {
			return &t
		}
	}
	return nil
}

// appendOrder adds a trade to the Done object
func (d *Done) appendOrder(order *Order, quantity, price fpdecimal.Decimal) {
	trade := newTradeOrder(order, quantity, price)

	if d.Order.ID() == order.ID() {
		// This is the taker order being added
		// Add it to the beginning of the trades list if it's not already there
		if len(d.Trades) == 0 || d.Trades[0].OrderID != d.Order.ID() {
			d.Trades = append([]TradeOrder{trade}, d.Trades...)
		} else {
			// Update the existing taker entry
			d.Trades[0] = trade
		}

		// Set processed and left quantities directly
		d.Processed = quantity
		d.Left = d.Quantity.Sub(d.Processed)
	} else {
		// This is a maker order being added (a match)
		d.Trades = append(d.Trades, trade)

		// If the taker wasn't added yet, add it first with zero quantity
		if len(d.Trades) == 0 || d.Trades[0].OrderID != d.Order.ID() {
			takerTrade := newTradeOrder(d.Order, fpdecimal.Zero, price)
			d.Trades = append([]TradeOrder{takerTrade}, d.Trades...)
		}

		// Do not accumulate the processed quantity here, as this is done
		// by the calling code that tracks the processed quantity separately

		// Only update taker's quantity in trades to match the total processed amount
		// if it was specifically provided by the caller (non-zero)
		if d.Trades[0].OrderID == d.Order.ID() && quantity.GreaterThan(fpdecimal.Zero) {
			d.Trades[0].Quantity = quantity
		}
	}
}

// tradesToSlice converts trades to a slice
func (d *Done) tradesToSlice() []TradeOrder {
	slice := make([]TradeOrder, 0, len(d.Trades))
	for _, v := range d.Trades {
		slice = append(slice, v)
	}
	return slice
}

// appendCanceled adds a canceled order to the Done object
func (d *Done) appendCanceled(order *Order) {
	d.Canceled = append(d.Canceled, order)
}

// appendActivated adds an activated order to the Done object
func (d *Done) appendActivated(order *Order) {
	d.Activated = append(d.Activated, order)
}

// SetLeftQuantity updates the left quantity and recalculates processed quantity
func (d *Done) SetLeftQuantity(quantity *fpdecimal.Decimal) {
	if quantity == nil {
		return
	}
	d.Left = *quantity
	d.Processed = d.Quantity.Sub(d.Left)

	// Update the taker order quantity to match processed amount if it exists
	for i := range d.Trades {
		if d.Trades[i].OrderID == d.Order.ID() {
			d.Trades[i].Quantity = d.Processed
			break
		}
	}
}

// ToMessagingDoneMessage converts the Done object to a messaging.DoneMessage.
func (d *Done) ToMessagingDoneMessage() *messaging.DoneMessage {
	if d == nil || d.Order == nil {
		return nil
	}

	msgTrades := convertTrades(d.Trades)

	// Convert canceled orders from []*Order to []string
	msgCanceled := make([]string, len(d.Canceled))
	for i, order := range d.Canceled {
		msgCanceled[i] = order.ID()
	}

	// Convert activated orders from []*Order to []string
	msgActivated := make([]string, len(d.Activated))
	for i, order := range d.Activated {
		msgActivated[i] = order.ID()
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

	return &messaging.DoneMessage{
		OrderID:      d.Order.ID(),
		ExecutedQty:  formatDecimal(d.Processed),
		RemainingQty: formatDecimal(d.Left),
		Trades:       msgTrades,
		Canceled:     msgCanceled,
		Activated:    msgActivated,
		Stored:       d.Stored,
		Quantity:     formatDecimal(d.Quantity),
		Processed:    formatDecimal(d.Processed),
		Left:         formatDecimal(d.Left),
	}
}

// Helper function to create trade orders
func newTradeOrder(order *Order, quantity, price fpdecimal.Decimal) TradeOrder {
	return TradeOrder{
		OrderID:  order.ID(),
		Role:     order.Role(),
		Price:    price,
		IsQuote:  order.IsQuote(),
		Quantity: quantity,
	}
}

// MarshalJSON implements json.Marshaler interface for Done
func (d *Done) MarshalJSON() ([]byte, error) {
	// Extract order IDs for canceled orders
	canceledIDs := make([]string, len(d.Canceled))
	for i, order := range d.Canceled {
		canceledIDs[i] = order.ID()
	}

	// Extract order IDs for activated orders
	activatedIDs := make([]string, len(d.Activated))
	for i, order := range d.Activated {
		activatedIDs[i] = order.ID()
	}

	return json.Marshal(struct {
		Order     *TradeOrder  `json:"order"`
		Trades    []TradeOrder `json:"trades"`
		Canceled  []string     `json:"canceled"`
		Activated []string     `json:"activated"`
		Left      string       `json:"left"`
		Processed string       `json:"processed"`
		Stored    bool         `json:"stored"`
	}{
		Order:     d.Order.ToSimple(),
		Trades:    d.tradesToSlice(),
		Canceled:  canceledIDs,
		Activated: activatedIDs,
		Left:      d.Left.String(),
		Processed: d.Processed.String(),
		Stored:    d.Stored,
	})
}
