#!/usr/bin/env bash
#
# DotEnv CLI Installation Script
#
# This script installs the DotEnv CLI on your system.
# It detects your operating system and architecture, downloads the appropriate binary,
# and installs it to /usr/local/bin.
#
# Usage:
#   curl -sSL https://dotenv.cloud/install.sh | bash
#   wget -qO- https://dotenv.cloud/install.sh | bash
#

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
GITHUB_REPO="lostlink/dotenv-cli"
INSTALL_DIR_ROOT="/usr/local/bin"
INSTALL_DIR_USER="$HOME/.local/bin"
BINARY_NAME="dotenv"
INSTALL_URL="https://dotenv.cloud/install.sh"
DRY_RUN=0

# Helper functions
info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1" >&2
    exit 1
}

# Detect OS and architecture
detect_os() {
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    case "$OS" in
        linux*) OS="linux" ;;
        darwin*) OS="darwin" ;;
        mingw*|msys*|cygwin*) OS="windows" ;;
        *) error "Unsupported operating system: $OS" ;;
    esac
    echo "$OS"
}

detect_arch() {
    ARCH=$(uname -m)
    case "$ARCH" in
        x86_64|amd64) ARCH="amd64" ;;
        arm64|aarch64) ARCH="arm64" ;;
        *) error "Unsupported architecture: $ARCH" ;;
    esac
    echo "$ARCH"
}

# Get the latest release version
get_latest_version() {
    curl -s "https://api.github.com/repos/$GITHUB_REPO/releases/latest" | \
        grep '"tag_name":' | \
        sed -E 's/.*"([^"]+)".*/\1/'
}

# Download and install
install_dotenv() {
    OS=$(detect_os)
    ARCH=$(detect_arch)
    VERSION=$(get_latest_version)

    if [ -z "$VERSION" ]; then
        if [ "$DRY_RUN" = "1" ]; then
            warn "Could not determine latest version (no releases yet?); using placeholder for dry-run"
            VERSION="v0.0.0"
        else
            error "Failed to get latest version"
        fi
    fi

    info "Installing DotEnv CLI $VERSION for $OS/$ARCH..."

    # Construct download URL
    FILENAME="dotenv-cli_${VERSION#v}_${OS}_${ARCH}"
    if [ "$OS" = "windows" ]; then
        FILENAME="${FILENAME}.zip"
    else
        FILENAME="${FILENAME}.tar.gz"
    fi

    DOWNLOAD_URL="https://github.com/$GITHUB_REPO/releases/download/$VERSION/$FILENAME"

    if [ "$DRY_RUN" = "1" ]; then
        info "Dry-run: would download $DOWNLOAD_URL"
        info "Dry-run: would install to ${INSTALL_DIR_ROOT} (fallback ${INSTALL_DIR_USER})"
        info "Dry-run: skipping download and install"
        return 0
    fi

    # Create temporary directory
    TMP_DIR=$(mktemp -d)
    trap "rm -rf $TMP_DIR" EXIT

    # Download binary
    info "Downloading from $DOWNLOAD_URL..."
    if ! curl -sL "$DOWNLOAD_URL" -o "$TMP_DIR/$FILENAME"; then
        error "Failed to download binary"
    fi
    
    # Extract binary
    cd "$TMP_DIR"
    if [ "$OS" = "windows" ]; then
        unzip -q "$FILENAME"
    else
        tar -xzf "$FILENAME"
    fi
    
    # Check if binary exists
    if [ ! -f "$BINARY_NAME" ] && [ ! -f "${BINARY_NAME}.exe" ]; then
        error "Binary not found in archive"
    fi
    
    # Install binary
    if [ "$OS" = "windows" ]; then
        warn "Please move ${BINARY_NAME}.exe to a directory in your PATH"
    else
        # Determine installation directory
        if [ -w "$INSTALL_DIR_ROOT" ]; then
            INSTALL_DIR="$INSTALL_DIR_ROOT"
        else
            INSTALL_DIR="$INSTALL_DIR_USER"
            # Create user bin directory if it doesn't exist
            mkdir -p "$INSTALL_DIR"
        fi
        
        info "Installing to $INSTALL_DIR..."
        if [ -w "$INSTALL_DIR" ]; then
            mv "$BINARY_NAME" "$INSTALL_DIR/"
        else
            info "Requesting sudo access to install to $INSTALL_DIR..."
            sudo mv "$BINARY_NAME" "$INSTALL_DIR/"
        fi
        chmod +x "$INSTALL_DIR/$BINARY_NAME"
    fi
    
    # Verify installation
    if command -v "$BINARY_NAME" >/dev/null 2>&1; then
        info "Installation successful!"
        info "Run 'dotenv version' to verify"
    else
        warn "Installation completed but 'dotenv' command not found in PATH"
        warn "You may need to add $INSTALL_DIR to your PATH"
        if [ "$INSTALL_DIR" = "$INSTALL_DIR_USER" ]; then
            echo
            info "To add to PATH, run:"
            info "  echo 'export PATH=\"$INSTALL_DIR:\$PATH\"' >> ~/.bashrc"
            info "  source ~/.bashrc"
            info "Or for zsh:"
            info "  echo 'export PATH=\"$INSTALL_DIR:\$PATH\"' >> ~/.zshrc"
            info "  source ~/.zshrc"
        fi
    fi
}

# Main
main() {
    # Parse args
    while [ $# -gt 0 ]; do
        case "$1" in
            --dry-run)
                DRY_RUN=1
                shift
                ;;
            -h|--help)
                echo "Usage: install.sh [--dry-run]"
                exit 0
                ;;
            *)
                error "Unknown argument: $1"
                ;;
        esac
    done

    echo "DotEnv CLI Installer"
    echo "===================="
    echo
    if [ "$DRY_RUN" = "1" ]; then
        info "Running in dry-run mode (no changes will be made)"
    fi

    # Check for curl
    if ! command -v curl >/dev/null 2>&1; then
        error "curl is required but not installed"
    fi

    # Install
    install_dotenv
    
    echo
    echo "Next steps:"
    echo "  1. Run 'dotenv init' to set up your configuration"
    echo "  2. Run 'dotenv login' to authenticate"
    echo "  3. Run 'dotenv pull <project>' to pull your secrets"
    echo
    echo "For more information, visit: https://dotenv.cloud/docs/cli"
}

main "$@"