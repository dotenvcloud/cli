package key

import (
	"encoding/base64"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateKey(t *testing.T) {
	// Test key generation
	key, err := GenerateKey()
	require.NoError(t, err)
	assert.Len(t, key, RequiredKeySize)

	// Ensure randomness - generate multiple keys
	keys := make(map[string]bool)
	for i := 0; i < 100; i++ {
		k, err := GenerateKey()
		require.NoError(t, err)
		keyStr := base64.StdEncoding.EncodeToString(k)
		keys[keyStr] = true
	}

	// All keys should be unique
	assert.Equal(t, 100, len(keys))
}

func TestGenerateKeyString(t *testing.T) {
	keyStr, err := GenerateKeyString()
	require.NoError(t, err)
	assert.NotEmpty(t, keyStr)

	// Should be valid base64
	decoded, err := base64.StdEncoding.DecodeString(keyStr)
	require.NoError(t, err)
	assert.Len(t, decoded, RequiredKeySize)
}

func TestGenerateKeyHex(t *testing.T) {
	keyHex, err := GenerateKeyHex()
	require.NoError(t, err)
	assert.NotEmpty(t, keyHex)

	// Should be valid hex
	decoded, err := hex.DecodeString(keyHex)
	require.NoError(t, err)
	assert.Len(t, decoded, RequiredKeySize)

	// Hex string should be 64 characters (32 bytes * 2)
	assert.Len(t, keyHex, RequiredKeySize*2)
}

func TestParseKey(t *testing.T) {
	// Generate test keys in different formats
	originalKey, _ := GenerateKey()
	base64Key := base64.StdEncoding.EncodeToString(originalKey)
	hexKey := hex.EncodeToString(originalKey)

	testCases := []struct {
		name    string
		input   string
		wantErr bool
		check   func([]byte)
	}{
		{
			name:    "valid base64",
			input:   base64Key,
			wantErr: false,
			check: func(key []byte) {
				assert.Equal(t, originalKey, key)
			},
		},
		{
			name:    "valid hex",
			input:   hexKey,
			wantErr: false,
			check: func(key []byte) {
				assert.Equal(t, originalKey, key)
			},
		},
		{
			name:    "raw 32 bytes",
			input:   string(make([]byte, 32)),
			wantErr: false,
			check: func(key []byte) {
				assert.Len(t, key, RequiredKeySize)
			},
		},
		{
			name:    "invalid base64",
			input:   "not-valid-base64!@#",
			wantErr: true,
		},
		{
			name:    "wrong length base64",
			input:   base64.StdEncoding.EncodeToString(make([]byte, 16)),
			wantErr: true,
		},
		{
			name:    "wrong length hex",
			input:   hex.EncodeToString(make([]byte, 20)), // 40 chars, not 64
			wantErr: true,
		},
		{
			name:    "wrong length raw",
			input:   "short",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			key, err := ParseKey(tc.input)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Len(t, key, RequiredKeySize)
				if tc.check != nil {
					tc.check(key)
				}
			}
		})
	}
}

func TestGenerateSalt(t *testing.T) {
	testCases := []struct {
		name    string
		size    int
		wantErr bool
	}{
		{
			name:    "valid size 8",
			size:    8,
			wantErr: false,
		},
		{
			name:    "valid size 16",
			size:    16,
			wantErr: false,
		},
		{
			name:    "valid size 32",
			size:    32,
			wantErr: false,
		},
		{
			name:    "too small",
			size:    4,
			wantErr: true,
		},
		{
			name:    "zero size",
			size:    0,
			wantErr: true,
		},
		{
			name:    "negative size",
			size:    -1,
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			salt, err := GenerateSalt(tc.size)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Len(t, salt, tc.size)

				// Check randomness
				salt2, err := GenerateSalt(tc.size)
				require.NoError(t, err)
				assert.NotEqual(t, salt, salt2)
			}
		})
	}
}

func BenchmarkGenerateKey(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := GenerateKey()
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseKey_Base64(b *testing.B) {
	key, _ := GenerateKey()
	base64Key := base64.StdEncoding.EncodeToString(key)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ParseKey(base64Key)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseKey_Hex(b *testing.B) {
	key, _ := GenerateKey()
	hexKey := hex.EncodeToString(key)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ParseKey(hexKey)
		if err != nil {
			b.Fatal(err)
		}
	}
}
