# syntax=docker/dockerfile:1.4
# Build stage
FROM golang:1.23-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /build

# Private SDK module needs a token to fetch. Pass via BuildKit secret:
#   docker build --secret id=gh_pat,env=GH_PAT .
# In CI this is wired through docker/build-push-action's `secrets:` input.
ENV GOPRIVATE=github.com/lostlink/*

COPY go.mod go.sum ./
RUN --mount=type=secret,id=gh_pat,required=true \
    git config --global url."https://x-access-token:$(cat /run/secrets/gh_pat)@github.com/".insteadOf "https://github.com/" && \
    go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s -X main.version=$(git describe --tags --always)" \
    -o dotenv .

# Final stage
FROM alpine:3.19

RUN apk add --no-cache ca-certificates

# Create non-root user
RUN addgroup -g 1000 -S dotenv && \
    adduser -u 1000 -S dotenv -G dotenv

# Copy binary
COPY --from=builder /build/dotenv /usr/local/bin/dotenv

# Create config directory
RUN mkdir -p /home/dotenv/.config/dotenv && \
    chown -R dotenv:dotenv /home/dotenv

USER dotenv
WORKDIR /home/dotenv

ENTRYPOINT ["dotenv"]
CMD ["--help"]
