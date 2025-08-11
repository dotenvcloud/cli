# CRITICAL DEVELOPMENT REMINDERS - CLI

<critical>
- NEVER make assumptions about how ANY code works. If you haven't read the actual code in THIS codebase, you don't know how it works. Period.
</critical>

## Go Development Standards
- Follow idiomatic Go patterns (effective Go guidelines)
- Error handling: Always check and wrap errors with context
- Use Go modules for dependency management
- Target single binary distribution

## Cobra Command Structure
- Commands in cmd/ directory
- Persistent flags on root command
- Command-specific flags on subcommands
- Use viper for configuration management

## Encryption Requirements
- Client-side encryption: AES-256-GCM mandatory
- 32-byte keys, 12-byte IV, base64(IV + ciphertext + tag) format
- Support both server-managed and client-managed keys
- NEVER log encryption keys or decrypted content

## Cross-Platform
- Test on Linux, macOS, Windows
- Use filepath (not path) for file operations
- Handle OS-specific behaviors explicitly