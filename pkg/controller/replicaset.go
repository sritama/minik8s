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

// ReplicaSetController manages ReplicaSet resources
type ReplicaSetController struct {
	mu sync.RWMutex

	// Configuration
	store store.Store
	name  string

	// State
	running bool
	stopCh  chan struct{}

	// ReplicaSet tracking
	replicaSets map[string]*ReplicaSetState
}

// ReplicaSetState tracks the state of a ReplicaSet
type ReplicaSetState struct {
	ReplicaSet *api.ReplicaSet
	Pods       []*api.Pod
	Updated    time.Time
}

// NewReplicaSetController creates a new ReplicaSet controller
func NewReplicaSetController(store store.Store) *ReplicaSetController {
	return &ReplicaSetController{
		store:       store,
		name:        "replicaset-controller",
		replicaSets: make(map[string]*ReplicaSetState),
		stopCh:      make(chan struct{}),
	}
}

// Name returns the name of the controller
func (r *ReplicaSetController) Name() string {
	return r.name
}

// Start starts the ReplicaSet controller
func (r *ReplicaSetController) Start(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.running {
		return fmt.Errorf("replicaset controller is already running")
	}

	// Start background goroutines
	go r.watchLoop(ctx)

	r.running = true
	return nil
}

// Stop stops the ReplicaSet controller
func (r *ReplicaSetController) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.running {
		return nil
	}

	close(r.stopCh)
	r.running = false
	return nil
}

// Sync performs a single sync operation
func (r *ReplicaSetController) Sync(ctx context.Context) error {
	return r.syncReplicaSets(ctx)
}

// watchLoop continuously watches for ReplicaSet changes
func (r *ReplicaSetController) watchLoop(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-r.stopCh:
			return
		case <-ticker.C:
			if err := r.syncReplicaSets(ctx); err != nil {
				// Log error but continue
				fmt.Printf("Error syncing replicasets: %v\n", err)
			}
		}
	}
}

// syncReplicaSets syncs all ReplicaSets
func (r *ReplicaSetController) syncReplicaSets(ctx context.Context) error {
	// Get all ReplicaSets
	replicaSets, err := r.store.List(ctx, "ReplicaSet", "")
	if err != nil {
		return fmt.Errorf("failed to list replicasets: %w", err)
	}

	// Sync each ReplicaSet
	for _, obj := range replicaSets {
		if replicaSet, ok := obj.(*api.ReplicaSet); ok {
			if err := r.syncReplicaSet(ctx, replicaSet); err != nil {
				fmt.Printf("Error syncing replicaset %s: %v\n", replicaSet.Name, err)
			}
		}
	}

	return nil
}

// syncReplicaSet syncs a single ReplicaSet
func (r *ReplicaSetController) syncReplicaSet(ctx context.Context, replicaSet *api.ReplicaSet) error {
	replicaSetKey := fmt.Sprintf("%s/%s", replicaSet.Namespace, replicaSet.Name)

	// Get or create ReplicaSet state
	r.mu.Lock()
	state, exists := r.replicaSets[replicaSetKey]
	if !exists {
		state = &ReplicaSetState{
			ReplicaSet: replicaSet,
			Pods:       []*api.Pod{},
			Updated:    time.Now(),
		}
		r.replicaSets[replicaSetKey] = state
	}
	r.mu.Unlock()

	// Check if ReplicaSet needs update
	if state.ReplicaSet.ResourceVersion != replicaSet.ResourceVersion {
		state.ReplicaSet = replicaSet
		state.Updated = time.Now()
	}

	// Ensure correct number of pods
	if err := r.ensurePods(ctx, replicaSet, state); err != nil {
		return fmt.Errorf("failed to ensure pods: %w", err)
	}

	// Update ReplicaSet status
	if err := r.updateReplicaSetStatus(ctx, replicaSet, state); err != nil {
		return fmt.Errorf("failed to update replicaset status: %w", err)
	}

	return nil
}

