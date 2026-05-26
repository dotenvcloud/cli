//go:build compatibility
// +build compatibility

package compatibility_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dotenv/cli/internal/crypto"
	"github.com/dotenv/cli/internal/crypto/key"
)

// TestVector represents a cross-platform test case
type TestVector struct {
	Key        string `json:"key"` // Base64
	IV         string `json:"iv"`  // Base64
	Plaintext  string `json:"plaintext"`
	Ciphertext string `json:"ciphertext"` // Base64
	Tag        string `json:"tag"`        // Base64
}

// Standard test vectors that should work across all implementations
var standardTestVectors = []TestVector{
	{
		// 32-byte key (base64)
		Key: "YTY0NzgyYjU4ZjU3MjM4YWQ3MjM0ZjgzYjM0ZmEzNGQ=",
		// 12-byte IV (base64)
		IV:        "MTIzNDU2Nzg5MDEy",
		Plaintext: "Hello, World!",
		// Expected format: base64(IV + ciphertext + tag)
	},
	{
		Key:       "MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTIzNDU2Nzg5MDE=",
		IV:        "YWJjZGVmZ2hpams=",
		Plaintext: "Secret data with special chars: !@#$%^&*()",
	},
	{
		Key:       "ZmVkY2JhOTg3NjU0MzIxMGZlZGNiYTk4NzY1NDMyMTA=",
		IV:        "eHl6MTIzNDU2Nzg=",
		Plaintext: "Unicode test: Hello 世界 🌍",
	},
	{
		Key:       "MTExMTExMTExMTExMTExMTExMTExMTExMTExMTExMTE=",
		IV:        "MjIyMjIyMjIyMjIy",
		Plaintext: "", // Empty string
	},
}

func TestEncryption_GoImplementation(t *testing.T) {
	for i, vector := range standardTestVectors {
		t.Run(fmt.Sprintf("vector_%d", i), func(t *testing.T) {
			// Decode key and IV
			key, err := base64.StdEncoding.DecodeString(vector.Key)
			require.NoError(t, err)

			// Encrypt
			ciphertext, err := crypto.EncryptString(vector.Plaintext, key)
			require.NoError(t, err)

			// Decrypt to verify
			decrypted, err := crypto.DecryptString(ciphertext, key)
			require.NoError(t, err)
			assert.Equal(t, vector.Plaintext, decrypted)

			// Store the ciphertext for manual verification with other implementations
			t.Logf("Go ciphertext for vector %d: %s", i, ciphertext)
		})
	}
}

func TestEncryption_PHPCompatibility(t *testing.T) {
	// Check if PHP is available
	if _, err := exec.LookPath("php"); err != nil {
		t.Skip("PHP not available")
	}

	// Test Go encryption -> PHP decryption
	t.Run("go_encrypt_php_decrypt", func(t *testing.T) {
		// Use our crypto package to encrypt
		key, err := key.GenerateKey()
		require.NoError(t, err)

		plaintext := "Test message for PHP compatibility"
		ciphertext, err := crypto.EncryptString(plaintext, key)
		require.NoError(t, err)

		// Create PHP script that decrypts
		phpScript := fmt.Sprintf(`<?php
$key = base64_decode('%s');
$data = base64_decode('%s');

// Extract components
$iv = substr($data, 0, 12);
$ciphertext = substr($data, 12, -16);
$tag = substr($data, -16);

// Decrypt
$decrypted = openssl_decrypt(
    $ciphertext,
    'aes-256-gcm',
    $key,
    OPENSSL_RAW_DATA,
    $iv,
    $tag
);

if ($decrypted === false) {
    echo "ERROR: " . openssl_error_string();
} else {
    echo $decrypted;
}
?>`, base64.StdEncoding.EncodeToString(key), ciphertext)

		// Run PHP
		cmd := exec.Command("php", "-r", phpScript)
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "PHP error: %s", string(output))

		assert.Equal(t, plaintext, string(output))
	})

	// Test PHP encryption -> Go decryption
	t.Run("php_encrypt_go_decrypt", func(t *testing.T) {
		key, err := key.GenerateKey()
		require.NoError(t, err)

		plaintext := "Test message from PHP"

		// PHP script that encrypts
		phpScript := fmt.Sprintf(`<?php
$key = base64_decode('%s');
$plaintext = '%s';
$iv = random_bytes(12);

$ciphertext = openssl_encrypt(
    $plaintext,
    'aes-256-gcm',
    $key,
    OPENSSL_RAW_DATA,
    $iv,
    $tag
);

// Combine and encode
$result = base64_encode($iv . $ciphertext . $tag);
echo $result;
?>`, base64.StdEncoding.EncodeToString(key), plaintext)

		cmd := exec.Command("php", "-r", phpScript)
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "PHP error: %s", string(output))

		// Decrypt with Go
		encrypted := strings.TrimSpace(string(output))
		decrypted, err := crypto.DecryptString(encrypted, key)
		require.NoError(t, err)
		assert.Equal(t, plaintext, decrypted)
	})
}

