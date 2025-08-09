package task

import (
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/hungtrd/lazytodo/internal/domain"
	"github.com/hungtrd/lazytodo/internal/repository"
)

// Service coordinates task operations and persistence.
type Service struct {
	taskRepo   repository.TaskRepository
	configRepo repository.ConfigRepository

	// cached state held in memory while program runs
	tasksByStatus map[domain.TaskStatus][]domain.Task
}

func NewService(taskRepo repository.TaskRepository, configRepo repository.ConfigRepository) *Service {
	return &Service{taskRepo: taskRepo, configRepo: configRepo}
}

func (s *Service) Load() (map[domain.TaskStatus][]domain.Task, error) {
	m, err := s.taskRepo.Load()
	if err != nil {
		return nil, err
	}
	s.tasksByStatus = m
	return s.copyState(), nil
}

func (s *Service) GetLayoutVertical() (bool, error) {
	cfg, err := s.configRepo.Load()
	if err != nil {
		// still return something usable
		return false, err
	}
	return cfg.Vertical, nil
}

func (s *Service) SetLayoutVertical(vertical bool) error {
	return s.configRepo.Save(repository.Config{Vertical: vertical})
}

func (s *Service) Add(content string) (domain.Task, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return domain.Task{}, errors.New("content is empty")
	}
	now := time.Now().Unix()
	t := domain.Task{Id: newID(), Content: content, Status: domain.TaskStatusTodo, CreatedAt: now}
	s.tasksByStatus[domain.TaskStatusTodo] = append([]domain.Task{t}, s.tasksByStatus[domain.TaskStatusTodo]...)
	if err := s.taskRepo.Save(s.tasksByStatus); err != nil {
		return domain.Task{}, err
	}
	return t, nil
}

func (s *Service) UpdateContent(taskID, content string) error {
	content = strings.TrimSpace(content)
	if content == "" {
		return errors.New("content is empty")
	}
	status, idx := s.findTask(taskID)
	if idx == -1 {
		return errors.New("task not found")
	}
	t := s.tasksByStatus[status][idx]
	t.Content = content
	t.UpdatedAt = time.Now().Unix()
	s.tasksByStatus[status][idx] = t
	return s.taskRepo.Save(s.tasksByStatus)
}

func (s *Service) ToggleStar(taskID string) error {
	status, idx := s.findTask(taskID)
	if idx == -1 {
		return errors.New("task not found")
	}
	t := s.tasksByStatus[status][idx]
	t.IsStarred = !t.IsStarred
	s.tasksByStatus[status][idx] = t
	return s.taskRepo.Save(s.tasksByStatus)
}

func (s *Service) Move(taskID string, to domain.TaskStatus) error {
	status, idx := s.findTask(taskID)
	if idx == -1 {
		return errors.New("task not found")
	}
	if status == to {
		return nil
	}
	list := s.tasksByStatus[status]
	t := list[idx]
	// remove from source
	s.tasksByStatus[status] = append(list[:idx], list[idx+1:]...)
	// insert at top of target
	t.Status = to
	t.UpdatedAt = time.Now().Unix()
	s.tasksByStatus[to] = append([]domain.Task{t}, s.tasksByStatus[to]...)
	return s.taskRepo.Save(s.tasksByStatus)
}

func (s *Service) Delete(taskID string) error {
	status, idx := s.findTask(taskID)
	if idx == -1 {
		return errors.New("task not found")
	}
	list := s.tasksByStatus[status]
	s.tasksByStatus[status] = append(list[:idx], list[idx+1:]...)
	return s.taskRepo.Save(s.tasksByStatus)
}

// Helpers
func (s *Service) findTask(taskID string) (domain.TaskStatus, int) {
	for st, list := range s.tasksByStatus {
		for i := range list {
			if list[i].Id == taskID {
				return st, i
			}
		}
	}
	return domain.TaskStatusTodo, -1
}

func (s *Service) copyState() map[domain.TaskStatus][]domain.Task {
	out := make(map[domain.TaskStatus][]domain.Task, len(s.tasksByStatus))
	for k, v := range s.tasksByStatus {
		vv := make([]domain.Task, len(v))
		copy(vv, v)
		out[k] = vv
	}
	return out
}

// Sorting logic reused by UI for selection mapping
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
