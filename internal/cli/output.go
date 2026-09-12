package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/term"
	"github.com/hungtrd/lazytodo/internal/domain"
	"github.com/hungtrd/lazytodo/internal/repository"
	repofs "github.com/hungtrd/lazytodo/internal/repository/fs"
	"github.com/hungtrd/lazytodo/internal/terminalstyle"
)

type taskJSON struct {
	ID         string `json:"id"`
	Content    string `json:"content"`
	Status     string `json:"status"`
	IsStarred  bool   `json:"is_starred"`
	StartedAt  int64  `json:"started_at,omitempty"`
	CreatedAt  int64  `json:"created_at"`
	UpdatedAt  int64  `json:"updated_at,omitempty"`
	ArchivedAt int64  `json:"archived_at,omitempty"`
}

type taskTableRow struct {
	cells   [5]string
	status  domain.TaskStatus
	starred bool
}

const (
	// contentColumn is the index of CONTENT in taskTableRow.cells; it is the
	// only column that gets truncated.
	contentColumn = 3
	// columnGap is the padding writeTaskTable puts between columns.
	columnGap = 2

	// contentEllipsis marks content the table had to cut short. Users get the
	// whole thing from `lazytodo show <id>`.
	contentEllipsis = " ..."

	// fallbackTableWidth applies when output is piped and there is no terminal
	// to measure.
	fallbackTableWidth = 100
	// minContentWidth keeps the column readable on a very narrow terminal even
	// if that means the row wraps.
	minContentWidth = 20
)

func writeTasks(w io.Writer, tasks []domain.Task, asJSON bool) error {
	if asJSON {
		items := make([]taskJSON, len(tasks))
		for i, item := range tasks {
			items[i] = toTaskJSON(item)
		}
		return writeJSON(w, items)
	}
	return writeTaskTable(w, tasks, terminalstyle.New(w))
}

