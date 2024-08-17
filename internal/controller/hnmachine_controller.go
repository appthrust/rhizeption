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
	"reflect"
	"time"

	"math"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	hnv1alpha1 "github.com/appthrust/hosted-node/api/v1alpha1"
)

// HnMachineReconciler reconciles a HnMachine object
type HnMachineReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=hn.appthrust.io,resources=hnmachines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=hn.appthrust.io,resources=hnmachines/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=hn.appthrust.io,resources=hnmachines/finalizers,verbs=update
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;machines,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *HnMachineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	logger := log.FromContext(ctx)

	// Fetch the HnMachine instance
	hnMachine := &hnv1alpha1.HnMachine{}
	if err := r.Get(ctx, req.NamespacedName, hnMachine); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Fetch the Machine
	machine, err := util.GetOwnerMachine(ctx, r.Client, hnMachine.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, err
	}
	if machine == nil {
		logger.Info("Waiting for Machine Controller to set OwnerRef on HnMachine")
		return ctrl.Result{}, nil
	}

	logger = logger.WithValues("machine", machine.Name)

	// Fetch the Cluster
	cluster, err := util.GetClusterFromMetadata(ctx, r.Client, machine.ObjectMeta)
	if err != nil {
		logger.Info("HnMachine owner Machine is missing cluster label or cluster does not exist")
		return ctrl.Result{}, err
	}
	if cluster == nil {
		logger.Info(fmt.Sprintf("Please associate this machine with a cluster using the label %s: <name of cluster>", clusterv1.ClusterNameLabel))
		return ctrl.Result{}, nil
	}

	// logger = logger.WithValues("cluster", cluster.Name)

	// Initialize the patch helper
	patchHelper, err := patch.NewHelper(hnMachine, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}
	// Always attempt to Patch the HnMachine object and status after each reconciliation.
	defer func() {
		if err := patchHelper.Patch(ctx, hnMachine); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	// Add finalizer first if not exist to avoid the race condition between init and delete
	if !controllerutil.ContainsFinalizer(hnMachine, hnv1alpha1.MachineFinalizer) {
		controllerutil.AddFinalizer(hnMachine, hnv1alpha1.MachineFinalizer)
		return ctrl.Result{}, nil
	}

	// Handle deleted machines
	if !hnMachine.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, hnMachine)
	}

	// Handle non-deleted machines
	return r.reconcileNormal(ctx, hnMachine, machine, cluster)
}

func (r *HnMachineReconciler) reconcileNormal(ctx context.Context, hnMachine *hnv1alpha1.HnMachine, machine *clusterv1.Machine, cluster *clusterv1.Cluster) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// If the HnMachine doesn't have our finalizer, add it.
	controllerutil.AddFinalizer(hnMachine, hnv1alpha1.MachineFinalizer)

	if !cluster.Status.InfrastructureReady {
		logger.Info("Cluster infrastructure is not ready yet")
		conditions.MarkFalse(hnMachine, hnv1alpha1.ContainerProvisionedCondition, hnv1alpha1.WaitingForClusterInfrastructureReason, clusterv1.ConditionSeverityInfo, "")
		return ctrl.Result{}, nil
	}

	// Make sure bootstrap data is available and populated.
	if machine.Spec.Bootstrap.DataSecretName == nil {
		logger.Info("Waiting for the Bootstrap provider controller to set bootstrap data")
		return ctrl.Result{}, nil
	}

	// Create or update the container
	if err := r.reconcileContainer(ctx, hnMachine); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to reconcile container")
	}

	hnMachine.Status.Ready = true
	hnMachine.Status.Addresses = []clusterv1.MachineAddress{
		{
			Type:    clusterv1.MachineHostName,
			Address: hnMachine.Name,
		},
		{
			Type:    clusterv1.MachineInternalIP,
			Address: "localhost", // This should be updated with the actual IP
		},
	}

	if hnMachine.Spec.ProviderID == nil {
		hnMachine.Spec.ProviderID = r.getProviderID(hnMachine)
	}

	return ctrl.Result{}, nil
}

func (r *HnMachineReconciler) reconcileDelete(ctx context.Context, hnMachine *hnv1alpha1.HnMachine) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Deleting HnMachine")

	// Delete the container
	if err := r.deleteContainer(ctx, hnMachine); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to delete container")
	}

	// Container is deleted so remove the finalizer.
	controllerutil.RemoveFinalizer(hnMachine, hnv1alpha1.MachineFinalizer)

	return ctrl.Result{}, nil
}

