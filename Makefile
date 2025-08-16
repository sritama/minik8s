.PHONY: build clean test test-etcd run-apiserver run-cli start-etcd stop-etcd

# Build variables
BINARY_DIR=bin
VERSION=$(shell git describe --tags --always --dirty)
LDFLAGS=-ldflags "-X main.Version=${VERSION}"

# Default target
all: build

# Build all binaries
build: clean
	@echo "Building Minik8s binaries..."
	@mkdir -p ${BINARY_DIR}
	go build ${LDFLAGS} -o ${BINARY_DIR}/apiserver cmd/apiserver/main.go
	go build ${LDFLAGS} -o ${BINARY_DIR}/cli cmd/cli/main.go
	go build ${LDFLAGS} -o ${BINARY_DIR}/nodeagent cmd/nodeagent/main.go
	go build ${LDFLAGS} -o ${BINARY_DIR}/controller-manager cmd/controller-manager/main.go
	@echo "Build complete!"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	@rm -rf ${BINARY_DIR}
	@go clean -cache

# Run tests
test:
	@echo "Running tests..."
	go test -v ./...

# Run tests with etcd integration
test-etcd:
	@echo "Running etcd integration tests..."
	./scripts/test-etcd.sh

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

# Run API server
run-apiserver:
	@echo "Starting API server..."
	go run cmd/apiserver/main.go

# Run API server with etcd
run-apiserver-etcd:
	@echo "Starting API server with etcd store..."
	go run cmd/apiserver/main.go --store=etcd --etcd-endpoints=localhost:2379

# Run CLI
run-cli:
	@echo "Starting CLI..."
	go run cmd/cli/main.go

# Run node agent
run-nodeagent:
	@echo "Starting node agent..."
	go run cmd/nodeagent/main.go --node-name=worker-node-1

# Run controller manager
run-controller-manager:
	@echo "Starting controller manager..."
	go run cmd/controller-manager/main.go

# Run controller manager with etcd
run-controller-manager-etcd:
	@echo "Starting controller manager with etcd store..."
	go run cmd/controller-manager/main.go --store=etcd --etcd-endpoints=localhost:2379

# Run all components (full system)
run-all: start-etcd
	@echo "Starting full Minik8s system..."
	@echo "Starting API server..."
	@go run cmd/apiserver/main.go --store=etcd --etcd-endpoints=localhost:2379 &
	@sleep 2
	@echo "Starting controller manager..."
	@go run cmd/controller-manager/main.go --store=etcd --etcd-endpoints=localhost:2379 &
	@sleep 2
	@echo "Starting node agent..."
	@go run cmd/nodeagent/main.go --node-name=worker-node-1 &
	@echo "All components started. Press Ctrl+C to stop all."
	@wait

# Start etcd
start-etcd:
	@echo "Starting etcd..."
	docker-compose up -d etcd

# Stop etcd
stop-etcd:
	@echo "Stopping etcd..."
	docker-compose down

# Install dependencies
deps:
	@echo "Installing dependencies..."
	go mod tidy
	go mod download

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Lint code
lint:
	@echo "Linting code..."
	golangci-lint run

# Generate mocks (if using mockery)
mocks:
	@echo "Generating mocks..."
	mockery --all

# Help
help:
	@echo "Available targets:"
	@echo "  build                    - Build all binaries"
	@echo "  clean                    - Clean build artifacts"
	@echo "  test                     - Run tests"
	@echo "  test-etcd                - Run etcd integration tests"
	@echo "  test-coverage            - Run tests with coverage"
	@echo "  run-apiserver            - Run API server (memory store)"
	@echo "  run-apiserver-etcd       - Run API server (etcd store)"
	@echo "  run-cli                  - Run CLI"
	@echo "  run-nodeagent            - Run node agent"
	@echo "  run-controller-manager   - Run controller manager (memory store)"
	@echo "  run-controller-manager-etcd - Run controller manager (etcd store)"
	@echo "  run-all                  - Run all components (full system)"
	@echo "  start-etcd               - Start etcd container"
	@echo "  stop-etcd                - Stop etcd container"
	@echo "  deps                     - Install dependencies"
	@echo "  fmt                      - Format code"
	@echo "  lint                     - Lint code"
	@echo "  mocks                    - Generate mocks"
	@echo "  help                     - Show this help" 