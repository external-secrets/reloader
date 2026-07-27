package v1alpha1

type AzureEventGridConfig struct {
	Host string `json:"host"`

	// +required
	// +kubebuilder:default=8080
	Port int32 `json:"port"`

	Subscriptions []string `json:"subscriptions"`
}
