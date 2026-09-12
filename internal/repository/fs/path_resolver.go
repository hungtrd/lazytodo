package fs

import (
	"errors"
	"fmt"
	iofs "io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/hungtrd/lazytodo/internal/repository"
)

const (
	defaultDirName  = ".lazytodo"
	customDirName   = "lazytodo"
	tasksFileName   = "tasks.jsonl"
	configFileName  = "config.json"
	syncLogFileName = "sync.log"

	// legacyTasksFileName is the pre-v4 single-document JSON store. It is read
	// once during migration and then removed.
	legacyTasksFileName = "tasks.json"
)

func defaultDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, defaultDirName), nil
}

func ConfigFilePath() (string, error) {
	dir, err := defaultDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, configFileName), nil
}

// SyncLogPath is where git sync records failures. It stays machine-local so it
// is never pushed to the sync remote.
func SyncLogPath() (string, error) {
	dir, err := defaultDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, syncLogFileName), nil
}

func TasksFilePath(cfg repository.Config) (string, error) {
	if cfg.StorageRoot == "" {
		dir, err := defaultDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(dir, tasksFileName), nil
	}
	return filepath.Join(cfg.StorageRoot, customDirName, tasksFileName), nil
}

// LegacyTasksFilePath returns the pre-v4 single-document JSON path that
// corresponds to a given tasks file, so a store can migrate it on first load.
func LegacyTasksFilePath(tasksPath string) string {
	if strings.HasSuffix(tasksPath, ".jsonl") {
		return strings.TrimSuffix(tasksPath, "l")
	}
	return filepath.Join(filepath.Dir(tasksPath), legacyTasksFileName)
}

func ResolveStorageRoot(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", errors.New("storage root is empty")
	}
	if value == "~" || strings.HasPrefix(value, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home dir: %w", err)
		}
		value = filepath.Join(home, strings.TrimPrefix(value, "~/"))
	} else if strings.HasPrefix(value, "~") {
		return "", errors.New("only ~ and ~/path home expansion are supported")
	}
	abs, err := filepath.Abs(value)
	if err != nil {
		return "", fmt.Errorf("resolve storage root: %w", err)
	}
	if info, err := os.Stat(abs); err == nil && !info.IsDir() {
		return "", fmt.Errorf("storage root %q is not a directory", abs)
	} else if err != nil && !errors.Is(err, iofs.ErrNotExist) {
		return "", fmt.Errorf("inspect storage root: %w", err)
	}
	return filepath.Clean(abs), nil
}

func EnsureTasksDir(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create tasks directory: %w", err)
	}
	return nil
}

func CopyTasksFile(source, destination string, force bool) (bool, error) {
	if source == destination {
		return false, nil
	}
	data, err := os.ReadFile(source)
	if err != nil {
		if errors.Is(err, iofs.ErrNotExist) {
			return false, EnsureTasksDir(destination)
		}
		return false, fmt.Errorf("read current tasks file: %w", err)
	}
	if _, err := os.Stat(destination); err == nil && !force {
		return false, fmt.Errorf("tasks file already exists at %s (use --force to overwrite)", destination)
	} else if err != nil && !errors.Is(err, iofs.ErrNotExist) {
		return false, fmt.Errorf("inspect destination tasks file: %w", err)
	}
	if err := atomicWriteFile(destination, data, 0o644); err != nil {
		return false, fmt.Errorf("copy tasks file: %w", err)
	}
	return true, nil
}

func RemoveTasksFile(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, iofs.ErrNotExist) {
		return fmt.Errorf("remove old tasks file: %w", err)
	}
	return nil
}

func atomicWriteFile(path string, data []byte, mode iofs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".lazytodo-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
