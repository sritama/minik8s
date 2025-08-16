package store

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/minik8s/minik8s/pkg/api"
)

// memoryStore implements the Store interface using in-memory storage
type memoryStore struct {
	mu       sync.RWMutex
	objects  map[string]map[string]Object // kind -> namespace -> name -> object
	watchers map[string][]*watcher        // kind -> watchers
	options  *Options
}

// watcher represents a single watch subscription
type watcher struct {
	events chan WatchEvent
	stop   chan struct{}
	kind   string
	ns     string
	closed bool
	mu     sync.Mutex
}

// NewMemoryStore creates a new in-memory store
func NewMemoryStore(options *Options) Store {
	if options == nil {
		options = DefaultOptions()
	}

	store := &memoryStore{
		objects:  make(map[string]map[string]Object),
		watchers: make(map[string][]*watcher),
		options:  options,
	}

	// Start garbage collection
	go store.gcLoop()

	return store
}

// Create creates a new object in the store
func (s *memoryStore) Create(ctx context.Context, obj Object) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	kind := obj.GetKind()
	namespace := obj.GetNamespace()
	name := obj.GetName()

	// Initialize namespace map if it doesn't exist
	if s.objects[kind] == nil {
		s.objects[kind] = make(map[string]Object)
	}

	// Check if object already exists
	if _, exists := s.objects[kind][namespace+"/"+name]; exists {
		return fmt.Errorf("object %s/%s of kind %s already exists", namespace, name, kind)
	}

	// Set metadata
	obj.SetResourceVersion(fmt.Sprintf("%d", time.Now().UnixNano()))
	obj.SetCreationTimestamp(time.Now())

	// Store the object
	key := namespace + "/" + name
	s.objects[kind][key] = obj

	// Notify watchers
	s.notifyWatchers(Added, obj)

	return nil
}

// Get retrieves an object by name and namespace
func (s *memoryStore) Get(ctx context.Context, kind, namespace, name string) (Object, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.objects[kind] == nil {
		return nil, fmt.Errorf("no objects of kind %s found", kind)
	}

	key := namespace + "/" + name
	obj, exists := s.objects[kind][key]
	if !exists {
		return nil, fmt.Errorf("object %s/%s of kind %s not found", namespace, name, kind)
	}

	return obj, nil
}

// List retrieves all objects of a given kind and namespace
func (s *memoryStore) List(ctx context.Context, kind, namespace string) ([]Object, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.objects[kind] == nil {
		return []Object{}, nil
	}

	var objects []Object
	for key, obj := range s.objects[kind] {
		// If namespace is empty, return all objects of this kind
		if namespace == "" {
			objects = append(objects, obj)
		} else {
			// Parse namespace from key (format: namespace/name)
			if len(key) > len(namespace)+1 && key[:len(namespace)] == namespace && key[len(namespace)] == '/' {
				objects = append(objects, obj)
			}
		}
	}

	return objects, nil
}

// Update updates an existing object
func (s *memoryStore) Update(ctx context.Context, obj Object) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	kind := obj.GetKind()
	namespace := obj.GetNamespace()
	name := obj.GetName()

	if s.objects[kind] == nil {
		return fmt.Errorf("no objects of kind %s found", kind)
	}

	key := namespace + "/" + name
	if _, exists := s.objects[kind][key]; !exists {
		return fmt.Errorf("object %s/%s of kind %s not found", namespace, name, kind)
	}

	// Update resource version
	obj.SetResourceVersion(fmt.Sprintf("%d", time.Now().UnixNano()))

	// Store the updated object
	s.objects[kind][key] = obj

	// Notify watchers
	s.notifyWatchers(Modified, obj)

	return nil
}

