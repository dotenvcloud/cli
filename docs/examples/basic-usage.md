# Basic Usage Examples

Common DotEnv CLI workflows and examples for everyday use.

## Table of Contents

- [First Time Setup](#first-time-setup)
- [Daily Development Workflow](#daily-development-workflow)
- [Team Collaboration](#team-collaboration)
- [Managing Multiple Projects](#managing-multiple-projects)
- [Environment Hierarchies](#environment-hierarchies)
- [Secret Rotation](#secret-rotation)
- [Different Output Formats](#different-output-formats)
- [Variable Interpolation](#variable-interpolation)
- [Integration with Development Tools](#integration-with-development-tools)

## First Time Setup

### 1. Installation and Initialization

```bash
# Install via Homebrew (macOS)
brew tap dotenv/tap
brew install dotenv

# Initialize configuration
dotenv init

# Follow prompts:
# - Accept default API URL
# - Choose browser authentication
# - Enable telemetry (optional)
```

### 2. Authentication

```bash
# Login via browser
dotenv login

# Browser opens -> Login -> Select organizations -> Done
```

### 3. First Pull

```bash
# List your projects
dotenv list projects

# Pull secrets for a project
dotenv pull myproject

# Save to .env file
dotenv pull myproject --output=.env
```

## Daily Development Workflow

### Local Development Setup

```bash
# Morning setup
cd ~/projects/myapp
dotenv pull myproject/development --output=.env
npm start
```

### Switching Environments

```bash
# Development
dotenv pull myproject/development --output=.env.development

# Staging
dotenv pull myproject/staging --output=.env.staging

# Production (be careful!)
dotenv pull myproject/production --output=.env.production
```

### Quick Environment Switch

```bash
# Create aliases in ~/.bashrc or ~/.zshrc
alias dev-env='dotenv pull myproject/development --output=.env'
alias stage-env='dotenv pull myproject/staging --output=.env'
alias prod-env='dotenv pull myproject/production --output=.env'

# Usage
dev-env
npm start
```

### Working with Docker

```bash
# Pull and run with Docker
dotenv pull myproject/development --output=.env
docker run --env-file=.env myapp

# Or export as Dockerfile format
dotenv export myproject/development --format=dockerfile > Dockerfile.env
```

## Team Collaboration

### Sharing Project Configuration

Create `.dotenv.example` in your repository:

```env
# .dotenv.example
DATABASE_URL=
API_KEY=
SECRET_KEY=
REDIS_URL=
```

Team members run:
```bash
# Get real values
dotenv pull myproject --output=.env

# Or interactively fill from example
cp .dotenv.example .env
# Edit .env with real values
```

### Onboarding New Team Members

1. **Admin**: Grant access in DotEnv dashboard
2. **New Member**: Install and setup
   ```bash
   brew install dotenv
   dotenv init
   dotenv login
   ```
3. **Pull Secrets**:
   ```bash
   dotenv pull project-name --output=.env
   ```

### Team Workflow Example

```bash
# Backend developer
cd backend/
dotenv pull myproject/backend/development --output=.env

# Frontend developer  
cd frontend/
dotenv pull myproject/frontend/development --output=.env

# DevOps engineer
dotenv pull myproject/infrastructure/production --output=terraform.tfvars
```

## Managing Multiple Projects

### Project Structure

```
~/projects/
├── frontend/
│   └── .env          # via: dotenv pull frontend
├── backend/
│   └── .env          # via: dotenv pull backend
└── mobile/
    └── .env          # via: dotenv pull mobile
```

### Batch Operations

```bash
#!/bin/bash
# update-all-envs.sh

projects=("frontend" "backend" "mobile")

for project in "${projects[@]}"; do
  echo "Updating $project..."
  cd ~/projects/$project
  dotenv pull $project --output=.env
done
```

### Monorepo Setup

```bash
# Root secrets
dotenv pull myapp/shared --output=.env.shared

# Service-specific secrets
dotenv pull myapp/api --output=services/api/.env
dotenv pull myapp/web --output=services/web/.env
dotenv pull myapp/worker --output=services/worker/.env
```

## Environment Hierarchies

### Setting Up Hierarchy

```bash
# Base configuration (shared across all environments)
dotenv push myproject .env.base

# Target-specific (e.g., AWS regions)
dotenv push myproject/us-east-1 .env.us-east-1
dotenv push myproject/eu-west-1 .env.eu-west-1

# Environment-specific
dotenv push myproject/us-east-1/production .env.production
dotenv push myproject/us-east-1/staging .env.staging
```

### Using Hierarchy

```bash
# Gets: base + us-east-1 overrides + production overrides
dotenv pull myproject/us-east-1/production

# Inheritance example:
# Base:       DATABASE_HOST=localhost, PORT=3000
# us-east-1:  DATABASE_HOST=us-db.example.com
# production: PORT=443
# Result:     DATABASE_HOST=us-db.example.com, PORT=443
```

### Regional Deployment

```bash
# Deploy to US region
dotenv pull myapp/us-east-1/production --output=.env
./deploy.sh us-east-1

# Deploy to EU region
dotenv pull myapp/eu-west-1/production --output=.env
./deploy.sh eu-west-1
```

## Secret Rotation

### Rotating API Keys

```bash
# 1. Update in DotEnv dashboard or via API
# 2. Pull new values
dotenv pull myproject --output=.env.new

# 3. Test with new values
cp .env .env.backup
cp .env.new .env
npm test

# 4. If tests pass, deploy
rm .env.backup .env.new
```

### Bulk Updates

```bash
# Export current secrets
dotenv pull myproject --format=json > secrets.json

# Edit secrets.json

# Push updated secrets
dotenv push myproject secrets.json --format=json --force
```

### Automated Rotation Script

```bash
#!/bin/bash
# rotate-secrets.sh

# Generate new API key
NEW_API_KEY=$(openssl rand -hex 32)

# Update in application
dotenv pull myproject --output=.env
sed -i "s/API_KEY=.*/API_KEY=$NEW_API_KEY/" .env
dotenv push myproject .env --force

# Update in external service
curl -X POST https://api.external.com/rotate \
  -H "Authorization: Bearer $OLD_API_KEY" \
  -d "new_key=$NEW_API_KEY"
```

## Different Output Formats

### JSON Format

```bash
# Pull as JSON
dotenv pull myproject --format=json

# Output:
{
  "DATABASE_URL": "postgres://...",
  "API_KEY": "sk_test_...",
  "NODE_ENV": "production"
}

# Use with jq
dotenv pull myproject --format=json | jq '.DATABASE_URL'
```

### Shell Format

```bash
# Generate shell exports
dotenv pull myproject --format=shell > env.sh

# Contents of env.sh:
export DATABASE_URL="postgres://..."
export API_KEY="sk_test_..."
export NODE_ENV="production"

# Use in scripts:
source env.sh
```

### Docker Format

```bash
# Generate Dockerfile ENV commands
dotenv pull myproject --format=dockerfile

# Output:
ENV DATABASE_URL=postgres://...
ENV API_KEY=sk_test_...
ENV NODE_ENV=production

# Use in Dockerfile:
FROM node:16
$(dotenv pull myproject --format=dockerfile)
COPY . .
CMD ["npm", "start"]
```

### YAML Format

```bash
# Export as YAML
dotenv pull myproject --format=yaml

# Output:
DATABASE_URL: postgres://...
API_KEY: sk_test_...
NODE_ENV: production

# Use with Kubernetes
dotenv export myproject --format=yaml > configmap.yaml
```

## Variable Interpolation

### Setting Up Variables

```bash
# Create base variables
cat > .env.base << EOF
API_BASE_URL=https://api.example.com
API_VERSION=v1
API_ENDPOINT=\${API_BASE_URL}/\${API_VERSION}
AUTH_ENDPOINT=\${API_ENDPOINT}/auth
EOF

dotenv push myproject .env.base
```

### Using Interpolation

```bash
# Pull with interpolation
dotenv pull myproject --resolve

# Result:
# API_BASE_URL=https://api.example.com
# API_VERSION=v1
# API_ENDPOINT=https://api.example.com/v1
# AUTH_ENDPOINT=https://api.example.com/v1/auth
```

### Complex Interpolation

```bash
# Multi-level interpolation
cat > .env << EOF
DOMAIN=example.com
SUBDOMAIN=api
BASE_URL=https://\${SUBDOMAIN}.\${DOMAIN}
API_V1=\${BASE_URL}/v1
API_V2=\${BASE_URL}/v2
WEBSOCKET_URL=wss://\${SUBDOMAIN}.\${DOMAIN}/ws
EOF

dotenv push myproject .env
dotenv pull myproject --resolve
```

## Integration with Development Tools

### VS Code

Add to `.vscode/tasks.json`:

```json
{
  "version": "2.0.0",
  "tasks": [
    {
      "label": "Pull Dev Secrets",
      "type": "shell",
      "command": "dotenv pull myproject/development --output=.env",
      "problemMatcher": []
    },
    {
      "label": "Pull Staging Secrets",
      "type": "shell",
      "command": "dotenv pull myproject/staging --output=.env",
      "problemMatcher": []
    }
  ]
}
```

### Git Hooks

`.git/hooks/post-checkout`:

```bash
#!/bin/bash
# Auto-update env on branch change

BRANCH=$(git rev-parse --abbrev-ref HEAD)

case $BRANCH in
  main|master)
    dotenv pull myproject/production --output=.env
    ;;
  develop)
    dotenv pull myproject/development --output=.env
    ;;
  staging)
    dotenv pull myproject/staging --output=.env
    ;;
esac
```

### Makefile

```makefile
.PHONY: env test run deploy

# Pull environment
env:
	@dotenv pull myproject/$(ENV) --output=.env

# Development shortcuts
dev:
	@$(MAKE) env ENV=development
	npm run dev

staging:
	@$(MAKE) env ENV=staging
	npm run start

# Test with fresh secrets
test: 
	@$(MAKE) env ENV=test
	npm test

# Deploy workflow
deploy:
	@$(MAKE) env ENV=production
	npm run build
	npm run deploy
```

### NPM Scripts

`package.json`:

```json
{
  "scripts": {
    "predev": "dotenv pull myproject/development --output=.env",
    "dev": "nodemon index.js",
    "pretest": "dotenv pull myproject/test --output=.env",
    "test": "jest",
    "prebuild": "dotenv pull myproject/production --output=.env",
    "build": "webpack --mode production"
  }
}
```

### Docker Compose

```yaml
version: '3.8'

services:
  app:
    build: .
    env_file:
      - .env
    command: |
      sh -c "
        dotenv pull myproject/development --output=.env &&
        npm start
      "
```

## Debugging

### Verbose Output

```bash
# Enable debug mode
dotenv --debug pull myproject

# Shows:
# - API calls being made
# - Response times
# - Decryption process
# - Error details
```

### Dry Run

```bash
# Check what would be pulled
dotenv pull myproject --dry-run

# Check what would be pushed
dotenv push myproject .env --dry-run
```

### Troubleshooting Common Issues

```bash
# Check connectivity
dotenv config test

# Verify authentication
dotenv whoami

# List available resources
dotenv list projects
dotenv list targets myproject
dotenv list environments myproject/production
```

## Security Best Practices

### Never Commit Secrets

`.gitignore`:
```
.env
.env.*
!.env.example
```

### Use Read-Only Tokens in CI

```bash
# Create read-only token in dashboard
# Use in CI:
export DOTENV_API_KEY="${{ secrets.DOTENV_RO_KEY }}"
dotenv pull myproject/production
```

### Audit Access

```bash
# Check who has access
dotenv list members myproject

# Rotate secrets after team changes
dotenv rotate myproject
```

### Secure Local Development

```bash
# Use temporary files
TMPDIR=$(mktemp -d)
dotenv pull myproject --output=$TMPDIR/.env
source $TMPDIR/.env
rm -rf $TMPDIR
```

## Next Steps

- Explore [Docker Integration](docker.md)
- Learn about [CI/CD Integration](../guides/ci-cd-integration.md)
- Read about [Advanced Patterns](advanced-usage.md)