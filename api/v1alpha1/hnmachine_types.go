/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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

type HnMachineSpec struct {
	// ProviderID is the identifier for the HnMachine instance
	// +optional
	ProviderID *string `json:"providerID,omitempty"`

	// Template defines the desired state of the Pod
	Template HnMachinePodTemplate `json:"template"`

	// Bootstrap specifies the bootstrap configuration for this machine.
	Bootstrap clusterv1.Bootstrap `json:"bootstrap"`

	// BootstrapCheckSpec defines how the controller is checking CAPI Sentinel file inside the container.
	// +optional
	BootstrapCheckSpec BootstrapCheckSpec `json:"bootstrapCheckSpec,omitempty"`

	// InfraClusterSecretRef is a reference to a secret with credentials for the infrastructure where the container will run.
	// +optional
	InfraClusterSecretRef *corev1.ObjectReference `json:"infraClusterSecretRef,omitempty"`
}

type HnMachinePodTemplate struct {
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Specification of the desired behavior of the pod.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	// +optional
	Spec corev1.PodSpec `json:"spec,omitempty"`
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
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type="boolean",JSONPath=".status.ready",description="Machine ready status"
// +kubebuilder:printcolumn:name="ProviderID",type="string",JSONPath=".spec.providerID",description="Provider ID"
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

func init() {
	SchemeBuilder.Register(&HnMachine{}, &HnMachineList{})
}
