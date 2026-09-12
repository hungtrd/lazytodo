package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/term"
	"github.com/hungtrd/lazytodo/internal/domain"
	"github.com/hungtrd/lazytodo/internal/task"
)

// ErrNoTerminal is returned when the edit form is asked for outside a terminal,
// where the caller should pass flags instead.
var ErrNoTerminal = fmt.Errorf("editing interactively needs a terminal; pass --content, --status, --star or --unstar instead")

type editField int

const (
	fieldContent editField = iota
	fieldStatus
	fieldStarred
	fieldCount
)

// editModel is a standalone form, separate from the kanban Model, so that
// `lazytodo edit <id>` can open it without pulling up the whole board.
type editModel struct {
	original domain.Task
	content  textarea.Model
	status   domain.TaskStatus
	starred  bool

	focus     editField
	styles    uiStyles
	width     int
	saved     bool
	cancelled bool
}

func newEditModel(item domain.Task, renderer *lipgloss.Renderer) editModel {
	area := textarea.New()
	area.Placeholder = "Task content..."
	area.SetValue(item.Content)
	area.CharLimit = 1024
	area.SetHeight(4)
	area.Focus()
	area.CursorEnd()

	return editModel{
		original: item,
		content:  area,
		status:   item.Status,
		starred:  item.IsStarred,
		focus:    fieldContent,
		styles:   newUIStyles(renderer),
		width:    72,
	}
}

func (m editModel) Init() tea.Cmd { return textarea.Blink }

func (m editModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.content.SetWidth(max(20, msg.Width-4))
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "ctrl+c":
			m.cancelled = true
			return m, tea.Quit
		case "ctrl+s":
			m.saved = true
			return m, tea.Quit
		case "tab":
			m = m.moveFocus(1)
			return m, nil
		case "shift+tab":
			m = m.moveFocus(-1)
			return m, nil
		}

		// The content area consumes every other key, including enter, so that
		// multi-line editing works; ctrl+s is what commits.
		if m.focus == fieldContent {
			var cmd tea.Cmd
			m.content, cmd = m.content.Update(msg)
			return m, cmd
		}
		switch msg.String() {
		case "left", "h":
			if m.focus == fieldStatus {
				m.status = prevEditStatus(m.status)
			}
		case "right", "l":
			if m.focus == fieldStatus {
				m.status = nextEditStatus(m.status)
			}
		case " ", "x":
			if m.focus == fieldStarred {
				m.starred = !m.starred
			}
		case "enter":
			m.saved = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m editModel) moveFocus(delta int) editModel {
	next := editField((int(m.focus) + delta + int(fieldCount)) % int(fieldCount))
	m.focus = next
	if next == fieldContent {
		m.content.Focus()
	} else {
		m.content.Blur()
	}
	return m
}

func (m editModel) View() string {
	title := m.styles.header.Render(fmt.Sprintf("Edit task %s", m.original.Id))

	statusValue := m.styles.theme.Status(m.status).Render(statusTitle(m.status))
	starValue := "no"
	if m.starred {
		starValue = m.styles.theme.Star().Render("★ yes")
	}

	rows := []string{
		m.label("Content", fieldContent),
		m.content.View(),
		m.label("Status", fieldStatus) + "  " + statusValue + m.styles.footer.Render("   ←/→ to change"),
		m.label("Starred", fieldStarred) + " " + starValue + m.styles.footer.Render("   space to toggle"),
	}
	help := m.styles.footer.Render("tab/shift+tab: next field   ctrl+s: save   esc: cancel")
	body := strings.Join(rows, "\n") + "\n" + help

	return m.styles.focusedColumn.Width(max(20, m.width-4)).Render(title + "\n\n" + body)
}

// label marks the focused row with the same bullet the board uses.
func (m editModel) label(text string, field editField) string {
	marker := "  "
	if m.focus == field {
		marker = cursorBullet + " "
	}
	return marker + m.styles.header.Render(text+":")
}

// prevEditStatus and nextEditStatus cycle through every status, archived
// included, so the form can archive or restore in place.
func prevEditStatus(status domain.TaskStatus) domain.TaskStatus {
	all := domain.AllStatuses()
	for i, candidate := range all {
		if candidate == status && i > 0 {
			return all[i-1]
		}
	}
	return status
}

func nextEditStatus(status domain.TaskStatus) domain.TaskStatus {
	all := domain.AllStatuses()
	for i, candidate := range all {
		if candidate == status && i < len(all)-1 {
			return all[i+1]
		}
	}
	return status
}

// RunEditForm opens the interactive editor and reports the changes the user
// made. saved is false when they cancelled or changed nothing.
func RunEditForm(item domain.Task) (task.Patch, bool, error) {
	if !term.IsTerminal(os.Stdout.Fd()) || !term.IsTerminal(os.Stdin.Fd()) {
		return task.Patch{}, false, ErrNoTerminal
	}

	renderer := lipgloss.NewRenderer(os.Stdout)
	program := tea.NewProgram(newEditModel(item, renderer), tea.WithOutput(os.Stdout))
	result, err := program.Run()
	if err != nil {
		return task.Patch{}, false, err
	}

	final, ok := result.(editModel)
	if !ok || final.cancelled || !final.saved {
		return task.Patch{}, false, nil
	}
	return final.patch()
}

// patch reports only the fields that actually changed, so an untouched form
// leaves updated_at alone.
func (m editModel) patch() (task.Patch, bool, error) {
	content := strings.TrimSpace(m.content.Value())
	if content == "" {
		return task.Patch{}, false, fmt.Errorf("content is empty")
	}

	var patch task.Patch
	changed := false
	if content != m.original.Content {
		patch.Content = &content
		changed = true
	}
	if m.status != m.original.Status {
		status := m.status
		patch.Status = &status
		changed = true
	}
	if m.starred != m.original.IsStarred {
		starred := m.starred
		patch.Starred = &starred
		changed = true
	}
	return patch, changed, nil
}
