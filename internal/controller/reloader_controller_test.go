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

	esv1 "github.com/external-secrets/external-secrets/apis/externalsecrets/v1"
	"github.com/external-secrets/reloader/internal/events"
	"github.com/external-secrets/reloader/internal/handler"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	esov1 "github.com/external-secrets/reloader/api/v1alpha1"
	"github.com/external-secrets/reloader/internal/listener"
)

var _ = Describe("Reloader Controller", func() {
	var (
		ctx            context.Context
		scheme         *runtime.Scheme
		fakeClient     client.Client
		reconciler     *ReloaderReconciler
		config         *esov1.Config
		externalSecret *esv1.ExternalSecret
		eventChan      chan events.SecretRotationEvent
	)

	BeforeEach(func() {
		ctx = context.Background()
		scheme = runtime.NewScheme()
		Expect(esov1.AddToScheme(scheme)).To(Succeed())
		Expect(esv1.AddToScheme(scheme)).To(Succeed())

		fakeClient = fake.NewClientBuilder().WithScheme(scheme).Build()

		eventChan = make(chan events.SecretRotationEvent, 10)
		manager := listener.NewListenerManager(ctx, eventChan, fakeClient, log.FromContext(ctx))
		eventHandler := handler.NewEventHandler(fakeClient)

		reconciler = &ReloaderReconciler{
			Client:          fakeClient,
			Scheme:          scheme,
			listenerManager: manager,
			eventChan:       eventChan,
			eventHandler:    eventHandler,
		}

		go reconciler.processEvents(ctx)

		config = &esov1.Config{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-reloader",
				Namespace: "default",
			},
			Spec: esov1.ConfigSpec{
				NotificationSources: []esov1.NotificationSource{
					{
						Type: "Mock",
						Mock: &esov1.MockConfig{EmitInterval: 1000},
					},
				},
				DestinationsToWatch: []esov1.DestinationToWatch{
					{
						Type: "ExternalSecret",
						ExternalSecret: &esov1.ExternalSecretDestination{
							Names: []string{
								"test-external-secret-data",
								"test-external-secret-datafrom-extract",
								"test-external-secret-datafrom-find",
							},
						},
					},
				},
			},
		}
		Expect(fakeClient.Create(context.Background(), config)).To(Succeed())

		// Reconcile the controller to set up the listeners
		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      config.Name,
				Namespace: config.Namespace,
			},
		})
		Expect(err).NotTo(HaveOccurred())
		// Reconcile the controller to set up the listeners
		_, err = reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      config.Name,
				Namespace: config.Namespace,
			},
		})
		Expect(err).NotTo(HaveOccurred())
	})

	Context("When a config is created/updated/deleted", func() {
		It("should add the processed annotation and push a Created/Updated/Deleted event for Reloader", func() {
			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      config.Name,
					Namespace: config.Namespace,
				},
			}

			// Get update reloader to check ProcessedAnnotation
			updatedconfig := &esov1.Config{}
			Expect(fakeClient.Get(ctx, req.NamespacedName, updatedconfig)).To(Succeed())
			Expect(updatedconfig.Annotations).To(HaveKey(ProcessedAnnotation))

			// Call reconcile to generate updated event
			_, err := reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())

			// Deleting reloader manifest to generate deleted event
			Expect(fakeClient.Delete(context.Background(), config)).To(Succeed())

			_, err = reconciler.Reconcile(ctx, req)
			Expect(err).NotTo(HaveOccurred())

		})
	})

	Context("When a secret rotation event is received, and the secret is not watched", func() {
		It("should not annotate any event out of the secrets to watch list", func() {
			// Create an ExternalSecret that references the secret not watched
			externalSecret = &esv1.ExternalSecret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-not-watched",
					Namespace: "default",
				},
				Spec: esv1.ExternalSecretSpec{
					SecretStoreRef: esv1.SecretStoreRef{
						Name: "my-secret-store",
						Kind: "SecretStore",
					},
					Data: []esv1.ExternalSecretData{
						{
							SecretKey: "password",
							RemoteRef: esv1.ExternalSecretDataRemoteRef{
								Key: "aws://secret/arn:aws:secretsmanager:us-east-1:123456789012:secret:mysecret",
							},
						},
					},
				},
			}

			Expect(fakeClient.Create(context.Background(), externalSecret)).To(Succeed())
			assertNotWatchedAnnotations(fakeClient, "test-not-watched")
		})
	})

	Context("When a secret rotation event is received", func() {
		It("should annotate the corresponding ExternalSecret using data field", func() {
			// Create an ExternalSecret that references the secret via data field
			externalSecret = &esv1.ExternalSecret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-external-secret-data",
					Namespace: "default",
				},
				Spec: esv1.ExternalSecretSpec{
					SecretStoreRef: esv1.SecretStoreRef{
						Name: "my-secret-store",
						Kind: "SecretStore",
					},
					Data: []esv1.ExternalSecretData{
						{
							SecretKey: "password",
							RemoteRef: esv1.ExternalSecretDataRemoteRef{
								Key: "aws://secret/arn:aws:secretsmanager:us-east-1:123456789012:secret:mysecret",
							},
						},
					},
				},
			}

			Expect(fakeClient.Create(context.Background(), externalSecret)).To(Succeed())
			assertAnnotations(fakeClient, "test-external-secret-data")
		})
	})

	Context("When a secret rotation event is received and ExternalSecret uses dataFrom.extract", func() {
		It("should annotate the corresponding ExternalSecret using dataFrom.extract", func() {
			secretIdentifier := "aws://secret/arn:aws:secretsmanager:us-east-1:123456789012:secret:mysecret"

			// Create an ExternalSecret that references the secret via dataFrom.extract
			externalSecret = &esv1.ExternalSecret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-external-secret-datafrom-extract",
					Namespace: "default",
				},
				Spec: esv1.ExternalSecretSpec{
					SecretStoreRef: esv1.SecretStoreRef{
						Name: "my-secret-store",
						Kind: "SecretStore",
					},
					DataFrom: []esv1.ExternalSecretDataFromRemoteRef{
						{
							Extract: &esv1.ExternalSecretDataRemoteRef{
								Key: secretIdentifier,
							},
						},
					},
				},
			}

			Expect(fakeClient.Create(context.Background(), externalSecret)).To(Succeed())
			assertAnnotations(fakeClient, "test-external-secret-datafrom-extract")
		})
	})

	Context("When a secret rotation event is received and ExternalSecret uses dataFrom.find", func() {
		It("should annotate the corresponding ExternalSecret using dataFrom.find", func() {
			secretIdentifier := "aws://secret/arn:aws:secretsmanager:us-east-1:123456789012:secret:mysecret"

			// Create an ExternalSecret that references the secret via dataFrom.find
			externalSecret = &esv1.ExternalSecret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-external-secret-datafrom-find",
					Namespace: "default",
				},
				Spec: esv1.ExternalSecretSpec{
					SecretStoreRef: esv1.SecretStoreRef{
						Name: "my-secret-store",
						Kind: "SecretStore",
					},
					DataFrom: []esv1.ExternalSecretDataFromRemoteRef{
						{
							Find: &esv1.ExternalSecretFind{
								Name: &esv1.FindName{
									RegExp: secretIdentifier,
								},
							},
						},
					},
				},
			}

			Expect(fakeClient.Create(context.Background(), externalSecret)).To(Succeed())
			assertAnnotations(fakeClient, "test-external-secret-datafrom-find")
		})
	})

	Context("When a secret rotation event is received and ExternalSecret uses target.template.templateFrom.configMap", func() {
		It("should annotate the ExternalSecret when templateFrom.configMap.name matches the event secret identifier", func() {
			configMapName := "operator-config"
			esName := "test-external-secret-templatefrom"

			// Update config to watch this ExternalSecret by name
			updatedConfig := &esov1.Config{}
			Expect(fakeClient.Get(ctx, types.NamespacedName{Name: config.Name, Namespace: config.Namespace}, updatedConfig)).To(Succeed())
			updatedConfig.Spec.DestinationsToWatch[0].ExternalSecret.Names = append(
				updatedConfig.Spec.DestinationsToWatch[0].ExternalSecret.Names,
				esName,
			)
			Expect(fakeClient.Update(ctx, updatedConfig)).To(Succeed())
			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: config.Name, Namespace: config.Namespace},
			})
			Expect(err).NotTo(HaveOccurred())

			// Create ExternalSecret that references the ConfigMap via templateFrom (e.g. ConfigMap-triggered event)
			externalSecret = &esv1.ExternalSecret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      esName,
					Namespace: "default",
				},
				Spec: esv1.ExternalSecretSpec{
					Target: esv1.ExternalSecretTarget{
						Template: &esv1.ExternalSecretTemplate{
							TemplateFrom: []esv1.TemplateFrom{
								{
									ConfigMap: &esv1.TemplateRef{
										Name:  configMapName,
										Items: []esv1.TemplateRefItem{{Key: "config", TemplateAs: esv1.TemplateScopeValues}},
									},
								},
							},
						},
					},
				},
			}
			Expect(fakeClient.Create(context.Background(), externalSecret)).To(Succeed())

			// Send event with ConfigMap name as identifier (as would happen for KubernetesConfigMap source)
			eventChan <- events.SecretRotationEvent{
				SecretIdentifier:  configMapName,
				RotationTimestamp: "2024-09-19T12:00:00Z",
				TriggerSource:     "KubernetesConfigMap",
			}

			assertAnnotationsWithSource(fakeClient, esName, "2024-09-19T12:00:00Z", "KubernetesConfigMap")
		})
	})
})

