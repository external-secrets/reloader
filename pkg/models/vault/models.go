package vault

import "time"

func ValidMessage(m *AuditLog) bool {
	return m.AuthType == "response" && m.AuthResponse != nil && m.AuthResponse.MountType == "kv"
}

type AuditLog struct {
	Auth         *Auth         `json:"auth,omitempty"`
	AuthRequest  *AuthRequest  `json:"request,omitempty"`
	AuthResponse *AuthResponse `json:"response,omitempty"`
	Time         time.Time     `json:"time,omitempty"`
	AuthType     string        `json:"type,omitempty"`
}

type Auth struct {
	Accessor       string         `json:"accessor,omitempty"`
	DisplayName    string         `json:"display_name,omitempty"`
	Policies       []string       `json:"policies,omitempty"`
	PolicyResults  *PolicyResults `json:"policy_results,omitempty"`
	ClientToken    string         `json:"client_token,omitempty"`
	TokenPolicies  []string       `json:"token_policies,omitempty"`
	TokenIssueTime time.Time      `json:"token_issue_time,omitempty"`
	TokenType      string         `json:"token_type,omitempty"`
}

type PolicyResults struct {
	Allowed          bool             `json:"allowed,omitempty"`
	GrantingPolicies []GrantingPolicy `json:"granting_policies,omitempty"`
}

type GrantingPolicy struct {
	Name        string `json:"name,omitempty"`
	NamespaceId string `json:"namespace_id,omitempty"`
	Type        string `json:"type,omitempty"`
}

type AuthRequest struct {
	ClientId            string                 `json:"client_id,omitempty"`
	ClientToken         string                 `json:"client_token,omitempty"`
	ClientTokenAccessor string                 `json:"client_token_accessor,omitempty"`
	Data                map[string]interface{} `json:"data,omitempty"`
	Id                  string                 `json:"id,omitempty"`
	MountAccessor       string                 `json:"mount_accessor,omitempty"`
	MountClass          string                 `json:"mount_class,omitempty"`
	MountPoint          string                 `json:"mount_point,omitempty"`
	MountRunningVersion string                 `json:"mount_running_version,omitempty"`
	MountType           string                 `json:"mount_type,omitempty"`
	Namespace           *Namespace             `json:"namespace,omitempty"`
	Operation           string                 `json:"operation,omitempty"`
	Path                string                 `json:"path,omitempty"`
	RemoteAddress       string                 `json:"remote_address,omitempty"`
	RemotePort          int                    `json:"remote_port,omitempty"`
	RequestUri          string                 `json:"request_uri,omitempty"`
}

type Namespace struct {
	Id string `json:"id,omitempty"`
}

type AuthResponse struct {
	Data                      map[string]interface{} `json:"data,omitempty"`
	MountAccessor             string                 `json:"mount_accessor,omitempty"`
	MountClass                string                 `json:"mount_class,omitempty"`
	MountPoint                string                 `json:"mount_point,omitempty"`
	MountRunningPluginVersion string                 `json:"mount_running_plugin_version,omitempty"`
	MountType                 string                 `json:"mount_type,omitempty"`
}
