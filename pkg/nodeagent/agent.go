package nodeagent

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/minik8s/minik8s/pkg/api"
	"github.com/minik8s/minik8s/pkg/store"
)

// Agent represents a node agent (kubelet-like component)
type Agent struct {
	mu sync.RWMutex

	// Configuration
	nodeName     string
	apiServerURL string
	store        store.Store

	// Runtime components
	criRuntime CRIRuntime
	networkMgr NetworkManager
	volumeMgr  VolumeManager

	// State
	pods       map[string]*PodState
	nodeStatus *api.NodeStatus
	running    bool
	stopCh     chan struct{}

	// Heartbeat
	heartbeatInterval time.Duration
	lastHeartbeat     time.Time
}

// PodState tracks the runtime state of a pod on this node
type PodState struct {
	Pod        *api.Pod
	Status     *api.PodStatus
	Containers map[string]*ContainerRuntimeState
	Volumes    map[string]*VolumeState
	Created    time.Time
	Updated    time.Time
}

// ContainerRuntimeState tracks the runtime state of a container
type ContainerRuntimeState struct {
	ID        string
	Status    string
	StartedAt time.Time
	ExitCode  int32
	Message   string
}

// VolumeState tracks the state of mounted volumes
type VolumeState struct {
	Name      string
	Path      string
	Mounted   bool
	MountTime time.Time
}

// Config holds the configuration for the node agent
type Config struct {
	NodeName          string
	APIServerURL      string
	Store             store.Store
	CRIRuntime        CRIRuntime
	NetworkManager    NetworkManager
	VolumeManager     VolumeManager
	HeartbeatInterval time.Duration
}

// NewAgent creates a new node agent
func NewAgent(config *Config) *Agent {
	if config.HeartbeatInterval == 0 {
		config.HeartbeatInterval = 30 * time.Second
	}

	return &Agent{
		nodeName:          config.NodeName,
		apiServerURL:      config.APIServerURL,
		store:             config.Store,
		criRuntime:        config.CRIRuntime,
		networkMgr:        config.NetworkManager,
		volumeMgr:         config.VolumeManager,
		pods:              make(map[string]*PodState),
		heartbeatInterval: config.HeartbeatInterval,
		stopCh:            make(chan struct{}),
	}
}

// Start starts the node agent
func (a *Agent) Start(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.running {
		return fmt.Errorf("agent is already running")
	}

	// Initialize node status
	if err := a.initializeNodeStatus(); err != nil {
		return fmt.Errorf("failed to initialize node status: %w", err)
	}

	// Start background goroutines
	go a.podSyncLoop(ctx)
	go a.heartbeatLoop(ctx)
	go a.statusReportingLoop(ctx)

	a.running = true
	return nil
}

// Stop stops the node agent
func (a *Agent) Stop() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.running {
		return
	}

	close(a.stopCh)
	a.running = false
}

// initializeNodeStatus initializes the node status
func (a *Agent) initializeNodeStatus() error {
	// Get node capacity from runtime
	capacity, err := a.criRuntime.GetNodeCapacity()
	if err != nil {
		return fmt.Errorf("failed to get node capacity: %w", err)
	}

	// Get node info from runtime
	nodeInfo, err := a.criRuntime.GetNodeInfo()
	if err != nil {
		return fmt.Errorf("failed to get node info: %w", err)
	}

	a.nodeStatus = &api.NodeStatus{
		Capacity:    capacity,
		Allocatable: capacity, // For now, same as capacity
		Conditions: []api.NodeCondition{
			{
				Type:               "Ready",
				Status:             "True",
				LastHeartbeatTime:  time.Now(),
				LastTransitionTime: time.Now(),
			},
		},
		NodeInfo: *nodeInfo,
	}

	return nil
}

// podSyncLoop continuously syncs pods assigned to this node
func (a *Agent) podSyncLoop(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-a.stopCh:
			return
		case <-ticker.C:
			if err := a.syncPods(ctx); err != nil {
				// Log error but continue
				fmt.Printf("Error syncing pods: %v\n", err)
			}
		}
	}
}

