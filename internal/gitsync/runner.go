package gitsync

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// Runner executes git in a fixed directory. Shelling out to the user's own git
// keeps SSH agents, credential helpers, commit signing and ~/.gitconfig working
// without lazytodo reimplementing any of it.
type Runner struct {
	dir  string
	auth Auth
}

func NewRunner(dir string, auth Auth) *Runner { return &Runner{dir: dir, auth: auth} }

func (r *Runner) Dir() string { return r.dir }

func (r *Runner) Auth() Auth { return r.auth }

// Available reports whether a git binary is on PATH.
func Available() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

// run executes a local git command. Network commands go through runNetwork so
// that credentials are only ever attached where they are needed.
func (r *Runner) run(args ...string) (string, error) {
	return r.exec(nil, args)
}

func (r *Runner) runNetwork(args ...string) (string, error) {
	return r.exec(r.auth.Args(), args)
}

func (r *Runner) exec(prefix []string, args []string) (string, error) {
	full := make([]string, 0, len(prefix)+len(args))
	full = append(full, prefix...)
	full = append(full, args...)

	cmd := exec.Command("git", full...)
	cmd.Dir = r.dir
	cmd.Env = append(os.Environ(), r.auth.Env()...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			detail = strings.TrimSpace(stdout.String())
		}
		return "", fmt.Errorf("git %s: %w: %s",
			r.auth.Scrub(strings.Join(args, " ")), err, r.auth.Scrub(detail))
	}
	return strings.TrimSpace(stdout.String()), nil
}

func (r *Runner) IsRepo() bool {
	out, err := r.run("rev-parse", "--is-inside-work-tree")
	return err == nil && out == "true"
}

// Init creates a repository whose initial branch is the configured one, using
// symbolic-ref rather than `init -b` so older git versions keep working.
func (r *Runner) Init(branch string) error {
	if err := os.MkdirAll(r.dir, 0o755); err != nil {
		return fmt.Errorf("create sync directory: %w", err)
	}
	if _, err := r.run("init"); err != nil {
		return err
	}
	_, err := r.run("symbolic-ref", "HEAD", "refs/heads/"+branch)
	return err
}

// Clone fetches url into the runner's directory. It is a no-op guard away from
// clobbering an existing repository.
func (r *Runner) Clone(url, branch string) error {
	if err := os.MkdirAll(r.dir, 0o755); err != nil {
		return fmt.Errorf("create sync directory: %w", err)
	}
	args := []string{"clone", url, "."}
	if branch != "" {
		args = []string{"clone", "--branch", branch, url, "."}
	}
	_, err := r.runNetwork(args...)
	return err
}

// IsEmptyDir reports whether the target directory has no entries yet, which
// decides between clone and init.
func (r *Runner) IsEmptyDir() (bool, error) {
	entries, err := os.ReadDir(r.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return true, nil
		}
		return false, err
	}
	return len(entries) == 0, nil
}

func (r *Runner) SetRemote(name, url string) error {
	if _, err := r.run("remote", "add", name, url); err == nil {
		return nil
	}
	_, err := r.run("remote", "set-url", name, url)
	return err
}

func (r *Runner) RemoteURL(name string) (string, error) {
	return r.run("remote", "get-url", name)
}

func (r *Runner) CurrentBranch() (string, error) {
	return r.run("rev-parse", "--abbrev-ref", "HEAD")
}

func (r *Runner) ConfigSet(key, value string) error {
	_, err := r.run("config", key, value)
	return err
}

// HasChanges reports whether the working tree differs from HEAD.
func (r *Runner) HasChanges() (bool, error) {
	out, err := r.run("status", "--porcelain")
	if err != nil {
		return false, err
	}
	return out != "", nil
}

// CommitAll stages everything and commits. It reports whether a commit was
// actually created, since an unchanged tree is the common case.
func (r *Runner) CommitAll(message string) (bool, error) {
	changed, err := r.HasChanges()
	if err != nil {
		return false, err
	}
	if !changed {
		return false, nil
	}
	if _, err := r.run("add", "-A"); err != nil {
		return false, err
	}
	if _, err := r.run("commit", "-m", message); err != nil {
		return false, err
	}
	return true, nil
}

func (r *Runner) Fetch(remote, branch string) error {
	_, err := r.runNetwork("fetch", remote, branch)
	return err
}

// RefExists reports whether a ref resolves, used to tell a fresh remote from a
// populated one.
func (r *Runner) RefExists(ref string) bool {
	_, err := r.run("rev-parse", "--verify", "--quiet", ref)
	return err == nil
}

// HasCommits reports whether HEAD points at a commit yet.
func (r *Runner) HasCommits() bool { return r.RefExists("HEAD") }

// Merge merges ref, aborting on a conflict the merge driver could not settle so
// the working tree is never left half-merged.
func (r *Runner) Merge(ref string) error {
	if _, err := r.run("merge", "--no-edit", ref); err != nil {
		_, _ = r.run("merge", "--abort")
		return err
	}
	return nil
}

func (r *Runner) Push(remote, branch string) error {
	_, err := r.runNetwork("push", "--set-upstream", remote, branch)
	return err
}

// AheadBehind counts local commits not on the remote tracking ref and vice
// versa. A missing remote ref reports everything as ahead.
func (r *Runner) AheadBehind(remote, branch string) (ahead, behind int, err error) {
	remoteRef := remote + "/" + branch
	if !r.RefExists(remoteRef) {
		out, countErr := r.run("rev-list", "--count", "HEAD")
		if countErr != nil {
			return 0, 0, countErr
		}
		ahead, _ = strconv.Atoi(out)
		return ahead, 0, nil
	}
	out, err := r.run("rev-list", "--left-right", "--count", "HEAD..."+remoteRef)
	if err != nil {
		return 0, 0, err
	}
	fields := strings.Fields(out)
	if len(fields) != 2 {
		return 0, 0, fmt.Errorf("unexpected rev-list output %q", out)
	}
	ahead, _ = strconv.Atoi(fields[0])
	behind, _ = strconv.Atoi(fields[1])
	return ahead, behind, nil
}
