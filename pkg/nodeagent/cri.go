package nodeagent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/minik8s/minik8s/pkg/api"
)

// CRIRuntime defines the interface for container runtime operations
type CRIRuntime interface {
	// Node information
	GetNodeCapacity() (api.ResourceList, error)
	GetNodeInfo() (*api.NodeSystemInfo, error)

	// Container operations
	CreateContainer(ctx context.Context, pod *api.Pod, container *api.Container) (string, error)
	StartContainer(ctx context.Context, containerID string) error
	StopContainer(ctx context.Context, containerID string, timeout int64) error
	RemoveContainer(ctx context.Context, containerID string) error
	GetContainerStatus(ctx context.Context, containerID string) (*ContainerStatus, error)
	ListContainers(ctx context.Context, filter *ContainerFilter) ([]*ContainerStatus, error)

	// Image operations
	PullImage(ctx context.Context, image string, auth *ImageAuth) error
	RemoveImage(ctx context.Context, imageID string) error
	ListImages(ctx context.Context, filter *ImageFilter) ([]*Image, error)

	// Pod operations
	CreatePodSandbox(ctx context.Context, pod *api.Pod) (string, error)
	RemovePodSandbox(ctx context.Context, podSandboxID string) error
	GetPodStatus(ctx context.Context, podSandboxID string) (*PodSandboxStatus, error)
}

// ContainerStatus represents the status of a container
type ContainerStatus struct {
	ID          string
	Metadata    *ContainerMetadata
	State       ContainerState
	CreatedAt   int64
	StartedAt   int64
	FinishedAt  int64
	ExitCode    int32
	Image       *ImageSpec
	ImageRef    string
	Reason      string
	Message     string
	Labels      map[string]string
	Annotations map[string]string
	Mounts      []*Mount
	LogPath     string
}

// ContainerMetadata contains metadata about a container
type ContainerMetadata struct {
	Name    string
	Attempt uint32
}

// ContainerState represents the state of a container
type ContainerState int32

const (
	ContainerStateCreated ContainerState = iota
	ContainerStateRunning
	ContainerStateExited
	ContainerStateUnknown
)

// ContainerFilter is used to filter containers
type ContainerFilter struct {
	ID            string
	State         *ContainerState
	PodSandboxID  string
	LabelSelector map[string]string
}

// Image represents a container image
type Image struct {
	ID          string
	RepoTags    []string
	RepoDigests []string
	Size        uint64
	UID         *int64
	Username    string
}

// ImageSpec represents an image specification
type ImageSpec struct {
	Image       string
	Annotations map[string]string
}

// ImageFilter is used to filter images
type ImageFilter struct {
	Image *ImageSpec
}

// ImageAuth contains authentication information for pulling images
type ImageAuth struct {
	Username      string
	Password      string
	Auth          string
	ServerAddress string
	IdentityToken string
	RegistryToken string
}

// Mount represents a volume mount
type Mount struct {
	ContainerPath  string
	HostPath       string
	Readonly       bool
	SelinuxRelabel bool
	Propagation    MountPropagation
}

// MountPropagation represents mount propagation mode
type MountPropagation int32

const (
	MountPropagationNone MountPropagation = iota
	MountPropagationHostToContainer
	MountPropagationBidirectional
)

// PodSandboxStatus represents the status of a pod sandbox
type PodSandboxStatus struct {
	ID          string
	Metadata    *PodSandboxMetadata
	State       PodSandboxState
	CreatedAt   int64
	Network     *PodSandboxNetworkStatus
	Linux       *LinuxPodSandboxStatus
	Labels      map[string]string
	Annotations map[string]string
}

// PodSandboxMetadata contains metadata about a pod sandbox
type PodSandboxMetadata struct {
	Name      string
	UID       string
	Namespace string
	Attempt   uint32
}

// PodSandboxState represents the state of a pod sandbox
type PodSandboxState int32

const (
	PodSandboxStateReady PodSandboxState = iota
	PodSandboxStateNotReady
)

// PodSandboxNetworkStatus represents the network status of a pod sandbox
type PodSandboxNetworkStatus struct {
	IP string
}

// LinuxPodSandboxStatus represents Linux-specific pod sandbox status
type LinuxPodSandboxStatus struct {
	Namespaces *Namespace
}

// Namespace represents a Linux namespace
type Namespace struct {
	Type NamespaceType
	Path string
}

// NamespaceType represents the type of a Linux namespace
type NamespaceType int32

const (
	NamespaceTypePID NamespaceType = iota
	NamespaceTypeNetwork
	NamespaceTypeMount
	NamespaceTypeIPC
	NamespaceTypeUTS
)

// MockCRIRuntime is a mock implementation for testing
type MockCRIRuntime struct {
	containers map[string]*ContainerStatus
	images     map[string]*Image
}

// NewMockCRIRuntime creates a new mock CRI runtime
func NewMockCRIRuntime() *MockCRIRuntime {
	return &MockCRIRuntime{
		containers: make(map[string]*ContainerStatus),
		images:     make(map[string]*Image),
	}
}

// GetNodeCapacity returns mock node capacity
func (m *MockCRIRuntime) GetNodeCapacity() (api.ResourceList, error) {
	return api.ResourceList{
		api.ResourceCPU:    "4",
		api.ResourceMemory: "8Gi",
	}, nil
}

