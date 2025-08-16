package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/minik8s/minik8s/pkg/api"
	"github.com/minik8s/minik8s/pkg/store"
)

func TestScheduler(t *testing.T) {
	// Create mock store
	mockStore := store.NewMemoryStore(store.DefaultOptions())

	// Create scheduler
	config := &Config{
		Store:               mockStore,
		DefaultNodeSelector: map[string]string{},
		SchedulingInterval:  10 * time.Second,
	}
	sched := NewScheduler(config)

	// Test starting scheduler
	ctx := context.Background()
	if err := sched.Start(ctx); err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}

	// Test that scheduler is running
	if !sched.running {
		t.Error("Scheduler should be running after Start()")
	}

	// Test stopping scheduler
	sched.Stop()
	if sched.running {
		t.Error("Scheduler should not be running after Stop()")
	}
}

func TestScheduler_FindBestNode(t *testing.T) {
	// Create mock store
	mockStore := store.NewMemoryStore(store.DefaultOptions())

	// Create scheduler
	config := &Config{
		Store:               mockStore,
		DefaultNodeSelector: map[string]string{},
		SchedulingInterval:  10 * time.Second,
	}
	sched := NewScheduler(config)

	// Create nodes
	node1 := &api.Node{
		TypeMeta: api.TypeMeta{
			Kind:       "Node",
			APIVersion: "v1alpha1",
		},
		ObjectMeta: api.ObjectMeta{
			Name:      "node-1",
			Namespace: "default",
			Labels: map[string]string{
				"zone": "us-west-1",
			},
		},
		Status: api.NodeStatus{
			Conditions: []api.NodeCondition{
				{
					Type:   "Ready",
					Status: "True",
				},
			},
			Allocatable: api.ResourceList{
				api.ResourceCPU:    "2",
				api.ResourceMemory: "4Gi",
			},
		},
	}

	node2 := &api.Node{
		TypeMeta: api.TypeMeta{
			Kind:       "Node",
			APIVersion: "v1alpha1",
		},
		ObjectMeta: api.ObjectMeta{
			Name:      "node-2",
			Namespace: "default",
			Labels: map[string]string{
				"zone": "us-east-1",
			},
		},
		Status: api.NodeStatus{
			Conditions: []api.NodeCondition{
				{
					Type:   "Ready",
					Status: "True",
				},
			},
			Allocatable: api.ResourceList{
				api.ResourceCPU:    "4",
				api.ResourceMemory: "8Gi",
			},
		},
	}

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
					Name:  "nginx",
					Image: "nginx:1.25",
					Resources: api.ResourceRequirements{
						Requests: api.ResourceList{
							api.ResourceCPU:    "100m",
							api.ResourceMemory: "128Mi",
						},
					},
				},
			},
		},
		Status: api.PodStatus{
			Phase: string(api.PodPending),
		},
	}

	// Test finding best node
	nodes := []store.Object{node1, node2}
	bestNode, err := sched.findBestNode(pod, nodes)
	if err != nil {
		t.Fatalf("Failed to find best node: %v", err)
	}

	// Node 2 should be selected as it has more resources
	if bestNode.GetName() != "node-2" {
		t.Errorf("Expected node-2 to be selected, got %s", bestNode.GetName())
	}
}

func TestScheduler_NodeSelectorMatching(t *testing.T) {
	// Create mock store
	mockStore := store.NewMemoryStore(store.DefaultOptions())

	// Create scheduler
	config := &Config{
		Store:               mockStore,
		DefaultNodeSelector: map[string]string{},
		SchedulingInterval:  10 * time.Second,
	}
	sched := NewScheduler(config)

	// Create a node with specific labels
	node := &api.Node{
		TypeMeta: api.TypeMeta{
			Kind:       "Node",
			APIVersion: "v1alpha1",
		},
		ObjectMeta: api.ObjectMeta{
			Name:      "test-node",
			Namespace: "default",
			Labels: map[string]string{
				"zone": "us-west-1",
				"env":  "production",
			},
		},
		Status: api.NodeStatus{
			Conditions: []api.NodeCondition{
				{
					Type:   "Ready",
					Status: "True",
				},
			},
		},
	}

	// Test pod with matching node selector
	podWithSelector := &api.Pod{
		TypeMeta: api.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1alpha1",
		},
		ObjectMeta: api.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
		Spec: api.PodSpec{
			NodeSelector: map[string]string{
				"zone": "us-west-1",
			},
			Containers: []api.Container{
				{
					Name:  "nginx",
					Image: "nginx:1.25",
				},
			},
		},
	}

	if !sched.matchesNodeSelector(podWithSelector, node) {
		t.Error("Pod should match node selector")
	}

	// Test pod with non-matching node selector
	podWithNonMatchingSelector := &api.Pod{
		TypeMeta: api.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1alpha1",
		},
		ObjectMeta: api.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
		Spec: api.PodSpec{
			NodeSelector: map[string]string{
				"zone": "us-east-1",
			},
			Containers: []api.Container{
				{
					Name:  "nginx",
					Image: "nginx:1.25",
				},
			},
		},
	}

	if sched.matchesNodeSelector(podWithNonMatchingSelector, node) {
		t.Error("Pod should not match node selector")
	}

	// Test pod without node selector
	podWithoutSelector := &api.Pod{
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
					Name:  "nginx",
					Image: "nginx:1.25",
				},
			},
		},
	}

	if !sched.matchesNodeSelector(podWithoutSelector, node) {
		t.Error("Pod without selector should match any node")
	}
}

