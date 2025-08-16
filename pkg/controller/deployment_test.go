package controller

import (
	"context"
	"testing"

	"github.com/minik8s/minik8s/pkg/api"
	"github.com/minik8s/minik8s/pkg/store"
)

func TestDeploymentController(t *testing.T) {
	// Create mock store
	mockStore := store.NewMemoryStore(store.DefaultOptions())

	// Create controller
	ctrl := NewDeploymentController(mockStore)

	// Test controller name
	if ctrl.Name() != "deployment-controller" {
		t.Errorf("Expected controller name 'deployment-controller', got '%s'", ctrl.Name())
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

func TestDeploymentController_SyncDeployment(t *testing.T) {
	// Create mock store
	mockStore := store.NewMemoryStore(store.DefaultOptions())

	// Create controller
	ctrl := NewDeploymentController(mockStore)

	// Create a deployment
	deployment := &api.Deployment{
		TypeMeta: api.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "v1alpha1",
		},
		ObjectMeta: api.ObjectMeta{
			Name:      "test-deployment",
			Namespace: "default",
			UID:       "test-uid",
		},
		Spec: api.DeploymentSpec{
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

	// Create deployment in store
	ctx := context.Background()
	if err := mockStore.Create(ctx, deployment); err != nil {
		t.Fatalf("Failed to create deployment: %v", err)
	}

	// Sync the deployment
	if err := ctrl.syncDeployment(ctx, deployment); err != nil {
		t.Fatalf("Failed to sync deployment: %v", err)
	}

	// Check that ReplicaSet was created
	replicaSets, err := mockStore.List(ctx, "ReplicaSet", "")
	if err != nil {
		t.Fatalf("Failed to list replicasets: %v", err)
	}

	if len(replicaSets) != 1 {
		t.Errorf("Expected 1 replicaset, got %d", len(replicaSets))
	}

	// Check that pods were created
	pods, err := mockStore.List(ctx, "Pod", "")
	if err != nil {
		t.Fatalf("Failed to list pods: %v", err)
	}

	if len(pods) != 2 {
		t.Errorf("Expected 2 pods, got %d", len(pods))
	}
}
