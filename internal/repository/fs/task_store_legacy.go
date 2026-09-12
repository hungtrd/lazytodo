package fs

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/hungtrd/lazytodo/internal/domain"
	"github.com/hungtrd/lazytodo/internal/repository"
)

// This file holds the readers for the pre-v4 tasks.json formats. They run once
// per installation, when Load finds no tasks.jsonl but a tasks.json is present.
//
//	v1 — a bare map of numeric status keys to tasks with Go-default field names
//	v2 — the {version, next_id, tasks} envelope, before the doing status
//	v3 — the same envelope with doing

// legacyTask matches the v1 shape, which used Go's default capitalized names.
type legacyTask struct {
	Id        string
	Content   string
	Status    domain.TaskStatus
	IsStarred bool
	StartedAt int64
	CreatedAt int64
	UpdatedAt int64
}

// decodeLegacyJSON converts a pre-v4 document and reports which version it was
// read as, so the caller can name the backup file accordingly.
func decodeLegacyJSON(raw []byte) (repository.TaskData, int, error) {
	var envelope repository.TaskData
	if err := json.Unmarshal(raw, &envelope); err == nil && envelope.Version > 0 {
		if envelope.Version > repository.CurrentTaskDataVersion {
			return repository.TaskData{}, 0, fmt.Errorf("unsupported tasks data version %d", envelope.Version)
		}
		sourceVersion := envelope.Version
		normalizeTaskData(&envelope)
		return envelope, sourceVersion, nil
	}

	var legacy map[domain.TaskStatus][]legacyTask
	if err := json.Unmarshal(raw, &legacy); err != nil {
		return repository.TaskData{}, 0, fmt.Errorf("decode tasks: %w", err)
	}
	return migrateLegacyTasks(legacy), 1, nil
}

// migrateLegacyTasks reassigns sequential ids, since v1 files predate the id
// counter and may hold duplicates or blanks.
func migrateLegacyTasks(legacy map[domain.TaskStatus][]legacyTask) repository.TaskData {
	data := emptyTaskData()
	var next int64 = 1
	for _, status := range domain.AllStatuses() {
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
