package fs

import (
	"encoding/json"
	"errors"
	"fmt"
	iofs "io/fs"
	"os"
	"strconv"

	"github.com/hungtrd/lazytodo/internal/domain"
	"github.com/hungtrd/lazytodo/internal/repository"
)

type TaskStore struct {
	path string
}

type legacyTask struct {
	Id        string
	Content   string
	Status    domain.TaskStatus
	IsStarred bool
	StartedAt int64
	CreatedAt int64
	UpdatedAt int64
}

func NewTaskStoreAt(path string) *TaskStore { return &TaskStore{path: path} }

func emptyTaskMap() map[domain.TaskStatus][]domain.Task {
	return map[domain.TaskStatus][]domain.Task{
		domain.TaskStatusTodo:  {},
		domain.TaskStatusDoing: {},
		domain.TaskStatusDone:  {},
	}
}

func emptyTaskData() repository.TaskData {
	return repository.TaskData{
		Version: repository.CurrentTaskDataVersion,
		NextID:  1,
		Tasks:   emptyTaskMap(),
	}
}

func (s *TaskStore) Load() (repository.TaskData, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, iofs.ErrNotExist) {
			return emptyTaskData(), nil
		}
		return repository.TaskData{}, fmt.Errorf("read tasks file: %w", err)
	}

	var envelope repository.TaskData
	if err := json.Unmarshal(data, &envelope); err == nil && envelope.Version > 0 {
		if envelope.Version == 2 {
			normalizeTaskData(&envelope)
			if err := s.backupDataFile(data, 2); err != nil {
				return repository.TaskData{}, fmt.Errorf("back up version 2 tasks: %w", err)
			}
			if err := s.Save(envelope); err != nil {
				return repository.TaskData{}, fmt.Errorf("persist version 3 migration: %w", err)
			}
			return envelope, nil
		}
		if envelope.Version != repository.CurrentTaskDataVersion {
			return repository.TaskData{}, fmt.Errorf("unsupported tasks data version %d", envelope.Version)
		}
		normalizeTaskData(&envelope)
		return envelope, nil
	}

	var legacy map[domain.TaskStatus][]legacyTask
	if err := json.Unmarshal(data, &legacy); err != nil {
		return repository.TaskData{}, fmt.Errorf("decode tasks: %w", err)
	}
	migrated := migrateLegacyTasks(legacy)
	if err := s.backupDataFile(data, 1); err != nil {
		return repository.TaskData{}, fmt.Errorf("back up legacy tasks: %w", err)
	}
	if err := s.Save(migrated); err != nil {
		return repository.TaskData{}, fmt.Errorf("persist tasks migration: %w", err)
	}
	return migrated, nil
}

func (s *TaskStore) backupDataFile(data []byte, version int) error {
	backupPath := fmt.Sprintf("%s.v%d.bak", s.path, version)
	if _, err := os.Stat(backupPath); err == nil {
		return nil
	} else if !errors.Is(err, iofs.ErrNotExist) {
		return err
	}
	return atomicWriteFile(backupPath, data, 0o644)
}

func (s *TaskStore) Save(data repository.TaskData) error {
	normalizeTaskData(&data)
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("encode tasks: %w", err)
	}
	if err := atomicWriteFile(s.path, b, 0o644); err != nil {
		return fmt.Errorf("write tasks file: %w", err)
	}
	return nil
}

func migrateLegacyTasks(legacy map[domain.TaskStatus][]legacyTask) repository.TaskData {
	data := emptyTaskData()
	var next int64 = 1
	for _, status := range []domain.TaskStatus{domain.TaskStatusTodo, domain.TaskStatusDoing, domain.TaskStatusDone} {
		for _, existing := range legacy[status] {
			migrated := domain.Task{
				Id:        strconv.FormatInt(next, 10),
				Content:   existing.Content,
				Status:    status,
				IsStarred: existing.IsStarred,
				StartedAt: existing.StartedAt,
				CreatedAt: existing.CreatedAt,
				UpdatedAt: existing.UpdatedAt,
			}
			data.Tasks[status] = append(data.Tasks[status], migrated)
			next++
		}
	}
	data.NextID = next
	return data
}

func normalizeTaskData(data *repository.TaskData) {
	data.Version = repository.CurrentTaskDataVersion
	if data.Tasks == nil {
		data.Tasks = emptyTaskMap()
	}
	var maxID int64
	for _, status := range []domain.TaskStatus{domain.TaskStatusTodo, domain.TaskStatusDoing, domain.TaskStatusDone} {
		if data.Tasks[status] == nil {
			data.Tasks[status] = []domain.Task{}
		}
		for i := range data.Tasks[status] {
			data.Tasks[status][i].Status = status
			if id, err := strconv.ParseInt(data.Tasks[status][i].Id, 10, 64); err == nil && id > maxID {
				maxID = id
			}
		}
	}
	if data.NextID <= maxID {
		data.NextID = maxID + 1
	}
	if data.NextID < 1 {
		data.NextID = 1
	}
}

var _ repository.TaskRepository = (*TaskStore)(nil)
