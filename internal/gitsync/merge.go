package gitsync

import (
	"fmt"
	"os"
	"sort"
	"strconv"

	"github.com/hungtrd/lazytodo/internal/domain"
	"github.com/hungtrd/lazytodo/internal/repository"
	repofs "github.com/hungtrd/lazytodo/internal/repository/fs"
)

// MergeAttribute is the .gitattributes line that routes the tasks file to our
// driver instead of git's line-based merge.
const MergeAttribute = "lazytodo/tasks.jsonl merge=lazytodo"

const (
	mergeDriverName = "merge.lazytodo.name"
	mergeDriverExec = "merge.lazytodo.driver"

	mergeDriverLabel = "lazytodo task merge"
)

// mergeDriverExecutable resolves the binary git should call back into. Tests
// override it to point at a freshly built binary.
var mergeDriverExecutable = func() string {
	// The absolute path of the running binary is used rather than the bare
	// name, so merges keep working when lazytodo is not on PATH or when the
	// copy on PATH is an older build without this subcommand.
	if path, err := os.Executable(); err == nil && path != "" {
		return path
	}
	return "lazytodo"
}

// MergeDriverCommand is the git config value for merge.lazytodo.driver. %O, %A
// and %B are the base, ours and theirs temporary files git passes in.
func MergeDriverCommand() string {
	return fmt.Sprintf("%q sync merge-driver %%O %%A %%B", mergeDriverExecutable())
}

// RegisterMergeDriver points this clone's git config at the driver. It is
// re-asserted on every sync because git config is per clone, so a repository
// cloned on another machine picks it up on first use.
func RegisterMergeDriver(runner *Runner) error {
	if err := runner.ConfigSet(mergeDriverName, mergeDriverLabel); err != nil {
		return err
	}
	return runner.ConfigSet(mergeDriverExec, MergeDriverCommand())
}

// MergeFiles is the entry point for `lazytodo sync merge-driver`. It merges the
// three revisions git hands over and writes the result back to oursPath, which
// is where git expects to find the merged content.
func MergeFiles(basePath, oursPath, theirsPath string) error {
	base, err := readMergeInput(basePath)
	if err != nil {
		return err
	}
	ours, err := readMergeInput(oursPath)
	if err != nil {
		return err
	}
	theirs, err := readMergeInput(theirsPath)
	if err != nil {
		return err
	}

	merged, err := MergeTaskData(base, ours, theirs)
	if err != nil {
		return err
	}
	encoded, err := repofs.EncodeTaskData(merged)
	if err != nil {
		return err
	}
	if err := os.WriteFile(oursPath, encoded, 0o644); err != nil {
		return fmt.Errorf("write merged tasks: %w", err)
	}
	return nil
}

// readMergeInput tolerates a missing or empty stage, which is what git passes
// for the base when a file was added on both sides.
func readMergeInput(path string) (repository.TaskData, error) {
	if path == "" {
		return emptyData(), nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return emptyData(), nil
		}
		return repository.TaskData{}, fmt.Errorf("read %s: %w", path, err)
	}
	if len(raw) == 0 {
		return emptyData(), nil
	}
	return repofs.DecodeTaskData(raw)
}

func emptyData() repository.TaskData {
	tasks := make(map[domain.TaskStatus][]domain.Task, len(domain.AllStatuses()))
	for _, status := range domain.AllStatuses() {
		tasks[status] = []domain.Task{}
	}
	return repository.TaskData{Version: repository.CurrentTaskDataVersion, NextID: 1, Tasks: tasks}
}

