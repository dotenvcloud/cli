package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"runtime"
)

// SimpleCrypto provides basic encryption for API keys
// Note: This is a simple implementation. For production,
// consider using system keychains or more secure storage.
type SimpleCrypto struct {
	key []byte
}

// NewSimpleCrypto creates a new SimpleCrypto instance
func NewSimpleCrypto() *SimpleCrypto {
	// Derive key from machine-specific data
	key := deriveKey()
	return &SimpleCrypto{key: key}
}

// Encrypt encrypts a string
func (s *SimpleCrypto) Encrypt(plaintext string) (string, error) {
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return "", err
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	// Create nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	// Encrypt
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)

	// Encode to base64
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts a string
func (s *SimpleCrypto) Decrypt(ciphertext string) (string, error) {
	// Decode from base64
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(s.key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// deriveKey creates a machine-specific encryption key
func deriveKey() []byte {
	// Combine multiple sources for key derivation
	sources := []string{
		os.Getenv("USER"),
		os.Getenv("HOME"),
		getMachineID(),
		"dotenv-cli-v1", // Salt
	}

	h := sha256.New()
	for _, s := range sources {
		h.Write([]byte(s))
	}

	return h.Sum(nil)[:32] // Use first 32 bytes for AES-256
}

// getMachineID attempts to get a machine-specific ID
func getMachineID() string {
	// Try various methods to get machine ID

	switch runtime.GOOS {
	case "darwin":
		// macOS - try to get hardware UUID
		if data, err := os.ReadFile("/Library/Preferences/SystemConfiguration/com.apple.smb.server.plist"); err == nil && len(data) > 0 {
			return fmt.Sprintf("mac-%x", sha256.Sum256(data))
		}
	case "linux":
		// Linux - use machine-id
		if data, err := os.ReadFile("/etc/machine-id"); err == nil {
			return string(data)
		}
		// Try alternative locations
		if data, err := os.ReadFile("/var/lib/dbus/machine-id"); err == nil {
			return string(data)
		}
	case "windows":
		// Windows - use COMPUTERNAME
		if computername := os.Getenv("COMPUTERNAME"); computername != "" {
			return computername
		}
	}

	// Fallback to hostname
	hostname, _ := os.Hostname()
	return hostname
}
