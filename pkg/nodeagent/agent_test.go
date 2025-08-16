package nodeagent

import (
	"context"
	"testing"
	"time"

	"github.com/minik8s/minik8s/pkg/api"
	"github.com/minik8s/minik8s/pkg/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAgent(t *testing.T) {
	config := &Config{
		NodeName:          "test-node",
		APIServerURL:      "http://localhost:8080",
		Store:             store.NewMemoryStore(nil),
		CRIRuntime:        NewMockCRIRuntime(),
		NetworkManager:    &MockNetworkManager{},
		VolumeManager:     &MockVolumeManager{},
		HeartbeatInterval: 30 * time.Second,
	}

	agent := NewAgent(config)
	assert.NotNil(t, agent)
	assert.Equal(t, "test-node", agent.nodeName)
	assert.Equal(t, "http://localhost:8080", agent.apiServerURL)
	assert.Equal(t, 30*time.Second, agent.heartbeatInterval)
}

func TestAgent_StartStop(t *testing.T) {
	config := &Config{
		NodeName:          "test-node",
		APIServerURL:      "http://localhost:8080",
		Store:             store.NewMemoryStore(nil),
		CRIRuntime:        NewMockCRIRuntime(),
		NetworkManager:    &MockNetworkManager{},
		VolumeManager:     &MockVolumeManager{},
		HeartbeatInterval: 30 * time.Second,
	}

	agent := NewAgent(config)

	// Test starting
	ctx := context.Background()
	err := agent.Start(ctx)
	require.NoError(t, err)
	assert.True(t, agent.running)

	// Test starting again (should fail)
	err = agent.Start(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already running")

	// Test stopping
	agent.Stop()
	assert.False(t, agent.running)

	// Test stopping again (should not panic)
	agent.Stop()
}

func TestAgent_InitializeNodeStatus(t *testing.T) {
	mockRuntime := NewMockCRIRuntime()
	config := &Config{
		NodeName:          "test-node",
		APIServerURL:      "http://localhost:8080",
		Store:             store.NewMemoryStore(nil),
		CRIRuntime:        mockRuntime,
		NetworkManager:    &MockNetworkManager{},
		VolumeManager:     &MockVolumeManager{},
		HeartbeatInterval: 30 * time.Second,
	}

	agent := NewAgent(config)

	err := agent.initializeNodeStatus()
	require.NoError(t, err)

	assert.NotNil(t, agent.nodeStatus)
	assert.Equal(t, "4", agent.nodeStatus.Capacity["cpu"])
	assert.Equal(t, "8Gi", agent.nodeStatus.Capacity["memory"])
	assert.Len(t, agent.nodeStatus.Conditions, 1)
	assert.Equal(t, "Ready", agent.nodeStatus.Conditions[0].Type)
	assert.Equal(t, "True", agent.nodeStatus.Conditions[0].Status)
}

func TestAgent_SyncPods(t *testing.T) {
	// Create a memory store and add a pod
	store := store.NewMemoryStore(nil)
	defer store.Close()

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
			NodeName: "test-node",
			Containers: []api.Container{
				{
					Name:  "test",
					Image: "nginx:latest",
				},
			},
		},
	}

	ctx := context.Background()
	err := store.Create(ctx, pod)
	require.NoError(t, err)

	config := &Config{
		NodeName:          "test-node",
		APIServerURL:      "http://localhost:8080",
		Store:             store,
		CRIRuntime:        NewMockCRIRuntime(),
		NetworkManager:    &MockNetworkManager{},
		VolumeManager:     &MockVolumeManager{},
		HeartbeatInterval: 30 * time.Second,
	}

	agent := NewAgent(config)

	// Test syncing pods
	err = agent.syncPods(ctx)
	require.NoError(t, err)

	// Check that the pod was processed
	agent.mu.RLock()
	podState, exists := agent.pods["default/test-pod"]
	agent.mu.RUnlock()

	assert.True(t, exists)
	assert.NotNil(t, podState)
	assert.Equal(t, pod, podState.Pod)
}

func TestAgent_SyncPod_NewPod(t *testing.T) {
	store := store.NewMemoryStore(nil)
	defer store.Close()

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
			NodeName: "test-node",
			Containers: []api.Container{
				{
					Name:  "test",
					Image: "nginx:latest",
				},
			},
		},
	}

	ctx := context.Background()
	err := store.Create(ctx, pod)
	require.NoError(t, err)

	config := &Config{
		NodeName:          "test-node",
		APIServerURL:      "http://localhost:8080",
		Store:             store,
		CRIRuntime:        NewMockCRIRuntime(),
		NetworkManager:    &MockNetworkManager{},
		VolumeManager:     &MockVolumeManager{},
		HeartbeatInterval: 30 * time.Second,
	}

	agent := NewAgent(config)

	// Test syncing a new pod
	err = agent.syncPod(ctx, pod)
	require.NoError(t, err)

	// Check that the pod was created
	agent.mu.RLock()
	podState, exists := agent.pods["default/test-pod"]
	agent.mu.RUnlock()

	assert.True(t, exists)
	assert.NotNil(t, podState)
	assert.Equal(t, string(api.PodRunning), podState.Status.Phase)
}

