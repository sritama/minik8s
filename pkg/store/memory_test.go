package store

import (
	"context"
	"testing"
	"time"

	"github.com/minik8s/minik8s/pkg/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryStore_Create(t *testing.T) {
	store := NewMemoryStore(nil)
	defer store.Close()

	ctx := context.Background()

	// Create a pod
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

	// Verify pod was created
	retrieved, err := store.Get(ctx, "Pod", "default", "test-pod")
	require.NoError(t, err)
	assert.Equal(t, "test-pod", retrieved.GetName())
	assert.Equal(t, "default", retrieved.GetNamespace())
	assert.NotEmpty(t, retrieved.GetResourceVersion())
	assert.NotZero(t, retrieved.GetCreationTimestamp())
}

func TestMemoryStore_Get(t *testing.T) {
	store := NewMemoryStore(nil)
	defer store.Close()

	ctx := context.Background()

	// Create a pod
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

	// Get the pod
	retrieved, err := store.Get(ctx, "Pod", "default", "test-pod")
	require.NoError(t, err)
	assert.Equal(t, "test-pod", retrieved.GetName())

	// Try to get non-existent pod
	_, err = store.Get(ctx, "Pod", "default", "non-existent")
	assert.Error(t, err)
}

func TestMemoryStore_List(t *testing.T) {
	store := NewMemoryStore(nil)
	defer store.Close()

	ctx := context.Background()

	// Create multiple pods
	pod1 := &api.Pod{
		TypeMeta: api.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1alpha1",
		},
		ObjectMeta: api.ObjectMeta{
			Name:      "pod-1",
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

	pod2 := &api.Pod{
		TypeMeta: api.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1alpha1",
		},
		ObjectMeta: api.ObjectMeta{
			Name:      "pod-2",
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

	err := store.Create(ctx, pod1)
	require.NoError(t, err)

	err = store.Create(ctx, pod2)
	require.NoError(t, err)

	// List pods
	pods, err := store.List(ctx, "Pod", "default")
	require.NoError(t, err)
	assert.Len(t, pods, 2)

	// List pods from non-existent namespace
	pods, err = store.List(ctx, "Pod", "non-existent")
	require.NoError(t, err)
	assert.Len(t, pods, 0)
}

func TestMemoryStore_Update(t *testing.T) {
	store := NewMemoryStore(nil)
	defer store.Close()

	ctx := context.Background()

	// Create a pod
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

	// Update the pod
	pod.Spec.Containers[0].Image = "nginx:1.25"
	err = store.Update(ctx, pod)
	require.NoError(t, err)

	// Verify update
	retrieved, err := store.Get(ctx, "Pod", "default", "test-pod")
	require.NoError(t, err)

	if retrievedPod, ok := retrieved.(*api.Pod); ok {
		assert.Equal(t, "nginx:1.25", retrievedPod.Spec.Containers[0].Image)
	}
}

func TestMemoryStore_Delete(t *testing.T) {
	store := NewMemoryStore(nil)
	defer store.Close()

	ctx := context.Background()

	// Create a pod
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

	// Delete the pod
	err = store.Delete(ctx, "Pod", "default", "test-pod")
	require.NoError(t, err)

	// Verify deletion
	_, err = store.Get(ctx, "Pod", "default", "test-pod")
	assert.Error(t, err)
}

func TestMemoryStore_Watch(t *testing.T) {
	store := NewMemoryStore(nil)
	defer store.Close()

	ctx := context.Background()

	// Start watching
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
		store.Create(ctx, pod)
	}()

	// Wait for event
	select {
	case event := <-watchResult.Events:
		assert.Equal(t, Added, event.Type)
		if pod, ok := event.Object.(*api.Pod); ok {
			assert.Equal(t, "test-pod", pod.GetName())
		}
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for watch event")
	}

	// Don't manually close the channel - let the store handle it
}

func TestMemoryStore_DuplicateCreate(t *testing.T) {
	store := NewMemoryStore(nil)
	defer store.Close()

	ctx := context.Background()

	// Create a pod
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

	// Try to create the same pod again
	err = store.Create(ctx, pod)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}
