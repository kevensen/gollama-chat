# Makefile for gollama-chat

.PHONY: build run clean test test-coverage test-short test-verbose test-bench fmt vet deps dev

# Variables
BINARY_NAME=gollama-chat
BUILD_DIR=bin
MAIN_PATH=cmd/main.go
GO_BIN=/home/kdevensen/bin/go/bin/go

# Default target
all: build

# Build the application
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	@$(GO_BIN) build -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

# Run the application
run: build
	@echo "Running $(BINARY_NAME)..."
	./$(BUILD_DIR)/$(BINARY_NAME)

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	@$(GO_BIN) clean

# Run tests
test:
	@echo "Running tests..."
	@$(GO_BIN) test ./...

# Run tests with verbose output
test-verbose:
	@echo "Running tests with verbose output..."
	@$(GO_BIN) test -v ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	@$(GO_BIN) test -cover ./...
	@echo "Generating detailed coverage report..."
	@$(GO_BIN) test -coverprofile=coverage.out ./...
	@$(GO_BIN) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run short tests only (skip longer running tests)
test-short:
	@echo "Running short tests..."
	@$(GO_BIN) test -short ./...

# Run benchmarks
test-bench:
	@echo "Running benchmarks..."
	@$(GO_BIN) test -bench=. ./...

# Run input performance tests with thresholds
test-performance:
	@echo "Running input performance tests..."
	@./test_input_performance.sh

# Run input benchmarks only
test-input-bench:
	@echo "Running input component benchmarks..."
	@$(GO_BIN) test -bench=. -benchmem ./internal/tui/tabs/chat/input/

# Run full input pipeline benchmarks
test-pipeline-bench:
	@echo "Running full input pipeline benchmarks..."
	@$(GO_BIN) test -bench=. -benchmem ./internal/tui/tui/

# Run specific test package
test-config:
	@echo "Running configuration tests..."
	@$(GO_BIN) test -v ./internal/configuration

test-rag:
	@echo "Running RAG service tests..."
	@$(GO_BIN) test -v ./internal/rag

test-chat:
	@echo "Running chat tests..."
	@$(GO_BIN) test -v ./internal/tui/tabs/chat

# Format code
fmt:
	@echo "Formatting code..."
	@$(GO_BIN) fmt ./...

# Vet code
vet:
	@echo "Vetting code..."
	@$(GO_BIN) vet ./...

# Install dependencies
deps:
	@echo "Installing dependencies..."
	@$(GO_BIN) mod download
	@$(GO_BIN) mod tidy

# Development workflow
dev: fmt vet test build
	@echo "Development workflow complete!"

# Show test coverage in terminal
coverage-terminal:
	@$(GO_BIN) test -coverprofile=coverage.out ./...
	@$(GO_BIN) tool cover -func=coverage.out

# Help
help:
	@echo "Available targets:"
	@echo "  build    - Build the application"
	@echo "  run      - Build and run the application"
	@echo "  clean    - Clean build artifacts"
	@echo "  test     - Run tests"
	@echo "  fmt      - Format code"
	@echo "  vet      - Vet code"
	@echo "  deps     - Install/update dependencies"
	@echo "  dev      - Run development workflow (fmt, vet, test, build)"
	@echo "  help     - Show this help"
