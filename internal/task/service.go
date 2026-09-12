package task

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/hungtrd/lazytodo/internal/domain"
	"github.com/hungtrd/lazytodo/internal/repository"
)

var (
	ErrTaskNotFound     = errors.New("task not found")
	ErrAlreadyArchived  = errors.New("task is already archived (use purge to delete it permanently)")
	ErrNotArchived      = errors.New("task is not archived")
	ErrRestoreToArchive = errors.New("cannot restore a task to archived")
)

type Patch struct {
	Content *string
	Status  *domain.TaskStatus
	Starred *bool
}

// Syncer mirrors task storage somewhere durable, such as a git remote. The
// task package only names the behaviour it needs, so it stays unaware of git.
type Syncer interface {
	// NotifyChanged runs after every successful write; reason describes it.
	NotifyChanged(reason string)
	// Pull brings in remote changes, used once at startup.
	Pull() error
	// Flush completes any deferred work before the process exits.
	Flush() error
	// StatusLabel is a short indicator for the UI.
	StatusLabel() string
}

// Service coordinates task operations and persistence.
type Service struct {
	taskRepo     repository.TaskRepository
	configRepo   repository.ConfigRepository
	settingsRepo repository.SettingsRepository
	syncer       Syncer
	data         repository.TaskData
}

// SetSyncer attaches an optional syncer. Leaving it unset keeps the service
// purely local.
func (s *Service) SetSyncer(syncer Syncer) { s.syncer = syncer }

// Syncer returns the attached syncer, or nil when storage is local only.
func (s *Service) Syncer() Syncer { return s.syncer }

func NewService(taskRepo repository.TaskRepository, configRepo repository.ConfigRepository, settingsRepo repository.SettingsRepository) *Service {
	return &Service{taskRepo: taskRepo, configRepo: configRepo, settingsRepo: settingsRepo}
}

func (s *Service) Load() (map[domain.TaskStatus][]domain.Task, error) {
	data, err := s.taskRepo.Load()
	if err != nil {
		return nil, err
	}
	s.data = data
	return copyTaskMap(data.Tasks), nil
}

func (s *Service) ensureLoaded() error {
	if s.data.Tasks != nil {
		return nil
	}
	_, err := s.Load()
	return err
}

// GetLayoutVertical reads the synced settings, falling back to the legacy
// config value the first time so an existing preference is not lost.
func (s *Service) GetLayoutVertical() (bool, error) {
	settings, err := s.settingsRepo.Load()
	if err != nil {
		return false, err
	}
	if settings.Vertical != nil {
		return *settings.Vertical, nil
	}
	cfg, err := s.configRepo.Load()
	if err != nil {
		return false, err
	}
	return cfg.Vertical, nil
}

func (s *Service) SetLayoutVertical(vertical bool) error {
	settings, err := s.settingsRepo.Load()
	if err != nil {
		return err
	}
	settings.Vertical = &vertical
	return s.settingsRepo.Save(settings)
}

func (s *Service) Add(content string) (domain.Task, error) {
	return s.Create(content, domain.TaskStatusTodo, false)
}

func (s *Service) Create(content string, status domain.TaskStatus, starred bool) (domain.Task, error) {
	if err := s.ensureLoaded(); err != nil {
		return domain.Task{}, err
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return domain.Task{}, errors.New("content is empty")
	}
	if !validStatus(status) {
		return domain.Task{}, errors.New("invalid task status")
	}

	now := time.Now().Unix()
	t := domain.Task{
		Id:        strconv.FormatInt(s.data.NextID, 10),
		UID:       domain.NewUID(),
		Content:   content,
		Status:    status,
		IsStarred: starred,
		CreatedAt: now,
	}
	next := cloneTaskData(s.data)
	next.NextID++
	next.Tasks[status] = append([]domain.Task{t}, next.Tasks[status]...)
	if err := s.persist(next, "create task "+t.Id); err != nil {
		return domain.Task{}, err
	}
	return t, nil
}

