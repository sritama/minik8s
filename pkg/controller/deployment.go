package controller

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/minik8s/minik8s/pkg/api"
	"github.com/minik8s/minik8s/pkg/store"
)

// DeploymentController manages Deployment resources
type DeploymentController struct {
	mu sync.RWMutex

	// Configuration
	store store.Store
	name  string

	// State
	running bool
	stopCh  chan struct{}

	// Deployment tracking
	deployments map[string]*DeploymentState
}

// DeploymentState tracks the state of a deployment
type DeploymentState struct {
	Deployment *api.Deployment
	ReplicaSet *api.ReplicaSet
	Pods       []*api.Pod
	Updated    time.Time
}

// NewDeploymentController creates a new deployment controller
func NewDeploymentController(store store.Store) *DeploymentController {
	return &DeploymentController{
		store:       store,
		name:        "deployment-controller",
		deployments: make(map[string]*DeploymentState),
		stopCh:      make(chan struct{}),
	}
}

// Name returns the name of the controller
func (d *DeploymentController) Name() string {
	return d.name
}

// Start starts the deployment controller
func (d *DeploymentController) Start(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.running {
		return fmt.Errorf("deployment controller is already running")
	}

	// Start background goroutines
	go d.watchLoop(ctx)

	d.running = true
	return nil
}

// Stop stops the deployment controller
func (d *DeploymentController) Stop() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.running {
		return nil
	}

	close(d.stopCh)
	d.running = false
	return nil
}

// Sync performs a single sync operation
func (d *DeploymentController) Sync(ctx context.Context) error {
	return d.syncDeployments(ctx)
}

// watchLoop continuously watches for deployment changes
func (d *DeploymentController) watchLoop(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-d.stopCh:
			return
		case <-ticker.C:
			if err := d.syncDeployments(ctx); err != nil {
				// Log error but continue
				fmt.Printf("Error syncing deployments: %v\n", err)
			}
		}
	}
}

// syncDeployments syncs all deployments
func (d *DeploymentController) syncDeployments(ctx context.Context) error {
	// Get all deployments
	deployments, err := d.store.List(ctx, "Deployment", "")
	if err != nil {
		return fmt.Errorf("failed to list deployments: %w", err)
	}

	// Sync each deployment
	for _, obj := range deployments {
		if deployment, ok := obj.(*api.Deployment); ok {
			if err := d.syncDeployment(ctx, deployment); err != nil {
				fmt.Printf("Error syncing deployment %s: %v\n", deployment.Name, err)
			}
		}
	}

	return nil
}

// syncDeployment syncs a single deployment
func (d *DeploymentController) syncDeployment(ctx context.Context, deployment *api.Deployment) error {
	deploymentKey := fmt.Sprintf("%s/%s", deployment.Namespace, deployment.Name)

	// Get or create deployment state
	d.mu.Lock()
	state, exists := d.deployments[deploymentKey]
	if !exists {
		state = &DeploymentState{
			Deployment: deployment,
			Pods:       []*api.Pod{},
			Updated:    time.Now(),
		}
		d.deployments[deploymentKey] = state
	}
	d.mu.Unlock()

	// Check if deployment needs update
	if state.Deployment.ResourceVersion != deployment.ResourceVersion {
		state.Deployment = deployment
		state.Updated = time.Now()
	}

	// Ensure ReplicaSet exists
	if err := d.ensureReplicaSet(ctx, deployment, state); err != nil {
		return fmt.Errorf("failed to ensure replicaset: %w", err)
	}

	// Ensure correct number of pods
	if err := d.ensurePods(ctx, deployment, state); err != nil {
		return fmt.Errorf("failed to ensure pods: %w", err)
	}

	return nil
}

// ensureReplicaSet ensures the ReplicaSet for a deployment exists
func (d *DeploymentController) ensureReplicaSet(ctx context.Context, deployment *api.Deployment, state *DeploymentState) error {
	// Check if ReplicaSet already exists
	if state.ReplicaSet != nil {
		// Check if it needs update
		if state.ReplicaSet.Spec.Template.Spec.Containers[0].Image == deployment.Spec.Template.Spec.Containers[0].Image {
			return nil // No update needed
		}
	}

	// Create new ReplicaSet
	replicaSet := &api.ReplicaSet{
		TypeMeta: api.TypeMeta{
			Kind:       "ReplicaSet",
			APIVersion: "v1alpha1",
		},
		ObjectMeta: api.ObjectMeta{
			Name:      fmt.Sprintf("%s-%d", deployment.Name, time.Now().Unix()),
			Namespace: deployment.Namespace,
			Labels:    deployment.Spec.Selector.MatchLabels,
			OwnerReferences: []api.OwnerReference{
				{
					APIVersion: deployment.APIVersion,
					Kind:       deployment.Kind,
					Name:       deployment.Name,
					UID:        deployment.UID,
				},
			},
		},
		Spec: api.ReplicaSetSpec{
			Replicas: deployment.Spec.Replicas,
			Selector: deployment.Spec.Selector,
			Template: deployment.Spec.Template,
		},
		Status: api.ReplicaSetStatus{
			Replicas: 0,
		},
	}

	// Create ReplicaSet in store
	if err := d.store.Create(ctx, replicaSet); err != nil {
		return fmt.Errorf("failed to create replicaset: %w", err)
	}

	// Update state
	state.ReplicaSet = replicaSet
	state.Updated = time.Now()

	fmt.Printf("Created ReplicaSet %s for deployment %s\n", replicaSet.Name, deployment.Name)
	return nil
}

