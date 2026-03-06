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
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"runtime"
	"testing"

	esov1 "github.com/external-secrets/external-secrets/apis/externalsecrets/v1"
	reloaderv1aplha1 "github.com/external-secrets/reloader/api/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	// +kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var cfg *rest.Config
var k8sClient client.Client
var testEnv *envtest.Environment
var ctx context.Context
var cancel context.CancelFunc

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	ctx, cancel = context.WithCancel(context.TODO())

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,

		// The BinaryAssetsDirectory is only required if you want to run the tests directly
		// without call the makefile target test. If not informed it will look for the
		// default path defined in controller-runtime which is /usr/local/kubebuilder/.
		// Note that you must have the required binaries setup under the bin directory to perform
		// the tests directly. When we run make test it will be setup and used automatically.
		BinaryAssetsDirectory: filepath.Join("..", "..", "bin", "k8s",
			fmt.Sprintf("1.31.0-%s-%s", runtime.GOOS, runtime.GOARCH)),
	}

	var err error
	// cfg is defined in this file globally.
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = esov1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = reloaderv1aplha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	// +kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	// Deploy External Secrets CRDs first
	crdURL := "https://raw.githubusercontent.com/external-secrets/external-secrets/v0.5.7/deploy/crds/bundle.yaml"
	crdBytes, err := fetchCRDBundle(crdURL)
	Expect(err).NotTo(HaveOccurred())
	err = applyCRDs(ctx, crdBytes)
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	cancel()
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})

// fetchCRDBundle fetches the CRD bundle from the specified URL and returns its content as a byte slice.
func fetchCRDBundle(url string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("received non-200 response: %v", resp.Status)
	}

	return io.ReadAll(resp.Body)
}

// applyCRDs decodes, creates, or updates Custom Resource Definitions (CRDs) described in the provided crdBytes.
func applyCRDs(ctx context.Context, crdBytes []byte) error {
	l := logf.FromContext(ctx)
	crdDecoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(crdBytes), 4096)

	for {
		obj := &unstructured.Unstructured{}
		if err := crdDecoder.Decode(obj); err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("failed to decode CRD: %w", err)
		}

		// Ensure that the CRD is applied correctly, even if it already exists
		createOrUpdateFunc := func() error {
			return client.IgnoreNotFound(k8sClient.Get(ctx, client.ObjectKeyFromObject(obj), obj))
		}

		_, err := controllerutil.CreateOrUpdate(ctx, k8sClient, obj, createOrUpdateFunc)
		if err != nil {
			l.Error(err, "failed to create or update CRD", "CRD.Name", obj.GetName())
			return fmt.Errorf("failed to create or update CRD %s: %w", obj.GetName(), err)
		}
		l.Info("CRD applied", "CRD.Name", obj.GetName())
	}

	return nil
}