func (s *Service) Get(taskID string) (domain.Task, error) {
	if err := s.ensureLoaded(); err != nil {
		return domain.Task{}, err
	}
	status, idx := findTask(s.data.Tasks, taskID)
	if idx == -1 {
		return domain.Task{}, ErrTaskNotFound
	}
	return s.data.Tasks[status][idx], nil
}

func (s *Service) List(status *domain.TaskStatus) ([]domain.Task, error) {
	if err := s.ensureLoaded(); err != nil {
		return nil, err
	}
	statuses := domain.ActiveStatuses()
	if status != nil {
		if !validStatus(*status) {
			return nil, errors.New("invalid task status")
		}
		statuses = []domain.TaskStatus{*status}
	}
	var result []domain.Task
	for _, st := range statuses {
		list := s.data.Tasks[st]
		for _, idx := range SortedOrder(list) {
			result = append(result, list[idx])
		}
	}
	return result, nil
}

func (s *Service) Search(query string, status *domain.TaskStatus) ([]domain.Task, error) {
	query = strings.ToLower(strings.TrimSpace(query))
	if query == "" {
		return nil, errors.New("search query is empty")
	}
	tasks, err := s.List(status)
	if err != nil {
		return nil, err
	}
	result := make([]domain.Task, 0)
	for _, item := range tasks {
		if strings.Contains(strings.ToLower(item.Content), query) {
			result = append(result, item)
		}
	}
	return result, nil
}

func (s *Service) Update(taskID string, patch Patch) (domain.Task, error) {
	if err := s.ensureLoaded(); err != nil {
		return domain.Task{}, err
	}
	status, idx := findTask(s.data.Tasks, taskID)
	if idx == -1 {
		return domain.Task{}, ErrTaskNotFound
	}
	if patch.Content == nil && patch.Status == nil && patch.Starred == nil {
		return domain.Task{}, errors.New("no changes specified")
	}

	next := cloneTaskData(s.data)
	t := next.Tasks[status][idx]
	if patch.Content != nil {
		content := strings.TrimSpace(*patch.Content)
		if content == "" {
			return domain.Task{}, errors.New("content is empty")
		}
		t.Content = content
	}
	if patch.Starred != nil {
		t.IsStarred = *patch.Starred
	}
	if patch.Status != nil && !validStatus(*patch.Status) {
		return domain.Task{}, errors.New("invalid task status")
	}
	t.UpdatedAt = time.Now().Unix()

	if patch.Status != nil && *patch.Status != status {
		next.Tasks[status] = append(next.Tasks[status][:idx], next.Tasks[status][idx+1:]...)
		t.Status = *patch.Status
		if t.Status == domain.TaskStatusArchived {
			t.ArchivedAt = t.UpdatedAt
		} else {
			t.ArchivedAt = 0
		}
		next.Tasks[t.Status] = append([]domain.Task{t}, next.Tasks[t.Status]...)
	} else {
		next.Tasks[status][idx] = t
	}
	if err := s.persist(next, "update task "+t.Id); err != nil {
		return domain.Task{}, err
	}
	return t, nil
}

func (s *Service) UpdateContent(taskID, content string) error {
	_, err := s.Update(taskID, Patch{Content: &content})
	return err
}

func (s *Service) ToggleStar(taskID string) error {
	t, err := s.Get(taskID)
	if err != nil {
		return err
	}
	starred := !t.IsStarred
	_, err = s.Update(taskID, Patch{Starred: &starred})
	return err
}

func (s *Service) Move(taskID string, to domain.TaskStatus) error {
	t, err := s.Get(taskID)
	if err != nil {
		return err
	}
	if t.Status == to {
		return nil
	}
	_, err = s.Update(taskID, Patch{Status: &to})
	return err
}

