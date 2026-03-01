package config

import (
	"os"
	"path/filepath"

	"github.com/SecDuckOps/shared/types"
	"github.com/pelletier/go-toml/v2"
)

// ========================
// TOML Config Structures
// ========================

// DuckOpsConfig is the top-level config loaded from ~/.duckops/config.toml.
type DuckOpsConfig struct {
	Profiles map[string]Profile `toml:"profiles"`
	Settings Settings           `toml:"settings"`
}

// Profile represents a named configuration profile (e.g., "Default", "Super Mode").
type Profile struct {
	APIEndpoint  string              `toml:"api_endpoint,omitempty"`
	Provider     string              `toml:"provider,omitempty"`
	Model        string              `toml:"model,omitempty"`
	RecentModels []string            `toml:"recent_models,omitempty"`
	Providers    map[string]Provider `toml:"providers,omitempty"`
	Warden       *WardenConfig       `toml:"warden,omitempty"`
	Secrets      *SecretsConfig      `toml:"secrets,omitempty"`
	Audit        *AuditConfig        `toml:"audit,omitempty"`
}

// Provider configures an LLM provider within a profile.
type Provider struct {
	Type    string        `toml:"type"`
	APIKey  string        `toml:"api_key,omitempty"`
	Model   string        `toml:"model,omitempty"`
	BaseURL string        `toml:"base_url,omitempty"`
	Auth    *ProviderAuth `toml:"auth,omitempty"`
}

// ProviderAuth configures authentication for a provider.
type ProviderAuth struct {
	Type string `toml:"type"` // "api", "env", "command"
	Key  string `toml:"key,omitempty"`
}

// WardenConfig holds sandbox/isolation and network proxy settings.
type WardenConfig struct {
	Enabled     bool     `toml:"enabled"`
	Volumes     []string `toml:"volumes,omitempty"`
	ProxyAddr   string   `toml:"proxy_addr,omitempty"`   // e.g., "127.0.0.1:9090"
	PolicyFiles []string `toml:"policy_files,omitempty"` // paths to .cedar files
	DefaultDeny bool     `toml:"default_deny,omitempty"` // deny unmatched requests
	CACert      string   `toml:"ca_cert,omitempty"`      // mTLS CA certificate path
	ClientCert  string   `toml:"client_cert,omitempty"`  // mTLS client certificate
	ClientKey   string   `toml:"client_key,omitempty"`   // mTLS client key
}

// SecretsConfig holds secret substitution settings.
type SecretsConfig struct {
	Enabled        bool   `toml:"enabled"`
	CustomPatterns string `toml:"custom_patterns,omitempty"` // path to extra patterns.json
}

// AuditConfig holds session audit logging settings.
type AuditConfig struct {
	Enabled       bool   `toml:"enabled"`
	LogDir        string `toml:"log_dir,omitempty"`         // default: ~/.duckops/audit
	BackupDir     string `toml:"backup_dir,omitempty"`      // default: ~/.duckops/audit/backups
	SSHBackupHost string `toml:"ssh_backup_host,omitempty"` // optional remote backup
	SSHBackupPath string `toml:"ssh_backup_path,omitempty"`
}

type Settings struct {
	MachineName      string `toml:"machine_name,omitempty"`
	AutoAppendIgnore bool   `toml:"auto_append_gitignore,omitempty"`
	AnonymousID      string `toml:"anonymous_id,omitempty"`
	CollectTelemetry bool   `toml:"collect_telemetry,omitempty"`
	Editor           string `toml:"editor,omitempty"`
	ServerAddr       string `toml:"server_addr,omitempty"`
	AgentMode        string `toml:"agent_mode,omitempty"`
	APIGatewayURL    string `toml:"api_gateway_url,omitempty"`
}

// // LLMConfig holds configuration for a specific LLM provider.
// // Used by the legacy setup system. New code should use Provider in duckops_config.go.
// type LLMConfig struct {
// 	APIKey  string `mapstructure:"api_key" json:"api_key,omitempty"`
// 	Model   string `mapstructure:"model" json:"model,omitempty"`
// 	BaseURL string `mapstructure:"base_url" json:"base_url,omitempty"`
// }

