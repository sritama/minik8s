package store

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/minik8s/minik8s/pkg/api"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// etcdStore implements the Store interface using etcd
type etcdStore struct {
	client   *clientv3.Client
	prefix   string
	options  *Options
	mu       sync.RWMutex
	watchers map[string][]*etcdWatcher
	leaseID  clientv3.LeaseID
	leaseTTL int64
}

// etcdWatcher represents a watch subscription in etcd
type etcdWatcher struct {
	events     chan WatchEvent
	stop       chan struct{}
	kind       string
	ns         string
	cancelFunc context.CancelFunc
}

// NewEtcdStore creates a new etcd store
func NewEtcdStore(endpoints []string, prefix string, options *Options) (Store, error) {
	if options == nil {
		options = DefaultOptions()
	}

	// Create etcd client
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: 5 * time.Second,
		Username:    "", // Add if authentication is needed
		Password:    "", // Add if authentication is needed
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create etcd client: %w", err)
	}

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = client.Get(ctx, "test", clientv3.WithCountOnly())
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to connect to etcd: %w", err)
	}

	store := &etcdStore{
		client:   client,
		prefix:   prefix,
		options:  options,
		watchers: make(map[string][]*etcdWatcher),
		leaseTTL: 30, // 30 seconds TTL for leases
	}

	// Create a lease for TTL operations
	lease, err := client.Grant(ctx, store.leaseTTL)
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to create lease: %w", err)
	}
	store.leaseID = lease.ID

	// Start lease keepalive
	go store.keepAliveLease()

	return store, nil
}

// Create creates a new object in etcd
func (s *etcdStore) Create(ctx context.Context, obj Object) error {
	key := s.buildKey(obj.GetKind(), obj.GetNamespace(), obj.GetName())

	// Check if object already exists
	resp, err := s.client.Get(ctx, key)
	if err != nil {
		return fmt.Errorf("failed to check existing object: %w", err)
	}

	if len(resp.Kvs) > 0 {
		return fmt.Errorf("object %s/%s of kind %s already exists", obj.GetNamespace(), obj.GetName(), obj.GetKind())
	}

	// Set metadata
	obj.SetResourceVersion(fmt.Sprintf("%d", time.Now().UnixNano()))
	obj.SetCreationTimestamp(time.Now())

	// Serialize object
	data, err := json.Marshal(obj)
	if err != nil {
		return fmt.Errorf("failed to marshal object: %w", err)
	}

	// Store with lease for TTL
	_, err = s.client.Put(ctx, key, string(data), clientv3.WithLease(s.leaseID))
	if err != nil {
		return fmt.Errorf("failed to store object: %w", err)
	}

	// Notify watchers
	s.notifyWatchers(Added, obj)

	return nil
}

// Get retrieves an object by name and namespace
func (s *etcdStore) Get(ctx context.Context, kind, namespace, name string) (Object, error) {
	key := s.buildKey(kind, namespace, name)

	resp, err := s.client.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("failed to get object: %w", err)
	}

	if len(resp.Kvs) == 0 {
		return nil, fmt.Errorf("object %s/%s of kind %s not found", namespace, name, kind)
	}

	// Deserialize object
	var obj Object
	switch kind {
	case "Pod":
		obj = &api.Pod{}
	case "Node":
		obj = &api.Node{}
	default:
		return nil, fmt.Errorf("unknown object kind: %s", kind)
	}

	err = json.Unmarshal(resp.Kvs[0].Value, obj)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal object: %w", err)
	}

	return obj, nil
}

// List retrieves all objects of a given kind and namespace
func (s *etcdStore) List(ctx context.Context, kind, namespace string) ([]Object, error) {
	prefix := s.buildKey(kind, namespace, "")

	resp, err := s.client.Get(ctx, prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("failed to list objects: %w", err)
	}

	var objects []Object
	for _, kv := range resp.Kvs {
		// Skip the prefix key itself
		if string(kv.Key) == prefix {
			continue
		}

		var obj Object
		switch kind {
		case "Pod":
			obj = &api.Pod{}
		case "Node":
			obj = &api.Node{}
		default:
			continue
		}

		err := json.Unmarshal(kv.Value, obj)
		if err != nil {
			continue // Skip malformed objects
		}

		objects = append(objects, obj)
	}

	return objects, nil
}