func assertAnnotations(fakeClient client.Client, secretName string) {
	updatedES := &esv1.ExternalSecret{}
	key := types.NamespacedName{
		Namespace: "default",
		Name:      secretName,
	}
	// Wait for the controller to process the event by polling
	Eventually(func() error {
		updatedES = &esv1.ExternalSecret{}
		err := fakeClient.Get(context.Background(), key, updatedES)
		if err != nil {
			return err
		}
		annotations := updatedES.GetAnnotations()
		if annotations == nil {
			return fmt.Errorf("ExternalSecret annotations should not be nil")
		}
		if annotations["reloader/last-rotated"] != "2024-09-19T12:00:00Z" {
			return fmt.Errorf("reloader/last-rotated annotation should be set to 2024-09-19T12:00:00Z")
		}
		if annotations["reloader/trigger-source"] != "aws-secretsmanager" {
			return fmt.Errorf("reloader/trigger-source annotation should be set to aws-secretsmanager")
		}
		return nil
	}, "5s", "500ms").Should(Succeed())
}

func assertNotWatchedAnnotations(fakeClient client.Client, secretName string) {
	updatedES := &esv1.ExternalSecret{}
	key := types.NamespacedName{
		Namespace: "default",
		Name:      secretName,
	}
	// Wait for the controller to process the event by polling
	Eventually(func() error {
		updatedES = &esv1.ExternalSecret{}
		err := fakeClient.Get(context.Background(), key, updatedES)
		if err != nil {
			return err
		}
		annotations := updatedES.GetAnnotations()
		if annotations != nil {
			return fmt.Errorf("ExternalSecret annotations should not be nil")
		}
		return nil
	}, "5s", "500ms").Should(Succeed())
}

func assertAnnotationsWithSource(fakeClient client.Client, secretName, expectedTimestamp, expectedTriggerSource string) {
	updatedES := &esv1.ExternalSecret{}
	key := types.NamespacedName{
		Namespace: "default",
		Name:      secretName,
	}
	Eventually(func() error {
		updatedES = &esv1.ExternalSecret{}
		err := fakeClient.Get(context.Background(), key, updatedES)
		if err != nil {
			return err
		}
		annotations := updatedES.GetAnnotations()
		if annotations == nil {
			return fmt.Errorf("ExternalSecret annotations should not be nil")
		}
		if annotations["reloader/last-rotated"] != expectedTimestamp {
			return fmt.Errorf("reloader/last-rotated annotation should be %q, got %q", expectedTimestamp, annotations["reloader/last-rotated"])
		}
		if annotations["reloader/trigger-source"] != expectedTriggerSource {
			return fmt.Errorf("reloader/trigger-source annotation should be %q, got %q", expectedTriggerSource, annotations["reloader/trigger-source"])
		}
		return nil
	}, "5s", "500ms").Should(Succeed())
}
