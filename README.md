# lazytodo

A todo manager with a scriptable Cobra CLI and an optional Bubble Tea TUI. Tasks are organized into Todo, In Progress, and Done statuses.

## Features

- Create, edit, delete, list, inspect, and search tasks from the CLI
- JSON output for scripts and integrations
- Optional kanban TUI with Todo, In Progress, and Done
- Smooth navigation and editing with vim-like keybindings
- Star tasks; starred items appear first
- Add, edit, delete tasks inline
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
lazytodo create "Fix login" --status in-progress --star
lazytodo edit 1 --content "Buy oat milk" --status done
lazytodo list --status todo
lazytodo show 1
lazytodo search "milk"
lazytodo delete 1 --yes

# Machine-readable output
lazytodo list --json
lazytodo show 1 --json
```

Run the terminal UI only when requested:

```bash
lazytodo --ui
# From source
go run ./cmd/lazytodo --ui
```

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
  - backspace or delete or d: remove task
  - [ or \\ : move task one column left
  - ] or / : move task one column right
- Layout & app
  - v: toggle layout (horizontal/vertical) and save to config
  - esc: cancel input
  - q or Ctrl+C: quit

## Persistence & Config

- Config is always stored at `~/.lazytodo/config.json`.
- Tasks default to `~/.lazytodo/tasks.json`.
- A custom storage root always gets a `lazytodo` subdirectory. For example, `/mnt/data` resolves to `/mnt/data/lazytodo/tasks.json`.
- Existing task data is migrated when the storage root changes. Use `--force` only when replacing an existing destination file.

```bash
lazytodo config get
lazytodo config get storage-root
lazytodo config set storage-root /mnt/data
lazytodo config reset storage-root
```

- Legacy task files are migrated automatically to the versioned schema with sequential IDs. The original file is retained as `tasks.json.v1.bak`.
- Layout choice is remembered between runs (`vertical` setting in config).

## Notes

- The UI uses Bubble Tea + Lip Gloss. Terminal TrueColor support is recommended for best visuals.
- Starred tasks render with a star (★) and are sorted to the top.

## License

MIT