// Update updates an existing object
func (s *etcdStore) Update(ctx context.Context, obj Object) error {
	key := s.buildKey(obj.GetKind(), obj.GetNamespace(), obj.GetName())

	// Check if object exists
	resp, err := s.client.Get(ctx, key)
	if err != nil {
		return fmt.Errorf("failed to check existing object: %w", err)
	}

	if len(resp.Kvs) == 0 {
		return fmt.Errorf("object %s/%s of kind %s not found", obj.GetNamespace(), obj.GetName(), obj.GetKind())
	}

	// Update resource version
	obj.SetResourceVersion(fmt.Sprintf("%d", time.Now().UnixNano()))

	// Serialize object
	data, err := json.Marshal(obj)
	if err != nil {
		return fmt.Errorf("failed to marshal object: %w", err)
	}

	// Store with lease for TTL
	_, err = s.client.Put(ctx, key, string(data), clientv3.WithLease(s.leaseID))
	if err != nil {
		return fmt.Errorf("failed to update object: %w", err)
	}

	// Notify watchers
	s.notifyWatchers(Modified, obj)

	return nil
}

// Delete deletes an object by name and namespace
func (s *etcdStore) Delete(ctx context.Context, kind, namespace, name string) error {
	key := s.buildKey(kind, namespace, name)

	// Get object before deletion for watcher notification
	obj, err := s.Get(ctx, kind, namespace, name)
	if err != nil {
		return err
	}

	// Delete from etcd
	_, err = s.client.Delete(ctx, key)
	if err != nil {
		return fmt.Errorf("failed to delete object: %w", err)
	}

	// Notify watchers
	s.notifyWatchers(Deleted, obj)

	return nil
}

// Watch watches for changes to objects of a given kind and namespace
func (s *etcdStore) Watch(ctx context.Context, kind, namespace string) (WatchResult, error) {
	prefix := s.buildKey(kind, namespace, "")

	// Create watcher
	w := &etcdWatcher{
		events: make(chan WatchEvent, s.options.WatchBufferSize),
		stop:   make(chan struct{}),
		kind:   kind,
		ns:     namespace,
	}

	// Create context for etcd watch
	watchCtx, cancel := context.WithCancel(context.Background())
	w.cancelFunc = cancel

	// Start etcd watch
	go s.startEtcdWatch(watchCtx, w, prefix)

	// Add to watchers list
	s.mu.Lock()
	key := kind + "/" + namespace
	s.watchers[key] = append(s.watchers[key], w)
	s.mu.Unlock()

	// Send initial events for existing objects
	objects, err := s.List(ctx, kind, namespace)
	if err == nil {
		for _, obj := range objects {
			select {
			case w.events <- WatchEvent{Type: Added, Object: obj}:
			default:
				// Channel is full, skip this event
			}
		}
	}

	// Start cleanup goroutine
	go func() {
		<-w.stop
		s.removeWatcher(w)
		cancel()
	}()

	return WatchResult{
		Events: w.events,
		Stop:   w.stop,
	}, nil
}

// Close closes the etcd store and releases resources
func (s *etcdStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Stop all watchers
	for _, watchers := range s.watchers {
		for _, w := range watchers {
			close(w.stop)
		}
	}

	// Close etcd client
	if s.client != nil {
		return s.client.Close()
	}

	return nil
}

// buildKey builds the etcd key for an object
func (s *etcdStore) buildKey(kind, namespace, name string) string {
	if namespace == "" {
		return path.Join(s.prefix, kind, name)
	}
	return path.Join(s.prefix, kind, namespace, name)
}

