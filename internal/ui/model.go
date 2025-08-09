package ui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/hungtrd/lazytodo/internal/domain"
	"github.com/hungtrd/lazytodo/internal/task"
)

type uiMode int

const (
	modeList uiMode = iota
	modeNew
	modeEdit
)

type taskRef struct {
	status domain.TaskStatus
	index  int
}

type Model struct {
	width  int
	height int

	svc *task.Service

	tasksByStatus map[domain.TaskStatus][]domain.Task
	selectedIdx   map[domain.TaskStatus]int
	focused       domain.TaskStatus

	mode       uiMode
	input      textinput.Model
	editingRef *taskRef

	vertical bool
}

func InitialModel(svc *task.Service) Model {
	ti := textinput.New()
	ti.Placeholder = "Task content..."
	ti.Prompt = "➤ "
	ti.CharLimit = 256

	m := Model{
		svc:           svc,
		tasksByStatus: map[domain.TaskStatus][]domain.Task{},
		selectedIdx: map[domain.TaskStatus]int{
			domain.TaskStatusTodo:       0,
			domain.TaskStatusInProgress: 0,
			domain.TaskStatusDone:       0,
		},
		focused: domain.TaskStatusTodo,
		mode:    modeList,
		input:   ti,
	}
	// load data
	if tasks, err := svc.Load(); err == nil {
		m.tasksByStatus = tasks
	}
	if v, err := svc.GetLayoutVertical(); err == nil {
		m.vertical = v
	}
	return m
}

func (m Model) Init() tea.Cmd { return nil }
