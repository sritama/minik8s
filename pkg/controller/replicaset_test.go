package controller

import (
	"context"
	"testing"
	"time"

	"github.com/minik8s/minik8s/pkg/api"
	"github.com/minik8s/minik8s/pkg/store"
)

func TestReplicaSetController(t *testing.T) {
	// Create mock store
	mockStore := store.NewMemoryStore(store.DefaultOptions())

	// Create controller
	ctrl := NewReplicaSetController(mockStore)

	// Test controller name
	if ctrl.Name() != "replicaset-controller" {
		t.Errorf("Expected controller name 'replicaset-controller', got '%s'", ctrl.Name())
	}

	// Test starting controller
	ctx := context.Background()
	if err := ctrl.Start(ctx); err != nil {
		t.Fatalf("Failed to start controller: %v", err)
	}

	// Test that controller is running
	if !ctrl.running {
		t.Error("Controller should be running after Start()")
	}

	// Test stopping controller
	ctrl.Stop()
	if ctrl.running {
		t.Error("Controller should not be running after Stop()")
	}
}

func TestReplicaSetController_SyncReplicaSet(t *testing.T) {
	// Create mock store
	mockStore := store.NewMemoryStore(store.DefaultOptions())

	// Create controller
	ctrl := NewReplicaSetController(mockStore)

	// Create a ReplicaSet
	replicaSet := &api.ReplicaSet{
		TypeMeta: api.TypeMeta{
			Kind:       "ReplicaSet",
			APIVersion: "v1alpha1",
		},
		ObjectMeta: api.ObjectMeta{
			Name:      "test-replicaset",
			Namespace: "default",
			UID:       "test-uid",
		},
		Spec: api.ReplicaSetSpec{
			Replicas: 2,
			Selector: &api.LabelSelector{
				MatchLabels: map[string]string{
					"app": "nginx",
				},
			},
			Template: api.PodTemplateSpec{
				ObjectMeta: api.ObjectMeta{
					Labels: map[string]string{
						"app": "nginx",
					},
				},
				Spec: api.PodSpec{
					Containers: []api.Container{
						{
							Name:  "nginx",
							Image: "nginx:1.25",
						},
					},
				},
			},
		},
	}

	// Create ReplicaSet in store
	ctx := context.Background()
	if err := mockStore.Create(ctx, replicaSet); err != nil {
		t.Fatalf("Failed to create replicaset: %v", err)
	}

	// Sync the ReplicaSet
	if err := ctrl.syncReplicaSet(ctx, replicaSet); err != nil {
		t.Fatalf("Failed to sync replicaset: %v", err)
	}

	// Check that pods were created
	pods, err := mockStore.List(ctx, "Pod", "")
	if err != nil {
		t.Fatalf("Failed to list pods: %v", err)
	}

	if len(pods) != 2 {
		t.Errorf("Expected 2 pods, got %d", len(pods))
	}

	// Check that pods have correct owner references
	for _, obj := range pods {
		pod, ok := obj.(*api.Pod)
		if !ok {
			continue
		}

		if len(pod.OwnerReferences) != 1 {
			t.Errorf("Expected 1 owner reference, got %d", len(pod.OwnerReferences))
		}

		ownerRef := pod.OwnerReferences[0]
		if ownerRef.Kind != "ReplicaSet" || ownerRef.Name != "test-replicaset" {
			t.Errorf("Expected owner reference to ReplicaSet 'test-replicaset', got %s '%s'", ownerRef.Kind, ownerRef.Name)
		}
	}
}

func TestReplicaSetController_ScaleDown(t *testing.T) {
	// Create mock store
	mockStore := store.NewMemoryStore(store.DefaultOptions())

	// Create controller
	ctrl := NewReplicaSetController(mockStore)

	// Create a ReplicaSet with 3 replicas
	replicaSet := &api.ReplicaSet{
		TypeMeta: api.TypeMeta{
			Kind:       "ReplicaSet",
			APIVersion: "v1alpha1",
		},
		ObjectMeta: api.ObjectMeta{
			Name:      "test-replicaset",
			Namespace: "default",
			UID:       "test-uid",
		},
		Spec: api.ReplicaSetSpec{
			Replicas: 3,
			Selector: &api.LabelSelector{
				MatchLabels: map[string]string{
					"app": "nginx",
				},
			},
			Template: api.PodTemplateSpec{
				ObjectMeta: api.ObjectMeta{
					Labels: map[string]string{
						"app": "nginx",
					},
				},
				Spec: api.PodSpec{
					Containers: []api.Container{
						{
							Name:  "nginx",
							Image: "nginx:1.25",
						},
					},
				},
			},
		},
	}

	// Create ReplicaSet in store
	ctx := context.Background()
	if err := mockStore.Create(ctx, replicaSet); err != nil {
		t.Fatalf("Failed to create replicaset: %v", err)
	}

	// Sync to create 3 pods
	if err := ctrl.syncReplicaSet(ctx, replicaSet); err != nil {
		t.Fatalf("Failed to sync replicaset: %v", err)
	}

	// Check that 3 pods were created
	pods, err := mockStore.List(ctx, "Pod", "")
	if err != nil {
		t.Fatalf("Failed to list pods: %v", err)
	}

	if len(pods) != 3 {
		t.Errorf("Expected 3 pods, got %d", len(pods))
	}

	// Scale down to 1 replica
	replicaSet.Spec.Replicas = 1
	if err := mockStore.Update(ctx, replicaSet); err != nil {
		t.Fatalf("Failed to update replicaset: %v", err)
	}

	// Sync again to scale down
	if err := ctrl.syncReplicaSet(ctx, replicaSet); err != nil {
		t.Fatalf("Failed to sync replicaset: %v", err)
	}

	// Check that only 1 pod remains
	pods, err = mockStore.List(ctx, "Pod", "")
	if err != nil {
		t.Fatalf("Failed to list pods: %v", err)
	}

	if len(pods) != 1 {
		t.Errorf("Expected 1 pod after scale down, got %d", len(pods))
	}
}

