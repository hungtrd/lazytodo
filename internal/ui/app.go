package ui

import (
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hungtrd/lazytodo/internal/task"
)

func Run(svc *task.Service) error {
	renderer := lipgloss.NewRenderer(os.Stdout)
	p := tea.NewProgram(initialModel(svc, renderer), tea.WithAltScreen(), tea.WithOutput(os.Stdout))
	_, err := p.Run()
	return err
}
