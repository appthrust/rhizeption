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
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	hnv1alpha1 "github.com/appthrust/hosted-node/api/v1alpha1"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	// +kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var cfg *rest.Config
var k8sClient client.Client
var testEnv *envtest.Environment

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	By("bootstrapping test environment")
	crdPath := filepath.Join("..", "..", "config", "crd", "bases")
	clusterAPIPath := getClusterAPIPath()

	// Print out the contents of the CRD directory
	fmt.Println("Contents of CRD directory:")
	files, err := ioutil.ReadDir(crdPath)
	Expect(err).NotTo(HaveOccurred())
	for _, file := range files {
		fmt.Println(file.Name())
	}

	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			crdPath,
			filepath.Join(clusterAPIPath, "config", "crd", "bases"),
		},
		ErrorIfCRDPathMissing: true,

		// The BinaryAssetsDirectory is only required if you want to run the tests directly
		// without call the makefile target test. If not informed it will look for the
		// default path defined in controller-runtime which is /usr/local/kubebuilder/.
		// Note that you must have the required binaries setup under the bin directory to perform
		// the tests directly. When we run make test it will be setup and used automatically.
		BinaryAssetsDirectory: filepath.Join("..", "..", "bin", "k8s",
			fmt.Sprintf("1.30.0-%s-%s", runtime.GOOS, runtime.GOARCH)),
	}

	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = hnv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = clusterv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = apiextensionsv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	// +kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	// Check if the CRD already exists
	existingCRD := &apiextensionsv1.CustomResourceDefinition{}
	err = k8sClient.Get(context.Background(), client.ObjectKey{Name: "hnmachines.hn.appthrust.io"}, existingCRD)
	if err != nil && !errors.IsNotFound(err) {
		Expect(err).NotTo(HaveOccurred())
	}

	if errors.IsNotFound(err) {
		fmt.Println("CRD not found, creating it...")
		// Create the CRD if it doesn't exist
		crd := &apiextensionsv1.CustomResourceDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name: "hnmachines.hn.appthrust.io",
			},
			Spec: apiextensionsv1.CustomResourceDefinitionSpec{
				Group: "hn.appthrust.io",
				Names: apiextensionsv1.CustomResourceDefinitionNames{
					Kind:     "HnMachine",
					ListKind: "HnMachineList",
					Plural:   "hnmachines",
					Singular: "hnmachine",
				},
				Scope: apiextensionsv1.NamespaceScoped,
				Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
					{
						Name:    "v1alpha1",
						Served:  true,
						Storage: true,
						Schema: &apiextensionsv1.CustomResourceValidation{
							OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
								Type: "object",
								Properties: map[string]apiextensionsv1.JSONSchemaProps{
									"spec": {
										Type: "object",
										Properties: map[string]apiextensionsv1.JSONSchemaProps{
											"providerID": {Type: "string"},
											"template": {
												Type: "object",
												Properties: map[string]apiextensionsv1.JSONSchemaProps{
													"image": {Type: "string"},
												},
												Required: []string{"image"},
											},
										},
										Required: []string{"template"},
									},
									"status": {
										Type: "object",
										Properties: map[string]apiextensionsv1.JSONSchemaProps{
											"ready": {Type: "boolean"},
										},
									},
								},
							},
						},
					},
				},
			},
		}

		err = k8sClient.Create(context.Background(), crd)
		Expect(err).NotTo(HaveOccurred())
	} else {
		fmt.Println("CRD already exists")
	}

	// Wait for the CRD to be ready
	fmt.Println("Waiting for CRD to be ready...")
	err = wait.PollImmediate(time.Second, time.Minute, func() (bool, error) {
		var createdCRD apiextensionsv1.CustomResourceDefinition
		if err := k8sClient.Get(context.Background(), client.ObjectKey{Name: "hnmachines.hn.appthrust.io"}, &createdCRD); err != nil {
			if errors.IsNotFound(err) {
				fmt.Println("CRD not found during wait")
				return false, nil
			}
			fmt.Printf("Error getting CRD: %v\n", err)
			return false, err
		}

		for _, condition := range createdCRD.Status.Conditions {
			if condition.Type == apiextensionsv1.Established &&
				condition.Status == apiextensionsv1.ConditionTrue {
				fmt.Println("CRD is ready")
				return true, nil
			}
		}

		fmt.Println("CRD not yet ready")
		return false, nil
	})
	Expect(err).NotTo(HaveOccurred())

	fmt.Println("Test environment setup completed")
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})

// getClusterAPIPath returns the path to the Cluster API package
func getClusterAPIPath() string {
	cmd := exec.Command("go", "list", "-f", "{{.Dir}}", "sigs.k8s.io/cluster-api")
	output, err := cmd.Output()
	if err != nil {
		panic(fmt.Sprintf("Failed to get Cluster API path: %v", err))
	}
	return strings.TrimSpace(string(output))
}
