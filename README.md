# DotEnv CLI

<div align="center">
  <img src="https://dotenv.cloud/logo.svg" alt="DotEnv Logo" width="200">
  
  [![Version](https://img.shields.io/github/v/release/dotenv/cli)](https://github.com/dotenv/cli/releases)
  [![Build Status](https://img.shields.io/github/workflow/status/dotenv/cli/test)](https://github.com/dotenv/cli/actions)
  [![Coverage](https://img.shields.io/codecov/c/github/dotenv/cli)](https://codecov.io/gh/dotenv/cli)
  [![Go Report Card](https://goreportcard.com/badge/github.com/dotenv/cli)](https://goreportcard.com/report/github.com/dotenv/cli)
  [![License](https://img.shields.io/github/license/dotenv/cli)](LICENSE)
</div>

<p align="center">
  <strong>Secure environment variable management for modern applications</strong>
</p>

<p align="center">
  <a href="#-installation">Installation</a> •
  <a href="#-quick-start">Quick Start</a> •
  <a href="#-features">Features</a> •
  <a href="#-documentation">Documentation</a> •
  <a href="#-contributing">Contributing</a>
</p>

---

## 🚀 Installation

### macOS/Linux

Using Homebrew:
```bash
brew tap dotenv/tap
brew install dotenv
```

Using curl:
```bash
curl -sSL https://dotenv.cloud/install.sh | bash
```

### Windows

Using Scoop:
```powershell
scoop bucket add dotenv https://github.com/dotenv/scoop-bucket
scoop install dotenv
```

Using PowerShell:
```powershell
iwr -useb https://dotenv.cloud/install.ps1 | iex
```

### From Source

```bash
go install github.com/dotenv/cli@latest
```

See [Installation Guide](docs/INSTALLATION.md) for more options.

## ⚡ Quick Start

### 1. Initialize

```bash
dotenv init
```

This interactive setup will:
- Configure API connection
- Set up authentication
- Configure telemetry preferences

### 2. Login

```bash
dotenv login
```

Authenticate via browser and select organizations to access. During `dotenv init`, you'll be offered to login immediately after choosing browser authentication.

### 3. Pull Secrets

```bash
# Pull all secrets for a project
dotenv pull myproject

# Pull specific environment
dotenv pull myproject/production

# Output to file
dotenv pull myproject --output=.env
```

### 4. Push Secrets

```bash
# Push from file
dotenv push myproject .env

# Push with hierarchy
dotenv push myproject --project=.env.project --target=.env.production
```

## ✨ Features

### 🔐 Security First
- **Client-side encryption** with AES-256-GCM
- **Zero-knowledge architecture** - we can't read your secrets
- **Secure key storage** in local configuration
- **API token scoping** per organization

### 🎯 Hierarchical Secrets
- **Project-level** defaults
- **Target-specific** overrides (staging, production)
- **Environment-specific** values
- **Smart inheritance** - most specific wins

### 📦 Multiple Formats
- **ENV** - Standard .env files
- **JSON** - For modern applications
- **YAML** - For configuration files
- **Shell** - Export statements
- **Dockerfile** - For container builds

### 🔄 Variable Interpolation
```bash
BASE_URL=https://api.example.com
API_ENDPOINT=${BASE_URL}/v1
```

### 🚀 CI/CD Ready
- **Non-interactive mode** for automation
- **Exit codes** for scripting
- **Machine-readable output** formats
- **Service account** support

### 📊 Optional Telemetry
Help improve DotEnv CLI with anonymous usage data:
- Command usage patterns
- Performance metrics
- Error rates
- **Never** includes secret values or personal data

## 🛠️ Commands

```bash
dotenv [command] [flags]

Commands:
  init          Initialize configuration
  login         Authenticate with DotEnv
  pull          Pull secrets from DotEnv
  push          Push secrets to DotEnv
  list          List resources (projects, environments)
  export        Export secrets in various formats
  refresh       Refresh API credentials
  update        Update CLI to latest version
  use-context   Switch between organizations
  version       Show version information

Flags:
  -h, --help      Show help
  -v, --version   Show version
  --debug         Enable debug output
  --quiet         Suppress non-error output
  --no-color      Disable colored output

Use "dotenv [command] --help" for more information about a command.
```

## 🔧 Configuration

Configuration is stored in `~/.dotenv/config.yaml`:

```yaml
version: "1.0"
telemetry_enabled: true
current_context: production
contexts:
  production:
    api_url: https://api.dotenv.cloud
    organization: acme-corp
  staging:
    api_url: https://api.dotenv.cloud
    organization: acme-corp-staging
```

See [Configuration Guide](docs/guides/configuration.md) for details.

## 📖 Documentation

### Guides
- [Getting Started](docs/guides/getting-started.md)
- [Configuration](docs/guides/configuration.md)
- [Encryption](docs/guides/encryption.md)
- [CI/CD Integration](docs/guides/ci-cd-integration.md)
- [Migration Guide](docs/guides/migration.md)

### References
- [Command Reference](docs/references/commands.md)
- [Config File Reference](docs/references/config-file.md)
- [Environment Variables](docs/references/environment-vars.md)
- [File Formats](docs/references/file-formats.md)

### Examples
- [Basic Usage](docs/examples/basic-usage.md)
- [Docker Integration](docs/examples/docker.md)
- [Kubernetes Secrets](docs/examples/kubernetes.md)
- [GitHub Actions](docs/examples/github-actions.md)
- [GitLab CI](docs/examples/gitlab-ci.md)

## 🐛 Troubleshooting

### Common Issues

**Authentication failed**
```bash
dotenv refresh
```

**Command not found**
```bash
echo 'export PATH="$HOME/.dotenv/bin:$PATH"' >> ~/.bashrc
source ~/.bashrc
```

**Permission denied**
```bash
chmod +x ~/.dotenv/bin/dotenv
```

See [Troubleshooting Guide](docs/TROUBLESHOOTING.md) for more solutions.

## 🤝 Contributing

We welcome contributions! Please see our [Contributing Guide](CONTRIBUTING.md) for details.

### Development Setup

```bash
# Clone the repository
git clone https://github.com/dotenv/cli.git
cd cli

# Install dependencies
go mod download

# Run tests
make test

# Build locally
make build
```

### Running Tests

```bash
# Unit tests
go test ./...

# Integration tests
go test -tags=integration ./tests/integration/...

# Coverage report
make test-coverage
```

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- Built with [Cobra](https://github.com/spf13/cobra) and [Viper](https://github.com/spf13/viper)
- Encryption powered by Go's crypto package
- Inspired by the simplicity of .env files

## 📞 Support

- 📧 Email: support@dotenv.cloud
- 💬 Discord: [Join our community](https://discord.gg/dotenv)
- 🐛 Issues: [GitHub Issues](https://github.com/dotenv/cli/issues)
- 📖 Docs: [Documentation](https://dotenv.cloud/docs/cli)

---

<p align="center">
  Made with ❤️ by the <a href="https://dotenv.cloud">DotEnv</a> team
</p>
