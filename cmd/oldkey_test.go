package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	dotenv "github.com/dotenvcloud/sdk-go"
)

// makeOldKeyDescriptor builds a key-history descriptor whose proof matches key.
func makeOldKeyDescriptor(t *testing.T, key string) *dotenv.EncryptionKeyVersion {
	t.Helper()
	salt, proof, iterations, err := dotenv.GenerateKeyProof(key)
	require.NoError(t, err)

	return &dotenv.EncryptionKeyVersion{
		Version:            "1",
		Managed:            "client",
		KeyCheck:           proof,
		KeyCheckSalt:       salt,
		KeyCheckIterations: iterations,
	}
}

func TestResolveOldClientKey(t *testing.T) {
	correct := "the-old-key"
	hist := makeOldKeyDescriptor(t, correct)

	noPrompt := func() (string, error) { return "", fmt.Errorf("prompt should not be called") }

	t.Run("accepts a matching literal candidate", func(t *testing.T) {
		key, err := resolveOldClientKey("1", hist, []string{correct}, noPrompt)
		require.NoError(t, err)
		require.Equal(t, correct, key)
	})

	t.Run("accepts a matching key file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "old.key")
		require.NoError(t, os.WriteFile(path, []byte(correct+"\n"), 0o600))

		key, err := resolveOldClientKey("1", hist, []string{path}, noPrompt)
		require.NoError(t, err)
		require.Equal(t, correct, key)
	})

	t.Run("skips non-matching candidates and falls back to the prompt", func(t *testing.T) {
		prompted := 0
		prompt := func() (string, error) {
			prompted++
			return correct, nil
		}

		key, err := resolveOldClientKey("1", hist, []string{"wrong-key"}, prompt)
		require.NoError(t, err)
		require.Equal(t, correct, key)
		require.Equal(t, 1, prompted)
	})

	t.Run("rejects after repeated wrong prompts", func(t *testing.T) {
		prompt := func() (string, error) { return "still-wrong", nil }

		_, err := resolveOldClientKey("1", hist, nil, prompt)
		require.Error(t, err)
		require.Contains(t, err.Error(), "could not resolve a valid key")
	})

	t.Run("accepts the first candidate when no proof is recorded", func(t *testing.T) {
		bare := &dotenv.EncryptionKeyVersion{Version: "1", Managed: "client"}

		key, err := resolveOldClientKey("1", bare, []string{"anything"}, noPrompt)
		require.NoError(t, err)
		require.Equal(t, "anything", key)
	})
}
