package key

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
)

const (
	// RequiredKeySize is the required key size in bytes for AES-256
	RequiredKeySize = 32
)

// GenerateKey generates a random 256-bit key
func GenerateKey() ([]byte, error) {
	key := make([]byte, RequiredKeySize)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}
	return key, nil
}

// GenerateKeyString generates a base64-encoded key
func GenerateKeyString() (string, error) {
	key, err := GenerateKey()
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(key), nil
}

// GenerateKeyHex generates a hex-encoded key
func GenerateKeyHex() (string, error) {
	key, err := GenerateKey()
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(key), nil
}

// ParseKey parses a key from various formats and applies padding
func ParseKey(keyStr string) ([]byte, error) {
	// Try base64 first
	if key, err := base64.StdEncoding.DecodeString(keyStr); err == nil && len(key) > 0 {
		return padKey(key), nil
	}

	// Try hex
	if key, err := hex.DecodeString(keyStr); err == nil && len(key) > 0 {
		return padKey(key), nil
	}

	// Otherwise use the raw string as bytes
	if len(keyStr) > 0 {
		return padKey([]byte(keyStr)), nil
	}

	return nil, fmt.Errorf("invalid key: empty string")
}

// padKey pads or truncates a key to exactly 32 bytes for AES-256
// This matches the web application's key padding behavior
func padKey(key []byte) []byte {
	if len(key) >= RequiredKeySize {
		// Key is 32 bytes or longer, truncate to 32 bytes
		return key[:RequiredKeySize]
	}

	// Key is shorter than 32 bytes, pad with '0' bytes
	padded := make([]byte, RequiredKeySize)
	copy(padded, key)
	for i := len(key); i < RequiredKeySize; i++ {
		padded[i] = '0'
	}
	return padded
}

// GenerateSalt generates a random salt for key derivation
func GenerateSalt(size int) ([]byte, error) {
	if size < 8 {
		return nil, fmt.Errorf("salt size must be at least 8 bytes")
	}

	salt := make([]byte, size)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, fmt.Errorf("failed to generate salt: %w", err)
	}
	return salt, nil
}
