package crypto

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestModeEncryptor_ServerManaged(t *testing.T) {
	encryptor := NewModeEncryptor()
	serverKey := makeTestKey()
	plaintext := []byte("test data")

	// Encrypt
	ciphertext, err := encryptor.EncryptWithMode(plaintext, ServerManaged, serverKey, nil)
	require.NoError(t, err)
	assert.NotEmpty(t, ciphertext)

	// Decrypt
	decrypted, err := encryptor.DecryptWithMode(ciphertext, ServerManaged, serverKey, nil)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)

	// Should fail without server key
	_, err = encryptor.EncryptWithMode(plaintext, ServerManaged, nil, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "server key required")
}

func TestModeEncryptor_ClientManaged(t *testing.T) {
	encryptor := NewModeEncryptor()
	clientKey := makeTestKey()
	plaintext := []byte("test data")

	// Encrypt
	ciphertext, err := encryptor.EncryptWithMode(plaintext, ClientManaged, nil, clientKey)
	require.NoError(t, err)
	assert.NotEmpty(t, ciphertext)

	// Decrypt
	decrypted, err := encryptor.DecryptWithMode(ciphertext, ClientManaged, nil, clientKey)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)

	// Should fail without client key
	_, err = encryptor.EncryptWithMode(plaintext, ClientManaged, nil, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "client key required")
}

func TestModeEncryptor_Hybrid(t *testing.T) {
	encryptor := NewModeEncryptor()
	serverKey := makeTestKey()
	clientKey := makeTestKey()
	clientKey[0] = serverKey[0] + 1 // Make sure keys are different
	plaintext := []byte("test data")

	// Encrypt
	ciphertext, err := encryptor.EncryptWithMode(plaintext, Hybrid, serverKey, clientKey)
	require.NoError(t, err)
	assert.NotEmpty(t, ciphertext)

	// Decrypt
	decrypted, err := encryptor.DecryptWithMode(ciphertext, Hybrid, serverKey, clientKey)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted)

	// Should fail without either key
	_, err = encryptor.EncryptWithMode(plaintext, Hybrid, serverKey, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "both server and client keys required")

	_, err = encryptor.EncryptWithMode(plaintext, Hybrid, nil, clientKey)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "both server and client keys required")

	// Should fail with wrong key order
	_, err = encryptor.DecryptWithMode(ciphertext, Hybrid, clientKey, serverKey)
	assert.Error(t, err)
}

func TestModeEncryptor_InvalidMode(t *testing.T) {
	encryptor := NewModeEncryptor()
	key := makeTestKey()
	plaintext := []byte("test")

	_, err := encryptor.EncryptWithMode(plaintext, "invalid", key, key)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown encryption mode")

	_, err = encryptor.DecryptWithMode("test", "invalid", key, key)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown encryption mode")
}

func TestValidateMode(t *testing.T) {
	testCases := []struct {
		mode    EncryptionMode
		wantErr bool
	}{
		{ServerManaged, false},
		{ClientManaged, false},
		{Hybrid, false},
		{"invalid", true},
		{"", true},
	}

	for _, tc := range testCases {
		t.Run(string(tc.mode), func(t *testing.T) {
			err := ValidateMode(tc.mode)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestParseMode(t *testing.T) {
	testCases := []struct {
		input   string
		want    EncryptionMode
		wantErr bool
	}{
		{"server", ServerManaged, false},
		{"client", ClientManaged, false},
		{"hybrid", Hybrid, false},
		{"invalid", "", true},
		{"", "", true},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			mode, err := ParseMode(tc.input)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.want, mode)
			}
		})
	}
}

func TestEncryptionMode_Methods(t *testing.T) {
	testCases := []struct {
		mode              EncryptionMode
		isServerManaged   bool
		isClientManaged   bool
		requiresServerKey bool
		requiresClientKey bool
	}{
		{ServerManaged, true, false, true, false},
		{ClientManaged, false, true, false, true},
		{Hybrid, true, true, true, true},
	}

	for _, tc := range testCases {
		t.Run(tc.mode.String(), func(t *testing.T) {
			assert.Equal(t, tc.isServerManaged, tc.mode.IsServerManaged())
			assert.Equal(t, tc.isClientManaged, tc.mode.IsClientManaged())
			assert.Equal(t, tc.requiresServerKey, tc.mode.RequiresServerKey())
			assert.Equal(t, tc.requiresClientKey, tc.mode.RequiresClientKey())
		})
	}
}
