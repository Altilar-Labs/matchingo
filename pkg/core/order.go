package core

import (
	"encoding/json"

	"github.com/nikolaydubina/fpdecimal"
)

// Side represents buy or sell side of the order
type Side int

// Order sides
const (
	Sell Side = iota
	Buy
)

// String returns side as string
func (s Side) String() string {
	switch s {
	case Buy:
		return "BUY"
	case Sell:
		return "SELL"
	default:
		return "UNKNOWN"
	}
}

// Role represents maker or taker role
type Role string

// Order roles
const (
	MAKER Role = "MAKER"
	TAKER Role = "TAKER"
)

// OrderType represents type of the order
type OrderType string

// Order types
const (
	TypeMarket    OrderType = "MARKET"
	TypeLimit     OrderType = "LIMIT"
	TypeStopLimit OrderType = "STOP_LIMIT"
)

// TIF represents time in force parameter
type TIF string

// Order Time In Force (TIF)
const (
	GTC TIF = "GTC" // Good Till Canceled
	IOC TIF = "IOC" // Immediate Or Cancel
	FOK TIF = "FOK" // Fill Or Kill
)

// Order stores information about order
type Order struct {
	id          string
	orderType   OrderType
	side        Side
	isQuote     bool
	quantity    fpdecimal.Decimal
	originalQty fpdecimal.Decimal
	price       fpdecimal.Decimal
	canceled    bool
	role        Role
	stop        fpdecimal.Decimal
	tif         TIF
	oco         string
	userAddress string
}

// MarshalJSON implements custom JSON marshaling for Order
func (o *Order) MarshalJSON() ([]byte, error) {
	type OrderJSON struct {
		ID          string    `json:"id"`
		OrderType   OrderType `json:"orderType"`
		Side        Side      `json:"side"`
		IsQuote     bool      `json:"isQuote"`
		Quantity    string    `json:"quantity"`
		OriginalQty string    `json:"originalQty"`
		Price       string    `json:"price"`
		Canceled    bool      `json:"canceled"`
		Role        Role      `json:"role"`
		Stop        string    `json:"stop"`
		TIF         TIF       `json:"tif"`
		OCO         string    `json:"oco"`
		UserAddress string    `json:"userAddress"`
	}

	return json.Marshal(OrderJSON{
		ID:          o.id,
		OrderType:   o.orderType,
		Side:        o.side,
		IsQuote:     o.isQuote,
		Quantity:    o.quantity.String(),
		OriginalQty: o.originalQty.String(),
		Price:       o.price.String(),
		Canceled:    o.canceled,
		Role:        o.role,
		Stop:        o.stop.String(),
		TIF:         o.tif,
		OCO:         o.oco,
		UserAddress: o.userAddress,
	})
}

// UnmarshalJSON implements custom JSON unmarshaling for Order
func (o *Order) UnmarshalJSON(data []byte) error {
	type OrderJSON struct {
		ID          string    `json:"id"`
		OrderType   OrderType `json:"orderType"`
		Side        Side      `json:"side"`
		IsQuote     bool      `json:"isQuote"`
		Quantity    string    `json:"quantity"`
		OriginalQty string    `json:"originalQty"`
		Price       string    `json:"price"`
		Canceled    bool      `json:"canceled"`
		Role        Role      `json:"role"`
		Stop        string    `json:"stop"`
		TIF         TIF       `json:"tif"`
		OCO         string    `json:"oco"`
		UserAddress string    `json:"userAddress"`
	}

	var orderJSON OrderJSON
	if err := json.Unmarshal(data, &orderJSON); err != nil {
		return err
	}

	var err error

	o.id = orderJSON.ID
	o.orderType = orderJSON.OrderType
	o.side = orderJSON.Side
	o.isQuote = orderJSON.IsQuote
	o.quantity, err = fpdecimal.FromString(orderJSON.Quantity)
	if err != nil {
		o.quantity = fpdecimal.Zero
	}

	o.originalQty, err = fpdecimal.FromString(orderJSON.OriginalQty)
	if err != nil {
		o.originalQty = fpdecimal.Zero
	}

	o.price, err = fpdecimal.FromString(orderJSON.Price)
	if err != nil {
		o.price = fpdecimal.Zero
	}

	o.canceled = orderJSON.Canceled
	o.role = orderJSON.Role

	o.stop, err = fpdecimal.FromString(orderJSON.Stop)
	if err != nil {
		o.stop = fpdecimal.Zero
	}

	o.tif = orderJSON.TIF
	o.oco = orderJSON.OCO
	o.userAddress = orderJSON.UserAddress

	return nil
}

// NewMarketOrder creates new constant object Order
func NewMarketOrder(orderID string, side Side, quantity fpdecimal.Decimal, userAddress string) (*Order, error) {
	if quantity.LessThanOrEqual(fpdecimal.Zero) {
		return nil, ErrInvalidQuantity
	}

	return &Order{
		id:          orderID,
		orderType:   TypeMarket,
		side:        side,
		quantity:    quantity,
		originalQty: quantity,
		price:       fpdecimal.Zero,
		canceled:    false,
		userAddress: userAddress,
	}, nil
}

// NewMarketQuoteOrder creates new constant object Order, but quantity is in Quote mode
func NewMarketQuoteOrder(orderID string, side Side, quantity fpdecimal.Decimal, userAddress string) (*Order, error) {
	if quantity.LessThanOrEqual(fpdecimal.Zero) {
		return nil, ErrInvalidQuantity
	}

	return &Order{
		id:          orderID,
		orderType:   TypeMarket,
		side:        side,
		quantity:    quantity,
		originalQty: quantity,
		price:       fpdecimal.Zero,
		canceled:    false,
		isQuote:     true,
		userAddress: userAddress,
	}, nil
}

