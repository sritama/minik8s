package api

import (
	"time"
)

// TypeMeta describes the type of the object
type TypeMeta struct {
	Kind       string `json:"kind"`
	APIVersion string `json:"apiVersion"`
}

// ObjectMeta contains metadata about the object
type ObjectMeta struct {
	Name              string            `json:"name"`
	Namespace         string            `json:"namespace"`
	UID               string            `json:"uid,omitempty"`
	ResourceVersion   string            `json:"resourceVersion,omitempty"`
	Generation        int64             `json:"generation,omitempty"`
	CreationTimestamp time.Time         `json:"creationTimestamp,omitempty"`
	Labels            map[string]string `json:"labels,omitempty"`
	Annotations       map[string]string `json:"annotations,omitempty"`
	OwnerReferences   []OwnerReference  `json:"ownerReferences,omitempty"`
}

// ResourceRequirements describes the compute resource requirements
type ResourceRequirements struct {
	Limits   ResourceList `json:"limits,omitempty"`
	Requests ResourceList `json:"requests,omitempty"`
}

// ResourceList is a set of (resource name, quantity) pairs
type ResourceList map[ResourceName]string

// ResourceName is the name identifying various resources
type ResourceName string

const (
	// CPU, in cores
	ResourceCPU ResourceName = "cpu"
	// Memory, in bytes
	ResourceMemory ResourceName = "memory"
)

// Container represents a single container within a pod
type Container struct {
	Name            string               `json:"name"`
	Image           string               `json:"image"`
	Command         []string             `json:"command,omitempty"`
	Args            []string             `json:"args,omitempty"`
	WorkingDir      string               `json:"workingDir,omitempty"`
	Ports           []ContainerPort      `json:"ports,omitempty"`
	Env             []EnvVar             `json:"env,omitempty"`
	Resources       ResourceRequirements `json:"resources,omitempty"`
	VolumeMounts    []VolumeMount        `json:"volumeMounts,omitempty"`
	LivenessProbe   *Probe               `json:"livenessProbe,omitempty"`
	ReadinessProbe  *Probe               `json:"readinessProbe,omitempty"`
	ImagePullPolicy string               `json:"imagePullPolicy,omitempty"`
}

// ContainerPort represents a network port in a single container
type ContainerPort struct {
	Name          string `json:"name,omitempty"`
	HostPort      int32  `json:"hostPort,omitempty"`
	ContainerPort int32  `json:"containerPort"`
	Protocol      string `json:"protocol,omitempty"`
	HostIP        string `json:"hostIP,omitempty"`
}

// EnvVar represents an environment variable present in a Container
type EnvVar struct {
	Name  string `json:"name"`
	Value string `json:"value,omitempty"`
}

// VolumeMount describes a mounting of a Volume within a container
type VolumeMount struct {
	Name      string `json:"name"`
	ReadOnly  bool   `json:"readOnly,omitempty"`
	MountPath string `json:"mountPath"`
}

// Probe describes a health check to be performed
type Probe struct {
	Exec      *ExecAction      `json:"exec,omitempty"`
	HTTPGet   *HTTPGetAction   `json:"httpGet,omitempty"`
	TCPSocket *TCPSocketAction `json:"tcpSocket,omitempty"`
}

// ExecAction describes a "run in container" action
type ExecAction struct {
	Command []string `json:"command,omitempty"`
}

// HTTPGetAction describes an action based on HTTP Get requests
type HTTPGetAction struct {
	Path   string `json:"path,omitempty"`
	Port   int32  `json:"port"`
	Scheme string `json:"scheme,omitempty"`
}

// TCPSocketAction describes an action based on opening a socket
type TCPSocketAction struct {
	Port int32 `json:"port"`
}

// PodSpec is a description of a pod
type PodSpec struct {
	Containers       []Container            `json:"containers"`
	Volumes          []Volume               `json:"volumes,omitempty"`
	NodeName         string                 `json:"nodeName,omitempty"`
	NodeSelector     map[string]string      `json:"nodeSelector,omitempty"`
	RestartPolicy    string                 `json:"restartPolicy,omitempty"`
	DNSPolicy        string                 `json:"dnsPolicy,omitempty"`
	HostNetwork      bool                   `json:"hostNetwork,omitempty"`
	HostPID          bool                   `json:"hostPID,omitempty"`
	HostIPC          bool                   `json:"hostIPC,omitempty"`
	ImagePullSecrets []LocalObjectReference `json:"imagePullSecrets,omitempty"`
}

// PodStatus represents information about the status of a pod
type PodStatus struct {
	Phase             string            `json:"phase"`
	Conditions        []PodCondition    `json:"conditions,omitempty"`
	Message           string            `json:"message,omitempty"`
	Reason            string            `json:"reason,omitempty"`
	HostIP            string            `json:"hostIP,omitempty"`
	PodIP             string            `json:"podIP,omitempty"`
	StartTime         *time.Time        `json:"startTime,omitempty"`
	ContainerStatuses []ContainerStatus `json:"containerStatuses,omitempty"`
}

