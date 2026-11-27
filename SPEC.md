# lplug contract (v0.3)

This is the language- and agent-neutral contract. The Go code in this repo is
one implementation of it. Anything that follows this document is compatible:
other dispatchers, adapters for other agents, and sources in any language.

## 1. Events

| neutral          | meaning                                   | Claude Code native   |
|------------------|-------------------------------------------|----------------------|
| `session_start`  | session starts, resumes or is cleared     | `SessionStart`       |
| `prompt_submit`  | the user sent a prompt                    | `UserPromptSubmit`   |
| `tool_before`    | before a tool runs (can block or confirm) | `PreToolUse`         |
| `tool_after`     | after a tool succeeded                    | `PostToolUse`        |
| `tool_failed`    | after a tool failed                       | `PostToolUseFailure` |
| `subagent_start` | a subagent starts                         | `SubagentStart`      |
| `notification`   | agent notification                        | `Notification`       |
| `stop`           | the agent is about to finish its turn     | `Stop`               |
| `subagent_stop`  | a subagent is about to finish             | `SubagentStop`       |
| `compact_before` | before context compaction                 | `PreCompact`         |
| `session_end`    | session ends                              | `SessionEnd`         |

A native event with no mapping uses its snake_case name. Names compare
case-insensitively, ignoring `_`, `-` and spaces, and rules may use either form.

**Subject**, the text that `when` and `unless` match against, is whichever applies:
- the prompt
- for `stop`, the agent's last message
- the shell command
- the compact tool-input JSON
- the notification text

