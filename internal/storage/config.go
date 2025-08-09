package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

const configFileName = "config.json"

type Config struct {
	Vertical bool `json:"vertical"`
}

func defaultConfig() Config { return Config{Vertical: false} }

func configFilePath() (string, error) {
	dir, err := DefaultDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, configFileName), nil
}

func LoadConfig() (Config, error) {
	path, err := configFilePath()
	if err != nil {
		return defaultConfig(), err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return defaultConfig(), nil
		}
		return defaultConfig(), fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return defaultConfig(), fmt.Errorf("decode config: %w", err)
	}
	return cfg, nil
}

func SaveConfig(cfg Config) error {
	if err := ensureDirExists(); err != nil {
		return err
	}
	path, err := configFilePath()
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}
