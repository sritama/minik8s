#!/bin/bash

# Development script for Minik8s

set -e

echo "🚀 Minik8s Development Environment"
echo "=================================="

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo "❌ Go is not installed. Please install Go 1.21+"
    exit 1
fi

# Check Go version
GO_VERSION=$(go version | awk '{print $3}' | sed 's/go//')
echo "✅ Go version: $GO_VERSION"

# Install dependencies
echo "📦 Installing dependencies..."
go mod tidy

# Run tests
echo "🧪 Running tests..."
go test -v ./...

# Build binaries
echo "🔨 Building binaries..."
go build -o bin/apiserver cmd/apiserver/main.go
go build -o bin/cli cmd/cli/main.go

echo "✅ Build complete!"
echo ""
echo "To start the API server:"
echo "  ./bin/apiserver"
echo ""
echo "To use the CLI:"
echo "  ./bin/cli get pods"
echo "  ./bin/cli create -f examples/pod.yaml"
echo ""
echo "Or use make commands:"
echo "  make run-apiserver"
echo "  make run-cli" 