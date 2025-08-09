package ui

import (
	"time"

	"github.com/hungtrd/lazytodo/internal/domain"
)

func (m Model) moveTask(from domain.TaskStatus, index int, to domain.TaskStatus) Model {
	if from == to { return m }
	list := m.tasksByStatus[from]
	if index < 0 || index >= len(list) { return m }
	task := list[index]
	_ = m.svc.Move(task.Id, to)
	// update local state similarly
	m.tasksByStatus[from] = append(list[:index], list[index+1:]...)
	task.Status = to
	task.UpdatedAt = time.Now().Unix()
	m.tasksByStatus[to] = append([]domain.Task{task}, m.tasksByStatus[to]...)
	if index >= len(m.tasksByStatus[from]) {
		m.selectedIdx[from] = max(0, len(m.tasksByStatus[from])-1)
	}
	m.focused = to
	m.selectedIdx[to] = 0
	return m
}

func (m *Model) deleteTask(status domain.TaskStatus, index int) {
	list := m.tasksByStatus[status]
	if index < 0 || index >= len(list) { return }
	m.tasksByStatus[status] = append(list[:index], list[index+1:]...)
	if index >= len(m.tasksByStatus[status]) {
		m.selectedIdx[status] = max(0, len(m.tasksByStatus[status])-1)
	}
}
