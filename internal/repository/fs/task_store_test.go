package fs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/hungtrd/lazytodo/internal/domain"
	"github.com/hungtrd/lazytodo/internal/repository"
)

func TestTaskStoreMigratesLegacyDataToSequentialIDs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tasks.json")
	legacy := map[domain.TaskStatus][]legacyTask{
		domain.TaskStatusTodo: {
			{Id: "1740000000000000000", Content: "first", Status: domain.TaskStatusTodo, IsStarred: true, CreatedAt: 100},
		},
		domain.TaskStatusDone: {
			{Id: "1740000000000000001", Content: "second", Status: domain.TaskStatusDone},
		},
	}
	data, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	loaded, err := NewTaskStoreAt(path).Load()
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

	persisted, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var envelope repository.TaskData
	if err := json.Unmarshal(persisted, &envelope); err != nil {
		t.Fatalf("migrated file is not an envelope: %v", err)
	}
	if envelope.Version != repository.CurrentTaskDataVersion {
		t.Fatalf("persisted version = %d", envelope.Version)
	}
	if _, err := os.Stat(path + ".v1.bak"); err != nil {
		t.Fatalf("legacy backup was not created: %v", err)
	}
}

func TestTaskStoreRejectsUnknownVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tasks.json")
	if err := os.WriteFile(path, []byte(`{"version":99,"next_id":1,"tasks":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := NewTaskStoreAt(path).Load(); err == nil {
		t.Fatal("expected unsupported version error")
	}
}

func TestTasksFilePathUsesCustomSubdirectory(t *testing.T) {
	root := t.TempDir()
	path, err := TasksFilePath(repository.Config{StorageRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "lazytodo", "tasks.json")
	if path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}
}
