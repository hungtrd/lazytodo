package cli

import (
	"fmt"
	"runtime"
	"runtime/debug"

	"github.com/spf13/cobra"
)

// version is overridden at build time with
//
//	-ldflags "-X github.com/hungtrd/lazytodo/internal/cli.version=v0.1.0"
//
// Left at its default the value is recovered from the module build info
// instead, which covers an install done with `go install ...@v1.2.3`.
var version = "dev"

type versionInfo struct {
	Version  string `json:"version"`
	Commit   string `json:"commit,omitempty"`
	Built    string `json:"built,omitempty"`
	Go       string `json:"go"`
	Platform string `json:"platform"`
}

// buildVersionInfo takes the build info as arguments rather than reading it
// itself so the resolution rules stay testable without a real build.
func buildVersionInfo(ldflagsVersion string, info *debug.BuildInfo, ok bool) versionInfo {
	resolved := versionInfo{
		Version:  ldflagsVersion,
		Go:       runtime.Version(),
		Platform: runtime.GOOS + "/" + runtime.GOARCH,
	}
	if !ok || info == nil {
		return resolved
	}

	// ldflags wins; the module version only fills in for a plain `go install`,
	// where a local build reports the placeholder "(devel)".
	if resolved.Version == "dev" && info.Main.Version != "" && info.Main.Version != "(devel)" {
		resolved.Version = info.Main.Version
	}

	var revision, modified string
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			revision = setting.Value
		case "vcs.time":
			resolved.Built = setting.Value
		case "vcs.modified":
			modified = setting.Value
		}
	}
	if revision != "" {
		resolved.Commit = shortCommit(revision)
		if modified == "true" {
			resolved.Commit += "-dirty"
		}
	}
	return resolved
}

func shortCommit(revision string) string {
	if len(revision) <= 7 {
		return revision
	}
	return revision[:7]
}

func newVersionCommand() *cobra.Command {
	var verbose bool
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Show the installed lazytodo version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			info, ok := debug.ReadBuildInfo()
			resolved := buildVersionInfo(version, info, ok)

			out := cmd.OutOrStdout()
			// JSON always carries every field: --verbose is about what a person
			// wants to read, not about what a consumer can parse.
			if jsonOutput {
				return writeJSON(out, resolved)
			}
			fmt.Fprintf(out, "lazytodo %s\n", resolved.Version)
			if !verbose {
				return nil
			}
			if resolved.Commit != "" {
				fmt.Fprintf(out, "Commit:   %s\n", resolved.Commit)
			}
			if resolved.Built != "" {
				fmt.Fprintf(out, "Built:    %s\n", resolved.Built)
			}
			fmt.Fprintf(out, "Go:       %s\n", resolved.Go)
			fmt.Fprintf(out, "Platform: %s\n", resolved.Platform)
			return nil
		},
	}
	cmd.Flags().BoolVar(&verbose, "verbose", false, "include commit, build date and Go version")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output JSON")
	return cmd
}
