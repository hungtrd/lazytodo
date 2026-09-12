package ui

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/hungtrd/lazytodo/internal/domain"
)

// statusOrder drives the kanban columns. Archived tasks are intentionally
// absent: they are reachable only through the CLI.
var statusOrder = domain.ActiveStatuses()

func statusTitle(s domain.TaskStatus) string {
	switch s {
	case domain.TaskStatusTodo:
		return "Todo"
	case domain.TaskStatusDoing:
		return "Doing"
	case domain.TaskStatusDone:
		return "Done"
	case domain.TaskStatusArchived:
		return "Archived"
	default:
		return "Unknown"
	}
}

func prevStatus(s domain.TaskStatus) domain.TaskStatus {
	switch s {
	case domain.TaskStatusTodo:
		return domain.TaskStatusTodo
	case domain.TaskStatusDoing:
		return domain.TaskStatusTodo
	case domain.TaskStatusDone:
		return domain.TaskStatusDoing
	default:
		return domain.TaskStatusTodo
	}
}

func nextStatus(s domain.TaskStatus) domain.TaskStatus {
	switch s {
	case domain.TaskStatusTodo:
		return domain.TaskStatusDoing
	case domain.TaskStatusDoing:
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
	if a > b {
		return a
	}
	return b
}

// interleave returns a slice like: a0, sep, a1, sep, a2 ...
func interleave(items []string, sep string) []string {
	if len(items) == 0 {
		return items
	}
	out := make([]string, 0, len(items)*2-1)
	for i, s := range items {
		if i > 0 {
			out = append(out, sep)
		}
		out = append(out, s)
	}
	return out
}

func textBlink() tea.Cmd { return textinput.Blink }
