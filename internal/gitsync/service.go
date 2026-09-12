package gitsync

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/hungtrd/lazytodo/internal/repository"
)

const (
	DefaultRemote     = "origin"
	DefaultBranch     = "main"
	DefaultDebounceMs = 3000
)

type State int

const (
	StateIdle State = iota
	StatePending
	StateSyncing
	StateError
)

// Status is a snapshot for the TUI footer and `sync status`.
type Status struct {
	State    State
	Message  string
	LastPush time.Time
}

// Label renders a short indicator for the TUI footer.
func (s Status) Label() string {
	switch s.State {
	case StatePending:
		return "⇅ pending"
	case StateSyncing:
		return "⇅ syncing"
	case StateError:
		return "⚠ sync error"
	default:
		return "⇅ synced"
	}
}

// Service commits every task change immediately and pushes on a debounce, so
// typing in the TUI is never blocked on the network.
type Service struct {
	runner   *Runner
	remote   string
	branch   string
	autoPush bool
	debounce time.Duration
	logPath  string

	// syncMu serializes network work; Flush blocks on it so that quitting waits
	// for an in-flight push rather than racing it.
	syncMu sync.Mutex

	mu     sync.Mutex
	timer  *time.Timer
	status Status
}

// New builds a syncer for a repository at dir.
func New(dir string, cfg repository.GitSync, logPath string) (*Service, error) {
	if !Available() {
		return nil, fmt.Errorf("git is not installed or not on PATH")
	}
	remoteURL := ""
	runner := NewRunner(dir, Auth{})
	if runner.IsRepo() {
		remoteURL, _ = runner.RemoteURL(remoteName(cfg))
	}

	auth := NewAuth(cfg, remoteURL)
	return &Service{
		runner:   NewRunner(dir, auth),
		remote:   remoteName(cfg),
		branch:   branchName(cfg),
		autoPush: cfg.AutoPush,
		debounce: debounceDuration(cfg),
		logPath:  logPath,
	}, nil
}

func remoteName(cfg repository.GitSync) string {
	if cfg.Remote == "" {
		return DefaultRemote
	}
	return cfg.Remote
}

func branchName(cfg repository.GitSync) string {
	if cfg.Branch == "" {
		return DefaultBranch
	}
	return cfg.Branch
}

func debounceDuration(cfg repository.GitSync) time.Duration {
	ms := cfg.DebounceMs
	if ms <= 0 {
		ms = DefaultDebounceMs
	}
	return time.Duration(ms) * time.Millisecond
}

func (s *Service) Runner() *Runner { return s.runner }

func (s *Service) Remote() string { return s.remote }

func (s *Service) Branch() string { return s.branch }

func (s *Service) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

// StatusLabel satisfies task.Syncer for the UI footer.
func (s *Service) StatusLabel() string { return s.Status().Label() }