// startEtcdWatch starts the etcd watch for a specific watcher
func (s *etcdStore) startEtcdWatch(ctx context.Context, w *etcdWatcher, prefix string) {
	watchChan := s.client.Watch(ctx, prefix, clientv3.WithPrefix())

	for {
		select {
		case resp := <-watchChan:
			if resp.Err() != nil {
				// Send error event
				select {
				case w.events <- WatchEvent{Type: Error, Object: nil}:
				default:
				}
				continue
			}

			for _, ev := range resp.Events {
				// Parse key to extract object info
				key := string(ev.Kv.Key)
				parts := strings.Split(strings.TrimPrefix(key, s.prefix+"/"), "/")
				if len(parts) < 2 {
					continue
				}

				kind := parts[0]
				if kind != w.kind {
					continue
				}

				var eventType EventType
				var obj Object

				switch ev.Type {
				case clientv3.EventTypePut:
					if ev.IsCreate() {
						eventType = Added
					} else {
						eventType = Modified
					}

					// Deserialize object
					switch kind {
					case "Pod":
						obj = &api.Pod{}
					case "Node":
						obj = &api.Node{}
					default:
						continue
					}

					err := json.Unmarshal(ev.Kv.Value, obj)
					if err != nil {
						continue
					}

				case clientv3.EventTypeDelete:
					eventType = Deleted
					// For delete events, we can't reconstruct the full object
					// We'll create a minimal object with just the metadata
					switch kind {
					case "Pod":
						obj = &api.Pod{
							ObjectMeta: api.ObjectMeta{
								Name:      parts[len(parts)-1],
								Namespace: parts[1],
							},
						}
					case "Node":
						obj = &api.Node{
							ObjectMeta: api.ObjectMeta{
								Name: parts[1],
							},
						}
					default:
						continue
					}
				}

				// Send event
				select {
				case w.events <- WatchEvent{Type: eventType, Object: obj}:
				default:
					// Channel is full, skip this event
				}
			}

		case <-ctx.Done():
			return
		}
	}
}

// notifyWatchers notifies all watchers of a change
func (s *etcdStore) notifyWatchers(eventType EventType, obj Object) {
	s.mu.RLock()
	defer s.mu.RUnlock()

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
func (s *etcdStore) removeWatcher(w *etcdWatcher) {
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

// keepAliveLease keeps the lease alive
func (s *etcdStore) keepAliveLease() {
	keepAlive, err := s.client.KeepAlive(context.Background(), s.leaseID)
	if err != nil {
		return
	}

	for {
		select {
		case resp := <-keepAlive:
			if resp == nil {
				// Lease expired, create a new one
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				lease, err := s.client.Grant(ctx, s.leaseTTL)
				cancel()

				if err == nil {
					s.leaseID = lease.ID
					keepAlive, err = s.client.KeepAlive(context.Background(), s.leaseID)
					if err != nil {
						return
					}
				}
			}
		}
	}
}

// NewEtcdStoreWithConfig creates a new etcd store with custom configuration
func NewEtcdStoreWithConfig(config clientv3.Config, prefix string, options *Options) (Store, error) {
	if options == nil {
		options = DefaultOptions()
	}

	// Create etcd client
	client, err := clientv3.New(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create etcd client: %w", err)
	}

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = client.Get(ctx, "test", clientv3.WithCountOnly())
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to connect to etcd: %w", err)
	}

	store := &etcdStore{
		client:   client,
		prefix:   prefix,
		options:  options,
		watchers: make(map[string][]*etcdWatcher),
		leaseTTL: 30, // 30 seconds TTL for leases
	}

	// Create a lease for TTL operations
	lease, err := client.Grant(ctx, store.leaseTTL)
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("failed to create lease: %w", err)
	}
	store.leaseID = lease.ID

	// Start lease keepalive
	go store.keepAliveLease()

	return store, nil
}
