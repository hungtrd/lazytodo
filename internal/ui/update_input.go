package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/hungtrd/lazytodo/internal/domain"
)

func (m Model) updateInputMode(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.Type {
	case tea.KeyEsc:
		m.mode = modeList
		m.editingRef = nil
		m.input.Blur()
		return m, nil
	case tea.KeyEnter:
		content := strings.TrimSpace(m.input.Value())
		if content != "" {
			if m.mode == modeNew {
				if t, err := m.svc.Add(content); err == nil {
					m.addTaskToState(t)
				}
			} else if m.mode == modeEdit && m.editingRef != nil {
				ref := *m.editingRef
				t := m.tasksByStatus[ref.status][ref.index]
				_ = m.svc.UpdateContent(t.Id, content)
				m.tasksByStatus[ref.status][ref.index].Content = content
			}
		}
		m.mode = modeList
		m.input.Blur()
		return m, nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(key)
	return m, cmd
}

func (m *Model) addTaskToState(t domain.Task) {
	// insert into Todo at top and update focus/selection
	m.tasksByStatus[domain.TaskStatusTodo] = append([]domain.Task{t}, m.tasksByStatus[domain.TaskStatusTodo]...)
	m.focused = domain.TaskStatusTodo
	if m.selectedIdx == nil {
		m.selectedIdx = map[domain.TaskStatus]int{}
	}
	m.selectedIdx[domain.TaskStatusTodo] = 0
}
