package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hungtrd/lazytodo/internal/domain"
	"github.com/hungtrd/lazytodo/internal/repository"
	"github.com/hungtrd/lazytodo/internal/task"
)

type testConfigRepo struct{ cfg repository.Config }

func (r *testConfigRepo) Load() (repository.Config, error) { return r.cfg, nil }
func (r *testConfigRepo) Save(cfg repository.Config) error {
	r.cfg = cfg
	return nil
}

func executeCLI(t *testing.T, cfg *testConfigRepo, args ...string) string {
	t.Helper()
	var output bytes.Buffer
	root := NewRootCommand(Dependencies{
		ConfigRepo: cfg,
		In:         strings.NewReader("yes\n"),
		Out:        &output,
		Err:        &output,
	})
	root.SetArgs(args)
	if err := root.Execute(); err != nil {
		t.Fatalf("execute %v: %v\n%s", args, err, output.String())
	}
	return output.String()
}

func TestCLIWorkflowAndJSONOutput(t *testing.T) {
	root := t.TempDir()
	cfg := &testConfigRepo{cfg: repository.Config{StorageRoot: root}}

	created := executeCLI(t, cfg, "create", "Buy milk", "--json")
	var item map[string]any
	if err := json.Unmarshal([]byte(created), &item); err != nil {
		t.Fatalf("decode create output: %v", err)
	}
	if item["id"] != "1" || item["status"] != "todo" {
		t.Fatalf("unexpected create output: %v", item)
	}

	listed := executeCLI(t, cfg, "list")
	if !strings.Contains(listed, "ID") || !strings.Contains(listed, "Buy milk") {
		t.Fatalf("unexpected list output: %s", listed)
	}

	executeCLI(t, cfg, "edit", "1", "-s", "done", "--content", "Buy oat milk")
	searched := executeCLI(t, cfg, "search", "OAT", "--json")
	if !strings.Contains(searched, `"status": "done"`) {
		t.Fatalf("unexpected search output: %s", searched)
	}

	executeCLI(t, cfg, "delete", "1", "--yes")
	if _, err := os.Stat(filepath.Join(root, "lazytodo", "tasks.jsonl")); err != nil {
		t.Fatalf("tasks file was not created: %v", err)
	}
	listed = executeCLI(t, cfg, "list", "--json")
	if strings.TrimSpace(listed) != "[]" {
		t.Fatalf("list after delete = %s", listed)
	}
}

// delete archives rather than destroys, so the task is recoverable until the
// user explicitly purges it.
func TestCLIArchiveRestorePurge(t *testing.T) {
	cfg := &testConfigRepo{cfg: repository.Config{StorageRoot: t.TempDir()}}
	executeCLI(t, cfg, "create", "Buy milk")

	archived := executeCLI(t, cfg, "delete", "1", "--yes")
	if !strings.Contains(archived, "Archived task 1") {
		t.Fatalf("unexpected delete output: %s", archived)
	}
	if listed := executeCLI(t, cfg, "list", "--json"); strings.TrimSpace(listed) != "[]" {
		t.Fatalf("archived task leaked into the default list: %s", listed)
	}
	if listed := executeCLI(t, cfg, "list", "--status", "archived", "--json"); !strings.Contains(listed, "Buy milk") {
		t.Fatalf("archived task is not listed with --status archived: %s", listed)
	}

	executeCLI(t, cfg, "restore", "1")
	if listed := executeCLI(t, cfg, "list", "--json"); !strings.Contains(listed, `"status": "todo"`) {
		t.Fatalf("task was not restored to todo: %s", listed)
	}

	executeCLI(t, cfg, "delete", "1", "--yes")
	purged := executeCLI(t, cfg, "purge", "--all", "--yes", "--json")
	if !strings.Contains(purged, `"purged_count": 1`) {
		t.Fatalf("unexpected purge output: %s", purged)
	}
	if listed := executeCLI(t, cfg, "list", "--status", "archived", "--json"); strings.TrimSpace(listed) != "[]" {
		t.Fatalf("archived tasks remain after purge --all: %s", listed)
	}
}

