# AegisFlux Local Agent Makefile

BINARY_NAME=local-agent
GO_VERSION=1.21
BUILD_DIR=build
DIST_DIR=dist

# Build flags
LDFLAGS=-ldflags "-X main.Version=$(shell git describe --tags --always --dirty) -X main.BuildTime=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)"

.PHONY: all build clean test lint fmt vet deps run docker-build docker-run help

# Default target
all: build

# Build the agent
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) .

# Build for multiple platforms
build-all:
	@echo "Building for multiple platforms..."
	@mkdir -p $(DIST_DIR)
	
	# Linux AMD64
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-linux-amd64 .
	
	# Linux ARM64
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-linux-arm64 .
	
	# macOS AMD64
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-amd64 .
	
	# macOS ARM64
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(DIST_DIR)/$(BINARY_NAME)-darwin-arm64 .

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -rf $(BUILD_DIR) $(DIST_DIR)

# Run tests
test:
	@echo "Running tests..."
	go test -v ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Lint the code
lint:
	@echo "Running linter..."
	golangci-lint run

# Format the code
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Vet the code
vet:
	@echo "Vetting code..."
	go vet ./...

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy

# Run the agent locally
run:
	@echo "Running agent locally..."
	go run .

# Run with environment variables
run-dev:
	@echo "Running agent in development mode..."
	AGENT_HOST_ID=localhost \
	AGENT_REGISTRY_URL=http://localhost:8084 \
	AGENT_POLL_INTERVAL_SEC=10 \
	AGENT_NATS_URL=nats://localhost:4222 \
	AGENT_VAULT_URL=http://localhost:8200 \
	AGENT_VAULT_TOKEN=dev-token \
	AGENT_CACHE_DIR=/tmp/aegisflux-agent \
	AGENT_LOG_LEVEL=debug \
	go run .

# Build Docker image
docker-build:
	@echo "Building Docker image..."
	docker build -t aegisflux/local-agent:latest .

# Run Docker container
docker-run:
	@echo "Running Docker container..."
	docker run --rm -it \
		--privileged \
		-v /sys/fs/bpf:/sys/fs/bpf \
		-e AGENT_HOST_ID=localhost \
		-e AGENT_REGISTRY_URL=http://host.docker.internal:8084 \
		-e AGENT_NATS_URL=nats://host.docker.internal:4222 \
		-e AGENT_VAULT_URL=http://host.docker.internal:8200 \
		aegisflux/local-agent:latest

# Install development tools
install-tools:
	@echo "Installing development tools..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Check if required tools are installed
check-tools:
	@echo "Checking required tools..."
	@command -v golangci-lint >/dev/null 2>&1 || { echo "golangci-lint is required but not installed. Run 'make install-tools'"; exit 1; }

# Development setup
dev-setup: install-tools deps
	@echo "Development setup complete"

# CI/CD pipeline
ci: check-tools deps fmt vet lint test

# Release build
release: clean build-all
	@echo "Release build complete"

# Help
help:
	@echo "Available targets:"
	@echo "  build        - Build the agent binary"
	@echo "  build-all    - Build for multiple platforms"
	@echo "  clean        - Clean build artifacts"
	@echo "  test         - Run tests"
	@echo "  test-coverage - Run tests with coverage"
	@echo "  lint         - Run linter"
	@echo "  fmt          - Format code"
	@echo "  vet          - Vet code"
	@echo "  deps         - Download dependencies"
	@echo "  run          - Run the agent locally"
	@echo "  run-dev      - Run in development mode"
	@echo "  docker-build - Build Docker image"
	@echo "  docker-run   - Run Docker container"
	@echo "  install-tools - Install development tools"
	@echo "  dev-setup    - Setup development environment"
	@echo "  ci           - Run CI/CD pipeline"
	@echo "  release      - Create release build"
	@echo "  help         - Show this help"
