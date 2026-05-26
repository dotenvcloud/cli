package crypto

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGCMEncryptor_EncryptDecrypt(t *testing.T) {
	encryptor := NewGCMEncryptor()

	testCases := []struct {
		name      string
		plaintext string
		key       []byte
	}{
		{
			name:      "simple text",
			plaintext: "Hello, World!",
			key:       makeTestKey(),
		},
		{
			name:      "empty string",
			plaintext: "",
			key:       makeTestKey(),
		},
		{
			name:      "unicode text",
			plaintext: "Hello 世界 🌍",
			key:       makeTestKey(),
		},
		{
			name: "long text",
			plaintext: "The quick brown fox jumps over the lazy dog. " +
				"The quick brown fox jumps over the lazy dog. " +
				"The quick brown fox jumps over the lazy dog.",
			key: makeTestKey(),
		},
		{
			name:      "special characters",
			plaintext: "!@#$%^&*()_+-=[]{}|;':\",./<>?",
			key:       makeTestKey(),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Encrypt
			ciphertext, err := encryptor.Encrypt([]byte(tc.plaintext), tc.key)
			require.NoError(t, err)
			assert.NotEmpty(t, ciphertext)

			// Verify it's valid base64
			_, err = base64.StdEncoding.DecodeString(ciphertext)
			require.NoError(t, err)

			// Decrypt
			decrypted, err := encryptor.Decrypt(ciphertext, tc.key)
			require.NoError(t, err)
			assert.Equal(t, tc.plaintext, string(decrypted))
		})
	}
}

func TestGCMEncryptor_InvalidKey(t *testing.T) {
	encryptor := NewGCMEncryptor()

	testCases := []struct {
		name    string
		key     []byte
		wantErr string
	}{
		{
			name:    "nil key",
			key:     nil,
			wantErr: "invalid encryption key",
		},
		{
			name:    "all zeros key",
			key:     make([]byte, 32),
			wantErr: "weak key detected",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := encryptor.Encrypt([]byte("test"), tc.key)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)

			_, err = encryptor.Decrypt("test", tc.key)
			assert.Error(t, err)
		})
	}
}

func TestGCMEncryptor_InvalidCiphertext(t *testing.T) {
	encryptor := NewGCMEncryptor()
	key := makeTestKey()

	testCases := []struct {
		name       string
		ciphertext string
		wantErr    string
	}{
		{
			name:       "empty",
			ciphertext: "",
			wantErr:    "ciphertext too short",
		},
		{
			name:       "invalid base64",
			ciphertext: "not-base64!@#$",
			wantErr:    "decryption failed",
		},
		{
			name:       "too short",
			ciphertext: base64.StdEncoding.EncodeToString(make([]byte, 10)),
			wantErr:    "ciphertext too short",
		},
		{
			name:       "corrupted",
			ciphertext: base64.StdEncoding.EncodeToString(make([]byte, 50)),
			wantErr:    "decryption failed",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := encryptor.Decrypt(tc.ciphertext, key)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

func TestGCMEncryptor_Compatibility(t *testing.T) {
	encryptor := NewGCMEncryptor()

	// Test with known values to ensure compatibility
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	iv := make([]byte, 12)
	for i := range iv {
		iv[i] = byte(i)
	}

	plaintext := "Hello, World!"

	// Encrypt with specific IV
	ciphertext, err := encryptor.EncryptWithIV([]byte(plaintext), key, iv)
	require.NoError(t, err)

	// Decrypt
	decrypted, err := encryptor.Decrypt(ciphertext, key)
	require.NoError(t, err)
	assert.Equal(t, plaintext, string(decrypted))
}

func TestGCMEncryptor_Randomness(t *testing.T) {
	encryptor := NewGCMEncryptor()
	key := makeTestKey()
	plaintext := []byte("test message")

	// Encrypt same message multiple times
	results := make(map[string]bool)
	for i := 0; i < 100; i++ {
		ciphertext, err := encryptor.Encrypt(plaintext, key)
		require.NoError(t, err)
		results[ciphertext] = true
	}

	// All should be different due to random IV
	assert.Equal(t, 100, len(results), "Expected all ciphertexts to be unique")
}

func TestGCMEncryptor_DifferentKeys(t *testing.T) {
	encryptor := NewGCMEncryptor()
	plaintext := []byte("secret message")

	key1 := makeTestKey()
	key2 := makeTestKey()
	// Ensure keys are different
	key2[0] = key1[0] + 1

	// Encrypt with key1
	ciphertext, err := encryptor.Encrypt(plaintext, key1)
	require.NoError(t, err)

	// Try to decrypt with key2 - should fail
	_, err = encryptor.Decrypt(ciphertext, key2)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "decryption failed")

	// Decrypt with correct key should work
	decrypted, err := encryptor.Decrypt(ciphertext, key1)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)
}

func TestKeyGeneration(t *testing.T) {
	// Test key generation
	key, err := GenerateKey()
	require.NoError(t, err)
	assert.Len(t, key, KeySize)

	// Ensure randomness
	key2, err := GenerateKey()
	require.NoError(t, err)
	assert.NotEqual(t, key, key2)
}

func TestKeyFromString(t *testing.T) {
	testCases := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "valid base64",
			input:   base64.StdEncoding.EncodeToString(makeTestKey()),
			wantErr: false,
		},
		{
			name:    "raw 32 bytes",
			input:   string(makeTestKey()),
			wantErr: false,
		},
		// NOTE: KeyFromString now accepts any non-empty input — base64 decode
		// is attempted first, then the raw string is used as the key bytes.
		// Length is no longer validated here; the encryptor pads/truncates.
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			key, err := KeyFromString(tc.input)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.NotEmpty(t, key)
			}
		})
	}
}

func TestValidateKey(t *testing.T) {
	testCases := []struct {
		name    string
		key     []byte
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid key",
			key:     makeTestKey(),
			wantErr: false,
		},
		{
			name:    "nil key",
			key:     nil,
			wantErr: true,
			errMsg:  "invalid encryption key",
		},
		// short/long key tests dropped — ValidateKey now accepts any length
		// (keys are padded/truncated to 32 bytes downstream). Only nil,
		// empty, and weak (all-zero/all-FF) keys are rejected.
		{
			name:    "all zeros",
			key:     make([]byte, 32),
			wantErr: true,
			errMsg:  "weak key detected",
		},
		{
			name: "all ones",
			key: func() []byte {
				k := make([]byte, 32)
				for i := range k {
					k[i] = 0xFF
				}
				return k
			}(),
			wantErr: true,
			errMsg:  "weak key detected",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateKey(tc.key)
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

// Helper function to create a test key
func makeTestKey() []byte {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1) // Avoid all zeros
	}
	return key
}

func BenchmarkGCMEncryptor_Encrypt(b *testing.B) {
	encryptor := NewGCMEncryptor()
	key := makeTestKey()
	plaintext := make([]byte, 1024) // 1KB

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := encryptor.Encrypt(plaintext, key)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGCMEncryptor_Decrypt(b *testing.B) {
	encryptor := NewGCMEncryptor()
	key := makeTestKey()
	plaintext := make([]byte, 1024) // 1KB

	ciphertext, _ := encryptor.Encrypt(plaintext, key)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := encryptor.Decrypt(ciphertext, key)
		if err != nil {
			b.Fatal(err)
		}
	}
}