func TestAgent_SyncPod_ExistingPod(t *testing.T) {
	store := store.NewMemoryStore(nil)
	defer store.Close()

	pod := &api.Pod{
		TypeMeta: api.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1alpha1",
		},
		ObjectMeta: api.ObjectMeta{
			Name:            "test-pod",
			Namespace:       "default",
			ResourceVersion: "1",
		},
		Spec: api.PodSpec{
			NodeName: "test-node",
			Containers: []api.Container{
				{
					Name:  "test",
					Image: "nginx:latest",
				},
			},
		},
	}

	ctx := context.Background()
	err := store.Create(ctx, pod)
	require.NoError(t, err)

	config := &Config{
		NodeName:          "test-node",
		APIServerURL:      "http://localhost:8080",
		Store:             store,
		CRIRuntime:        NewMockCRIRuntime(),
		NetworkManager:    &MockNetworkManager{},
		VolumeManager:     &MockVolumeManager{},
		HeartbeatInterval: 30 * time.Second,
	}

	agent := NewAgent(config)

	// Create the pod first
	err = agent.syncPod(ctx, pod)
	require.NoError(t, err)

	// Update the pod
	pod.ObjectMeta.ResourceVersion = "2"
	pod.Spec.Containers[0].Image = "nginx:1.25"

	// Test syncing the updated pod
	err = agent.syncPod(ctx, pod)
	require.NoError(t, err)

	// Check that the pod was updated (recreated in this case)
	agent.mu.RLock()
	podState, exists := agent.pods["default/test-pod"]
	agent.mu.RUnlock()

	// Since we're using a simple update strategy (delete and recreate),
	// the pod should still exist
	assert.True(t, exists)
	assert.NotNil(t, podState)
}

func TestAgent_DeletePod(t *testing.T) {
	store := store.NewMemoryStore(nil)
	defer store.Close()

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
			NodeName: "test-node",
			Containers: []api.Container{
				{
					Name:  "test",
					Image: "nginx:latest",
				},
			},
		},
	}

	ctx := context.Background()
	err := store.Create(ctx, pod)
	require.NoError(t, err)

	config := &Config{
		NodeName:          "test-node",
		APIServerURL:      "http://localhost:8080",
		Store:             store,
		CRIRuntime:        NewMockCRIRuntime(),
		NetworkManager:    &MockNetworkManager{},
		VolumeManager:     &MockVolumeManager{},
		HeartbeatInterval: 30 * time.Second,
	}

	agent := NewAgent(config)

	// Create the pod first
	err = agent.syncPod(ctx, pod)
	require.NoError(t, err)

	// Verify pod exists
	agent.mu.RLock()
	_, exists := agent.pods["default/test-pod"]
	agent.mu.RUnlock()
	assert.True(t, exists)

	// Delete the pod
	err = agent.deletePod(ctx, "default", "test-pod")
	require.NoError(t, err)

	// Verify pod was removed
	agent.mu.RLock()
	_, exists = agent.pods["default/test-pod"]
	agent.mu.RUnlock()
	assert.False(t, exists)
}

func TestAgent_UpdatePodState(t *testing.T) {
	config := &Config{
		NodeName:          "test-node",
		APIServerURL:      "http://localhost:8080",
		Store:             store.NewMemoryStore(nil),
		CRIRuntime:        NewMockCRIRuntime(),
		NetworkManager:    &MockNetworkManager{},
		VolumeManager:     &MockVolumeManager{},
		HeartbeatInterval: 30 * time.Second,
	}

	agent := NewAgent(config)

	podState := &PodState{
		Pod: &api.Pod{
			ObjectMeta: api.ObjectMeta{
				Name:      "test-pod",
				Namespace: "default",
			},
		},
		Status:     &api.PodStatus{},
		Containers: make(map[string]*ContainerRuntimeState),
		Volumes:    make(map[string]*VolumeState),
		Created:    time.Now(),
		Updated:    time.Now(),
	}

	// Update pod state
	agent.updatePodState("default/test-pod", podState)

	// Verify it was stored
	agent.mu.RLock()
	storedState, exists := agent.pods["default/test-pod"]
	agent.mu.RUnlock()

	assert.True(t, exists)
	assert.Equal(t, podState, storedState)
	assert.True(t, storedState.Updated.After(storedState.Created))
}
