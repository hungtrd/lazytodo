package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/hungtrd/lazytodo/internal/domain"
	"github.com/hungtrd/lazytodo/internal/task"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC || msg.String() == "q" {
			return m, tea.Quit
		}
		switch m.mode {
		case modeList:
			return m.updateListMode(msg)
		case modeNew, modeEdit:
			return m.updateInputMode(msg)
		}
	}
	return m, nil
}

func (m Model) updateListMode(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	col := m.focused
	items := m.tasksByStatus[col]
	cur := m.selectedIdx[col]

	switch key.String() {
	case "up", "k":
		if len(items) == 0 {
			return m, nil
		}
		order := task.SortedOrder(items)
		pos := indexOf(order, cur)
		if pos == -1 {
			pos = 0
		}
		if pos > 0 {
			pos--
		}
		m.selectedIdx[col] = order[pos]
	case "down", "j":
		if len(items) == 0 {
			return m, nil
		}
		order := task.SortedOrder(items)
		pos := indexOf(order, cur)
		if pos == -1 {
			pos = 0
		}
		if pos < len(order)-1 {
			pos++
		}
		m.selectedIdx[col] = order[pos]
	case "left", "h":
		m.focused = prevStatus(m.focused)
	case "right", "l":
		m.focused = nextStatus(m.focused)
	case "[", "\\":
		m = m.moveTask(col, cur, prevStatus(col))
	case "]", "/":
		m = m.moveTask(col, cur, nextStatus(col))
	case " ", "x":
		target := domain.TaskStatusDone
		if col == domain.TaskStatusDone {
			target = domain.TaskStatusTodo
		}
		m = m.moveTask(col, cur, target)
	case "s":
		if len(items) == 0 {
			return m, nil
		}
		it := items[cur]
		_ = m.svc.ToggleStar(it.Id)
		m.tasksByStatus[col][cur].IsStarred = !m.tasksByStatus[col][cur].IsStarred
	case "n":
		m.mode = modeNew
		m.input.SetValue("")
		m.input.Focus()
		return m, textBlink()
	case "e":
		if len(items) == 0 {
			return m, nil
		}
		m.mode = modeEdit
		m.editingRef = &taskRef{status: col, index: cur}
		m.input.SetValue(items[cur].Content)
		m.input.CursorEnd()
		m.input.Focus()
		return m, textBlink()
	case "backspace", "delete", "d":
		if len(items) == 0 {
			return m, nil
		}
		it := items[cur]
		_ = m.svc.Delete(it.Id)
		m.deleteTask(col, cur)
	case "g":
		if len(items) > 0 {
			order := task.SortedOrder(items)
			m.selectedIdx[col] = order[0]
		}
	case "G":
		if len(items) > 0 {
			order := task.SortedOrder(items)
			m.selectedIdx[col] = order[len(order)-1]
		}
	case "v":
		m.vertical = !m.vertical
		_ = m.svc.SetLayoutVertical(m.vertical)
	}
	return m, nil
}
