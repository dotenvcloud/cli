package crypto

import (
	"encoding/base64"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestVector represents a test case for cross-platform compatibility
type TestVector struct {
	Name       string
	Key        string // Base64 encoded
	IV         string // Base64 encoded
	Plaintext  string
	Ciphertext string // Expected base64 output
}

// getTestVectors returns test vectors that should match PHP/JS implementations
func getTestVectors() []TestVector {
	return []TestVector{
		{
			Name:      "Simple ASCII",
			Key:       "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=", // 32 zeros
			IV:        "AAAAAAAAAAAAAAAA",                             // 12 zeros
			Plaintext: "Hello, World!",
			// Note: These are placeholder values - actual values should come from PHP/JS
			Ciphertext: "", // Will be filled with actual PHP/JS output
		},
		{
			Name:       "Empty String",
			Key:        "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
			IV:         "AAAAAAAAAAAAAAAA",
			Plaintext:  "",
			Ciphertext: "",
		},
		{
			Name:       "Unicode",
			Key:        "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
			IV:         "AAAAAAAAAAAAAAAA",
			Plaintext:  "Hello 世界 🌍",
			Ciphertext: "",
		},
	}
}

func TestCompatibility_EncryptWithKnownIV(t *testing.T) {
	encryptor := NewGCMEncryptor()

	// Test that we can encrypt with a specific IV
	key := make([]byte, 32)
	// Make key non-zero to avoid weak key detection
	for i := range key {
		key[i] = byte(i)
	}
	iv := make([]byte, 12)
	plaintext := "Test message"

	ciphertext, err := encryptor.EncryptWithIV([]byte(plaintext), key, iv)
	require.NoError(t, err)

	// Verify the structure
	decoded, err := base64.StdEncoding.DecodeString(ciphertext)
	require.NoError(t, err)

	// Should start with the IV
	assert.Equal(t, iv, decoded[:12])

	// Should be able to decrypt
	decrypted, err := encryptor.Decrypt(ciphertext, key)
	require.NoError(t, err)
	assert.Equal(t, plaintext, string(decrypted))
}

func TestCompatibility_Format(t *testing.T) {
	// Test that our format matches the expected structure
	encryptor := NewGCMEncryptor()
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	plaintext := "Test data for format verification"

	// Encrypt
	ciphertext, err := encryptor.Encrypt([]byte(plaintext), key)
	require.NoError(t, err)

	// Decode and verify structure
	decoded, err := base64.StdEncoding.DecodeString(ciphertext)
	require.NoError(t, err)

	// Check length: IV (12) + ciphertext + tag (16)
	assert.GreaterOrEqual(t, len(decoded), NonceSize+TagSize)

	// Extract components
	iv := decoded[:NonceSize]
	assert.Len(t, iv, NonceSize)

	// The remaining part contains ciphertext + tag
	ciphertextAndTag := decoded[NonceSize:]
	assert.GreaterOrEqual(t, len(ciphertextAndTag), TagSize)
}

// TestPHPCompatibilityVector tests a specific vector from PHP
// To use this test, generate the ciphertext using the PHP script below
func TestPHPCompatibilityVector(t *testing.T) {
	t.Skip("Enable this test after generating PHP test vectors")

	/*
		PHP script to generate test vector:
		<?php
		$key = str_repeat("\x00", 32); // 32 zeros
		$iv = str_repeat("\x00", 12);  // 12 zeros
		$plaintext = "Hello, World!";
		$tag = '';

		$ciphertext = openssl_encrypt(
			$plaintext,
			'aes-256-gcm',
			$key,
			OPENSSL_RAW_DATA,
			$iv,
			$tag
		);

		$result = base64_encode($iv . $ciphertext . $tag);
		echo "Ciphertext: " . $result . "\n";
		?>
	*/

	encryptor := NewGCMEncryptor()
	key := make([]byte, 32)  // All zeros
	expectedCiphertext := "" // Fill with PHP output

	// Decrypt PHP-generated ciphertext
	plaintext, err := encryptor.Decrypt(expectedCiphertext, key)
	require.NoError(t, err)
	assert.Equal(t, "Hello, World!", string(plaintext))
}

// TestJSCompatibilityVector tests a specific vector from JavaScript
// To use this test, generate the ciphertext using the JS script below
func TestJSCompatibilityVector(t *testing.T) {
	t.Skip("Enable this test after generating JS test vectors")

	/*
		JavaScript script to generate test vector:
		const crypto = require('crypto');

		const key = Buffer.alloc(32); // 32 zeros
		const iv = Buffer.alloc(12);  // 12 zeros
		const plaintext = 'Hello, World!';

		const cipher = crypto.createCipheriv('aes-256-gcm', key, iv);
		let encrypted = cipher.update(plaintext, 'utf8');
		encrypted = Buffer.concat([encrypted, cipher.final()]);
		const tag = cipher.getAuthTag();

		const result = Buffer.concat([iv, encrypted, tag]);
		console.log('Ciphertext:', result.toString('base64'));
	*/

	encryptor := NewGCMEncryptor()
	key := make([]byte, 32)  // All zeros
	expectedCiphertext := "" // Fill with JS output

	// Decrypt JS-generated ciphertext
	plaintext, err := encryptor.Decrypt(expectedCiphertext, key)
	require.NoError(t, err)
	assert.Equal(t, "Hello, World!", string(plaintext))
}

// TestCrossCompatibility tests that data encrypted by our implementation
// can be decrypted by our implementation (sanity check)
func TestCrossCompatibility(t *testing.T) {
	encryptor := NewGCMEncryptor()

	// Use the same test vectors format
	testCases := []struct {
		plaintext string
		keySize   int
	}{
		{"Hello, World!", 32},
		{"", 32},
		{"Special chars: !@#$%^&*()", 32},
		{"Unicode: 你好世界 🌍", 32},
		{"Multi-line\ntext\nhere", 32},
	}

	for _, tc := range testCases {
		t.Run(tc.plaintext, func(t *testing.T) {
			// Generate key
			key := make([]byte, tc.keySize)
			for i := range key {
				key[i] = byte(i % 256)
			}

			// Encrypt
			ciphertext, err := encryptor.Encrypt([]byte(tc.plaintext), key)
			require.NoError(t, err)

			// Decrypt
			decrypted, err := encryptor.Decrypt(ciphertext, key)
			require.NoError(t, err)
			assert.Equal(t, tc.plaintext, string(decrypted))
		})
	}
}

// BenchmarkCompatibility benchmarks encryption/decryption with various sizes
func BenchmarkCompatibility(b *testing.B) {
	encryptor := NewGCMEncryptor()
	key := make([]byte, 32)

	sizes := []int{16, 64, 256, 1024, 4096}

	for _, size := range sizes {
		plaintext := make([]byte, size)

		b.Run(fmt.Sprintf("Encrypt_%d", size), func(b *testing.B) {
			b.SetBytes(int64(size))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := encryptor.Encrypt(plaintext, key)
				if err != nil {
					b.Fatal(err)
				}
			}
		})

		// Prepare ciphertext for decryption benchmark
		ciphertext, _ := encryptor.Encrypt(plaintext, key)

		b.Run(fmt.Sprintf("Decrypt_%d", size), func(b *testing.B) {
			b.SetBytes(int64(size))
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := encryptor.Decrypt(ciphertext, key)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
