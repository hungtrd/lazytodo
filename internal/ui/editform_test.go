package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hungtrd/lazytodo/internal/domain"
)

func newTestEditModel(item domain.Task) editModel {
	return newEditModel(item, lipgloss.NewRenderer(nil))
}

func send(m editModel, keys ...string) editModel {
	for _, key := range keys {
		var msg tea.Msg
		switch key {
		case "tab":
			msg = tea.KeyMsg{Type: tea.KeyTab}
		case "shift+tab":
			msg = tea.KeyMsg{Type: tea.KeyShiftTab}
		case "ctrl+s":
			msg = tea.KeyMsg{Type: tea.KeyCtrlS}
		case "esc":
			msg = tea.KeyMsg{Type: tea.KeyEsc}
		case "enter":
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		case " ":
			msg = tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}}
		case "left":
			msg = tea.KeyMsg{Type: tea.KeyLeft}
		case "right":
			msg = tea.KeyMsg{Type: tea.KeyRight}
		default:
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
		}
		next, _ := m.Update(msg)
		m = next.(editModel)
	}
	return m
}

func TestEditFormProducesPatchForChangedFieldsOnly(t *testing.T) {
	item := domain.Task{Id: "1", Content: "original", Status: domain.TaskStatusTodo}
	m := send(newTestEditModel(item), "tab", "right", "tab", " ", "ctrl+s")

	if !m.saved {
		t.Fatal("ctrl+s did not save")
	}
	patch, changed, err := m.patch()
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("changed = false, want true")
	}
	if patch.Content != nil {
		t.Fatalf("content was reported as changed: %q", *patch.Content)
	}
	if patch.Status == nil || *patch.Status != domain.TaskStatusDoing {
		t.Fatalf("status patch = %v, want doing", patch.Status)
	}
	if patch.Starred == nil || !*patch.Starred {
		t.Fatalf("starred patch = %v, want true", patch.Starred)
	}
}

func TestEditFormReportsNoChangeWhenUntouched(t *testing.T) {
	item := domain.Task{Id: "1", Content: "original", Status: domain.TaskStatusDoing, IsStarred: true}
	_, changed, err := send(newTestEditModel(item), "ctrl+s").patch()
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Fatal("an untouched form reported a change")
	}
}

func TestEditFormCancels(t *testing.T) {
	m := send(newTestEditModel(domain.Task{Id: "1", Content: "original"}), "esc")
	if !m.cancelled || m.saved {
		t.Fatalf("esc did not cancel: cancelled=%t saved=%t", m.cancelled, m.saved)
	}
}

// The content area has to keep every printable key, including enter, so that
// multi-line editing works.
func TestEditFormContentFieldConsumesTypingAndEnter(t *testing.T) {
	m := send(newTestEditModel(domain.Task{Id: "1", Content: "line"}), "enter", "x")
	if m.saved {
		t.Fatal("enter saved while the content field was focused")
	}
	if got := m.content.Value(); !strings.Contains(got, "\n") || !strings.HasSuffix(got, "x") {
		t.Fatalf("content = %q, want the newline and typed rune kept", got)
	}
}

// Enter is a shortcut for save once focus has left the content area.
func TestEditFormEnterSavesOutsideContent(t *testing.T) {
	m := send(newTestEditModel(domain.Task{Id: "1", Content: "line"}), "tab", "enter")
	if !m.saved {
		t.Fatal("enter did not save from the status field")
	}
}

func TestEditFormRejectsEmptyContent(t *testing.T) {
	m := newTestEditModel(domain.Task{Id: "1", Content: "original"})
	m.content.SetValue("   ")
	if _, _, err := m.patch(); err == nil {
		t.Fatal("expected an empty-content error")
	}
}

func TestEditFormStatusCyclesThroughArchived(t *testing.T) {
	m := newTestEditModel(domain.Task{Id: "1", Content: "x", Status: domain.TaskStatusDone})
	m = send(m, "tab", "right")
	if m.status != domain.TaskStatusArchived {
		t.Fatalf("status = %v, want archived", m.status)
	}
	// The cycle clamps rather than wrapping, so a stray key cannot flip a task
	// from archived back to todo unexpectedly.
	if got := send(m, "right").status; got != domain.TaskStatusArchived {
		t.Fatalf("status = %v, want it to stay at archived", got)
	}
	if got := send(m, "left", "left", "left", "left").status; got != domain.TaskStatusTodo {
		t.Fatalf("status = %v, want todo", got)
	}
}
