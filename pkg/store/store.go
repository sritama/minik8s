package store

import (
	"context"
	"time"
)

// Object is the interface that all API objects must implement
type Object interface {
	GetKind() string
	GetAPIVersion() string
	GetName() string
	GetNamespace() string
	GetUID() string
	GetResourceVersion() string
	SetResourceVersion(version string)
	GetCreationTimestamp() time.Time
	SetCreationTimestamp(timestamp time.Time)
}

// EventType represents the type of watch event
type EventType string

const (
	Added    EventType = "ADDED"
	Modified EventType = "MODIFIED"
	Deleted  EventType = "DELETED"
	Error    EventType = "ERROR"
)

// WatchEvent represents a single watch event
type WatchEvent struct {
	Type   EventType `json:"type"`
	Object Object    `json:"object"`
}

// WatchResult represents the result of a watch operation
type WatchResult struct {
	Events chan WatchEvent
	Stop   chan struct{}
}

// Store defines the interface for a data store
type Store interface {
	// Create creates a new object in the store
	Create(ctx context.Context, obj Object) error

	// Get retrieves an object by name and namespace
	Get(ctx context.Context, kind, namespace, name string) (Object, error)

	// List retrieves all objects of a given kind and namespace
	List(ctx context.Context, kind, namespace string) ([]Object, error)

	// Update updates an existing object
	Update(ctx context.Context, obj Object) error

	// Delete deletes an object by name and namespace
	Delete(ctx context.Context, kind, namespace, name string) error

	// Watch watches for changes to objects of a given kind and namespace
	Watch(ctx context.Context, kind, namespace string) (WatchResult, error)

	// Close closes the store and releases resources
	Close() error
}

// Options contains configuration options for the store
type Options struct {
	// WatchBufferSize is the size of the buffer for watch events
	WatchBufferSize int
	// GCInterval is the interval for garbage collection
	GCInterval time.Duration
}

// DefaultOptions returns the default store options
func DefaultOptions() *Options {
	return &Options{
		WatchBufferSize: 100,
		GCInterval:      5 * time.Minute,
	}
}
