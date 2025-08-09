# lazytodo

A fast, keyboard-centric TUI todo app inspired by Taskbook. Organize tasks in a kanban board with three columns: Todo, In Progress, Done. Built with Go and Bubble Tea.

## Features

- Kanban board with Todo, In Progress, Done
- Smooth navigation and editing with vim-like keybindings
- Star tasks; starred items appear first
- Add, edit, delete tasks inline
- Toggle Done quickly (and toggle back)
- Move tasks left/right across columns
- Persistent storage under `~/.lazytodo`
- Configurable layout (horizontal or vertical) with on-the-fly toggle and persistence
- Responsive help shown at the bottom in multiple columns

## Install

Prerequisites: Go 1.21+

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

## Run

```bash
# From source
go run ./cmd/lazytodo

# Or after building
./lazytodo
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

- Data directory: `~/.lazytodo`
  - Tasks: `~/.lazytodo/tasks.json`
  - Config: `~/.lazytodo/config.json`
- The app auto-creates this directory and files as needed.
- Layout choice is remembered between runs (`vertical` setting in config).

## Notes

- The UI uses Bubble Tea + Lip Gloss. Terminal TrueColor support is recommended for best visuals.
- Starred tasks render with a star (★) and are sorted to the top.

## Roadmap (ideas)

- Filtering and search
- Due dates and badges
- Drag-like reordering within a column
- Export/import

## License

MIT
