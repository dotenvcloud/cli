package crypto

import (
	"encoding/base64"
)

// Encryptor handles encryption/decryption operations
type Encryptor interface {
	Encrypt(plaintext []byte, key []byte) (string, error)
	Decrypt(ciphertext string, key []byte) ([]byte, error)
}

// KeyManager defines the interface for key operations
type KeyManager interface {
	GenerateKey() ([]byte, error)
	ValidateKey(key []byte) error
	DeriveKey(password string, salt []byte) ([]byte, error)
}

// Default implementations
var (
	DefaultEncryptor  Encryptor
	DefaultKeyManager KeyManager
)

// Initialize sets up default implementations
func Initialize() {
	DefaultEncryptor = NewGCMEncryptor()
	DefaultKeyManager = NewDefaultKeyManager()
}

// Encrypt encrypts plaintext using the default encryptor
func Encrypt(plaintext, key []byte) (string, error) {
	if DefaultEncryptor == nil {
		Initialize()
	}
	return DefaultEncryptor.Encrypt(plaintext, key)
}

// Decrypt decrypts ciphertext using the default encryptor
func Decrypt(ciphertext string, key []byte) ([]byte, error) {
	if DefaultEncryptor == nil {
		Initialize()
	}
	return DefaultEncryptor.Decrypt(ciphertext, key)
}

// EncryptString is a convenience function for string encryption
func EncryptString(plaintext string, key []byte) (string, error) {
	return Encrypt([]byte(plaintext), key)
}

// DecryptString is a convenience function for string decryption
func DecryptString(ciphertext string, key []byte) (string, error) {
	plaintext, err := Decrypt(ciphertext, key)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// defaultKeyManager implements the KeyManager interface
type defaultKeyManager struct{}

// NewDefaultKeyManager creates a new default key manager
func NewDefaultKeyManager() KeyManager {
	return &defaultKeyManager{}
}

// GenerateKey generates a new encryption key
func (m *defaultKeyManager) GenerateKey() ([]byte, error) {
	return GenerateKey()
}

// ValidateKey validates an encryption key
func (m *defaultKeyManager) ValidateKey(key []byte) error {
	return ValidateKey(key)
}

// DeriveKey derives a key from a password
func (m *defaultKeyManager) DeriveKey(password string, salt []byte) ([]byte, error) {
	// This will be implemented in key/derivation.go
	return DeriveKeyFromPassword(password, salt)
}

// EncodeKey encodes a key to base64
func EncodeKey(key []byte) string {
	return base64.StdEncoding.EncodeToString(key)
}

// DecodeKey decodes a base64 encoded key
func DecodeKey(encoded string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(encoded)
}
