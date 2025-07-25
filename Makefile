# Makefile for dockswap

# Variables
BINARY_NAME=dockswap
BINARY_PATH=./$(BINARY_NAME)
GO_FILES=$(shell find . -name "*.go" -type f -not -path "./vendor/*")
TEST_PACKAGES=$(shell go list ./... | grep -v /vendor/)

# Build information
VERSION?=$(shell cat VERSION 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Go build flags
LDFLAGS=-ldflags "-X dockswap/internal/cli.Version=$(VERSION) -X dockswap/internal/cli.commit=$(COMMIT) -X dockswap/internal/cli.date=$(BUILD_TIME)"

.PHONY: help build clean install test test-verbose test-coverage fmt vet lint deps mod-tidy run dev all

# Default target
all: fmt vet test build

# Help target
help:
	@echo "Available targets:"
	@echo "  build         Build the binary"
	@echo "  clean         Remove build artifacts"
	@echo "  install       Install the binary to GOPATH/bin"
	@echo "  test          Run Go tests"
	@echo "  test-verbose  Run Go tests with verbose output"
	@echo "  test-coverage Run Go tests with coverage report"
	@echo "  test-e2e      Run E2E tests"
	@echo "  test-e2e-basic Run basic flow E2E tests"
	@echo "  test-e2e-error Run error scenario E2E tests"
	@echo "  test-all      Run all tests (Go + E2E)"
	@echo "  fmt           Format Go code"
	@echo "  vet           Run go vet"
	@echo "  lint          Run golangci-lint (if available)"
	@echo "  deps          Download dependencies"
	@echo "  mod-tidy      Tidy go modules"
	@echo "  run           Run the application with --help"
	@echo "  dev           Development build and run"
	@echo "  all           Run fmt, vet, test, and build"

# Build targets
build:
	@echo "Building $(BINARY_NAME)..."
	@go build $(LDFLAGS) -o $(BINARY_PATH) .
	@echo "Build complete: $(BINARY_PATH)"

clean:
	@echo "Cleaning build artifacts..."
	@rm -f $(BINARY_PATH)
	@rm -f coverage.out
	@rm -f coverage.html
	@echo "Clean complete"

install: build
	@echo "Installing $(BINARY_NAME)..."
	@go install $(LDFLAGS) .
	@echo "Install complete"

# Test targets
test:
	@echo "Running tests..."
	@go test -v $(TEST_PACKAGES)

test-verbose:
	@echo "Running tests with verbose output..."
	@go test -v -count=1 $(TEST_PACKAGES)

test-coverage:
	@echo "Running tests with coverage..."
	@go test -v -coverprofile=coverage.out $(TEST_PACKAGES)
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"
	@go tool cover -func=coverage.out

# E2E test targets
test-e2e: build
	@echo "Running E2E tests..."
	@bun test e2e/*.test.js --timeout=30000

test-e2e-basic: build
	@echo "Running basic flow E2E tests..."
	@bun test e2e/01-basic* --timeout=30000

test-e2e-error: build
	@echo "Running error scenario E2E tests..."
	@bun test e2e/02-error* --timeout=30000

test-all: test test-e2e

# Development targets
fmt:
	@echo "Formatting Go code..."
	@go fmt $(TEST_PACKAGES)

vet:
	@echo "Running go vet..."
	@go vet $(TEST_PACKAGES)

lint:
	@echo "Running linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "golangci-lint not found, skipping lint check"; \
		echo "Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

# Dependency management
deps:
	@echo "Downloading dependencies..."
	@go mod download

mod-tidy:
	@echo "Tidying go modules..."
	@go mod tidy

# Run targets
run: build
	@echo "Running $(BINARY_NAME)..."
	@$(BINARY_PATH) --help

dev: fmt vet
	@echo "Development build and run..."
	@go run . --help

# Watch for changes (requires entr or similar tool)
watch:
	@if command -v entr >/dev/null 2>&1; then \
		echo "Watching for changes..."; \
		find . -name "*.go" | entr -r make dev; \
	else \
		echo "entr not found. Install with: brew install entr (macOS) or apt-get install entr (Ubuntu)"; \
	fi