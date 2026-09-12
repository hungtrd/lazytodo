package cli

import (
	"fmt"
	"path/filepath"
	"strings"

	appconfig "github.com/hungtrd/lazytodo/internal/config"
	"github.com/hungtrd/lazytodo/internal/gitsync"
	"github.com/hungtrd/lazytodo/internal/repository"
	repofs "github.com/hungtrd/lazytodo/internal/repository/fs"
	"github.com/spf13/cobra"
)

func (a *app) newSyncCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "sync",
		Short: "Sync tasks with a git repository",
		Long: "Store tasks in a git repository so they follow you across machines.\n\n" +
			"Every change is committed immediately and pushed shortly after. HTTPS remotes\n" +
			"authenticate with a personal access token read from $" + gitsync.DefaultTokenEnv + ";\n" +
			"the token is never written to disk or passed on the command line.",
	}
	command.AddCommand(
		a.newSyncInitCommand(),
		a.newSyncStatusCommand(),
		a.newSyncNowCommand(),
		a.newSyncToggleCommand("enable", true),
		a.newSyncToggleCommand("disable", false),
		newSyncMergeDriverCommand(),
	)
	return command
}

func (a *app) newSyncInitCommand() *cobra.Command {
	var path, branch, remote, tokenUser string
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "init <git-url>",
		Short: "Connect task storage to a git repository",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			url := strings.TrimSpace(args[0])
			if url == "" {
				return fmt.Errorf("git URL is empty")
			}
			if !gitsync.Available() {
				return fmt.Errorf("git is not installed or not on PATH")
			}

			cfg, err := a.configRepo.Load()
			if err != nil {
				return err
			}
			git := repository.GitSync{
				Enabled:   true,
				AutoPush:  true,
				Remote:    remote,
				Branch:    branch,
				TokenUser: tokenUser,
			}
			// Fail before touching anything if HTTPS auth cannot work.
			auth := gitsync.NewAuth(git, url)
			if err := auth.Check(); err != nil {
				return err
			}

			root, err := resolveSyncPath(path)
			if err != nil {
				return err
			}
			runner := gitsync.NewRunner(root, auth)
			if err := prepareRepo(runner, url, branch, remote); err != nil {
				return err
			}
			if err := gitsync.EnsureMergeAttribute(root); err != nil {
				return err
			}
			if err := gitsync.RegisterMergeDriver(runner); err != nil {
				return err
			}

			// SetStorageRoot moves any existing tasks file into the repository.
			cfg, err = appconfig.NewService(a.configRepo).SetStorageRoot(root, false)
			if err != nil {
				return err
			}
			cfg.Git = &git
			if err := a.configRepo.Save(cfg); err != nil {
				return err
			}

			syncer, err := a.syncer(cfg)
			if err != nil {
				return err
			}
			if err := syncer.Sync(); err != nil {
				return err
			}
			if jsonOutput {
				return writeJSON(cmd.OutOrStdout(), map[string]string{
					"storage_root": root,
					"remote":       syncer.Remote(),
					"branch":       syncer.Branch(),
					"auth":         auth.Describe(),
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Syncing tasks with %s (%s/%s).\nStorage root: %s\nAuth: %s\n",
				url, syncer.Remote(), syncer.Branch(), root, auth.Describe())
			return nil
		},
	}
	cmd.Flags().StringVar(&path, "path", "", "local directory for the sync repository (default ~/.lazytodo/sync)")
	cmd.Flags().StringVar(&branch, "branch", gitsync.DefaultBranch, "branch to sync")
	cmd.Flags().StringVar(&remote, "remote", gitsync.DefaultRemote, "remote name")
	cmd.Flags().StringVar(&tokenUser, "token-user", gitsync.DefaultTokenUser, "username paired with the token (use oauth2 for GitLab)")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output JSON")
	return cmd
}

