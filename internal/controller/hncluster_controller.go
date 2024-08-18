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

package controller

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	controlplanev1 "sigs.k8s.io/cluster-api/controlplane/kubeadm/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	hnv1alpha1 "github.com/appthrust/hosted-node/api/v1alpha1"
)

// HnClusterReconciler reconciles a HnCluster object
type HnClusterReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=hn.appthrust.io,resources=hnclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=hn.appthrust.io,resources=hnclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=hn.appthrust.io,resources=hnclusters/finalizers,verbs=update
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *HnClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling HnCluster")

	// Fetch the HnCluster instance
	hnCluster := &hnv1alpha1.HnCluster{}
	if err := r.Get(ctx, req.NamespacedName, hnCluster); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("HnCluster resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get HnCluster")
		return ctrl.Result{}, err
	}

	// Initialize the patch helper
	patchHelper, err := patch.NewHelper(hnCluster, r.Client)
	if err != nil {
		logger.Error(err, "Failed to initialize patch helper")
		return ctrl.Result{}, err
	}

	// Always attempt to Patch the HnCluster object and status after each reconciliation.
	defer func() {
		if err := patchHelper.Patch(ctx, hnCluster); err != nil {
			logger.Error(err, "Failed to patch HnCluster")
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	// Add finalizer first if not exist to avoid the race condition between init and delete
	if !controllerutil.ContainsFinalizer(hnCluster, hnv1alpha1.ClusterFinalizer) {
		controllerutil.AddFinalizer(hnCluster, hnv1alpha1.ClusterFinalizer)
		logger.Info("Added finalizer to HnCluster")
		return ctrl.Result{}, nil
	}

	// Handle deleted clusters
	if !hnCluster.ObjectMeta.DeletionTimestamp.IsZero() {
		logger.Info("HnCluster is being deleted")
		return r.reconcileDelete(ctx, hnCluster)
	}

	// Handle non-deleted clusters
	return r.reconcileNormal(ctx, hnCluster)
}

func (r *HnClusterReconciler) reconcileNormal(ctx context.Context, hnCluster *hnv1alpha1.HnCluster) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the Cluster
	cluster, err := util.GetOwnerCluster(ctx, r.Client, hnCluster.ObjectMeta)
	if err != nil {
		logger.Error(err, "Failed to get owner Cluster")
		conditions.MarkFalse(hnCluster, clusterv1.ReadyCondition, "OwnerClusterNotFound", clusterv1.ConditionSeverityError, err.Error())
		return ctrl.Result{}, err
	}
	if cluster == nil {
		logger.Info("Waiting for Cluster Controller to set OwnerRef on HnCluster")
		conditions.MarkFalse(hnCluster, clusterv1.ReadyCondition, "OwnerClusterNotSet", clusterv1.ConditionSeverityInfo, "Waiting for Cluster Controller to set OwnerRef")
		return ctrl.Result{}, nil
	}

	logger = logger.WithValues("cluster", cluster.Name)

	// Update ControlPlaneEndpoint if it has changed
	if hnCluster.Spec.ControlPlaneEndpoint != cluster.Spec.ControlPlaneEndpoint {
		hnCluster.Spec.ControlPlaneEndpoint = cluster.Spec.ControlPlaneEndpoint
		logger.Info("Updated ControlPlaneEndpoint", "endpoint", hnCluster.Spec.ControlPlaneEndpoint)
	}

	// Apply network configuration
	if err := r.applyNetworkConfig(ctx, hnCluster, cluster); err != nil {
		logger.Error(err, "Failed to apply network configuration")
		conditions.MarkFalse(hnCluster, clusterv1.ReadyCondition, "NetworkConfigFailed", clusterv1.ConditionSeverityError, err.Error())
		return ctrl.Result{}, err
	}

	// Manage control plane configuration
	if err := r.manageControlPlaneConfig(ctx, hnCluster, cluster); err != nil {
		logger.Error(err, "Failed to manage control plane configuration")
		conditions.MarkFalse(hnCluster, clusterv1.ReadyCondition, "ControlPlaneConfigFailed", clusterv1.ConditionSeverityError, err.Error())
		return ctrl.Result{}, err
	}

	// Check overall cluster health
	healthy, err := r.checkClusterHealth(ctx, hnCluster, cluster)
	if err != nil {
		logger.Error(err, "Failed to check cluster health")
		conditions.MarkFalse(hnCluster, clusterv1.ReadyCondition, "HealthCheckFailed", clusterv1.ConditionSeverityError, err.Error())
		return ctrl.Result{}, err
	}

	if healthy {
		hnCluster.Status.Ready = true
		conditions.MarkTrue(hnCluster, clusterv1.ReadyCondition)
		logger.Info("HnCluster is healthy and ready")
	} else {
		hnCluster.Status.Ready = false
		conditions.MarkFalse(hnCluster, clusterv1.ReadyCondition, "ClusterNotHealthy", clusterv1.ConditionSeverityWarning, "Cluster is not in a healthy state")
		logger.Info("HnCluster is not ready", "reason", "ClusterNotHealthy")
	}

	logger.Info("Successfully reconciled HnCluster")
	return ctrl.Result{}, nil
}

