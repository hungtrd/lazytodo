package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/hungtrd/lazytodo/internal/domain"
	"github.com/hungtrd/lazytodo/internal/terminalstyle"
)

const cursorBullet = "•"

type uiStyles struct {
	theme           terminalstyle.Theme
	header          lipgloss.Style
	column          lipgloss.Style
	focusedColumn   lipgloss.Style
	unfocusedColumn lipgloss.Style
	selectedTask    lipgloss.Style
	footer          lipgloss.Style
}

func newUIStyles(renderer *lipgloss.Renderer) uiStyles {
	theme := terminalstyle.WithRenderer(renderer)
	column := renderer.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 1)
	return uiStyles{
		theme:           theme,
		header:          renderer.NewStyle().Bold(true),
		column:          column,
		focusedColumn:   column.Copy().BorderForeground(lipgloss.Color("12")),
		unfocusedColumn: column.Copy().BorderForeground(lipgloss.Color("8")),
		selectedTask:    renderer.NewStyle().Reverse(true).Bold(true),
		footer:          renderer.NewStyle().Foreground(lipgloss.Color("8")).MarginTop(1),
	}
}

func (s uiStyles) renderHeader(status domain.TaskStatus, title string, count int) string {
	label := s.theme.Status(status).Bold(true).Render(title)
	return label + s.header.Render(fmt.Sprintf(" (%d)", count))
}

func (s uiStyles) taskStyle(status domain.TaskStatus, selected bool) lipgloss.Style {
	style := s.theme.Status(status)
	if status == domain.TaskStatusDone {
		style = style.Strikethrough(true)
	}
	if selected {
		style = style.Inherit(s.selectedTask)
	}
	return style
}

func (s uiStyles) gap(width int) string {
	return s.theme.Renderer().NewStyle().Width(width).Render(" ")
}
