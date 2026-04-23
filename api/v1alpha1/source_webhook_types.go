package v1alpha1

// WebhookConfig contains configuration for Webhook notifications.
type WebhookConfig struct {
	// SecretIdentifierOnPayload is the key that the reloader will look for in the payload.
	// The value of this key should be the same name as in the external secret. It will default to `0.data.ObjectName` if not set
	// +optional
	SecretIdentifierOnPayload string `json:"identifierPathOnPayload,omitempty"`

	// Auth is the authentication method for the webhook
	// +optional
	Auth *WebhookAuth `json:"webhookAuth,omitempty"`

	// RetryPolicy represents the policy to retry when a message fails.
	// If it's empty, reloader will return a 4xx and won't retry.
	// +optional
	RetryPolicy *RetryPolicy `json:"retryPolicy,omitempty"`
}

type RetryPolicy struct {
	// MaxRetries represents the maximum times the reloader should retry to process a message. Numbers greater than 10 will be ignored and 10 will be used instead
	// +optional
	MaxRetries int `json:"maxRetries"`

	// Algorithm represents how watiting time will change for each retry.
	// Currently supports "linear" and "exponential". If an invalid string or null is given, "exponential" will be used
	// +optional
	Algorithm string `json:"algorithm"`
}

// WebhookAuth contains authentication methods for webhooks.
type WebhookAuth struct {
	// BasicAuth contains basic authentication credentials.
	// +optional
	BasicAuth *BasicAuth `json:"basicAuth,omitempty"`

	// BearerToken references a Kubernetes Secret containing the bearer token.
	// +optional
	BearerToken *BearerToken `json:"bearerToken,omitempty"`
}

// BasicAuth contains basic authentication credentials.
type BasicAuth struct {
	// UsernameSecretRef contains a secret reference for the username
	// +required
	UsernameSecretRef SecretKeySelector `json:"usernameSecretRef,omitempty"`

	// PasswordSecretRef contains a secret reference for the password
	// +required
	PasswordSecretRef SecretKeySelector `json:"passwordSecretRef,omitempty"`
}

// BearerToken contains the bearer token credentials.
type BearerToken struct {
	// BearerTokenSecretRef references a Kubernetes Secret containing the bearer token.
	// +required
	BearerTokenSecretRef SecretKeySelector `json:"bearerTokenSecretRef"`
}
