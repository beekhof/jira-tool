.PHONY: build test lint clean install help

# Binary name
BINARY_NAME=jira-tool

# Build directory
BUILD_DIR=bin

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOMOD=$(GOCMD) mod
GOLINT=golangci-lint

# Linter configuration
LINT_CONFIG=.golangci.yml

# Default target
.DEFAULT_GOAL := help

## build: Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) -o $(BUILD_DIR)/$(BINARY_NAME) .
	@echo "Binary built: $(BUILD_DIR)/$(BINARY_NAME)"

## test: Run all tests
test:
	@echo "Running tests..."
	$(GOTEST) -v ./...

## test-coverage: Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

## lint: Run the linter
lint:
	@echo "Running linter..."
	@if ! command -v $(GOLINT) > /dev/null; then \
		echo "golangci-lint not found. Installing..."; \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin v1.55.2; \
	fi
	$(GOLINT) run --show-stats  ./...

## lint-install: Install golangci-lint
lint-install:
	@echo "Installing golangci-lint..."
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin v1.55.2

## fmt: Format code
fmt:
	@echo "Formatting code..."
	$(GOCMD) fmt ./...

## vet: Run go vet
vet:
	@echo "Running go vet..."
	$(GOCMD) vet ./...

## tidy: Tidy go modules
tidy:
	@echo "Tidying go modules..."
	$(GOMOD) tidy

## clean: Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html
	@echo "Clean complete"

## install: Install the binary to GOPATH/bin
install: build
	@echo "Installing $(BINARY_NAME)..."
	cp $(BUILD_DIR)/$(BINARY_NAME) $$(go env GOPATH)/bin/$(BINARY_NAME)
	@echo "Installed to $$(go env GOPATH)/bin/$(BINARY_NAME)"

## all: Build, test, and lint
all: fmt vet test lint build

## help: Show this help message
help:
	@echo "Available targets:"
	@grep -E '^##' $(MAKEFILE_LIST) | sed 's/## //' | column -t -s ':'