// syncPods syncs all pods assigned to this node
func (a *Agent) syncPods(ctx context.Context) error {
	// Get pods assigned to this node
	pods, err := a.store.List(ctx, "Pod", "")
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}

	// Filter pods assigned to this node
	var nodePods []*api.Pod
	for _, obj := range pods {
		if pod, ok := obj.(*api.Pod); ok && pod.Spec.NodeName == a.nodeName {
			nodePods = append(nodePods, pod)
		}
	}

	// Sync each pod
	for _, pod := range nodePods {
		if err := a.syncPod(ctx, pod); err != nil {
			fmt.Printf("Error syncing pod %s: %v\n", pod.Name, err)
		}
	}

	return nil
}

// syncPod syncs a single pod
func (a *Agent) syncPod(ctx context.Context, pod *api.Pod) error {
	podKey := fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)

	a.mu.Lock()
	podState, exists := a.pods[podKey]
	a.mu.Unlock()

	if !exists {
		// New pod, create it
		return a.createPod(ctx, pod)
	}

	// Existing pod, check if it needs updates
	if podState.Pod.ResourceVersion != pod.ResourceVersion {
		return a.updatePod(ctx, pod)
	}

	// Sync pod status
	return a.syncPodStatus(ctx, pod, podState)
}

// createPod creates a new pod on this node
func (a *Agent) createPod(ctx context.Context, pod *api.Pod) error {
	podKey := fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)

	// Create pod state
	podState := &PodState{
		Pod:        pod,
		Status:     &api.PodStatus{},
		Containers: make(map[string]*ContainerRuntimeState),
		Volumes:    make(map[string]*VolumeState),
		Created:    time.Now(),
		Updated:    time.Now(),
	}

	// Set initial status
	podState.Status.Phase = string(api.PodPending)
	podState.Status.Conditions = []api.PodCondition{
		{
			Type:               "PodScheduled",
			Status:             "True",
			LastTransitionTime: time.Now(),
		},
	}

	// Mount volumes
	if err := a.mountPodVolumes(pod, podState); err != nil {
		podState.Status.Phase = string(api.PodFailed)
		podState.Status.Message = fmt.Sprintf("Failed to mount volumes: %v", err)
		a.updatePodState(podKey, podState)
		return err
	}

	// Create containers
	if err := a.createPodContainers(pod, podState); err != nil {
		podState.Status.Phase = string(api.PodFailed)
		podState.Status.Message = fmt.Sprintf("Failed to create containers: %v", err)
		a.updatePodState(podKey, podState)
		return err
	}

	// Start containers
	if err := a.startPodContainers(pod, podState); err != nil {
		podState.Status.Phase = string(api.PodFailed)
		podState.Status.Message = fmt.Sprintf("Failed to start containers: %v", err)
		a.updatePodState(podKey, podState)
		return err
	}

	// Set up networking
	if err := a.setupPodNetworking(pod, podState); err != nil {
		podState.Status.Phase = string(api.PodFailed)
		podState.Status.Message = fmt.Sprintf("Failed to setup networking: %v", err)
		a.updatePodState(podKey, podState)
		return err
	}

	// Update status to running
	podState.Status.Phase = string(api.PodRunning)
	podState.Status.StartTime = &time.Time{}
	*podState.Status.StartTime = time.Now()

	a.updatePodState(podKey, podState)
	return nil
}

// updatePod updates an existing pod
func (a *Agent) updatePod(ctx context.Context, pod *api.Pod) error {
	// For now, just recreate the pod
	// In a real implementation, you'd want to handle updates more gracefully
	return a.deletePod(ctx, pod.Namespace, pod.Name)
}