// PodCondition contains details for the current condition of this pod
type PodCondition struct {
	Type               string    `json:"type"`
	Status             string    `json:"status"`
	LastProbeTime      time.Time `json:"lastProbeTime,omitempty"`
	LastTransitionTime time.Time `json:"lastTransitionTime,omitempty"`
	Reason             string    `json:"reason,omitempty"`
	Message            string    `json:"message,omitempty"`
}

// ContainerStatus describes the current state of a container
type ContainerStatus struct {
	Name         string         `json:"name"`
	State        ContainerState `json:"state"`
	Ready        bool           `json:"ready"`
	RestartCount int32          `json:"restartCount"`
	Image        string         `json:"image"`
	ImageID      string         `json:"imageID,omitempty"`
	Started      *bool          `json:"started,omitempty"`
}

// ContainerState holds a possible state of a container
type ContainerState struct {
	Waiting    *ContainerStateWaiting    `json:"waiting,omitempty"`
	Running    *ContainerStateRunning    `json:"running,omitempty"`
	Terminated *ContainerStateTerminated `json:"terminated,omitempty"`
}

// ContainerStateWaiting is a waiting state of a container
type ContainerStateWaiting struct {
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}

// ContainerStateRunning is a running state of a container
type ContainerStateRunning struct {
	StartedAt time.Time `json:"startedAt,omitempty"`
}

// ContainerStateTerminated is a terminated state of a container
type ContainerStateTerminated struct {
	ExitCode   int32     `json:"exitCode"`
	Signal     int32     `json:"signal,omitempty"`
	Reason     string    `json:"reason,omitempty"`
	Message    string    `json:"message,omitempty"`
	StartedAt  time.Time `json:"startedAt,omitempty"`
	FinishedAt time.Time `json:"finishedAt,omitempty"`
}

// Pod represents a pod in the system
type Pod struct {
	TypeMeta   `json:",inline"`
	ObjectMeta `json:"metadata"`
	Spec       PodSpec   `json:"spec"`
	Status     PodStatus `json:"status"`
}

// GetKind returns the kind of the pod
func (p *Pod) GetKind() string {
	return p.Kind
}

// GetAPIVersion returns the API version of the pod
func (p *Pod) GetAPIVersion() string {
	return p.APIVersion
}

// GetName returns the name of the pod
func (p *Pod) GetName() string {
	return p.Name
}

// GetNamespace returns the namespace of the pod
func (p *Pod) GetNamespace() string {
	return p.Namespace
}

// GetUID returns the UID of the pod
func (p *Pod) GetUID() string {
	return p.UID
}

// GetResourceVersion returns the resource version of the pod
func (p *Pod) GetResourceVersion() string {
	return p.ResourceVersion
}

// SetResourceVersion sets the resource version of the pod
func (p *Pod) SetResourceVersion(version string) {
	p.ResourceVersion = version
}

// GetCreationTimestamp returns the creation timestamp of the pod
func (p *Pod) GetCreationTimestamp() time.Time {
	return p.CreationTimestamp
}

// SetCreationTimestamp sets the creation timestamp of the pod
func (p *Pod) SetCreationTimestamp(timestamp time.Time) {
	p.CreationTimestamp = timestamp
}

// NodeSpec is a description of a node
type NodeSpec struct {
	PodCIDR       string  `json:"podCIDR,omitempty"`
	Unschedulable bool    `json:"unschedulable,omitempty"`
	Taints        []Taint `json:"taints,omitempty"`
}

// NodeStatus represents information about the status of a node
type NodeStatus struct {
	Capacity        ResourceList        `json:"capacity,omitempty"`
	Allocatable     ResourceList        `json:"allocatable,omitempty"`
	Conditions      []NodeCondition     `json:"conditions,omitempty"`
	Addresses       []NodeAddress       `json:"addresses,omitempty"`
	DaemonEndpoints NodeDaemonEndpoints `json:"daemonEndpoints,omitempty"`
	NodeInfo        NodeSystemInfo      `json:"nodeInfo,omitempty"`
}

// Node represents a node in the system
type Node struct {
	TypeMeta   `json:",inline"`
	ObjectMeta `json:"metadata"`
	Spec       NodeSpec   `json:"spec"`
	Status     NodeStatus `json:"status"`
}

// GetKind returns the kind of the node
func (n *Node) GetKind() string {
	return n.Kind
}

// GetAPIVersion returns the API version of the node
func (n *Node) GetAPIVersion() string {
	return n.APIVersion
}

// GetName returns the name of the node
func (n *Node) GetName() string {
	return n.Name
}

