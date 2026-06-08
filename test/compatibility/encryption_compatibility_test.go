//go:build compatibility
// +build compatibility

package compatibility_test

import (
	"encoding/base64"
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	dotenv "github.com/dotenvcloud/sdk-go"
)

// These tests guard the AES-256-GCM wire format (base64(IV[12] + ciphertext +
// tag[16])) for cross-language interop. The crypto contract lives in sdk-go
// (the Go mirror of the web app's EncryptionService = source of truth), so the
// Go side here exercises sdk-go directly rather than a CLI-local copy.

// TestVector represents a cross-platform test case.
type TestVector struct {
	Key       string `json:"key"` // Base64-encoded 32-byte key
	IV        string `json:"iv"`  // Base64 (informational; sdk-go generates its own IV)
	Plaintext string `json:"plaintext"`
}

// Standard test vectors that should work across all implementations.
var standardTestVectors = []TestVector{
	{
		// 32-byte key (base64)
		Key:       "YTY0NzgyYjU4ZjU3MjM4YWQ3MjM0ZjgzYjM0ZmEzNGQ=",
		IV:        "MTIzNDU2Nzg5MDEy",
		Plaintext: "Hello, World!",
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
			key, err := base64.StdEncoding.DecodeString(vector.Key)
			require.NoError(t, err)

			ciphertext, err := dotenv.Encrypt(vector.Plaintext, key)
			require.NoError(t, err)

			decrypted, err := dotenv.Decrypt(ciphertext, key)
			require.NoError(t, err)
			assert.Equal(t, vector.Plaintext, decrypted)

			// Surface the ciphertext for manual cross-impl verification.
			t.Logf("Go ciphertext for vector %d: %s", i, ciphertext)
		})
	}
}

func TestEncryption_PHPCompatibility(t *testing.T) {
	if _, err := exec.LookPath("php"); err != nil {
		t.Skip("PHP not available")
	}

	// Go encrypt -> PHP decrypt
	t.Run("go_encrypt_php_decrypt", func(t *testing.T) {
		key, err := dotenv.GenerateKey()
		require.NoError(t, err)

		plaintext := "Test message for PHP compatibility"
		ciphertext, err := dotenv.Encrypt(plaintext, key)
		require.NoError(t, err)

		// `php -r` expects raw code with no opening/closing tags.
		phpScript := fmt.Sprintf(`
$key = base64_decode('%s');
$data = base64_decode('%s');

$iv = substr($data, 0, 12);
$ciphertext = substr($data, 12, -16);
$tag = substr($data, -16);

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
`, base64.StdEncoding.EncodeToString(key), ciphertext)

		cmd := exec.Command("php", "-r", phpScript)
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "PHP error: %s", string(output))

		assert.Equal(t, plaintext, string(output))
	})

	// PHP encrypt -> Go decrypt
	t.Run("php_encrypt_go_decrypt", func(t *testing.T) {
		key, err := dotenv.GenerateKey()
		require.NoError(t, err)

		plaintext := "Test message from PHP"

		phpScript := fmt.Sprintf(`
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

$result = base64_encode($iv . $ciphertext . $tag);
echo $result;
`, base64.StdEncoding.EncodeToString(key), plaintext)

		cmd := exec.Command("php", "-r", phpScript)
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "PHP error: %s", string(output))

		encrypted := strings.TrimSpace(string(output))
		decrypted, err := dotenv.Decrypt(encrypted, key)
		require.NoError(t, err)
		assert.Equal(t, plaintext, decrypted)
	})
}

func TestEncryption_NodeCompatibility(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("Node.js not available")
	}

	// Go encrypt -> Node decrypt
	t.Run("go_encrypt_node_decrypt", func(t *testing.T) {
		key, err := dotenv.GenerateKey()
		require.NoError(t, err)

		plaintext := "Test message for Node.js compatibility"
		ciphertext, err := dotenv.Encrypt(plaintext, key)
		require.NoError(t, err)

		nodeScript := fmt.Sprintf(`
const crypto = require('crypto');

const key = Buffer.from('%s', 'base64');
const data = Buffer.from('%s', 'base64');

const iv = data.slice(0, 12);
const tag = data.slice(-16);
const ciphertext = data.slice(12, -16);

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

	// Node encrypt -> Go decrypt
	t.Run("node_encrypt_go_decrypt", func(t *testing.T) {
		key, err := dotenv.GenerateKey()
		require.NoError(t, err)

		plaintext := "Test message from Node.js"

		nodeScript := fmt.Sprintf(`
const crypto = require('crypto');

const key = Buffer.from('%s', 'base64');
const plaintext = '%s';
const iv = crypto.randomBytes(12);

const cipher = crypto.createCipheriv('aes-256-gcm', key, iv);
let encrypted = cipher.update(plaintext, 'utf8');
encrypted = Buffer.concat([encrypted, cipher.final()]);

const tag = cipher.getAuthTag();

const result = Buffer.concat([iv, encrypted, tag]);
console.log(result.toString('base64'));
`, base64.StdEncoding.EncodeToString(key), plaintext)

		cmd := exec.Command("node", "-e", nodeScript)
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "Node error: %s", string(output))

		encrypted := strings.TrimSpace(string(output))
		decrypted, err := dotenv.Decrypt(encrypted, key)
		require.NoError(t, err)
		assert.Equal(t, plaintext, decrypted)
	})
}

func TestEncryption_EdgeCases(t *testing.T) {
	key, err := dotenv.GenerateKey()
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
			ciphertext, err := dotenv.Encrypt(tt.plaintext, key)
			require.NoError(t, err)

			decrypted, err := dotenv.Decrypt(ciphertext, key)
			require.NoError(t, err)
			assert.Equal(t, tt.plaintext, decrypted)

			data, err := base64.StdEncoding.DecodeString(ciphertext)
			require.NoError(t, err)

			// At least IV (12) + tag (16) = 28 bytes.
			assert.GreaterOrEqual(t, len(data), 28)
		})
	}
}

func BenchmarkEncryption_Compatibility(b *testing.B) {
	key, _ := dotenv.GenerateKey()
	plaintext := strings.Repeat("a", 1024) // 1KB

	b.Run("encrypt", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if _, err := dotenv.Encrypt(plaintext, key); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("decrypt", func(b *testing.B) {
		ciphertext, _ := dotenv.Encrypt(plaintext, key)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if _, err := dotenv.Decrypt(ciphertext, key); err != nil {
				b.Fatal(err)
			}
		}
	})
}
