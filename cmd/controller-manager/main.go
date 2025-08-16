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

	"github.com/minik8s/minik8s/pkg/controller"
	"github.com/minik8s/minik8s/pkg/scheduler"
	"github.com/minik8s/minik8s/pkg/store"
)

var (
	storeType        = flag.String("store", "memory", "Store type: memory or etcd")
	etcdEndpoints    = flag.String("etcd-endpoints", "localhost:2379", "Comma-separated list of etcd endpoints")
	storePrefix      = flag.String("store-prefix", "/minik8s", "Store key prefix")
	enableFallback   = flag.Bool("enable-fallback", true, "Enable fallback to in-memory store if etcd fails")
	syncInterval     = flag.Duration("sync-interval", 30*time.Second, "Controller sync interval")
	scheduleInterval = flag.Duration("schedule-interval", 10*time.Second, "Scheduler sync interval")
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

	// Log configuration
	fmt.Printf("Starting controller manager\n")
	fmt.Printf("Store type: %s\n", storeConfig.Type)
	if storeConfig.Type == store.StoreTypeEtcd {
		fmt.Printf("Etcd endpoints: %v\n", storeConfig.Endpoints)
		fmt.Printf("Store prefix: %s\n", storeConfig.Prefix)
	}
	fmt.Printf("Controller sync interval: %v\n", *syncInterval)
	fmt.Printf("Scheduler sync interval: %v\n", *scheduleInterval)

	// Create scheduler
	schedulerConfig := &scheduler.Config{
		Store:               s,
		DefaultNodeSelector: map[string]string{},
		SchedulingInterval:  *scheduleInterval,
	}
	sched := scheduler.NewScheduler(schedulerConfig)

	// Create controller manager
	controllerConfig := &controller.Config{
		Store:        s,
		SyncInterval: *syncInterval,
	}
	ctrlMgr := controller.NewManager(controllerConfig)

	// Add controllers
	deploymentCtrl := controller.NewDeploymentController(s)
	replicaSetCtrl := controller.NewReplicaSetController(s)
	ctrlMgr.AddController(deploymentCtrl)
	ctrlMgr.AddController(replicaSetCtrl)

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start scheduler
	if err := sched.Start(ctx); err != nil {
		log.Fatalf("Failed to start scheduler: %v", err)
	}

	// Start controller manager
	if err := ctrlMgr.Start(ctx); err != nil {
		log.Fatalf("Failed to start controller manager: %v", err)
	}

	fmt.Printf("Controller manager started successfully\n")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	fmt.Println("\nShutting down controller manager...")

	// Stop scheduler and controller manager
	sched.Stop()
	ctrlMgr.Stop()

	fmt.Println("Controller manager stopped")
}
