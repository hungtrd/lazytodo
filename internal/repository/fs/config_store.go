package fs

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/hungtrd/lazytodo/internal/repository"
)

type ConfigStore struct{}

func NewConfigStore() *ConfigStore { return &ConfigStore{} }

func (s *ConfigStore) Load() (repository.Config, error) {
	path, err := ConfigFilePath()
	if err != nil {
		return repository.Config{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return repository.Config{Vertical: false}, nil
		}
		return repository.Config{Vertical: false}, fmt.Errorf("read config: %w", err)
	}
	var cfg repository.Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return repository.Config{Vertical: false}, fmt.Errorf("decode config: %w", err)
	}
	return cfg, nil
}

func (s *ConfigStore) Save(cfg repository.Config) error {
	path, err := ConfigFilePath()
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	if err := atomicWriteFile(path, b, 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

var _ repository.ConfigRepository = (*ConfigStore)(nil)