A `stop` that a hook already continued (Claude Code's `stop_hook_active`) is
skipped entirely, so stop rules can't loop.

## 2. Layers and settings

| layer    | where | relative paths in its files are relative to |
|----------|-------|---------------------------------------------|
| personal | `$LPLUG_CONFIG_DIR`, else `$XDG_CONFIG_HOME/lplug`, else `~/.config/lplug` | the config dir |
| project  | the nearest `<dir>/.config/lplug` above the working directory, excluding `~/.config/lplug` | the project root |

Setting precedence: **env var > project `settings.ini` > personal `settings.ini` > default.**

```ini
[lplug]
state_dir = …      ; LPLUG_STATE_DIR    default $XDG_STATE_HOME/lplug
text_path = …      ; LPLUG_TEXT_PATH    default <project>/text, then <personal>/text
rules     = …      ; LPLUG_RULES        default <layer>/rules.d in each layer
timeout   = 10s    ; LPLUG_TIMEOUT
debug     = false  ; LPLUG_DEBUG

[sources]
<event> = …        ; LPLUG_<EVENT>      default <layer>/<event>.d in each layer
```

Lists use the OS list separator (`:`, or `;` on Windows). An env var replaces
the value from every layer; otherwise each layer contributes.

## 3. Sources

For an event, sources are read in this order: the event's sources (personal layer, then project), then the rule sources.
- Directories are read non-recursively, in byte order of filename.
- Names starting with `.` or ending in `~` or `.swp` are ignored.
- A file reached twice is used once.

**Trust:** a file is only used if it's owned by the current user or root and isn't writable by group or others.

| type      | recognized by                           | effect |
|-----------|-----------------------------------------|--------|
| rule file | `.ini` `.toml` `.json` `.conf` `.hocon` | rules (§5) |
| text      | `.md` `.txt`                            | injected as context |
| program   | executable bit (Windows: .exe .bat .cmd) | run (§4) |

## 4. Programs

| channel | meaning |
|---|---|
| stdin | the agent's **native** payload, unchanged |
| `LPLUG_EVENT_FILE` | the normalized event (§7) as JSON |
| env | `LPLUG_AGENT` `LPLUG_EVENT` `LPLUG_NATIVE_EVENT` `LPLUG_SESSION_ID` `LPLUG_TOOL_NAME` `LPLUG_SOURCE` `LPLUG_RULE` |
| cwd | the session's working directory |
| exit 0 | stdout is context. The agent's own hook JSON is interpreted, including deny and ask. |
| exit 2 | block; stderr is the reason |
| other | logged and ignored |
| timeout | the whole process group is killed |

## 5. Rules

Top-level keys are rule ids, and each holds the settings below. Rules apply in
id order. A project rule replaces a personal rule with the same id, in place.

| key | meaning |
|---|---|
| `event` | **required**; one or more names |
| `agent`, `tags`, `enabled`, `description` | filtering, grouping, default state, label |
| `tool` | RE2 regex the tool name must match |
| `when` / `unless` | RE2 regex the subject must / must not match |
| `every` | fire on the Nth, 2Nth, … matching event in a session |
| `once` | at most once per session |
| `text` | add context |
| `file` | add a file's contents. A relative name is looked up on the text path, then in the rule's layer. |
| `ask` | ask the agent to state its intent on a line starting `INTENT:` |
| `run` | run a program (§4): `lplug …` runs the dispatcher itself, a bare name is looked up on PATH, a path is relative to the layer |
| `confirm` | ask the user to approve (tool events); where the agent can't ask, it blocks |
| `block` | block with this reason |

Actions apply in the order text, file, ask, run, confirm, block.
- TOML rule files must be flat tables of `key = value`.
- In INI files, ` ;` or ` #` starts an inline comment in an unquoted value.

**Counting:** each process appends a unique token (`<event>:<tool_use_id>`) to
the rule's counter file. Its position is the line number its token landed on.
No locks are needed.

## 6. Toggles, knowledge bases, skills

**Toggles** live in `<state>/toggles` as `pattern on|off` lines; the last matching line wins. Item names:
- **file:** its name without the `NN-` order prefix and extension, plus the full filename
- **rule:** `file:id`, `id`, `file`, and `@tag` for each tag
- **skill:** its folder name

**Knowledge bases** are listed in `knowledge-bases.ini` in each layer (`$LPLUG_KB_MAP` replaces the personal one).
- `[defaults]` and each base merge key by key, and the project wins.
- Keys: `description path kind(wiki|scripts) index rules edits(none|suggest|apply) preload(full|outline|none) preload_limit only_in aliases enabled`.
- If `index` / `rules` aren't set, the first existing `index.md`/`README.md`/`_index.md` and `AGENTS.md`/`CLAUDE.md`/`SCHEMA.md` are used.
- An `outline` preload keeps only the heading and link lines.
- `kind = scripts` lists each file's `Purpose:`, `Usage:` and `Tags:` header.

**Skills** are folders at `<layer>/skills.d/<name>/SKILL.md`.
- On `session_start`, and with `lplug skills sync`, enabled skills are symlinked into the agent's skills dir.
- Links that point into a `skills.d` are lplug's own. Those are the only ones lplug ever changes.

**Commands** are files at `<layer>/commands.d/<name>.md`, in the agent's slash-command format. A project command replaces a personal one with the same name.
- A line that is only `[indent][list marker] /name [args]`, where `name` is another lplug command, is replaced by that command's body (front matter dropped), nested under the line's list marker. This is recursive, and lines inside code fences are left alone.
- A call's arguments fill the called body's `$ARGUMENTS` and `$1`..`$9`. A call with no arguments leaves them unfilled, so they pass the top command's arguments through.
- A loop, or nesting deeper than 10, is an error. That command isn't built, and `status` and `commands list` report why. A `/name` that isn't an lplug command is left as written.
- On `session_start`, and with `lplug commands sync`, every enabled command that builds is written to the agent's commands dir with a marker line. Files with that marker are lplug's own: the only ones lplug ever rewrites or removes. Switching a command off removes its built file, but other commands can still call it.

## 7. Normalized event

```json
{"agent":"claude-code","event":"tool_before","native_event":"PreToolUse",
 "session_id":"…","tool_name":"Bash","tool_use_id":"…","subject":"git push",
 "cwd":"…","transcript_path":"…","reentrant":false,"raw":{…}}
```

## 8. Output and state

Results are merged as follows:
- any block blocks the event, and every reason is reported in order
- otherwise, any confirm asks the user
- context from every source is joined with blank lines

Claude Code output:
- exit 2 with the reasons on stderr, or
- `hookSpecificOutput` carrying `additionalContext` and/or `permissionDecision: "ask"`
- `PreCompact` context goes to plain stdout

State lives in `<state>/`:
- `toggles`
- `lplug.log`, which always records failures, and every decision when `debug` is on
- `sessions/<agent>-<session>/`, holding the counters; it's removed at `session_end` and pruned after 7 idle days

## 9. Adapters

An adapter supplies:
- a name
- a native→neutral event map
- payload parsing
- interpretation of its own native hook output
- which events can carry context
- output encoding
- its skills directory
- its commands directory

It must always drain stdin.
