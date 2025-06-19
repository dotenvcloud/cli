# Migration Guide

This guide helps you migrate to DotEnv CLI from other secret management solutions.

## Table of Contents

- [From .env Files](#from-env-files)
- [From HashiCorp Vault](#from-hashicorp-vault)
- [From AWS Secrets Manager](#from-aws-secrets-manager)
- [From Azure Key Vault](#from-azure-key-vault)
- [From Google Secret Manager](#from-google-secret-manager)
- [From Kubernetes Secrets](#from-kubernetes-secrets)
- [From Doppler](#from-doppler)
- [From Chamber](#from-chamber)
- [Bulk Migration Strategies](#bulk-migration-strategies)
- [Best Practices](#best-practices)

## From .env Files

The easiest migration - DotEnv CLI works natively with .env files.

### Single File Migration

```bash
# Push existing .env file
dotenv push myproject .env

# Push to specific environment
dotenv push myproject/production .env.production
```

### Multiple Environment Files

```bash
# Push environment hierarchy
dotenv push myproject \
  --project=.env.shared \
  --target=.env.production \
  --env=.env.production.local
```

### Directory of .env Files

```bash
#!/bin/bash
# migrate-env-files.sh

for file in .env.*; do
  env_name=${file#.env.}
  dotenv push myproject/$env_name $file
done
```

## From HashiCorp Vault

### Export from Vault

```bash
# Export single path
vault kv get -format=json secret/myapp | \
  jq -r '.data.data | to_entries[] | "\(.key)=\(.value)"' > .env

# Export multiple paths
for path in myapp/dev myapp/prod; do
  vault kv get -format=json secret/$path | \
    jq -r '.data.data | to_entries[] | "\(.key)=\(.value)"' > .env.${path##*/}
done
```

### Import to DotEnv

```bash
# Push exported files
dotenv push myproject/development .env.dev
dotenv push myproject/production .env.prod
```

### Automated Migration Script

```bash
#!/bin/bash
# vault-to-dotenv.sh

VAULT_PATHS=("secret/myapp/dev" "secret/myapp/staging" "secret/myapp/prod")
DOTENV_PROJECT="myapp"

for path in "${VAULT_PATHS[@]}"; do
  env=${path##*/}
  
  # Export from Vault
  vault kv get -format=json "$path" | \
    jq -r '.data.data | to_entries[] | "\(.key)=\(.value)"' > ".env.$env"
  
  # Push to DotEnv
  dotenv push "$DOTENV_PROJECT/$env" ".env.$env"
  
  # Clean up
  rm ".env.$env"
done
```

## From AWS Secrets Manager

### Using AWS CLI

```bash
# Export single secret
aws secretsmanager get-secret-value \
  --secret-id myapp/production \
  --query SecretString \
  --output text | \
  jq -r 'to_entries[] | "\(.key)=\(.value)"' > .env

# Export multiple secrets
for secret in myapp/dev myapp/staging myapp/prod; do
  aws secretsmanager get-secret-value \
    --secret-id "$secret" \
    --query SecretString \
    --output text | \
    jq -r 'to_entries[] | "\(.key)=\(.value)"' > ".env.${secret##*/}"
done
```

### Import to DotEnv

```bash
# Push to DotEnv
dotenv push myproject/production .env.prod
```

### Migration Script with Tags

```bash
#!/bin/bash
# aws-secrets-to-dotenv.sh

# Find all secrets with specific tag
secrets=$(aws secretsmanager list-secrets \
  --filters Key=tag-key,Values=app Key=tag-value,Values=myapp \
  --query 'SecretList[].Name' \
  --output json | jq -r '.[]')

for secret in $secrets; do
  env=${secret##*/}
  
  # Export secret
  aws secretsmanager get-secret-value \
    --secret-id "$secret" \
    --query SecretString \
    --output text | \
    jq -r 'to_entries[] | "\(.key)=\(.value)"' > ".env.$env"
  
  # Push to DotEnv
  dotenv push "myproject/$env" ".env.$env"
  
  rm ".env.$env"
done
```

## From Azure Key Vault

### Using Azure CLI

```bash
# Export all secrets from vault
vault_name="myapp-vault"
secrets=$(az keyvault secret list --vault-name $vault_name --query "[].name" -o tsv)

> .env
for secret in $secrets; do
  value=$(az keyvault secret show --vault-name $vault_name --name $secret --query "value" -o tsv)
  echo "${secret}=${value}" >> .env
done
```

### Import to DotEnv

```bash
dotenv push myproject .env
```

### Environment-based Migration

```bash
#!/bin/bash
# azure-to-dotenv.sh

ENVIRONMENTS=("dev" "staging" "prod")
PROJECT="myapp"

for env in "${ENVIRONMENTS[@]}"; do
  vault_name="${PROJECT}-${env}-vault"
  
  # List and export secrets
  > ".env.$env"
  secrets=$(az keyvault secret list --vault-name $vault_name --query "[].name" -o tsv)
  
  for secret in $secrets; do
    value=$(az keyvault secret show \
      --vault-name $vault_name \
      --name $secret \
      --query "value" -o tsv)
    echo "${secret}=${value}" >> ".env.$env"
  done
  
  # Push to DotEnv
  dotenv push "$PROJECT/$env" ".env.$env"
  
  rm ".env.$env"
done
```

## From Google Secret Manager

### Using gcloud CLI

```bash
# Export secrets with labels
project_id="my-project"
app_label="app=myapp"

# List secrets
secrets=$(gcloud secrets list --project=$project_id --filter="labels.$app_label" --format="value(name)")

> .env
for secret in $secrets; do
  value=$(gcloud secrets versions access latest --secret=$secret --project=$project_id)
  echo "${secret}=${value}" >> .env
done
```

### Import to DotEnv

```bash
dotenv push myproject .env
```

### Batch Migration Script

```bash
#!/bin/bash
# gcp-secrets-to-dotenv.sh

PROJECT_ID="my-project"
ENVIRONMENTS=("dev" "staging" "prod")

for env in "${ENVIRONMENTS[@]}"; do
  # Export secrets with environment label
  secrets=$(gcloud secrets list \
    --project=$PROJECT_ID \
    --filter="labels.env=$env" \
    --format="value(name)")
  
  > ".env.$env"
  for secret in $secrets; do
    value=$(gcloud secrets versions access latest \
      --secret=$secret \
      --project=$PROJECT_ID)
    # Remove environment prefix if present
    key=${secret#${env}_}
    echo "${key}=${value}" >> ".env.$env"
  done
  
  # Push to DotEnv
  dotenv push "myproject/$env" ".env.$env"
  
  rm ".env.$env"
done
```

## From Kubernetes Secrets

### Export from Kubernetes

```bash
# Export single secret
kubectl get secret myapp-secrets -o json | \
  jq -r '.data | to_entries[] | "\(.key)=\(.value | @base64d)"' > .env

# Export by namespace
namespaces=("dev" "staging" "prod")
for ns in "${namespaces[@]}"; do
  kubectl get secret myapp-secrets -n $ns -o json | \
    jq -r '.data | to_entries[] | "\(.key)=\(.value | @base64d)"' > ".env.$ns"
done
```

### Import to DotEnv

```bash
# Push to corresponding environments
dotenv push myproject/development .env.dev
dotenv push myproject/staging .env.staging
dotenv push myproject/production .env.prod
```

### ConfigMap Migration

```bash
# Export ConfigMaps
kubectl get configmap myapp-config -o json | \
  jq -r '.data | to_entries[] | "\(.key)=\(.value)"' >> .env
```

## From Doppler

### Export from Doppler

```bash
# Export using Doppler CLI
doppler secrets download --no-file --format env > .env

# Export multiple environments
for env in dev staging prod; do
  doppler secrets download \
    --project myapp \
    --config $env \
    --no-file \
    --format env > ".env.$env"
done
```

### Import to DotEnv

```bash
# Push all environments
dotenv push myproject/development .env.dev
dotenv push myproject/staging .env.staging
dotenv push myproject/production .env.prod
```

## From Chamber

### Export from Chamber

```bash
# Export service secrets
chamber export myapp/production > .env.json

# Convert to env format
cat .env.json | jq -r 'to_entries[] | "\(.key)=\(.value)"' > .env
```

### Import to DotEnv

```bash
dotenv push myproject/production .env
```

### Batch Migration

```bash
#!/bin/bash
# chamber-to-dotenv.sh

SERVICES=("myapp/dev" "myapp/staging" "myapp/prod")

for service in "${SERVICES[@]}"; do
  env=${service##*/}
  
  # Export from Chamber
  chamber export $service | \
    jq -r 'to_entries[] | "\(.key)=\(.value)"' > ".env.$env"
  
  # Push to DotEnv
  dotenv push "myproject/$env" ".env.$env"
  
  rm ".env.$env"
done
```

## Bulk Migration Strategies

### Parallel Migration

```bash
#!/bin/bash
# parallel-migration.sh

migrate_env() {
  local source=$1
  local env=$2
  local project=$3
  
  # Export from source (customize per source type)
  export_from_source "$source" "$env" > ".env.$env"
  
  # Push to DotEnv
  dotenv push "$project/$env" ".env.$env"
  
  rm ".env.$env"
}

# Run migrations in parallel
export -f migrate_env
parallel -j 4 migrate_env ::: \
  "vault" "aws" "azure" ::: \
  "dev" "staging" "prod" ::: \
  "myproject"
```

### Incremental Migration

```bash
#!/bin/bash
# incremental-migration.sh

# Phase 1: Development environment
echo "Migrating development environment..."
dotenv push myproject/development .env.dev

# Test
dotenv pull myproject/development --output=.env.test
diff .env.dev .env.test

# Phase 2: Staging
echo "Migrating staging environment..."
dotenv push myproject/staging .env.staging

# Phase 3: Production (with backup)
echo "Creating production backup..."
cp .env.prod .env.prod.backup
dotenv push myproject/production .env.prod
```

### Validation Script

```bash
#!/bin/bash
# validate-migration.sh

validate_env() {
  local env=$1
  local original=$2
  
  # Pull from DotEnv
  dotenv pull "myproject/$env" --output=".env.$env.new"
  
  # Sort and compare
  sort "$original" > "$original.sorted"
  sort ".env.$env.new" > ".env.$env.new.sorted"
  
  if diff -q "$original.sorted" ".env.$env.new.sorted" > /dev/null; then
    echo "✓ $env migration successful"
  else
    echo "✗ $env migration failed - differences found:"
    diff "$original.sorted" ".env.$env.new.sorted"
  fi
  
  rm -f "$original.sorted" ".env.$env.new.sorted" ".env.$env.new"
}

# Validate all environments
validate_env "dev" ".env.dev"
validate_env "staging" ".env.staging"
validate_env "prod" ".env.prod"
```

## Best Practices

### 1. Plan Your Hierarchy

Before migration, design your secret hierarchy:

```
myproject/
├── shared secrets (database URLs, API endpoints)
├── development/
│   └── dev-specific secrets
├── staging/
│   └── staging-specific secrets
└── production/
    ├── us-east-1/
    │   └── region-specific secrets
    └── eu-west-1/
        └── region-specific secrets
```

### 2. Migration Checklist

- [ ] Audit existing secrets
- [ ] Design hierarchy structure
- [ ] Create DotEnv projects
- [ ] Export from current system
- [ ] Validate exports
- [ ] Push to DotEnv
- [ ] Verify migration
- [ ] Update CI/CD pipelines
- [ ] Update documentation
- [ ] Decommission old system

### 3. Security During Migration

```bash
# Use temporary directory with restricted permissions
TEMP_DIR=$(mktemp -d -t migration.XXXXXX)
chmod 700 "$TEMP_DIR"
cd "$TEMP_DIR"

# Perform migration
# ... migration commands ...

# Clean up
cd ..
rm -rf "$TEMP_DIR"
```

### 4. Rollback Plan

```bash
# Before migration, create backups
for env in dev staging prod; do
  cp ".env.$env" ".env.$env.backup.$(date +%Y%m%d)"
done

# If rollback needed
for env in dev staging prod; do
  cp ".env.$env.backup.$(date +%Y%m%d)" ".env.$env"
done
```

### 5. Post-Migration

1. **Update Access Controls**: Configure team permissions in DotEnv
2. **Rotate Secrets**: Generate new values for sensitive secrets
3. **Update Documentation**: Document new secret management process
4. **Train Team**: Ensure everyone knows the new workflow
5. **Monitor Usage**: Track adoption and issues

## Common Migration Patterns

### Environment Promotion

```bash
# Promote dev to staging
dotenv pull myproject/development --output=.env
dotenv push myproject/staging .env

# Promote staging to production (with review)
dotenv pull myproject/staging --output=.env.staging
# Review and modify as needed
dotenv push myproject/production .env.staging
```

### Secret Rotation

```bash
# Export current secrets
dotenv pull myproject/production --output=.env.old

# Generate new values (example)
sed -i 's/API_KEY=.*/API_KEY=new_generated_key/' .env.old

# Push rotated secrets
dotenv push myproject/production .env.old --force
```

## Next Steps

1. Set up [CI/CD Integration](guides/ci-cd-integration.md)
2. Configure [Team Access](guides/team-management.md)
3. Implement [Secret Rotation](guides/secret-rotation.md)
4. Review [Security Best Practices](guides/security.md)