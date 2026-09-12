package fs

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hungtrd/lazytodo/internal/domain"
	"github.com/hungtrd/lazytodo/internal/repository"
)

// writeLegacy drops a pre-v4 tasks.json beside the tasks.jsonl the store will
// use, which is what triggers the migration on first load.
func writeLegacy(t *testing.T, contents string) (tasksPath, legacyPath string) {
	t.Helper()
	dir := t.TempDir()
	tasksPath = filepath.Join(dir, "tasks.jsonl")
	legacyPath = filepath.Join(dir, "tasks.json")
	if err := os.WriteFile(legacyPath, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	return tasksPath, legacyPath
}

func TestTaskStoreMigratesLegacyDataToSequentialIDs(t *testing.T) {
	tasksPath, legacyPath := writeLegacy(t, `{
		"0": [{"Id":"1740000000000000000","Content":"first","Status":0,"IsStarred":true,"CreatedAt":100}],
		"2": [{"Id":"1740000000000000001","Content":"second","Status":2}]
	}`)

	loaded, err := NewTaskStoreAt(tasksPath).Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Version != repository.CurrentTaskDataVersion || loaded.NextID != 3 {
		t.Fatalf("unexpected migrated metadata: %+v", loaded)
	}
	if got := loaded.Tasks[domain.TaskStatusTodo][0].Id; got != "1" {
		t.Fatalf("todo ID = %q, want 1", got)
	}
	if item := loaded.Tasks[domain.TaskStatusTodo][0]; !item.IsStarred || item.CreatedAt != 100 {
		t.Fatalf("legacy fields were not preserved: %+v", item)
	}
	if got := loaded.Tasks[domain.TaskStatusDone][0].Id; got != "2" {
		t.Fatalf("done ID = %q, want 2", got)
	}

	if _, err := os.Stat(legacyPath + ".v1.bak"); err != nil {
		t.Fatalf("legacy backup was not created: %v", err)
	}
	// The old file is removed so there is a single source of truth afterwards.
	if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
		t.Fatalf("legacy file still exists after migration: %v", err)
	}
	assertJSONL(t, tasksPath, 2)
}

func TestTaskStoreMigratesVersion2StatusesToDoing(t *testing.T) {
	tasksPath, legacyPath := writeLegacy(t, `{
		"version": 2,
		"next_id": 8,
		"tasks": {
			"0": [],
			"1": [{"id":"7","content":"active task","status":1,"created_at":100}],
			"2": []
		}
	}`)

	loaded, err := NewTaskStoreAt(tasksPath).Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Version != repository.CurrentTaskDataVersion {
		t.Fatalf("version = %d, want %d", loaded.Version, repository.CurrentTaskDataVersion)
	}
	if tasks := loaded.Tasks[domain.TaskStatusDoing]; len(tasks) != 1 || tasks[0].Status != domain.TaskStatusDoing {
		t.Fatalf("doing tasks were not migrated: %+v", tasks)
	}
	persisted, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(persisted), `"doing"`) {
		t.Fatalf("persisted data does not use doing status: %s", persisted)
	}
	if _, err := os.Stat(legacyPath + ".v2.bak"); err != nil {
		t.Fatalf("version 2 backup was not created: %v", err)
	}
}

func TestTaskStoreMigratesVersion3JSONToJSONL(t *testing.T) {
	tasksPath, legacyPath := writeLegacy(t, `{
		"version": 3,
		"next_id": 4,
		"tasks": {
			"todo": [{"id":"3","content":"write docs","status":"todo","created_at":300}],
			"doing": [],
			"done": [{"id":"1","content":"ship it","status":"done","is_starred":true,"created_at":100}]
		}
	}`)

	loaded, err := NewTaskStoreAt(tasksPath).Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Version != repository.CurrentTaskDataVersion || loaded.NextID != 4 {
		t.Fatalf("unexpected migrated metadata: %+v", loaded)
	}
	if got := loaded.Tasks[domain.TaskStatusDone][0].Content; got != "ship it" {
		t.Fatalf("done content = %q", got)
	}
	// v4 adds the archived bucket even though the source had no such key.
	if loaded.Tasks[domain.TaskStatusArchived] == nil {
		t.Fatal("archived bucket is missing after migration")
	}
	if _, err := os.Stat(legacyPath + ".v3.bak"); err != nil {
		t.Fatalf("version 3 backup was not created: %v", err)
	}
	assertJSONL(t, tasksPath, 2)
}

