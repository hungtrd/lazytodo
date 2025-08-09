package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/hungtrd/lazytodo/internal/domain"
)

const defaultDirName = ".lazytodo"
const tasksFileName = "tasks.json"

// DefaultDir returns the default directory path, e.g. ~/.lazytodo
func DefaultDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, defaultDirName), nil
}

func tasksFilePath() (string, error) {
	dir, err := DefaultDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, tasksFileName), nil
}

func ensureDirExists() error {
	dir, err := DefaultDir()
	if err != nil {
		return err
	}
	return os.MkdirAll(dir, 0o755)
}

// LoadTasks loads tasks from ~/.lazytodo/tasks.json. If the file
// does not exist, it returns an initialized empty map.
func LoadTasks() (map[domain.TaskStatus][]domain.Task, error) {
	path, err := tasksFilePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return emptyTaskMap(), nil
		}
		return nil, fmt.Errorf("read tasks file: %w", err)
	}
	var m map[domain.TaskStatus][]domain.Task
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("decode tasks: %w", err)
	}
	if m == nil {
		m = emptyTaskMap()
	}
	return m, nil
}

// SaveTasks writes tasks to ~/.lazytodo/tasks.json, creating directories as needed.
func SaveTasks(m map[domain.TaskStatus][]domain.Task) error {
	if err := ensureDirExists(); err != nil {
		return err
	}
	path, err := tasksFilePath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("encode tasks: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write tasks file: %w", err)
	}
	return nil
}

func emptyTaskMap() map[domain.TaskStatus][]domain.Task {
	return map[domain.TaskStatus][]domain.Task{
		domain.TaskStatusTodo:       {},
		domain.TaskStatusInProgress: {},
		domain.TaskStatusDone:       {},
	}
}