// NewLimitOrder creates new constant object Order
func NewLimitOrder(orderID string, side Side, quantity, price fpdecimal.Decimal, tif TIF, oco string, userAddress string) (*Order, error) {
	if quantity.LessThanOrEqual(fpdecimal.Zero) {
		return nil, ErrInvalidQuantity
	}

	if price.LessThanOrEqual(fpdecimal.Zero) {
		return nil, ErrInvalidPrice
	}

	if tif != "" && tif != GTC && tif != FOK && tif != IOC {
		return nil, ErrInvalidTif
	}

	return &Order{
		id:          orderID,
		orderType:   TypeLimit,
		side:        side,
		quantity:    quantity,
		originalQty: quantity,
		price:       price,
		canceled:    false,
		oco:         oco,
		tif:         tif,
		userAddress: userAddress,
	}, nil
}

// NewStopLimitOrder creates new constant object Order
func NewStopLimitOrder(orderID string, side Side, quantity, price, stop fpdecimal.Decimal, oco string, userAddress string) (*Order, error) {
	if quantity.LessThanOrEqual(fpdecimal.Zero) {
		return nil, ErrInvalidQuantity
	}

	if price.LessThanOrEqual(fpdecimal.Zero) || stop.LessThanOrEqual(fpdecimal.Zero) {
		return nil, ErrInvalidPrice
	}

	return &Order{
		id:          orderID,
		orderType:   TypeStopLimit,
		side:        side,
		quantity:    quantity,
		originalQty: quantity,
		price:       price,
		canceled:    false,
		stop:        stop,
		oco:         oco,
		userAddress: userAddress,
	}, nil
}

// ID returns OrderID field copy
func (o *Order) ID() string {
	return o.id
}

// Side returns side of the Order
func (o *Order) Side() Side {
	return o.side
}

// IsQuote returns isQuote field copy
func (o *Order) IsQuote() bool {
	return o.isQuote
}

// Quantity returns Quantity field copy
func (o *Order) Quantity() fpdecimal.Decimal {
	return o.quantity
}

// OriginalQty returns originalQty field copy
func (o *Order) OriginalQty() fpdecimal.Decimal {
	return o.originalQty
}

// SetQuantity set Quantity field
func (o *Order) SetQuantity(quantity fpdecimal.Decimal) {
	o.quantity = quantity
}

// DecreaseQuantity set Quantity field
func (o *Order) DecreaseQuantity(quantity fpdecimal.Decimal) {
	o.quantity = o.quantity.Sub(quantity)
}

// Price returns Price field copy
func (o *Order) Price() fpdecimal.Decimal {
	return o.price
}

// StopPrice returns Price field copy
func (o *Order) StopPrice() fpdecimal.Decimal {
	return o.stop
}

// OCO returns reference ID
func (o *Order) OCO() string {
	return o.oco
}

// TIF returns tif field
func (o *Order) TIF() TIF {
	return o.tif
}

// IsCanceled returns Canceled status
func (o *Order) IsCanceled() bool {
	return o.canceled
}

// Cancel set Canceled status
func (o *Order) Cancel() bool {
	o.canceled = true
	return o.canceled
}

// IsMarketOrder returns true if Order is MARKET
func (o *Order) IsMarketOrder() bool {
	return o.orderType == TypeMarket
}

// IsLimitOrder returns true if Order is LIMIT
func (o *Order) IsLimitOrder() bool {
	return o.orderType == TypeLimit
}

// IsStopOrder returns true if Order is STOP-LIMIT
func (o *Order) IsStopOrder() bool {
	return o.orderType == TypeStopLimit
}

// ActivateStopOrder transforms Stop-GetOrder into Order
func (o *Order) ActivateStopOrder() {
	if !o.IsStopOrder() {
		panic("GetOrder isn't Stop")
	}

	o.stop = fpdecimal.Zero
	o.orderType = TypeLimit
}

// SetMaker sets Maker role
func (o *Order) SetMaker() {
	o.role = MAKER
}

// SetTaker sets Taker role
func (o *Order) SetTaker() {
	o.role = TAKER
}

// Role returns role of Order
func (o *Order) Role() Role {
	if o.role == MAKER {
		return MAKER
	}

	return TAKER
}

// ToSimple returns TradeOrder
func (o *Order) ToSimple() *TradeOrder {
	return &TradeOrder{
		OrderID:     o.ID(),
		Role:        o.Role(),
		IsQuote:     o.IsQuote(),
		Quantity:    o.Quantity(),
		Price:       o.Price(),
		UserAddress: o.UserAddress(),
	}
}

// String implements Stringer interface
func (o *Order) String() string {
	j, _ := o.ToSimple().MarshalJSON()
	return string(j)
}

// ToLimitOrder converts a stop order to a limit order
func (o *Order) ToLimitOrder() *Order {
	return &Order{
		id:          o.id,
		orderType:   TypeLimit,
		side:        o.side,
		isQuote:     o.isQuote,
		quantity:    o.quantity,
		originalQty: o.originalQty,
		price:       o.price,
		canceled:    o.canceled,
		role:        o.role,
		tif:         o.tif,
		oco:         o.oco,
		userAddress: o.userAddress,
	}
}

// UserAddress returns the user's address
func (o *Order) UserAddress() string {
	return o.userAddress
}
