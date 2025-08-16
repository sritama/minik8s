# Minik8s - Kubernetes-like Orchestrator

A pragmatic Kubernetes-like orchestration system built for learning, experimentation, and small-to-medium production use.

## Project Overview

Minik8s follows the proven declarative API + control-loop pattern and reuses industry standards (OCI image format, CRI, CNI, CSI, Raft-based datastore). The system provides a simplified but production-ready container orchestration platform.



## Architecture

- **API Server**: REST/gRPC endpoints with validation and watch semantics
- **Datastore**: Raft-backed key-value store (etcd) for strong consistency
- **Controller Manager**: Runs controllers with leader election
- **Scheduler**: Places pods on nodes based on resources and constraints
- **Node Agent**: Manages containers via CRI, networking via CNI, storage via CSI
- **CLI**: kubectl-like command-line interface

## Quick Start

### Prerequisites
- Go 1.21+
- Docker (for running etcd)
- Make (optional, for build scripts)

### Local Development
```bash
# Clone the repository
git clone <your-repo-url>
cd Minik8s

# Install dependencies
go mod tidy

# Start etcd (required for persistent storage)
make start-etcd

# Run the API server with etcd store
make run-apiserver-etcd

# In another terminal, test with the CLI
go run cmd/cli/main.go create -f examples/pod.yaml
go run cmd/cli/main.go get pods
```

### Building
```bash
# Build all components
make build

# Build specific component
go build -o bin/apiserver cmd/apiserver/main.go
go build -o bin/cli cmd/cli/main.go
```

## 🚀 Live Demo

The system is fully functional with persistent storage! Here's a quick test:

```bash
# Terminal 1: Start etcd and API server
make start-etcd
make run-apiserver-etcd

# Terminal 2: Create and manage resources
go run cmd/cli/main.go create -f examples/pod.yaml
go run cmd/cli/main.go get pods
go run cmd/cli/main.go watch pods nginx-pod
go run cmd/cli/main.go delete pods nginx-pod

# Data persists across API server restarts!
```

## Project Structure

```
.
├── cmd/                    # Main applications
│   ├── apiserver/         # API server binary ✅
│   ├── controller-manager/ # Controller manager binary
│   ├── scheduler/         # Scheduler binary
│   ├── node-agent/        # Node agent binary
│   └── cli/               # Command-line interface ✅
├── pkg/                   # Library code
│   ├── api/               # API definitions and types ✅
│   ├── apiserver/         # API server implementation ✅
│   ├── store/             # Data store interfaces and implementations ✅
│   ├── controller/        # Controller framework and implementations
│   ├── scheduler/         # Scheduler implementation
│   ├── nodeagent/         # Node agent implementation
│   └── client/            # Client libraries
├── examples/               # Example manifests and configurations ✅
├── docs/                   # Documentation ✅
├── scripts/                # Build and deployment scripts ✅
├── config/                 # Configuration management ✅
└── test/                   # Test files and test data
```

## Development Phases

- **Phase 0** ✅ - Project scaffolding and basic API server (COMPLETE)
- **Phase 1** ✅ - Datastore integration (etcd) (COMPLETE)
- **Phase 2** ✅ - Node agent and CRI integration (COMPLETE)
- **Phase 3** - Scheduler and controllers
- **Phase 4** - Networking and services
- **Phase 5** - Storage and persistent volumes
- **Phase 6** - Hardening and high availability

## 🔧 Development Commands

```bash
# Build all components
make build

# Run tests
make test

# Run etcd integration tests
make test-etcd

# Start API server (memory store)
make run-apiserver

# Start API server (etcd store)
make run-apiserver-etcd

# Start CLI
make run-cli

# Start etcd
make start-etcd

# Stop etcd
make stop-etcd

# Format code
make fmt

# Clean build artifacts
make clean

# Run development setup
./scripts/dev.sh

# Run etcd integration test
./scripts/test-etcd.sh
```

## 📊 API Endpoints (Phase 1)

### Pods
- `POST /api/v1alpha1/namespaces/{namespace}/pods` - Create pod
- `GET /api/v1alpha1/namespaces/{namespace}/pods` - List pods in namespace
- `GET /api/v1alpha1/namespaces/{namespace}/pods/{name}` - Get specific pod
- `PUT /api/v1alpha1/namespaces/{namespace}/pods/{name}` - Update pod
- `DELETE /api/v1alpha1/namespaces/{namespace}/pods/{name}` - Delete pod
- `GET /api/v1alpha1/namespaces/{namespace}/pods/{name}/watch` - Watch pod

### Nodes
- `POST /api/v1alpha1/nodes` - Create node
- `GET /api/v1alpha1/nodes` - List all nodes
- `GET /api/v1alpha1/nodes/{name}` - Get specific node
- `PUT /api/v1alpha1/nodes/{name}` - Update node
- `DELETE /api/v1alpha1/nodes/{name}` - Delete node
- `GET /api/v1alpha1/nodes/{name}/watch` - Watch node

### Health
- `GET /healthz` - Health check
- `GET /readyz` - Readiness check

## 🏗️ Store Configuration

### **In-Memory Store (Default)**
```bash
go run cmd/apiserver/main.go --store=memory
```

### **Etcd Store**
```bash
go run cmd/apiserver/main.go --store=etcd --etcd-endpoints=localhost:2379
```

### **Environment Variables**
```bash
export MINIK8S_STORE_TYPE=etcd
export MINIK8S_ETCD_ENDPOINTS=localhost:2379
export MINIK8S_STORE_PREFIX=/minik8s
export MINIK8S_ENABLE_FALLBACK=true
```

## 🧪 Testing

### **Unit Tests**
```bash
# Run all tests
go test -v ./...

# Run store tests
go test -v ./pkg/store/...

# Run with coverage
make test-coverage
```

### **Integration Tests**
```bash
# Run etcd integration tests
make test-etcd

# Or manually
./scripts/test-etcd.sh
```

## 🔍 Key Features

### **Phase 0 Features**
- ✅ REST API with CRUD operations
- ✅ Real-time watch semantics
- ✅ CLI interface for resource management
- ✅ In-memory storage for development

### **Phase 1 Features**
- ✅ **Persistent Storage** with etcd
- ✅ **Strong Consistency** via Raft consensus
- ✅ **High Availability** with leader election
- ✅ **Fallback Support** for graceful degradation
- ✅ **Production Ready** data persistence
- ✅ **Docker Integration** for easy deployment

### **Phase 2 Features**
- ✅ **Node Agent** with pod lifecycle management
- ✅ **CRI Integration** for container runtime operations
- ✅ **Pod Synchronization** with automatic detection
- ✅ **Network & Volume Management** interfaces
- ✅ **Status Reporting** with real-time updates
- ✅ **Mock Implementations** for development

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request

## License

[Your chosen license] 