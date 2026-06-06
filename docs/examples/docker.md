# Docker Integration

Run the DotEnv CLI from its official container image — no install step. This is
the natural fit for **container-native CI** (GitLab CI, Jenkins docker agents,
Tekton, Argo Workflows) and for ad-hoc `docker run` usage.

> **GitHub Actions users:** use the [GitHub Action](https://github.com/dotenvcloud/action-github)
> instead. It installs the CLI binary so it runs on Linux, macOS, **and**
> Windows runners. A Docker-based action would be Linux-only, so there isn't
> one — use the image directly only in container-native CI.

## Table of Contents

- [Images & Tags](#images--tags)
- [Architectures](#architectures)
- [Quick Start](#quick-start)
- [Pulling Secrets to a File](#pulling-secrets-to-a-file)
- [Output Formats & stdout](#output-formats--stdout)
- [Client-Managed Encryption Keys](#client-managed-encryption-keys)
- [The Non-Root User Caveat](#the-non-root-user-caveat)
- [Use in a Dockerfile](#use-in-a-dockerfile)
- [Container-Native CI](#container-native-ci)

## Images & Tags

| Registry | Image |
|----------|-------|
| Docker Hub | `dotenvcloud/cli` |
| GitHub Container Registry | `ghcr.io/dotenvcloud/cli` |

Tags:

- `:latest` and `:{version}` (e.g. `:1.2.3`) — published with each **stable**
  release.
- `:main` — rolling build of the latest `main` HEAD, rebuilt on every successful
  CI run. Use this until the first stable release exists.

The image runs as the non-root user `dotenv` (UID 1000), with the binary at
`/usr/local/bin/dotenv` and `ENTRYPOINT ["dotenv"]`.

## Architectures

Stable tags (`:latest`, `:{version}`) are multi-arch manifests covering
`linux/amd64` and `linux/arm64`. The rolling `:main` tag additionally includes
`linux/arm/v7`. Docker selects the correct image for the host automatically —
**only the runner's architecture matters**, and you never specify it explicitly.

## Quick Start

```bash
docker run --rm dotenvcloud/cli:latest version
```

Because `ENTRYPOINT` is `dotenv`, everything after the image name is passed
straight to the CLI (e.g. `version`, `pull …`, `--help`).

## Pulling Secrets to a File

Pass the API key as an environment variable and mount your working directory so
the generated file is written back to the host:

```bash
docker run --rm \
  -e DOTENV_API_KEY="$DOTENV_API_KEY" \
  -v "$PWD":/work -w /work \
  dotenvcloud/cli:latest \
  pull myproject/production/web --output=.env
```

`-w /work` makes the mounted directory the working directory so `--output=.env`
resolves to `"$PWD"/.env` on the host.

## Output Formats & stdout

```bash
# JSON to stdout (no volume mount needed)
docker run --rm -e DOTENV_API_KEY="$DOTENV_API_KEY" \
  dotenvcloud/cli:latest \
  pull myproject/production/web --format=json

# YAML to a file
docker run --rm -e DOTENV_API_KEY="$DOTENV_API_KEY" \
  -v "$PWD":/work -w /work \
  dotenvcloud/cli:latest \
  pull myproject/production/web --format=yaml --output=config.yaml
```

Supported formats: `env`, `json`, `yaml`, `shell`, `dockerfile`.

## Client-Managed Encryption Keys

For zero-knowledge (client-managed) projects, mount your key file and point
`--client-key` at it. The flag takes a **file path** that is read inside the
container — mount it read-only:

```bash
docker run --rm \
  -e DOTENV_API_KEY="$DOTENV_API_KEY" \
  -v "$PWD":/work -w /work \
  -v "$PWD/encryption.key":/key:ro \
  dotenvcloud/cli:latest \
  pull myproject/production/web --client-key=/key --output=.env
```

## The Non-Root User Caveat

The container runs as UID 1000. A host directory you mount must be writable by
that UID or the CLI cannot write the output file. On most CI runners the
workspace is world-writable and this just works. If you hit a permission error,
either make the target writable or run as your own UID:

```bash
docker run --rm -u "$(id -u):$(id -g)" \
  -e DOTENV_API_KEY="$DOTENV_API_KEY" \
  -v "$PWD":/work -w /work \
  dotenvcloud/cli:latest \
  pull myproject/production/web --output=.env
```

## Use in a Dockerfile

Copy the binary out of the image into your own build (multi-stage):

```dockerfile
# Grab the CLI binary
FROM dotenvcloud/cli:latest AS dotenv

FROM alpine:3.19
COPY --from=dotenv /usr/local/bin/dotenv /usr/local/bin/dotenv
# ... your image ...
```

## Container-Native CI

See the [CI/CD Integration guide](../guides/ci-cd-integration.md) for GitLab CI,
Jenkins, and other systems that consume the image directly via `image:` /
`docker run`.
