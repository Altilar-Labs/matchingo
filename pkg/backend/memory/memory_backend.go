package memory

import (
	"fmt"
	"strings"
	"sync"

	"github.com/erain9/matchingo/pkg/core"
	"github.com/nikolaydubina/fpdecimal"
)

// OrderQueue represents a price level in the order book
type OrderQueue struct {
	orders    map[string]*core.Order
	priceStr  string
	priceDecm fpdecimal.Decimal
	next      *OrderQueue
	prev      *OrderQueue
}

// NewOrderQueue creates a new OrderQueue with the given price
func NewOrderQueue(price fpdecimal.Decimal) *OrderQueue {
	return &OrderQueue{
		orders:    make(map[string]*core.Order),
		priceStr:  price.String(),
		priceDecm: price,
	}
}

// OrderSide represents one side (bid/ask) of the order book
type OrderSide struct {
	head    *OrderQueue
	tail    *OrderQueue
	orderID map[string]*OrderQueue
}

// String implements fmt.Stringer interface
func (os *OrderSide) String() string {
	sb := strings.Builder{}
	current := os.head

	for current != nil {
		orderCount := len(current.orders)

		sb.WriteString(fmt.Sprintf("\n%s -> orders: %d", current.priceStr, orderCount))
		current = current.next
	}

	return sb.String()
}

// Prices returns all prices in the order side
func (os *OrderSide) Prices() []fpdecimal.Decimal {
	prices := make([]fpdecimal.Decimal, 0)
	current := os.head

	for current != nil {
		prices = append(prices, current.priceDecm)
		current = current.next
	}

	return prices
}

// Orders returns all orders at a given price level
func (os *OrderSide) Orders(price fpdecimal.Decimal) []*core.Order {
	priceStr := price.String()
	queue, exists := os.orderID[priceStr]
	if !exists {
		return []*core.Order{}
	}

	orders := make([]*core.Order, 0, len(queue.orders))
	for _, order := range queue.orders {
		orders = append(orders, order)
	}

	return orders
}

// StopBook stores stop orders
type StopBook struct {
	buy  *OrderSide
	sell *OrderSide
}

// String implements fmt.Stringer interface
func (sb *StopBook) String() string {
	builder := strings.Builder{}

	builder.WriteString("Buy Stop Orders:")
	builder.WriteString(sb.buy.String())
	builder.WriteString("\n")

	builder.WriteString("Sell Stop Orders:")
	builder.WriteString(sb.sell.String())

	return builder.String()
}

// MemoryBackend implements OrderBookBackend interface with in-memory storage
type MemoryBackend struct {
	sync.RWMutex
	orders     map[string]*core.Order
	bids       *OrderSide
	asks       *OrderSide
	stopBook   *StopBook
	ocoMapping map[string]string
}

// NewMemoryBackend creates new instance of MemoryBackend
func NewMemoryBackend() *MemoryBackend {
	return &MemoryBackend{
		orders: make(map[string]*core.Order),
		bids: &OrderSide{
			orderID: make(map[string]*OrderQueue),
		},
		asks: &OrderSide{
			orderID: make(map[string]*OrderQueue),
		},
		stopBook: &StopBook{
			buy: &OrderSide{
				orderID: make(map[string]*OrderQueue),
			},
			sell: &OrderSide{
				orderID: make(map[string]*OrderQueue),
			},
		},
		ocoMapping: make(map[string]string),
	}
}

// GetOrder retrieves an order by ID
func (b *MemoryBackend) GetOrder(orderID string) *core.Order {
	b.RLock()
	defer b.RUnlock()
	return b.orders[orderID]
}

// StoreOrder stores an order
func (b *MemoryBackend) StoreOrder(order *core.Order) error {
	b.Lock()
	defer b.Unlock()

	if _, exists := b.orders[order.ID()]; exists {
		return core.ErrOrderExists
	}

	b.orders[order.ID()] = order

	// Store OCO mapping if exists
	if oco := order.OCO(); oco != "" {
		b.ocoMapping[order.ID()] = oco
		b.ocoMapping[oco] = order.ID()
	}

	return nil
}

