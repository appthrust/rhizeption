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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	hnv1alpha1 "github.com/appthrust/hosted-node/api/v1alpha1"
)

const (
	dummyUID = "12345678-1234-1234-1234-123456789abc"
)

var _ = Describe("HnMachine Controller", func() {
	const (
		timeout  = time.Second * 10
		interval = time.Millisecond * 250
	)

	BeforeEach(func() {
		// Ensure the CRD is registered
		fmt.Println("Checking if HnMachine CRD is registered...")
		Eventually(func() error {
			crd := &apiextensionsv1.CustomResourceDefinition{}
			err := k8sClient.Get(context.Background(), types.NamespacedName{Name: "hnmachines.hn.appthrust.io"}, crd)
			if err != nil {
				fmt.Printf("Error getting HnMachine CRD: %v\n", err)
				return err
			}
			fmt.Println("HnMachine CRD is registered")
			return nil
		}, timeout, interval).Should(Succeed())
	})

	Context("When reconciling a resource", func() {
		const (
			resourceName      = "test-hnmachine"
			resourceNamespace = "default"
		)

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: resourceNamespace,
		}

		BeforeEach(func() {
			// Ensure cleanup from previous tests
			fmt.Println("Cleaning up resources...")
			cleanupResources(ctx)

			// Create a mock cluster
			cluster := &clusterv1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: resourceNamespace,
				},
				Status: clusterv1.ClusterStatus{
					InfrastructureReady: true,
				},
			}
			Expect(k8sClient.Create(ctx, cluster)).To(Succeed())

			// Create a mock machine
			machine := &clusterv1.Machine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-machine",
					Namespace: resourceNamespace,
					Labels: map[string]string{
						clusterv1.ClusterNameLabel: "test-cluster",
					},
				},
				Spec: clusterv1.MachineSpec{
					ClusterName: "test-cluster",
					Bootstrap: clusterv1.Bootstrap{
						DataSecretName: pointer.String("test-bootstrap-data"),
					},
				},
			}
			Expect(k8sClient.Create(ctx, machine)).To(Succeed())

			// Create a mock bootstrap data secret
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-bootstrap-data",
					Namespace: resourceNamespace,
				},
				Data: map[string][]byte{
					"value": []byte("test-bootstrap-data"),
				},
			}
			Expect(k8sClient.Create(ctx, secret)).To(Succeed())

			fmt.Println("Mock resources created successfully")
		})

		AfterEach(func() {
			fmt.Println("Cleaning up resources after test...")
			cleanupResources(ctx)
		})

		It("should successfully reconcile a new HnMachine", func() {
			By("Creating a new HnMachine")
			hnMachine := &hnv1alpha1.HnMachine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: resourceNamespace,
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "cluster.x-k8s.io/v1beta1",
							Kind:       "Machine",
							Name:       "test-machine",
							UID:        types.UID(dummyUID),
						},
					},
				},
				Spec: hnv1alpha1.HnMachineSpec{
					ProviderID: pointer.String("hn://default/test-hnmachine"),
					Template: hnv1alpha1.HnMachineContainerTemplate{
						Image: "test-image",
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("1"),
								corev1.ResourceMemory: resource.MustParse("1Gi"),
							},
						},
					},
					Bootstrap: clusterv1.Bootstrap{
						DataSecretName: pointer.String("test-bootstrap-data"),
					},
				},
			}
			Expect(k8sClient.Create(ctx, hnMachine)).To(Succeed())

			By("Checking if the HnMachine was successfully created")
			createdHnMachine := &hnv1alpha1.HnMachine{}
			Eventually(func() error {
				return k8sClient.Get(ctx, typeNamespacedName, createdHnMachine)
			}, timeout, interval).Should(Succeed())
			fmt.Printf("Created HnMachine: %+v\n", createdHnMachine)

			By("Reconciling the created resource")
			hnMachineReconciler := &HnMachineReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			result, err := hnMachineReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			fmt.Printf("Reconcile result: %+v\n", result)

			By("Checking the state of the HnMachine after reconciliation")
			reconciledHnMachine := &hnv1alpha1.HnMachine{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, reconciledHnMachine)).To(Succeed())
			fmt.Printf("Reconciled HnMachine: %+v\n", reconciledHnMachine)

			By("Checking if the Pod was created")
			pod := &corev1.Pod{}
			Eventually(func() error {
				err := k8sClient.Get(ctx, types.NamespacedName{Name: resourceName, Namespace: resourceNamespace}, pod)
				if err != nil {
					fmt.Printf("Error getting Pod: %v\n", err)
					return err
				}
				return nil
			}, timeout, interval).Should(Succeed())

			fmt.Printf("Created Pod: %+v\n", pod)

			Expect(pod.Spec.Containers).To(HaveLen(1))
			Expect(pod.Spec.Containers[0].Image).To(Equal("test-image"))
			Expect(pod.Spec.Containers[0].Resources.Requests.Cpu().String()).To(Equal("1"))
			Expect(pod.Spec.Containers[0].Resources.Requests.Memory().String()).To(Equal("1Gi"))

			By("Checking if the HnMachine status was updated")
			updatedHnMachine := &hnv1alpha1.HnMachine{}
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, typeNamespacedName, updatedHnMachine); err != nil {
					fmt.Printf("Error getting updated HnMachine: %v\n", err)
					return false
				}
				fmt.Printf("Updated HnMachine: %+v\n", updatedHnMachine)
				return updatedHnMachine.Status.Ready
			}, timeout, interval).Should(BeTrue())

			Expect(updatedHnMachine.Status.Addresses).To(HaveLen(2))
			Expect(updatedHnMachine.Status.Addresses[0].Type).To(Equal(clusterv1.MachineHostName))
			Expect(updatedHnMachine.Status.Addresses[1].Type).To(Equal(clusterv1.MachineInternalIP))
		})
	})
})

