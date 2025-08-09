package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/hungtrd/lazytodo/internal/task"
)

func Run(svc *task.Service) error {
	p := tea.NewProgram(InitialModel(svc), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
