package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"

	dotenv "github.com/dotenvcloud/sdk-go"
)

const (
	// KeySize is the required key size for AES-256
	KeySize = 32 // 256 bits
	// NonceSize is the IV size for GCM
	NonceSize = 12 // 96 bits
	// TagSize is the authentication tag size
	TagSize = 16 // 128 bits
)

// Package-level errors
var (
	ErrInvalidKey        = errors.New("invalid encryption key")
	ErrInvalidCiphertext = errors.New("invalid ciphertext format")
	ErrDecryptionFailed  = errors.New("decryption failed")
	ErrKeyTooShort       = errors.New("key too short")
	ErrKeyTooLong        = errors.New("key too long")
)

// GCMEncryptor implements AES-256-GCM encryption
type GCMEncryptor struct{}

// NewGCMEncryptor creates a new AES-GCM encryptor
func NewGCMEncryptor() *GCMEncryptor {
	return &GCMEncryptor{}
}

// Encrypt encrypts plaintext using AES-256-GCM.
//
// Delegates to dotenv.Encrypt so the SDK is the single source of truth for
// the wire format. Key validation is kept here because the SDK is
// intentionally permissive about weak keys.
func (e *GCMEncryptor) Encrypt(plaintext, key []byte) (string, error) {
	if err := ValidateKey(key); err != nil {
		return "", err
	}
	return dotenv.Encrypt(string(plaintext), key)
}

// Decrypt decrypts ciphertext using AES-256-GCM.
//
// Delegates to dotenv.Decrypt; see Encrypt for rationale.
func (e *GCMEncryptor) Decrypt(ciphertext string, key []byte) ([]byte, error) {
	if err := ValidateKey(key); err != nil {
		return nil, err
	}
	plaintext, err := dotenv.Decrypt(ciphertext, key)
	if err != nil {
		return nil, fmt.Errorf("decryption failed: %w", err)
	}
	return []byte(plaintext), nil
}

// EncryptWithIV encrypts with a specific IV (for testing compatibility)
func (e *GCMEncryptor) EncryptWithIV(plaintext, key, iv []byte) (string, error) {
	// Validate inputs
	if err := ValidateKey(key); err != nil {
		return "", err
	}

	if len(iv) != NonceSize {
		return "", fmt.Errorf("invalid IV size: expected %d bytes, got %d", NonceSize, len(iv))
	}

	// Apply key padding to ensure key is exactly 32 bytes
	paddedKey := padKey(key)

	// Create cipher block
	block, err := aes.NewCipher(paddedKey)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// Encrypt with provided IV
	ciphertext := gcm.Seal(nil, iv, plaintext, nil)

	// Format: IV || ciphertext || tag
	result := make([]byte, len(iv)+len(ciphertext))
	copy(result, iv)
	copy(result[len(iv):], ciphertext)

	// Encode to base64
	return base64.StdEncoding.EncodeToString(result), nil
}

// ValidateKey validates an encryption key
// Keys of any length are now accepted (they will be padded/truncated as needed)
func ValidateKey(key []byte) error {
	if key == nil {
		return ErrInvalidKey
	}

	if len(key) == 0 {
		return ErrInvalidKey
	}

	// Check for weak keys (all zeros, all ones) on the actual key length
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

// padKey pads or truncates a key to exactly 32 bytes for AES-256
// This matches the web application's key padding behavior
func padKey(key []byte) []byte {
	if len(key) >= KeySize {
		// Key is 32 bytes or longer, truncate to 32 bytes
		return key[:KeySize]
	}

	// Key is shorter than 32 bytes, pad with '0' bytes
	padded := make([]byte, KeySize)
	copy(padded, key)
	for i := len(key); i < KeySize; i++ {
		padded[i] = '0'
	}
	return padded
}

// GenerateKey generates a random 256-bit key
func GenerateKey() ([]byte, error) {
	key := make([]byte, KeySize)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}
	return key, nil
}

// KeyFromString converts a base64 or hex encoded string to a key
// Keys of any length are now accepted (they will be padded/truncated as needed)
func KeyFromString(s string) ([]byte, error) {
	// Try base64 first
	if key, err := base64.StdEncoding.DecodeString(s); err == nil && len(key) > 0 {
		return key, nil
	}

	// Otherwise use the raw string as bytes
	if s != "" {
		return []byte(s), nil
	}

	return nil, fmt.Errorf("invalid key: empty string")
}

// DeriveKeyFromPassword derives a key from a password using PBKDF2.
//
// Deprecated: use github.com/dotenvcloud/cli/internal/crypto/key.DeriveKey instead.
func DeriveKeyFromPassword(_ string, _ []byte) ([]byte, error) {
	return nil, fmt.Errorf("deprecated: use github.com/dotenvcloud/cli/internal/crypto/key.DeriveKey instead")
}