func (r *HnMachineReconciler) reconcileContainer(ctx context.Context, hnMachine *hnv1alpha1.HnMachine) error {
	log := log.FromContext(ctx)

	// 1. Check if the Pod exists
	pod := &corev1.Pod{}
	err := r.Get(ctx, client.ObjectKey{Namespace: hnMachine.Namespace, Name: hnMachine.Name}, pod)
	if err != nil && !apierrors.IsNotFound(err) {
		conditions.MarkFalse(hnMachine, hnv1alpha1.ContainerProvisionedCondition, "FailedToGetPod", clusterv1.ConditionSeverityError, err.Error())
		return err
	}

	if apierrors.IsNotFound(err) {
		// 2. If the Pod doesn't exist, create a new Pod
		log.Info("Creating new Pod for HnMachine", "hnMachine", hnMachine.Name)
		pod, err = r.createPodForHnMachine(ctx, hnMachine)
		if err != nil {
			conditions.MarkFalse(hnMachine, hnv1alpha1.ContainerProvisionedCondition, "FailedToCreatePod", clusterv1.ConditionSeverityError, err.Error())
			return err
		}
	} else {
		// 3. If the Pod exists, check if an update is needed
		updateRequired, significantChange := r.shouldUpdatePod(hnMachine, pod)
		if updateRequired {
			if significantChange {
				// If there are significant changes, report the status but don't perform the update
				log.Info("Significant changes detected, update required but not performed", "hnMachine", hnMachine.Name)
				conditions.MarkFalse(hnMachine, hnv1alpha1.ContainerProvisionedCondition, "SignificantChangesDetected", clusterv1.ConditionSeverityWarning, "Significant changes detected, new Machine may be required")
				return nil
			}
			// Only perform the update for minor changes
			log.Info("Updating existing Pod for HnMachine", "hnMachine", hnMachine.Name)
			if err := r.updatePod(ctx, hnMachine, pod); err != nil {
				conditions.MarkFalse(hnMachine, hnv1alpha1.ContainerProvisionedCondition, "FailedToUpdatePod", clusterv1.ConditionSeverityError, err.Error())
				return err
			}
		}
	}

	// 4. Check the Pod's status
	if pod.Status.Phase == corev1.PodRunning {
		// If the Pod is running, update the HnMachine status
		hnMachine.Status.Ready = true
		hnMachine.Status.Addresses = []clusterv1.MachineAddress{
			{
				Type:    clusterv1.MachineHostName,
				Address: pod.Spec.NodeName,
			},
			{
				Type:    clusterv1.MachineInternalIP,
				Address: pod.Status.PodIP,
			},
		}
		conditions.MarkTrue(hnMachine, hnv1alpha1.ContainerProvisionedCondition)
	} else {
		// If the Pod is not running yet, update the status
		hnMachine.Status.Ready = false
		conditions.MarkFalse(hnMachine, hnv1alpha1.ContainerProvisionedCondition, "PodNotRunning", clusterv1.ConditionSeverityWarning, fmt.Sprintf("Pod is in %s state", pod.Status.Phase))
	}

	return nil
}

func (r *HnMachineReconciler) createPodForHnMachine(ctx context.Context, hnMachine *hnv1alpha1.HnMachine) (*corev1.Pod, error) {
	// Get the bootstrap data Secret
	bootstrapSecret := &corev1.Secret{}
	if err := r.Get(ctx, client.ObjectKey{Namespace: hnMachine.Namespace, Name: *hnMachine.Spec.Bootstrap.DataSecretName}, bootstrapSecret); err != nil {
		return nil, fmt.Errorf("failed to get bootstrap data secret: %w", err)
	}

	// Copy the PodSpec
	podSpec := hnMachine.Spec.Template.Spec.DeepCopy()

	// Add bootstrap data as an environment variable
	for i := range podSpec.Containers {
		podSpec.Containers[i].Env = append(podSpec.Containers[i].Env, corev1.EnvVar{
			Name: "BOOTSTRAP_DATA",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: bootstrapSecret.Name,
					},
					Key: "value",
				},
			},
		})
	}

	// Pod definition
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hnMachine.Name,
			Namespace: hnMachine.Namespace,
			Labels: map[string]string{
				"app":       "hnmachine",
				"hnmachine": hnMachine.Name,
			},
		},
		Spec: *podSpec,
	}

	// Create the Pod
	if err := r.Create(ctx, pod); err != nil {
		return nil, fmt.Errorf("failed to create Pod: %w", err)
	}

	return pod, nil
}

