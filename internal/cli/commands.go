package cli

import (
	"bufio"
	"fmt"
	"strings"

	appconfig "github.com/hungtrd/lazytodo/internal/config"
	"github.com/hungtrd/lazytodo/internal/domain"
	"github.com/hungtrd/lazytodo/internal/task"
	"github.com/spf13/cobra"
)

func (a *app) newCreateCommand() *cobra.Command {
	var statusValue string
	var starred, jsonOutput bool
	cmd := &cobra.Command{
		Use:     "create <content>",
		Aliases: []string{"add", "new"},
		Short:   "Create a task",
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			status, err := domain.ParseTaskStatus(statusValue)
			if err != nil {
				return err
			}
			svc, err := a.taskService()
			if err != nil {
				return err
			}
			created, err := svc.Create(strings.Join(args, " "), status, starred)
			if err != nil {
				return err
			}
			return writeTask(cmd.OutOrStdout(), created, jsonOutput)
		},
	}
	cmd.Flags().StringVar(&statusValue, "status", "todo", "task status: todo, doing, or done")
	cmd.Flags().BoolVar(&starred, "star", false, "mark the task as starred")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output JSON")
	return cmd
}

func (a *app) newEditCommand() *cobra.Command {
	var content, statusValue string
	var starred, unstarred, jsonOutput bool
	cmd := &cobra.Command{
		Use:   "edit <id>",
		Short: "Edit a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if starred && unstarred {
				return fmt.Errorf("--star and --unstar cannot be used together")
			}
			patch := task.Patch{}
			if cmd.Flags().Changed("content") {
				patch.Content = &content
			}
			if cmd.Flags().Changed("status") {
				status, err := domain.ParseTaskStatus(statusValue)
				if err != nil {
					return err
				}
				patch.Status = &status
			}
			if starred || unstarred {
				value := starred
				patch.Starred = &value
			}
			svc, err := a.taskService()
			if err != nil {
				return err
			}
			updated, err := svc.Update(args[0], patch)
			if err != nil {
				return err
			}
			return writeTask(cmd.OutOrStdout(), updated, jsonOutput)
		},
	}
	cmd.Flags().StringVar(&content, "content", "", "replace task content")
	cmd.Flags().StringVarP(&statusValue, "status", "s", "", "set task status")
	cmd.Flags().BoolVar(&starred, "star", false, "mark the task as starred")
	cmd.Flags().BoolVar(&unstarred, "unstar", false, "remove the starred mark")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output JSON")
	return cmd
}

func (a *app) newDeleteCommand() *cobra.Command {
	var yes, jsonOutput bool
	cmd := &cobra.Command{
		Use:     "delete <id>",
		Aliases: []string{"del", "rm"},
		Short:   "Delete a task",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if jsonOutput && !yes {
				return fmt.Errorf("--json requires --yes when deleting a task")
			}
			svc, err := a.taskService()
			if err != nil {
				return err
			}
			item, err := svc.Get(args[0])
			if err != nil {
				return err
			}
			if !yes {
				fmt.Fprintf(cmd.ErrOrStderr(), "Delete task %s (%s)? [y/N] ", item.Id, item.Content)
				answer, _ := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
				answer = strings.ToLower(strings.TrimSpace(answer))
				if answer != "y" && answer != "yes" {
					fmt.Fprintln(cmd.OutOrStdout(), "Delete cancelled.")
					return nil
				}
			}
			if err := svc.Delete(item.Id); err != nil {
				return err
			}
			if jsonOutput {
				return writeJSON(cmd.OutOrStdout(), map[string]string{"deleted_id": item.Id})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Deleted task %s.\n", item.Id)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip confirmation")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output JSON")
	return cmd
}

func (a *app) newListCommand() *cobra.Command {
	var statusValue string
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List tasks",
		Aliases: []string{"ls"},
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			status, err := optionalStatus(statusValue)
			if err != nil {
				return err
			}
			svc, err := a.taskService()
			if err != nil {
				return err
			}
			items, err := svc.List(status)
			if err != nil {
				return err
			}
			return writeTasks(cmd.OutOrStdout(), items, jsonOutput)
		},
	}
	cmd.Flags().StringVar(&statusValue, "status", "", "filter by task status")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output JSON")
	return cmd
}

func (a *app) newShowCommand() *cobra.Command {
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:     "show <id>",
		Aliases: []string{"detail"},
		Short:   "Show task details",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := a.taskService()
			if err != nil {
				return err
			}
			item, err := svc.Get(args[0])
			if err != nil {
				return err
			}
			return writeTaskDetail(cmd.OutOrStdout(), item, jsonOutput)
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output JSON")
	return cmd
}

func (a *app) newSearchCommand() *cobra.Command {
	var statusValue string
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search task content",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			status, err := optionalStatus(statusValue)
			if err != nil {
				return err
			}
			svc, err := a.taskService()
			if err != nil {
				return err
			}
			items, err := svc.Search(strings.Join(args, " "), status)
			if err != nil {
				return err
			}
			return writeTasks(cmd.OutOrStdout(), items, jsonOutput)
		},
	}
	cmd.Flags().StringVar(&statusValue, "status", "", "filter by task status")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output JSON")
	return cmd
}

func (a *app) newConfigCommand() *cobra.Command {
	command := &cobra.Command{Use: "config", Short: "Manage lazytodo configuration"}
	command.AddCommand(a.newConfigGetCommand(), a.newConfigSetCommand(), a.newConfigResetCommand())
	return command
}

func (a *app) newConfigGetCommand() *cobra.Command {
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "get [storage-root]",
		Short: "Show configuration and resolved paths",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 && args[0] != "storage-root" {
				return fmt.Errorf("unknown config key %q", args[0])
			}
			cfg, err := a.configRepo.Load()
			if err != nil {
				return err
			}
			if len(args) == 1 {
				if jsonOutput {
					return writeJSON(cmd.OutOrStdout(), map[string]string{"storage_root": cfg.StorageRoot})
				}
				fmt.Fprintln(cmd.OutOrStdout(), cfg.StorageRoot)
				return nil
			}
			return writeConfig(cmd.OutOrStdout(), cfg, jsonOutput)
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output JSON")
	return cmd
}

func (a *app) newConfigSetCommand() *cobra.Command {
	var force, jsonOutput bool
	cmd := &cobra.Command{
		Use:   "set storage-root <path>",
		Short: "Set the root directory used to store tasks",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if args[0] != "storage-root" {
				return fmt.Errorf("unknown config key %q", args[0])
			}
			cfg, err := appconfig.NewService(a.configRepo).SetStorageRoot(args[1], force)
			if err != nil {
				return err
			}
			return writeConfig(cmd.OutOrStdout(), cfg, jsonOutput)
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "overwrite an existing destination tasks file")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output JSON")
	return cmd
}

func (a *app) newConfigResetCommand() *cobra.Command {
	var force, jsonOutput bool
	cmd := &cobra.Command{
		Use:   "reset storage-root",
		Short: "Return task storage to ~/.lazytodo",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if args[0] != "storage-root" {
				return fmt.Errorf("unknown config key %q", args[0])
			}
			cfg, err := appconfig.NewService(a.configRepo).ResetStorageRoot(force)
			if err != nil {
				return err
			}
			return writeConfig(cmd.OutOrStdout(), cfg, jsonOutput)
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "overwrite an existing destination tasks file")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output JSON")
	return cmd
}

func optionalStatus(value string) (*domain.TaskStatus, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	status, err := domain.ParseTaskStatus(value)
	if err != nil {
		return nil, err
	}
	return &status, nil
}
