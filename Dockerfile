# Build stage
FROM golang:1.23-alpine AS builder

# Install dependencies
RUN apk add --no-cache git make ca-certificates

# Create non-root user
RUN adduser -D -g '' appuser

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s -X main.version=docker" \
    -a -installsuffix cgo \
    -o dotenv .

# Final stage
FROM scratch

# Copy CA certificates
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy user
COPY --from=builder /etc/passwd /etc/passwd

# Copy binary
COPY --from=builder /app/dotenv /dotenv

# Use non-root user
USER appuser

ENTRYPOINT ["/dotenv"]