package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

const (
	// ClusterFinalizer allows HnClusterReconciler to clean up resources associated with HnCluster before
	// removing it from the apiserver.
	ClusterFinalizer = "hncluster.infrastructure.cluster.x-k8s.io"
)

// HnClusterSpec defines the desired state of HnCluster
type HnClusterSpec struct {
	// ControlPlaneEndpoint represents the endpoint used to communicate with the control plane.
	ControlPlaneEndpoint clusterv1.APIEndpoint `json:"controlPlaneEndpoint"`

	// ControlPlaneConfig defines the desired state of the control plane.
	// +optional
	ControlPlaneConfig *HnControlPlaneConfig `json:"controlPlaneConfig,omitempty"`

	// NetworkConfig defines the cluster networking configuration.
	// +optional
	NetworkConfig *HnNetworkConfig `json:"networkConfig,omitempty"`
}

// HnControlPlaneConfig defines the desired state of the control plane
type HnControlPlaneConfig struct {
	// Replicas is the number of control plane nodes.
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// MachineType is the type of machine to use for control plane nodes.
	// +optional
	MachineType string `json:"machineType,omitempty"`
}

// HnNetworkConfig defines the networking configuration for the cluster
type HnNetworkConfig struct {
	// Pods defines the pod network CIDR.
	// +optional
	Pods *string `json:"pods,omitempty"`

	// Services defines the service network CIDR.
	// +optional
	Services *string `json:"services,omitempty"`
}

// HnClusterStatus defines the observed state of HnCluster
type HnClusterStatus struct {
	// Ready denotes that the cluster is ready.
	// +optional
	Ready bool `json:"ready"`

	// FailureReason will be set in the event that there is a terminal problem
	// reconciling the cluster and will contain a succinct value suitable
	// for machine interpretation.
	// +optional
	FailureReason *string `json:"failureReason,omitempty"`

	// FailureMessage will be set in the event that there is a terminal problem
	// reconciling the cluster and will contain a more verbose string suitable
	// for logging and human consumption.
	// +optional
	FailureMessage *string `json:"failureMessage,omitempty"`

	// Conditions defines current service state of the HnCluster.
	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type="boolean",JSONPath=".status.ready",description="Cluster ready status"
// +kubebuilder:printcolumn:name="ControlPlaneEndpoint",type="string",JSONPath=".spec.controlPlaneEndpoint",description="Control Plane Endpoint"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation of HnCluster"

// HnCluster is the Schema for the hnclusters API
type HnCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HnClusterSpec   `json:"spec,omitempty"`
	Status HnClusterStatus `json:"status,omitempty"`
}

// GetConditions returns the conditions of the HnCluster.
func (c *HnCluster) GetConditions() clusterv1.Conditions {
	return c.Status.Conditions
}

// SetConditions sets the conditions of the HnCluster.
func (c *HnCluster) SetConditions(conditions clusterv1.Conditions) {
	c.Status.Conditions = conditions
}

// +kubebuilder:object:root=true

// HnClusterList contains a list of HnCluster
type HnClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HnCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&HnCluster{}, &HnClusterList{})
}
