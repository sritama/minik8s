#!/bin/bash

# Script to test etcd integration

set -e

echo "🧪 Testing etcd integration for Minik8s"
echo "======================================="

# Check if Docker is running
if ! docker info > /dev/null 2>&1; then
    echo "❌ Docker is not running. Please start Docker and try again."
    exit 1
fi

# Start etcd
echo "🚀 Starting etcd..."
docker-compose up -d etcd

# Wait for etcd to be ready
echo "⏳ Waiting for etcd to be ready..."
timeout=30
counter=0
while [ $counter -lt $timeout ]; do
    if docker exec minik8s-etcd etcdctl endpoint health > /dev/null 2>&1; then
        echo "✅ Etcd is ready!"
        break
    fi
    sleep 1
    counter=$((counter + 1))
done

if [ $counter -eq $timeout ]; then
    echo "❌ Etcd failed to start within $timeout seconds"
    docker-compose logs etcd
    docker-compose down
    exit 1
fi

# Set environment variable for tests
export ETCD_ENDPOINT="localhost:2379"

# Run etcd tests
echo "🧪 Running etcd integration tests..."
go test -v -tags=integration ./pkg/store/...

# Test the API server with etcd
echo "🧪 Testing API server with etcd..."
echo "Starting API server with etcd store..."

# Start API server in background
go run cmd/apiserver/main.go --store=etcd --etcd-endpoints=localhost:2379 &
SERVER_PID=$!

# Wait for server to start
sleep 3

# Test basic operations
echo "Testing basic operations..."

# Create a pod
echo "Creating pod..."
go run cmd/cli/main.go create -f examples/pod.yaml

# List pods
echo "Listing pods..."
go run cmd/cli/main.go get pods

# Create a node
echo "Creating node..."
go run cmd/cli/main.go create -f examples/node.yaml

# List nodes
echo "Listing nodes..."
go run cmd/cli/main.go get nodes

# Clean up
echo "Cleaning up..."
go run cmd/cli/main.go delete pods nginx-pod
go run cmd/cli/main.go delete nodes worker-node-1

# Stop API server
echo "Stopping API server..."
kill $SERVER_PID

# Stop etcd
echo "🛑 Stopping etcd..."
docker-compose down

echo "✅ Etcd integration test completed successfully!" 