package task

import (
	"testing"

	"github.com/hungtrd/lazytodo/internal/domain"
	"github.com/hungtrd/lazytodo/internal/repository"
)

type memoryTaskRepo struct {
	data repository.TaskData
}

func (r *memoryTaskRepo) Load() (repository.TaskData, error) {
	return cloneTaskData(r.data), nil
}

func (r *memoryTaskRepo) Save(data repository.TaskData) error {
	r.data = cloneTaskData(data)
	return nil
}

type memoryConfigRepo struct{ cfg repository.Config }

func (r *memoryConfigRepo) Load() (repository.Config, error) { return r.cfg, nil }
func (r *memoryConfigRepo) Save(cfg repository.Config) error {
	r.cfg = cfg
	return nil
}

func TestServiceCRUDUsesMonotonicIDs(t *testing.T) {
	repo := &memoryTaskRepo{data: repository.TaskData{
		Version: repository.CurrentTaskDataVersion,
		NextID:  1,
		Tasks: map[domain.TaskStatus][]domain.Task{
			domain.TaskStatusTodo: {}, domain.TaskStatusInProgress: {}, domain.TaskStatusDone: {},
		},
	}}
	svc := NewService(repo, &memoryConfigRepo{})

	first, err := svc.Add("Buy milk")
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.Create("Fix bug", domain.TaskStatusInProgress, true)
	if err != nil {
		t.Fatal(err)
	}
	if first.Id != "1" || second.Id != "2" {
		t.Fatalf("IDs = %q, %q; want 1, 2", first.Id, second.Id)
	}
	if err := svc.Delete(second.Id); err != nil {
		t.Fatal(err)
	}
	third, err := svc.Add("Write tests")
	if err != nil {
		t.Fatal(err)
	}
	if third.Id != "3" {
		t.Fatalf("ID after deletion = %q, want 3", third.Id)
	}

	done := domain.TaskStatusDone
	content := "Buy oat milk"
	updated, err := svc.Update(first.Id, Patch{Content: &content, Status: &done})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Content != content || updated.Status != done {
		t.Fatalf("unexpected update: %+v", updated)
	}
	results, err := svc.Search("OAT", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Id != first.Id {
		t.Fatalf("unexpected search results: %+v", results)
	}
}

func TestSetLayoutPreservesStorageRoot(t *testing.T) {
	cfgRepo := &memoryConfigRepo{cfg: repository.Config{StorageRoot: "/tmp/tasks"}}
	svc := NewService(&memoryTaskRepo{}, cfgRepo)
	if err := svc.SetLayoutVertical(true); err != nil {
		t.Fatal(err)
	}
	if !cfgRepo.cfg.Vertical || cfgRepo.cfg.StorageRoot != "/tmp/tasks" {
		t.Fatalf("config was not preserved: %+v", cfgRepo.cfg)
	}
}
