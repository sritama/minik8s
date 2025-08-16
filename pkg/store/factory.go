package store

import (
	"fmt"
	"os"
	"strings"
)

// StoreType represents the type of store to use
type StoreType string

const (
	// StoreTypeMemory represents the in-memory store
	StoreTypeMemory StoreType = "memory"
	// StoreTypeEtcd represents the etcd store
	StoreTypeEtcd StoreType = "etcd"
)

// StoreConfig contains configuration for creating a store
type StoreConfig struct {
	Type      StoreType
	Endpoints []string
	Prefix    string
	Options   *Options
}

// NewStore creates a new store based on configuration
func NewStore(config *StoreConfig) (Store, error) {
	if config == nil {
		config = &StoreConfig{
			Type:    StoreTypeMemory,
			Prefix:  "/minik8s",
			Options: DefaultOptions(),
		}
	}

	switch config.Type {
	case StoreTypeMemory:
		return NewMemoryStore(config.Options), nil
	case StoreTypeEtcd:
		if len(config.Endpoints) == 0 {
			config.Endpoints = []string{"localhost:2379"}
		}
		if config.Prefix == "" {
			config.Prefix = "/minik8s"
		}
		return NewEtcdStore(config.Endpoints, config.Prefix, config.Options)
	default:
		return nil, fmt.Errorf("unknown store type: %s", config.Type)
	}
}

// NewStoreFromEnv creates a store based on environment variables
func NewStoreFromEnv() (Store, error) {
	storeType := StoreType(os.Getenv("MINIK8S_STORE_TYPE"))
	if storeType == "" {
		storeType = StoreTypeMemory
	}

	endpointsStr := os.Getenv("MINIK8S_ETCD_ENDPOINTS")
	var endpoints []string
	if endpointsStr != "" {
		endpoints = strings.Split(endpointsStr, ",")
	}

	prefix := os.Getenv("MINIK8S_STORE_PREFIX")
	if prefix == "" {
		prefix = "/minik8s"
	}

	config := &StoreConfig{
		Type:      storeType,
		Endpoints: endpoints,
		Prefix:    prefix,
		Options:   DefaultOptions(),
	}

	return NewStore(config)
}

// NewStoreWithFallback creates a store with fallback to in-memory if etcd fails
func NewStoreWithFallback(config *StoreConfig) (Store, error) {
	if config.Type == StoreTypeEtcd {
		store, err := NewEtcdStore(config.Endpoints, config.Prefix, config.Options)
		if err != nil {
			// Fallback to in-memory store
			fmt.Printf("Warning: Failed to connect to etcd: %v, falling back to in-memory store\n", err)
			return NewMemoryStore(config.Options), nil
		}
		return store, nil
	}

	return NewStore(config)
}
