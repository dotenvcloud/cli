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

// ParseKey parses a key from various formats
func ParseKey(keyStr string) ([]byte, error) {
	// Try base64 first
	if key, err := base64.StdEncoding.DecodeString(keyStr); err == nil {
		if len(key) == RequiredKeySize {
			return key, nil
		}
	}

	// Try hex - must be 64 characters for 32 bytes
	if len(keyStr) == RequiredKeySize*2 {
		if key, err := hex.DecodeString(keyStr); err == nil {
			if len(key) == RequiredKeySize {
				return key, nil
			}
		}
	}

	// If it's exactly 32 bytes, use as-is
	if len(keyStr) == RequiredKeySize {
		return []byte(keyStr), nil
	}

	return nil, fmt.Errorf("invalid key format or length")
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
