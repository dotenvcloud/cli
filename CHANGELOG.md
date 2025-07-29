# Changelog

All notable changes to the DotEnv CLI will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Initial release of DotEnv CLI
- Core commands: init, login, pull, push, list, export, refresh, update, use-context, version
- Support for hierarchical secrets (project/target/environment)
- Client-side encryption with AES-256-GCM
- Multiple output formats: env, json, yaml, shell, dockerfile
- Browser-based authentication flow
- Context management for multiple organizations
- Configuration file with encrypted credential storage
- Shell completion for bash, zsh, fish, and powershell
- Optional telemetry for usage analytics
- Cross-platform support (macOS, Linux, Windows)
- Variable interpolation in secret values
- Non-interactive mode for CI/CD environments
- Debug mode for troubleshooting
- Automatic update checking and self-update capability

### Security
- Zero-knowledge architecture - secrets encrypted client-side
- Machine-specific key derivation for config encryption
- Secure credential storage with proper file permissions
- TLS verification for all API communications
- API token scoping per organization

## [1.0.0] - 2024-01-15

### Added
- Command: `init` - Interactive configuration setup
- Command: `login` - Browser-based authentication
- Command: `pull` - Retrieve secrets from DotEnv
- Command: `push` - Upload secrets to DotEnv
- Command: `list` - List projects, targets, environments
- Command: `export` - Export secrets in various formats
- Command: `refresh` - Refresh API credentials
- Command: `update` - Self-update functionality
- Command: `use-context` - Switch between organizations
- Command: `version` - Display version information

### Features
- Hierarchical secret management (project → target → environment)
- Client-side encryption using AES-256-GCM
- Support for multiple file formats:
  - Standard .env files
  - JSON format
  - YAML format
  - Shell export statements
  - Dockerfile ENV instructions
- Variable interpolation support (${VAR} syntax)
- Context management for multiple organizations
- Secure local configuration storage
- Shell completions for all major shells
- Colored output with --no-color flag
- Debug mode with --debug flag
- Quiet mode with --quiet flag

### Security
- All secrets encrypted before leaving the client
- Encryption keys never transmitted to servers
- Config file permissions set to 0600 (owner only)
- Machine-specific encryption for stored credentials

### Documentation
- Comprehensive README with examples
- Installation guide for all platforms
- Command reference with detailed usage
- Configuration guide
- Troubleshooting guide
- Migration guides from other tools
- Development guide for contributors

## Version History

### Versioning Policy

- **Major versions** (X.0.0): Breaking changes to CLI interface or config format
- **Minor versions** (1.X.0): New features, backwards compatible
- **Patch versions** (1.0.X): Bug fixes and minor improvements

### Deprecation Policy

- Features will be deprecated with warnings for at least one minor version
- Breaking changes will be documented in upgrade guides
- Config format changes will include automatic migration

## Future Releases

### [1.1.0] - Planned

#### Planned Features
- Secret rotation commands
- Audit logging for secret access
- Team collaboration features
- Secret versioning and rollback
- Integration with CI/CD platforms
- Web UI for secret management

### [1.2.0] - Planned

#### Planned Features
- Kubernetes operator
- Terraform provider
- GitHub Action
- VS Code extension
- Secret scanning in code
- Compliance reporting

## Upgrade Guide

### Upgrading from 0.x to 1.0

1. **Backup your configuration**:
   ```bash
   cp ~/.dotenv/config.yaml ~/.dotenv/config.yaml.backup
   ```

2. **Update the CLI**:
   ```bash
   dotenv update
   ```

3. **Re-authenticate**:
   ```bash
   dotenv login
   ```

4. **Verify configuration**:
   ```bash
   dotenv config validate
   ```

## Support

- **Bug Reports**: [GitHub Issues](https://github.com/dotenv/cli/issues)
- **Feature Requests**: [GitHub Discussions](https://github.com/dotenv/cli/discussions)
- **Security Issues**: security@dotenv.cloud
- **General Support**: support@dotenv.cloud

---

[Unreleased]: https://github.com/dotenv/cli/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/dotenv/cli/releases/tag/v1.0.0