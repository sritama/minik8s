package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/minik8s/minik8s/pkg/nodeagent"
	"github.com/minik8s/minik8s/pkg/store"
)

var (
	nodeName          = flag.String("node-name", "", "Name of this node (required)")
	apiServerURL      = flag.String("api-server", "http://localhost:8080", "API server URL")
	storeType         = flag.String("store", "memory", "Store type: memory or etcd")
	etcdEndpoints     = flag.String("etcd-endpoints", "localhost:2379", "Comma-separated list of etcd endpoints")
	storePrefix       = flag.String("store-prefix", "/minik8s", "Store key prefix")
	enableFallback    = flag.Bool("enable-fallback", true, "Enable fallback to in-memory store if etcd fails")
	heartbeatInterval = flag.Duration("heartbeat-interval", 30*time.Second, "Heartbeat interval")
)

func main() {
	flag.Parse()

	// Validate required flags
	if *nodeName == "" {
		log.Fatal("--node-name is required")
	}

	// Create store configuration
	storeConfig := &store.StoreConfig{
		Type:      store.StoreType(*storeType),
		Endpoints: []string{*etcdEndpoints},
		Prefix:    *storePrefix,
		Options:   store.DefaultOptions(),
	}

	// Create store
	var s store.Store
	var err error

	if *enableFallback {
		s, err = store.NewStoreWithFallback(storeConfig)
	} else {
		s, err = store.NewStore(storeConfig)
	}

	if err != nil {
		log.Fatalf("Failed to create store: %v", err)
	}
	defer s.Close()

	// Log configuration
	fmt.Printf("Starting node agent for node: %s\n", *nodeName)
	fmt.Printf("API server URL: %s\n", *apiServerURL)
	fmt.Printf("Store type: %s\n", storeConfig.Type)
	if storeConfig.Type == store.StoreTypeEtcd {
		fmt.Printf("Etcd endpoints: %v\n", storeConfig.Endpoints)
		fmt.Printf("Store prefix: %s\n", storeConfig.Prefix)
	}
	fmt.Printf("Heartbeat interval: %v\n", *heartbeatInterval)

	// Create mock runtime components for now
	criRuntime := nodeagent.NewMockCRIRuntime()
	networkMgr := &nodeagent.MockNetworkManager{}
	volumeMgr := &nodeagent.MockVolumeManager{}

	// Create node agent configuration
	agentConfig := &nodeagent.Config{
		NodeName:          *nodeName,
		APIServerURL:      *apiServerURL,
		Store:             s,
		CRIRuntime:        criRuntime,
		NetworkManager:    networkMgr,
		VolumeManager:     volumeMgr,
		HeartbeatInterval: *heartbeatInterval,
	}

	// Create and start node agent
	agent := nodeagent.NewAgent(agentConfig)

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start the agent
	if err := agent.Start(ctx); err != nil {
		log.Fatalf("Failed to start node agent: %v", err)
	}

	fmt.Printf("Node agent started successfully\n")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	fmt.Println("\nShutting down node agent...")

	// Stop the agent
	agent.Stop()

	fmt.Println("Node agent stopped")
}
