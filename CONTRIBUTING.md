# Contributing to DotEnv CLI

First off, thank you for considering contributing to DotEnv CLI! It's people like you that make DotEnv CLI such a great tool.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [How Can I Contribute?](#how-can-i-contribute)
- [Development Process](#development-process)
- [Style Guidelines](#style-guidelines)
- [Commit Guidelines](#commit-guidelines)
- [Pull Request Process](#pull-request-process)
- [Community](#community)

## Code of Conduct

This project and everyone participating in it is governed by our Code of Conduct. By participating, you are expected to uphold this code. Please report unacceptable behavior to conduct@dotenv.com.

### Our Standards

- Be respectful and inclusive
- Welcome newcomers and help them get started
- Focus on what is best for the community
- Show empathy towards other community members

## Getting Started

### Prerequisites

- Go 1.21 or higher
- Git
- Make
- A GitHub account

### Setting Up Your Development Environment

1. **Fork the repository** on GitHub

2. **Clone your fork**:
   ```bash
   git clone https://github.com/YOUR_USERNAME/cli.git
   cd cli
   ```

3. **Add upstream remote**:
   ```bash
   git remote add upstream https://github.com/dotenv/cli.git
   ```

4. **Install dependencies**:
   ```bash
   go mod download
   ```

5. **Run tests** to ensure everything is working:
   ```bash
   make test
   ```

## How Can I Contribute?

### Reporting Bugs

Before creating bug reports, please check existing issues to avoid duplicates. When you create a bug report, include as many details as possible:

**Bug Report Template**:
```markdown
**Describe the bug**
A clear and concise description of what the bug is.

**To Reproduce**
Steps to reproduce the behavior:
1. Run command '...'
2. With flags '...'
3. See error

**Expected behavior**
What you expected to happen.

**Environment:**
 - OS: [e.g. macOS 12.0]
 - CLI Version: [e.g. 1.0.0]
 - Go Version: [e.g. 1.21]

**Additional context**
Add any other context about the problem here.
```

### Suggesting Enhancements

Enhancement suggestions are tracked as GitHub issues. When creating an enhancement suggestion, include:

- **Use a clear and descriptive title**
- **Provide a detailed description** of the suggested enhancement
- **Explain why this enhancement would be useful**
- **List any similar features** in other tools

### Your First Code Contribution

Unsure where to begin? Look for issues labeled:
- `good first issue` - Simple issues perfect for beginners
- `help wanted` - Issues where we need community help
- `documentation` - Documentation improvements

### Pull Requests

1. **Small, focused PRs** are easier to review
2. **Write tests** for new functionality
3. **Update documentation** as needed
4. **Follow the style guidelines**

## Development Process

### 1. Create a Branch

```bash
# Update your fork
git checkout main
git pull upstream main
git push origin main

# Create a feature branch
git checkout -b feature/your-feature-name
```

### 2. Make Your Changes

- Write clean, well-documented code
- Add tests for new functionality
- Update documentation if needed
- Ensure all tests pass

### 3. Test Your Changes

```bash
# Run all tests
make test

# Run linter
make lint

# Build the binary
make build

# Test your changes manually
./bin/dotenv your-command
```

### 4. Commit Your Changes

We use [Conventional Commits](https://www.conventionalcommits.org/):

```bash
git add .
git commit -m "feat: add new export format"
```

### 5. Push and Create a Pull Request

```bash
git push origin feature/your-feature-name
```

Then create a pull request on GitHub.

## Style Guidelines

### Go Code Style

We follow standard Go conventions:

1. **Format your code** with `gofmt`
2. **Follow [Effective Go](https://golang.org/doc/effective_go)**
3. **Use meaningful names** for variables and functions
4. **Keep functions small** and focused
5. **Handle errors explicitly**

Example:
```go
// Good
func EncryptData(data []byte, key []byte) ([]byte, error) {
    if len(key) != 32 {
        return nil, ErrInvalidKeyLength
    }
    
    cipher, err := aes.NewCipher(key)
    if err != nil {
        return nil, fmt.Errorf("creating cipher: %w", err)
    }
    
    // ... rest of function
}

// Bad
func encrypt(d []byte, k []byte) []byte {
    c, _ := aes.NewCipher(k)
    // ... 
}
```

### Documentation Style

1. **Package comments** should be at the top of the file
2. **Exported functions** must have comments
3. **Use examples** in documentation
4. **Keep comments up to date**

Example:
```go
// Package crypto provides encryption and decryption functionality
// for the DotEnv CLI using AES-256-GCM.
package crypto

// Encrypt encrypts the provided data using AES-256-GCM.
// It returns the encrypted data with the nonce prepended.
//
// Example:
//
//	key := GenerateKey()
//	encrypted, err := Encrypt([]byte("secret"), key)
//	if err != nil {
//	    log.Fatal(err)
//	}
func Encrypt(data []byte, key []byte) ([]byte, error) {
    // Implementation
}
```

### Test Style

1. **Table-driven tests** for multiple cases
2. **Descriptive test names**
3. **Test both success and error cases**
4. **Use test helpers** to reduce duplication

Example:
```go
func TestEncrypt(t *testing.T) {
    tests := []struct {
        name    string
        data    []byte
        key     []byte
        wantErr bool
    }{
        {
            name:    "valid encryption",
            data:    []byte("test data"),
            key:     GenerateKey(),
            wantErr: false,
        },
        {
            name:    "invalid key length",
            data:    []byte("test data"),
            key:     []byte("short key"),
            wantErr: true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            _, err := Encrypt(tt.data, tt.key)
            if (err != nil) != tt.wantErr {
                t.Errorf("Encrypt() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

## Commit Guidelines

### Commit Message Format

We use [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <subject>

<body>

<footer>
```

### Types

- **feat**: New feature
- **fix**: Bug fix
- **docs**: Documentation changes
- **style**: Code style changes (formatting, etc)
- **refactor**: Code refactoring
- **test**: Test additions or modifications
- **chore**: Maintenance tasks

### Examples

```bash
# Feature
git commit -m "feat(auth): add OAuth2 authentication support"

# Bug fix
git commit -m "fix(crypto): resolve decryption error with unicode characters"

# Documentation
git commit -m "docs(readme): update installation instructions for Windows"

# With body
git commit -m "feat(export): add Kubernetes secret format

- Add new format handler for Kubernetes secrets
- Support both stringData and data fields
- Include namespace in output"
```

## Pull Request Process

### Before Submitting

1. **Update from upstream**:
   ```bash
   git fetch upstream
   git rebase upstream/main
   ```

2. **Run all checks**:
   ```bash
   make test
   make lint
   make build
   ```

3. **Update documentation** if needed

### PR Template

```markdown
## Description
Brief description of what this PR does.

## Type of Change
- [ ] Bug fix (non-breaking change which fixes an issue)
- [ ] New feature (non-breaking change which adds functionality)
- [ ] Breaking change (fix or feature that would cause existing functionality to not work as expected)
- [ ] Documentation update

## How Has This Been Tested?
- [ ] Unit tests
- [ ] Integration tests
- [ ] Manual testing

## Checklist
- [ ] My code follows the style guidelines of this project
- [ ] I have performed a self-review of my own code
- [ ] I have commented my code, particularly in hard-to-understand areas
- [ ] I have made corresponding changes to the documentation
- [ ] My changes generate no new warnings
- [ ] I have added tests that prove my fix is effective or that my feature works
- [ ] New and existing unit tests pass locally with my changes
```

### Review Process

1. **Automated checks** must pass
2. **Code review** by at least one maintainer
3. **Address feedback** promptly
4. **Keep PR updated** with main branch

### After Merge

- Delete your feature branch
- Update your fork
- Celebrate your contribution! 🎉

## Community

### Getting Help

- **Discord**: [Join our community](https://discord.gg/dotenv)
- **GitHub Discussions**: For questions and discussions
- **Stack Overflow**: Tag questions with `dotenv-cli`

### Ways to Contribute Beyond Code

- **Improve documentation**
- **Review pull requests**
- **Help triage issues**
- **Write blog posts or tutorials**
- **Give talks about DotEnv CLI**
- **Help other users**

### Recognition

We value all contributions! Contributors are:
- Listed in our [Contributors](https://github.com/dotenv/cli/graphs/contributors) page
- Mentioned in release notes
- Given credit in the changelog

## Questions?

Feel free to:
- Open an issue for questions
- Ask in Discord
- Email us at contributors@dotenv.com

Thank you for contributing to DotEnv CLI! 💙