// Field flags must keep the old non-interactive behaviour, because scripts and
// agents rely on it and cannot answer a form.
func TestCLIEditUsesFormOnlyWithoutFieldFlags(t *testing.T) {
	cfg := &testConfigRepo{cfg: repository.Config{StorageRoot: t.TempDir()}}
	executeCLI(t, cfg, "create", "Buy milk")

	calls := 0
	runForm := func(item domain.Task) (task.Patch, bool, error) {
		calls++
		content := "edited in the form"
		status := domain.TaskStatusDoing
		return task.Patch{Content: &content, Status: &status}, true, nil
	}
	run := func(t *testing.T, form EditFormRunner, args ...string) string {
		t.Helper()
		var output bytes.Buffer
		root := NewRootCommand(Dependencies{ConfigRepo: cfg, RunEditForm: form, Out: &output, Err: &output})
		root.SetArgs(args)
		if err := root.Execute(); err != nil {
			t.Fatalf("execute %v: %v\n%s", args, err, output.String())
		}
		return output.String()
	}

	run(t, runForm, "edit", "1", "--content", "edited by flag")
	if calls != 0 {
		t.Fatal("the form was opened even though a field flag was given")
	}

	run(t, runForm, "edit", "1")
	if calls != 1 {
		t.Fatalf("form was called %d times, want 1", calls)
	}
	listed := executeCLI(t, cfg, "list", "--json")
	if !strings.Contains(listed, "edited in the form") || !strings.Contains(listed, `"status": "doing"`) {
		t.Fatalf("form edit was not applied: %s", listed)
	}

	// Cancelling leaves the task untouched.
	cancelled := run(t, func(domain.Task) (task.Patch, bool, error) {
		return task.Patch{}, false, nil
	}, "edit", "1")
	if !strings.Contains(cancelled, "No changes.") {
		t.Fatalf("unexpected output when the form was cancelled: %s", cancelled)
	}
}

func TestCLIPurgeRequiresIDOrAll(t *testing.T) {
	cfg := &testConfigRepo{cfg: repository.Config{StorageRoot: t.TempDir()}}
	for _, args := range [][]string{
		{"purge"},
		{"purge", "1", "--all"},
	} {
		root := NewRootCommand(Dependencies{ConfigRepo: cfg, Out: &bytes.Buffer{}, Err: &bytes.Buffer{}})
		root.SetArgs(args)
		if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "either an id or --all") {
			t.Fatalf("purge %v error = %v, want an id/--all conflict", args, err)
		}
	}
}

func TestRootShowsHelpAndRejectsUIWithCommand(t *testing.T) {
	cfg := &testConfigRepo{cfg: repository.Config{StorageRoot: t.TempDir()}}
	output := executeCLI(t, cfg)
	if !strings.Contains(output, "Usage:") {
		t.Fatalf("root did not show help: %s", output)
	}
	for _, description := range []string{
		"create (add, new)",
		"delete (del, rm)",
		"list (ls)",
		"show (detail)",
	} {
		if !strings.Contains(output, description) {
			t.Fatalf("root help is missing %q: %s", description, output)
		}
	}

	root := NewRootCommand(Dependencies{ConfigRepo: cfg})
	root.SetArgs([]string{"list", "--ui"})
	if err := root.Execute(); err == nil || !strings.Contains(err.Error(), "cannot be combined") {
		t.Fatalf("unexpected --ui/subcommand result: %v", err)
	}
}

func TestUIRunsOnlyWithFlag(t *testing.T) {
	cfg := &testConfigRepo{cfg: repository.Config{StorageRoot: t.TempDir()}}
	called := false
	root := NewRootCommand(Dependencies{
		ConfigRepo: cfg,
		RunUI: func(_ *task.Service) error {
			called = true
			return nil
		},
	})
	root.SetArgs([]string{"--ui"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("UI runner was not called")
	}
}