func (a *app) newSyncStatusCommand() *cobra.Command {
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show sync configuration and state",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := a.configRepo.Load()
			if err != nil {
				return err
			}
			if cfg.Git == nil || !cfg.Git.Enabled {
				if jsonOutput {
					return writeJSON(cmd.OutOrStdout(), map[string]any{"enabled": false})
				}
				fmt.Fprintln(cmd.OutOrStdout(), "Sync is not enabled. Run: lazytodo sync init <git-url>")
				return nil
			}
			syncer, err := a.syncer(cfg)
			if err != nil {
				return err
			}
			url, _ := syncer.Runner().RemoteURL(syncer.Remote())
			ahead, behind, err := syncer.AheadBehind()
			if err != nil {
				return err
			}
			dirty, err := syncer.Runner().HasChanges()
			if err != nil {
				return err
			}

			if jsonOutput {
				return writeJSON(cmd.OutOrStdout(), map[string]any{
					"enabled": true,
					// The token value is deliberately absent: auth only ever
					// reports whether the environment variable is populated.
					"auth":         syncer.Runner().Auth().Describe(),
					"storage_root": cfg.StorageRoot,
					"remote":       syncer.Remote(),
					"remote_url":   url,
					"branch":       syncer.Branch(),
					"ahead":        ahead,
					"behind":       behind,
					"dirty":        dirty,
					"auto_push":    cfg.Git.AutoPush,
				})
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Storage root: %s\n", cfg.StorageRoot)
			fmt.Fprintf(out, "Remote:       %s (%s)\n", syncer.Remote(), url)
			fmt.Fprintf(out, "Branch:       %s\n", syncer.Branch())
			fmt.Fprintf(out, "Auth:         %s\n", syncer.Runner().Auth().Describe())
			fmt.Fprintf(out, "Auto push:    %t\n", cfg.Git.AutoPush)
			fmt.Fprintf(out, "Ahead/behind: %d/%d\n", ahead, behind)
			fmt.Fprintf(out, "Uncommitted:  %t\n", dirty)
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output JSON")
	return cmd
}

func (a *app) newSyncNowCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "now",
		Short: "Commit, merge and push right away",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, syncer, err := a.enabledSyncer()
			if err != nil {
				return err
			}
			if err := syncer.Sync(); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Sync complete.")
			return nil
		},
	}
	return cmd
}

func (a *app) newSyncToggleCommand(name string, enable bool) *cobra.Command {
	short := "Resume automatic commit and push"
	if !enable {
		short = "Stop automatic commit and push (files are left in place)"
	}
	return &cobra.Command{
		Use:   name,
		Short: short,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := a.configRepo.Load()
			if err != nil {
				return err
			}
			if cfg.Git == nil {
				return fmt.Errorf("sync is not configured; run: lazytodo sync init <git-url>")
			}
			cfg.Git.Enabled = enable
			cfg.Git.AutoPush = enable
			if err := a.configRepo.Save(cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Sync %sd.\n", name)
			return nil
		},
	}
}

// newSyncMergeDriverCommand is invoked by git itself, never by a person, so it
// stays out of the help listing.
func newSyncMergeDriverCommand() *cobra.Command {
	return &cobra.Command{
		Use:    "merge-driver <base> <ours> <theirs>",
		Short:  "Merge two revisions of a tasks file (used by git)",
		Args:   cobra.ExactArgs(3),
		Hidden: true,
		RunE: func(_ *cobra.Command, args []string) error {
			return gitsync.MergeFiles(args[0], args[1], args[2])
		},
	}
}

// resolveSyncPath defaults the repository location to ~/.lazytodo/sync.
func resolveSyncPath(path string) (string, error) {
	if strings.TrimSpace(path) != "" {
		return repofs.ResolveStorageRoot(path)
	}
	configPath, err := repofs.ConfigFilePath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(configPath), "sync"), nil
}

// prepareRepo clones into an empty directory, or initialises one in place and
// points it at the remote when the directory already holds files.
func prepareRepo(runner *gitsync.Runner, url, branch, remote string) error {
	if runner.IsRepo() {
		return runner.SetRemote(remote, url)
	}
	empty, err := runner.IsEmptyDir()
	if err != nil {
		return err
	}
	if empty {
		if err := runner.Clone(url, ""); err == nil {
			return nil
		}
		// A brand new remote has nothing to clone; fall through to init.
	}
	if err := runner.Init(branch); err != nil {
		return err
	}
	return runner.SetRemote(remote, url)
}