func TestReplicaSetController_PodBelongsToReplicaSet(t *testing.T) {
	// Create mock store
	mockStore := store.NewMemoryStore(store.DefaultOptions())

	// Create controller
	ctrl := NewReplicaSetController(mockStore)

	// Create a ReplicaSet
	replicaSet := &api.ReplicaSet{
		TypeMeta: api.TypeMeta{
			Kind:       "ReplicaSet",
			APIVersion: "v1alpha1",
		},
		ObjectMeta: api.ObjectMeta{
			Name:      "test-replicaset",
			Namespace: "default",
			UID:       "test-uid",
		},
		Spec: api.ReplicaSetSpec{
			Replicas: 1,
			Selector: &api.LabelSelector{
				MatchLabels: map[string]string{
					"app": "nginx",
				},
			},
			Template: api.PodTemplateSpec{
				ObjectMeta: api.ObjectMeta{
					Labels: map[string]string{
						"app": "nginx",
					},
				},
				Spec: api.PodSpec{
					Containers: []api.Container{
						{
							Name:  "nginx",
							Image: "nginx:1.25",
						},
					},
				},
			},
		},
	}

	// Create a pod that belongs to the ReplicaSet
	pod := &api.Pod{
		TypeMeta: api.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1alpha1",
		},
		ObjectMeta: api.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			OwnerReferences: []api.OwnerReference{
				{
					APIVersion: "v1alpha1",
					Kind:       "ReplicaSet",
					Name:       "test-replicaset",
					UID:        "test-uid",
				},
			},
		},
		Spec: api.PodSpec{
			Containers: []api.Container{
				{
					Name:  "nginx",
					Image: "nginx:1.25",
				},
			},
		},
	}

	// Test that pod belongs to ReplicaSet
	if !ctrl.podBelongsToReplicaSet(pod, replicaSet) {
		t.Error("Pod should belong to ReplicaSet")
	}

	// Create a pod that doesn't belong to the ReplicaSet
	otherPod := &api.Pod{
		TypeMeta: api.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1alpha1",
		},
		ObjectMeta: api.ObjectMeta{
			Name:      "other-pod",
			Namespace: "default",
			OwnerReferences: []api.OwnerReference{
				{
					APIVersion: "v1alpha1",
					Kind:       "ReplicaSet",
					Name:       "other-replicaset",
					UID:        "other-uid",
				},
			},
		},
		Spec: api.PodSpec{
			Containers: []api.Container{
				{
					Name:  "nginx",
					Image: "nginx:1.25",
				},
			},
		},
	}

	// Test that pod doesn't belong to ReplicaSet
	if ctrl.podBelongsToReplicaSet(otherPod, replicaSet) {
		t.Error("Pod should not belong to ReplicaSet")
	}
}

func TestReplicaSetController_UpdateStatus(t *testing.T) {
	// Create mock store
	mockStore := store.NewMemoryStore(store.DefaultOptions())

	// Create controller
	ctrl := NewReplicaSetController(mockStore)

	// Create a ReplicaSet
	replicaSet := &api.ReplicaSet{
		TypeMeta: api.TypeMeta{
			Kind:       "ReplicaSet",
			APIVersion: "v1alpha1",
		},
		ObjectMeta: api.ObjectMeta{
			Name:      "test-replicaset",
			Namespace: "default",
			UID:       "test-uid",
		},
		Spec: api.ReplicaSetSpec{
			Replicas: 2,
			Selector: &api.LabelSelector{
				MatchLabels: map[string]string{
					"app": "nginx",
				},
			},
			Template: api.PodTemplateSpec{
				ObjectMeta: api.ObjectMeta{
					Labels: map[string]string{
						"app": "nginx",
					},
				},
				Spec: api.PodSpec{
					Containers: []api.Container{
						{
							Name:  "nginx",
							Image: "nginx:1.25",
						},
					},
				},
			},
		},
	}

	// Create ReplicaSet in store
	ctx := context.Background()
	if err := mockStore.Create(ctx, replicaSet); err != nil {
		t.Fatalf("Failed to create replicaset: %v", err)
	}

	// Create state
	state := &ReplicaSetState{
		ReplicaSet: replicaSet,
		Pods:       []*api.Pod{},
		Updated:    time.Now(),
	}

	// Update status
	if err := ctrl.updateReplicaSetStatus(ctx, replicaSet, state); err != nil {
		t.Fatalf("Failed to update status: %v", err)
	}

	// Check that status was updated
	if replicaSet.Status.Replicas != 0 {
		t.Errorf("Expected 0 replicas, got %d", replicaSet.Status.Replicas)
	}

	if replicaSet.Status.ReadyReplicas != 0 {
		t.Errorf("Expected 0 ready replicas, got %d", replicaSet.Status.ReadyReplicas)
	}
}
