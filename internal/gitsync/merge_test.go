package gitsync

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/hungtrd/lazytodo/internal/domain"
	"github.com/hungtrd/lazytodo/internal/repository"
	repofs "github.com/hungtrd/lazytodo/internal/repository/fs"
)

func data(nextID int64, tasks ...domain.Task) repository.TaskData {
	out := emptyData()
	out.NextID = nextID
	for _, item := range tasks {
		out.Tasks[item.Status] = append(out.Tasks[item.Status], item)
	}
	return out
}

// newTask builds a task carrying a UID, mirroring real data where the store
// fills one in on load and it never changes afterwards.
func newTask(uid, id, content string, status domain.TaskStatus, created, updated int64) domain.Task {
	return domain.Task{
		Id:        id,
		UID:       "uid-" + uid,
		Content:   content,
		Status:    status,
		CreatedAt: created,
		UpdatedAt: updated,
	}
}

// byUID flattens a merge result so assertions do not care which bucket a task
// landed in.
func byUID(t *testing.T, merged repository.TaskData) map[string]domain.Task {
	t.Helper()
	out := map[string]domain.Task{}
	for _, status := range domain.AllStatuses() {
		for _, item := range merged.Tasks[status] {
			out[item.UID] = item
		}
	}
	return out
}

func TestMergeTaskData(t *testing.T) {
	base := data(3,
		newTask("a", "1", "shared", domain.TaskStatusTodo, 100, 0),
		newTask("b", "2", "untouched", domain.TaskStatusTodo, 100, 0),
	)

	for _, tc := range []struct {
		name   string
		ours   repository.TaskData
		theirs repository.TaskData
		want   map[string]string // uid -> expected content; absent means removed
		nextID int64
	}{
		{
			name:   "only local edited",
			ours:   data(3, newTask("a", "1", "local edit", domain.TaskStatusTodo, 100, 200), newTask("b", "2", "untouched", domain.TaskStatusTodo, 100, 0)),
			theirs: base,
			want:   map[string]string{"uid-a": "local edit", "uid-b": "untouched"},
			nextID: 3,
		},
		{
			name:   "only remote edited",
			ours:   base,
			theirs: data(3, newTask("a", "1", "remote edit", domain.TaskStatusTodo, 100, 200), newTask("b", "2", "untouched", domain.TaskStatusTodo, 100, 0)),
			want:   map[string]string{"uid-a": "remote edit", "uid-b": "untouched"},
			nextID: 3,
		},
		{
			name:   "both edited, newer updated_at wins",
			ours:   data(3, newTask("a", "1", "local edit", domain.TaskStatusTodo, 100, 200), newTask("b", "2", "untouched", domain.TaskStatusTodo, 100, 0)),
			theirs: data(3, newTask("a", "1", "remote edit", domain.TaskStatusTodo, 100, 300), newTask("b", "2", "untouched", domain.TaskStatusTodo, 100, 0)),
			want:   map[string]string{"uid-a": "remote edit", "uid-b": "untouched"},
			nextID: 3,
		},
		{
			name:   "new task on each side is kept and next_id takes the max",
			ours:   data(4, newTask("a", "1", "shared", domain.TaskStatusTodo, 100, 0), newTask("b", "2", "untouched", domain.TaskStatusTodo, 100, 0), newTask("c", "3", "from A", domain.TaskStatusTodo, 400, 0)),
			theirs: data(9, newTask("a", "1", "shared", domain.TaskStatusTodo, 100, 0), newTask("b", "2", "untouched", domain.TaskStatusTodo, 100, 0), newTask("d", "8", "from B", domain.TaskStatusDoing, 500, 0)),
			want:   map[string]string{"uid-a": "shared", "uid-b": "untouched", "uid-c": "from A", "uid-d": "from B"},
			nextID: 9,
		},
		{
			// A purge on one machine must not be undone by a machine that
			// simply had not seen it yet.
			name:   "remote purge wins over an untouched local copy",
			ours:   base,
			theirs: data(3, newTask("a", "1", "shared", domain.TaskStatusTodo, 100, 0)),
			want:   map[string]string{"uid-a": "shared"},
			nextID: 3,
		},
		{
			// But an edit is never silently thrown away by a delete.
			name:   "local edit survives a remote purge",
			ours:   data(3, newTask("a", "1", "shared", domain.TaskStatusTodo, 100, 0), newTask("b", "2", "edited after they purged", domain.TaskStatusTodo, 100, 500)),
			theirs: data(3, newTask("a", "1", "shared", domain.TaskStatusTodo, 100, 0)),
			want:   map[string]string{"uid-a": "shared", "uid-b": "edited after they purged"},
			nextID: 3,
		},
		{
			name:   "archive on one side propagates",
			ours:   base,
			theirs: data(3, newTask("a", "1", "shared", domain.TaskStatusTodo, 100, 0), newTask("b", "2", "untouched", domain.TaskStatusArchived, 100, 600)),
			want:   map[string]string{"uid-a": "shared", "uid-b": "untouched"},
			nextID: 3,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			merged, err := MergeTaskData(base, tc.ours, tc.theirs)
			if err != nil {
				t.Fatal(err)
			}
			got := byUID(t, merged)
			if len(got) != len(tc.want) {
				t.Fatalf("merged %d tasks, want %d: %+v", len(got), len(tc.want), got)
			}
			for uid, content := range tc.want {
				item, ok := got[uid]
				if !ok {
					t.Fatalf("task %s is missing from the merge: %+v", uid, got)
				}
				if item.Content != content {
					t.Fatalf("task %s content = %q, want %q", uid, item.Content, content)
				}
			}
			if merged.NextID != tc.nextID {
				t.Fatalf("NextID = %d, want %d", merged.NextID, tc.nextID)
			}
		})
	}
}

