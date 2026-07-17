package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hungtrd/lazytodo/internal/repository"
	repofs "github.com/hungtrd/lazytodo/internal/repository/fs"
)

type memoryConfigRepo struct{ cfg repository.Config }

func (r *memoryConfigRepo) Load() (repository.Config, error) { return r.cfg, nil }
func (r *memoryConfigRepo) Save(cfg repository.Config) error {
	r.cfg = cfg
	return nil
}

func TestSetStorageRootMovesTasksIntoLazytodoSubdirectory(t *testing.T) {
	oldRoot := t.TempDir()
	newRoot := t.TempDir()
	repo := &memoryConfigRepo{cfg: repository.Config{StorageRoot: oldRoot, Vertical: true}}
	oldPath, err := repofs.TasksFilePath(repo.cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(oldPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(oldPath, []byte(`{"version":2,"next_id":1,"tasks":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := NewService(repo).SetStorageRoot(newRoot, false)
	if err != nil {
		t.Fatal(err)
	}
	newPath, err := repofs.TasksFilePath(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if newPath != filepath.Join(newRoot, "lazytodo", "tasks.json") {
		t.Fatalf("new path = %q", newPath)
	}
	if _, err := os.Stat(newPath); err != nil {
		t.Fatalf("destination does not exist: %v", err)
	}
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatalf("source still exists or stat failed: %v", err)
	}
	if !cfg.Vertical {
		t.Fatal("unrelated config was not preserved")
	}
}

func TestSetStorageRootDoesNotOverwriteWithoutForce(t *testing.T) {
	oldRoot := t.TempDir()
	newRoot := t.TempDir()
	repo := &memoryConfigRepo{cfg: repository.Config{StorageRoot: oldRoot}}
	oldPath, _ := repofs.TasksFilePath(repo.cfg)
	newPath, _ := repofs.TasksFilePath(repository.Config{StorageRoot: newRoot})
	if err := os.MkdirAll(filepath.Dir(oldPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(newPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(oldPath, []byte("source"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newPath, []byte("destination"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := NewService(repo).SetStorageRoot(newRoot, false); err == nil {
		t.Fatal("expected destination conflict")
	}
	if repo.cfg.StorageRoot != oldRoot {
		t.Fatalf("config changed after failed migration: %+v", repo.cfg)
	}
}
