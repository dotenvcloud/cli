package key

import (
	"crypto/sha256"
	"fmt"

	"golang.org/x/crypto/pbkdf2"
)

const (
	// DefaultIterations for PBKDF2
	DefaultIterations = 100000
	// DefaultSaltSize in bytes
	DefaultSaltSize = 16
)

// DeriveKey derives a key from a password using PBKDF2
func DeriveKey(password string, salt []byte, iterations int) ([]byte, error) {
	if password == "" {
		return nil, fmt.Errorf("password cannot be empty")
	}

	if len(salt) < 8 {
		return nil, fmt.Errorf("salt too short: minimum 8 bytes required")
	}

	if iterations < 1000 {
		return nil, fmt.Errorf("iterations too low: minimum 1000 required")
	}

	// Derive key using PBKDF2 with SHA-256
	key := pbkdf2.Key([]byte(password), salt, iterations, RequiredKeySize, sha256.New)

	return key, nil
}

// DeriveKeyWithDefaults derives a key with default iterations
func DeriveKeyWithDefaults(password string, salt []byte) ([]byte, error) {
	return DeriveKey(password, salt, DefaultIterations)
}
