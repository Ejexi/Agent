package domain

// ProviderConfig represents the configuration for an AI provider.
// This is a pure domain type used by adapters to retrieve provider credentials
// without depending on the infrastructure config package.
type ProviderConfig struct {
	APIKey  string
	Model   string
	BaseURL string
}
