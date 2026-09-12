package fs

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	iofs "io/fs"
	"os"
	"sort"
	"strconv"

	"github.com/hungtrd/lazytodo/internal/domain"
	"github.com/hungtrd/lazytodo/internal/repository"
)

// maxLineBytes bounds a single JSONL record. Task content is capped well below
// this, so the limit only guards against a corrupt file.
const maxLineBytes = 1 << 20

// TaskStore persists tasks as JSONL: a header line carrying the schema version
// and the id counter, followed by one task per line. One task per line keeps
// git diffs minimal and lets the sync merge driver reconcile per task.
type TaskStore struct {
	path       string
	legacyPath string
}

// fileHeader is the first line of a tasks file.
type fileHeader struct {
	Version int   `json:"version"`
	NextID  int64 `json:"next_id"`
}

func NewTaskStoreAt(path string) *TaskStore {
	return &TaskStore{path: path, legacyPath: LegacyTasksFilePath(path)}
}

func emptyTaskMap() map[domain.TaskStatus][]domain.Task {
	tasks := make(map[domain.TaskStatus][]domain.Task, len(domain.AllStatuses()))
	for _, status := range domain.AllStatuses() {
		tasks[status] = []domain.Task{}
	}
	return tasks
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
			return s.migrateLegacyFile()
		}
		return repository.TaskData{}, fmt.Errorf("read tasks file: %w", err)
	}
	return decodeJSONL(data)
}

func (s *TaskStore) Save(data repository.TaskData) error {
	normalizeTaskData(&data)
	encoded, err := encodeJSONL(data)
	if err != nil {
		return err
	}
	if err := atomicWriteFile(s.path, encoded, 0o644); err != nil {
		return fmt.Errorf("write tasks file: %w", err)
	}
	return nil
}

// migrateLegacyFile converts a pre-v4 tasks.json into tasks.jsonl, keeping a
// backup of the original before removing it so there is a single source of
// truth afterwards.
func (s *TaskStore) migrateLegacyFile() (repository.TaskData, error) {
	if s.legacyPath == s.path {
		return emptyTaskData(), nil
	}
	raw, err := os.ReadFile(s.legacyPath)
	if err != nil {
		if errors.Is(err, iofs.ErrNotExist) {
			return emptyTaskData(), nil
		}
		return repository.TaskData{}, fmt.Errorf("read legacy tasks file: %w", err)
	}

	migrated, sourceVersion, err := decodeLegacyJSON(raw)
	if err != nil {
		return repository.TaskData{}, err
	}
	if err := backupFile(s.legacyPath, raw, sourceVersion); err != nil {
		return repository.TaskData{}, fmt.Errorf("back up version %d tasks: %w", sourceVersion, err)
	}
	if err := s.Save(migrated); err != nil {
		return repository.TaskData{}, fmt.Errorf("persist version %d migration: %w", repository.CurrentTaskDataVersion, err)
	}
	if err := RemoveTasksFile(s.legacyPath); err != nil {
		return repository.TaskData{}, err
	}
	return migrated, nil
}

// EncodeTaskData and DecodeTaskData expose the JSONL codec so the sync merge
// driver can reconcile two revisions of a tasks file without duplicating the
// on-disk format.
func EncodeTaskData(data repository.TaskData) ([]byte, error) {
	normalizeTaskData(&data)
	return encodeJSONL(data)
}

func DecodeTaskData(raw []byte) (repository.TaskData, error) { return decodeJSONL(raw) }

// encodeJSONL writes the header line followed by every task ordered by id. The
// stable order is what keeps a git diff limited to the tasks that changed.
func encodeJSONL(data repository.TaskData) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)

	if err := enc.Encode(fileHeader{Version: data.Version, NextID: data.NextID}); err != nil {
		return nil, fmt.Errorf("encode tasks header: %w", err)
	}
	for _, item := range sortedTasks(data.Tasks) {
		if err := enc.Encode(item); err != nil {
			return nil, fmt.Errorf("encode task %s: %w", item.Id, err)
		}
	}
	return buf.Bytes(), nil
}

func decodeJSONL(raw []byte) (repository.TaskData, error) {
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 0, 64*1024), maxLineBytes)

	data := emptyTaskData()
	headerSeen := false
	for line := 1; scanner.Scan(); line++ {
		text := bytes.TrimSpace(scanner.Bytes())
		if len(text) == 0 {
			continue
		}
		if !headerSeen {
			headerSeen = true
			var header fileHeader
			// A missing version means the file has no header line, which
			// happens when someone hand-edits it. Fall through and read the
			// line as a task instead of rejecting the whole file.
			if err := json.Unmarshal(text, &header); err == nil && header.Version > 0 {
				if header.Version > repository.CurrentTaskDataVersion {
					return repository.TaskData{}, fmt.Errorf("unsupported tasks data version %d", header.Version)
				}
				data.Version = header.Version
				data.NextID = header.NextID
				continue
			}
		}
		var item domain.Task
		if err := json.Unmarshal(text, &item); err != nil {
			return repository.TaskData{}, fmt.Errorf("decode task on line %d: %w", line, err)
		}
		data.Tasks[item.Status] = append(data.Tasks[item.Status], item)
	}
	if err := scanner.Err(); err != nil {
		return repository.TaskData{}, fmt.Errorf("read tasks file: %w", err)
	}

	normalizeTaskData(&data)
	return data, nil
}

// sortedTasks flattens the per-status buckets into one id-ordered slice.
// Numeric ids sort numerically and ahead of any non-numeric ones.
func sortedTasks(tasks map[domain.TaskStatus][]domain.Task) []domain.Task {
	var out []domain.Task
	for _, status := range domain.AllStatuses() {
		out = append(out, tasks[status]...)
	}
	sort.SliceStable(out, func(i, j int) bool {
		left, leftOK := numericID(out[i].Id)
		right, rightOK := numericID(out[j].Id)
		if leftOK && rightOK {
			return left < right
		}
		if leftOK != rightOK {
			return leftOK
		}
		return out[i].Id < out[j].Id
	})
	return out
}

func numericID(id string) (int64, bool) {
	value, err := strconv.ParseInt(id, 10, 64)
	return value, err == nil
}

// backupFile keeps the original bytes next to the source as <path>.v<N>.bak.
// An existing backup is never overwritten.
func backupFile(path string, data []byte, version int) error {
	backupPath := fmt.Sprintf("%s.v%d.bak", path, version)
	if _, err := os.Stat(backupPath); err == nil {
		return nil
	} else if !errors.Is(err, iofs.ErrNotExist) {
		return err
	}
	return atomicWriteFile(backupPath, data, 0o644)
}

func normalizeTaskData(data *repository.TaskData) {
	data.Version = repository.CurrentTaskDataVersion
	if data.Tasks == nil {
		data.Tasks = emptyTaskMap()
	}
	var maxID int64
	for _, status := range domain.AllStatuses() {
		if data.Tasks[status] == nil {
			data.Tasks[status] = []domain.Task{}
		}
		for i := range data.Tasks[status] {
			item := &data.Tasks[status][i]
			item.Status = status
			// Tasks written before UIDs existed get one derived from their
			// contents, which keeps the value identical on every machine.
			if item.UID == "" {
				item.UID = domain.DeriveUID(*item)
			}
			if id, ok := numericID(item.Id); ok && id > maxID {
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
