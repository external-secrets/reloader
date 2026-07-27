package v1alpha1

// HashicorpVault contains configuration for HashicorpVault notifications.
type HashicorpVaultConfig struct {
	// Host is the hostname or IP address to listen on.
	// +required
	Host string `json:"host"`

	// Port is the port number to listen on.
	// +required
	// +kubebuilder:default=8000
	Port int32 `json:"port"`
}
