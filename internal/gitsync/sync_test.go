package gitsync

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/hungtrd/lazytodo/internal/domain"
	"github.com/hungtrd/lazytodo/internal/repository"
	repofs "github.com/hungtrd/lazytodo/internal/repository/fs"
	"github.com/hungtrd/lazytodo/internal/task"
)

// requireGit skips the suite on machines without git rather than failing.
func requireGit(t *testing.T) {
	t.Helper()
	if !Available() {
		t.Skip("git is not installed")
	}
}

// buildLazytodo compiles the binary git calls back into for merges. Without it
// the driver would fall back to whatever `lazytodo` happens to be on PATH,
// which is exactly the stale-binary case this test needs to rule out.
func useBuiltMergeDriver(t *testing.T) {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "lazytodo")
	build := exec.Command("go", "build", "-o", binary, "github.com/hungtrd/lazytodo/cmd/lazytodo")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build lazytodo: %v\n%s", err, out)
	}
	previous := mergeDriverExecutable
	mergeDriverExecutable = func() string { return binary }
	t.Cleanup(func() { mergeDriverExecutable = previous })
}

// gitInDir runs git for test setup, where a failure means the fixture is broken.
func gitInDir(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
	}
}

// newBareRemote creates a repository the clones can push to, standing in for
// GitHub without any network access.
func newBareRemote(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gitInDir(t, dir, "init", "--bare", "--initial-branch=main", ".")
	return dir
}

// machine is one checkout wired to a task service, i.e. one of the user's
// computers.
type machine struct {
	dir    string
	svc    *task.Service
	syncer *Service
}

func newMachine(t *testing.T, name, remoteURL string) *machine {
	t.Helper()
	dir := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	cfg := repository.GitSync{Enabled: true, AutoPush: false, Branch: "main", Remote: "origin"}
	runner := NewRunner(dir, NewAuth(cfg, remoteURL))
	if err := runner.Clone(remoteURL, ""); err != nil {
		t.Fatalf("clone: %v", err)
	}
	// Identity is not configured in CI sandboxes, and commits would fail.
	gitInDir(t, dir, "config", "user.email", name+"@example.test")
	gitInDir(t, dir, "config", "user.name", name)
	gitInDir(t, dir, "checkout", "-B", "main")

	syncer, err := New(dir, cfg, filepath.Join(dir, "..", name+"-sync.log"))
	if err != nil {
		t.Fatal(err)
	}

	tasksPath := filepath.Join(dir, "lazytodo", "tasks.jsonl")
	svc := task.NewService(
		repofs.NewTaskStoreAt(tasksPath),
		&stubConfigRepo{},
		repofs.NewSettingsStoreFor(tasksPath),
	)
	svc.SetSyncer(syncer)
	return &machine{dir: dir, svc: svc, syncer: syncer}
}

type stubConfigRepo struct{ cfg repository.Config }

func (r *stubConfigRepo) Load() (repository.Config, error) { return r.cfg, nil }
func (r *stubConfigRepo) Save(cfg repository.Config) error { r.cfg = cfg; return nil }

func (m *machine) contents(t *testing.T) map[string]string {
	t.Helper()
	if _, err := m.svc.Load(); err != nil {
		t.Fatal(err)
	}
	items, err := m.svc.List(nil)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, item := range items {
		out[item.Id] = item.Content
	}
	return out
}

