# Kanban

The user plans and tracks work with you on a Markdown kanban board (Obsidian
Kanban format). It's the shared to-do list: they drop work in, you keep it
current. Updating it is part of every task, never an afterthought: the user
reads the board, not the chat.

`kanban` is the helper (`~/.claude/skills/kanban/kanban` if not on PATH).
`kanban path` prints the active board; `kanban show [BOARD]` prints a summary.

## Which board

- The active board, first found: `$LPLUG_KANBAN`, the one picked with `kanban use` for this project, `KANBAN.md` at the repo root.
- "Use ~/vault/Sprint.md" or "use the sprint board": `kanban use <path or name>`. Any Obsidian Kanban file works, with its own columns.
- "New board for X" (columns optional): `kanban new <name> [columns…]`. It's saved in `$XDG_DATA_HOME/lplug/kanban/` and becomes active.
- No board and none asked for: don't create one. Mention `kanban init` once if the work is multi-step.

## Columns

Default: **In**, **Ready**, **Blocked**, **Doing**, **Done**. A board's own headings always win. For custom ones, the first column is intake and the last is done; ask once if the middle ones are unclear.

| Column | Means | You |
|---|---|---|
| In | the user's drop zone | triage every card, never work it directly |
| Ready | you read it, it's clear and doable | move cards here when triaging |
| Blocked | unclear, missing info, waiting on something | add `blocked: <what you need>` to the card |
| Doing | being worked on now | one card, two at most |
| Done | finished | `- [x]`, newest at the top |

## Card size: keep the board fun

Cards move slowly and visibly. Don't do a whole board in one go and dump it in Done.

- **A card** is one outcome the user can see or try, about an hour to half a day.
- **Steps** are the card's nested checklist, indented one tab under it. Tick each step as you finish it, not all at the end. Progress shows as `[2/5]`.
- **Too big** (more than ~7 steps, or several days): give it its own board with `kanban child <name>` and link it from the card as `[[name]]`. Child boards nest the same way, to any depth. Tick the parent card when its child board's Done holds everything.
- **Too small** (minutes): fold it into a step of a bigger card.

## When to update

- **New cards in In:** triage them in the same turn. Clear ones go to Ready, broken into steps if big. Unclear ones go to Blocked with the question. Report in one line: `Board: 2 to Ready, 1 Blocked (need the API key).` Don't start them unless asked.
- **Starting work:** move its card to Doing first. If the work has no card, add one.
- **Each step done:** tick it right away.
- **Card done:** move it to Done before you tell the user it's done.
- **Hit a wall:** move it to Blocked with the reason. Then ask, or pick the next Ready card.
- **Work you discover:** add it to the intake column tagged `#suggested`. The user decides; don't triage your own suggestions.
- **"Next" / "work the board":** take the top Ready card.
- **The user changed the board** (e.g. in Obsidian): their version wins. Re-read before editing; never undo their moves.

## "Show me kanban plan"

When the user asks for a plan as a board, or says this after a plan:

1. `kanban new <plan-name>` (or `kanban child` if it belongs under an existing card).
2. Each plan step becomes a card in Ready; small steps are nested as a checklist; big ones get a child board.
3. Real open questions go in Blocked; ideas beyond the plan go in the intake column as `#suggested`.
4. Show it with `kanban view <name>` and give the file path, so they can open it in Obsidian.
5. **Wait for "go".** Don't move anything to Doing until the user says so.

## Editing the file

- Edit in place with your edit tool, moving a card's whole block (the card line and its indented lines). `kanban move "<text>" <Column>` does the same from the shell.
- Keep the front matter, headings, a `**Complete**` line under a heading, and anything from `***` on (the archive and the `%% kanban:settings` block) exactly as they are.
- Cards are `- [ ] short imperative text`; Done cards are `- [x]`.
- Mention board changes in one line, e.g. `Board: login → Done, OAuth callback → Doing.` Don't paste the board into the chat unless asked.
