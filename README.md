# lazytodo

A todo manager with a scriptable Cobra CLI and an optional Bubble Tea TUI. Tasks are organized into Todo, Doing, and Done statuses, with an Archived status for deleted tasks.

## Features

- Create, edit, archive, restore, list, inspect, and search tasks from the CLI
- One row per task in the table view; long or multi-line content is elided, with `show` for the full text
- Deleting archives instead of destroying; `purge` is the only permanent removal
- Git sync: keep tasks in your own git repository and share them across machines
- Interactive edit form (`lazytodo edit <id>` with no flags)
- JSON output for scripts and integrations, plus an OpenClaw skill
- Optional kanban TUI with Todo, Doing, and Done
- Smooth navigation and editing with vim-like keybindings
- Star tasks; starred items appear first
- Toggle Done quickly (and toggle back)
- Move tasks left/right across columns
- Sequential task IDs that are easy to type
- Configurable task storage directory
- Configurable layout (horizontal or vertical) with on-the-fly toggle and persistence
- Responsive help shown at the bottom in multiple columns

## Install

Prerequisites: Go 1.24+

- Via go install (if the repo is public):

  ```bash
  go install github.com/hungtrd/lazytodo/cmd/lazytodo@latest
  ```

- From source:

  ```bash
  git clone https://github.com/hungtrd/lazytodo.git
  cd lazytodo
  go build -o lazytodo ./cmd/lazytodo
  ```

## CLI

```bash
# Show command help
go run ./cmd/lazytodo

# Manage tasks
lazytodo create "Buy milk"
lazytodo create "Fix login" --status doing --star
lazytodo edit 1 --content "Buy oat milk" --status done
lazytodo list --status todo
lazytodo show 1
lazytodo search "milk"

# Check the installed version
lazytodo version
lazytodo version --verbose

# Machine-readable output
lazytodo list --json
lazytodo show 1 --json
```

The table view keeps one row per task: content is cut at the first line break or
at the width the other columns leave over, and the cut is marked with ` ...`.
Use `lazytodo show <id>` for the full text, which prints every line. JSON output
is never truncated.

Run the terminal UI only when requested:

```bash
lazytodo --ui
# From source
go run ./cmd/lazytodo --ui
```

### Interactive edit

`lazytodo edit <id>` with no field flags opens a small Bubble Tea form for the
content (multi-line), status and starred flag. Tab moves between fields, Ctrl+S
saves, Esc cancels.

Passing any of `--content`, `--status`, `--star` or `--unstar` keeps the old
non-interactive behaviour, so scripts and agents are unaffected. Outside a
terminal the form is refused with an error rather than hanging.

## Archive, restore, purge

`delete` archives a task instead of destroying it:

```bash
lazytodo delete 1 --yes        # -> archived
lazytodo list                  # archived tasks are hidden here
lazytodo list --status archived
lazytodo restore 1             # back to todo (--status doing|done to choose)

lazytodo purge 1 --yes         # permanent, cannot be undone
lazytodo purge --all --yes     # permanently remove every archived task
```

The TUI never shows archived tasks; `d`/`backspace`/`delete` archives, and
recovery is done from the CLI.

## Git sync

Store tasks in a git repository so they follow you between machines. Every
change is committed immediately; the push is debounced by a few seconds and
flushed when the TUI exits or a CLI command finishes.

```bash
# SSH remote: uses your existing SSH agent and keys
lazytodo sync init git@github.com:you/lazytodo-data.git

# HTTPS remote: needs a personal access token in the environment
export LAZYTODO_GIT_TOKEN=github_pat_...
lazytodo sync init https://github.com/you/lazytodo-data.git

lazytodo sync status     # remote, branch, ahead/behind, auth mode
lazytodo sync now        # commit, merge and push right now
lazytodo sync disable    # stop syncing; files stay where they are
```

On another machine, run the same `sync init` against the same URL. Tasks already
stored locally are moved into the repository, and the repository holds
`lazytodo/tasks.jsonl` plus a synced `settings.json`.

### How conflicts are resolved

`sync init` registers a custom git merge driver, so concurrent edits are merged
per task rather than per line of text:

