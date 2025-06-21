package oauth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
)

// PKCEChallenge represents a PKCE challenge pair
type PKCEChallenge struct {
	Verifier  string
	Challenge string
	Method    string
}

// GeneratePKCEChallenge creates a new PKCE challenge pair
func GeneratePKCEChallenge() (*PKCEChallenge, error) {
	// Generate 32 random bytes for the verifier
	verifierBytes := make([]byte, 32)
	if _, err := rand.Read(verifierBytes); err != nil {
		return nil, fmt.Errorf("failed to generate random bytes: %w", err)
	}

	// Base64URL encode the verifier
	verifier := base64URLEncode(verifierBytes)

	// Create SHA256 hash of the verifier
	h := sha256.New()
	h.Write([]byte(verifier))
	challengeBytes := h.Sum(nil)

	// Base64URL encode the challenge
	challenge := base64URLEncode(challengeBytes)

	return &PKCEChallenge{
		Verifier:  verifier,
		Challenge: challenge,
		Method:    "S256",
	}, nil
}

// base64URLEncode performs base64url encoding without padding
func base64URLEncode(data []byte) string {
	encoded := base64.URLEncoding.EncodeToString(data)
	// Remove padding
	encoded = strings.TrimRight(encoded, "=")
	return encoded
}

// GenerateState generates a random state parameter for CSRF protection
func GenerateState() (string, error) {
	// Generate 32 bytes to ensure we get at least 32 characters when base64 encoded
	stateBytes := make([]byte, 32)
	if _, err := rand.Read(stateBytes); err != nil {
		return "", fmt.Errorf("failed to generate state: %w", err)
	}
	return base64URLEncode(stateBytes), nil
}