// NotifyChanged implements task.ChangeNotifier. The commit is synchronous
// because it is local and fast; only the push is deferred.
func (s *Service) NotifyChanged(reason string) {
	if _, err := s.runner.CommitAll("lazytodo: " + reason); err != nil {
		s.fail("commit", err)
		return
	}
	s.setStatus(StatePending, "")
	if !s.autoPush {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.timer != nil {
		s.timer.Stop()
	}
	s.timer = time.AfterFunc(s.debounce, func() {
		if err := s.Sync(); err != nil {
			s.logf("background sync failed: %v", err)
		}
	})
}

// Flush cancels any pending debounce and syncs right away. Callers use it when
// the TUI exits or a CLI command is about to return.
func (s *Service) Flush() error {
	s.mu.Lock()
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
	pending := s.status.State
	s.mu.Unlock()
	if pending == StateIdle {
		return nil
	}
	return s.Sync()
}

// Sync commits anything outstanding, merges the remote and pushes. It is the
// single place network work happens, and it is serialized.
func (s *Service) Sync() error {
	s.syncMu.Lock()
	defer s.syncMu.Unlock()

	s.setStatus(StateSyncing, "")
	if err := s.prepare(); err != nil {
		s.fail("prepare", err)
		return err
	}
	if _, err := s.runner.CommitAll("lazytodo: sync"); err != nil {
		s.fail("commit", err)
		return err
	}
	if !s.runner.HasCommits() {
		s.setStatus(StateIdle, "")
		return nil
	}
	if err := s.pullLocked(); err != nil {
		s.fail("pull", err)
		return err
	}
	if err := s.runner.Push(s.remote, s.branch); err != nil {
		s.fail("push", err)
		return err
	}
	s.mu.Lock()
	s.status = Status{State: StateIdle, LastPush: time.Now()}
	s.mu.Unlock()
	return nil
}

// Pull brings in remote changes without pushing, used at startup.
func (s *Service) Pull() error {
	s.syncMu.Lock()
	defer s.syncMu.Unlock()
	if err := s.prepare(); err != nil {
		return err
	}
	return s.pullLocked()
}

func (s *Service) pullLocked() error {
	// A fetch failure is not fatal on its own: the branch may not exist on a
	// brand new remote. Push is the authority on whether the network works.
	if err := s.runner.Fetch(s.remote, s.branch); err != nil {
		s.logf("fetch failed: %v", err)
		return nil
	}
	remoteRef := s.remote + "/" + s.branch
	if !s.runner.RefExists(remoteRef) || !s.runner.HasCommits() {
		return nil
	}
	if err := s.runner.Merge(remoteRef); err != nil {
		return fmt.Errorf("%w\nlocal changes were kept; resolve the conflict in %s", err, s.runner.Dir())
	}
	return nil
}

// prepare re-asserts the per-clone git settings the driver needs. It is cheap
// and idempotent, and it is what makes a clone on a second machine work without
// running `sync init` there.
func (s *Service) prepare() error {
	if err := s.runner.Auth().Check(); err != nil {
		return err
	}
	if err := RegisterMergeDriver(s.runner); err != nil {
		return err
	}
	return EnsureMergeAttribute(s.runner.Dir())
}

// AheadBehind reports the local/remote divergence for `sync status`.
func (s *Service) AheadBehind() (ahead, behind int, err error) {
	if !s.runner.HasCommits() {
		return 0, 0, nil
	}
	return s.runner.AheadBehind(s.remote, s.branch)
}

func (s *Service) setStatus(state State, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status.State = state
	s.status.Message = message
}

// fail records the error for the UI and appends it to the sync log. Task
// operations themselves are never failed because of sync trouble.
func (s *Service) fail(stage string, err error) {
	message := s.runner.Auth().Scrub(fmt.Sprintf("%s: %v", stage, err))
	s.mu.Lock()
	s.status.State = StateError
	s.status.Message = message
	s.mu.Unlock()
	s.logf("%s", message)
}

func (s *Service) logf(format string, args ...any) {
	if s.logPath == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(s.logPath), 0o755); err != nil {
		return
	}
	file, err := os.OpenFile(s.logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer file.Close()
	line := s.runner.Auth().Scrub(fmt.Sprintf(format, args...))
	fmt.Fprintf(file, "%s %s\n", time.Now().Format(time.RFC3339), line)
}

// EnsureMergeAttribute writes the .gitattributes entry that routes tasks.jsonl
// to the lazytodo merge driver, leaving any other entries alone.
func EnsureMergeAttribute(dir string) error {
	path := filepath.Join(dir, ".gitattributes")
	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read .gitattributes: %w", err)
	}
	contents := string(existing)
	for line := range strings.SplitSeq(contents, "\n") {
		if strings.TrimSpace(line) == MergeAttribute {
			return nil
		}
	}
	if contents != "" && !strings.HasSuffix(contents, "\n") {
		contents += "\n"
	}
	contents += MergeAttribute + "\n"
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		return fmt.Errorf("write .gitattributes: %w", err)
	}
	return nil
}
