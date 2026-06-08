package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	dotenv "github.com/dotenvcloud/sdk-go"

	"github.com/dotenvcloud/cli/internal/config"
	"github.com/dotenvcloud/cli/internal/ui"
)

// resolvedKey is the outcome of resolving a project's encryption key. For
// client-managed projects it also carries the PBKDF2 proof parameters the
// caller needs to prove (server-side) that it holds the right key.
type resolvedKey struct {
	key        string // RAW key string (never hex/base64-decoded)
	managed    string // "server" | "client"
	proofSalt  string // client-managed: base64 PBKDF2 salt
	proofIters int    // client-managed: PBKDF2 iterations
}

// resolveEncryptionKey returns the project encryption key as a RAW STRING — it
// is never hex/base64-decoded, because the platform contract derives the AES
// key from the key string's bytes (see dotenv.DeriveProjectKey). Decoding it
// here would break decryption of data written by the web app and JS SDK.
//
// It first fetches the key descriptor to learn the storage mode:
//
//   - server-managed: the server hands back the key; any --client-key is ignored.
//   - client-managed: the server returns only the proof params (no key), so the
//     key is resolved from --client-key (file or value), then the
//     DOTENV_CLIENT_KEY env var, then the interactive prompt.
//
// prompt is injected so the client-managed branch can be exercised in tests
// without a TTY.
func resolveEncryptionKey(
	ctx context.Context,
	client *dotenv.Client,
	projectSlug, clientKeyFlag string,
	prompt func() (string, error),
) (resolvedKey, error) {
	desc, resp, err := client.Encryption.GetEncryptionKey(ctx, projectSlug)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		if dotenv.IsNotFound(err) {
			return resolvedKey{}, fmt.Errorf("encryption key not found for project '%s'. The project may not have encryption enabled", projectSlug)
		}
		return resolvedKey{}, HandleAPIError(err, accountForErrorContext())
	}

	// Server-managed: the server holds the authoritative key.
	if !desc.IsClientManaged && desc.Managed != "client" {
		if clientKeyFlag != "" {
			ui.PrintWarningf("project '%s' is server-managed; ignoring --client-key and using the server key.", projectSlug)
		}
		return resolvedKey{key: desc.Key, managed: "server"}, nil
	}

	// Client-managed: resolve the key locally and carry the proof params.
	key, kerr := resolveClientKeyValue(
		clientKeyFlag, prompt,
		"This project uses client-managed encryption. Please provide your encryption key.",
	)
	if kerr != nil {
		return resolvedKey{}, kerr
	}
	return resolvedKey{
		key:        key,
		managed:    "client",
		proofSalt:  desc.KeyCheckSalt,
		proofIters: desc.KeyCheckIterations,
	}, nil
}

// resolveClientKeyValue resolves a client-managed key from (in order) the
// --client-key flag (file or literal value), the DOTENV_CLIENT_KEY env var, or
// the interactive prompt. promptIntro, when non-empty, is printed just before
// prompting so the user understands why they are being asked.
func resolveClientKeyValue(
	clientKeyFlag string,
	prompt func() (string, error),
	promptIntro string,
) (string, error) {
	if clientKeyFlag != "" {
		return interpretClientKeyFlag(clientKeyFlag)
	}

	if envKey := strings.TrimSpace(os.Getenv(config.EnvClientKey)); envKey != "" {
		ui.PrintWarningf(
			"client key read from %s — less safe than a file (it can leak via the process environment). Prefer --client-key=<file>.",
			config.EnvClientKey,
		)
		return envKey, nil
	}

	if promptIntro != "" {
		ui.PrintInfof("%s", promptIntro)
	}
	key, perr := prompt()
	if perr != nil {
		return "", fmt.Errorf(
			"client-managed encryption requires a key; provide one via --client-key=<file>, the %s env var, or the interactive prompt: %w",
			config.EnvClientKey, perr,
		)
	}
	return key, nil
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
// encryption (the key descriptor's managed field is "client").
func projectIsClientManaged(ctx context.Context, client *dotenv.Client, projectSlug string) bool {
	desc, resp, err := client.Encryption.GetEncryptionKey(ctx, projectSlug)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return false
	}
	return desc.IsClientManaged || desc.Managed == "client"
}
