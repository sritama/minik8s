package store

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/minik8s/minik8s/pkg/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEtcdStore_Integration tests the etcd store integration
// This test requires a running etcd instance
func TestEtcdStore_Integration(t *testing.T) {
	// Skip if no etcd endpoint is available
	endpoint := os.Getenv("ETCD_ENDPOINT")
	if endpoint == "" {
		t.Skip("Skipping etcd integration test: ETCD_ENDPOINT not set")
	}

	// Create etcd store
	store, err := NewEtcdStore([]string{endpoint}, "/test", nil)
	require.NoError(t, err)
	defer store.Close()

	ctx := context.Background()

	// Test Create
	t.Run("Create", func(t *testing.T) {
		pod := &api.Pod{
			TypeMeta: api.TypeMeta{
				Kind:       "Pod",
				APIVersion: "v1alpha1",
			},
			ObjectMeta: api.ObjectMeta{
				Name:      "test-pod",
				Namespace: "default",
			},
			Spec: api.PodSpec{
				Containers: []api.Container{
					{
						Name:  "test",
						Image: "nginx:latest",
					},
				},
			},
		}

		err := store.Create(ctx, pod)
		require.NoError(t, err)
	})

	// Test Get
	t.Run("Get", func(t *testing.T) {
		pod, err := store.Get(ctx, "Pod", "default", "test-pod")
		require.NoError(t, err)
		assert.Equal(t, "test-pod", pod.GetName())
		assert.Equal(t, "default", pod.GetNamespace())
	})

	// Test List
	t.Run("List", func(t *testing.T) {
		pods, err := store.List(ctx, "Pod", "default")
		require.NoError(t, err)
		assert.Len(t, pods, 1)
	})

	// Test Update
	t.Run("Update", func(t *testing.T) {
		pod := &api.Pod{
			TypeMeta: api.TypeMeta{
				Kind:       "Pod",
				APIVersion: "v1alpha1",
			},
			ObjectMeta: api.ObjectMeta{
				Name:      "test-pod",
				Namespace: "default",
			},
			Spec: api.PodSpec{
				Containers: []api.Container{
					{
						Name:  "test",
						Image: "nginx:1.25",
					},
				},
			},
		}

		err := store.Update(ctx, pod)
		require.NoError(t, err)

		// Verify update
		retrieved, err := store.Get(ctx, "Pod", "default", "test-pod")
		require.NoError(t, err)
		if retrievedPod, ok := retrieved.(*api.Pod); ok {
			assert.Equal(t, "nginx:1.25", retrievedPod.Spec.Containers[0].Image)
		}
	})

	// Test Watch
	t.Run("Watch", func(t *testing.T) {
		watchResult, err := store.Watch(ctx, "Pod", "default")
		require.NoError(t, err)

		// Create a pod in a goroutine
		go func() {
			time.Sleep(100 * time.Millisecond)
			pod := &api.Pod{
				TypeMeta: api.TypeMeta{
					Kind:       "Pod",
					APIVersion: "v1alpha1",
				},
				ObjectMeta: api.ObjectMeta{
					Name:      "watch-pod",
					Namespace: "default",
				},
				Spec: api.PodSpec{
					Containers: []api.Container{
						{
							Name:  "test",
							Image: "nginx:latest",
						},
					},
				},
			}
			store.Create(ctx, pod)
		}()

		// Wait for event
		select {
		case event := <-watchResult.Events:
			assert.Equal(t, Added, event.Type)
			if pod, ok := event.Object.(*api.Pod); ok {
				assert.Equal(t, "watch-pod", pod.GetName())
			}
		case <-time.After(2 * time.Second):
			t.Fatal("Timeout waiting for watch event")
		}

		// Clean up
		close(watchResult.Stop)
	})

	// Test Delete
	t.Run("Delete", func(t *testing.T) {
		err := store.Delete(ctx, "Pod", "default", "test-pod")
		require.NoError(t, err)

		// Verify deletion
		_, err = store.Get(ctx, "Pod", "default", "test-pod")
		assert.Error(t, err)
	})
}

// TestEtcdStore_ConnectionFailure tests the store creation with invalid endpoints
func TestEtcdStore_ConnectionFailure(t *testing.T) {
	_, err := NewEtcdStore([]string{"invalid:9999"}, "/test", nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to connect to etcd")
}

// TestStoreFactory tests the store factory
func TestStoreFactory(t *testing.T) {
	t.Run("MemoryStore", func(t *testing.T) {
		config := &StoreConfig{
			Type: StoreTypeMemory,
		}

		store, err := NewStore(config)
		require.NoError(t, err)
		defer store.Close()

		// Test that it's actually a memory store
		_, ok := store.(*memoryStore)
		assert.True(t, ok)
	})

	t.Run("EtcdStore", func(t *testing.T) {
		// This will fail but we can test the factory logic
		config := &StoreConfig{
			Type:      StoreTypeEtcd,
			Endpoints: []string{"invalid:9999"},
		}

		_, err := NewStore(config)
		assert.Error(t, err)
	})

	t.Run("UnknownStoreType", func(t *testing.T) {
		config := &StoreConfig{
			Type: "unknown",
		}

		_, err := NewStore(config)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unknown store type")
	})
}

// TestStoreFactoryWithFallback tests the fallback functionality
func TestStoreFactoryWithFallback(t *testing.T) {
	config := &StoreConfig{
		Type:      StoreTypeEtcd,
		Endpoints: []string{"invalid:9999"},
	}

	store, err := NewStoreWithFallback(config)
	require.NoError(t, err)
	defer store.Close()

	// Should fallback to memory store
	_, ok := store.(*memoryStore)
	assert.True(t, ok)
}

// TestEtcdStore_KeyBuilding tests the key building functionality
func TestEtcdStore_KeyBuilding(t *testing.T) {
	// Create a mock etcd store to test key building
	store := &etcdStore{
		prefix: "/minik8s",
	}

	// Test with namespace
	key := store.buildKey("Pod", "default", "test-pod")
	assert.Equal(t, "/minik8s/Pod/default/test-pod", key)

	// Test without namespace (for nodes)
	key = store.buildKey("Node", "", "test-node")
	assert.Equal(t, "/minik8s/Node/test-node", key)
}
