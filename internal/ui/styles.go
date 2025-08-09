package ui

import "github.com/charmbracelet/lipgloss"

var (
	headerStyle         = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	columnStyle         = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 1)
	focusedColStyle     = columnStyle.Copy().BorderForeground(lipgloss.Color("12"))
	unfocusedColStyle   = columnStyle.Copy().BorderForeground(lipgloss.Color("240"))
	selectedItemStyle   = lipgloss.NewStyle().Reverse(true).Bold(true)
	selectedLineBgStyle = lipgloss.NewStyle().Background(lipgloss.Color("236"))
	selectedTextStyle   = lipgloss.NewStyle().Background(lipgloss.Color("236")).Foreground(lipgloss.Color("229")).Bold(true)
	starredStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	doneStyle           = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Strikethrough(true)
	footerStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).MarginTop(1)
	cursorBullet        = "•"
)
