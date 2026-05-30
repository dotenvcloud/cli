# Installation Guide

This guide covers all installation methods for the DotEnv CLI across different platforms.

## Table of Contents
- [System Requirements](#system-requirements)
- [Release Channels](#release-channels)
- [Recommended Installation](#recommended-installation)
- [Platform-Specific Instructions](#platform-specific-instructions)
- [Installation from Source](#installation-from-source)
- [Verifying Installation](#verifying-installation)
- [Updating](#updating)
- [Uninstalling](#uninstalling)

## System Requirements

- **Operating System**: macOS 10.15+, Linux (64-bit), Windows 10+
- **Architecture**: x86_64 (Intel/AMD) or ARM64 (Apple Silicon, ARM)
- **Disk Space**: ~50MB
- **Network**: Internet connection for API access

## Release Channels

The CLI publishes two install channels.

### Stable

Tagged releases (`v0.1.0`, `v0.2.0`, …). Published by `release.yml` on every `v*` tag push. Available via every install method below — Homebrew, Scoop, deb/rpm/apk, Docker (`:v0.1.0`, `:latest`), and the install script's default behavior.

```bash
curl -sSL https://dotenv.cloud/install.sh | bash
```

### Nightly (bleeding-edge `main` HEAD)

For staging/dev environments that want the latest unreleased commits. A single `nightly` GitHub pre-release is **rebuilt on every successful CI on `main`** and pinned to that commit's SHA. Replaces the previous nightly in place — no Release-page churn.

```bash
curl -sSL https://dotenv.cloud/install.sh | bash -s -- --nightly
```

Direct asset URL pattern (versioned filename — the installer's `--nightly` flag handles the lookup for you):

```
https://github.com/dotenvcloud/cli/releases/download/nightly/dotenv-cli_<version>-next_<os>_<arch>.<ext>
```

Or pull the rolling main image from Docker Hub / GHCR (published by `docker-publish.yml`):

```bash
docker pull dotenvcloud/cli:main
# or
docker pull ghcr.io/dotenvcloud/cli:main
```

**Caveats**:
- Nightly artifacts are **not** published to the Homebrew tap or Scoop bucket. Those carry stable releases only.
- Nightly versions look like `0.1.1-next` — the next patch after the most recent stable, with `-next` suffix.
- No stability or back-compat guarantees. Use stable for production.

## Recommended Installation

We recommend using your platform's package manager for easy updates.

### macOS (Homebrew)

```bash
brew tap dotenv/tap
brew install dotenv
```

### Linux (APT/Debian/Ubuntu)

```bash
curl -sSL https://dotenv.cloud/public.key | sudo apt-key add -
echo "deb https://dotenv.cloud/apt stable main" | sudo tee /etc/apt/sources.list.d/dotenv.list
sudo apt update
sudo apt install dotenv-cli
```

### Linux (YUM/RHEL/CentOS/Fedora)

```bash
sudo rpm --import https://dotenv.cloud/public.key
sudo curl -o /etc/yum.repos.d/dotenv.repo https://dotenv.cloud/yum/dotenv.repo
sudo yum install dotenv-cli
```

### Windows (Scoop)

```powershell
scoop bucket add dotenv https://github.com/dotenv/scoop-bucket
scoop install dotenv
```

## Platform-Specific Instructions

### macOS

#### Using the Install Script

```bash
curl -sSL https://dotenv.cloud/install.sh | bash
```

This script will:
1. Detect your system architecture (Intel or Apple Silicon)
2. Download the appropriate binary
3. Install to `/usr/local/bin` (requires sudo)
4. Verify the installation

#### Manual Installation

1. Download the latest release:
   - Intel Mac: `dotenv-darwin-amd64`
   - Apple Silicon: `dotenv-darwin-arm64`

2. Make it executable:
   ```bash
   chmod +x dotenv-darwin-*
   ```

3. Move to your PATH:
   ```bash
   sudo mv dotenv-darwin-* /usr/local/bin/dotenv
   ```

### Linux

#### Using the Install Script

```bash
curl -sSL https://dotenv.cloud/install.sh | bash
```

#### Manual Installation

1. Download the appropriate binary:
   ```bash
   # For x86_64
   wget https://github.com/dotenv/cli/releases/latest/download/dotenv-linux-amd64
   
   # For ARM64
   wget https://github.com/dotenv/cli/releases/latest/download/dotenv-linux-arm64
   ```

2. Make executable and install:
   ```bash
   chmod +x dotenv-linux-*
   sudo mv dotenv-linux-* /usr/local/bin/dotenv
   ```

#### Snap Package

```bash
sudo snap install dotenv-cli
```

### Windows

#### Using PowerShell Script

Run PowerShell as Administrator:

```powershell
Set-ExecutionPolicy RemoteSigned -Scope CurrentUser
iwr -useb https://dotenv.cloud/install.ps1 | iex
```

#### Using Chocolatey

```powershell
choco install dotenv-cli
```

#### Manual Installation

1. Download `dotenv-windows-amd64.exe` from [releases](https://github.com/dotenv/cli/releases)
2. Rename to `dotenv.exe`
3. Add to PATH:
   - Create directory: `C:\Program Files\DotEnv`
   - Move `dotenv.exe` to this directory
   - Add `C:\Program Files\DotEnv` to your PATH environment variable

### Docker

```dockerfile
FROM alpine:latest
RUN apk add --no-cache curl \
    && curl -sSL https://dotenv.cloud/install.sh | sh
```

Or use our official image:

```bash
docker run --rm -v ~/.dotenv:/root/.dotenv dotenv/cli:latest pull myproject
```

## Installation from Source

### Prerequisites
- Go 1.21 or higher
- Git

### Steps

1. Clone the repository:
   ```bash
   git clone https://github.com/dotenv/cli.git
   cd cli
   ```

2. Build:
   ```bash
   make build
   ```

3. Install:
   ```bash
   make install
   ```

Or using `go install`:

```bash
go install github.com/dotenv/cli@latest
```

This installs to `$GOPATH/bin` (usually `~/go/bin`).

## Verifying Installation

After installation, verify it works:

```bash
dotenv version
```

Expected output:
```
DotEnv CLI v1.0.0 (commit: abc123, built: 2024-01-01, go: 1.21)
```

Check installation location:
```bash
which dotenv
```

## Shell Completions

### Bash

```bash
echo 'source <(dotenv completion bash)' >> ~/.bashrc
source ~/.bashrc
```

### Zsh

```bash
echo 'source <(dotenv completion zsh)' >> ~/.zshrc
source ~/.zshrc
```

### Fish

```bash
dotenv completion fish > ~/.config/fish/completions/dotenv.fish
```

### PowerShell

```powershell
dotenv completion powershell | Out-String | Invoke-Expression
```

To make permanent:
```powershell
dotenv completion powershell > $PROFILE
```

## Updating

### Using Package Managers

#### Homebrew
```bash
brew upgrade dotenv
```

#### APT
```bash
sudo apt update
sudo apt upgrade dotenv-cli
```

#### Scoop
```powershell
scoop update dotenv
```

### Using Built-in Command

The CLI can update itself:

```bash
# Check for updates
dotenv update --check

# Update to latest
dotenv update
```

### Manual Update

Download the latest binary and replace the existing one.

## Uninstalling

### Package Managers

#### Homebrew
```bash
brew uninstall dotenv
brew untap dotenv/tap
```

#### APT
```bash
sudo apt remove dotenv-cli
```

#### Scoop
```powershell
scoop uninstall dotenv
```

### Manual Uninstall

1. Remove the binary:
   ```bash
   sudo rm /usr/local/bin/dotenv
   ```

2. Remove configuration:
   ```bash
   rm -rf ~/.dotenv
   ```

3. Remove from PATH if manually added

## Troubleshooting Installation

### Permission Denied

If you get permission errors:

```bash
# Option 1: Use sudo
curl -sSL https://dotenv.cloud/install.sh | sudo bash

# Option 2: Install to user directory
curl -sSL https://dotenv.cloud/install.sh | bash -s -- --prefix=$HOME/.local
```

Then add to PATH:
```bash
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
```

### SSL Certificate Errors

For self-signed certificates in development:

```bash
export DOTENV_TLS_SKIP_VERIFY=true
dotenv init
```

### Proxy Configuration

If behind a corporate proxy:

```bash
export HTTP_PROXY=http://proxy.company.com:8080
export HTTPS_PROXY=http://proxy.company.com:8080
curl -sSL https://dotenv.cloud/install.sh | bash
```

### Architecture Mismatch

If you download the wrong architecture:

```bash
# Check your architecture
uname -m

# x86_64 = amd64
# aarch64 = arm64
```

## Next Steps

After installation:

1. Run `dotenv init` to set up configuration
2. Run `dotenv login` to authenticate
3. See [Getting Started Guide](guides/getting-started.md)