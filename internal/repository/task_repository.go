package repository

import "github.com/hungtrd/lazytodo/internal/domain"

const CurrentTaskDataVersion = 3

type TaskData struct {
	Version int                                 `json:"version"`
	NextID  int64                               `json:"next_id"`
	Tasks   map[domain.TaskStatus][]domain.Task `json:"tasks"`
}

// TaskRepository abstracts persistence for tasks grouped by status.
type TaskRepository interface {
	Load() (TaskData, error)
	Save(TaskData) error
}