// ensurePods ensures the correct number of pods exist
func (r *ReplicaSetController) ensurePods(ctx context.Context, replicaSet *api.ReplicaSet, state *ReplicaSetState) error {
	// Get current pods for this ReplicaSet
	pods, err := r.store.List(ctx, "Pod", "")
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}

	var currentPods []*api.Pod
	for _, obj := range pods {
		if pod, ok := obj.(*api.Pod); ok {
			// Check if pod belongs to this ReplicaSet
			if r.podBelongsToReplicaSet(pod, replicaSet) {
				currentPods = append(currentPods, pod)
			}
		}
	}

	desiredReplicas := replicaSet.Spec.Replicas
	currentReplicas := int32(len(currentPods))

	fmt.Printf("ReplicaSet %s: desired=%d, current=%d\n", replicaSet.Name, desiredReplicas, currentReplicas)

	// Scale up if needed
	if currentReplicas < desiredReplicas {
		podsToCreate := desiredReplicas - currentReplicas
		for i := int32(0); i < podsToCreate; i++ {
			if err := r.createPod(ctx, replicaSet); err != nil {
				fmt.Printf("Failed to create pod for replicaset %s: %v\n", replicaSet.Name, err)
			}
		}
	}

	// Scale down if needed
	if currentReplicas > desiredReplicas {
		podsToDelete := currentReplicas - desiredReplicas
		for i := int32(0); i < podsToDelete; i++ {
			if int(i) < len(currentPods) {
				if err := r.deletePod(ctx, currentPods[i]); err != nil {
					fmt.Printf("Failed to delete pod for replicaset %s: %v\n", replicaSet.Name, err)
				}
			}
		}
	}

	// Update state
	state.Pods = currentPods
	state.Updated = time.Now()

	return nil
}

// createPod creates a new pod for a ReplicaSet
func (r *ReplicaSetController) createPod(ctx context.Context, replicaSet *api.ReplicaSet) error {
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
	if err := r.store.Create(ctx, pod); err != nil {
		return fmt.Errorf("failed to create pod: %w", err)
	}

	fmt.Printf("Created pod %s for replicaset %s\n", pod.Name, replicaSet.Name)
	return nil
}

// deletePod deletes a pod
func (r *ReplicaSetController) deletePod(ctx context.Context, pod *api.Pod) error {
	if err := r.store.Delete(ctx, "Pod", pod.Namespace, pod.Name); err != nil {
		return fmt.Errorf("failed to delete pod: %w", err)
	}

	fmt.Printf("Deleted pod %s\n", pod.Name)
	return nil
}

// podBelongsToReplicaSet checks if a pod belongs to a ReplicaSet
func (r *ReplicaSetController) podBelongsToReplicaSet(pod *api.Pod, replicaSet *api.ReplicaSet) bool {
	// Check owner references
	for _, ownerRef := range pod.OwnerReferences {
		if ownerRef.Kind == "ReplicaSet" && ownerRef.Name == replicaSet.Name {
			return true
		}
	}
	return false
}

// updateReplicaSetStatus updates the ReplicaSet status
func (r *ReplicaSetController) updateReplicaSetStatus(ctx context.Context, replicaSet *api.ReplicaSet, state *ReplicaSetState) error {
	// Count ready pods
	readyPods := int32(0)
	for _, pod := range state.Pods {
		if pod.Status.Phase == string(api.PodRunning) {
			readyPods++
		}
	}

	// Update status
	replicaSet.Status.Replicas = int32(len(state.Pods))
	replicaSet.Status.ReadyReplicas = readyPods
	replicaSet.Status.AvailableReplicas = readyPods

	// Update in store
	if err := r.store.Update(ctx, replicaSet); err != nil {
		return fmt.Errorf("failed to update replicaset status: %w", err)
	}

	return nil
}

// GetReplicaSetState returns the state of a ReplicaSet
func (r *ReplicaSetController) GetReplicaSetState(namespace, name string) *ReplicaSetState {
	r.mu.RLock()
	defer r.mu.RUnlock()

	key := fmt.Sprintf("%s/%s", namespace, name)
	return r.replicaSets[key]
}

// ListReplicaSetStates returns all ReplicaSet states
func (r *ReplicaSetController) ListReplicaSetStates() map[string]*ReplicaSetState {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string]*ReplicaSetState)
	for k, v := range r.replicaSets {
		result[k] = v
	}
	return result
}