// UpdateOrder updates an existing order
func (b *MemoryBackend) UpdateOrder(order *core.Order) error {
	b.Lock()
	defer b.Unlock()

	if _, exists := b.orders[order.ID()]; !exists {
		return core.ErrNonexistentOrder
	}

	b.orders[order.ID()] = order
	return nil
}

// DeleteOrder deletes an order
func (b *MemoryBackend) DeleteOrder(orderID string) {
	b.Lock()
	defer b.Unlock()

	order := b.orders[orderID]
	if order == nil {
		return
	}

	// Clean up OCO references
	if oco := order.OCO(); oco != "" {
		delete(b.ocoMapping, oco)
		delete(b.ocoMapping, orderID)
	}

	delete(b.orders, orderID)
}

// AppendToSide adds an order to the specified side
func (b *MemoryBackend) AppendToSide(side core.Side, order *core.Order) {
	if order.IsMarketOrder() {
		return
	}

	var orderSide *OrderSide
	if side == core.Buy {
		orderSide = b.bids
	} else {
		orderSide = b.asks
	}

	b.Lock()
	defer b.Unlock()

	price := order.Price()
	priceStr := price.String()

	if q, ok := orderSide.orderID[priceStr]; ok {
		// Price level exists, add order to queue
		q.orders[order.ID()] = order
		return
	}

	// Create new price level
	newQueue := NewOrderQueue(price)
	newQueue.orders[order.ID()] = order
	orderSide.orderID[priceStr] = newQueue

	// Add to linked list
	if orderSide.head == nil {
		// Empty list
		orderSide.head = newQueue
		orderSide.tail = newQueue
		return
	}

	// Find position in ordered list
	if side == core.Buy {
		// Buy side: highest price first
		if price.GreaterThan(orderSide.head.priceDecm) {
			// Insert at head
			newQueue.next = orderSide.head
			orderSide.head.prev = newQueue
			orderSide.head = newQueue
		} else if price.LessThanOrEqual(orderSide.tail.priceDecm) {
			// Insert at tail
			newQueue.prev = orderSide.tail
			orderSide.tail.next = newQueue
			orderSide.tail = newQueue
		} else {
			// Insert in middle
			current := orderSide.head
			for current != nil && price.LessThan(current.priceDecm) {
				current = current.next
			}
			newQueue.next = current
			newQueue.prev = current.prev
			current.prev.next = newQueue
			current.prev = newQueue
		}
	} else {
		// Sell side: lowest price first
		if price.LessThan(orderSide.head.priceDecm) {
			// Insert at head
			newQueue.next = orderSide.head
			orderSide.head.prev = newQueue
			orderSide.head = newQueue
		} else if price.GreaterThanOrEqual(orderSide.tail.priceDecm) {
			// Insert at tail
			newQueue.prev = orderSide.tail
			orderSide.tail.next = newQueue
			orderSide.tail = newQueue
		} else {
			// Insert in middle
			current := orderSide.head
			for current != nil && price.GreaterThan(current.priceDecm) {
				current = current.next
			}
			newQueue.next = current
			newQueue.prev = current.prev
			current.prev.next = newQueue
			current.prev = newQueue
		}
	}
}

// RemoveFromSide removes an order from the specified side
func (b *MemoryBackend) RemoveFromSide(side core.Side, order *core.Order) bool {
	if order.IsMarketOrder() {
		return false
	}

	var orderSide *OrderSide
	if side == core.Buy {
		orderSide = b.bids
	} else {
		orderSide = b.asks
	}

	b.Lock()
	defer b.Unlock()

	priceStr := order.Price().String()
	queue, ok := orderSide.orderID[priceStr]
	if !ok {
		return false
	}

	delete(queue.orders, order.ID())

	// If queue is empty, remove it
	if len(queue.orders) == 0 {
		delete(orderSide.orderID, priceStr)

		// Update linked list
		if queue.prev != nil {
			queue.prev.next = queue.next
		} else {
			orderSide.head = queue.next
		}

		if queue.next != nil {
			queue.next.prev = queue.prev
		} else {
			orderSide.tail = queue.prev
		}
	}

	return true
}

