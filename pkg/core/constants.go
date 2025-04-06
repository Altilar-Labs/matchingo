package core

import "errors"

// Errors
var (
	ErrInvalidQuantity      = errors.New("invalid quantity")
	ErrInvalidPrice         = errors.New("invalid price")
	ErrInvalidArgument      = errors.New("invalid argument")
	ErrInvalidTif           = errors.New("invalid TIF")
	ErrOrderExists          = errors.New("order exists")
	ErrNonexistentOrder     = errors.New("nonexistent order")
	ErrInsufficientQuantity = errors.New("insufficient quantity")
)
