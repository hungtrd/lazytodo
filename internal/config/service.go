package config

import (
	"fmt"

	"github.com/hungtrd/lazytodo/internal/repository"
	repofs "github.com/hungtrd/lazytodo/internal/repository/fs"
)

type Service struct {
	repo repository.ConfigRepository
}

func NewService(repo repository.ConfigRepository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Get() (repository.Config, error) {
	return s.repo.Load()
}

func (s *Service) SetStorageRoot(value string, force bool) (repository.Config, error) {
	root, err := repofs.ResolveStorageRoot(value)
	if err != nil {
		return repository.Config{}, err
	}
	return s.updateStorageRoot(root, force)
}

func (s *Service) ResetStorageRoot(force bool) (repository.Config, error) {
	return s.updateStorageRoot("", force)
}

func (s *Service) updateStorageRoot(root string, force bool) (repository.Config, error) {
	cfg, err := s.repo.Load()
	if err != nil {
		return repository.Config{}, err
	}
	oldPath, err := repofs.TasksFilePath(cfg)
	if err != nil {
		return repository.Config{}, err
	}
	next := cfg
	next.StorageRoot = root
	newPath, err := repofs.TasksFilePath(next)
	if err != nil {
		return repository.Config{}, err
	}
	copied, err := repofs.CopyTasksFile(oldPath, newPath, force)
	if err != nil {
		return repository.Config{}, err
	}
	if err := s.repo.Save(next); err != nil {
		return repository.Config{}, fmt.Errorf("save storage config: %w", err)
	}
	if copied {
		if err := repofs.RemoveTasksFile(oldPath); err != nil {
			return next, err
		}
	}
	return next, nil
}