func writeTaskTable(w io.Writer, tasks []domain.Task, theme terminalstyle.Theme) error {
	rows := make([]taskTableRow, 0, len(tasks)+1)
	rows = append(rows, taskTableRow{cells: [5]string{"ID", "STATUS", "STAR", "CONTENT", "UPDATED"}})
	for _, item := range tasks {
		star := ""
		if item.IsStarred {
			star = "*"
		}
		updated := item.UpdatedAt
		if updated == 0 {
			updated = item.CreatedAt
		}
		rows = append(rows, taskTableRow{
			cells:   [5]string{item.Id, item.Status.String(), star, item.Content, formatTime(updated)},
			status:  item.Status,
			starred: item.IsStarred,
		})
	}

	// Content is truncated to whatever the other columns leave over, so a long
	// or multi-line task cannot wreck the table. The widths of the remaining
	// columns do not depend on it, so they can be measured first.
	widths := [5]int{}
	for _, row := range rows {
		for column, cell := range row.cells {
			if column == contentColumn {
				continue
			}
			widths[column] = max(widths[column], lipgloss.Width(cell))
		}
	}
	budget := contentBudget(w, widths)
	for i := range rows {
		rows[i].cells[contentColumn] = truncateContent(rows[i].cells[contentColumn], budget)
		widths[contentColumn] = max(widths[contentColumn], lipgloss.Width(rows[i].cells[contentColumn]))
	}
	for rowIndex, row := range rows {
		for column, cell := range row.cells {
			rendered := cell
			if rowIndex > 0 {
				switch column {
				case 1:
					rendered = theme.Status(row.status).Render(cell)
				case 2:
					if row.starred {
						rendered = theme.Star().Render(cell)
					}
				}
			}
			if _, err := fmt.Fprint(w, rendered); err != nil {
				return err
			}
			if column < len(row.cells)-1 {
				padding := widths[column] - lipgloss.Width(cell) + 2
				if _, err := fmt.Fprint(w, strings.Repeat(" ", padding)); err != nil {
					return err
				}
			}
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
	}
	return nil
}

// contentBudget returns how many cells the CONTENT column may occupy, given
// the measured widths of the other columns.
func contentBudget(w io.Writer, widths [5]int) int {
	total := fallbackTableWidth
	if file, ok := w.(*os.File); ok {
		if width, _, err := term.GetSize(file.Fd()); err == nil && width > 0 {
			total = width
		}
	}
	used := columnGap * (len(widths) - 1)
	for column, width := range widths {
		if column != contentColumn {
			used += width
		}
	}
	return max(total-used, minContentWidth)
}

// truncateContent renders task content as a single table cell. Anything past
// the first line, or past the column budget, is replaced with an ellipsis
// pointing the user at `lazytodo show`.
func truncateContent(content string, limit int) string {
	firstLine := content
	multiline := false
	if index := strings.IndexAny(content, "\r\n"); index >= 0 {
		firstLine = strings.TrimRight(content[:index], " \t")
		multiline = true
	}
	if lipgloss.Width(firstLine)+len(contentEllipsis) > limit {
		// Truncate is grapheme aware, so Vietnamese diacritics and wide
		// characters are never cut in half, and it budgets for the tail.
		return ansi.Truncate(firstLine, limit, contentEllipsis)
	}
	if multiline {
		return firstLine + contentEllipsis
	}
	return firstLine
}

func writeTask(w io.Writer, item domain.Task, asJSON bool) error {
	if asJSON {
		return writeJSON(w, toTaskJSON(item))
	}
	return writeTasks(w, []domain.Task{item}, false)
}

func writeTaskDetail(w io.Writer, item domain.Task, asJSON bool) error {
	if asJSON {
		return writeJSON(w, toTaskJSON(item))
	}
	theme := terminalstyle.New(w)
	status := theme.Status(item.Status).Render(item.Status.String())
	starred := fmt.Sprintf("%t", item.IsStarred)
	if item.IsStarred {
		starred = theme.Star().Render(starred)
	}
	fmt.Fprintf(w, "ID:      %s\n", item.Id)
	fmt.Fprintf(w, "Status:  %s\n", status)
	fmt.Fprintf(w, "Starred: %s\n", starred)
	// show is where the full text lives, so multi-line content is printed
	// whole, with continuation lines aligned under the first.
	fmt.Fprintf(w, "Content: %s\n", indentContinuation(item.Content, "         "))
	fmt.Fprintf(w, "Created: %s\n", formatTime(item.CreatedAt))
	fmt.Fprintf(w, "Updated: %s\n", formatTime(item.UpdatedAt))
	if item.ArchivedAt != 0 {
		fmt.Fprintf(w, "Archived: %s\n", formatTime(item.ArchivedAt))
	}
	return nil
}

// indentContinuation aligns every line after the first under the label.
func indentContinuation(text, indent string) string {
	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	return strings.ReplaceAll(normalized, "\n", "\n"+indent)
}

func writeConfig(w io.Writer, cfg repository.Config, asJSON bool) error {
	configPath, err := repofs.ConfigFilePath()
	if err != nil {
		return err
	}
	tasksPath, err := repofs.TasksFilePath(cfg)
	if err != nil {
		return err
	}
	// Sync details never include the token: it lives only in the environment.
	syncEnabled := cfg.Git != nil && cfg.Git.Enabled
	if asJSON {
		return writeJSON(w, map[string]any{
			"config_file":  configPath,
			"storage_root": cfg.StorageRoot,
			"tasks_file":   tasksPath,
			"vertical":     cfg.Vertical,
			"sync_enabled": syncEnabled,
		})
	}
	fmt.Fprintf(w, "Config file:  %s\n", configPath)
	if cfg.StorageRoot == "" {
		fmt.Fprintln(w, "Storage root: (default)")
	} else {
		fmt.Fprintf(w, "Storage root: %s\n", cfg.StorageRoot)
	}
	fmt.Fprintf(w, "Tasks file:   %s\n", tasksPath)
	fmt.Fprintf(w, "Vertical UI:  %t\n", cfg.Vertical)
	fmt.Fprintf(w, "Git sync:     %t\n", syncEnabled)
	return nil
}

func writeJSON(w io.Writer, value any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func toTaskJSON(item domain.Task) taskJSON {
	return taskJSON{
		ID:         item.Id,
		Content:    item.Content,
		Status:     item.Status.String(),
		IsStarred:  item.IsStarred,
		StartedAt:  item.StartedAt,
		CreatedAt:  item.CreatedAt,
		UpdatedAt:  item.UpdatedAt,
		ArchivedAt: item.ArchivedAt,
	}
}

func formatTime(timestamp int64) string {
	if timestamp == 0 {
		return "-"
	}
	return time.Unix(timestamp, 0).Local().Format("2006-01-02 15:04")
}
