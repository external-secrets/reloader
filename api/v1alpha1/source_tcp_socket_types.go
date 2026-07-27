package v1alpha1

// TCPSocketConfig contains configuration for TCP Socket notifications.
type TCPSocketConfig struct {
	// Host is the hostname or IP address to listen on.
	// +required
	Host string `json:"host"`

	// Port is the port number to listen on.
	// +required
	// +kubebuilder:default=8000
	Port int32 `json:"port"`

	// SecretIdentifierOnPayload is the key that the reloader will look for in the payload.
	// The value of this key should be the same name as in the external secret. It will default to `0.data.ObjectName` if not set
	SecretIdentifierOnPayload string `json:"identifierPathOnPayload,omitempty"`
}
