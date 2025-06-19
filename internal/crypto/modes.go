package crypto

import (
	"fmt"
)

// EncryptionMode defines how encryption is handled
type EncryptionMode string

const (
	// ServerManaged mode - server stores and manages encryption keys
	ServerManaged EncryptionMode = "server"

	// ClientManaged mode - client provides and manages encryption keys
	ClientManaged EncryptionMode = "client"

	// Hybrid mode - double encryption with both server and client keys
	Hybrid EncryptionMode = "hybrid"
)

// ModeEncryptor handles encryption with different modes
type ModeEncryptor struct {
	encryptor Encryptor
}

// NewModeEncryptor creates a new mode-aware encryptor
func NewModeEncryptor() *ModeEncryptor {
	return &ModeEncryptor{
		encryptor: NewGCMEncryptor(),
	}
}

// EncryptWithMode encrypts data according to the specified mode
func (m *ModeEncryptor) EncryptWithMode(plaintext []byte, mode EncryptionMode, serverKey, clientKey []byte) (string, error) {
	switch mode {
	case ServerManaged:
		if serverKey == nil {
			return "", fmt.Errorf("server key required for server-managed mode")
		}
		return m.encryptor.Encrypt(plaintext, serverKey)

	case ClientManaged:
		if clientKey == nil {
			return "", fmt.Errorf("client key required for client-managed mode")
		}
		return m.encryptor.Encrypt(plaintext, clientKey)

	case Hybrid:
		if serverKey == nil || clientKey == nil {
			return "", fmt.Errorf("both server and client keys required for hybrid mode")
		}

		// First encrypt with client key
		clientEncrypted, err := m.encryptor.Encrypt(plaintext, clientKey)
		if err != nil {
			return "", fmt.Errorf("client encryption failed: %w", err)
		}

		// Then encrypt the result with server key
		serverEncrypted, err := m.encryptor.Encrypt([]byte(clientEncrypted), serverKey)
		if err != nil {
			return "", fmt.Errorf("server encryption failed: %w", err)
		}

		return serverEncrypted, nil

	default:
		return "", fmt.Errorf("unknown encryption mode: %s", mode)
	}
}

// DecryptWithMode decrypts data according to the specified mode
func (m *ModeEncryptor) DecryptWithMode(ciphertext string, mode EncryptionMode, serverKey, clientKey []byte) ([]byte, error) {
	switch mode {
	case ServerManaged:
		if serverKey == nil {
			return nil, fmt.Errorf("server key required for server-managed mode")
		}
		return m.encryptor.Decrypt(ciphertext, serverKey)

	case ClientManaged:
		if clientKey == nil {
			return nil, fmt.Errorf("client key required for client-managed mode")
		}
		return m.encryptor.Decrypt(ciphertext, clientKey)

	case Hybrid:
		if serverKey == nil || clientKey == nil {
			return nil, fmt.Errorf("both server and client keys required for hybrid mode")
		}

		// First decrypt with server key
		serverDecrypted, err := m.encryptor.Decrypt(ciphertext, serverKey)
		if err != nil {
			return nil, fmt.Errorf("server decryption failed: %w", err)
		}

		// Then decrypt the result with client key
		clientDecrypted, err := m.encryptor.Decrypt(string(serverDecrypted), clientKey)
		if err != nil {
			return nil, fmt.Errorf("client decryption failed: %w", err)
		}

		return clientDecrypted, nil

	default:
		return nil, fmt.Errorf("unknown encryption mode: %s", mode)
	}
}

// ValidateMode checks if an encryption mode is valid
func ValidateMode(mode EncryptionMode) error {
	switch mode {
	case ServerManaged, ClientManaged, Hybrid:
		return nil
	default:
		return fmt.Errorf("invalid encryption mode: %s", mode)
	}
}

// ParseMode converts a string to EncryptionMode
func ParseMode(s string) (EncryptionMode, error) {
	mode := EncryptionMode(s)
	if err := ValidateMode(mode); err != nil {
		return "", err
	}
	return mode, nil
}

// String returns the string representation of the mode
func (m EncryptionMode) String() string {
	return string(m)
}

// IsServerManaged returns true if the mode involves server-side encryption
func (m EncryptionMode) IsServerManaged() bool {
	return m == ServerManaged || m == Hybrid
}

// IsClientManaged returns true if the mode involves client-side encryption
func (m EncryptionMode) IsClientManaged() bool {
	return m == ClientManaged || m == Hybrid
}

// RequiresServerKey returns true if the mode requires a server key
func (m EncryptionMode) RequiresServerKey() bool {
	return m == ServerManaged || m == Hybrid
}

// RequiresClientKey returns true if the mode requires a client key
func (m EncryptionMode) RequiresClientKey() bool {
	return m == ClientManaged || m == Hybrid
}