func TestEncryption_NodeCompatibility(t *testing.T) {
	// Check if Node.js is available
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("Node.js not available")
	}

	// Test Go encryption -> Node decryption
	t.Run("go_encrypt_node_decrypt", func(t *testing.T) {
		key, err := key.GenerateKey()
		require.NoError(t, err)

		plaintext := "Test message for Node.js compatibility"
		ciphertext, err := crypto.EncryptString(plaintext, key)
		require.NoError(t, err)

		// Create Node.js script
		nodeScript := fmt.Sprintf(`
const crypto = require('crypto');

const key = Buffer.from('%s', 'base64');
const data = Buffer.from('%s', 'base64');

// Extract components
const iv = data.slice(0, 12);
const tag = data.slice(-16);
const ciphertext = data.slice(12, -16);

// Decrypt
const decipher = crypto.createDecipheriv('aes-256-gcm', key, iv);
decipher.setAuthTag(tag);

try {
    let decrypted = decipher.update(ciphertext);
    decrypted = Buffer.concat([decrypted, decipher.final()]);
    console.log(decrypted.toString('utf8'));
} catch (err) {
    console.error('ERROR:', err.message);
}
`, base64.StdEncoding.EncodeToString(key), ciphertext)

		cmd := exec.Command("node", "-e", nodeScript)
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "Node error: %s", string(output))

		assert.Equal(t, plaintext, strings.TrimSpace(string(output)))
	})

	// Test Node encryption -> Go decryption
	t.Run("node_encrypt_go_decrypt", func(t *testing.T) {
		key, err := key.GenerateKey()
		require.NoError(t, err)

		plaintext := "Test message from Node.js"

		// Node.js script that encrypts
		nodeScript := fmt.Sprintf(`
const crypto = require('crypto');

const key = Buffer.from('%s', 'base64');
const plaintext = '%s';
const iv = crypto.randomBytes(12);

const cipher = crypto.createCipheriv('aes-256-gcm', key, iv);
let encrypted = cipher.update(plaintext, 'utf8');
encrypted = Buffer.concat([encrypted, cipher.final()]);

const tag = cipher.getAuthTag();

// Combine and encode
const result = Buffer.concat([iv, encrypted, tag]);
console.log(result.toString('base64'));
`, base64.StdEncoding.EncodeToString(key), plaintext)

		cmd := exec.Command("node", "-e", nodeScript)
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "Node error: %s", string(output))

		// Decrypt with Go
		encrypted := strings.TrimSpace(string(output))
		decrypted, err := crypto.DecryptString(encrypted, key)
		require.NoError(t, err)
		assert.Equal(t, plaintext, decrypted)
	})
}