func TestTaskStoreRejectsUnknownVersion(t *testing.T) {
	tasksPath, _ := writeLegacy(t, `{"version":99,"next_id":1,"tasks":{}}`)
	if _, err := NewTaskStoreAt(tasksPath).Load(); err == nil {
		t.Fatal("expected unsupported version error")
	}
}

func TestTaskStoreRoundTripsJSONLAndIsStable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tasks.jsonl")
	store := NewTaskStoreAt(path)

	data := repository.TaskData{
		Version: repository.CurrentTaskDataVersion,
		NextID:  4,
		Tasks: map[domain.TaskStatus][]domain.Task{
			// Deliberately out of id order: the writer sorts, so that editing
			// one task only touches one line of the git diff.
			domain.TaskStatusTodo:     {{Id: "3", Content: "third", CreatedAt: 300}},
			domain.TaskStatusDone:     {{Id: "1", Content: "first", CreatedAt: 100, IsStarred: true}},
			domain.TaskStatusArchived: {{Id: "2", Content: "second", CreatedAt: 200, ArchivedAt: 250}},
		},
	}
	if err := store.Save(data); err != nil {
		t.Fatal(err)
	}
	first, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.NextID != 4 {
		t.Fatalf("NextID = %d, want 4", loaded.NextID)
	}
	archived := loaded.Tasks[domain.TaskStatusArchived]
	if len(archived) != 1 || archived[0].ArchivedAt != 250 {
		t.Fatalf("archived task did not survive the round trip: %+v", archived)
	}

	if err := store.Save(loaded); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("save is not stable:\nfirst:\n%s\nsecond:\n%s", first, second)
	}

	lines := splitLines(t, first)
	var header fileHeader
	if err := json.Unmarshal(lines[0], &header); err != nil {
		t.Fatalf("first line is not a header: %v", err)
	}
	if header.Version != repository.CurrentTaskDataVersion || header.NextID != 4 {
		t.Fatalf("unexpected header: %+v", header)
	}
	for i, want := range []string{"1", "2", "3"} {
		var item domain.Task
		if err := json.Unmarshal(lines[i+1], &item); err != nil {
			t.Fatal(err)
		}
		if item.Id != want {
			t.Fatalf("line %d id = %q, want %q", i+1, item.Id, want)
		}
	}
}

// A hand-edited file that lost its header should still load rather than fail,
// since the tasks file lives in a git repo that people will edit directly.
func TestTaskStoreReadsHeaderlessFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tasks.jsonl")
	contents := `{"id":"5","content":"no header","status":"todo","created_at":1}` + "\n"
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
	loaded, err := NewTaskStoreAt(path).Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Tasks[domain.TaskStatusTodo]) != 1 {
		t.Fatalf("task was not read: %+v", loaded.Tasks)
	}
	if loaded.NextID != 6 {
		t.Fatalf("NextID = %d, want 6 (derived from the highest id)", loaded.NextID)
	}
}

func TestTaskStoreReturnsEmptyDataWhenNothingExists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tasks.jsonl")
	loaded, err := NewTaskStoreAt(path).Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.NextID != 1 || len(loaded.Tasks) != len(domain.AllStatuses()) {
		t.Fatalf("unexpected empty data: %+v", loaded)
	}
}

func TestTasksFilePathUsesCustomSubdirectory(t *testing.T) {
	root := t.TempDir()
	path, err := TasksFilePath(repository.Config{StorageRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "lazytodo", "tasks.jsonl")
	if path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}
}

func assertJSONL(t *testing.T, path string, wantTasks int) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("tasks file was not written: %v", err)
	}
	lines := splitLines(t, raw)
	if len(lines) != wantTasks+1 {
		t.Fatalf("got %d lines, want %d (header + %d tasks):\n%s", len(lines), wantTasks+1, wantTasks, raw)
	}
	var header fileHeader
	if err := json.Unmarshal(lines[0], &header); err != nil || header.Version != repository.CurrentTaskDataVersion {
		t.Fatalf("bad header line %q: %v", lines[0], err)
	}
}

func splitLines(t *testing.T, raw []byte) [][]byte {
	t.Helper()
	var lines [][]byte
	for _, line := range bytes.Split(raw, []byte("\n")) {
		if len(bytes.TrimSpace(line)) > 0 {
			lines = append(lines, line)
		}
	}
	return lines
}
