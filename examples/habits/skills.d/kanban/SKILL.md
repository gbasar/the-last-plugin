---
name: kanban
description: >
  Work from a Markdown kanban board (Obsidian Kanban format, also usable from
  the terminal): triage the user's cards, keep Doing and Done current, tick
  nested steps, link child boards for big cards. Use when the user says
  "kanban", "board", "show me kanban plan", "use <board>", "new board",
  "work the board", "what's next", or /kanban.
argument-hint: "[use BOARD | new NAME [columns...] | plan | show]"
license: MIT
---

# Kanban

Requested: `$ARGUMENTS`

1. If the rules in [RULES.md](RULES.md) are not already in your context, read them now. Follow them for the rest of the session.
2. Act on the request:
   - empty or `show`: run `kanban view` and summarize it in one line per column.
   - `use BOARD`: `kanban use BOARD`, then show it.
   - `new NAME [columns...]`: `kanban new NAME [columns...]`, then show it.
   - `plan`: turn the current plan into a board, as in "Show me kanban plan" in RULES.md, and wait for "go".
3. The helper is `kanban` on PATH, or [kanban](kanban) in this folder.
