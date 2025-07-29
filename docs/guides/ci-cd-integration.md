# CI/CD Integration Guide

Integrate DotEnv CLI into your continuous integration and deployment pipelines.

## Table of Contents

- [Overview](#overview)
- [General Principles](#general-principles)
- [GitHub Actions](#github-actions)
- [GitLab CI](#gitlab-ci)
- [Jenkins](#jenkins)
- [CircleCI](#circleci)
- [Azure DevOps](#azure-devops)
- [AWS CodeBuild](#aws-codebuild)
- [Google Cloud Build](#google-cloud-build)
- [Bitbucket Pipelines](#bitbucket-pipelines)
- [Security Best Practices](#security-best-practices)
- [Troubleshooting](#troubleshooting)

## Overview

The DotEnv CLI is designed to work seamlessly in CI/CD environments:
- Non-interactive mode by default
- Exit codes for scripting
- Service account support
- Read-only access options
- Machine-readable output formats

## General Principles

### Authentication

1. **Create a Service Account** in DotEnv dashboard
2. **Generate API Token** with minimal required permissions
3. **Store Token** in CI/CD secret storage
4. **Set Environment Variable** in pipeline

### Best Practices

- Use read-only tokens when possible
- Scope tokens to specific organizations/projects
- Rotate tokens regularly
- Never log secret values
- Clean up secrets after use
- Use non-interactive mode

## GitHub Actions

### Basic Setup

```yaml
name: Deploy

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  deploy:
    runs-on: ubuntu-latest
    
    steps:
    - uses: actions/checkout@v3
    
    - name: Install DotEnv CLI
      run: |
        curl -sSL https://dotenv.cloud/install.sh | bash
        echo "$HOME/.local/bin" >> $GITHUB_PATH
    
    - name: Pull Secrets
      env:
        DOTENV_API_KEY: ${{ secrets.DOTENV_API_KEY }}
        DOTENV_ORGANIZATION: my-org
      run: |
        dotenv pull myproject/production --output=.env
    
    - name: Build Application
      run: |
        npm install
        npm run build
    
    - name: Run Tests
      run: |
        npm test
    
    - name: Deploy
      if: github.ref == 'refs/heads/main'
      run: |
        npm run deploy
    
    - name: Clean Up Secrets
      if: always()
      run: rm -f .env
```

### Reusable Action

Create `.github/actions/dotenv-pull/action.yml`:

```yaml
name: 'DotEnv Pull'
description: 'Pull secrets from DotEnv'

inputs:
  api-key:
    description: 'DotEnv API Key'
    required: true
  project:
    description: 'Project path'
    required: true
  output:
    description: 'Output file'
    default: '.env'
  organization:
    description: 'Organization name'
    required: false

runs:
  using: 'composite'
  steps:
    - name: Install DotEnv CLI
      shell: bash
      run: |
        if ! command -v dotenv &> /dev/null; then
          curl -sSL https://dotenv.cloud/install.sh | bash
          echo "$HOME/.local/bin" >> $GITHUB_PATH
        fi
    
    - name: Pull Secrets
      shell: bash
      env:
        DOTENV_API_KEY: ${{ inputs.api-key }}
        DOTENV_ORGANIZATION: ${{ inputs.organization }}
      run: |
        dotenv pull ${{ inputs.project }} --output=${{ inputs.output }}
```

Usage in workflow:

```yaml
- uses: ./.github/actions/dotenv-pull
  with:
    api-key: ${{ secrets.DOTENV_API_KEY }}
    project: myproject/production
    output: .env
```

### Matrix Builds

```yaml
strategy:
  matrix:
    environment: [staging, production]
    region: [us-east-1, eu-west-1]

steps:
  - name: Pull Secrets
    env:
      DOTENV_API_KEY: ${{ secrets.DOTENV_API_KEY }}
    run: |
      dotenv pull myproject/${{ matrix.region }}/${{ matrix.environment }} --output=.env
  
  - name: Deploy to Region
    run: |
      ./deploy.sh ${{ matrix.region }} ${{ matrix.environment }}
```

### Caching

```yaml
- name: Cache DotEnv CLI
  uses: actions/cache@v3
  with:
    path: ~/.local/bin/dotenv
    key: ${{ runner.os }}-dotenv-cli-v1.0.0
```

## GitLab CI

### Basic Setup

`.gitlab-ci.yml`:

```yaml
variables:
  DOTENV_VERSION: "latest"

before_script:
  - |
    if ! command -v dotenv &> /dev/null; then
      curl -sSL https://dotenv.cloud/install.sh | bash
      export PATH="$HOME/.local/bin:$PATH"
    fi

stages:
  - build
  - test
  - deploy

build:
  stage: build
  script:
    - dotenv pull $CI_PROJECT_NAME/$CI_COMMIT_REF_NAME --output=.env
    - docker build -t $CI_REGISTRY_IMAGE:$CI_COMMIT_SHA .
  after_script:
    - rm -f .env

test:
  stage: test
  script:
    - dotenv pull $CI_PROJECT_NAME/test --output=.env
    - npm install
    - npm test
  after_script:
    - rm -f .env

deploy:
  stage: deploy
  only:
    - main
  script:
    - dotenv pull $CI_PROJECT_NAME/production --output=.env
    - |
      kubectl create secret generic app-secrets \
        --from-env-file=.env \
        --dry-run=client -o yaml | kubectl apply -f -
    - kubectl set image deployment/app app=$CI_REGISTRY_IMAGE:$CI_COMMIT_SHA
  after_script:
    - rm -f .env
  environment:
    name: production
    url: https://app.example.com
```

### GitLab CI/CD Variables

Set in Project Settings > CI/CD > Variables:

```
DOTENV_API_KEY = dotenv_xxx_yyy (masked, protected)
DOTENV_ORGANIZATION = my-org
```

### Environment-specific Deployments

```yaml
.deploy_template: &deploy_template
  script:
    - dotenv pull $CI_PROJECT_NAME/$CI_ENVIRONMENT_NAME --output=.env
    - ./deploy.sh
  after_script:
    - rm -f .env

deploy_staging:
  <<: *deploy_template
  stage: deploy
  environment:
    name: staging
    url: https://staging.example.com
  only:
    - develop

deploy_production:
  <<: *deploy_template
  stage: deploy
  environment:
    name: production
    url: https://app.example.com
  only:
    - main
```

## Jenkins

### Jenkinsfile

```groovy
pipeline {
  agent any
  
  environment {
    DOTENV_API_KEY = credentials('dotenv-api-key')
    DOTENV_ORGANIZATION = 'my-org'
  }
  
  stages {
    stage('Setup') {
      steps {
        sh '''
          if ! command -v dotenv &> /dev/null; then
            curl -sSL https://dotenv.cloud/install.sh | bash
            export PATH="$HOME/.local/bin:$PATH"
          fi
        '''
      }
    }
    
    stage('Pull Secrets') {
      steps {
        script {
          def environment = env.BRANCH_NAME == 'main' ? 'production' : 'staging'
          sh "dotenv pull myproject/${environment} --output=.env"
        }
      }
    }
    
    stage('Build') {
      steps {
        sh '''
          npm install
          npm run build
        '''
      }
    }
    
    stage('Test') {
      steps {
        sh 'npm test'
      }
    }
    
    stage('Deploy') {
      when {
        branch 'main'
      }
      steps {
        sh '''
          dotenv pull myproject/production --output=.env.prod
          ./deploy.sh production
        '''
      }
    }
  }
  
  post {
    always {
      sh 'rm -f .env .env.*'
    }
  }
}
```

### Shared Library

`vars/dotenvPull.groovy`:

```groovy
def call(String project, String outputFile = '.env') {
  sh """
    if ! command -v dotenv &> /dev/null; then
      curl -sSL https://dotenv.cloud/install.sh | bash
      export PATH="\$HOME/.local/bin:\$PATH"
    fi
    dotenv pull ${project} --output=${outputFile}
  """
}
```

Usage:
```groovy
dotenvPull('myproject/production')
```

## CircleCI

`.circleci/config.yml`:

```yaml
version: 2.1

orbs:
  dotenv: dotenv/cli@1.0.0

executors:
  node:
    docker:
      - image: cimg/node:16.0
    environment:
      DOTENV_ORGANIZATION: my-org

jobs:
  build:
    executor: node
    steps:
      - checkout
      
      - run:
          name: Install DotEnv CLI
          command: |
            curl -sSL https://dotenv.cloud/install.sh | bash
            echo 'export PATH="$HOME/.local/bin:$PATH"' >> $BASH_ENV
      
      - run:
          name: Pull Secrets
          command: |
            dotenv pull myproject/$CIRCLE_BRANCH --output=.env
      
      - run:
          name: Install Dependencies
          command: npm install
      
      - run:
          name: Build
          command: npm run build
      
      - run:
          name: Test
          command: npm test
      
      - run:
          name: Cleanup
          when: always
          command: rm -f .env

  deploy:
    executor: node
    steps:
      - checkout
      
      - run:
          name: Install DotEnv CLI
          command: |
            curl -sSL https://dotenv.cloud/install.sh | bash
            echo 'export PATH="$HOME/.local/bin:$PATH"' >> $BASH_ENV
      
      - run:
          name: Deploy
          command: |
            dotenv pull myproject/production --output=.env
            npm run deploy
      
      - run:
          name: Cleanup
          when: always
          command: rm -f .env

workflows:
  main:
    jobs:
      - build:
          context: dotenv-context
      - deploy:
          requires:
            - build
          filters:
            branches:
              only: main
          context: dotenv-context
```

## Azure DevOps

`azure-pipelines.yml`:

```yaml
trigger:
  - main
  - develop

pool:
  vmImage: 'ubuntu-latest'

variables:
  - group: dotenv-secrets

steps:
  - task: Bash@3
    displayName: 'Install DotEnv CLI'
    inputs:
      targetType: 'inline'
      script: |
        curl -sSL https://dotenv.cloud/install.sh | bash
        echo "##vso[task.prependpath]$HOME/.local/bin"

  - task: Bash@3
    displayName: 'Pull Secrets'
    env:
      DOTENV_API_KEY: $(DOTENV_API_KEY)
      DOTENV_ORGANIZATION: $(DOTENV_ORGANIZATION)
    inputs:
      targetType: 'inline'
      script: |
        if [ "$(Build.SourceBranchName)" = "main" ]; then
          ENV="production"
        else
          ENV="staging"
        fi
        dotenv pull myproject/$ENV --output=.env

  - task: NodeTool@0
    inputs:
      versionSpec: '16.x'
    displayName: 'Install Node.js'

  - script: |
      npm install
      npm run build
    displayName: 'Build'

  - script: npm test
    displayName: 'Test'

  - task: Docker@2
    condition: eq(variables['Build.SourceBranch'], 'refs/heads/main')
    displayName: 'Build and Push Docker Image'
    inputs:
      command: 'buildAndPush'
      repository: '$(dockerRepository)'
      dockerfile: '**/Dockerfile'
      tags: |
        $(Build.BuildId)
        latest

  - task: Bash@3
    displayName: 'Cleanup'
    condition: always()
    inputs:
      targetType: 'inline'
      script: rm -f .env
```

## AWS CodeBuild

`buildspec.yml`:

```yaml
version: 0.2

env:
  secrets-manager:
    DOTENV_API_KEY: dotenv:api-key
  variables:
    DOTENV_ORGANIZATION: my-org

phases:
  install:
    runtime-versions:
      nodejs: 16
    commands:
      - echo "Installing DotEnv CLI..."
      - curl -sSL https://dotenv.cloud/install.sh | bash
      - export PATH="$HOME/.local/bin:$PATH"
  
  pre_build:
    commands:
      - echo "Pulling secrets..."
      - |
        if [ "$CODEBUILD_WEBHOOK_HEAD_REF" = "refs/heads/main" ]; then
          ENV="production"
        else
          ENV="staging"
        fi
      - dotenv pull myproject/$ENV --output=.env
      - echo "Installing dependencies..."
      - npm install
  
  build:
    commands:
      - echo "Building application..."
      - npm run build
      - echo "Building Docker image..."
      - docker build -t $IMAGE_REPO_NAME:$IMAGE_TAG .
  
  post_build:
    commands:
      - echo "Cleaning up secrets..."
      - rm -f .env
      - echo "Pushing Docker image..."
      - docker push $IMAGE_REPO_NAME:$IMAGE_TAG
      - echo "Updating ECS service..."
      - |
        aws ecs update-service \
          --cluster $ECS_CLUSTER \
          --service $ECS_SERVICE \
          --force-new-deployment

artifacts:
  files:
    - '**/*'
```

## Google Cloud Build

`cloudbuild.yaml`:

```yaml
steps:
  # Install DotEnv CLI
  - name: 'gcr.io/cloud-builders/curl'
    id: 'install-dotenv'
    entrypoint: 'bash'
    args:
    - '-c'
    - |
      curl -sSL https://dotenv.cloud/install.sh | bash
      cp $$HOME/.local/bin/dotenv /workspace/dotenv

  # Pull secrets
  - name: 'gcr.io/cloud-builders/curl'
    id: 'pull-secrets'
    entrypoint: 'bash'
    env:
    - 'DOTENV_API_KEY=${_DOTENV_API_KEY}'
    - 'DOTENV_ORGANIZATION=${_DOTENV_ORGANIZATION}'
    args:
    - '-c'
    - |
      if [ "$BRANCH_NAME" = "main" ]; then
        ENV="production"
      else
        ENV="staging"
      fi
      /workspace/dotenv pull myproject/$$ENV --output=.env

  # Build application
  - name: 'gcr.io/cloud-builders/npm'
    id: 'npm-install'
    args: ['install']

  - name: 'gcr.io/cloud-builders/npm'
    id: 'npm-build'
    args: ['run', 'build']

  # Build Docker image
  - name: 'gcr.io/cloud-builders/docker'
    id: 'docker-build'
    args: ['build', '-t', 'gcr.io/$PROJECT_ID/$_SERVICE_NAME:$COMMIT_SHA', '.']

  # Push Docker image
  - name: 'gcr.io/cloud-builders/docker'
    id: 'docker-push'
    args: ['push', 'gcr.io/$PROJECT_ID/$_SERVICE_NAME:$COMMIT_SHA']

  # Deploy to Cloud Run
  - name: 'gcr.io/google.com/cloudsdktool/cloud-sdk'
    id: 'deploy'
    entrypoint: gcloud
    args:
    - 'run'
    - 'deploy'
    - '$_SERVICE_NAME'
    - '--image'
    - 'gcr.io/$PROJECT_ID/$_SERVICE_NAME:$COMMIT_SHA'
    - '--region'
    - '$_REGION'

  # Clean up
  - name: 'gcr.io/cloud-builders/gcloud'
    id: 'cleanup'
    entrypoint: 'bash'
    args: ['-c', 'rm -f .env']

substitutions:
  _DOTENV_API_KEY: '${DOTENV_API_KEY}'
  _DOTENV_ORGANIZATION: 'my-org'
  _SERVICE_NAME: 'myapp'
  _REGION: 'us-central1'

options:
  substitution_option: 'ALLOW_LOOSE'
```

## Bitbucket Pipelines

`bitbucket-pipelines.yml`:

```yaml
image: node:16

definitions:
  caches:
    dotenv: ~/.local/bin
  
  steps:
    - step: &install-dotenv
        name: Install DotEnv CLI
        caches:
          - dotenv
        script:
          - |
            if ! command -v dotenv &> /dev/null; then
              curl -sSL https://dotenv.cloud/install.sh | bash
            fi
          - export PATH="$HOME/.local/bin:$PATH"
    
    - step: &build-test
        name: Build and Test
        caches:
          - node
        script:
          - export PATH="$HOME/.local/bin:$PATH"
          - dotenv pull myproject/$BITBUCKET_BRANCH --output=.env
          - npm install
          - npm run build
          - npm test
        after-script:
          - rm -f .env

pipelines:
  default:
    - step: *install-dotenv
    - step: *build-test

  branches:
    main:
      - step: *install-dotenv
      - step: *build-test
      - step:
          name: Deploy to Production
          deployment: production
          script:
            - export PATH="$HOME/.local/bin:$PATH"
            - dotenv pull myproject/production --output=.env
            - npm run deploy
          after-script:
            - rm -f .env

    develop:
      - step: *install-dotenv
      - step: *build-test
      - step:
          name: Deploy to Staging
          deployment: staging
          script:
            - export PATH="$HOME/.local/bin:$PATH"
            - dotenv pull myproject/staging --output=.env
            - npm run deploy:staging
          after-script:
            - rm -f .env
```

## Security Best Practices

### 1. Use Service Accounts

Create dedicated service accounts with minimal permissions:

```bash
# In DotEnv Dashboard
# Create service account with read-only access to specific projects
```

### 2. Rotate Keys Regularly

```yaml
# GitHub Actions example
- name: Rotate API Key
  if: github.event.schedule == '0 0 1 * *'  # Monthly
  run: |
    # Script to rotate keys
    ./scripts/rotate-dotenv-key.sh
```

### 3. Audit Logs

```bash
# Check API key usage
dotenv audit --key=$DOTENV_API_KEY --days=30
```

### 4. Limit Scope

```bash
# Create project-specific tokens
dotenv auth create-token --project=myproject --read-only
```

### 5. Clean Up Secrets

Always remove secret files after use:

```yaml
# Always run cleanup
after_script:
  - rm -f .env .env.* || true
  - docker secret rm $(docker secret ls -q) || true
```

### 6. Secure Storage

Store API keys in CI/CD secret management:
- GitHub: Repository Secrets
- GitLab: CI/CD Variables (masked)
- Jenkins: Credentials Plugin
- CircleCI: Context Variables
- Azure DevOps: Variable Groups
- AWS: Systems Manager Parameter Store
- GCP: Secret Manager

## Troubleshooting

### Authentication Failures

```bash
# Test authentication
DOTENV_API_KEY=xxx dotenv list projects

# Common issues:
# - Expired token
# - Wrong organization
# - Insufficient permissions
```

### Network Issues

```bash
# Proxy configuration
export HTTP_PROXY=http://proxy:8080
export HTTPS_PROXY=http://proxy:8080

# Skip TLS (development only!)
export DOTENV_TLS_SKIP_VERIFY=true
```

### Debugging

```bash
# Enable debug output
DOTENV_DEBUG=true dotenv pull myproject

# Check version
dotenv version

# Verify installation
which dotenv
```

### Performance Optimization

#### Caching

```yaml
# GitHub Actions
- uses: actions/cache@v3
  with:
    path: ~/.dotenv/cache
    key: dotenv-${{ hashFiles('**/project.json') }}
```

#### Parallel Pulls

```bash
# Pull multiple projects in parallel
projects=(frontend backend api)
for project in "${projects[@]}"; do
  dotenv pull $project --output=.env.$project &
done
wait
```

#### Minimal Docker Images

```dockerfile
# Multi-stage build
FROM alpine:latest AS secrets
RUN apk add --no-cache curl bash
COPY --from=installer /usr/local/bin/dotenv /usr/local/bin/
RUN dotenv pull myproject/production --output=/secrets/.env

FROM scratch
COPY --from=builder /app/binary /
COPY --from=secrets /secrets/.env /.env
ENTRYPOINT ["/binary"]
```

## Common Patterns

### Environment Detection

```bash
# Detect environment from branch/tag
if [[ "$CI_COMMIT_REF_NAME" == "main" ]]; then
  ENV="production"
elif [[ "$CI_COMMIT_REF_NAME" == "develop" ]]; then
  ENV="staging"
elif [[ "$CI_COMMIT_TAG" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  ENV="production"
else
  ENV="development"
fi

dotenv pull myproject/$ENV --output=.env
```

### Multi-Region Deployment

```bash
# Deploy to multiple regions
REGIONS=(us-east-1 eu-west-1 ap-southeast-1)

for region in "${REGIONS[@]}"; do
  echo "Deploying to $region..."
  dotenv pull myproject/$region/production --output=.env.$region
  ./deploy.sh --region=$region --env-file=.env.$region
  rm -f .env.$region
done
```

### Conditional Secrets

```bash
# Pull different secrets based on conditions
if [[ "$DEPLOY_FEATURE_FLAGS" == "true" ]]; then
  dotenv pull myproject/feature-flags --output=.env.flags
  cat .env.flags >> .env
  rm -f .env.flags
fi
```

## Next Steps

- Review [Security Best Practices](security.md)
- Explore [Advanced Patterns](../examples/advanced-usage.md)
- Set up [Monitoring](monitoring.md)