package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/minik8s/minik8s/pkg/apiserver"
	"github.com/minik8s/minik8s/pkg/store"
)

var (
	port           = flag.Int("port", 8080, "Port to listen on")
	storeType      = flag.String("store", "memory", "Store type: memory or etcd")
	etcdEndpoints  = flag.String("etcd-endpoints", "localhost:2379", "Comma-separated list of etcd endpoints")
	storePrefix    = flag.String("store-prefix", "/minik8s", "Store key prefix")
	enableFallback = flag.Bool("enable-fallback", true, "Enable fallback to in-memory store if etcd fails")
)

func main() {
	flag.Parse()

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

	// Log store type
	fmt.Printf("Using store type: %s\n", storeConfig.Type)
	if storeConfig.Type == store.StoreTypeEtcd {
		fmt.Printf("Etcd endpoints: %v\n", storeConfig.Endpoints)
		fmt.Printf("Store prefix: %s\n", storeConfig.Prefix)
	}

	// Create API server
	server := apiserver.NewServer(s, *port)

	// Start server in goroutine
	go func() {
		if err := server.Start(); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	fmt.Printf("API server started on port %d\n", *port)
	fmt.Println("Press Ctrl+C to stop")

	<-sigChan
	fmt.Println("\nShutting down API server...")
}