func TestScheduler_ResourceRequirements(t *testing.T) {
	// Create mock store
	mockStore := store.NewMemoryStore(store.DefaultOptions())

	// Create scheduler
	config := &Config{
		Store:               mockStore,
		DefaultNodeSelector: map[string]string{},
		SchedulingInterval:  10 * time.Second,
	}
	sched := NewScheduler(config)

	// Create a node with limited resources
	node := &api.Node{
		TypeMeta: api.TypeMeta{
			Kind:       "Node",
			APIVersion: "v1alpha1",
		},
		ObjectMeta: api.ObjectMeta{
			Name:      "test-node",
			Namespace: "default",
		},
		Status: api.NodeStatus{
			Conditions: []api.NodeCondition{
				{
					Type:   "Ready",
					Status: "True",
				},
			},
			Allocatable: api.ResourceList{
				api.ResourceCPU:    "1",
				api.ResourceMemory: "1Gi",
			},
		},
	}

	// Test pod with acceptable resource requirements
	podWithAcceptableResources := &api.Pod{
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
					Name:  "nginx",
					Image: "nginx:1.25",
					Resources: api.ResourceRequirements{
						Requests: api.ResourceList{
							api.ResourceCPU:    "500m",
							api.ResourceMemory: "512Mi",
						},
					},
				},
			},
		},
	}

	if !sched.hasSufficientResources(podWithAcceptableResources, node) {
		t.Error("Pod should have sufficient resources")
	}

	// Test pod with excessive resource requirements
	podWithExcessiveResources := &api.Pod{
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
					Name:  "nginx",
					Image: "nginx:1.25",
					Resources: api.ResourceRequirements{
						Requests: api.ResourceList{
							api.ResourceCPU:    "2",
							api.ResourceMemory: "2Gi",
						},
					},
				},
			},
		},
	}

	if sched.hasSufficientResources(podWithExcessiveResources, node) {
		t.Error("Pod should not have sufficient resources")
	}
}

func TestScheduler_NodeScoring(t *testing.T) {
	// Create mock store
	mockStore := store.NewMemoryStore(store.DefaultOptions())

	// Create scheduler
	config := &Config{
		Store:               mockStore,
		DefaultNodeSelector: map[string]string{},
		SchedulingInterval:  10 * time.Second,
	}
	sched := NewScheduler(config)

	// Create nodes with different resource capacities
	node1 := &api.Node{
		TypeMeta: api.TypeMeta{
			Kind:       "Node",
			APIVersion: "v1alpha1",
		},
		ObjectMeta: api.ObjectMeta{
			Name:      "node-1",
			Namespace: "default",
		},
		Status: api.NodeStatus{
			Conditions: []api.NodeCondition{
				{
					Type:   "Ready",
					Status: "True",
				},
			},
			Allocatable: api.ResourceList{
				api.ResourceCPU:    "2",
				api.ResourceMemory: "4Gi",
			},
		},
	}

	node2 := &api.Node{
		TypeMeta: api.TypeMeta{
			Kind:       "Node",
			APIVersion: "v1alpha1",
		},
		ObjectMeta: api.ObjectMeta{
			Name:      "node-2",
			Namespace: "default",
		},
		Status: api.NodeStatus{
			Conditions: []api.NodeCondition{
				{
					Type:   "Ready",
					Status: "True",
				},
			},
			Allocatable: api.ResourceList{
				api.ResourceCPU:    "4",
				api.ResourceMemory: "8Gi",
			},
		},
	}

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
					Name:  "nginx",
					Image: "nginx:1.25",
				},
			},
		},
	}

	// Test node scoring
	score1 := sched.calculateNodeScore(pod, node1)
	score2 := sched.calculateNodeScore(pod, node2)

	// Node 2 should have a higher score due to more resources
	if score2 <= score1 {
		t.Errorf("Expected node2 score (%f) to be higher than node1 score (%f)", score2, score1)
	}
}