- edited on one machine only: that edit wins
- edited on both: the more recent `updated_at` wins
- purged on one machine: the removal wins, unless the other machine edited the
  task in the meantime, in which case the edit is kept
- the same short id given to two different tasks offline: both are kept and one
  is renumbered

Tasks carry a hidden `uid` alongside their short id. The uid is what identifies
a task during a merge; the short id is only a handle for typing, which is why it
can be reassigned. Failures never block a task operation: they are recorded in
`~/.lazytodo/sync.log` and surfaced by `lazytodo sync status`.

### Personal access tokens

The token is read from `$LAZYTODO_GIT_TOKEN` and nothing else. lazytodo never
writes it to `config.json`, never embeds it in the remote URL, and never passes
it on a command line where other processes could read it: it reaches git through
a credential helper that expands the variable inside the git child process.

Give the token the least access that works. On GitHub, create a **fine-grained**
personal access token, set *Repository access* to **Only select repositories**
and pick just your data repository, and grant **Contents: Read and write** and
nothing else. Set an expiry. That way a leaked token can only affect the todo
repository.

For GitLab, pass `--token-user oauth2` to `sync init`.

## Keybindings

- Navigation
  - j/k: move cursor down/up within a column
  - h/l: focus previous/next column
  - g/G: jump to top/bottom in current column
- Task actions
  - n: new task
  - e: edit task
  - s: star/unstar
  - space or x: toggle done (moves to Done; if in Done, moves back to Todo)
  - backspace or delete or d: archive task
  - [ or \\ : move task one column left
  - ] or / : move task one column right
- Layout & app
  - v: toggle layout (horizontal/vertical) and save to config
  - esc: cancel input
  - q or Ctrl+C: quit

## Persistence & Config

- Machine-local config is always stored at `~/.lazytodo/config.json`. It holds the storage root and the git sync settings, and never contains credentials.
- Tasks default to `~/.lazytodo/tasks.jsonl`.
- A custom storage root always gets a `lazytodo` subdirectory. For example, `/mnt/data` resolves to `/mnt/data/lazytodo/tasks.jsonl`.
- Synced preferences (such as the layout) live in `settings.json` beside the tasks file, so they travel with a sync repository.
- Existing task data is migrated when the storage root changes. Use `--force` only when replacing an existing destination file.

```bash
lazytodo config get
lazytodo config get storage-root
lazytodo config set storage-root /mnt/data
lazytodo config reset storage-root
```

### Storage format

Tasks are stored as JSONL (schema version 4): a header line with the schema
version and id counter, then one task per line, ordered by id.

```
{"version":4,"next_id":4}
{"id":"1","uid":"22bf01a328953832","content":"Buy milk","status":"todo","is_starred":false,"created_at":1786524464}
{"id":"2","uid":"14fb72495d3e10cc","content":"Fix login","status":"doing","is_starred":false,"created_at":1786524466}
```

One task per line is what makes git diffs small and per-task merges possible.

Older files are migrated automatically on first run. A pre-v4 `tasks.json` is
converted to `tasks.jsonl`, backed up as `tasks.json.v1.bak`, `.v2.bak` or
`.v3.bak` depending on the version it came from, and then removed.

## OpenClaw integration

`skills/lazytodo/SKILL.md` teaches the [OpenClaw](https://github.com/openclaw/openclaw)
assistant to drive this CLI. Install it with:

```bash
ln -s "$(pwd)/skills/lazytodo" ~/.openclaw/skills/lazytodo
```

The skill documents the JSON output shape and a set of safety rules: ask before
purging, never change config or sync settings, never touch the git token.

**lazytodo does not enforce those rules.** If you want a hard boundary, rely on
the two mechanisms that actually provide one: a fine-grained personal access
token scoped to only the data repository (see above), and OpenClaw's own
approval settings for destructive commands.

## Notes

- The UI uses Bubble Tea + Lip Gloss and adapts to the terminal's detected color profile.
- Starred tasks render with a star (★) and are sorted to the top.
- Status and star colors use the terminal's ANSI palette: Doing is blue, Done is green, and stars are yellow. Color is disabled for non-TTY output and when `NO_COLOR` is set; `CLICOLOR_FORCE=1` forces ANSI output for CLI text commands.

## License

MIT