// ensurePods ensures the correct number of pods exist
func (d *DeploymentController) ensurePods(ctx context.Context, deployment *api.Deployment, state *DeploymentState) error {
	if state.ReplicaSet == nil {
		return fmt.Errorf("no replicaset for deployment %s", deployment.Name)
	}

	// Get current pods for this ReplicaSet
	pods, err := d.store.List(ctx, "Pod", "")
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}

	var currentPods []*api.Pod
	for _, obj := range pods {
		if pod, ok := obj.(*api.Pod); ok {
			// Check if pod belongs to this ReplicaSet
			if d.podBelongsToReplicaSet(pod, state.ReplicaSet) {
				currentPods = append(currentPods, pod)
			}
		}
	}

	desiredReplicas := deployment.Spec.Replicas
	currentReplicas := int32(len(currentPods))

	fmt.Printf("Deployment %s: desired=%d, current=%d\n", deployment.Name, desiredReplicas, currentReplicas)

	// Scale up if needed
	if currentReplicas < desiredReplicas {
		podsToCreate := desiredReplicas - currentReplicas
		for i := int32(0); i < podsToCreate; i++ {
			if err := d.createPod(ctx, deployment, state.ReplicaSet); err != nil {
				fmt.Printf("Failed to create pod for deployment %s: %v\n", deployment.Name, err)
			}
		}
	}

	// Scale down if needed
	if currentReplicas > desiredReplicas {
		podsToDelete := currentReplicas - desiredReplicas
		for i := int32(0); i < podsToDelete; i++ {
			if int(i) < len(currentPods) {
				if err := d.deletePod(ctx, currentPods[i]); err != nil {
					fmt.Printf("Failed to delete pod for deployment %s: %v\n", deployment.Name, err)
				}
			}
		}
	}

	// Update ReplicaSet status
	state.ReplicaSet.Status.Replicas = int32(len(currentPods))
	if err := d.store.Update(ctx, state.ReplicaSet); err != nil {
		return fmt.Errorf("failed to update replicaset status: %w", err)
	}

	return nil
}

// createPod creates a new pod for a deployment
func (d *DeploymentController) createPod(ctx context.Context, deployment *api.Deployment, replicaSet *api.ReplicaSet) error {
	// Create pod from template
	pod := &api.Pod{
		TypeMeta: api.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1alpha1",
		},
		ObjectMeta: replicaSet.Spec.Template.ObjectMeta,
		Spec:       replicaSet.Spec.Template.Spec,
		Status: api.PodStatus{
			Phase: string(api.PodPending),
		},
	}

	// Generate unique name
	pod.Name = fmt.Sprintf("%s-%s", replicaSet.Name, strconv.FormatInt(time.Now().UnixNano(), 10))

	// Set owner reference
	pod.OwnerReferences = []api.OwnerReference{
		{
			APIVersion: replicaSet.APIVersion,
			Kind:       replicaSet.Kind,
			Name:       replicaSet.Name,
			UID:        replicaSet.UID,
		},
	}

	// Create pod in store
	if err := d.store.Create(ctx, pod); err != nil {
		return fmt.Errorf("failed to create pod: %w", err)
	}

	fmt.Printf("Created pod %s for deployment %s\n", pod.Name, deployment.Name)
	return nil
}

// deletePod deletes a pod
func (d *DeploymentController) deletePod(ctx context.Context, pod *api.Pod) error {
	if err := d.store.Delete(ctx, "Pod", pod.Namespace, pod.Name); err != nil {
		return fmt.Errorf("failed to delete pod: %w", err)
	}

	fmt.Printf("Deleted pod %s\n", pod.Name)
	return nil
}

// podBelongsToReplicaSet checks if a pod belongs to a ReplicaSet
func (d *DeploymentController) podBelongsToReplicaSet(pod *api.Pod, replicaSet *api.ReplicaSet) bool {
	// Check owner references
	for _, ownerRef := range pod.OwnerReferences {
		if ownerRef.Kind == "ReplicaSet" && ownerRef.Name == replicaSet.Name {
			return true
		}
	}
	return false
}

// GetDeploymentState returns the state of a deployment
func (d *DeploymentController) GetDeploymentState(namespace, name string) *DeploymentState {
	d.mu.RLock()
	defer d.mu.RUnlock()

	key := fmt.Sprintf("%s/%s", namespace, name)
	return d.deployments[key]
}

// ListDeploymentStates returns all deployment states
func (d *DeploymentController) ListDeploymentStates() map[string]*DeploymentState {
	d.mu.RLock()
	defer d.mu.RUnlock()

	result := make(map[string]*DeploymentState)
	for k, v := range d.deployments {
		result[k] = v
	}
	return result
}
