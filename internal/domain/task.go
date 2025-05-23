package domain

type TaskStatus int

const (
	TaskStatusTodo TaskStatus = iota
	TaskStatusInProgress
	TaskStatusDone
)

type Task struct {
	Id        string
	Content   string
	Status    TaskStatus
	IsStarred bool
	StartedAt int64
	CreatedAt int64
	UpdatedAt int64
}
