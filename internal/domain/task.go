package domain

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type TaskStatus int

const (
	TaskStatusTodo TaskStatus = iota
	TaskStatusDoing
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
	case TaskStatusDoing:
		return "doing"
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
	case "doing":
		return TaskStatusDoing, nil
	case "done":
		return TaskStatusDone, nil
	default:
		return TaskStatusTodo, fmt.Errorf("invalid status %q (expected todo, doing, or done)", value)
	}
}

func (s TaskStatus) MarshalJSON() ([]byte, error) {
	value, err := s.MarshalText()
	if err != nil {
		return nil, err
	}
	return json.Marshal(string(value))
}

func (s *TaskStatus) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '"' {
		var value string
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		return s.UnmarshalText([]byte(value))
	}
	var legacy int
	if err := json.Unmarshal(data, &legacy); err != nil {
		return fmt.Errorf("decode task status: %w", err)
	}
	return s.UnmarshalText([]byte(strconv.Itoa(legacy)))
}

func (s TaskStatus) MarshalText() ([]byte, error) {
	if s < TaskStatusTodo || s > TaskStatusDone {
		return nil, fmt.Errorf("invalid task status %d", s)
	}
	return []byte(s.String()), nil
}

func (s *TaskStatus) UnmarshalText(data []byte) error {
	value := string(data)
	switch value {
	case "0":
		*s = TaskStatusTodo
		return nil
	case "1":
		*s = TaskStatusDoing
		return nil
	case "2":
		*s = TaskStatusDone
		return nil
	}
	status, err := ParseTaskStatus(value)
	if err != nil {
		return err
	}
	*s = status
	return nil
}