func TestEncryption_CrossLanguageVectors(t *testing.T) {
	// This test uses pre-computed test vectors that should work
	// across all three implementations (Go, PHP, Node.js)

	testVectors := []struct {
		name       string
		key        string // base64
		plaintext  string
		ciphertext string // base64 - this is pre-computed
	}{
		{
			name:      "simple_message",
			key:       "MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTIzNDU2Nzg5MDE=",
			plaintext: "Hello",
			// This would be a known-good ciphertext from a reference implementation
		},
	}

	for _, tv := range testVectors {
		t.Run(tv.name, func(t *testing.T) {
			if tv.ciphertext == "" {
				t.Skip("Test vector not complete")
			}

			key, err := base64.StdEncoding.DecodeString(tv.key)
			require.NoError(t, err)

			// Test decryption
			decrypted, err := crypto.DecryptString(tv.ciphertext, key)
			require.NoError(t, err)
			assert.Equal(t, tv.plaintext, decrypted)
		})
	}
}

func TestEncryption_EdgeCases(t *testing.T) {
	key, err := key.GenerateKey()
	require.NoError(t, err)

	tests := []struct {
		name      string
		plaintext string
	}{
		{"empty", ""},
		{"single_byte", "a"},
		{"unicode", "Hello 世界 🌍"},
		{"newlines", "line1\nline2\r\nline3"},
		{"special_chars", "!@#$%^&*()_+-=[]{}|;':,.<>?"},
		{"null_bytes", "before\x00after"},
		{"long_text", strings.Repeat("a", 10000)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encrypt
			ciphertext, err := crypto.EncryptString(tt.plaintext, key)
			require.NoError(t, err)

			// Decrypt
			decrypted, err := crypto.DecryptString(ciphertext, key)
			require.NoError(t, err)

			// Compare
			assert.Equal(t, tt.plaintext, decrypted)

			// Verify format
			data, err := base64.StdEncoding.DecodeString(ciphertext)
			require.NoError(t, err)

			// Should have at least IV (12) + tag (16) = 28 bytes
			assert.GreaterOrEqual(t, len(data), 28)
		})
	}
}

func TestKeyDerivation_Compatibility(t *testing.T) {
	// Test that key derivation is consistent
	password := "test-password-123"
	salt := []byte("test-salt")

	// Derive key using our implementation
	derivedKey := key.DeriveKey(password, salt, 32)
	assert.Len(t, derivedKey, 32)

	// The derived key should be deterministic
	derivedKey2 := key.DeriveKey(password, salt, 32)
	assert.Equal(t, derivedKey, derivedKey2)

	// Different password should give different key
	differentKey := key.DeriveKey("different-password", salt, 32)
	assert.NotEqual(t, derivedKey, differentKey)
}

func BenchmarkEncryption_Compatibility(b *testing.B) {
	key, _ := key.GenerateKey()
	plaintext := strings.Repeat("a", 1024) // 1KB

	b.Run("encrypt", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := crypto.EncryptString(plaintext, key)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("decrypt", func(b *testing.B) {
		ciphertext, _ := crypto.EncryptString(plaintext, key)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := crypto.DecryptString(ciphertext, key)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// GenerateTestVectors generates test vectors for manual verification
func TestGenerateTestVectors(t *testing.T) {
	t.Skip("Enable this test to generate test vectors")

	// Generate a few test vectors with known keys and IVs
	testCases := []struct {
		plaintext string
	}{
		{"Hello, World!"},
		{"Secret data"},
		{""},
		{"Unicode: 世界"},
	}

	// Fixed key for test vectors
	fixedKey := bytes.Repeat([]byte{0x01}, 32)

	for i, tc := range testCases {
		// We can't use a fixed IV with the current API,
		// but we can encrypt and provide the result
		ciphertext, err := crypto.EncryptString(tc.plaintext, fixedKey)
		require.NoError(t, err)

		vector := TestVector{
			Key:        base64.StdEncoding.EncodeToString(fixedKey),
			Plaintext:  tc.plaintext,
			Ciphertext: ciphertext,
		}

		vectorJSON, _ := json.MarshalIndent(vector, "", "  ")
		t.Logf("Test Vector %d:\n%s\n", i, string(vectorJSON))
	}
}
