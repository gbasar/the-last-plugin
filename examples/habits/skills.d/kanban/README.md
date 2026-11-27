# kanban

Work with Claude on a kanban board that it actually keeps up to date.

The board is one Markdown file in the [Obsidian Kanban](https://github.com/mgmeyers/obsidian-kanban)
format: open it in Obsidian, in any editor, or in the terminal with `kanban view`.
lplug hooks remind Claude to update it, so it can't quietly forget.

## Columns

| In | Ready | Blocked | Doing | Done |
|---|---|---|---|---|
| you drop work here | Claude read it: clear and good to go | not good: Claude says what it needs | being worked on | finished |

Those are the defaults. Any board can have its own columns: the first is
intake, the last is done.

## Cards, steps and child boards

```markdown
## Doing

- [ ] Login with OAuth
	- [x] client config
	- [ ] callback route
	- [ ] [[oauth-tests]]
```

Each card holds a checklist that Claude ticks one step at a time, shown as `[1/3]`.
A card too big for a checklist gets its own board (`kanban child oauth-tests`),
linked as `[[oauth-tests]]`; those nest to any depth.

## Which board

1. `$LPLUG_KANBAN`, if set
2. the board picked with `kanban use` for this project (repo)
3. `KANBAN.md` at the repo root (can be a symlink into your vault)

Ad hoc boards (`kanban new trip Ideas Booked Packed`) live in
`$XDG_DATA_HOME/lplug/kanban/` (`~/.local/share/lplug/kanban/`). Point Obsidian
there, or move them into your vault and `kanban use` the new path.

## Talking to Claude

| Say | Claude |
|---|---|
| "use ~/vault/Sprint.md" | makes that the active board |
| "new board for the move: Boxes, Packed, Loaded" | creates it with those columns |
| "show me kanban plan" (after a plan) | turns the plan into a board, shows it, waits for "go" |
| "next" / "work the board" | takes the top Ready card |
| `/kanban` | shows the board |

## Terminal

| Command | Does |
|---|---|
| `kanban view [BOARD]` | the board, numbered, with steps |
| `kanban watch [BOARD]` | `view`, redrawn every 2 s (`KANBAN_WATCH_SECONDS`) |
| `kanban add "text"` | card at the end of the first column |
| `kanban move 3 Ready` / `kanban move "login" Done` | move by number or by text, with its steps |
| `kanban edit` | open the board in `$VISUAL` / `$EDITOR` |
| `kanban use BOARD` / `use --none` | pick the active board / back to the default |
| `kanban new NAME [columns…]` / `child NAME` / `init` | ad hoc board / child board / repo `KANBAN.md` |
| `kanban list` / `path` / `show` | boards, the active path, the agent's summary |

BOARD is a path, or a name looked up next to the active board, then in the data dir.

## Hooks (`rules.d/kanban.ini`)

| Rule | When | Does |
|---|---|---|
| `kanban` | session start | Claude gets the rules and the board (only where a board exists) |
| `kanban-prompt` | each message | shows the board if you changed it; lists untriaged cards in In |
| `kanban-tool` | after each file edit | notes work done; a board edit clears it |
| `kanban-stop` | before Claude stops | files changed but the board didn't: Claude is sent back once to update it |

`lplug off @kanban` turns all of it off; `lplug off kanban-prompt` turns off one.

Only `bash`, POSIX `awk` and `sed` are needed. No jq, no Python.
