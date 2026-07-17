package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/hungtrd/lazytodo/internal/domain"
	"github.com/hungtrd/lazytodo/internal/terminalstyle"
	"github.com/muesli/termenv"
)

func TestTaskTableColorsAndAlignment(t *testing.T) {
	tasks := []domain.Task{
		{Id: "1", Content: "plain", Status: domain.TaskStatusTodo, CreatedAt: 100},
		{Id: "20", Content: "active", Status: domain.TaskStatusDoing, IsStarred: true, CreatedAt: 100},
		{Id: "300", Content: "finished", Status: domain.TaskStatusDone, CreatedAt: 100},
	}

	var plain bytes.Buffer
	plainRenderer := lipgloss.NewRenderer(&plain)
	plainRenderer.SetColorProfile(termenv.Ascii)
	if err := writeTaskTable(&plain, tasks, terminalstyle.WithRenderer(plainRenderer)); err != nil {
		t.Fatal(err)
	}

	var colored bytes.Buffer
	colorRenderer := lipgloss.NewRenderer(&colored)
	colorRenderer.SetColorProfile(termenv.ANSI)
	if err := writeTaskTable(&colored, tasks, terminalstyle.WithRenderer(colorRenderer)); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(colored.String(), "\x1b[34mdoing") {
		t.Fatalf("doing status is not blue: %q", colored.String())
	}
	if !strings.Contains(colored.String(), "\x1b[32mdone") {
		t.Fatalf("done status is not green: %q", colored.String())
	}
	if !strings.Contains(colored.String(), "\x1b[33m*") {
		t.Fatalf("star is not yellow: %q", colored.String())
	}
	if stripped := ansi.Strip(colored.String()); stripped != plain.String() {
		t.Fatalf("ANSI changed table layout:\ncolored: %q\nplain:   %q", stripped, plain.String())
	}
}

func TestJSONOutputNeverContainsANSI(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("CLICOLOR_FORCE", "1")
	var output bytes.Buffer
	err := writeTasks(&output, []domain.Task{{Id: "1", Content: "active", Status: domain.TaskStatusDoing}}, true)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), "\x1b[") {
		t.Fatalf("JSON contains ANSI: %q", output.String())
	}
}

func TestTaskDetailColorsStatusAndStar(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("CLICOLOR_FORCE", "1")
	var output bytes.Buffer
	item := domain.Task{Id: "1", Content: "active", Status: domain.TaskStatusDoing, IsStarred: true}
	if err := writeTaskDetail(&output, item, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "Status:  \x1b[34mdoing") {
		t.Fatalf("detail status is not blue: %q", output.String())
	}
	if !strings.Contains(output.String(), "Starred: \x1b[33mtrue") {
		t.Fatalf("detail starred value is not yellow: %q", output.String())
	}
}
