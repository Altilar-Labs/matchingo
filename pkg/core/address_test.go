package core

import (
	"testing"
)

func TestGenerateFakeERC20Address(t *testing.T) {
	// Test generating multiple addresses
	for i := 0; i < 10; i++ {
		address, err := GenerateFakeERC20Address()
		if err != nil {
			t.Fatalf("Failed to generate address: %v", err)
		}

		// Check address format
		if len(address) != 42 {
			t.Errorf("Expected address length 42, got %d", len(address))
		}

		// Check prefix
		if address[:2] != ERC20AddressPrefix {
			t.Errorf("Expected prefix %s, got %s", ERC20AddressPrefix, address[:2])
		}

		// Check if it's a valid ERC20 address
		if !IsERC20Address(address) {
			t.Errorf("Generated address %s is not recognized as an ERC20 address", address)
		}
	}
}

func TestIsERC20Address(t *testing.T) {
	tests := []struct {
		name    string
		address string
		want    bool
	}{
		{
			name:    "Valid ERC20 address",
			address: "0x1234567890123456789012345678901234567890",
			want:    true,
		},
		{
			name:    "Invalid prefix",
			address: "0MM12345678901234567890123456789012345678",
			want:    false,
		},
		{
			name:    "Too short",
			address: "0x123",
			want:    false,
		},
		{
			name:    "Too long",
			address: "0x123456789012345678901234567890123456789012",
			want:    false,
		},
		{
			name:    "Empty string",
			address: "",
			want:    false,
		},
		{
			name:    "Missing 0x prefix",
			address: "1234567890123456789012345678901234567890",
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsERC20Address(tt.address); got != tt.want {
				t.Errorf("IsERC20Address() = %v, want %v", got, tt.want)
			}
		})
	}
}