func (r *HnMachineReconciler) shouldUpdatePod(hnMachine *hnv1alpha1.HnMachine, pod *corev1.Pod) (updateRequired bool, significantChange bool) {
	// 1. Check for minor changes
	if !reflect.DeepEqual(hnMachine.Spec.Template.Labels, pod.Labels) ||
		!reflect.DeepEqual(hnMachine.Spec.Template.Annotations, pod.Annotations) {
		return true, false
	}

	// 2. Check for minor changes in resource requests and limits
	for i, containerSpec := range hnMachine.Spec.Template.Spec.Containers {
		if i < len(pod.Spec.Containers) {
			podContainer := pod.Spec.Containers[i]
			if !areResourcesEqual(containerSpec.Resources, podContainer.Resources) {
				return true, false
			}
		}
	}

	// 3. Check for changes in environment variables (excluding BOOTSTRAP_DATA)
	if !areEnvVarsEqual(hnMachine.Spec.Template.Spec.Containers, pod.Spec.Containers) {
		return true, false
	}

	// 4. Check for significant changes
	if !reflect.DeepEqual(hnMachine.Spec.Template.Spec.Containers, pod.Spec.Containers) ||
		!reflect.DeepEqual(hnMachine.Spec.Template.Spec.Volumes, pod.Spec.Volumes) ||
		!reflect.DeepEqual(hnMachine.Spec.Template.Spec.NodeSelector, pod.Spec.NodeSelector) ||
		!reflect.DeepEqual(hnMachine.Spec.Template.Spec.Affinity, pod.Spec.Affinity) ||
		!reflect.DeepEqual(hnMachine.Spec.Template.Spec.Tolerations, pod.Spec.Tolerations) {
		return true, true
	}

	return false, false
}

// Helper function to tolerate minor changes in resources
func areResourcesEqual(spec, current corev1.ResourceRequirements) bool {
	resourceNames := []corev1.ResourceName{
		corev1.ResourceCPU,
		corev1.ResourceMemory,
	}

	for _, name := range resourceNames {
		if !spec.Requests[name].Equal(current.Requests[name]) ||
			!spec.Limits[name].Equal(current.Limits[name]) {
			// Tolerate changes within 10%
			if isWithinTolerance(spec.Requests[name], current.Requests[name], 0.1) &&
				isWithinTolerance(spec.Limits[name], current.Limits[name], 0.1) {
				continue
			}
			return false
		}
	}
	return true
}

// Helper function to check if a value change is within the specified tolerance
func isWithinTolerance(a, b resource.Quantity, tolerance float64) bool {
	aFloat := float64(a.MilliValue())
	bFloat := float64(b.MilliValue())
	if aFloat == 0 && bFloat == 0 {
		return true
	}
	if aFloat == 0 || bFloat == 0 {
		return false
	}
	diff := math.Abs(aFloat - bFloat)
	average := (aFloat + bFloat) / 2
	return (diff / average) <= tolerance
}

// Helper function to check for changes in environment variables
func areEnvVarsEqual(specContainers, podContainers []corev1.Container) bool {
	for i, specContainer := range specContainers {
		if i < len(podContainers) {
			podContainer := podContainers[i]
			specEnvMap := makeEnvMap(specContainer.Env)
			podEnvMap := makeEnvMap(podContainer.Env)

			for key, specValue := range specEnvMap {
				if key == "BOOTSTRAP_DATA" {
					continue
				}
				if podValue, exists := podEnvMap[key]; !exists || specValue != podValue {
					return false
				}
			}

			for key := range podEnvMap {
				if key == "BOOTSTRAP_DATA" {
					continue
				}
				if _, exists := specEnvMap[key]; !exists {
					return false
				}
			}
		}
	}
	return true
}

// Helper function to convert environment variables to a map
func makeEnvMap(envVars []corev1.EnvVar) map[string]string {
	envMap := make(map[string]string)
	for _, env := range envVars {
		if env.Value != "" {
			envMap[env.Name] = env.Value
		} else if env.ValueFrom != nil {
			// For ValueFrom, consider it the same if it exists
			envMap[env.Name] = "VALUE_FROM_PRESENT"
		}
	}
	return envMap
}

