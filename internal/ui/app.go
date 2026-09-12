package ui

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hungtrd/lazytodo/internal/task"
)

func Run(svc *task.Service) error {
	// Pull once before the board is built so the session starts from what other
	// machines have already pushed. A failure here is not fatal: the local
	// tasks are still perfectly usable offline.
	if syncer := svc.Syncer(); syncer != nil {
		if err := syncer.Pull(); err != nil {
			fmt.Fprintf(os.Stderr, "warning: sync pull failed: %v\n", err)
		}
	}

	renderer := lipgloss.NewRenderer(os.Stdout)
	p := tea.NewProgram(initialModel(svc, renderer), tea.WithAltScreen(), tea.WithOutput(os.Stdout))
	_, err := p.Run()

	// The debounced push would otherwise be lost when the process exits.
	if syncer := svc.Syncer(); syncer != nil {
		if flushErr := syncer.Flush(); flushErr != nil && err == nil {
			fmt.Fprintf(os.Stderr, "warning: sync failed: %v\n", flushErr)
		}
	}
	return err
}
