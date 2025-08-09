package repository

import "github.com/hungtrd/lazytodo/internal/domain"

// TaskRepository abstracts persistence for tasks grouped by status.
// Implementations should be concurrency-safe if used across goroutines.
type TaskRepository interface {
	Load() (map[domain.TaskStatus][]domain.Task, error)
	Save(map[domain.TaskStatus][]domain.Task) error
}