// Helper function to get a condition from the HnMachine status
func getCondition(hnMachine *hnv1alpha1.HnMachine, conditionType clusterv1.ConditionType) *clusterv1.Condition {
	for _, condition := range hnMachine.Status.Conditions {
		if condition.Type == conditionType {
			return &condition
		}
	}
	return nil
}

// Helper function to clean up resources
func cleanupResources(ctx context.Context) {
	// Delete all HnMachines
	fmt.Println("Deleting all HnMachines...")
	err := k8sClient.DeleteAllOf(ctx, &hnv1alpha1.HnMachine{}, client.InNamespace("default"))
	if err != nil && !errors.IsNotFound(err) {
		fmt.Printf("Error deleting HnMachines: %v\n", err)
	}

	// Delete all Clusters
	fmt.Println("Deleting all Clusters...")
	err = k8sClient.DeleteAllOf(ctx, &clusterv1.Cluster{}, client.InNamespace("default"))
	if err != nil && !errors.IsNotFound(err) {
		fmt.Printf("Error deleting Clusters: %v\n", err)
	}

	// Delete all Machines
	fmt.Println("Deleting all Machines...")
	err = k8sClient.DeleteAllOf(ctx, &clusterv1.Machine{}, client.InNamespace("default"))
	if err != nil && !errors.IsNotFound(err) {
		fmt.Printf("Error deleting Machines: %v\n", err)
	}

	// Delete all Secrets
	fmt.Println("Deleting all Secrets...")
	err = k8sClient.DeleteAllOf(ctx, &corev1.Secret{}, client.InNamespace("default"))
	if err != nil && !errors.IsNotFound(err) {
		fmt.Printf("Error deleting Secrets: %v\n", err)
	}

	// Delete all Pods
	fmt.Println("Deleting all Pods...")
	err = k8sClient.DeleteAllOf(ctx, &corev1.Pod{}, client.InNamespace("default"))
	if err != nil && !errors.IsNotFound(err) {
		fmt.Printf("Error deleting Pods: %v\n", err)
	}

	// Wait for resources to be deleted
	fmt.Println("Waiting for resources to be deleted...")
	Eventually(func() bool {
		return noResourcesExist(ctx)
	}, time.Second*30, time.Second).Should(BeTrue())
	fmt.Println("All resources deleted")
}

// Helper function to check if resources exist
func noResourcesExist(ctx context.Context) bool {
	hnMachines := &hnv1alpha1.HnMachineList{}
	clusters := &clusterv1.ClusterList{}
	machines := &clusterv1.MachineList{}
	secrets := &corev1.SecretList{}
	pods := &corev1.PodList{}

	if err := k8sClient.List(ctx, hnMachines, client.InNamespace("default")); err != nil {
		if !errors.IsNotFound(err) {
			fmt.Printf("Error listing HnMachines: %v\n", err)
			return false
		}
	} else if len(hnMachines.Items) > 0 {
		fmt.Printf("HnMachines still exist: %d\n", len(hnMachines.Items))
		return false
	}

	if err := k8sClient.List(ctx, clusters, client.InNamespace("default")); err != nil {
		if !errors.IsNotFound(err) {
			fmt.Printf("Error listing Clusters: %v\n", err)
			return false
		}
	} else if len(clusters.Items) > 0 {
		fmt.Printf("Clusters still exist: %d\n", len(clusters.Items))
		return false
	}

	if err := k8sClient.List(ctx, machines, client.InNamespace("default")); err != nil {
		if !errors.IsNotFound(err) {
			fmt.Printf("Error listing Machines: %v\n", err)
			return false
		}
	} else if len(machines.Items) > 0 {
		fmt.Printf("Machines still exist: %d\n", len(machines.Items))
		return false
	}

	if err := k8sClient.List(ctx, secrets, client.InNamespace("default")); err != nil {
		if !errors.IsNotFound(err) {
			fmt.Printf("Error listing Secrets: %v\n", err)
			return false
		}
	} else if len(secrets.Items) > 0 {
		fmt.Printf("Secrets still exist: %d\n", len(secrets.Items))
		return false
	}

	if err := k8sClient.List(ctx, pods, client.InNamespace("default")); err != nil {
		if !errors.IsNotFound(err) {
			fmt.Printf("Error listing Pods: %v\n", err)
			return false
		}
	} else if len(pods.Items) > 0 {
		fmt.Printf("Pods still exist: %d\n", len(pods.Items))
		return false
	}

	return true
}
