package task

import (
	"errors"
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

type memorySettingsRepo struct{ settings repository.Settings }

func (r *memorySettingsRepo) Load() (repository.Settings, error) { return r.settings, nil }
func (r *memorySettingsRepo) Save(settings repository.Settings) error {
	r.settings = settings
	return nil
}

func TestServiceCRUDUsesMonotonicIDs(t *testing.T) {
	repo := &memoryTaskRepo{data: repository.TaskData{
		Version: repository.CurrentTaskDataVersion,
		NextID:  1,
		Tasks: map[domain.TaskStatus][]domain.Task{
			domain.TaskStatusTodo: {}, domain.TaskStatusDoing: {}, domain.TaskStatusDone: {},
		},
	}}
	svc := NewService(repo, &memoryConfigRepo{}, &memorySettingsRepo{})

	first, err := svc.Add("Buy milk")
	if err != nil {
		t.Fatal(err)
	}
	second, err := svc.Create("Fix bug", domain.TaskStatusDoing, true)
	if err != nil {
		t.Fatal(err)
	}
	if first.Id != "1" || second.Id != "2" {
		t.Fatalf("IDs = %q, %q; want 1, 2", first.Id, second.Id)
	}
	if err := svc.Purge(second.Id); err != nil {
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

func TestSetLayoutWritesSettingsAndLeavesConfigAlone(t *testing.T) {
	cfgRepo := &memoryConfigRepo{cfg: repository.Config{StorageRoot: "/tmp/tasks"}}
	settingsRepo := &memorySettingsRepo{}
	svc := NewService(&memoryTaskRepo{}, cfgRepo, settingsRepo)
	if err := svc.SetLayoutVertical(true); err != nil {
		t.Fatal(err)
	}
	if settingsRepo.settings.Vertical == nil || !*settingsRepo.settings.Vertical {
		t.Fatalf("settings were not written: %+v", settingsRepo.settings)
	}
	if cfgRepo.cfg.StorageRoot != "/tmp/tasks" {
		t.Fatalf("config was not preserved: %+v", cfgRepo.cfg)
	}
	vertical, err := svc.GetLayoutVertical()
	if err != nil {
		t.Fatal(err)
	}
	if !vertical {
		t.Fatal("GetLayoutVertical = false, want true")
	}
}

// A user upgrading from the config-only layout keeps their preference until
// they toggle it, at which point settings.json takes over for good.
func TestGetLayoutSeedsFromLegacyConfigUntilSet(t *testing.T) {
	cfgRepo := &memoryConfigRepo{cfg: repository.Config{Vertical: true}}
	settingsRepo := &memorySettingsRepo{}
	svc := NewService(&memoryTaskRepo{}, cfgRepo, settingsRepo)

	vertical, err := svc.GetLayoutVertical()
	if err != nil {
		t.Fatal(err)
	}
	if !vertical {
		t.Fatal("legacy vertical config was not honoured")
	}

	if err := svc.SetLayoutVertical(false); err != nil {
		t.Fatal(err)
	}
	vertical, err = svc.GetLayoutVertical()
	if err != nil {
		t.Fatal(err)
	}
	if vertical {
		t.Fatal("settings did not override the legacy config value")
	}
}

func TestArchiveRestorePurgeLifecycle(t *testing.T) {
	svc := NewService(&memoryTaskRepo{}, &memoryConfigRepo{}, &memorySettingsRepo{})

	item, err := svc.Add("Ship the feature")
	if err != nil {
		t.Fatal(err)
	}

	archived, err := svc.Archive(item.Id)
	if err != nil {
		t.Fatal(err)
	}
	if archived.Status != domain.TaskStatusArchived || archived.ArchivedAt == 0 {
		t.Fatalf("unexpected archived task: %+v", archived)
	}

	// Archived tasks stay out of an unfiltered list but remain addressable.
	active, err := svc.List(nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 0 {
		t.Fatalf("archived task leaked into the default list: %+v", active)
	}
	archivedStatus := domain.TaskStatusArchived
	stored, err := svc.List(&archivedStatus)
	if err != nil {
		t.Fatal(err)
	}
	if len(stored) != 1 || stored[0].Id != item.Id {
		t.Fatalf("archived list = %+v, want the archived task", stored)
	}

	if _, err := svc.Archive(item.Id); !errors.Is(err, ErrAlreadyArchived) {
		t.Fatalf("second archive error = %v, want ErrAlreadyArchived", err)
	}
	if _, err := svc.Restore(item.Id, domain.TaskStatusArchived); !errors.Is(err, ErrRestoreToArchive) {
		t.Fatalf("restore-to-archived error = %v, want ErrRestoreToArchive", err)
	}

	restored, err := svc.Restore(item.Id, domain.TaskStatusDoing)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Status != domain.TaskStatusDoing || restored.ArchivedAt != 0 {
		t.Fatalf("unexpected restored task: %+v", restored)
	}
	if _, err := svc.Restore(item.Id, domain.TaskStatusTodo); !errors.Is(err, ErrNotArchived) {
		t.Fatalf("restore of an active task = %v, want ErrNotArchived", err)
	}

	if _, err := svc.Archive(item.Id); err != nil {
		t.Fatal(err)
	}
	count, err := svc.PurgeArchived()
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("PurgeArchived = %d, want 1", count)
	}
	if _, err := svc.Get(item.Id); !errors.Is(err, ErrTaskNotFound) {
		t.Fatalf("get after purge = %v, want ErrTaskNotFound", err)
	}
}