func (r *HnClusterReconciler) reconcileDelete(ctx context.Context, hnCluster *hnv1alpha1.HnCluster) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling HnCluster delete")

	// TODO: Add your deletion logic here
	// For example:
	// - Clean up any resources created by this HnCluster
	// - Remove any finalizers

	// Example: Remove the finalizer
	controllerutil.RemoveFinalizer(hnCluster, hnv1alpha1.ClusterFinalizer)

	logger.Info("Successfully deleted HnCluster")
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *HnClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hnv1alpha1.HnCluster{}).
		Complete(r)
}

func (r *HnClusterReconciler) applyNetworkConfig(ctx context.Context, hnCluster *hnv1alpha1.HnCluster, cluster *clusterv1.Cluster) error {
	logger := log.FromContext(ctx)

	if hnCluster.Spec.NetworkConfig == nil {
		logger.Info("NetworkConfig is not specified, skipping network configuration")
		return nil
	}

	// Apply Pod network CIDR
	if hnCluster.Spec.NetworkConfig.Pods != nil {
		if cluster.Spec.ClusterNetwork.Pods == nil {
			cluster.Spec.ClusterNetwork.Pods = &clusterv1.NetworkRanges{}
		}
		cluster.Spec.ClusterNetwork.Pods.CIDRBlocks = []string{*hnCluster.Spec.NetworkConfig.Pods}
		logger.Info("Applied Pod network CIDR", "CIDR", *hnCluster.Spec.NetworkConfig.Pods)
	}

	// Apply Service network CIDR
	if hnCluster.Spec.NetworkConfig.Services != nil {
		if cluster.Spec.ClusterNetwork.Services == nil {
			cluster.Spec.ClusterNetwork.Services = &clusterv1.NetworkRanges{}
		}
		cluster.Spec.ClusterNetwork.Services.CIDRBlocks = []string{*hnCluster.Spec.NetworkConfig.Services}
		logger.Info("Applied Service network CIDR", "CIDR", *hnCluster.Spec.NetworkConfig.Services)
	}

	// Update the Cluster object
	if err := r.Update(ctx, cluster); err != nil {
		return fmt.Errorf("failed to update Cluster network configuration: %w", err)
	}

	logger.Info("Successfully applied network configuration")
	return nil
}

