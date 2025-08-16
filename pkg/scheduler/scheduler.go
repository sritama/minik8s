package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/minik8s/minik8s/pkg/api"
	"github.com/minik8s/minik8s/pkg/store"
)

// Scheduler represents the pod scheduler
type Scheduler struct {
	mu sync.RWMutex

	// Configuration
	store store.Store

	// State
	running       bool
	stopCh        chan struct{}
	scheduledPods map[string]*ScheduledPod

	// Scheduling configuration
	defaultNodeSelector map[string]string
	schedulingInterval  time.Duration
}

// ScheduledPod tracks a pod that has been scheduled
type ScheduledPod struct {
	Pod      *api.Pod
	NodeName string
	Time     time.Time
	Status   string
}

// Config holds the configuration for the scheduler
type Config struct {
	Store               store.Store
	DefaultNodeSelector map[string]string
	SchedulingInterval  time.Duration
}

// NewScheduler creates a new scheduler
func NewScheduler(config *Config) *Scheduler {
	if config.SchedulingInterval == 0 {
		config.SchedulingInterval = 10 * time.Second
	}

	return &Scheduler{
		store:               config.Store,
		defaultNodeSelector: config.DefaultNodeSelector,
		schedulingInterval:  config.SchedulingInterval,
		scheduledPods:       make(map[string]*ScheduledPod),
		stopCh:              make(chan struct{}),
	}
}

// Start starts the scheduler
func (s *Scheduler) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("scheduler is already running")
	}

	// Start background goroutines
	go s.schedulingLoop(ctx)

	s.running = true
	return nil
}

// Stop stops the scheduler
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	close(s.stopCh)
	s.running = false
}

// schedulingLoop continuously processes unscheduled pods
func (s *Scheduler) schedulingLoop(ctx context.Context) {
	ticker := time.NewTicker(s.schedulingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case <-ticker.C:
			if err := s.processUnscheduledPods(ctx); err != nil {
				// Log error but continue
				fmt.Printf("Error processing unscheduled pods: %v\n", err)
			}
		}
	}
}

// processUnscheduledPods finds and schedules unscheduled pods
func (s *Scheduler) processUnscheduledPods(ctx context.Context) error {
	// Get all pods
	pods, err := s.store.List(ctx, "Pod", "")
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}

	// Get all nodes
	nodes, err := s.store.List(ctx, "Node", "")
	if err != nil {
		return fmt.Errorf("failed to list nodes: %w", err)
	}

	// Filter unscheduled pods
	var unscheduledPods []*api.Pod
	for _, obj := range pods {
		if pod, ok := obj.(*api.Pod); ok {
			if pod.Spec.NodeName == "" && pod.Status.Phase == string(api.PodPending) {
				unscheduledPods = append(unscheduledPods, pod)
			}
		}
	}

	if len(unscheduledPods) == 0 {
		return nil
	}

	fmt.Printf("Found %d unscheduled pods\n", len(unscheduledPods))

	// Try to schedule each pod
	for _, pod := range unscheduledPods {
		if err := s.schedulePod(ctx, pod, nodes); err != nil {
			fmt.Printf("Failed to schedule pod %s: %v\n", pod.Name, err)
		}
	}

	return nil
}

// schedulePod attempts to schedule a pod to a node
func (s *Scheduler) schedulePod(ctx context.Context, pod *api.Pod, nodes []store.Object) error {
	// Find the best node for this pod
	node, err := s.findBestNode(pod, nodes)
	if err != nil {
		return fmt.Errorf("failed to find suitable node: %w", err)
	}

	// Assign the pod to the node
	pod.Spec.NodeName = node.GetName()
	pod.Status.Phase = string(api.PodScheduled)
	pod.Status.Conditions = append(pod.Status.Conditions, api.PodCondition{
		Type:               "PodScheduled",
		Status:             "True",
		LastTransitionTime: time.Now(),
		Reason:             "Scheduled",
		Message:            fmt.Sprintf("Pod scheduled to node %s", node.GetName()),
	})

	// Update the pod in the store
	if err := s.store.Update(ctx, pod); err != nil {
		return fmt.Errorf("failed to update pod: %w", err)
	}

	// Track the scheduled pod
	s.mu.Lock()
	s.scheduledPods[fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)] = &ScheduledPod{
		Pod:      pod,
		NodeName: node.GetName(),
		Time:     time.Now(),
		Status:   "Scheduled",
	}
	s.mu.Unlock()

	fmt.Printf("Pod %s/%s scheduled to node %s\n", pod.Namespace, pod.Name, node.GetName())
	return nil
}

// findBestNode finds the best node for a pod
func (s *Scheduler) findBestNode(pod *api.Pod, nodes []store.Object) (store.Object, error) {
	var bestNode store.Object
	var bestScore float64
	var suitableNodes []*api.Node

	// First pass: find all suitable nodes
	for _, obj := range nodes {
		node, ok := obj.(*api.Node)
		if !ok {
			continue
		}

		// Check if node is ready
		if !s.isNodeReady(node) {
			continue
		}

		// Check node selector
		if !s.matchesNodeSelector(pod, node) {
			continue
		}

		// Check resource requirements
		if !s.hasSufficientResources(pod, node) {
			continue
		}

		// Check taints and tolerations (basic implementation)
		if !s.matchesTaintsAndTolerations(pod, node) {
			continue
		}

		suitableNodes = append(suitableNodes, node)
	}

	if len(suitableNodes) == 0 {
		return nil, fmt.Errorf("no suitable node found for pod %s", pod.Name)
	}

	// Second pass: score suitable nodes
	for _, node := range suitableNodes {
		score := s.calculateNodeScore(pod, node)
		if score > bestScore {
			bestScore = score
			bestNode = node
		}
	}

	return bestNode, nil
}

