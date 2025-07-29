# Getting Started Guide

Welcome to DotEnv CLI! This guide will walk you through the initial setup and basic usage of the DotEnv command-line interface.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Initial Setup](#initial-setup)
- [Your First Commands](#your-first-commands)
- [Understanding Hierarchies](#understanding-hierarchies)
- [Working with Environments](#working-with-environments)
- [Best Practices](#best-practices)
- [Next Steps](#next-steps)

## Prerequisites

Before you begin, ensure you have:

1. **A DotEnv account** - Sign up at [dotenv.cloud](https://dotenv.cloud)
2. **An organization** - Created during signup
3. **A project** - Create one in the dashboard
4. **Terminal access** - Command line on macOS, Linux, or Windows

## Installation

### Quick Install (Recommended)

**macOS/Linux:**
```bash
curl -sSL https://dotenv.cloud/install.sh | bash
```

**Windows (PowerShell as Administrator):**
```powershell
iwr -useb https://dotenv.cloud/install.ps1 | iex
```

### Verify Installation

```bash
dotenv version
```

You should see output like:
```
DotEnv CLI v1.0.0 (commit: abc123, built: 2024-01-01, go: 1.21)
```

## Initial Setup

### Step 1: Initialize Configuration

Run the initialization command:

```bash
dotenv init
```

You'll be prompted for:

1. **API URL**: Press Enter to use the default (https://api.dotenv.cloud)
2. **Authentication**: Choose 'browser' for the easiest setup
3. **Telemetry**: Choose whether to help improve DotEnv CLI

Example session:
```
$ dotenv init
Welcome to DotEnv CLI! Let's get you set up.

API URL [https://api.dotenv.cloud]: ↵
Authentication method (browser/token) [browser]: ↵
Enable telemetry to help improve DotEnv CLI? [y/N]: y

Configuration initialized successfully!
Next step: Run 'dotenv login' to authenticate.
```

### Step 2: Login

Authenticate with your DotEnv account:

```bash
dotenv login
```

This will:
1. Open your browser to the DotEnv login page
2. After login, show your available organizations
3. Let you select which organizations to access
4. Save encrypted credentials locally

Example:
```
$ dotenv login
Opening browser for authentication...
If the browser doesn't open, visit: https://app.dotenv.cloud/cli/auth?token=abc123

Waiting for authentication...
✓ Authentication successful!

Available organizations:
  1. acme-corp (Owner)
  2. personal-projects (Member)

Select organizations to access (space to select, enter to confirm):
> [x] acme-corp
  [ ] personal-projects

✓ Context 'acme-corp' created and set as current context
✓ Login successful!
```

### Step 3: Verify Setup

Check that everything is working:

```bash
# List your projects
dotenv list projects
```

You should see your projects listed:
```
NAME          ENVIRONMENTS  SECRETS  CREATED
web-app       3            25       2024-01-01
mobile-api    2            18       2024-01-02
backend       4            42       2024-01-03
```

## Your First Commands

### Pull Secrets

Retrieve secrets from your project:

```bash
# Display secrets to terminal
dotenv pull web-app

# Save to .env file
dotenv pull web-app --output=.env

# Pull specific environment
dotenv pull web-app/production
```

Example output:
```
$ dotenv pull web-app
DATABASE_URL=postgres://user:pass@localhost/db
API_KEY=sk_test_123456789
REDIS_URL=redis://localhost:6379
NODE_ENV=development
```

### Push Secrets

Upload secrets from a file:

```bash
# Create a .env file
cat > .env << EOF
DATABASE_URL=postgres://localhost/myapp
API_KEY=my-secret-key
DEBUG=true
EOF

# Push to DotEnv
dotenv push web-app .env
```

You'll see a confirmation:
```
$ dotenv push web-app .env
Pushing secrets to web-app...
  DATABASE_URL = postgres://localhost/myapp
  API_KEY = ******* (hidden)
  DEBUG = true

Proceed? [y/N]: y
✓ Successfully pushed 3 secrets to web-app
```

### List Resources

Explore your organization's structure:

```bash
# List all projects
dotenv list projects

# List targets in a project
dotenv list targets web-app

# List environments in a target
dotenv list environments web-app/production
```

## Understanding Hierarchies

DotEnv uses a three-level hierarchy:

```
Project (Base configuration)
  └── Target (Regional/service specific)
        └── Environment (Deploy stage)
```

### Example Structure

```
web-app/                      # Project
  ├── shared secrets         # DATABASE_URL, API_ENDPOINT
  ├── us-east-1/            # Target (Region)
  │   ├── shared           # REGION=us-east-1
  │   ├── staging/         # Environment
  │   │   └── secrets     # DEBUG=true, LOG_LEVEL=debug
  │   └── production/      # Environment
  │       └── secrets     # DEBUG=false, LOG_LEVEL=error
  └── eu-west-1/           # Target (Region)
      ├── shared          # REGION=eu-west-1
      └── production/     # Environment
          └── secrets     # GDPR_MODE=true
```

### Inheritance Rules

1. **More specific values override less specific**
2. **Environment > Target > Project**
3. **Explicit paths give you exactly what you ask for**

Example:
```bash
# Get project defaults only
dotenv pull web-app

# Get project + target overrides
dotenv pull web-app/us-east-1

# Get complete hierarchy (project + target + environment)
dotenv pull web-app/us-east-1/production
```

## Working with Environments

### Development Workflow

1. **Create different .env files for each environment:**
   ```bash
   # Development secrets
   cat > .env.development << EOF
   DATABASE_URL=postgres://localhost/dev
   DEBUG=true
   LOG_LEVEL=debug
   EOF
   
   # Production secrets
   cat > .env.production << EOF
   DATABASE_URL=postgres://prod-server/app
   DEBUG=false
   LOG_LEVEL=error
   EOF
   ```

2. **Push to appropriate environments:**
   ```bash
   dotenv push web-app/development .env.development
   dotenv push web-app/production .env.production
   ```

3. **Pull for local development:**
   ```bash
   # For local development
   dotenv pull web-app/development --output=.env
   npm run dev
   ```

### Environment Switching

Create aliases for quick switching:

```bash
# Add to ~/.bashrc or ~/.zshrc
alias dev-env='dotenv pull web-app/development --output=.env'
alias stage-env='dotenv pull web-app/staging --output=.env'
alias prod-env='dotenv pull web-app/production --output=.env'

# Usage
dev-env  # Switch to development
npm start
```

### CI/CD Integration

For automated deployments:

```bash
# In your CI/CD pipeline
export DOTENV_API_KEY="${{ secrets.DOTENV_API_KEY }}"
dotenv pull web-app/production --output=.env
npm run build
```

## Best Practices

### 1. Security First

- **Never commit .env files** to version control
- **Use .gitignore**:
  ```gitignore
  .env
  .env.*
  !.env.example
  ```
- **Rotate secrets regularly**
- **Use read-only tokens in CI/CD**

### 2. Organization

- **Use clear naming conventions**:
  - Projects: `app-name` (e.g., `web-app`, `mobile-api`)
  - Targets: Purpose or region (e.g., `us-east-1`, `kubernetes`)
  - Environments: Standard names (e.g., `development`, `staging`, `production`)

### 3. Documentation

- **Create .env.example files**:
  ```bash
  # .env.example
  DATABASE_URL=
  API_KEY=
  REDIS_URL=
  ```

- **Document required variables** in README

### 4. Team Workflow

- **Share organization access**, not credentials
- **Use descriptive secret names**
- **Comment complex configurations**:
  ```bash
  # Connection string format: postgres://user:pass@host:port/db
  DATABASE_URL=postgres://app:secret@db.example.com:5432/myapp
  ```

## Common Patterns

### Multiple Projects

Working with multiple related projects:

```bash
# Frontend secrets
dotenv pull frontend/production --output=frontend/.env

# Backend secrets
dotenv pull backend/production --output=backend/.env

# Shared configuration
dotenv pull shared-config --output=.env.shared
```

### Variable Interpolation

Use variables within variables:

```bash
# Push base configuration
cat > .env << EOF
API_BASE=https://api.example.com
API_V1=${API_BASE}/v1
API_V2=${API_BASE}/v2
EOF

dotenv push web-app .env

# Pull with interpolation
dotenv pull web-app --resolve
```

### Export Formats

Export in different formats for various tools:

```bash
# For Docker
dotenv export web-app --format=dockerfile > Dockerfile.env

# For Kubernetes
dotenv export web-app --format=kubernetes | kubectl apply -f -

# For shell scripts
dotenv export web-app --format=shell > env.sh
source env.sh
```

## Troubleshooting

### Common Issues

**"No current context"**
```bash
dotenv login
```

**"Project not found"**
```bash
# Check available projects
dotenv list projects

# Check current context
dotenv list contexts
```

**"Permission denied"**
- Ensure you have access to the organization
- Check with your admin for proper permissions

### Debug Mode

For detailed output:
```bash
dotenv --debug pull web-app
```

## Next Steps

Now that you're set up:

1. **Read the [Command Reference](../references/commands.md)** for detailed command usage
2. **Learn about [Configuration](configuration.md)** options
3. **Set up [CI/CD Integration](ci-cd-integration.md)**
4. **Explore [Advanced Features](../examples/advanced-usage.md)**

### Quick Reference

```bash
# Most common commands
dotenv init                      # Initialize
dotenv login                     # Authenticate
dotenv pull project              # Get secrets
dotenv push project file         # Set secrets
dotenv list projects             # List projects
dotenv pull project/prod         # Get environment
dotenv export project            # Export secrets
dotenv --help                    # Get help
```

Welcome to the DotEnv community! 🎉