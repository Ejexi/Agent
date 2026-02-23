package setup

import (
	"github.com/SecDuckOps/Agent/internal/config"
	"github.com/SecDuckOps/Shared/types"
	"encoding/json"
	"os"
	"path/filepath"
)

// FileSetupRepository implements SetupRepository for a JSON file on disk.
type FileSetupRepository struct {
	path string
}

func NewFileSetupRepository(path string) *FileSetupRepository {
	return &FileSetupRepository{path: path}
}

func (r *FileSetupRepository) Load() (*config.Config, error) {
	data, err := os.ReadFile(r.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, types.Wrap(err, types.ErrCodeNotFound, "config file not found")
		}
		return nil, types.Wrap(err, types.ErrCodeInternal, "failed to read config file")
	}

	var cfg config.Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, types.Wrap(err, types.ErrCodeInternal, "failed to unmarshal config")
	}

	return &cfg, nil
}

func (r *FileSetupRepository) Save(cfg *config.Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return types.Wrap(err, types.ErrCodeInternal, "failed to marshal config")
	}

	dir := filepath.Dir(r.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return types.Wrap(err, types.ErrCodeInternal, "failed to create config directory")
	}

	if err := os.WriteFile(r.path, data, 0644); err != nil {
		return types.Wrap(err, types.ErrCodeInternal, "failed to write config file")
	}

	return nil
}