// MergeTaskData reconciles two task files against their common ancestor.
//
// Because every task is a self-contained line keyed by a stable id, the merge
// happens per task rather than per line:
//   - changed on both sides, the newer updated_at wins
//   - changed on one side only, that side wins
//   - removed on one side, the removal wins unless the other side edited it,
//     in which case the edit is kept rather than silently dropped
//   - the same short id reused for two different tasks, both are kept and one
//     is renumbered
//
// Tasks are matched on UID, not on the short id, because the short id comes
// from a per-machine counter and two machines working offline will reuse it.
func MergeTaskData(base, ours, theirs repository.TaskData) (repository.TaskData, error) {
	baseByUID := indexByUID(base)
	oursByUID := indexByUID(ours)
	theirsByUID := indexByUID(theirs)

	var survivors []domain.Task
	for _, uid := range unionUIDs(ours, theirs) {
		ourTask, inOurs := oursByUID[uid]
		theirTask, inTheirs := theirsByUID[uid]
		baseTask, inBase := baseByUID[uid]

		switch {
		case inOurs && inTheirs:
			winner := ourTask
			if newerThan(theirTask, ourTask) {
				winner = theirTask
			}
			survivors = append(survivors, winner)
		case inOurs:
			// Absent on their side: either they purged it, or we created it.
			// An untouched copy means they purged it and we should follow.
			if inBase && ourTask == baseTask {
				continue
			}
			survivors = append(survivors, ourTask)
		default:
			if inBase && theirTask == baseTask {
				continue
			}
			survivors = append(survivors, theirTask)
		}
	}

	merged := emptyData()
	merged.NextID = assignUniqueIDs(survivors, max(base.NextID, ours.NextID, theirs.NextID))
	for _, item := range survivors {
		merged.Tasks[item.Status] = append(merged.Tasks[item.Status], item)
	}
	return merged, nil
}

// assignUniqueIDs hands a fresh short id to any task whose id is already taken
// by a different task, and reports the next free counter value. Tasks are
// processed in UID order so both machines renumber the same one.
func assignUniqueIDs(tasks []domain.Task, nextID int64) int64 {
	sort.SliceStable(tasks, func(i, j int) bool { return tasks[i].UID < tasks[j].UID })

	taken := make(map[string]bool, len(tasks))
	var conflicted []int
	var highest int64
	for i, item := range tasks {
		if taken[item.Id] {
			conflicted = append(conflicted, i)
			continue
		}
		taken[item.Id] = true
		if value, err := strconv.ParseInt(item.Id, 10, 64); err == nil && value > highest {
			highest = value
		}
	}

	nextID = max(nextID, highest+1)
	for _, index := range conflicted {
		for taken[strconv.FormatInt(nextID, 10)] {
			nextID++
		}
		id := strconv.FormatInt(nextID, 10)
		tasks[index].Id = id
		taken[id] = true
		nextID++
	}
	return nextID
}

func indexByUID(data repository.TaskData) map[string]domain.Task {
	out := make(map[string]domain.Task)
	for _, status := range domain.AllStatuses() {
		for _, item := range data.Tasks[status] {
			out[uidOf(item)] = item
		}
	}
	return out
}

// unionUIDs lists every task identity on either side, in a deterministic order
// so a merge is reproducible.
func unionUIDs(ours, theirs repository.TaskData) []string {
	seen := make(map[string]bool)
	var uids []string
	for _, data := range []repository.TaskData{ours, theirs} {
		for _, status := range domain.AllStatuses() {
			for _, item := range data.Tasks[status] {
				uid := uidOf(item)
				if !seen[uid] {
					seen[uid] = true
					uids = append(uids, uid)
				}
			}
		}
	}
	return uids
}

// uidOf tolerates a task whose UID was never filled in, which can only happen
// if a file was hand-edited between load and merge.
func uidOf(item domain.Task) string {
	if item.UID != "" {
		return item.UID
	}
	return domain.DeriveUID(item)
}

// newerThan compares by last write, falling back to creation time for tasks
// that were never edited.
func newerThan(candidate, current domain.Task) bool {
	return stamp(candidate) > stamp(current)
}

func stamp(item domain.Task) int64 {
	return max(item.CreatedAt, item.UpdatedAt, item.ArchivedAt)
}
