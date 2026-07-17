package terminalstyle

import (
	"bytes"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/hungtrd/lazytodo/internal/domain"
	"github.com/muesli/termenv"
)

func TestThemeUsesBasicTerminalColors(t *testing.T) {
	renderer := lipgloss.NewRenderer(&bytes.Buffer{})
	renderer.SetColorProfile(termenv.ANSI)
	theme := WithRenderer(renderer)

	for name, testCase := range map[string]struct {
		rendered string
		sequence string
	}{
		"doing": {theme.Status(domain.TaskStatusDoing).Render("doing"), "\x1b[34m"},
		"done":  {theme.Status(domain.TaskStatusDone).Render("done"), "\x1b[32m"},
		"star":  {theme.Star().Render("*"), "\x1b[33m"},
	} {
		if !strings.Contains(testCase.rendered, testCase.sequence) {
			t.Fatalf("%s did not contain %q: %q", name, testCase.sequence, testCase.rendered)
		}
	}
	if got := theme.Status(domain.TaskStatusTodo).Render("todo"); got != "todo" {
		t.Fatalf("todo should use the terminal default color: %q", got)
	}
}

func TestThemeDetectsEnvironmentAndOutputCapabilities(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("CLICOLOR_FORCE", "")
	var output bytes.Buffer
	if theme := New(&output); theme.SupportsColor() {
		t.Fatal("a non-TTY output should not enable color")
	}

	t.Setenv("CLICOLOR_FORCE", "1")
	if theme := New(&output); !theme.SupportsColor() {
		t.Fatal("CLICOLOR_FORCE should enable ANSI color")
	}

	t.Setenv("NO_COLOR", "1")
	if theme := New(&output); theme.SupportsColor() {
		t.Fatal("NO_COLOR should take precedence over CLICOLOR_FORCE")
	}
}

func TestThemeOmitsColorForASCIIProfile(t *testing.T) {
	renderer := lipgloss.NewRenderer(&bytes.Buffer{})
	renderer.SetColorProfile(termenv.Ascii)
	theme := WithRenderer(renderer)

	for _, rendered := range []string{
		theme.Status(domain.TaskStatusDoing).Render("doing"),
		theme.Status(domain.TaskStatusDone).Render("done"),
		theme.Star().Render("*"),
	} {
		if strings.Contains(rendered, "\x1b[") {
			t.Fatalf("ASCII output contained ANSI: %q", rendered)
		}
	}
}
