# Build stage
FROM golang:1.23-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s -X main.version=$(git describe --tags --always)" \
    -o dotenv ./cmd/dotenv

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