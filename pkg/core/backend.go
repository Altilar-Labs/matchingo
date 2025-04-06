package core

// OrderBookBackend defines the interface for different backend implementations
type OrderBookBackend interface {
	// Order operations
	GetOrder(orderID string) *Order
	StoreOrder(order *Order) error
	UpdateOrder(order *Order) error
	DeleteOrder(orderID string)

	// Side operations
	AppendToSide(side Side, order *Order)
	RemoveFromSide(side Side, order *Order) bool

	// Stop book operations
	AppendToStopBook(order *Order)
	RemoveFromStopBook(order *Order) bool

	// OCO operations
	CheckOCO(orderID string) string

	// Get sides for iterating
	GetBids() interface{}
	GetAsks() interface{}
	GetStopBook() interface{}
}
