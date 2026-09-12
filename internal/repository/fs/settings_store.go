package fs

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/hungtrd/lazytodo/internal/repository"
)

const settingsFileName = "settings.json"

// SettingsStore keeps synced preferences beside the tasks file, which puts them
// inside the git repository when sync is enabled.
type SettingsStore struct {
	path string
}

// NewSettingsStoreFor derives the settings path from the tasks file location.
func NewSettingsStoreFor(tasksPath string) *SettingsStore {
	return &SettingsStore{path: filepath.Join(filepath.Dir(tasksPath), settingsFileName)}
}

func NewSettingsStoreAt(path string) *SettingsStore { return &SettingsStore{path: path} }

func (s *SettingsStore) Path() string { return s.path }

func (s *SettingsStore) Load() (repository.Settings, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return repository.Settings{}, nil
		}
		return repository.Settings{}, fmt.Errorf("read settings: %w", err)
	}
	var settings repository.Settings
	if err := json.Unmarshal(data, &settings); err != nil {
		return repository.Settings{}, fmt.Errorf("decode settings: %w", err)
	}
	return settings, nil
}

func (s *SettingsStore) Save(settings repository.Settings) error {
	b, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("encode settings: %w", err)
	}
	if err := atomicWriteFile(s.path, b, 0o644); err != nil {
		return fmt.Errorf("write settings: %w", err)
	}
	return nil
}

var _ repository.SettingsRepository = (*SettingsStore)(nil)
