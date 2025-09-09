# Makefile for gollama-chat

.PHONY: build run clean test fmt vet

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
	@$(GO_BIN) test -v ./...

# Format code
fmt:
	@$(GO_BIN) fmt ./...

# Vet code
vet:
	@$(GO_BIN) vet ./...

# Install dependencies
deps:
	@$(GO_BIN) mod download
	@$(GO_BIN) mod tidy

# Development workflow
dev: fmt vet test build

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