func (r *HnClusterReconciler) manageControlPlaneConfig(ctx context.Context, hnCluster *hnv1alpha1.HnCluster, cluster *clusterv1.Cluster) error {
	logger := log.FromContext(ctx)

	if hnCluster.Spec.ControlPlaneConfig == nil {
		logger.Info("ControlPlaneConfig is not specified, skipping control plane configuration")
		return nil
	}

	// Fetch the KubeadmControlPlane object
	kcp := &controlplanev1.KubeadmControlPlane{}
	if err := r.Get(ctx, client.ObjectKey{Namespace: cluster.Namespace, Name: cluster.Spec.ControlPlaneRef.Name}, kcp); err != nil {
		return fmt.Errorf("failed to get KubeadmControlPlane: %w", err)
	}

	updated := false

	// Update replicas if specified
	if hnCluster.Spec.ControlPlaneConfig.Replicas != nil &&
		(kcp.Spec.Replicas == nil || *hnCluster.Spec.ControlPlaneConfig.Replicas != *kcp.Spec.Replicas) {
		kcp.Spec.Replicas = hnCluster.Spec.ControlPlaneConfig.Replicas
		updated = true
		logger.Info("Updated control plane replicas", "replicas", *hnCluster.Spec.ControlPlaneConfig.Replicas)
	}

	// Update machine type if specified
	if hnCluster.Spec.ControlPlaneConfig.MachineType != "" {
		// Assuming the machine type is set in the KubeadmControlPlaneTemplate
		if kcp.Spec.MachineTemplate.InfrastructureRef.Kind == "HnMachineTemplate" {
			hnMachineTemplate := &hnv1alpha1.HnMachineTemplate{}
			if err := r.Get(ctx, client.ObjectKey{Namespace: kcp.Namespace, Name: kcp.Spec.MachineTemplate.InfrastructureRef.Name}, hnMachineTemplate); err != nil {
				return fmt.Errorf("failed to get HnMachineTemplate: %w", err)
			}

			if hnMachineTemplate.Spec.Template.Spec.MachineType != hnCluster.Spec.ControlPlaneConfig.MachineType {
				hnMachineTemplate.Spec.Template.Spec.MachineType = hnCluster.Spec.ControlPlaneConfig.MachineType
				if err := r.Update(ctx, hnMachineTemplate); err != nil {
					return fmt.Errorf("failed to update HnMachineTemplate: %w", err)
				}
				updated = true
				logger.Info("Updated control plane machine type", "machineType", hnCluster.Spec.ControlPlaneConfig.MachineType)
			}
		}
	}

	// Update the KubeadmControlPlane object if changes were made
	if updated {
		if err := r.Update(ctx, kcp); err != nil {
			return fmt.Errorf("failed to update KubeadmControlPlane: %w", err)
		}
		logger.Info("Successfully updated control plane configuration")
	} else {
		logger.Info("No changes required for control plane configuration")
	}

	return nil
}

func (r *HnClusterReconciler) checkClusterHealth(ctx context.Context, hnCluster *hnv1alpha1.HnCluster, cluster *clusterv1.Cluster) (bool, error) {
	logger := log.FromContext(ctx)

	// Check control plane health
	if !cluster.Status.ControlPlaneReady {
		logger.Info("Control plane is not ready")
		return false, nil
	}

	// Check infrastructure health
	if !cluster.Status.InfrastructureReady {
		logger.Info("Infrastructure is not ready")
		return false, nil
	}

	// Check node health
	nodeList := &corev1.NodeList{}
	if err := r.List(ctx, nodeList, client.InNamespace(cluster.Namespace)); err != nil {
		return false, fmt.Errorf("failed to list nodes: %w", err)
	}

	readyNodes := 0
	for _, node := range nodeList.Items {
		if isNodeReady(&node) {
			readyNodes++
		}
	}

	if readyNodes == 0 {
		logger.Info("No ready nodes found")
		return false, nil
	}

	// Check important component health (e.g., CoreDNS, kube-proxy)
	componentStatuses := &corev1.ComponentStatusList{}
	if err := r.List(ctx, componentStatuses); err != nil {
		return false, fmt.Errorf("failed to list component statuses: %w", err)
	}

	for _, cs := range componentStatuses.Items {
		if !isComponentHealthy(&cs) {
			logger.Info("Component is not healthy", "component", cs.Name)
			return false, nil
		}
	}

	logger.Info("Cluster is healthy", "readyNodes", readyNodes)
	return true, nil
}

func isNodeReady(node *corev1.Node) bool {
	for _, condition := range node.Status.Conditions {
		if condition.Type == corev1.NodeReady {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}

func isComponentHealthy(cs *corev1.ComponentStatus) bool {
	for _, condition := range cs.Conditions {
		if condition.Type == corev1.ComponentHealthy {
			return condition.Status == corev1.ConditionTrue
		}
	}
	return false
}
