package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	dotenv "github.com/dotenvcloud/sdk-go"

	"github.com/dotenvcloud/cli/internal/config"
	"github.com/dotenvcloud/cli/internal/ui"
)

// resolveEncryptionKey returns the project encryption key as a RAW STRING — it
// is never hex/base64-decoded, because the platform contract derives the AES
// key from the key string's bytes (see dotenv.DeriveProjectKey). Decoding it
// here would break decryption of data written by the web app and JS SDK.
//
// Resolution order (shared by push and pull):
//
//  1. clientKeyFlag set   -> treat as a file path or a literal key value
//     (interpretClientKeyFlag) and use it unconditionally.
//  2. clientKeyFlag empty -> fetch the server-managed key. If the project is
//     client-managed (the server holds no retrievable key) fall back to the
//     DOTENV_CLIENT_KEY env var, then to the interactive prompt.
//
// prompt is injected so the client-managed branch can be exercised in tests
// without a TTY.
func resolveEncryptionKey(
	ctx context.Context,
	client *dotenv.Client,
	projectSlug, clientKeyFlag string,
	prompt func() (string, error),
) (string, error) {
	if clientKeyFlag != "" {
		return interpretClientKeyFlag(clientKeyFlag)
	}

	encKeyResp, encResp, err := client.Encryption.GetEncryptionKey(ctx, projectSlug)
	if encResp != nil {
		defer encResp.Body.Close()
	}
	if err == nil {
		return encKeyResp.Key, nil
	}

	// Client-managed: the server cannot hand us a key. Try the env var, then
	// prompt interactively.
	if errors.Is(err, dotenv.ErrClientManagedEncryption) {
		if envKey := strings.TrimSpace(os.Getenv(config.EnvClientKey)); envKey != "" {
			ui.PrintWarningf(
				"client key read from %s — less safe than a file (it can leak via the process environment). Prefer --client-key=<file>.",
				config.EnvClientKey,
			)
			return envKey, nil
		}
		ui.PrintInfof("This project uses client-managed encryption. Please provide your encryption key.")
		key, perr := prompt()
		if perr != nil {
			return "", fmt.Errorf(
				"this project uses client-managed encryption; provide a key via --client-key=<file>, the %s env var, or the interactive prompt: %w",
				config.EnvClientKey, perr,
			)
		}
		return key, nil
	}

	if dotenv.IsNotFound(err) {
		return "", fmt.Errorf("encryption key not found for project '%s'. The project may not have encryption enabled", projectSlug)
	}
	return "", HandleAPIError(err, accountForErrorContext())
}

// interpretClientKeyFlag resolves a --client-key value that may be either a
// file path or the literal key string.
//
//   - existing file        -> read it (safe; no warning).
//   - missing but path-like -> error (the user clearly meant a file; do NOT
//     silently use a mistyped path as a wrong key).
//   - otherwise            -> treat as a literal key + warn (less safe).
//
// The value is trimmed so a key file written with `echo` (trailing newline)
// matches the bytes the web/JS would use.
func interpretClientKeyFlag(value string) (string, error) {
	if info, err := os.Stat(value); err == nil && !info.IsDir() {
		data, rerr := os.ReadFile(value)
		if rerr != nil {
			return "", fmt.Errorf("failed to read client key from %s: %w", value, rerr)
		}
		return strings.TrimSpace(string(data)), nil
	} else if looksLikePath(value) {
		return "", fmt.Errorf("client key file not found: %s", value)
	}

	ui.PrintWarningf("client key passed as a literal argument — it can leak via shell history and the process list. Prefer --client-key=<file>.")
	return strings.TrimSpace(value), nil
}

// looksLikePath reports whether value is more plausibly a (mistyped) file path
// than a raw key, so a typo doesn't get silently used as a wrong key.
func looksLikePath(value string) bool {
	if strings.ContainsAny(value, `/\`) {
		return true
	}
	if strings.HasPrefix(value, ".") || strings.HasPrefix(value, "~") {
		return true
	}
	switch strings.ToLower(filepath.Ext(value)) {
	case ".key", ".pem", ".txt", ".json", ".b64", ".env":
		return true
	}
	return false
}

// promptForClientKey prompts the user to enter their encryption key.
func promptForClientKey() (string, error) {
	keyStr, err := ui.Password("Enter encryption key")
	if err != nil {
		return "", fmt.Errorf("failed to read encryption key: %w", err)
	}
	if keyStr == "" {
		return "", fmt.Errorf("encryption key cannot be empty")
	}
	return strings.TrimSpace(keyStr), nil
}

// projectIsClientManaged reports whether the project uses client-managed
// encryption (the server returns the client_managed_encryption sentinel rather
// than a retrievable key).
func projectIsClientManaged(ctx context.Context, client *dotenv.Client, projectSlug string) bool {
	_, resp, err := client.Encryption.GetEncryptionKey(ctx, projectSlug)
	if resp != nil {
		defer resp.Body.Close()
	}
	return errors.Is(err, dotenv.ErrClientManagedEncryption)
}
