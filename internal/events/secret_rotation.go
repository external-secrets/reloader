package events

// SecretRotationEvent represents an event triggered during the secret rotation process.
// It contains the secret identifier, the timestamp of the rotation, and the source that triggered the event.
type SecretRotationEvent struct {
	SecretIdentifier  string
	RotationTimestamp string
	TriggerSource     string
	// Optional bit so we can filter down better depending on the namespace.
	Namespace string
}
