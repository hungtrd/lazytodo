package cli

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/hungtrd/lazytodo/internal/domain"
	"github.com/hungtrd/lazytodo/internal/gitsync"
	"github.com/hungtrd/lazytodo/internal/repository"
	repofs "github.com/hungtrd/lazytodo/internal/repository/fs"
	"github.com/hungtrd/lazytodo/internal/task"
	"github.com/spf13/cobra"
)

type UIRunner func(*task.Service) error

// EditFormRunner opens an interactive editor for a task and reports the fields
// the user changed. changed is false when nothing was edited or the form was
// cancelled. It is injected so tests never need a terminal.
type EditFormRunner func(domain.Task) (patch task.Patch, changed bool, err error)

type Dependencies struct {
	ConfigRepo  repository.ConfigRepository
	RunUI       UIRunner
	RunEditForm EditFormRunner
	In          io.Reader
	Out         io.Writer
	Err         io.Writer
}

type app struct {
	configRepo  repository.ConfigRepository
	runUI       UIRunner
	runEditForm EditFormRunner

	// activeSyncer is set when a command built a task service with sync
	// enabled, so the root command can flush the debounced push before exiting.
	activeSyncer *gitsync.Service
}

func NewRootCommand(deps Dependencies) *cobra.Command {
	app := &app{configRepo: deps.ConfigRepo, runUI: deps.RunUI, runEditForm: deps.RunEditForm}
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
		// A CLI process is short lived, so the debounced push has to be
		// completed here rather than left to a timer that never fires.
		PersistentPostRunE: func(cmd *cobra.Command, _ []string) error {
			if app.activeSyncer == nil {
				return nil
			}
			if err := app.activeSyncer.Flush(); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: sync failed: %v\n", err)
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
		app.newRestoreCommand(),
		app.newPurgeCommand(),
		app.newListCommand(),
		app.newShowCommand(),
		app.newSearchCommand(),
		app.newConfigCommand(),
		app.newSyncCommand(),
	)
	configureUsageTemplate(root)
	return root
}

func configureUsageTemplate(root *cobra.Command) {
	cobra.AddTemplateFunc("lazytodoCommandLabel", commandLabel)
	cobra.AddTemplateFunc("lazytodoCommandLabelPadding", commandLabelPadding)

	const defaultCommandRow = "{{rpad .Name .NamePadding }} {{.Short}}"
	const commandRowWithAliases = "{{rpad (lazytodoCommandLabel .) (lazytodoCommandLabelPadding $cmds) }} {{.Short}}"
	root.SetUsageTemplate(strings.ReplaceAll(root.UsageTemplate(), defaultCommandRow, commandRowWithAliases))
}

func commandLabel(cmd *cobra.Command) string {
	if len(cmd.Aliases) == 0 {
		return cmd.Name()
	}
	return fmt.Sprintf("%s (%s)", cmd.Name(), strings.Join(cmd.Aliases, ", "))
}

func commandLabelPadding(commands []*cobra.Command) int {
	padding := 0
	for _, cmd := range commands {
		if !cmd.IsAvailableCommand() && cmd.Name() != "help" {
			continue
		}
		if width := utf8.RuneCountInString(commandLabel(cmd)); width > padding {
			padding = width
		}
	}
	return padding
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
	svc := task.NewService(repofs.NewTaskStoreAt(path), a.configRepo, repofs.NewSettingsStoreFor(path))

	// Sync trouble must never stop a task operation, so a syncer that cannot be
	// built is reported and then ignored.
	if cfg.Git != nil && cfg.Git.Enabled {
		syncer, err := a.syncer(cfg)
		if err != nil {
			return svc, nil
		}
		a.activeSyncer = syncer
		svc.SetSyncer(syncer)
	}
	return svc, nil
}

// syncer builds a git syncer for the configured storage root.
func (a *app) syncer(cfg repository.Config) (*gitsync.Service, error) {
	if cfg.Git == nil {
		return nil, fmt.Errorf("sync is not configured; run: lazytodo sync init <git-url>")
	}
	if cfg.StorageRoot == "" {
		return nil, fmt.Errorf("sync needs a storage root; run: lazytodo sync init <git-url>")
	}
	logPath, err := repofs.SyncLogPath()
	if err != nil {
		return nil, err
	}
	return gitsync.New(cfg.StorageRoot, *cfg.Git, logPath)
}

// enabledSyncer is for commands that only make sense once sync is turned on.
func (a *app) enabledSyncer() (repository.Config, *gitsync.Service, error) {
	cfg, err := a.configRepo.Load()
	if err != nil {
		return repository.Config{}, nil, err
	}
	if cfg.Git == nil || !cfg.Git.Enabled {
		return cfg, nil, fmt.Errorf("sync is not enabled; run: lazytodo sync init <git-url>")
	}
	syncer, err := a.syncer(cfg)
	return cfg, syncer, err
}