// GetNamespace returns the namespace of the node
func (n *Node) GetNamespace() string {
	return n.Namespace
}

// GetUID returns the UID of the node
func (n *Node) GetUID() string {
	return n.UID
}

// GetResourceVersion returns the resource version of the node
func (n *Node) GetResourceVersion() string {
	return n.ResourceVersion
}

// SetResourceVersion sets the resource version of the node
func (n *Node) SetResourceVersion(version string) {
	n.ResourceVersion = version
}

// GetCreationTimestamp returns the creation timestamp of the node
func (n *Node) GetCreationTimestamp() time.Time {
	return n.CreationTimestamp
}

// SetCreationTimestamp sets the creation timestamp of the node
func (n *Node) SetCreationTimestamp(timestamp time.Time) {
	n.CreationTimestamp = timestamp
}

// NodeCondition contains condition information for a node
type NodeCondition struct {
	Type               string    `json:"type"`
	Status             string    `json:"status"`
	LastHeartbeatTime  time.Time `json:"lastHeartbeatTime,omitempty"`
	LastTransitionTime time.Time `json:"lastTransitionTime,omitempty"`
	Reason             string    `json:"reason,omitempty"`
	Message            string    `json:"message,omitempty"`
}

// NodeAddress contains information for the node's address
type NodeAddress struct {
	Type    string `json:"type"`
	Address string `json:"address"`
}

// NodeDaemonEndpoints lists ports opened by daemons running on the Node
type NodeDaemonEndpoints struct {
	KubeletEndpoint DaemonEndpoint `json:"kubeletEndpoint,omitempty"`
}

// DaemonEndpoint contains information about a single Daemon endpoint
type DaemonEndpoint struct {
	Port int32 `json:"port"`
}

// NodeSystemInfo is a set of ids/uuids to uniquely identify the node
type NodeSystemInfo struct {
	MachineID               string `json:"machineID"`
	SystemUUID              string `json:"systemUUID"`
	BootID                  string `json:"bootID"`
	KernelVersion           string `json:"kernelVersion"`
	OSImage                 string `json:"osImage"`
	ContainerRuntimeVersion string `json:"containerRuntimeVersion"`
	KubeletVersion          string `json:"kubeletVersion"`
	OperatingSystem         string `json:"operatingSystem"`
	Architecture            string `json:"architecture"`
}

// Taint represents a taint applied to a node
type Taint struct {
	Key    string `json:"key"`
	Value  string `json:"value,omitempty"`
	Effect string `json:"effect"`
}

// Volume represents a named volume in a pod
type Volume struct {
	Name         string       `json:"name"`
	VolumeSource VolumeSource `json:"volumeSource"`
}

// VolumeSource represents the source of a volume
type VolumeSource struct {
	HostPath *HostPathVolumeSource `json:"hostPath,omitempty"`
	EmptyDir *EmptyDirVolumeSource `json:"emptyDir,omitempty"`
}

// HostPathVolumeSource represents a host path mapped into a pod
type HostPathVolumeSource struct {
	Path string `json:"path"`
	Type string `json:"type,omitempty"`
}

// EmptyDirVolumeSource represents an empty directory for a pod
type EmptyDirVolumeSource struct {
	Medium string `json:"medium,omitempty"`
}

// LocalObjectReference contains enough information to let you locate the referenced object
type LocalObjectReference struct {
	Name string `json:"name"`
}

// PodPhase is a label for the condition of a pod at the current time
type PodPhase string

const (
	// PodPending means the pod has been accepted by the system, but one or more of the
	// containers has not been started
	PodPending PodPhase = "Pending"
	// PodScheduled means the pod has been scheduled to a node
	PodScheduled PodPhase = "Scheduled"
	// PodRunning means the pod has been bound to a node and all of the containers have been started
	PodRunning PodPhase = "Running"
	// PodSucceeded means that all containers in the pod have voluntarily terminated
	// with a container exit code of 0
	PodSucceeded PodPhase = "Succeeded"
	// PodFailed means that all containers in the pod have terminated, and at least one container has
	// terminated in a failure (exited with a non-zero exit code or was stopped by the system)
	PodFailed PodPhase = "Failed"
	// PodUnknown means that for some reason the state of the pod could not be obtained
	PodUnknown PodPhase = "Unknown"
)

// NodePhase is a label for the condition of a node at the current time
type NodePhase string

const (
	// NodePending means the node has been created/added to the cluster, but not ready
	NodePending NodePhase = "Pending"
	// NodeRunning means the node is running and ready
	NodeRunning NodePhase = "Running"
	// NodeTerminated means the node has been removed from the cluster
	NodeTerminated NodePhase = "Terminated"
)

// OwnerReference contains enough information to let you identify an owning object
type OwnerReference struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	UID        string `json:"uid"`
}

