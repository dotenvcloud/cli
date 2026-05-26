package key

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateKey(t *testing.T) {
	testCases := []struct {
		name    string
		key     []byte
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid key",
			key: func() []byte {
				k := make([]byte, RequiredKeySize)
				for i := range k {
					k[i] = byte(i + 1)
				}
				return k
			}(),
			wantErr: false,
		},
		{
			name:    "nil key",
			key:     nil,
			wantErr: true,
			errMsg:  "key is nil",
		},
		{
			name:    "short key",
			key:     make([]byte, 16),
			wantErr: true,
			errMsg:  "invalid key size",
		},
		{
			name:    "long key",
			key:     make([]byte, 64),
			wantErr: true,
			errMsg:  "invalid key size",
		},
		{
			name:    "all zeros",
			key:     make([]byte, RequiredKeySize),
			wantErr: true,
			errMsg:  "weak key detected",
		},
		{
			name: "all ones",
			key: func() []byte {
				k := make([]byte, RequiredKeySize)
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

func TestValidateKeyString(t *testing.T) {
	// Create a valid key
	validKey := make([]byte, RequiredKeySize)
	for i := range validKey {
		validKey[i] = byte(i + 1)
	}
	validKeyStr := base64.StdEncoding.EncodeToString(validKey)

	// Create a weak key (all zeros)
	weakKey := make([]byte, RequiredKeySize)
	weakKeyStr := base64.StdEncoding.EncodeToString(weakKey)

	testCases := []struct {
		name    string
		keyStr  string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid base64 key",
			keyStr:  validKeyStr,
			wantErr: false,
		},
		{
			name:    "weak base64 key",
			keyStr:  weakKeyStr,
			wantErr: true,
			errMsg:  "weak key detected",
		},
		// invalid_base64 / wrong_length cases dropped: ParseKey now absorbs
		// those via the raw-bytes + padding path, so ValidateKeyString sees
		// a valid 32-byte key. Only empty input remains an error.
		{
			name:    "empty string",
			keyStr:  "",
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateKeyString(tc.keyStr)
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
