package core

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// ERC20AddressPrefix is the standard prefix for ERC20 addresses
const ERC20AddressPrefix = "0x"

// GenerateFakeERC20Address generates a fake ERC20-compatible address
// Format: 0x + 40 random hex characters
func GenerateFakeERC20Address() (string, error) {
	// Generate 20 random bytes (40 hex characters)
	bytes := make([]byte, 20)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// Convert to hex and add prefix
	address := ERC20AddressPrefix + hex.EncodeToString(bytes)
	return address, nil
}

// IsERC20Address checks if an address is a valid ERC20 address
func IsERC20Address(address string) bool {
	return len(address) == 42 && address[:2] == ERC20AddressPrefix
}
