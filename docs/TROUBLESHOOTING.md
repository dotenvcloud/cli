# Troubleshooting Guide

Solutions for common issues with DotEnv CLI.

## Table of Contents

- [Installation Issues](#installation-issues)
- [Authentication Problems](#authentication-problems)
- [Command Errors](#command-errors)
- [Network Issues](#network-issues)
- [Encryption Problems](#encryption-problems)
- [Configuration Issues](#configuration-issues)
- [Performance Problems](#performance-problems)
- [Getting Help](#getting-help)

## Installation Issues

### Command Not Found

**Problem**: `dotenv: command not found`

**Solutions**:

1. **Check installation**:
   ```bash
   # Check if installed
   which dotenv
   ls -la /usr/local/bin/dotenv
   ```

2. **Add to PATH**:
   ```bash
   # For bash
   echo 'export PATH="/usr/local/bin:$PATH"' >> ~/.bashrc
   source ~/.bashrc
   
   # For zsh
   echo 'export PATH="/usr/local/bin:$PATH"' >> ~/.zshrc
   source ~/.zshrc
   ```

3. **Reinstall**:
   ```bash
   curl -sSL https://dotenv.cloud/install.sh | bash
   ```

### Permission Denied

**Problem**: `permission denied` when installing

**Solutions**:

1. **Use sudo**:
   ```bash
   curl -sSL https://dotenv.cloud/install.sh | sudo bash
   ```

2. **Install to user directory**:
   ```bash
   curl -sSL https://dotenv.cloud/install.sh | bash -s -- --prefix=$HOME/.local
   echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc
   ```

3. **Fix permissions**:
   ```bash
   sudo chmod +x /usr/local/bin/dotenv
   ```

### Wrong Architecture

**Problem**: `cannot execute binary file: Exec format error`

**Solution**:

1. **Check your architecture**:
   ```bash
   uname -m
   # x86_64 = amd64
   # aarch64 = arm64
   ```

2. **Download correct version**:
   ```bash
   # For Apple Silicon Mac
   curl -Lo dotenv https://github.com/dotenv/cli/releases/latest/download/dotenv-darwin-arm64
   
   # For Intel Mac
   curl -Lo dotenv https://github.com/dotenv/cli/releases/latest/download/dotenv-darwin-amd64
   ```

## Authentication Problems

### No Current Context

**Problem**: `no current context. Run 'dotenv init' to get started`

**Solutions**:

1. **Initialize config**:
   ```bash
   dotenv init
   ```

2. **Login**:
   ```bash
   dotenv login
   ```

3. **Check contexts**:
   ```bash
   dotenv list contexts
   dotenv use-context <name>
   ```

### Invalid API Key

**Problem**: `authentication failed: invalid API key`

**Solutions**:

1. **Refresh credentials**:
   ```bash
   dotenv refresh
   ```

2. **Re-login**:
   ```bash
   dotenv login
   ```

3. **Check environment variable**:
   ```bash
   echo $DOTENV_API_KEY
   unset DOTENV_API_KEY  # Remove if set incorrectly
   ```

### Browser Doesn't Open

**Problem**: Browser doesn't open during `dotenv login`

**Solutions**:

1. **Use no-browser mode**:
   ```bash
   dotenv login --no-browser
   # Copy URL and open manually
   ```

2. **Check default browser**:
   ```bash
   # Linux
   xdg-settings get default-web-browser
   
   # macOS
   open -a "Safari" https://example.com
   ```

3. **SSH session**:
   ```bash
   # Forward port if using SSH
   ssh -L 8080:localhost:8080 user@server
   dotenv login --callback-port=8080
   ```

## Command Errors

### Project Not Found

**Problem**: `project not found: myproject`

**Solutions**:

1. **List available projects**:
   ```bash
   dotenv list projects
   ```

2. **Check organization**:
   ```bash
   dotenv list contexts
   dotenv use-context correct-org
   ```

3. **Verify spelling**:
   ```bash
   # Projects are case-sensitive
   dotenv pull MyProject  # not myproject
   ```

### No Secrets Found

**Problem**: `no secrets found`

**Solutions**:

1. **Check hierarchy**:
   ```bash
   # Try different levels
   dotenv pull project
   dotenv pull project/target
   dotenv pull project/target/environment
   ```

2. **Verify project has secrets**:
   ```bash
   dotenv list projects
   # Check SecretCount column
   ```

### Permission Denied

**Problem**: `permission denied: insufficient access`

**Solutions**:

1. **Check your role**:
   ```bash
   dotenv whoami
   ```

2. **Request access**:
   - Contact organization admin
   - Request appropriate role

3. **Use correct context**:
   ```bash
   dotenv list contexts
   dotenv use-context authorized-org
   ```

## Network Issues

### Connection Timeout

**Problem**: `connection timeout`

**Solutions**:

1. **Check internet connection**:
   ```bash
   ping api.dotenv.cloud
   curl -I https://api.dotenv.cloud
   ```

2. **Proxy configuration**:
   ```bash
   export HTTP_PROXY=http://proxy.company.com:8080
   export HTTPS_PROXY=http://proxy.company.com:8080
   export NO_PROXY=localhost,127.0.0.1
   ```

3. **Firewall rules**:
   - Allow HTTPS (443) to api.dotenv.cloud
   - Check corporate firewall settings

### SSL Certificate Error

**Problem**: `x509: certificate signed by unknown authority`

**Solutions**:

1. **Update CA certificates**:
   ```bash
   # macOS
   brew install ca-certificates
   
   # Linux
   sudo update-ca-certificates
   ```

2. **Corporate proxy/MITM**:
   ```bash
   # Add corporate CA
   export SSL_CERT_FILE=/path/to/corporate-ca.crt
   ```

3. **Development only - skip verification**:
   ```bash
   export DOTENV_TLS_SKIP_VERIFY=true
   dotenv pull myproject
   ```

### Rate Limiting

**Problem**: `rate limit exceeded`

**Solutions**:

1. **Wait and retry**:
   ```bash
   # Check retry-after header
   sleep 60
   dotenv pull myproject
   ```

2. **Reduce frequency**:
   - Batch operations
   - Cache results locally
   - Use webhooks for updates

## Encryption Problems

### Decryption Failed

**Problem**: `decryption failed: cipher: message authentication failed`

**Solutions**:

1. **Verify encryption key**:
   ```bash
   # Re-fetch key
   dotenv pull myproject --force-key-refresh
   ```

2. **Check key rotation**:
   ```bash
   # If keys were rotated
   dotenv refresh
   dotenv pull myproject
   ```

3. **Corrupted data**:
   - Contact support
   - Restore from backup

### Invalid Key Format

**Problem**: `invalid encryption key format`

**Solutions**:

1. **Check key encoding**:
   ```bash
   # Key should be base64 encoded
   echo $ENCRYPTION_KEY | base64 -d | wc -c
   # Should be 32 bytes
   ```

2. **Re-export keys**:
   ```bash
   dotenv export-keys myproject
   ```

## Configuration Issues

### Config File Corrupted

**Problem**: `failed to parse config: yaml error`

**Solutions**:

1. **Backup and recreate**:
   ```bash
   mv ~/.dotenv/config.yaml ~/.dotenv/config.yaml.backup
   dotenv init
   ```

2. **Fix syntax**:
   ```bash
   # Validate YAML
   cat ~/.dotenv/config.yaml | python -c "import yaml, sys; yaml.safe_load(sys.stdin)"
   ```

3. **Manual edit**:
   ```bash
   $EDITOR ~/.dotenv/config.yaml
   ```

### Can't Write Config

**Problem**: `failed to save configuration: permission denied`

**Solutions**:

1. **Check permissions**:
   ```bash
   ls -la ~/.dotenv/
   chmod 700 ~/.dotenv
   chmod 600 ~/.dotenv/config.yaml
   ```

2. **Check disk space**:
   ```bash
   df -h ~
   ```

### Lost Configuration

**Problem**: Configuration file deleted or lost

**Solutions**:

1. **Re-initialize**:
   ```bash
   dotenv init
   dotenv login
   ```

2. **Restore from backup**:
   ```bash
   cp ~/.dotenv/config.yaml.backup ~/.dotenv/config.yaml
   ```

## Performance Problems

### Slow Commands

**Problem**: Commands take too long to execute

**Solutions**:

1. **Check network latency**:
   ```bash
   ping api.dotenv.cloud
   traceroute api.dotenv.cloud
   ```

2. **Enable caching**:
   ```bash
   dotenv config set cache.enabled true
   dotenv config set cache.ttl 300
   ```

3. **Use specific paths**:
   ```bash
   # Slower - searches all levels
   dotenv pull myproject
   
   # Faster - direct path
   dotenv pull myproject/production/api
   ```

### High Memory Usage

**Problem**: CLI uses too much memory

**Solutions**:

1. **Large files**:
   ```bash
   # Split large secret sets
   dotenv pull myproject --limit=100 --offset=0
   dotenv pull myproject --limit=100 --offset=100
   ```

2. **Clear cache**:
   ```bash
   rm -rf ~/.dotenv/cache/*
   ```

## Getting Help

### Debug Mode

Enable detailed output:
```bash
dotenv --debug pull myproject
```

### Version Information

```bash
dotenv version
# Shows version, build info, Go version
```

### Logs

Check logs:
```bash
# macOS/Linux
tail -f ~/.dotenv/logs/dotenv.log

# Enable logging
export DOTENV_LOG_LEVEL=debug
```

### Support Channels

1. **Documentation**: https://dotenv.cloud/docs/cli
2. **GitHub Issues**: https://github.com/dotenv/cli/issues
3. **Discord Community**: https://discord.gg/dotenv
4. **Email Support**: support@dotenv.cloud

### Reporting Bugs

Include:
1. CLI version: `dotenv version`
2. OS and architecture: `uname -a`
3. Error message (full output)
4. Steps to reproduce
5. Debug output: `dotenv --debug <command>`

### Common Error Codes

| Code | Meaning | Solution |
|------|---------|----------|
| 1 | General error | Check error message |
| 2 | Command syntax error | Check `dotenv <cmd> --help` |
| 3 | Authentication required | Run `dotenv login` |
| 4 | Resource not found | Verify resource exists |
| 5 | Permission denied | Check access rights |
| 6 | Network error | Check connection |
| 7 | Decryption failed | Verify encryption key |

## Quick Fixes

### Reset Everything

```bash
# Complete reset
rm -rf ~/.dotenv
curl -sSL https://dotenv.cloud/install.sh | bash
dotenv init
dotenv login
```

### Test Connection

```bash
# Test API connection
curl -H "Authorization: Bearer $DOTENV_API_KEY" \
  https://api.dotenv.cloud/v1/organizations
```

### Verify Installation

```bash
# Check all components
dotenv version
which dotenv
ls -la $(which dotenv)
dotenv config validate
```