// deletePod deletes a pod from this node
func (a *Agent) deletePod(ctx context.Context, namespace, name string) error {
	podKey := fmt.Sprintf("%s/%s", namespace, name)

	a.mu.Lock()
	podState, exists := a.pods[podKey]
	a.mu.Unlock()

	if !exists {
		return nil
	}

	// Stop containers
	if err := a.stopPodContainers(podState); err != nil {
		fmt.Printf("Error stopping containers for pod %s: %v\n", podKey, err)
	}

	// Clean up networking
	if err := a.cleanupPodNetworking(podState); err != nil {
		fmt.Printf("Error cleaning up networking for pod %s: %v\n", podKey, err)
	}

	// Unmount volumes
	if err := a.unmountPodVolumes(podState); err != nil {
		fmt.Printf("Error unmounting volumes for pod %s: %v\n", podKey, err)
	}

	// Remove from local state
	a.mu.Lock()
	delete(a.pods, podKey)
	a.mu.Unlock()

	return nil
}

// syncPodStatus syncs the status of a pod
func (a *Agent) syncPodStatus(ctx context.Context, pod *api.Pod, podState *PodState) error {
	// Update container statuses
	if err := a.updateContainerStatuses(podState); err != nil {
		return err
	}

	// Update pod status in store
	podState.Pod.Status = *podState.Status
	if err := a.store.Update(ctx, podState.Pod); err != nil {
		return fmt.Errorf("failed to update pod status: %w", err)
	}

	return nil
}

// heartbeatLoop sends regular heartbeats to the API server
func (a *Agent) heartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(a.heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-a.stopCh:
			return
		case <-ticker.C:
			if err := a.sendHeartbeat(ctx); err != nil {
				fmt.Printf("Error sending heartbeat: %v\n", err)
			}
		}
	}
}

// statusReportingLoop reports node status to the API server
func (a *Agent) statusReportingLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-a.stopCh:
			return
		case <-ticker.C:
			if err := a.reportNodeStatus(ctx); err != nil {
				fmt.Printf("Error reporting node status: %v\n", err)
			}
		}
	}
}

// sendHeartbeat sends a heartbeat to the API server
func (a *Agent) sendHeartbeat(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Update heartbeat time
	a.lastHeartbeat = time.Now()

	// Update node condition
	for i, condition := range a.nodeStatus.Conditions {
		if condition.Type == "Ready" {
			a.nodeStatus.Conditions[i].LastHeartbeatTime = a.lastHeartbeat
			break
		}
	}

	return nil
}

// reportNodeStatus reports the current node status to the API server
func (a *Agent) reportNodeStatus(ctx context.Context) error {
	// Get current node from store
	node, err := a.store.Get(ctx, "Node", "", a.nodeName)
	if err != nil {
		return fmt.Errorf("failed to get node: %w", err)
	}

	// Update status
	if nodeObj, ok := node.(*api.Node); ok {
		nodeObj.Status = *a.nodeStatus
		if err := a.store.Update(ctx, nodeObj); err != nil {
			return fmt.Errorf("failed to update node status: %w", err)
		}
	}

	return nil
}

// updatePodState updates the pod state and stores it locally
func (a *Agent) updatePodState(podKey string, podState *PodState) {
	a.mu.Lock()
	defer a.mu.Unlock()

	podState.Updated = time.Now()
	a.pods[podKey] = podState
}

// Helper methods for pod operations (to be implemented)
func (a *Agent) mountPodVolumes(pod *api.Pod, podState *PodState) error {
	// TODO: Implement volume mounting
	return nil
}

func (a *Agent) createPodContainers(pod *api.Pod, podState *PodState) error {
	// TODO: Implement container creation
	return nil
}

func (a *Agent) startPodContainers(pod *api.Pod, podState *PodState) error {
	// TODO: Implement container starting
	return nil
}

func (a *Agent) setupPodNetworking(pod *api.Pod, podState *PodState) error {
	// TODO: Implement networking setup
	return nil
}

func (a *Agent) stopPodContainers(podState *PodState) error {
	// TODO: Implement container stopping
	return nil
}

func (a *Agent) cleanupPodNetworking(podState *PodState) error {
	// TODO: Implement networking cleanup
	return nil
}

func (a *Agent) unmountPodVolumes(podState *PodState) error {
	// TODO: Implement volume unmounting
	return nil
}

func (a *Agent) updateContainerStatuses(podState *PodState) error {
	// TODO: Implement container status updates
	return nil
}