// Two machines working offline both allocate the next number from their own
// counter, so different tasks routinely arrive carrying the same short id.
// Neither may be dropped.
func TestMergeRenumbersCollidingShortIDs(t *testing.T) {
	base := data(2, newTask("a", "1", "shared", domain.TaskStatusTodo, 100, 0))
	// Same second, same id, genuinely different tasks: the case a timestamp
	// comparison alone cannot tell apart.
	ours := data(3, newTask("a", "1", "shared", domain.TaskStatusTodo, 100, 0), newTask("b", "2", "from A", domain.TaskStatusTodo, 200, 0))
	theirs := data(3, newTask("a", "1", "shared", domain.TaskStatusTodo, 100, 0), newTask("c", "2", "from B", domain.TaskStatusDoing, 200, 0))

	merged, err := MergeTaskData(base, ours, theirs)
	if err != nil {
		t.Fatal(err)
	}
	got := byUID(t, merged)
	if len(got) != 3 {
		t.Fatalf("merged %d tasks, want 3: %+v", len(got), got)
	}

	ids := map[string]string{}
	for uid, item := range got {
		if previous, taken := ids[item.Id]; taken {
			t.Fatalf("id %s is shared by %s and %s", item.Id, previous, uid)
		}
		ids[item.Id] = uid
	}
	if merged.NextID <= 3 {
		t.Fatalf("NextID = %d, want it advanced past the renumbered task", merged.NextID)
	}

	// Both machines must renumber the same task, or they never converge.
	other, err := MergeTaskData(base, theirs, ours)
	if err != nil {
		t.Fatal(err)
	}
	for uid, item := range byUID(t, other) {
		if got[uid].Id != item.Id {
			t.Fatalf("task %s got id %s one way and %s the other", uid, got[uid].Id, item.Id)
		}
	}
}

// A merge keeps the same tasks whichever machine performs it.
func TestMergeTaskDataConverges(t *testing.T) {
	base := data(2, newTask("a", "1", "shared", domain.TaskStatusTodo, 100, 0))
	ours := data(3, newTask("a", "1", "local", domain.TaskStatusTodo, 100, 200), newTask("b", "2", "from A", domain.TaskStatusTodo, 300, 0))
	theirs := data(4, newTask("a", "1", "remote", domain.TaskStatusTodo, 100, 250), newTask("c", "3", "from B", domain.TaskStatusTodo, 350, 0))

	forward, err := MergeTaskData(base, ours, theirs)
	if err != nil {
		t.Fatal(err)
	}
	backward, err := MergeTaskData(base, theirs, ours)
	if err != nil {
		t.Fatal(err)
	}

	left, right := byUID(t, forward), byUID(t, backward)
	if len(left) != len(right) {
		t.Fatalf("merge is not symmetric: %+v vs %+v", left, right)
	}
	for uid, item := range left {
		if right[uid] != item {
			t.Fatalf("task %s differs by direction: %+v vs %+v", uid, item, right[uid])
		}
	}
}

func TestMergeFilesWritesResultOverOurs(t *testing.T) {
	dir := t.TempDir()
	write := func(name string, contents repository.TaskData) string {
		t.Helper()
		encoded, err := repofs.EncodeTaskData(contents)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, encoded, 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}

	basePath := write("base", data(2, newTask("a", "1", "shared", domain.TaskStatusTodo, 100, 0)))
	oursPath := write("ours", data(3, newTask("a", "1", "shared", domain.TaskStatusTodo, 100, 0), newTask("b", "2", "from A", domain.TaskStatusTodo, 200, 0)))
	theirsPath := write("theirs", data(4, newTask("a", "1", "shared", domain.TaskStatusTodo, 100, 0), newTask("c", "3", "from B", domain.TaskStatusTodo, 300, 0)))

	if err := MergeFiles(basePath, oursPath, theirsPath); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(oursPath)
	if err != nil {
		t.Fatal(err)
	}
	merged, err := repofs.DecodeTaskData(raw)
	if err != nil {
		t.Fatal(err)
	}
	got := byUID(t, merged)
	for _, uid := range []string{"uid-a", "uid-b", "uid-c"} {
		if _, ok := got[uid]; !ok {
			t.Fatalf("task %s missing after MergeFiles: %+v", uid, got)
		}
	}
	if merged.NextID != 4 {
		t.Fatalf("NextID = %d, want 4", merged.NextID)
	}
}

// git passes an empty base when the file was added on both sides.
func TestMergeFilesHandlesMissingBase(t *testing.T) {
	dir := t.TempDir()
	encoded, err := repofs.EncodeTaskData(data(2, newTask("a", "1", "only mine", domain.TaskStatusTodo, 100, 0)))
	if err != nil {
		t.Fatal(err)
	}
	oursPath := filepath.Join(dir, "ours")
	theirsPath := filepath.Join(dir, "theirs")
	if err := os.WriteFile(oursPath, encoded, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(theirsPath, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := MergeFiles(filepath.Join(dir, "does-not-exist"), oursPath, theirsPath); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(oursPath)
	if err != nil {
		t.Fatal(err)
	}
	merged, err := repofs.DecodeTaskData(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(byUID(t, merged)) != 1 {
		t.Fatalf("unexpected merge result: %+v", merged.Tasks)
	}
}

func TestEnsureMergeAttributeIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".gitattributes")
	if err := os.WriteFile(path, []byte("*.md text\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := EnsureMergeAttribute(dir); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "*.md text\n" + MergeAttribute + "\n"
	if string(raw) != want {
		t.Fatalf(".gitattributes = %q, want %q", raw, want)
	}
}
