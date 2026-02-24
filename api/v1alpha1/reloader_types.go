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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ConfigSpec defines the desired state of a Reloader Config
type ConfigSpec struct {
	// NotificationSources specifies the notification systems to listen to.
	// +required
	NotificationSources []NotificationSource `json:"notificationSources"`

	// DestinationsToWatch specifies which secrets the controller should monitor.
	// +required
	DestinationsToWatch []DestinationToWatch `json:"destinationsToWatch"`
}

// NotificationSource represents a notification system configuration.
type NotificationSource struct {
	// Type of the notification source (e.g., AwsSqs, AzureEventGrid, GooglePubSub, HashicorpVault, Webhook, TCPSocket, KubernetesSecret, KubernetesConfigMap).
	// +kubebuilder:validation:Enum=AwsSqs;AzureEventGrid;GooglePubSub;HashicorpVault;Webhook;TCPSocket;KubernetesSecret;KubernetesConfigMap
	// +required
	Type string `json:"type"`

	// AwsSqs configuration (required if Type is AwsSqs).
	// +optional
	AwsSqs *AWSSQSConfig `json:"awsSqs,omitempty"`

	AzureEventGrid *AzureEventGridConfig `json:"azureEventGrid,omitempty"`

	// GooglePubSub configuration (required if Type is GooglePubSub).
	// +optional
	GooglePubSub *GooglePubSubConfig `json:"googlePubSub,omitempty"`

	// Webhook configuration (required if Type is Webhook).
	// +optional
	Webhook *WebhookConfig `json:"webhook,omitempty"`

	// HashicorpVault configuration (required if Type is HashicorpVault).
	// +optional
	HashicorpVault *HashicorpVaultConfig `json:"hashicorpVault,omitempty"`

	// Kubernetes Secret watch configuration (required if Type is KubernetesSecret).
	// +optional
	KubernetesSecret *KubernetesSecretConfig `json:"kubernetesSecret,omitempty"`

	// Kubernetes ConfigMap watch configuration (required if Type is KubernetesConfigMap).
	// +optional
	KubernetesConfigMap *KubernetesConfigMapConfig `json:"kubernetesConfigMap,omitempty"`

	// TCPSocket configuration (required if Type is TCPSocket).
	// +optional
	TCPSocket *TCPSocketConfig `json:"tcpSocket,omitempty"`

	// Mock configuration (optional field for testing purposes).
	Mock *MockConfig `json:"mock,omitempty"`
}

// DestinationToWatch specifies the criteria for monitoring secrets in the cluster.
type DestinationToWatch struct {
	// Type specifies the type of destination to watch.
	// +required
	// +kubebuilder:validation:Enum=ExternalSecret;Deployment;PushSecret;WorkflowRunTemplate
	Type string `json:"type"`
	// +optional
	WorkflowRunTemplate *WorkflowRunTemplateDestination `json:"workflowRunTemplate,omitempty"`
	// +optional
	ExternalSecret *ExternalSecretDestination `json:"externalSecret,omitempty"`
	// +optional
	PushSecret *PushSecretDestination `json:"pushSecret,omitempty"`
	// +optional
	Deployment *DeploymentDestination `json:"deployment,omitempty"`
	//UpdateStrategy. If not specified, will use each destinations' default update strategy.
	UpdateStrategy *UpdateStrategy `json:"updateStrategy,omitempty"`
	//MatchStrategy. If not specified, will use each destinations' default match strategy.
	// +optional
	MatchStrategy *MatchStrategy `json:"matchStrategy,omitempty"`
	//WaitStrategy. If not specified, will use each destinations's default wait strategy.
	// +optional
	WaitStrategy *WaitStrategy `json:"waitStrategy,omitempty"`
}

// ConfigStatus defines the observed state of Reloader
type ConfigStatus struct {
	// Conditions represent the latest available observations of the resource's state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:subresource:status

// Config is the Schema for the reloader config API
type Config struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ConfigSpec   `json:"spec,omitempty"`
	Status ConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ConfigList contains a list of Config
type ConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Config `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Config{}, &ConfigList{})
}
