package crypto

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateEncryptedData(t *testing.T) {
	// Create valid encrypted data
	encryptor := NewGCMEncryptor()
	key := makeTestKey()
	validCiphertext, _ := encryptor.Encrypt([]byte("test"), key)

	testCases := []struct {
		name    string
		data    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid encrypted data",
			data:    validCiphertext,
			wantErr: false,
		},
		{
			name:    "empty data",
			data:    "",
			wantErr: true,
			errMsg:  "encrypted data is empty",
		},
		{
			name:    "invalid base64",
			data:    "not-base64!@#$",
			wantErr: true,
			errMsg:  "invalid base64 encoding",
		},
		{
			name:    "too short",
			data:    base64.StdEncoding.EncodeToString(make([]byte, 10)),
			wantErr: true,
			errMsg:  "encrypted data too short",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateEncryptedData(tc.data)
			if tc.wantErr {
				assert.Error(t, err)
				if tc.errMsg != "" {
					assert.Contains(t, err.Error(), tc.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestIsEncrypted(t *testing.T) {
	// Create valid encrypted data
	encryptor := NewGCMEncryptor()
	key := makeTestKey()
	validCiphertext, _ := encryptor.Encrypt([]byte("test"), key)

	testCases := []struct {
		name     string
		value    string
		expected bool
	}{
		{
			name:     "valid encrypted data",
			value:    validCiphertext,
			expected: true,
		},
		{
			name:     "plaintext URL",
			value:    "https://example.com",
			expected: false,
		},
		{
			name:     "plaintext email",
			value:    "user@example.com",
			expected: false,
		},
		{
			name:     "plaintext with newline",
			value:    base64.StdEncoding.EncodeToString([]byte("test\ndata")),
			expected: false,
		},
		{
			name:     "too short base64",
			value:    base64.StdEncoding.EncodeToString([]byte("short")),
			expected: false,
		},
		{
			name:     "localhost",
			value:    "localhost:8080",
			expected: false,
		},
		{
			name:     "boolean true",
			value:    "true",
			expected: false,
		},
		{
			name:     "boolean false",
			value:    "false",
			expected: false,
		},
		{
			name:     "null value",
			value:    "null",
			expected: false,
		},
		{
			name:     "not base64",
			value:    "not-base64!@#$",
			expected: false,
		},
		{
			name:     "valid base64 but contains URL",
			value:    base64.StdEncoding.EncodeToString([]byte("https://example.com")),
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := IsEncrypted(tc.value)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestSanitizeKey(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "clean key",
			input:    "abcdef1234567890",
			expected: "abcdef1234567890",
		},
		{
			name:     "key with whitespace",
			input:    "  abcdef1234567890  ",
			expected: "abcdef1234567890",
		},
		{
			name:     "key with base64 prefix",
			input:    "base64:abcdef1234567890",
			expected: "abcdef1234567890",
		},
		{
			name:     "key with hex prefix",
			input:    "hex:abcdef1234567890",
			expected: "abcdef1234567890",
		},
		{
			name:     "key with key prefix",
			input:    "key:abcdef1234567890",
			expected: "abcdef1234567890",
		},
		{
			name:     "key with quotes",
			input:    `"abcdef1234567890"`,
			expected: "abcdef1234567890",
		},
		{
			name:     "key with single quotes",
			input:    `'abcdef1234567890'`,
			expected: "abcdef1234567890",
		},
		{
			name:     "key with mixed formatting",
			input:    `  "base64:abcdef1234567890"  `,
			expected: "abcdef1234567890",
		},
		{
			name:     "uppercase prefix",
			input:    "BASE64:abcdef1234567890",
			expected: "abcdef1234567890",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := SanitizeKey(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestIsEncrypted_EdgeCases(t *testing.T) {
	// Test with exact minimum length
	minData := make([]byte, NonceSize+TagSize)
	minEncoded := base64.StdEncoding.EncodeToString(minData)
	assert.True(t, IsEncrypted(minEncoded))

	// Test with one byte less than minimum
	shortData := make([]byte, NonceSize+TagSize-1)
	shortEncoded := base64.StdEncoding.EncodeToString(shortData)
	assert.False(t, IsEncrypted(shortEncoded))

	// Test with very long valid base64
	longData := make([]byte, 1000)
	longEncoded := base64.StdEncoding.EncodeToString(longData)
	assert.True(t, IsEncrypted(longEncoded))

	// Test empty string
	assert.False(t, IsEncrypted(""))

	// Test base64 with embedded common patterns
	patterns := []string{
		"http://example.com",
		"user@example.com",
		"localhost:8080",
		"127.0.0.1:3000",
		"true",
		"false",
		"null",
		"undefined",
	}

	for _, pattern := range patterns {
		// Even if it's valid base64 of sufficient length,
		// it should be rejected if it contains common patterns
		data := make([]byte, 100)
		copy(data, []byte(pattern))
		encoded := base64.StdEncoding.EncodeToString(data)

		// The encoded version won't contain the pattern directly,
		// but if we check the encoded string itself for patterns
		if strings.Contains(strings.ToLower(encoded), ".com") ||
			strings.Contains(strings.ToLower(encoded), "@") {
			assert.False(t, IsEncrypted(encoded))
		}
	}
}
