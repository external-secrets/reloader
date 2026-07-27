package gcp

type AuditLogMessage struct {
	ProtoPayload     AuditLog `json:"protoPayload"`
	Resource         Resource `json:"resource"`
	Timestamp        string   `json:"timestamp"`
	ReiveceTimestamp string   `json:"receiveTimestamp"`
}
type AuditLog struct {
	AuthenticationInfo AuthenticationInfo `json:"authenticationInfo"`
	MethodName         string             `json:"methodName"`
	RequestMetadata    RequestMetadata    `json:"requestMetadata"`
	ResourceName       string             `json:"resourceName"`
	ServiceName        string             `json:"serviceName"`
}

type AuthenticationInfo struct {
	PrincipalEmail string `json:"principalEmail"`
}
type RequestMetadata struct {
	CallerIp       string `json:"callerIp"`
	CallerSupplied string `json:"callerSuppliedUserAgent"`
}
type Resource struct {
	Labels map[string]string `json:"labels"`
	Type   string            `json:"type"`
}
