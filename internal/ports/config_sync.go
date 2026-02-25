package ports

import "context"

// RemoteConfig represents the configuration fetched from the Cloud API Gateway.
type RemoteConfig struct {
	Rules        []Rule
	Models       []Model
	Context      string
	Capabilities []Capability
}

type Rule struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Action      string `json:"action"` // e.g., "allow", "deny", "require_approval"
}

type Model struct {
	ID       string `json:"id"`
	Provider string `json:"provider"`
	Name     string `json:"name"`
	APIKey   string `json:"api_key,omitempty"` // Usually omitted or masked
}

type Capability struct {
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	SystemPrompt string   `json:"system_prompt"`
	AllowedTools []string `json:"allowed_tools"`
}

// ConfigSyncPort defines the interface for fetching dynamic configuration from the cloud.
type ConfigSyncPort interface {
	FetchRemoteConfig(ctx context.Context) (*RemoteConfig, error)
}
