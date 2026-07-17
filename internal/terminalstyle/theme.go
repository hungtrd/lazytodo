package terminalstyle

import (
	"io"

	"github.com/charmbracelet/lipgloss"
	"github.com/hungtrd/lazytodo/internal/domain"
	"github.com/muesli/termenv"
)

const (
	green  = "2"
	yellow = "3"
	blue   = "4"
)

type Theme struct {
	renderer *lipgloss.Renderer
}

func New(w io.Writer) Theme {
	return WithRenderer(lipgloss.NewRenderer(w))
}

func WithRenderer(renderer *lipgloss.Renderer) Theme {
	return Theme{renderer: renderer}
}

func (t Theme) Renderer() *lipgloss.Renderer {
	return t.renderer
}

func (t Theme) SupportsColor() bool {
	return t.renderer.ColorProfile() != termenv.Ascii
}

func (t Theme) Status(status domain.TaskStatus) lipgloss.Style {
	style := t.renderer.NewStyle()
	if !t.SupportsColor() {
		return style
	}
	switch status {
	case domain.TaskStatusDoing:
		return style.Foreground(lipgloss.Color(blue))
	case domain.TaskStatusDone:
		return style.Foreground(lipgloss.Color(green))
	default:
		return style
	}
}

func (t Theme) Star() lipgloss.Style {
	style := t.renderer.NewStyle()
	if !t.SupportsColor() {
		return style
	}
	return style.Foreground(lipgloss.Color(yellow))
}
