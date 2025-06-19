package key

import (
	"fmt"
)

// ValidateKey validates an encryption key
func ValidateKey(key []byte) error {
	if key == nil {
		return fmt.Errorf("key is nil")
	}

	if len(key) != RequiredKeySize {
		return fmt.Errorf("invalid key size: expected %d bytes, got %d", RequiredKeySize, len(key))
	}

	// Check for weak keys (all zeros, all ones)
	allZero := true
	allOne := true
	for _, b := range key {
		if b != 0 {
			allZero = false
		}
		if b != 0xFF {
			allOne = false
		}
	}

	if allZero || allOne {
		return fmt.Errorf("weak key detected")
	}

	return nil
}

// ValidateKeyString validates a string key
func ValidateKeyString(keyStr string) error {
	key, err := ParseKey(keyStr)
	if err != nil {
		return err
	}

	return ValidateKey(key)
}

// SanitizeKey removes common formatting from keys
func SanitizeKey(key string) string {
	// Implementation will be in the main crypto package
	// to avoid circular dependencies
	return key
}
