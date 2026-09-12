package domain

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
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
	TaskStatusArchived
)

type Task struct {
	// Id is the short handle people type. It is only unique within one
	// machine's counter, so sync may renumber it; UID is what identifies the
	// task itself.
	Id string `json:"id"`
	// UID is globally unique and never changes, which is what lets two
	// machines tell "the same task, edited twice" from "two different tasks
	// that happened to get the same number".
	UID        string     `json:"uid,omitempty"`
	Content    string     `json:"content"`
	Status     TaskStatus `json:"status"`
	IsStarred  bool       `json:"is_starred"`
	StartedAt  int64      `json:"started_at,omitempty"`
	CreatedAt  int64      `json:"created_at"`
	UpdatedAt  int64      `json:"updated_at,omitempty"`
	ArchivedAt int64      `json:"archived_at,omitempty"`
}

// NewUID returns a random identifier for a freshly created task.
func NewUID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		// A failing system RNG is not a reason to refuse to create a task;
		// DeriveUID still yields something stable and collision-resistant.
		return ""
	}
	return hex.EncodeToString(buf)
}

// DeriveUID back-fills a UID for a task that predates the field. It is a pure
// function of the task, so two machines migrating the same history arrive at
// the same identifier instead of duplicating every task on first sync.
func DeriveUID(task Task) string {
	sum := sha256.Sum256(fmt.Appendf(nil, "%s\x00%d\x00%s", task.Id, task.CreatedAt, task.Content))
	return hex.EncodeToString(sum[:8])
}

// ActiveStatuses lists the statuses shown by default in the TUI and in an
// unfiltered list. Archived tasks are deliberately excluded.
func ActiveStatuses() []TaskStatus {
	return []TaskStatus{TaskStatusTodo, TaskStatusDoing, TaskStatusDone}
}

// AllStatuses lists every status, including archived. Use it when looking a
// task up by id or when iterating over stored buckets.
func AllStatuses() []TaskStatus {
	return []TaskStatus{TaskStatusTodo, TaskStatusDoing, TaskStatusDone, TaskStatusArchived}
}

func (s TaskStatus) String() string {
	switch s {
	case TaskStatusTodo:
		return "todo"
	case TaskStatusDoing:
		return "doing"
	case TaskStatusDone:
		return "done"
	case TaskStatusArchived:
		return "archived"
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
	case "archived":
		return TaskStatusArchived, nil
	default:
		return TaskStatusTodo, fmt.Errorf("invalid status %q (expected todo, doing, done, or archived)", value)
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
	if s < TaskStatusTodo || s > TaskStatusArchived {
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
	case "3":
		*s = TaskStatusArchived
		return nil
	}
	status, err := ParseTaskStatus(value)
	if err != nil {
		return err
	}
	*s = status
	return nil
}