// Delete deletes an object by name and namespace
func (s *memoryStore) Delete(ctx context.Context, kind, namespace, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.objects[kind] == nil {
		return fmt.Errorf("no objects of kind %s found", kind)
	}

	key := namespace + "/" + name
	obj, exists := s.objects[kind][key]
	if !exists {
		return fmt.Errorf("object %s/%s of kind %s not found", namespace, name, kind)
	}

	// Notify watchers before deletion
	s.notifyWatchers(Deleted, obj)

	// Delete the object
	delete(s.objects[kind], key)

	// Clean up empty namespace maps
	if len(s.objects[kind]) == 0 {
		delete(s.objects, kind)
	}

	return nil
}

// Watch watches for changes to objects of a given kind and namespace
func (s *memoryStore) Watch(ctx context.Context, kind, namespace string) (WatchResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create watcher
	w := &watcher{
		events: make(chan WatchEvent, s.options.WatchBufferSize),
		stop:   make(chan struct{}),
		kind:   kind,
		ns:     namespace,
	}

	// Add to watchers list
	key := kind + "/" + namespace
	s.watchers[key] = append(s.watchers[key], w)

	// Send initial events for existing objects
	if s.objects[kind] != nil {
		for objKey, obj := range s.objects[kind] {
			if len(objKey) > len(namespace)+1 && objKey[:len(namespace)] == namespace && objKey[len(namespace)] == '/' {
				select {
				case w.events <- WatchEvent{Type: Added, Object: obj}:
				default:
					// Channel is full, skip this event
				}
			}
		}
	}

	// Start cleanup goroutine
	go func() {
		<-w.stop
		s.removeWatcher(w)
	}()

	return WatchResult{
		Events: w.events,
		Stop:   w.stop,
	}, nil
}

// Close closes the store and releases resources
func (s *memoryStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Stop all watchers
	for _, watchers := range s.watchers {
		for _, w := range watchers {
			w.mu.Lock()
			if !w.closed {
				close(w.stop)
				w.closed = true
			}
			w.mu.Unlock()
		}
	}

	// Clear objects and watchers
	s.objects = make(map[string]map[string]Object)
	s.watchers = make(map[string][]*watcher)

	return nil
}

// notifyWatchers notifies all watchers of a change
func (s *memoryStore) notifyWatchers(eventType EventType, obj Object) {
	kind := obj.GetKind()
	namespace := obj.GetNamespace()
	key := kind + "/" + namespace

	watchers := s.watchers[key]
	for _, w := range watchers {
		select {
		case w.events <- WatchEvent{Type: eventType, Object: obj}:
		default:
			// Channel is full, skip this event
		}
	}
}

// removeWatcher removes a watcher from the store
func (s *memoryStore) removeWatcher(w *watcher) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := w.kind + "/" + w.ns
	watchers := s.watchers[key]

	for i, watcher := range watchers {
		if watcher == w {
			// Remove watcher from slice
			s.watchers[key] = append(watchers[:i], watchers[i+1:]...)
			break
		}
	}

	// Clean up empty watcher lists
	if len(s.watchers[key]) == 0 {
		delete(s.watchers, key)
	}
}

// gcLoop runs garbage collection periodically
func (s *memoryStore) gcLoop() {
	ticker := time.NewTicker(s.options.GCInterval)
	defer ticker.Stop()

	for range ticker.C {
		s.gc()
	}
}

// gc performs garbage collection
func (s *memoryStore) gc() {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Clean up stopped watchers
	for key, watchers := range s.watchers {
		var active []*watcher
		for _, w := range watchers {
			select {
			case <-w.stop:
				// Watcher is stopped, skip it
			default:
				active = append(active, w)
			}
		}

		if len(active) == 0 {
			delete(s.watchers, key)
		} else {
			s.watchers[key] = active
		}
	}
}

// DeepCopy creates a deep copy of an object
func DeepCopy(obj Object) (Object, error) {
	data, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}

	// Create a new object of the same type
	var copy Object
	switch obj.GetKind() {
	case "Pod":
		copy = &api.Pod{}
	case "Node":
		copy = &api.Node{}
	default:
		return nil, fmt.Errorf("unknown object kind: %s", obj.GetKind())
	}

	err = json.Unmarshal(data, copy)
	return copy, err
}
