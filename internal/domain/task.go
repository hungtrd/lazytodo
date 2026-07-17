package domain

import (
	"fmt"
	"strings"
)

type TaskStatus int

const (
	TaskStatusTodo TaskStatus = iota
	TaskStatusInProgress
	TaskStatusDone
)

type Task struct {
	Id        string     `json:"id"`
	Content   string     `json:"content"`
	Status    TaskStatus `json:"status"`
	IsStarred bool       `json:"is_starred"`
	StartedAt int64      `json:"started_at,omitempty"`
	CreatedAt int64      `json:"created_at"`
	UpdatedAt int64      `json:"updated_at,omitempty"`
}

func (s TaskStatus) String() string {
	switch s {
	case TaskStatusTodo:
		return "todo"
	case TaskStatusInProgress:
		return "in-progress"
	case TaskStatusDone:
		return "done"
	default:
		return "unknown"
	}
}

func ParseTaskStatus(value string) (TaskStatus, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "todo":
		return TaskStatusTodo, nil
	case "in-progress", "in_progress", "inprogress":
		return TaskStatusInProgress, nil
	case "done":
		return TaskStatusDone, nil
	default:
		return TaskStatusTodo, fmt.Errorf("invalid status %q (expected todo, in-progress, or done)", value)
	}
}
