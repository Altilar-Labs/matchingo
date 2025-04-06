package core

import (
	"errors"
	"testing"
)

func TestErrors(t *testing.T) {
	// Verify that all error variables are defined
	errorTests := []struct {
		name string
		err  error
		msg  string
	}{
		{"ErrInvalidQuantity", ErrInvalidQuantity, "invalid quantity"},
		{"ErrInvalidPrice", ErrInvalidPrice, "invalid price"},
		{"ErrInvalidArgument", ErrInvalidArgument, "invalid argument"},
		{"ErrInvalidTif", ErrInvalidTif, "invalid TIF"},
		{"ErrOrderExists", ErrOrderExists, "order exists"},
		{"ErrNonexistentOrder", ErrNonexistentOrder, "nonexistent order"},
		{"ErrInsufficientQuantity", ErrInsufficientQuantity, "insufficient quantity"},
	}

	for _, tt := range errorTests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err == nil {
				t.Errorf("Error %s is nil", tt.name)
			}

			if tt.err.Error() != tt.msg {
				t.Errorf("Expected error message %q, got %q", tt.msg, tt.err.Error())
			}

			// Check if the error matches itself using errors.Is
			if !errors.Is(tt.err, tt.err) {
				t.Errorf("Error %s does not match itself with errors.Is", tt.name)
			}
		})
	}
}
