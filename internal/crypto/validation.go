package crypto

import (
	"encoding/base64"
	"fmt"
	"strings"
)

// ValidateEncryptedData validates encrypted data format
func ValidateEncryptedData(data string) error {
	if data == "" {
		return fmt.Errorf("encrypted data is empty")
	}

	// Check if it's valid base64
	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return fmt.Errorf("invalid base64 encoding: %w", err)
	}

	// Check minimum length (IV + tag)
	minLength := NonceSize + TagSize
	if len(decoded) < minLength {
		return fmt.Errorf("encrypted data too short: minimum %d bytes required", minLength)
	}

	return nil
}

// IsEncrypted checks if a value appears to be encrypted
func IsEncrypted(value string) bool {
	// Check if it's base64
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return false
	}

	// Check minimum length for our encryption format
	if len(decoded) < NonceSize+TagSize {
		return false
	}

	// Additional heuristics
	// - Encrypted data typically doesn't contain newlines
	// - Doesn't look like common plaintext patterns

	if strings.Contains(value, "\n") || strings.Contains(value, "\r") {
		return false
	}

	// Check for common plaintext patterns
	lowerValue := strings.ToLower(value)
	commonPatterns := []string{
		"http://", "https://", "ftp://",
		"localhost", "127.0.0.1",
		"@", ".com", ".org",
		"true", "false",
		"null", "undefined",
	}

	for _, pattern := range commonPatterns {
		if strings.Contains(lowerValue, pattern) {
			return false
		}
	}

	return true
}

// SanitizeKey removes common formatting from keys
func SanitizeKey(key string) string {
	// Remove whitespace first
	key = strings.TrimSpace(key)

	// Remove quotes early
	key = strings.Trim(key, `"'`)

	// Remove whitespace again after quote removal
	key = strings.TrimSpace(key)

	// Remove common prefixes
	prefixes := []string{
		"base64:",
		"hex:",
		"key:",
	}

	for _, prefix := range prefixes {
		if strings.HasPrefix(strings.ToLower(key), prefix) {
			key = key[len(prefix):]
			break
		}
	}

	return key
}
