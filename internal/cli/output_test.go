package cli

import (
	"bytes"
	"strings"
	"testing"
	"unicode/utf8"

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

func TestTruncateContent(t *testing.T) {
	for _, tc := range []struct {
		name    string
		content string
		limit   int
		want    string
	}{
		{name: "short content is untouched", content: "Buy milk", limit: 40, want: "Buy milk"},
		{
			name:    "content stops at the first line",
			content: "Buy milk\nand oat milk\nand bread",
			limit:   40,
			want:    "Buy milk ...",
		},
		{
			// A first line that already fills the column must not gain an
			// ellipsis that pushes it past the budget.
			name:    "multiline and overlong is cut once",
			content: "the first line is quite long indeed\nsecond",
			limit:   20,
			want:    "the first line i" + contentEllipsis,
		},
		{
			// The ellipsis is inside the budget, not added on top of it.
			name:    "long single line is cut to the limit",
			content: strings.Repeat("a", 50),
			limit:   20,
			want:    strings.Repeat("a", 16) + contentEllipsis,
		},
		{
			name:    "trailing whitespace before a newline is dropped",
			content: "Buy milk   \nmore",
			limit:   40,
			want:    "Buy milk ...",
		},
		{
			name:    "carriage returns count as line breaks",
			content: "Buy milk\r\nmore",
			limit:   40,
			want:    "Buy milk ...",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := truncateContent(tc.content, tc.limit)
			if got != tc.want {
				t.Fatalf("truncateContent(%q, %d) = %q, want %q", tc.content, tc.limit, got, tc.want)
			}
			if width := lipgloss.Width(got); width > tc.limit {
				t.Fatalf("result is %d cells wide, over the %d limit: %q", width, tc.limit, got)
			}
		})
	}
}

// Diacritics and wide characters must never be cut in half.
func TestTruncateContentIsGraphemeAware(t *testing.T) {
	for _, content := range []string{
		"viết tài liệu cho phần đồng bộ git và kiểm tra lại",
		"日本語のタスクをここに書いておく、とても長い行です",
	} {
		got := truncateContent(content, 20)
		if width := lipgloss.Width(got); width > 20 {
			t.Fatalf("%q truncated to %d cells, over the limit: %q", content, width, got)
		}
		if !strings.HasSuffix(got, contentEllipsis) {
			t.Fatalf("%q was not marked as truncated: %q", content, got)
		}
		if !utf8.ValidString(got) {
			t.Fatalf("truncation produced invalid UTF-8: %q", got)
		}
	}
}

// The table is the scannable view; a multi-line task must not break its rows.
func TestTaskTableKeepsOneRowPerTask(t *testing.T) {
	tasks := []domain.Task{
		{Id: "1", Content: "short", Status: domain.TaskStatusTodo, CreatedAt: 100},
		{Id: "2", Content: "first line\nsecond line", Status: domain.TaskStatusTodo, CreatedAt: 100},
		{Id: "3", Content: strings.Repeat("long ", 60), Status: domain.TaskStatusTodo, CreatedAt: 100},
	}
	var output bytes.Buffer
	renderer := lipgloss.NewRenderer(&output)
	renderer.SetColorProfile(termenv.Ascii)
	if err := writeTaskTable(&output, tasks, terminalstyle.WithRenderer(renderer)); err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimRight(output.String(), "\n"), "\n")
	if len(lines) != len(tasks)+1 {
		t.Fatalf("got %d lines, want %d (header + one per task):\n%s", len(lines), len(tasks)+1, output.String())
	}
	if strings.Contains(output.String(), "second line") {
		t.Fatalf("the second line leaked into the table:\n%s", output.String())
	}
	for _, line := range lines[2:] {
		if !strings.Contains(line, contentEllipsis) {
			t.Fatalf("truncated row is not marked with an ellipsis: %q", line)
		}
	}
	// No row may exceed the budget, which is what keeps the table from wrapping.
	for _, line := range lines {
		if width := lipgloss.Width(line); width > fallbackTableWidth {
			t.Fatalf("row is %d cells wide, over the %d budget:\n%s", width, fallbackTableWidth, output.String())
		}
	}
	// Truncating must not shift the column: every row starts its content at
	// the same offset as the header.
	contentStart := strings.Index(lines[0], "CONTENT")
	for i, marker := range []string{"short", "first line", "long long"} {
		if got := strings.Index(lines[i+1], marker); got != contentStart {
			t.Fatalf("row %d starts content at %d, want %d:\n%s", i+1, got, contentStart, output.String())
		}
	}
}

// show is the escape hatch the table points at, so it must print everything.
func TestTaskDetailShowsFullMultilineContent(t *testing.T) {
	var output bytes.Buffer
	item := domain.Task{Id: "1", Content: "first line\nsecond line", Status: domain.TaskStatusTodo}
	if err := writeTaskDetail(&output, item, false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "Content: first line\n         second line\n") {
		t.Fatalf("detail did not print the full content aligned: %q", output.String())
	}
	if strings.Contains(output.String(), contentEllipsis) {
		t.Fatalf("detail truncated the content: %q", output.String())
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