// LabelSelector represents a label selector
type LabelSelector struct {
	MatchLabels map[string]string `json:"matchLabels,omitempty"`
}

// PodTemplateSpec describes the data a pod should have when created from a template
type PodTemplateSpec struct {
	ObjectMeta `json:"metadata,omitempty"`
	Spec       PodSpec `json:"spec,omitempty"`
}

// DeploymentSpec describes the desired state of a Deployment
type DeploymentSpec struct {
	Replicas int32           `json:"replicas,omitempty"`
	Selector *LabelSelector  `json:"selector"`
	Template PodTemplateSpec `json:"template"`
}

// DeploymentStatus represents the current state of a Deployment
type DeploymentStatus struct {
	Replicas            int32 `json:"replicas,omitempty"`
	UpdatedReplicas     int32 `json:"updatedReplicas,omitempty"`
	AvailableReplicas   int32 `json:"availableReplicas,omitempty"`
	UnavailableReplicas int32 `json:"unavailableReplicas,omitempty"`
}

// Deployment represents a deployment
type Deployment struct {
	TypeMeta   `json:",inline"`
	ObjectMeta `json:"metadata"`
	Spec       DeploymentSpec   `json:"spec"`
	Status     DeploymentStatus `json:"status"`
}

// GetKind returns the kind of the deployment
func (d *Deployment) GetKind() string {
	return d.Kind
}

// GetAPIVersion returns the API version of the deployment
func (d *Deployment) GetAPIVersion() string {
	return d.APIVersion
}

// GetName returns the name of the deployment
func (d *Deployment) GetName() string {
	return d.Name
}

// GetNamespace returns the namespace of the deployment
func (d *Deployment) GetNamespace() string {
	return d.Namespace
}

// GetUID returns the UID of the deployment
func (d *Deployment) GetUID() string {
	return d.UID
}

// GetResourceVersion returns the resource version of the deployment
func (d *Deployment) GetResourceVersion() string {
	return d.ResourceVersion
}

// SetResourceVersion sets the resource version of the deployment
func (d *Deployment) SetResourceVersion(version string) {
	d.ResourceVersion = version
}

// GetCreationTimestamp returns the creation timestamp of the deployment
func (d *Deployment) GetCreationTimestamp() time.Time {
	return d.CreationTimestamp
}

// SetCreationTimestamp sets the creation timestamp of the deployment
func (d *Deployment) SetCreationTimestamp(timestamp time.Time) {
	d.CreationTimestamp = timestamp
}

// ReplicaSetSpec describes the desired state of a ReplicaSet
type ReplicaSetSpec struct {
	Replicas int32           `json:"replicas,omitempty"`
	Selector *LabelSelector  `json:"selector"`
	Template PodTemplateSpec `json:"template"`
}

// ReplicaSetStatus represents the current state of a ReplicaSet
type ReplicaSetStatus struct {
	Replicas             int32 `json:"replicas"`
	FullyLabeledReplicas int32 `json:"fullyLabeledReplicas,omitempty"`
	ReadyReplicas        int32 `json:"readyReplicas,omitempty"`
	AvailableReplicas    int32 `json:"availableReplicas,omitempty"`
}

// ReplicaSet represents a ReplicaSet
type ReplicaSet struct {
	TypeMeta   `json:",inline"`
	ObjectMeta `json:"metadata"`
	Spec       ReplicaSetSpec   `json:"spec"`
	Status     ReplicaSetStatus `json:"status"`
}

// GetKind returns the kind of the replicaset
func (r *ReplicaSet) GetKind() string {
	return r.Kind
}

// GetAPIVersion returns the API version of the replicaset
func (r *ReplicaSet) GetAPIVersion() string {
	return r.APIVersion
}

// GetName returns the name of the replicaset
func (r *ReplicaSet) GetName() string {
	return r.Name
}

// GetNamespace returns the namespace of the replicaset
func (r *ReplicaSet) GetNamespace() string {
	return r.Namespace
}

// GetUID returns the UID of the replicaset
func (r *ReplicaSet) GetUID() string {
	return r.UID
}

// GetResourceVersion returns the resource version of the replicaset
func (r *ReplicaSet) GetResourceVersion() string {
	return r.ResourceVersion
}

// SetResourceVersion sets the resource version of the replicaset
func (r *ReplicaSet) SetResourceVersion(version string) {
	r.ResourceVersion = version
}

// GetCreationTimestamp returns the creation timestamp of the replicaset
func (r *ReplicaSet) GetCreationTimestamp() time.Time {
	return r.CreationTimestamp
}

// SetCreationTimestamp sets the creation timestamp of the replicaset
func (r *ReplicaSet) SetCreationTimestamp(timestamp time.Time) {
	r.CreationTimestamp = timestamp
}
