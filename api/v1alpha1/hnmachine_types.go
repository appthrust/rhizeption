package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

const (
	// MachineFinalizer allows ReconcileHnMachine to clean up resources associated with HnMachine before
	// removing it from the apiserver.
	MachineFinalizer = "hnmachine.infrastructure.cluster.x-k8s.io"

	// ContainerProvisionedCondition reports on whether the container has been provisioned.
	ContainerProvisionedCondition clusterv1.ConditionType = "ContainerProvisioned"

	// WaitingForClusterInfrastructureReason (Severity=Info) documents a HnMachine waiting for cluster infrastructure to be ready.
	WaitingForClusterInfrastructureReason = "WaitingForClusterInfrastructure"
)

// HnMachineSpec defines the desired state of HnMachine
type HnMachineSpec struct {
	// ProviderID is the identifier for the HnMachine instance
	// +optional
	ProviderID *string `json:"providerID,omitempty"`

	// Template defines the desired state of the Container
	Template HnMachineContainerTemplate `json:"template"`

	// Bootstrap specifies the bootstrap configuration for this machine.
	Bootstrap clusterv1.Bootstrap `json:"bootstrap"`

	// BootstrapCheckSpec defines how the controller is checking CAPI Sentinel file inside the container.
	// +optional
	BootstrapCheckSpec BootstrapCheckSpec `json:"bootstrapCheckSpec,omitempty"`

	// ClusterName is the name of the Cluster this object belongs to.
	// +kubebuilder:validation:MinLength=1
	ClusterName string `json:"clusterName"`

	// FailureDomain is the failure domain the machine will be created in.
	// Must match a key in the FailureDomains map stored on the cluster object.
	// +optional
	FailureDomain *string `json:"failureDomain,omitempty"`

	// MachineType is the type of machine to use for this HnMachine
	// +optional
	MachineType string `json:"machineType,omitempty"`
}

// HnMachineContainerTemplate defines the core settings for the HnMachine container
type HnMachineContainerTemplate struct {
	// Image is the container image to use
	Image string `json:"image"`

	// Command is the entrypoint array. Not executed within a shell.
	// +optional
	Command []string `json:"command,omitempty"`

	// Args are the arguments to the entrypoint.
	// +optional
	Args []string `json:"args,omitempty"`

	// Env is a list of environment variables to set in the container.
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// Resources are the compute resource requirements.
	// +optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// VolumeMounts are the volumes to mount into the container's filesystem.
	// +optional
	VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty"`

	// SecurityContext holds security configuration that will be applied to the container.
	// +optional
	SecurityContext *corev1.SecurityContext `json:"securityContext,omitempty"`
}

// HnMachineTemplateSpec defines the desired state of HnMachineTemplate
type HnMachineTemplateSpec struct {
	Template HnMachineTemplateResource `json:"template"`
}

// HnMachineTemplateResource describes the data needed to create a HnMachine from a template
type HnMachineTemplateResource struct {
	// Spec is the specification of the desired behavior of the machine.
	Spec HnMachineSpec `json:"spec"`
}

// BootstrapCheckSpec defines how the controller will remotely check CAPI Sentinel file content.
type BootstrapCheckSpec struct {
	// CheckStrategy describes how the controller will validate a successful CAPI bootstrap.
	// Possible values are: "none" or "exec" (default is "exec") and this value is validated by apiserver.
	// +optional
	// +kubebuilder:validation:Enum=none;exec
	// +kubebuilder:default:=exec
	CheckStrategy string `json:"checkStrategy,omitempty"`
}

// HnMachineStatus defines the observed state of HnMachine
type HnMachineStatus struct {
	// Ready denotes that the machine is ready
	// +kubebuilder:default=false
	Ready bool `json:"ready"`

	// Addresses contains the associated addresses for the machine.
	// +optional
	Addresses []clusterv1.MachineAddress `json:"addresses,omitempty"`

	// Conditions defines current service state of the HnMachine.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`

	// FailureReason will be set in the event that there is a terminal problem
	// reconciling the Machine and will contain a succinct value suitable
	// for machine interpretation.
	// +optional
	FailureReason *string `json:"failureReason,omitempty"`

	// FailureMessage will be set in the event that there is a terminal problem
	// reconciling the Machine and will contain a more verbose string suitable
	// for logging and human consumption.
	// +optional
	FailureMessage *string `json:"failureMessage,omitempty"`

	// Phase represents the current phase of machine actuation.
	// E.g. Pending, Running, Terminating, Failed etc.
	// +optional
	Phase string `json:"phase,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".spec.clusterName",description="Cluster"
// +kubebuilder:printcolumn:name="Ready",type="boolean",JSONPath=".status.ready",description="Machine ready status"
// +kubebuilder:printcolumn:name="ProviderID",type="string",JSONPath=".spec.providerID",description="Provider ID"
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase",description="Machine phase"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation of HnMachine"

// HnMachine is the Schema for the hnmachines API
type HnMachine struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HnMachineSpec   `json:"spec,omitempty"`
	Status HnMachineStatus `json:"status,omitempty"`
}

// GetConditions returns the conditions of the HnMachine.
func (m *HnMachine) GetConditions() clusterv1.Conditions {
	return m.Status.Conditions
}

// SetConditions sets the conditions of the HnMachine.
func (m *HnMachine) SetConditions(conditions clusterv1.Conditions) {
	m.Status.Conditions = conditions
}

// +kubebuilder:object:root=true

// HnMachineList contains a list of HnMachine
type HnMachineList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HnMachine `json:"items"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// HnMachineTemplate is the Schema for the hnmachinetemplates API
type HnMachineTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec HnMachineTemplateSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// HnMachineTemplateList contains a list of HnMachineTemplate
type HnMachineTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HnMachineTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&HnMachine{}, &HnMachineList{})
}

func init() {
	SchemeBuilder.Register(&HnMachine{}, &HnMachineList{})
	SchemeBuilder.Register(&HnMachineTemplate{}, &HnMachineTemplateList{})
}
