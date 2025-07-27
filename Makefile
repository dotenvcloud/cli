# Variables
BINARY_NAME := dotenv
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GO_VERSION := $(shell go version | cut -d' ' -f3)

# Build variables
LDFLAGS := -ldflags "\
  -X main.version=$(VERSION) \
  -X main.commit=$(COMMIT) \
  -X main.date=$(DATE) \
  -X main.goVersion=$(GO_VERSION) \
  -s -w"

# Platforms
PLATFORMS := darwin/amd64 darwin/arm64 linux/amd64 linux/arm64 windows/amd64

.PHONY: all build clean test test-unit test-integration test-compatibility test-coverage test-race test-bench install

all: clean test build

# Build for current platform
build:
	@echo "Building $(BINARY_NAME) $(VERSION) for current platform..."
	go build $(LDFLAGS) -o bin/$(BINARY_NAME) .

# Build for all platforms
build-all:
	@echo "Building for all platforms..."
	@mkdir -p bin
	@for platform in $(PLATFORMS); do \
		GOOS=$${platform%/*} GOARCH=$${platform#*/} \
		go build $(LDFLAGS) \
		-o bin/$(BINARY_NAME)-$${platform%/*}-$${platform#*/}$(if $(findstring windows,$${platform}),.exe,) .; \
		echo "Built: bin/$(BINARY_NAME)-$${platform%/*}-$${platform#*/}"; \
	done

# Install locally
install: build
	@echo "Installing $(BINARY_NAME) to /usr/local/bin..."
	@sudo cp bin/$(BINARY_NAME) /usr/local/bin/
	@echo "Installation complete!"

# Run all tests
test: test-unit test-integration
	@echo "All tests complete"

# Run unit tests only
test-unit:
	@echo "Running unit tests..."
	go test -v -timeout=10m \
		./cmd/... \
		./internal/... \
		-tags="!integration,!compatibility"

# Run integration tests
test-integration:
	@echo "Running integration tests..."
	go test -v -timeout=10m \
		./test/integration/... \
		-tags=integration

# Run compatibility tests (requires PHP and Node.js)
test-compatibility:
	@echo "Running compatibility tests..."
	go test -v -timeout=10m \
		./test/compatibility/... \
		-tags=compatibility

# Run tests with coverage report
test-coverage:
	@echo "Running tests with coverage..."
	go test -v -timeout=10m \
		-coverprofile=coverage.out \
		-covermode=atomic \
		./cmd/... ./internal/...
	@echo "Generating coverage report..."
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"
	@echo "Coverage summary:"
	@go tool cover -func=coverage.out | grep total | awk '{print "Total coverage: " $$3}'

# Run tests with race condition detection
test-race:
	@echo "Running tests with race detection..."
	go test -v -race -timeout=10m ./...

# Run benchmark tests
test-bench:
	@echo "Running benchmarks..."
	go test -bench=. -benchmem \
		./internal/crypto/... \
		./internal/formats/...

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf bin/
	@rm -f coverage.out coverage.html

# Run linters
lint:
	@echo "Running linters..."
	golangci-lint run ./...

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...
	gofumpt -w .

# Generate completions
completions:
	@echo "Generating shell completions..."
	@mkdir -p completions
	@go run . completion bash > completions/dotenv.bash
	@go run . completion zsh > completions/dotenv.zsh
	@go run . completion fish > completions/dotenv.fish

# Development build with race detector
dev:
	@echo "Building development version with race detector..."
	go build -race $(LDFLAGS) -o bin/$(BINARY_NAME) .

# Run the CLI
run: build
	./bin/$(BINARY_NAME) $(ARGS)

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy

# Verify dependencies
verify:
	@echo "Verifying dependencies..."
	go mod verify

# Update dependencies
update-deps:
	@echo "Updating dependencies..."
	go get -u ./...
	go mod tidy

# Run SDK tests
test-sdk:
	@echo "Running SDK tests..."
	@cd ../../packages/sdk-go && go test -v ./...

# Run SDK integration tests
test-sdk-integration:
	@echo "Running SDK integration tests..."
	@cd ../../packages/sdk-go && go test -v -tags=integration ./...

# Run SDK benchmarks
test-sdk-bench:
	@echo "Running SDK benchmarks..."
	@cd ../../packages/sdk-go && go test -bench=. -benchmem ./...

# Run all tests including SDK
test-all: test test-compatibility test-sdk test-sdk-integration
	@echo "All tests complete"

# Run CI pipeline locally
ci: clean build test-coverage test-race lint
	@echo "CI pipeline complete"

# Docker commands
docker-build:
	@echo "Building CLI Docker image..."
	docker build -t lostlink/dotenv-cli:local .

# Multi-platform Docker build
docker-build-multi:
	@echo "Building multi-platform CLI Docker image..."
	docker buildx build --platform linux/amd64,linux/arm64,linux/arm/v7 -t lostlink/dotenv-cli:local .

# Push Docker image to registry
docker-push:
	docker push lostlink/dotenv-cli:latest

# Pull Docker image from registry
docker-pull:
	docker pull lostlink/dotenv-cli:latest

# Test CLI in Docker
docker-test:
	@echo "Running CLI tests in Docker..."
	docker run --rm lostlink/dotenv-cli:local test

# Shell into Docker container for debugging
docker-shell:
	docker run --rm -it lostlink/dotenv-cli:local sh

# Run CLI in Docker with custom arguments
# Usage: make docker-run ARGS="login --email=user@example.com"
docker-run:
	docker run --rm -it lostlink/dotenv-cli:local $(ARGS)

# Clean Docker images
docker-clean:
	docker image prune -f
	docker rmi lostlink/dotenv-cli:local || true

# Show help
help:
	@echo "Available targets:"
	@echo ""
	@echo "Build & Test:"
	@echo "  make build          - Build for current platform"
	@echo "  make build-all      - Build for all platforms"
	@echo "  make install        - Install to /usr/local/bin"
	@echo "  make test           - Run all tests"
	@echo "  make test-unit      - Run unit tests only"
	@echo "  make test-integration - Run integration tests"
	@echo "  make test-compatibility - Run compatibility tests"
	@echo "  make test-coverage  - Run tests with coverage report"
	@echo "  make test-race      - Run tests with race detection"
	@echo "  make test-bench     - Run benchmark tests"
	@echo "  make test-sdk       - Run SDK tests"
	@echo "  make test-all       - Run all tests including SDK"
	@echo "  make ci             - Run CI pipeline locally"
	@echo ""
	@echo "Docker Commands:"
	@echo "  make docker-build   - Build Docker image"
	@echo "  make docker-build-multi - Build multi-platform image"
	@echo "  make docker-push    - Push image to registry"
	@echo "  make docker-pull    - Pull image from registry"
	@echo "  make docker-test    - Run tests in Docker"
	@echo "  make docker-shell   - Shell into container"
	@echo "  make docker-run     - Run CLI in Docker"
	@echo "  make docker-clean   - Clean Docker images"
	@echo ""
	@echo "Maintenance:"
	@echo "  make clean          - Clean build artifacts"
	@echo "  make lint           - Run linters"
	@echo "  make fmt            - Format code"
	@echo "  make completions    - Generate shell completions"
	@echo "  make dev            - Build with race detector"
	@echo "  make run ARGS=      - Build and run with arguments"
	@echo "  make deps           - Download dependencies"
	@echo "  make verify         - Verify dependencies"
	@echo "  make update-deps    - Update dependencies"