// isNodeReady checks if a node is ready
func (s *Scheduler) isNodeReady(node *api.Node) bool {
	for _, condition := range node.Status.Conditions {
		if condition.Type == "Ready" && condition.Status == "True" {
			return true
		}
	}
	return false
}

// matchesNodeSelector checks if a pod matches a node's labels
func (s *Scheduler) matchesNodeSelector(pod *api.Pod, node *api.Node) bool {
	// For now, just check if the node has the required labels
	// In a real implementation, you'd want more sophisticated node affinity rules
	if len(pod.Spec.NodeSelector) == 0 {
		return true
	}

	for key, value := range pod.Spec.NodeSelector {
		if nodeValue, exists := node.Labels[key]; !exists || nodeValue != value {
			return false
		}
	}

	return true
}

// matchesTaintsAndTolerations checks if a pod can tolerate node taints
func (s *Scheduler) matchesTaintsAndTolerations(pod *api.Pod, node *api.Node) bool {
	// Basic implementation - in a real system, you'd want proper taint/toleration logic
	// For now, just return true to allow all pods
	return true
}

// hasSufficientResources checks if a node has sufficient resources
func (s *Scheduler) hasSufficientResources(pod *api.Pod, node *api.Node) bool {
	// Calculate total resource requests for the pod
	var totalCPU, totalMemory float64

	for _, container := range pod.Spec.Containers {
		if container.Resources.Requests != nil {
			if cpu, exists := container.Resources.Requests[api.ResourceCPU]; exists {
				if cpuValue, err := parseCPU(cpu); err == nil {
					totalCPU += cpuValue
				}
			}
			if memory, exists := container.Resources.Requests[api.ResourceMemory]; exists {
				if memoryValue, err := parseMemory(memory); err == nil {
					totalMemory += memoryValue
				}
			}
		}
	}

	// Check if node has sufficient resources
	if totalCPU > 0 {
		if nodeCPU, exists := node.Status.Allocatable[api.ResourceCPU]; exists {
			if availableCPU, err := parseCPU(nodeCPU); err == nil {
				if totalCPU > availableCPU {
					return false
				}
			}
		}
	}

	if totalMemory > 0 {
		if nodeMemory, exists := node.Status.Allocatable[api.ResourceMemory]; exists {
			if availableMemory, err := parseMemory(nodeMemory); err == nil {
				if totalMemory > availableMemory {
					return false
				}
			}
		}
	}

	return true
}

// calculateNodeScore calculates a score for a node
func (s *Scheduler) calculateNodeScore(pod *api.Pod, node *api.Node) float64 {
	score := 0.0

	// Prefer nodes with more available resources
	if allocatable, exists := node.Status.Allocatable[api.ResourceCPU]; exists {
		if cpu, err := parseCPU(allocatable); err == nil {
			score += cpu
		}
	}

	if allocatable, exists := node.Status.Allocatable[api.ResourceMemory]; exists {
		if memory, err := parseMemory(allocatable); err == nil {
			score += memory / (1024 * 1024 * 1024) // Convert to GB
		}
	}

	// Prefer nodes with fewer pods
	s.mu.RLock()
	nodePodCount := 0
	for _, scheduledPod := range s.scheduledPods {
		if scheduledPod.NodeName == node.GetName() {
			nodePodCount++
		}
	}
	s.mu.RUnlock()

	score -= float64(nodePodCount)

	return score
}

// GetScheduledPods returns all scheduled pods
func (s *Scheduler) GetScheduledPods() map[string]*ScheduledPod {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]*ScheduledPod)
	for k, v := range s.scheduledPods {
		result[k] = v
	}
	return result
}

// Helper functions for resource parsing
func parseCPU(cpu string) (float64, error) {
	// Simple CPU parsing - in a real implementation, you'd want more robust parsing
	if cpu == "" {
		return 0, nil
	}

	// Handle millicores (e.g., "100m" = 0.1)
	if len(cpu) > 1 && cpu[len(cpu)-1] == 'm' {
		if value, err := parseFloat(cpu[:len(cpu)-1]); err == nil {
			return value / 1000, nil
		}
	}

	// Handle cores (e.g., "1", "0.5")
	return parseFloat(cpu)
}

func parseMemory(memory string) (float64, error) {
	// Simple memory parsing - in a real implementation, you'd want more robust parsing
	if memory == "" {
		return 0, nil
	}

	// Handle bytes (e.g., "1Gi", "512Mi")
	if len(memory) > 2 {
		suffix := memory[len(memory)-2:]
		value, err := parseFloat(memory[:len(memory)-2])
		if err != nil {
			return 0, err
		}

		switch suffix {
		case "Ki":
			return value * 1024, nil
		case "Mi":
			return value * 1024 * 1024, nil
		case "Gi":
			return value * 1024 * 1024 * 1024, nil
		}
	}

	// Assume bytes
	return parseFloat(memory)
}

func parseFloat(s string) (float64, error) {
	// Simple float parsing - in a real implementation, you'd want more robust parsing
	var result float64
	_, err := fmt.Sscanf(s, "%f", &result)
	return result, err
}