// AppendToStopBook adds a stop order to the stop book
func (b *MemoryBackend) AppendToStopBook(order *core.Order) {
	if !order.IsStopOrder() {
		return
	}

	var stopSide *OrderSide
	if order.Side() == core.Buy {
		stopSide = b.stopBook.buy
	} else {
		stopSide = b.stopBook.sell
	}

	b.Lock()
	defer b.Unlock()

	price := order.StopPrice()
	priceStr := price.String()

	if q, ok := stopSide.orderID[priceStr]; ok {
		// Price level exists, add order to queue
		q.orders[order.ID()] = order
		return
	}

	// Create new price level
	newQueue := NewOrderQueue(price)
	newQueue.orders[order.ID()] = order
	stopSide.orderID[priceStr] = newQueue

	// Add to linked list
	if stopSide.head == nil {
		// Empty list
		stopSide.head = newQueue
		stopSide.tail = newQueue
		return
	}

	// Find position in ordered list - same order as order book sides
	if order.Side() == core.Buy {
		// Buy side: lowest price first (unlike order book)
		if price.LessThan(stopSide.head.priceDecm) {
			// Insert at head
			newQueue.next = stopSide.head
			stopSide.head.prev = newQueue
			stopSide.head = newQueue
		} else if price.GreaterThanOrEqual(stopSide.tail.priceDecm) {
			// Insert at tail
			newQueue.prev = stopSide.tail
			stopSide.tail.next = newQueue
			stopSide.tail = newQueue
		} else {
			// Insert in middle
			current := stopSide.head
			for current != nil && price.GreaterThan(current.priceDecm) {
				current = current.next
			}
			newQueue.next = current
			newQueue.prev = current.prev
			current.prev.next = newQueue
			current.prev = newQueue
		}
	} else {
		// Sell side: highest price first (unlike order book)
		if price.GreaterThan(stopSide.head.priceDecm) {
			// Insert at head
			newQueue.next = stopSide.head
			stopSide.head.prev = newQueue
			stopSide.head = newQueue
		} else if price.LessThanOrEqual(stopSide.tail.priceDecm) {
			// Insert at tail
			newQueue.prev = stopSide.tail
			stopSide.tail.next = newQueue
			stopSide.tail = newQueue
		} else {
			// Insert in middle
			current := stopSide.head
			for current != nil && price.LessThan(current.priceDecm) {
				current = current.next
			}
			newQueue.next = current
			newQueue.prev = current.prev
			current.prev.next = newQueue
			current.prev = newQueue
		}
	}
}

// RemoveFromStopBook removes a stop order from the stop book
func (b *MemoryBackend) RemoveFromStopBook(order *core.Order) bool {
	if !order.IsStopOrder() {
		return false
	}

	var stopSide *OrderSide
	if order.Side() == core.Buy {
		stopSide = b.stopBook.buy
	} else {
		stopSide = b.stopBook.sell
	}

	b.Lock()
	defer b.Unlock()

	priceStr := order.StopPrice().String()
	queue, ok := stopSide.orderID[priceStr]
	if !ok {
		return false
	}

	delete(queue.orders, order.ID())

	// If queue is empty, remove it
	if len(queue.orders) == 0 {
		delete(stopSide.orderID, priceStr)

		// Update linked list
		if queue.prev != nil {
			queue.prev.next = queue.next
		} else {
			stopSide.head = queue.next
		}

		if queue.next != nil {
			queue.next.prev = queue.prev
		} else {
			stopSide.tail = queue.prev
		}
	}

	return true
}

// CheckOCO checks and cancels any OCO (One Cancels Other) orders
func (b *MemoryBackend) CheckOCO(orderID string) string {
	b.Lock()
	defer b.Unlock()

	ocoID, exists := b.ocoMapping[orderID]
	if !exists {
		return ""
	}

	// Clean up mappings
	delete(b.ocoMapping, orderID)
	delete(b.ocoMapping, ocoID)

	return ocoID
}

// GetBids returns the bid side of the order book for iteration
func (b *MemoryBackend) GetBids() interface{} {
	return b.bids
}

// GetAsks returns the ask side of the order book for iteration
func (b *MemoryBackend) GetAsks() interface{} {
	return b.asks
}

// GetStopBook returns the stop book for iteration
func (b *MemoryBackend) GetStopBook() interface{} {
	return b.stopBook
}
