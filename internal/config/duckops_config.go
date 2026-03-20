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
	MCP      MCPConfig          `toml:"mcp"`
	Hooks    HooksConfig        `toml:"hooks"`
}

// HooksConfig holds all user-defined hook registrations.
// Hooks can also be discovered automatically from ~/.duckops/hooks/<EventType>/*.sh.
//
// Example config.toml:
//
//	[hooks]
//	dir = "~/.duckops/hooks"
//
//	[[hooks.BeforeScan]]
//	name    = "approve-scan"
//	command = "~/.duckops/hooks/approve-scan.sh"
//	timeout = 10000
//
//	[[hooks.AfterScan]]
//	name    = "notify-slack"
//	command = "~/.duckops/hooks/slack-notify.sh"
//
//	[[hooks.BeforeTool]]
//	name    = "block-dangerous"
//	matcher = "shell|terminal"
//	command = "~/.duckops/hooks/security-gate.sh"
type HooksConfig struct {
	// Dir is the base directory for convention-based hook discovery.
	// Defaults to ~/.duckops/hooks.
	Dir string `toml:"dir,omitempty"`

	BeforeTool   []HookEntry `toml:"BeforeTool,omitempty"`
	AfterTool    []HookEntry `toml:"AfterTool,omitempty"`
	BeforeScan   []HookEntry `toml:"BeforeScan,omitempty"`
	AfterScan    []HookEntry `toml:"AfterScan,omitempty"`
	SessionStart []HookEntry `toml:"SessionStart,omitempty"`
	SessionEnd   []HookEntry `toml:"SessionEnd,omitempty"`
}

// HookEntry is a single hook registration in config.toml.
type HookEntry struct {
	Name      string `toml:"name"`
	Matcher   string `toml:"matcher,omitempty"`
	Command   string `toml:"command"`
	TimeoutMs int    `toml:"timeout,omitempty"`
	Enabled   *bool  `toml:"enabled,omitempty"` // pointer so absent = true
}

// MCPConfig holds global MCP server configuration.
// Individual servers are listed under [[mcp.servers]] in config.toml.
type MCPConfig struct {
	Servers []MCPServerEntry `toml:"servers"`
}

// MCPServerEntry maps directly to domain/mcp.ServerConfig.
type MCPServerEntry struct {
	Name         string            `toml:"name"`
	Transport    string            `toml:"transport"` // "stdio" | "sse"
	Command      []string          `toml:"command"`
	URL          string            `toml:"url,omitempty"`
	Env          map[string]string `toml:"env,omitempty"`
	AllowedTools []string          `toml:"allowed_tools,omitempty"`
	Enabled      bool              `toml:"enabled"`
}

