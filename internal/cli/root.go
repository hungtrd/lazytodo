package cli

import (
	"fmt"
	"io"

	"github.com/hungtrd/lazytodo/internal/repository"
	repofs "github.com/hungtrd/lazytodo/internal/repository/fs"
	"github.com/hungtrd/lazytodo/internal/task"
	"github.com/spf13/cobra"
)

type UIRunner func(*task.Service) error

type Dependencies struct {
	ConfigRepo repository.ConfigRepository
	RunUI      UIRunner
	In         io.Reader
	Out        io.Writer
	Err        io.Writer
}

type app struct {
	configRepo repository.ConfigRepository
	runUI      UIRunner
}

func NewRootCommand(deps Dependencies) *cobra.Command {
	app := &app{configRepo: deps.ConfigRepo, runUI: deps.RunUI}
	if app.configRepo == nil {
		app.configRepo = repofs.NewConfigStore()
	}
	var uiMode bool
	root := &cobra.Command{
		Use:           "lazytodo",
		Short:         "Manage todo tasks from the command line",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !uiMode {
				return cmd.Help()
			}
			if app.runUI == nil {
				return fmt.Errorf("UI runner is not configured")
			}
			svc, err := app.taskService()
			if err != nil {
				return err
			}
			return app.runUI(svc)
		},
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			if uiMode && cmd != cmd.Root() {
				return fmt.Errorf("--ui cannot be combined with subcommands")
			}
			return nil
		},
	}
	root.PersistentFlags().BoolVar(&uiMode, "ui", false, "run the terminal UI")
	if deps.In != nil {
		root.SetIn(deps.In)
	}
	if deps.Out != nil {
		root.SetOut(deps.Out)
	}
	if deps.Err != nil {
		root.SetErr(deps.Err)
	}

	root.AddCommand(
		app.newCreateCommand(),
		app.newEditCommand(),
		app.newDeleteCommand(),
		app.newListCommand(),
		app.newShowCommand(),
		app.newSearchCommand(),
		app.newConfigCommand(),
	)
	return root
}

func (a *app) taskService() (*task.Service, error) {
	cfg, err := a.configRepo.Load()
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	path, err := repofs.TasksFilePath(cfg)
	if err != nil {
		return nil, err
	}
	return task.NewService(repofs.NewTaskStoreAt(path), a.configRepo), nil
}
