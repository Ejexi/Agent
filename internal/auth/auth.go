package auth

import (
	"os"
	"path/filepath"

	"github.com/SecDuckOps/shared/types"
	"github.com/pelletier/go-toml/v2"
)

// authData is the on-disk format of ~/.duckops/auth.toml.
type authData struct {
	APIKey string `toml:"api_key"`
}

// Store manages authentication credentials stored in ~/.duckops/auth.toml.
// Secrets are written with 0600 permissions and never logged.
type Store struct {
	path string
}

// NewStore creates a new auth store.
// If path is empty, defaults to ~/.duckops/auth.toml.
func NewStore(path string) (*Store, error) {
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, types.Wrap(err, types.ErrCodeInternal, "cannot determine home directory")
		}
		path = filepath.Join(home, ".duckops", "auth.toml")
	}

	// Ensure parent directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, types.Wrap(err, types.ErrCodeInternal, "cannot create auth directory")
	}

	return &Store{path: path}, nil
}

// APIKey loads and returns the stored API key.
// Returns empty string if the file doesn't exist.
func (s *Store) APIKey() string {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return ""
	}

	var auth authData
	if err := toml.Unmarshal(data, &auth); err != nil {
		return ""
	}
	return auth.APIKey
}

// SaveAPIKey writes the API key to disk with restricted permissions.
func (s *Store) SaveAPIKey(key string) error {
	auth := authData{APIKey: key}

	data, err := toml.Marshal(auth)
	if err != nil {
		return types.Wrap(err, types.ErrCodeInternal, "failed to marshal auth data")
	}

	// Write with 0600 — only owner can read/write
	if err := os.WriteFile(s.path, data, 0600); err != nil {
		return types.Wrap(err, types.ErrCodeInternal, "failed to write auth file")
	}

	return nil
}

// HasAPIKey returns true if an API key is stored.
func (s *Store) HasAPIKey() bool {
	return s.APIKey() != ""
}

// Path returns the auth file path.
func (s *Store) Path() string {
	return s.path
}
