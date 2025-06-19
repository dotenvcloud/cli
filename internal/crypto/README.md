# Crypto Module

This module provides AES-256-GCM encryption that is 100% compatible with PHP and JavaScript implementations.

## Features

- **AES-256-GCM encryption**: Industry-standard authenticated encryption
- **Cross-platform compatibility**: Works identically with PHP (OpenSSL) and JavaScript (Web Crypto/Node.js)
- **Multiple encryption modes**: Server-managed, client-managed, and hybrid modes
- **Key management**: Generation, validation, and derivation (PBKDF2)
- **Secure defaults**: Random IVs, strong key validation, constant-time operations

## Usage

### Basic Encryption/Decryption

```go
import "github.com/dotenv/cli/internal/crypto"

// Initialize the crypto module
crypto.Initialize()

// Generate a key
key, err := crypto.GenerateKey()
if err != nil {
    panic(err)
}

// Encrypt
plaintext := "Hello, World!"
ciphertext, err := crypto.EncryptString(plaintext, key)
if err != nil {
    panic(err)
}

// Decrypt
decrypted, err := crypto.DecryptString(ciphertext, key)
if err != nil {
    panic(err)
}
```

### Encryption Modes

```go
import "github.com/dotenv/cli/internal/crypto"

encryptor := crypto.NewModeEncryptor()

// Server-managed encryption
ciphertext, err := encryptor.EncryptWithMode(
    []byte("secret data"),
    crypto.ServerManaged,
    serverKey,
    nil,
)

// Client-managed encryption
ciphertext, err := encryptor.EncryptWithMode(
    []byte("secret data"),
    crypto.ClientManaged,
    nil,
    clientKey,
)

// Hybrid encryption (double encryption)
ciphertext, err := encryptor.EncryptWithMode(
    []byte("secret data"),
    crypto.Hybrid,
    serverKey,
    clientKey,
)
```

### Key Management

```go
import "github.com/dotenv/cli/internal/crypto/key"

// Generate a new key
key, err := key.GenerateKey()

// Generate a base64-encoded key
keyStr, err := key.GenerateKeyString()

// Parse a key from various formats
key, err := key.ParseKey(keyString) // Supports base64, hex, or raw

// Derive a key from password
salt, err := key.GenerateSalt(16)
derivedKey, err := key.DeriveKey(password, salt, 100000)

// Validate a key
err := key.ValidateKey(keyBytes)
```

## Encryption Format

The encryption format is: `base64(IV || ciphertext || tag)`

- **IV**: 12 bytes (96 bits) - randomly generated for each encryption
- **Ciphertext**: Variable length - the encrypted data
- **Tag**: 16 bytes (128 bits) - authentication tag from GCM

## Cross-Platform Compatibility

### PHP Example

```php
<?php
$key = base64_decode($keyBase64);
$data = base64_decode($encryptedData);

$iv = substr($data, 0, 12);
$ciphertext = substr($data, 12, -16);
$tag = substr($data, -16);

$plaintext = openssl_decrypt(
    $ciphertext,
    'aes-256-gcm',
    $key,
    OPENSSL_RAW_DATA,
    $iv,
    $tag
);
?>
```

### JavaScript Example

```javascript
const key = Buffer.from(keyBase64, 'base64');
const data = Buffer.from(encryptedData, 'base64');

const iv = data.slice(0, 12);
const tag = data.slice(-16);
const ciphertext = data.slice(12, -16);

const decipher = crypto.createDecipheriv('aes-256-gcm', key, iv);
decipher.setAuthTag(tag);

const plaintext = Buffer.concat([
    decipher.update(ciphertext),
    decipher.final()
]).toString('utf8');
```

## Testing

### Unit Tests

```bash
go test ./internal/crypto/...
```

### Cross-Platform Tests

```bash
# Generate test data with Go
cd internal/crypto/examples
go run test-go.go -mode=test

# Test with PHP
php test-php.php

# Test with Node.js
node test-node.js
```

### Benchmarks

```bash
go test ./internal/crypto/... -bench=. -benchmem
```

## Security Considerations

1. **Key Management**: Never hardcode keys. Use environment variables or secure key storage.
2. **IV Uniqueness**: Never reuse an IV with the same key.
3. **Authentication**: Always verify the authentication tag (handled automatically by GCM).
4. **Key Rotation**: Implement regular key rotation for enhanced security.
5. **Error Handling**: Don't leak information through error messages.

## Performance

Typical performance on modern hardware:
- Encryption: ~500 MB/s
- Decryption: ~500 MB/s
- Key derivation (100k iterations): ~50ms

## Troubleshooting

### "Invalid key size" error
Ensure your key is exactly 32 bytes. Use `key.GenerateKey()` or validate with `key.ValidateKey()`.

### "Decryption failed" error
- Verify the key is correct
- Ensure the ciphertext hasn't been modified
- Check that the format matches (base64 encoded)

### Cross-platform issues
- Ensure all platforms use standard base64 (not URL-safe)
- Verify byte order is consistent
- Use the provided test scripts to validate compatibility