func (r *HnMachineReconciler) updatePod(ctx context.Context, hnMachine *hnv1alpha1.HnMachine, oldPod *corev1.Pod) error {
	log := log.FromContext(ctx)

	// 1. Delete the old Pod
	log.Info("Deleting old Pod for HnMachine", "hnMachine", hnMachine.Name, "pod", oldPod.Name)
	if err := r.Delete(ctx, oldPod); err != nil {
		log.Error(err, "Failed to delete old Pod", "hnMachine", hnMachine.Name, "pod", oldPod.Name)
		return fmt.Errorf("failed to delete old Pod: %w", err)
	}

	// 2. Wait for the Pod to be deleted
	log.Info("Waiting for Pod deletion", "hnMachine", hnMachine.Name, "pod", oldPod.Name)
	ctx, cancel := context.WithTimeout(ctx, time.Second*30)
	defer cancel()

	if err := wait.PollUntilContextTimeout(ctx, time.Second, time.Second*30, true, func(ctx context.Context) (bool, error) {
		var pod corev1.Pod
		if err := r.Get(ctx, client.ObjectKey{Namespace: oldPod.Namespace, Name: oldPod.Name}, &pod); err != nil {
			if apierrors.IsNotFound(err) {
				log.Info("Pod successfully deleted", "hnMachine", hnMachine.Name, "pod", oldPod.Name)
				// Pod is deleted, continue with the rest of your logic
				return true, nil
			}
			// An error occurred, but it's not a "not found" error
			log.Error(err, "Error checking Pod existence", "hnMachine", hnMachine.Name, "pod", oldPod.Name)
			return false, err
		}
		// Pod still exists, continue polling
		log.V(1).Info("Pod still exists, continuing to wait", "hnMachine", hnMachine.Name, "pod", oldPod.Name)
		return false, nil
	}); err != nil {
		// Handle the error (could be context timeout or other errors)
		log.Error(err, "Failed to confirm Pod deletion", "hnMachine", hnMachine.Name, "pod", oldPod.Name)
		return err
	}

	// 3. Create a new Pod
	log.Info("Creating new Pod for HnMachine", "hnMachine", hnMachine.Name)
	newPod, err := r.createPodForHnMachine(ctx, hnMachine)
	if err != nil {
		log.Error(err, "Failed to create new Pod", "hnMachine", hnMachine.Name)
		return fmt.Errorf("failed to create new Pod: %w", err)
	}

	log.Info("Successfully updated Pod for HnMachine", "hnMachine", hnMachine.Name, "newPod", newPod.Name)
	return nil
}

func (r *HnMachineReconciler) deleteContainer(ctx context.Context, hnMachine *hnv1alpha1.HnMachine) error {
	log := log.FromContext(ctx)

	// Get the associated Pod
	pod := &corev1.Pod{}
	err := r.Get(ctx, client.ObjectKey{Namespace: hnMachine.Namespace, Name: hnMachine.Name}, pod)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// If the Pod doesn't exist, consider it already deleted
			log.Info("Pod not found, considering it already deleted", "hnMachine", hnMachine.Name)
			return nil
		}
		// For other errors
		return fmt.Errorf("failed to get Pod: %w", err)
	}

	// Delete the Pod
	log.Info("Deleting Pod for HnMachine", "hnMachine", hnMachine.Name, "pod", pod.Name)
	if err := r.Delete(ctx, pod); err != nil {
		return fmt.Errorf("failed to delete Pod: %w", err)
	}

	// Wait for the Pod to be deleted
	log.Info("Waiting for Pod deletion", "hnMachine", hnMachine.Name, "pod", pod.Name)
	ctx, cancel := context.WithTimeout(ctx, time.Second*30)
	defer cancel()

	if err := wait.PollUntilContextTimeout(ctx, time.Second, time.Second*30, true, func(ctx context.Context) (bool, error) {
		if err := r.Get(ctx, client.ObjectKey{Namespace: pod.Namespace, Name: pod.Name}, &corev1.Pod{}); err != nil {
			if apierrors.IsNotFound(err) {
				log.Info("Pod successfully deleted", "hnMachine", hnMachine.Name, "pod", pod.Name)
				return true, nil
			}
			log.Error(err, "Error checking Pod existence", "hnMachine", hnMachine.Name, "pod", pod.Name)
			return false, err
		}
		log.V(1).Info("Pod still exists, continuing to wait", "hnMachine", hnMachine.Name, "pod", pod.Name)
		return false, nil
	}); err != nil {
		log.Error(err, "Failed to confirm Pod deletion", "hnMachine", hnMachine.Name, "pod", pod.Name)
		return fmt.Errorf("failed to confirm Pod deletion: %w", err)
	}

	log.Info("Successfully deleted Pod for HnMachine", "hnMachine", hnMachine.Name)
	return nil
}

func (r *HnMachineReconciler) getProviderID(hnMachine *hnv1alpha1.HnMachine) *string {
	pid := fmt.Sprintf("hn://%s/%s", hnMachine.Namespace, hnMachine.Name)
	return &pid
}

// SetupWithManager sets up the controller with the Manager.
func (r *HnMachineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&hnv1alpha1.HnMachine{}).
		Complete(r)
}