// // Config is the legacy configuration structure used by SetupService.
// // Deprecated: Use DuckOpsConfig (config.toml) for all new config needs.
// type Config struct {
// 	Environment string               `mapstructure:"env" json:"env,omitempty"`
// 	LLMs        map[string]LLMConfig `mapstructure:"llm" json:"llm,omitempty"`
// 	Provider    string               `json:"provider"`
// }

// ========================
// ~/.duckops/ Management
// ========================

// DuckOpsDir returns the path to ~/.duckops/.
func DuckOpsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", types.Wrap(err, types.ErrCodeInternal, "cannot determine home directory")
	}
	return filepath.Join(home, ".duckops"), nil
}

// EnsureDuckOpsDir creates ~/.duckops/ and required subdirectories.
func EnsureDuckOpsDir() (string, error) {
	dir, err := DuckOpsDir()
	if err != nil {
		return "", err
	}

	dirs := []string{
		dir,
		filepath.Join(dir, "data"),
	}

	for _, d := range dirs {
		if err := os.MkdirAll(d, 0700); err != nil {
			return "", types.Wrapf(err, types.ErrCodeInternal, "cannot create directory %s", d)
		}
	}

	configPath := filepath.Join(dir, "config.toml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		if err := writeDefaultConfig(configPath); err != nil {
			return "", types.Wrap(err, types.ErrCodeInternal, "failed to write default config")
		}
	}

	return dir, nil
}

// writeDefaultConfig creates the initial config.toml with sensible defaults.
func writeDefaultConfig(path string) error {
	cfg := DuckOpsConfig{
		Profiles: map[string]Profile{
			"default": {
				APIEndpoint: "http://localhost:8090",
				Provider:    "openrouter",
				Providers: map[string]Provider{
					"openai": {
						Type: "openai",
						Auth: &ProviderAuth{Type: "env", Key: "OPENAI_API_KEY"},
					},
					"openrouter": {
						Type:    "openrouter",
						BaseURL: "https://openrouter.ai/api/v1",
						Auth:    &ProviderAuth{Type: "env", Key: "OPENROUTER_API_KEY"},
					},
				},
				Warden: &WardenConfig{
					Enabled:     true,
					DefaultDeny: false,
				},
				Secrets: &SecretsConfig{
					Enabled: true,
				},
				Audit: &AuditConfig{
					Enabled: true,
				},
			},
		},
		Settings: Settings{
			AutoAppendIgnore: true,
			CollectTelemetry: false,
			Editor:           "nano",
			ServerAddr:       ":8090",
			AgentMode:        "standalone",
			APIGatewayURL:    "http://localhost:8080",
		},
	}

	data, err := toml.Marshal(cfg)
	if err != nil {
		return err
	}

	header := []byte("# DuckOps Agent Configuration\n# See: https://github.com/SecDuckOps/agent/docs\n\n")
	return os.WriteFile(path, append(header, data...), 0600)
}

// ========================
// Config Loading
// ========================

// LoadTOML loads the DuckOps config from ~/.duckops/config.toml.
// Auto-creates the directory and default config if missing.
func LoadTOML() (*DuckOpsConfig, error) {
	dir, err := EnsureDuckOpsDir()
	if err != nil {
		return nil, err
	}

	configPath := filepath.Join(dir, "config.toml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, types.Wrap(err, types.ErrCodeInternal, "failed to read config")
	}

	var cfg DuckOpsConfig
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, types.Wrap(err, types.ErrCodeInternal, "failed to parse config.toml")
	}

	// Resolve env var auth keys
	for profName, profile := range cfg.Profiles {
		for provName, prov := range profile.Providers {
			if prov.Auth != nil && prov.Auth.Type == "env" && prov.Auth.Key != "" {
				prov.APIKey = os.Getenv(prov.Auth.Key)
				profile.Providers[provName] = prov
			}
		}
		cfg.Profiles[profName] = profile
	}

	return &cfg, nil
}

// GetProfile returns the named profile, or "default" if name is empty.
func (c *DuckOpsConfig) GetProfile(name string) (Profile, bool) {
	if name == "" {
		name = "default"
	}
	p, ok := c.Profiles[name]
	return p, ok
}

// DatabasePath returns the path to the local SQLite database.
func DatabasePath() (string, error) {
	dir, err := DuckOpsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "data", "local.db"), nil
}
