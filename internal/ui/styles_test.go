package ui

import (
	"bytes"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/hungtrd/lazytodo/internal/domain"
	"github.com/muesli/termenv"
)

func TestUIStylesRenderStatusAndStarColors(t *testing.T) {
	renderer := lipgloss.NewRenderer(&bytes.Buffer{})
	renderer.SetColorProfile(termenv.ANSI)
	styles := newUIStyles(renderer)

	for name, rendered := range map[string]string{
		"doing header": styles.renderHeader(domain.TaskStatusDoing, "Doing", 1),
		"doing task":   styles.taskStyle(domain.TaskStatusDoing, false).Render("active"),
		"done header":  styles.renderHeader(domain.TaskStatusDone, "Done", 1),
		"done task":    styles.taskStyle(domain.TaskStatusDone, false).Render("finished"),
		"star":         styles.theme.Star().Render("★"),
	} {
		if !strings.Contains(rendered, "\x1b[") {
			t.Fatalf("%s did not contain ANSI styling: %q", name, rendered)
		}
	}
	selectedStyle := styles.taskStyle(domain.TaskStatusDoing, true)
	if !selectedStyle.GetBold() || !selectedStyle.GetReverse() {
		t.Fatal("selected task should be bold and reverse")
	}
	if selected := selectedStyle.Render("active"); !strings.Contains(selected, "\x1b[") || ansi.Strip(selected) != "active" {
		t.Fatalf("selected task lost styling or content: %q", selected)
	}
	if !styles.taskStyle(domain.TaskStatusDone, false).GetStrikethrough() {
		t.Fatal("done task should remain strikethrough")
	}
	if todo := styles.taskStyle(domain.TaskStatusTodo, false).Render("plain"); todo != "plain" {
		t.Fatalf("todo should retain the default color: %q", todo)
	}
}

func TestUIStylesOmitANSIForASCIIProfile(t *testing.T) {
	renderer := lipgloss.NewRenderer(&bytes.Buffer{})
	renderer.SetColorProfile(termenv.Ascii)
	styles := newUIStyles(renderer)

	for _, rendered := range []string{
		styles.renderHeader(domain.TaskStatusDoing, "Doing", 1),
		styles.taskStyle(domain.TaskStatusDone, false).Render("finished"),
		styles.theme.Star().Render("★"),
	} {
		if strings.Contains(rendered, "\x1b[") {
			t.Fatalf("ASCII profile emitted ANSI: %q", rendered)
		}
	}
}
