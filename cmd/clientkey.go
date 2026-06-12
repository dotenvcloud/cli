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

// resolvedKey is the outcome of resolving a project's encryption key. It carries
// the (non-secret) PBKDF2 data-key parameters for BOTH custody modes — server-
// managed projects now derive the AES key the same way as client-managed.
type resolvedKey struct {
	key        string // RAW key string (never hex/base64-decoded)
	managed    string // "server" | "client"
	proofSalt  string // base64 PBKDF2 data-key salt (sent for both modes)
	proofIters int    // PBKDF2 iterations
}

// dataKey returns the actual key bytes to feed the AES-256-GCM layer, as a
// string (the crypto layer treats the key as raw bytes; padKey is a no-op on a
// 32-byte input).
//
// UNIFIED: both server- and client-managed projects derive the AES key via
// dotenv.DeriveDataKey(key, salt, iterations). The only difference is custody —
// for server-managed the server sent us the key; for client-managed the user
// supplied it. Either way decryption happens here, on the client. A
// server-managed key with no salt (legacy/pre-unification) falls back to the raw
// key. Both the CLI and the browser MUST derive this way or they cannot decrypt
// each other's data.
func (rk resolvedKey) dataKey() (string, error) {
	if rk.proofSalt != "" {
		aesKey, err := dotenv.DeriveDataKey(rk.key, rk.proofSalt, rk.proofIters)
		if err != nil {
			return "", fmt.Errorf("failed to derive data key: %w", err)
		}
		return string(aesKey), nil
	}

	// No salt configured.
	if rk.managed == managedClient {
		return "", fmt.Errorf(
			"this project's client-managed key has no salt configured; re-establish " +
				"its encryption key (web key setup) or recreate it with " +
				"`dotenv project create --storage client`",
		)
	}
	// Server-managed legacy key without a salt: use the raw key (padKey no-op).
	return rk.key, nil
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

	// Server-managed: the server hands back the key value AND the data-key
	// salt/iterations so we derive the same unified PBKDF2 AES key as the browser.
	if !desc.IsClientManaged && desc.Managed != managedClient {
		if clientKeyFlag != "" {
			ui.PrintWarningf("project '%s' is server-managed; ignoring --client-key and using the server key.", projectSlug)
		}
		return resolvedKey{
			key:        desc.Key,
			managed:    managedServer,
			proofSalt:  desc.KeyCheckSalt,
			proofIters: desc.KeyCheckIterations,
		}, nil
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
		managed:    managedClient,
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

	ui.PrintWarningf(
		"client key passed as a literal argument — it can leak via shell history " +
			"and the process list. Prefer --client-key=<file>.",
	)
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

// resolveOldClientKey resolves and VALIDATES the key for a rotated client-managed
// key version. Candidates (each a file path or literal value, from repeatable
// --old-key flags) are tried first, then the interactive prompt — every candidate
// is checked against the key version's stored proof BEFORE use, so a wrong key is
// rejected up front rather than producing garbage. When the old key has no stored
// proof, the first resolvable candidate is accepted (AES-GCM authentication is
// the backstop). No env-var tier on purpose: a stale DOTENV_CLIENT_KEY must never
// silently masquerade as an old key.
func resolveOldClientKey(
	keyVersion string,
	hist *dotenv.EncryptionKeyVersion,
	candidates []string,
	prompt func() (string, error),
) (string, error) {
	verify := func(value string) (bool, error) {
		if hist == nil || hist.KeyCheck == "" {
			return true, nil // no proof recorded; GCM auth is the backstop
		}
		return dotenv.VerifyKeyProof(value, hist.KeyCheckSalt, hist.KeyCheckIterations, hist.KeyCheck)
	}

	for _, candidate := range candidates {
		value, err := interpretClientKeyFlag(candidate)
		if err != nil {
			ui.PrintWarningf("skipping --old-key %q: %v", candidate, err)
			continue
		}
		ok, verr := verify(value)
		if verr != nil {
			return "", fmt.Errorf("failed to verify old key: %w", verr)
		}
		if ok {
			return value, nil
		}
	}

	ui.PrintInfof("Key v%s is needed to decrypt this data.", keyVersion)
	for attempt := 0; attempt < 3; attempt++ {
		value, err := prompt()
		if err != nil {
			return "", fmt.Errorf("old key v%s is required: %w", keyVersion, err)
		}
		ok, verr := verify(value)
		if verr != nil {
			return "", fmt.Errorf("failed to verify old key: %w", verr)
		}
		if ok {
			return value, nil
		}
		ui.PrintWarningf("that key does not match key v%s — try again.", keyVersion)
	}

	return "", fmt.Errorf("could not resolve a valid key for key version %s", keyVersion)
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
	return desc.IsClientManaged || desc.Managed == managedClient
}
