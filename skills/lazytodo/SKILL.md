---
name: lazytodo
description: Manage the user's local todo list with the lazytodo CLI - create, list, search, edit, archive and restore tasks across todo/doing/done.
homepage: https://github.com/hungtrd/lazytodo
metadata: {"openclaw":{"requires":{"bins":["lazytodo"]},"os":["darwin","linux"],"emoji":"✅"}}
---

# lazytodo

`lazytodo` is a command line todo list. Tasks move through `todo`, `doing` and
`done`, and deleted tasks go to `archived` rather than disappearing.

## Safety rules

Read these before running anything. lazytodo does **not** enforce them; they are
your responsibility.

- **`purge` is permanent and cannot be undone. Always ask the user first.**
  Never run `lazytodo purge --all` on your own initiative.
- **Never run `config set`, `config reset`, `sync init`, `sync enable` or
  `sync disable`.** These change where the user's data lives and how it is
  synced. Suggest the command and let the user run it.
- **Never read, print, echo or copy `LAZYTODO_GIT_TOKEN`**, and do not run
  `env`, `printenv` or similar to look for it. It is a git credential.
- `delete` is safe: it archives, and `restore` brings the task back.
- Archived tasks are hidden from `list` unless you pass `--status archived`. If
  a task seems to have vanished, check there before telling the user it is gone.

## Using the CLI

Always pass `--json`. Every command below supports it and returns machine
readable output on stdout; errors go to stderr with exit status 1.

| Command | What it does |
| --- | --- |
| `lazytodo create <content> [--status todo\|doing\|done] [--star] --json` | Create a task. Aliases: `add`, `new`. |
| `lazytodo list [--status <status>] --json` | List tasks. Without `--status`, archived tasks are excluded. Alias: `ls`. |
| `lazytodo show <id> --json` | One task in full. Alias: `detail`. |
| `lazytodo search <query> [--status <status>] --json` | Case-insensitive substring match on content. |
| `lazytodo edit <id> --content <text> --json` | Change the text. |
| `lazytodo edit <id> --status <status> --json` | Move between statuses. `-s` is short for `--status`. |
| `lazytodo edit <id> --star --json` / `--unstar --json` | Star or unstar. Starred tasks sort first. |
| `lazytodo delete <id> --yes --json` | Archive a task. Aliases: `del`, `rm`. |
| `lazytodo restore <id> [--status todo\|doing\|done] --json` | Bring an archived task back; defaults to `todo`. |
| `lazytodo purge <id> --yes --json` | **Destructive.** Ask the user first. |
| `lazytodo sync status --json` | Read-only check of git sync state. |
| `lazytodo version --json` | Version of the running binary. Add `--verbose` for commit, build date, Go version and platform. |

Notes that will otherwise trip you up:

- **`delete` and `purge` need `--yes` whenever you pass `--json`.** Without
  `--yes` they prompt on stderr and wait for input, which will hang.
- **`lazytodo edit <id>` with no field flag opens an interactive terminal
  form.** Always pass at least one of `--content`, `--status`, `--star` or
  `--unstar`.
- Multi-word content does not need quoting, but quoting is safer:
  `lazytodo create "Buy oat milk" --json`.
- Without `--json`, the table view truncates long or multi-line content and
  marks it with ` ...`. With `--json` you always get the whole text, which is
  another reason to always pass it.
- Ids are short numbers. They are stable on one machine, but git sync may
  renumber a task if two machines created tasks offline at the same time, so
  re-read ids with `list` rather than caching them across a conversation.

## Output shape

`create`, `show`, `edit` and `restore` return one object; `list` and `search`
return an array of them:

```json
{
  "id": "3",
  "content": "Write the release notes",
  "status": "todo",
  "is_starred": false,
  "created_at": 1786524466,
  "updated_at": 1786524476
}
```

- `status` is always one of `"todo"`, `"doing"`, `"done"`, `"archived"`.
- Timestamps are Unix seconds. `updated_at` and `archived_at` are **absent**
  rather than zero when unset, so check for the key before reading it. An
  archived task carries `"archived_at"`; a never-edited task has no
  `"updated_at"`.
- `delete` returns `{"archived_id": "3"}`, `purge` returns `{"purged_id": "3"}`,
  and `purge --all` returns `{"purged_count": 2}`.

## Worked examples

```bash
# What is on the list right now?
lazytodo list --json

# Add something and start working on it.
lazytodo create "Review the sync PR" --json
lazytodo edit 4 --status doing --json

# Finish it.
lazytodo edit 4 --status done --json

# Clear it off the board without losing it.
lazytodo delete 4 --yes --json

# The user asks where a task went.
lazytodo list --status archived --json
lazytodo restore 4 --json

# Find something by text.
lazytodo search "release notes" --json
```

## When not to use this skill

- The user is asking about a todo list in another tool (Things, Todoist, a
  GitHub issue tracker). This skill only manages the local lazytodo store.
- The user wants to browse visually. Suggest `lazytodo --ui`, which opens a
  kanban board, and let them run it themselves; it takes over the terminal.