// Archive moves a task into the archived bucket. It is the non-destructive
// counterpart of Purge and is what the delete affordances in the CLI and TUI
// call, so a task is never lost by accident.
func (s *Service) Archive(taskID string) (domain.Task, error) {
	current, err := s.Get(taskID)
	if err != nil {
		return domain.Task{}, err
	}
	if current.Status == domain.TaskStatusArchived {
		return domain.Task{}, ErrAlreadyArchived
	}
	archived := domain.TaskStatusArchived
	return s.Update(taskID, Patch{Status: &archived})
}

// Restore brings an archived task back into an active status.
func (s *Service) Restore(taskID string, to domain.TaskStatus) (domain.Task, error) {
	if to == domain.TaskStatusArchived {
		return domain.Task{}, ErrRestoreToArchive
	}
	if !validStatus(to) {
		return domain.Task{}, errors.New("invalid task status")
	}
	current, err := s.Get(taskID)
	if err != nil {
		return domain.Task{}, err
	}
	if current.Status != domain.TaskStatusArchived {
		return domain.Task{}, ErrNotArchived
	}
	return s.Update(taskID, Patch{Status: &to})
}

// Purge removes a task from storage permanently. This cannot be undone.
func (s *Service) Purge(taskID string) error {
	if err := s.ensureLoaded(); err != nil {
		return err
	}
	status, idx := findTask(s.data.Tasks, taskID)
	if idx == -1 {
		return ErrTaskNotFound
	}
	next := cloneTaskData(s.data)
	next.Tasks[status] = append(next.Tasks[status][:idx], next.Tasks[status][idx+1:]...)
	return s.persist(next, "purge task "+taskID)
}

// PurgeArchived permanently removes every archived task and reports how many
// were removed.
func (s *Service) PurgeArchived() (int, error) {
	if err := s.ensureLoaded(); err != nil {
		return 0, err
	}
	count := len(s.data.Tasks[domain.TaskStatusArchived])
	if count == 0 {
		return 0, nil
	}
	next := cloneTaskData(s.data)
	next.Tasks[domain.TaskStatusArchived] = []domain.Task{}
	if err := s.persist(next, fmt.Sprintf("purge %d archived task(s)", count)); err != nil {
		return 0, err
	}
	return count, nil
}

// persist is the single write path, which makes it the one place a sync hook
// needs to observe. reason becomes the git commit subject.
func (s *Service) persist(data repository.TaskData, reason string) error {
	if err := s.taskRepo.Save(data); err != nil {
		return err
	}
	s.data = data
	if s.syncer != nil {
		s.syncer.NotifyChanged(reason)
	}
	return nil
}

func findTask(tasks map[domain.TaskStatus][]domain.Task, taskID string) (domain.TaskStatus, int) {
	for _, status := range domain.AllStatuses() {
		for i := range tasks[status] {
			if tasks[status][i].Id == taskID {
				return status, i
			}
		}
	}
	return domain.TaskStatusTodo, -1
}

func cloneTaskData(data repository.TaskData) repository.TaskData {
	return repository.TaskData{
		Version: data.Version,
		NextID:  data.NextID,
		Tasks:   copyTaskMap(data.Tasks),
	}
}

func copyTaskMap(tasks map[domain.TaskStatus][]domain.Task) map[domain.TaskStatus][]domain.Task {
	out := make(map[domain.TaskStatus][]domain.Task, len(tasks))
	for key, value := range tasks {
		out[key] = append([]domain.Task(nil), value...)
	}
	for _, status := range domain.AllStatuses() {
		if out[status] == nil {
			out[status] = []domain.Task{}
		}
	}
	return out
}

func validStatus(status domain.TaskStatus) bool {
	return status >= domain.TaskStatusTodo && status <= domain.TaskStatusArchived
}

// SortedOrder is reused by the TUI for selection mapping.
func SortedOrder(list []domain.Task) []int {
	order := make([]int, len(list))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(i, j int) bool {
		ti := list[order[i]]
		tj := list[order[j]]
		if ti.IsStarred != tj.IsStarred {
			return ti.IsStarred
		}
		return ti.CreatedAt > tj.CreatedAt
	})
	return order
}
