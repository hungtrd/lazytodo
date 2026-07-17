package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"
	"time"

	"github.com/hungtrd/lazytodo/internal/domain"
	"github.com/hungtrd/lazytodo/internal/repository"
	repofs "github.com/hungtrd/lazytodo/internal/repository/fs"
)

type taskJSON struct {
	ID        string `json:"id"`
	Content   string `json:"content"`
	Status    string `json:"status"`
	IsStarred bool   `json:"is_starred"`
	StartedAt int64  `json:"started_at,omitempty"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at,omitempty"`
}

func writeTasks(w io.Writer, tasks []domain.Task, asJSON bool) error {
	if asJSON {
		items := make([]taskJSON, len(tasks))
		for i, item := range tasks {
			items[i] = toTaskJSON(item)
		}
		return writeJSON(w, items)
	}
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tSTATUS\tSTAR\tCONTENT\tUPDATED")
	for _, item := range tasks {
		star := ""
		if item.IsStarred {
			star = "*"
		}
		updated := item.UpdatedAt
		if updated == 0 {
			updated = item.CreatedAt
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", item.Id, item.Status, star, item.Content, formatTime(updated))
	}
	return tw.Flush()
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
	fmt.Fprintf(w, "ID:      %s\n", item.Id)
	fmt.Fprintf(w, "Status:  %s\n", item.Status)
	fmt.Fprintf(w, "Starred: %t\n", item.IsStarred)
	fmt.Fprintf(w, "Content: %s\n", item.Content)
	fmt.Fprintf(w, "Created: %s\n", formatTime(item.CreatedAt))
	fmt.Fprintf(w, "Updated: %s\n", formatTime(item.UpdatedAt))
	return nil
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
	if asJSON {
		return writeJSON(w, map[string]any{
			"config_file":  configPath,
			"storage_root": cfg.StorageRoot,
			"tasks_file":   tasksPath,
			"vertical":     cfg.Vertical,
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
	return nil
}

func writeJSON(w io.Writer, value any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func toTaskJSON(item domain.Task) taskJSON {
	return taskJSON{
		ID:        item.Id,
		Content:   item.Content,
		Status:    item.Status.String(),
		IsStarred: item.IsStarred,
		StartedAt: item.StartedAt,
		CreatedAt: item.CreatedAt,
		UpdatedAt: item.UpdatedAt,
	}
}

func formatTime(timestamp int64) string {
	if timestamp == 0 {
		return "-"
	}
	return time.Unix(timestamp, 0).Local().Format("2006-01-02 15:04")
}
