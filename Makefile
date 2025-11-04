.PHONY: all build test clean proto deps lint fmt install-tools help

# Variables
BINARY_DAEMON := iperf-daemon
BINARY_CONTROLLER := iperf-controller
BUILD_DIR := build
PROTO_DIR := api/proto
PROTO_OUT_DIR := api/proto/daemon/v1

# Go parameters
GOCMD := go
GOBUILD := $(GOCMD) build
GOTEST := $(GOCMD) test
GOCLEAN := $(GOCMD) clean
GOMOD := $(GOCMD) mod
GOFMT := $(GOCMD) fmt
GOVET := $(GOCMD) vet

# Build flags
LDFLAGS := -ldflags "-s -w"
RACE_FLAG := -race

# Default target
all: deps proto build test

help: ## Display this help message
	@echo "iperf-cnc Makefile targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

deps: ## Download Go dependencies
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy

install-tools: ## Install development tools
	@echo "Installing development tools..."
	@go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	@go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@echo "Tools installed. Ensure protoc is installed on your system."

proto: ## Generate Go code from Protocol Buffers
	@echo "Generating protobuf code..."
	@mkdir -p $(PROTO_OUT_DIR)
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		$(PROTO_DIR)/daemon.proto
	@echo "Protobuf generation complete"

build: build-daemon build-controller ## Build both daemon and controller

build-daemon: ## Build the daemon binary
	@echo "Building daemon..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_DAEMON) ./cmd/daemon
	@echo "Daemon built: $(BUILD_DIR)/$(BINARY_DAEMON)"

build-controller: ## Build the controller binary
	@echo "Building controller..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_CONTROLLER) ./cmd/controller
	@echo "Controller built: $(BUILD_DIR)/$(BINARY_CONTROLLER)"

build-race: ## Build with race detector enabled
	@echo "Building with race detector..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(RACE_FLAG) -o $(BUILD_DIR)/$(BINARY_DAEMON)-race ./cmd/daemon
	$(GOBUILD) $(RACE_FLAG) -o $(BUILD_DIR)/$(BINARY_CONTROLLER)-race ./cmd/controller
	@echo "Race detector builds complete"

test: ## Run all tests
	@echo "Running tests..."
	$(GOTEST) -v ./...

test-coverage: ## Run tests with coverage
	@echo "Running tests with coverage..."
	$(GOTEST) -v -cover -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

test-race: ## Run tests with race detector
	@echo "Running tests with race detector..."
	$(GOTEST) -v -race ./...

bench: ## Run benchmarks
	@echo "Running benchmarks..."
	$(GOTEST) -bench=. -benchmem ./...

fmt: ## Format Go code
	@echo "Formatting code..."
	$(GOFMT) ./...

vet: ## Run go vet
	@echo "Running go vet..."
	$(GOVET) ./...

lint: ## Run golangci-lint
	@echo "Running golangci-lint..."
	golangci-lint run ./...

lint-fix: ## Run golangci-lint with auto-fix
	@echo "Running golangci-lint with auto-fix..."
	golangci-lint run --fix ./...

clean: ## Clean build artifacts
	@echo "Cleaning..."
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	rm -f coverage.out coverage.html
	@echo "Clean complete"

clean-proto: ## Remove generated protobuf files
	@echo "Cleaning generated protobuf files..."
	rm -rf $(PROTO_OUT_DIR)

clean-all: clean clean-proto ## Clean everything including proto files

run-daemon: build-daemon ## Build and run the daemon
	@echo "Running daemon..."
	$(BUILD_DIR)/$(BINARY_DAEMON)

run-controller: build-controller ## Build and run the controller
	@echo "Running controller..."
	$(BUILD_DIR)/$(BINARY_CONTROLLER)

install: build ## Install binaries to $GOPATH/bin
	@echo "Installing binaries..."
	$(GOCMD) install ./cmd/daemon
	$(GOCMD) install ./cmd/controller
	@echo "Installation complete"

docker-build: ## Build Docker images
	@echo "Building Docker images..."
	docker build -t iperf-cnc-daemon:latest -f Dockerfile.daemon .
	docker build -t iperf-cnc-controller:latest -f Dockerfile.controller .

# Development helpers
watch: ## Watch for changes and rebuild (requires entr)
	@echo "Watching for changes... (requires 'entr' to be installed)"
	find . -name '*.go' | entr -r make build

.DEFAULT_GOAL := help
