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
	cmd.Flags().StringVar(&statusValue, "status", "todo", "task status: todo, doing, done, or archived")
	cmd.Flags().BoolVar(&starred, "star", false, "mark the task as starred")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output JSON")
	return cmd
}

func (a *app) newEditCommand() *cobra.Command {
	var content, statusValue string
	var starred, unstarred, jsonOutput bool
	cmd := &cobra.Command{
		Use:   "edit <id>",
		Short: "Edit a task; opens an interactive form when no field flags are given",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if starred && unstarred {
				return fmt.Errorf("--star and --unstar cannot be used together")
			}
			svc, err := a.taskService()
			if err != nil {
				return err
			}

			flags := cmd.Flags()
			interactive := !flags.Changed("content") && !flags.Changed("status") && !starred && !unstarred
			if interactive {
				return a.editInteractively(cmd, svc, args[0], jsonOutput)
			}

			patch := task.Patch{}
			if flags.Changed("content") {
				patch.Content = &content
			}
			if flags.Changed("status") {
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
			updated, err := svc.Update(args[0], patch)
			if err != nil {
				return err
			}
			return writeTask(cmd.OutOrStdout(), updated, jsonOutput)
		},
	}
	cmd.Flags().StringVar(&content, "content", "", "replace task content")
	cmd.Flags().StringVarP(&statusValue, "status", "s", "", "set task status: todo, doing, done, or archived")
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
		Short:   "Archive a task (reversible with restore)",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if jsonOutput && !yes {
				return fmt.Errorf("--json requires --yes when archiving a task")
			}
			svc, err := a.taskService()
			if err != nil {
				return err
			}
			item, err := svc.Get(args[0])
			if err != nil {
				return err
			}
			if !yes && !confirm(cmd, fmt.Sprintf("Archive task %s (%s)?", item.Id, item.Content)) {
				fmt.Fprintln(cmd.OutOrStdout(), "Archive cancelled.")
				return nil
			}
			archived, err := svc.Archive(item.Id)
			if err != nil {
				return err
			}
			if jsonOutput {
				return writeJSON(cmd.OutOrStdout(), map[string]string{"archived_id": archived.Id})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Archived task %s. Restore it with: lazytodo restore %s\n", archived.Id, archived.Id)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip confirmation")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output JSON")
	return cmd
}

// editInteractively opens the Bubble Tea form. It is only reached when the user
// passed no field flags, so scripts and agents keep the non-interactive path.
func (a *app) editInteractively(cmd *cobra.Command, svc *task.Service, taskID string, jsonOutput bool) error {
	if a.runEditForm == nil {
		return fmt.Errorf("interactive editing is not available; pass --content, --status, --star or --unstar")
	}
	item, err := svc.Get(taskID)
	if err != nil {
		return err
	}
	patch, changed, err := a.runEditForm(item)
	if err != nil {
		return err
	}
	if !changed {
		fmt.Fprintln(cmd.OutOrStdout(), "No changes.")
		return nil
	}
	updated, err := svc.Update(taskID, patch)
	if err != nil {
		return err
	}
	return writeTask(cmd.OutOrStdout(), updated, jsonOutput)
}

func (a *app) newRestoreCommand() *cobra.Command {
	var statusValue string
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "restore <id>",
		Short: "Restore an archived task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			status, err := domain.ParseTaskStatus(statusValue)
			if err != nil {
				return err
			}
			svc, err := a.taskService()
			if err != nil {
				return err
			}
			restored, err := svc.Restore(args[0], status)
			if err != nil {
				return err
			}
			return writeTask(cmd.OutOrStdout(), restored, jsonOutput)
		},
	}
	cmd.Flags().StringVarP(&statusValue, "status", "s", "todo", "status to restore into: todo, doing, or done")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output JSON")
	return cmd
}

func (a *app) newPurgeCommand() *cobra.Command {
	var all, yes, jsonOutput bool
	cmd := &cobra.Command{
		Use:   "purge [id]",
		Short: "Permanently delete a task or every archived task",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if all == (len(args) == 1) {
				return fmt.Errorf("specify either an id or --all")
			}
			if jsonOutput && !yes {
				return fmt.Errorf("--json requires --yes when purging")
			}
			svc, err := a.taskService()
			if err != nil {
				return err
			}
			if all {
				return a.purgeAll(cmd, svc, yes, jsonOutput)
			}
			item, err := svc.Get(args[0])
			if err != nil {
				return err
			}
			if !yes && !confirm(cmd, fmt.Sprintf("Permanently delete task %s (%s)? This cannot be undone.", item.Id, item.Content)) {
				fmt.Fprintln(cmd.OutOrStdout(), "Purge cancelled.")
				return nil
			}
			if err := svc.Purge(item.Id); err != nil {
				return err
			}
			if jsonOutput {
				return writeJSON(cmd.OutOrStdout(), map[string]string{"purged_id": item.Id})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Purged task %s.\n", item.Id)
			return nil
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "purge every archived task")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip confirmation")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output JSON")
	return cmd
}

func (a *app) purgeAll(cmd *cobra.Command, svc *task.Service, yes, jsonOutput bool) error {
	archived := domain.TaskStatusArchived
	items, err := svc.List(&archived)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		if jsonOutput {
			return writeJSON(cmd.OutOrStdout(), map[string]int{"purged_count": 0})
		}
		fmt.Fprintln(cmd.OutOrStdout(), "No archived tasks to purge.")
		return nil
	}
	if !yes && !confirm(cmd, fmt.Sprintf("Permanently delete %d archived task(s)? This cannot be undone.", len(items))) {
		fmt.Fprintln(cmd.OutOrStdout(), "Purge cancelled.")
		return nil
	}
	count, err := svc.PurgeArchived()
	if err != nil {
		return err
	}
	if jsonOutput {
		return writeJSON(cmd.OutOrStdout(), map[string]int{"purged_count": count})
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Purged %d archived task(s).\n", count)
	return nil
}

// confirm prompts on stderr so that piping stdout stays machine-readable. A
// read error with no input counts as a decline.
func confirm(cmd *cobra.Command, prompt string) bool {
	fmt.Fprintf(cmd.ErrOrStderr(), "%s [y/N] ", prompt)
	answer, _ := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "y" || answer == "yes"
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
	cmd.Flags().StringVar(&statusValue, "status", "", "filter by task status; archived tasks are hidden unless you pass --status archived")
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
	cmd.Flags().StringVar(&statusValue, "status", "", "filter by task status; archived tasks are hidden unless you pass --status archived")
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
