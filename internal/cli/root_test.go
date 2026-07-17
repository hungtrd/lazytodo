package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

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

	executeCLI(t, cfg, "edit", "1", "--status", "done", "--content", "Buy oat milk")
	searched := executeCLI(t, cfg, "search", "OAT", "--json")
	if !strings.Contains(searched, `"status": "done"`) {
		t.Fatalf("unexpected search output: %s", searched)
	}

	executeCLI(t, cfg, "delete", "1", "--yes")
	if _, err := os.Stat(filepath.Join(root, "lazytodo", "tasks.json")); err != nil {
		t.Fatalf("tasks file was not created: %v", err)
	}
	listed = executeCLI(t, cfg, "list", "--json")
	if strings.TrimSpace(listed) != "[]" {
		t.Fatalf("list after delete = %s", listed)
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