// GetNodeInfo returns mock node info
func (m *MockCRIRuntime) GetNodeInfo() (*api.NodeSystemInfo, error) {
	return &api.NodeSystemInfo{
		MachineID:               "mock-machine-id",
		SystemUUID:              "mock-system-uuid",
		BootID:                  "mock-boot-id",
		KernelVersion:           "5.15.0",
		OSImage:                 "Ubuntu 22.04",
		ContainerRuntimeVersion: "containerd://1.7.0",
		KubeletVersion:          "v1.0.0",
		OperatingSystem:         "linux",
		Architecture:            "amd64",
	}, nil
}

// CreateContainer creates a mock container
func (m *MockCRIRuntime) CreateContainer(ctx context.Context, pod *api.Pod, container *api.Container) (string, error) {
	containerID := fmt.Sprintf("mock-container-%d", time.Now().UnixNano())

	m.containers[containerID] = &ContainerStatus{
		ID: containerID,
		Metadata: &ContainerMetadata{
			Name:    container.Name,
			Attempt: 0,
		},
		State:     ContainerStateCreated,
		CreatedAt: time.Now().UnixNano(),
		Image: &ImageSpec{
			Image: container.Image,
		},
	}

	return containerID, nil
}

// StartContainer starts a mock container
func (m *MockCRIRuntime) StartContainer(ctx context.Context, containerID string) error {
	if container, exists := m.containers[containerID]; exists {
		container.State = ContainerStateRunning
		container.StartedAt = time.Now().UnixNano()
		return nil
	}
	return fmt.Errorf("container %s not found", containerID)
}

// StopContainer stops a mock container
func (m *MockCRIRuntime) StopContainer(ctx context.Context, containerID string, timeout int64) error {
	if container, exists := m.containers[containerID]; exists {
		container.State = ContainerStateExited
		container.FinishedAt = time.Now().UnixNano()
		return nil
	}
	return fmt.Errorf("container %s not found", containerID)
}

// RemoveContainer removes a mock container
func (m *MockCRIRuntime) RemoveContainer(ctx context.Context, containerID string) error {
	if _, exists := m.containers[containerID]; exists {
		delete(m.containers, containerID)
		return nil
	}
	return fmt.Errorf("container %s not found", containerID)
}

// GetContainerStatus gets the status of a mock container
func (m *MockCRIRuntime) GetContainerStatus(ctx context.Context, containerID string) (*ContainerStatus, error) {
	if container, exists := m.containers[containerID]; exists {
		return container, nil
	}
	return nil, fmt.Errorf("container %s not found", containerID)
}

// ListContainers lists mock containers
func (m *MockCRIRuntime) ListContainers(ctx context.Context, filter *ContainerFilter) ([]*ContainerStatus, error) {
	var containers []*ContainerStatus
	for _, container := range m.containers {
		if filter != nil {
			if filter.ID != "" && container.ID != filter.ID {
				continue
			}
			if filter.State != nil && container.State != *filter.State {
				continue
			}
		}
		containers = append(containers, container)
	}
	return containers, nil
}

// PullImage pulls a mock image
func (m *MockCRIRuntime) PullImage(ctx context.Context, image string, auth *ImageAuth) error {
	imageID := fmt.Sprintf("mock-image-%s", strings.ReplaceAll(image, ":", "-"))
	m.images[imageID] = &Image{
		ID:       imageID,
		RepoTags: []string{image},
		Size:     1024 * 1024 * 100, // 100MB
	}
	return nil
}

// RemoveImage removes a mock image
func (m *MockCRIRuntime) RemoveImage(ctx context.Context, imageID string) error {
	if _, exists := m.images[imageID]; exists {
		delete(m.images, imageID)
		return nil
	}
	return fmt.Errorf("image %s not found", imageID)
}

// ListImages lists mock images
func (m *MockCRIRuntime) ListImages(ctx context.Context, filter *ImageFilter) ([]*Image, error) {
	var images []*Image
	for _, image := range m.images {
		if filter != nil && filter.Image != nil {
			if filter.Image.Image != "" {
				found := false
				for _, tag := range image.RepoTags {
					if tag == filter.Image.Image {
						found = true
						break
					}
				}
				if !found {
					continue
				}
			}
		}
		images = append(images, image)
	}
	return images, nil
}

// CreatePodSandbox creates a mock pod sandbox
func (m *MockCRIRuntime) CreatePodSandbox(ctx context.Context, pod *api.Pod) (string, error) {
	sandboxID := fmt.Sprintf("mock-sandbox-%s-%s", pod.Namespace, pod.Name)
	return sandboxID, nil
}

// RemovePodSandbox removes a mock pod sandbox
func (m *MockCRIRuntime) RemovePodSandbox(ctx context.Context, podSandboxID string) error {
	return nil
}

// GetPodStatus gets the status of a mock pod sandbox
func (m *MockCRIRuntime) GetPodStatus(ctx context.Context, podSandboxID string) (*PodSandboxStatus, error) {
	return &PodSandboxStatus{
		ID:    podSandboxID,
		State: PodSandboxStateReady,
		Network: &PodSandboxNetworkStatus{
			IP: "192.168.1.100",
		},
	}, nil
}