// Two machines editing different tasks must both survive the round trip. This
// is the case a plain text merge would turn into a conflict, and the whole
// reason tasks are stored one per line.
func TestTwoMachinesConvergeThroughRemote(t *testing.T) {
	requireGit(t)
	useBuiltMergeDriver(t)
	remote := newBareRemote(t)

	alice := newMachine(t, "alice", remote)
	if _, err := alice.svc.Create("from alice", domain.TaskStatusTodo, false); err != nil {
		t.Fatal(err)
	}
	if err := alice.syncer.Sync(); err != nil {
		t.Fatalf("alice sync: %v", err)
	}

	bob := newMachine(t, "bob", remote)
	if got := bob.contents(t); got["1"] != "from alice" {
		t.Fatalf("bob did not receive alice's task: %+v", got)
	}

	// Both add a task without seeing the other's work.
	if _, err := bob.svc.Create("from bob", domain.TaskStatusDoing, false); err != nil {
		t.Fatal(err)
	}
	if _, err := alice.svc.Create("also from alice", domain.TaskStatusTodo, false); err != nil {
		t.Fatal(err)
	}

	if err := bob.syncer.Sync(); err != nil {
		t.Fatalf("bob sync: %v", err)
	}
	if err := alice.syncer.Sync(); err != nil {
		t.Fatalf("alice second sync: %v", err)
	}
	if err := bob.syncer.Sync(); err != nil {
		t.Fatalf("bob second sync: %v", err)
	}

	aliceTasks := alice.contents(t)
	bobTasks := bob.contents(t)
	if len(aliceTasks) != 3 {
		t.Fatalf("alice has %d tasks, want 3: %+v", len(aliceTasks), aliceTasks)
	}
	if len(bobTasks) != len(aliceTasks) {
		t.Fatalf("machines diverged: alice=%+v bob=%+v", aliceTasks, bobTasks)
	}
	for id, content := range aliceTasks {
		if bobTasks[id] != content {
			t.Fatalf("task %s differs: alice=%q bob=%q", id, content, bobTasks[id])
		}
	}

	// Ids stay unique even though both machines allocated from their own
	// counter while offline.
	seen := map[string]bool{}
	for _, content := range aliceTasks {
		if seen[content] {
			t.Fatalf("duplicate task content after merge: %+v", aliceTasks)
		}
		seen[content] = true
	}
}

// An archive on one machine has to reach the other, and stay out of its board.
func TestArchivePropagatesBetweenMachines(t *testing.T) {
	requireGit(t)
	useBuiltMergeDriver(t)
	remote := newBareRemote(t)

	alice := newMachine(t, "alice", remote)
	created, err := alice.svc.Create("finish the report", domain.TaskStatusTodo, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := alice.syncer.Sync(); err != nil {
		t.Fatal(err)
	}

	bob := newMachine(t, "bob", remote)
	if _, err := bob.svc.Archive(created.Id); err != nil {
		t.Fatal(err)
	}
	if err := bob.syncer.Sync(); err != nil {
		t.Fatal(err)
	}

	if err := alice.syncer.Sync(); err != nil {
		t.Fatal(err)
	}
	if got := alice.contents(t); len(got) != 0 {
		t.Fatalf("archived task is still active on alice: %+v", got)
	}
	archived := domain.TaskStatusArchived
	items, err := alice.svc.List(&archived)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Id != created.Id {
		t.Fatalf("archived task did not reach alice: %+v", items)
	}
}

// The merge driver has to be registered per clone; a machine that never ran
// `sync init` still needs conflicting edits merged rather than rejected.
func TestSyncRegistersMergeDriverOnEachClone(t *testing.T) {
	requireGit(t)
	useBuiltMergeDriver(t)
	remote := newBareRemote(t)

	alice := newMachine(t, "alice", remote)
	created, err := alice.svc.Create("shared task", domain.TaskStatusTodo, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := alice.syncer.Sync(); err != nil {
		t.Fatal(err)
	}

	bob := newMachine(t, "bob", remote)
	if err := bob.syncer.Sync(); err != nil {
		t.Fatal(err)
	}

	// Both edit the same task offline. Bob's edit is newer.
	content := "edited by alice"
	if _, err := alice.svc.Update(created.Id, task.Patch{Content: &content}); err != nil {
		t.Fatal(err)
	}
	bobContent := "edited by bob"
	if _, err := bob.svc.Update(created.Id, task.Patch{Content: &bobContent}); err != nil {
		t.Fatal(err)
	}

	if err := alice.syncer.Sync(); err != nil {
		t.Fatal(err)
	}
	// Bob is behind and has a conflicting edit: without the driver this merge
	// would fail outright.
	if err := bob.syncer.Sync(); err != nil {
		t.Fatalf("bob could not merge a conflicting edit: %v", err)
	}
	if got := bob.contents(t); len(got) != 1 {
		t.Fatalf("merge lost or duplicated the task: %+v", got)
	}

	driver, err := bob.syncer.Runner().run("config", "--get", mergeDriverExec)
	if err != nil || driver != MergeDriverCommand() {
		t.Fatalf("merge driver = %q (%v), want %q", driver, err, MergeDriverCommand())
	}
	attributes, err := os.ReadFile(filepath.Join(bob.dir, ".gitattributes"))
	if err != nil {
		t.Fatal(err)
	}
	if string(attributes) == "" {
		t.Fatal(".gitattributes was not written")
	}
}
