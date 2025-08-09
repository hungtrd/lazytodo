package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/hungtrd/lazytodo/internal/domain"
)

var statusOrder = []domain.TaskStatus{
	domain.TaskStatusTodo,
	domain.TaskStatusInProgress,
	domain.TaskStatusDone,
}

func statusTitle(s domain.TaskStatus) string {
	switch s {
	case domain.TaskStatusTodo:
		return "Todo"
	case domain.TaskStatusInProgress:
		return "In Progress"
	case domain.TaskStatusDone:
		return "Done"
	default:
		return "Unknown"
	}
}

func prevStatus(s domain.TaskStatus) domain.TaskStatus {
	switch s {
	case domain.TaskStatusTodo:
		return domain.TaskStatusTodo
	case domain.TaskStatusInProgress:
		return domain.TaskStatusTodo
	case domain.TaskStatusDone:
		return domain.TaskStatusInProgress
	default:
		return domain.TaskStatusTodo
	}
}

func nextStatus(s domain.TaskStatus) domain.TaskStatus {
	switch s {
	case domain.TaskStatusTodo:
		return domain.TaskStatusInProgress
	case domain.TaskStatusInProgress:
		return domain.TaskStatusDone
	case domain.TaskStatusDone:
		return domain.TaskStatusDone
	default:
		return domain.TaskStatusDone
	}
}

func indexOf(slice []int, value int) int {
	for i, v := range slice {
		if v == value {
			return i
		}
	}
	return -1
}

func max(a, b int) int {
	if a > b { return a }
	return b
}

// interleave returns a slice like: a0, sep, a1, sep, a2 ...
func interleave(items []string, sep string) []string {
	if len(items) == 0 { return items }
	out := make([]string, 0, len(items)*2-1)
	for i, s := range items {
		if i > 0 { out = append(out, sep) }
		out = append(out, s)
	}
	return out
}

func textBlink() tea.Cmd { return textinput.Blink }