// Profile represents a named configuration profile.
type Profile struct {
	APIEndpoint  string              `toml:"api_endpoint,omitempty"`
	Provider     string              `toml:"provider,omitempty"`
	Model        string              `toml:"model,omitempty"`
	RecentModels []string            `toml:"recent_models,omitempty"`
	Providers    map[string]Provider `toml:"providers,omitempty"`
	Warden       *WardenConfig       `toml:"warden,omitempty"`
	Secrets      *SecretsConfig      `toml:"secrets,omitempty"`
	Audit        *AuditConfig        `toml:"audit,omitempty"`
	// AllowedTools restricts which tools the agent may call (empty = all).
	AllowedTools []string `toml:"allowed_tools,omitempty"`
	// AutoApproveTools lists tools that are approved automatically without pausing.
	// Mirrors duckops profile.auto_approve.
	// Example: ["view", "load_skill", "mcp_list"]
	AutoApproveTools []string `toml:"auto_approve,omitempty"`
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
		filepath.Join(dir, "policies"),
	}

	for _, d := range dirs {
		if err := os.MkdirAll(d, 0700); err != nil {
			return "", types.Wrapf(err, types.ErrCodeInternal, "cannot create directory %s", d)
		}
	}

	configPath := filepath.Join(dir, "config.toml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		if err := writeDefaultConfig(configPath, dir); err != nil {
			return "", types.Wrap(err, types.ErrCodeInternal, "failed to write default config")
		}
	}

	// Ensure default policies exist
	policyDir := filepath.Join(dir, "policies")
	safePolicyPath := filepath.Join(policyDir, "safe_execution.cedar")
	if _, err := os.Stat(safePolicyPath); os.IsNotExist(err) {
		defaultSafe := "// Default Safe Execution Policy\nALLOW all command\n"
		if err := os.WriteFile(safePolicyPath, []byte(defaultSafe), 0600); err != nil {
			return "", types.Wrap(err, types.ErrCodeInternal, "failed to write default safe policy")
		}
	}

	denyPolicyPath := filepath.Join(policyDir, "deny_execution.cedar")
	if _, err := os.Stat(denyPolicyPath); os.IsNotExist(err) {
		defaultDeny := "// Default Deny Execution Policy\n// DENY command \"rm\"\n"
		if err := os.WriteFile(denyPolicyPath, []byte(defaultDeny), 0600); err != nil {
			return "", types.Wrap(err, types.ErrCodeInternal, "failed to write default deny policy")
		}
	}

	return dir, nil
}

// writeDefaultConfig creates the initial config.toml with sensible defaults.
func writeDefaultConfig(path string, duckopsDir string) error {
	cfg := DuckOpsConfig{
		Profiles: map[string]Profile{
			"default": {
				APIEndpoint:  "https://api.secduckops.dev",
				Provider:     "openrouter",
				Model:        "openrouter/arcee-ai/trinity-large-preview:free",
				RecentModels: []string{"openrouter/trinity-large-preview:free"},
				Providers: map[string]Provider{
					"openai": {
						Type: "openai",
						Auth: &ProviderAuth{Type: "env", Key: "OPENAI_API_KEY"},
					},
					"openrouter": {
						Type:    "custom",
						BaseURL: "https://openrouter.ai/api/v1",
						Auth:    &ProviderAuth{Type: "env", Key: "OPENROUTER_API_KEY"},
					},
				},
				Warden: &WardenConfig{
					Enabled:     true,
					DefaultDeny: true,
					Volumes: []string{
						"~/.duckops/config.toml:/home/agent/.duckops/config.toml:ro",
						"~/.duckops/data/local.db:/home/agent/.duckops/data/local.db",
						"./:/agent:ro",
						"~/.aws/credentials:/home/agent/.aws/credentials:ro",
					},
					PolicyFiles: []string{
						filepath.Join(duckopsDir, "policies", "deny_execution.cedar"),
					},
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
			MachineName:      "duck-agent-01",
			AnonymousID:      "default-uuid-for-telemetry",
			AutoAppendIgnore: true,
			CollectTelemetry: true,
			Editor:           "nano",
			ServerAddr:       ":8090",
			AgentMode:        "Stand Duck",
			APIGatewayURL:    "https://api.secduckops.dev",
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

// SaveTOML saves the DuckOps config back to ~/.duckops/config.toml.
func (c *DuckOpsConfig) SaveTOML() error {
	dir, err := DuckOpsDir()
	if err != nil {
		return err
	}
	configPath := filepath.Join(dir, "config.toml")

	data, err := toml.Marshal(c)
	if err != nil {
		return types.Wrap(err, types.ErrCodeInternal, "failed to serialize config to TOML")
	}

	header := []byte("# DuckOps Agent Configuration\n# Updated via interactive setup\n\n")
	return os.WriteFile(configPath, append(header, data...), 0600)
}

// SetProfile adds or updates a profile in the config.
func (c *DuckOpsConfig) SetProfile(name string, profile Profile) {
	if c.Profiles == nil {
		c.Profiles = make(map[string]Profile)
	}
	c.Profiles[name] = profile
}

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
