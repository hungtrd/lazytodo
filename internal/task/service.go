package task

import (
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/hungtrd/lazytodo/internal/domain"
	"github.com/hungtrd/lazytodo/internal/repository"
)

var ErrTaskNotFound = errors.New("task not found")

type Patch struct {
	Content *string
	Status  *domain.TaskStatus
	Starred *bool
}

// Service coordinates task operations and persistence.
type Service struct {
	taskRepo   repository.TaskRepository
	configRepo repository.ConfigRepository
	data       repository.TaskData
}

func NewService(taskRepo repository.TaskRepository, configRepo repository.ConfigRepository) *Service {
	return &Service{taskRepo: taskRepo, configRepo: configRepo}
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

func (s *Service) GetLayoutVertical() (bool, error) {
	cfg, err := s.configRepo.Load()
	if err != nil {
		return false, err
	}
	return cfg.Vertical, nil
}

func (s *Service) SetLayoutVertical(vertical bool) error {
	cfg, err := s.configRepo.Load()
	if err != nil {
		return err
	}
	cfg.Vertical = vertical
	return s.configRepo.Save(cfg)
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
		Content:   content,
		Status:    status,
		IsStarred: starred,
		CreatedAt: now,
	}
	next := cloneTaskData(s.data)
	next.NextID++
	next.Tasks[status] = append([]domain.Task{t}, next.Tasks[status]...)
	if err := s.persist(next); err != nil {
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
	statuses := []domain.TaskStatus{domain.TaskStatusTodo, domain.TaskStatusDoing, domain.TaskStatusDone}
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
		next.Tasks[t.Status] = append([]domain.Task{t}, next.Tasks[t.Status]...)
	} else {
		next.Tasks[status][idx] = t
	}
	if err := s.persist(next); err != nil {
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

func (s *Service) Delete(taskID string) error {
	if err := s.ensureLoaded(); err != nil {
		return err
	}
	status, idx := findTask(s.data.Tasks, taskID)
	if idx == -1 {
		return ErrTaskNotFound
	}
	next := cloneTaskData(s.data)
	next.Tasks[status] = append(next.Tasks[status][:idx], next.Tasks[status][idx+1:]...)
	return s.persist(next)
}

func (s *Service) persist(data repository.TaskData) error {
	if err := s.taskRepo.Save(data); err != nil {
		return err
	}
	s.data = data
	return nil
}

func findTask(tasks map[domain.TaskStatus][]domain.Task, taskID string) (domain.TaskStatus, int) {
	for _, status := range []domain.TaskStatus{domain.TaskStatusTodo, domain.TaskStatusDoing, domain.TaskStatusDone} {
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
	for _, status := range []domain.TaskStatus{domain.TaskStatusTodo, domain.TaskStatusDoing, domain.TaskStatusDone} {
		if out[status] == nil {
			out[status] = []domain.Task{}
		}
	}
	return out
}

func validStatus(status domain.TaskStatus) bool {
	return status >= domain.TaskStatusTodo && status <= domain.TaskStatusDone
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
