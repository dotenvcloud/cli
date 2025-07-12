package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
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

// Encrypt encrypts plaintext using AES-256-GCM
// Returns base64 encoded string in format: base64(IV || ciphertext || tag)
func (e *GCMEncryptor) Encrypt(plaintext []byte, key []byte) (string, error) {
	// Validate key
	if err := ValidateKey(key); err != nil {
		return "", err
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

	// Create nonce (IV)
	nonce := make([]byte, NonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt and authenticate
	// GCM Seal appends the authentication tag to the ciphertext
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	// Format: IV || ciphertext || tag
	// Since Seal appends the tag, we have: ciphertext = encrypted_data || tag
	result := make([]byte, len(nonce)+len(ciphertext))
	copy(result, nonce)
	copy(result[len(nonce):], ciphertext)

	// Encode to base64
	return base64.StdEncoding.EncodeToString(result), nil
}

// Decrypt decrypts ciphertext using AES-256-GCM
// Expects base64 encoded string in format: base64(IV || ciphertext || tag)
func (e *GCMEncryptor) Decrypt(ciphertext string, key []byte) ([]byte, error) {
	// Validate key
	if err := ValidateKey(key); err != nil {
		return nil, err
	}

	// Apply key padding to ensure key is exactly 32 bytes
	paddedKey := padKey(key)

	// Decode from base64
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64: %w", err)
	}

	// Check minimum length (nonce + tag at least)
	if len(data) < NonceSize+TagSize {
		return nil, fmt.Errorf("ciphertext too short: minimum %d bytes required", NonceSize+TagSize)
	}

	// Extract components
	nonce := data[:NonceSize]
	ciphertextWithTag := data[NonceSize:]

	// Create cipher block
	block, err := aes.NewCipher(paddedKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Decrypt and verify
	plaintext, err := gcm.Open(nil, nonce, ciphertextWithTag, nil)
	if err != nil {
		return nil, fmt.Errorf("decryption failed: %w", err)
	}

	return plaintext, nil
}

// EncryptWithIV encrypts with a specific IV (for testing compatibility)
func (e *GCMEncryptor) EncryptWithIV(plaintext []byte, key []byte, iv []byte) (string, error) {
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
	if len(s) > 0 {
		return []byte(s), nil
	}

	return nil, fmt.Errorf("invalid key: empty string")
}

// DeriveKeyFromPassword derives a key from a password using PBKDF2
func DeriveKeyFromPassword(password string, salt []byte) ([]byte, error) {
	// This is now a wrapper around the key package implementation
	// to maintain backward compatibility
	return nil, fmt.Errorf("deprecated: use github.com/dotenv/cli/internal/crypto/key.DeriveKey instead")
}
