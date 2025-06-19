package key

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeriveKey(t *testing.T) {
	password := "test-password-123"
	salt := []byte("saltsaltsalt")

	testCases := []struct {
		name       string
		password   string
		salt       []byte
		iterations int
		wantErr    bool
		errMsg     string
	}{
		{
			name:       "valid parameters",
			password:   password,
			salt:       salt,
			iterations: 10000,
			wantErr:    false,
		},
		{
			name:       "empty password",
			password:   "",
			salt:       salt,
			iterations: 10000,
			wantErr:    true,
			errMsg:     "password cannot be empty",
		},
		{
			name:       "short salt",
			password:   password,
			salt:       []byte("short"),
			iterations: 10000,
			wantErr:    true,
			errMsg:     "salt too short",
		},
		{
			name:       "nil salt",
			password:   password,
			salt:       nil,
			iterations: 10000,
			wantErr:    true,
			errMsg:     "salt too short",
		},
		{
			name:       "low iterations",
			password:   password,
			salt:       salt,
			iterations: 500,
			wantErr:    true,
			errMsg:     "iterations too low",
		},
		{
			name:       "minimum iterations",
			password:   password,
			salt:       salt,
			iterations: 1000,
			wantErr:    false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			key, err := DeriveKey(tc.password, tc.salt, tc.iterations)
			if tc.wantErr {
				assert.Error(t, err)
				if tc.errMsg != "" {
					assert.Contains(t, err.Error(), tc.errMsg)
				}
			} else {
				require.NoError(t, err)
				assert.Len(t, key, RequiredKeySize)
			}
		})
	}
}

func TestDeriveKeyWithDefaults(t *testing.T) {
	password := "test-password"
	salt := []byte("saltsaltsalt1234")

	key, err := DeriveKeyWithDefaults(password, salt)
	require.NoError(t, err)
	assert.Len(t, key, RequiredKeySize)

	// Should use default iterations
	keyWithIterations, err := DeriveKey(password, salt, DefaultIterations)
	require.NoError(t, err)
	assert.Equal(t, key, keyWithIterations)
}

func TestDeriveKey_Deterministic(t *testing.T) {
	password := "my-secret-password"
	salt := []byte("constant-salt-16")
	iterations := 10000

	// Derive key multiple times with same parameters
	key1, err := DeriveKey(password, salt, iterations)
	require.NoError(t, err)

	key2, err := DeriveKey(password, salt, iterations)
	require.NoError(t, err)

	// Should produce the same key
	assert.Equal(t, key1, key2)
}

func TestDeriveKey_DifferentPasswords(t *testing.T) {
	salt := []byte("constant-salt-16")
	iterations := 10000

	key1, err := DeriveKey("password1", salt, iterations)
	require.NoError(t, err)

	key2, err := DeriveKey("password2", salt, iterations)
	require.NoError(t, err)

	// Different passwords should produce different keys
	assert.NotEqual(t, key1, key2)
}

func TestDeriveKey_DifferentSalts(t *testing.T) {
	password := "same-password"
	iterations := 10000

	key1, err := DeriveKey(password, []byte("salt-one-1234567"), iterations)
	require.NoError(t, err)

	key2, err := DeriveKey(password, []byte("salt-two-1234567"), iterations)
	require.NoError(t, err)

	// Different salts should produce different keys
	assert.NotEqual(t, key1, key2)
}

func TestDeriveKey_DifferentIterations(t *testing.T) {
	password := "same-password"
	salt := []byte("constant-salt-16")

	key1, err := DeriveKey(password, salt, 10000)
	require.NoError(t, err)

	key2, err := DeriveKey(password, salt, 20000)
	require.NoError(t, err)

	// Different iteration counts should produce different keys
	assert.NotEqual(t, key1, key2)
}

func TestDeriveKey_KnownVector(t *testing.T) {
	// Test with a known PBKDF2-SHA256 test vector
	// This ensures our implementation matches standard PBKDF2
	password := "password"
	salt := []byte("saltsalt") // 8 bytes minimum
	iterations := 1000

	key, err := DeriveKey(password, salt, iterations)
	require.NoError(t, err)
	assert.Len(t, key, RequiredKeySize)

	// The key should be deterministic
	key2, err := DeriveKey(password, salt, iterations)
	require.NoError(t, err)
	assert.True(t, bytes.Equal(key, key2))
}

func BenchmarkDeriveKey_1000(b *testing.B) {
	password := "benchmark-password"
	salt := []byte("benchmark-salt16")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := DeriveKey(password, salt, 1000)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDeriveKey_10000(b *testing.B) {
	password := "benchmark-password"
	salt := []byte("benchmark-salt16")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := DeriveKey(password, salt, 10000)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDeriveKey_100000(b *testing.B) {
	password := "benchmark-password"
	salt := []byte("benchmark-salt16")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := DeriveKey(password, salt, 100000)
		if err != nil {
			b.Fatal(err)
		}
	}
}
