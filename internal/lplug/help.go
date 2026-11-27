package lplug

import (
	"fmt"
	"strings"
	"text/tabwriter"
)

// Help topics. Funcs, so they can show live values (paths, agents, keys).
var topics = []struct {
	name, summary string
	text          func(c *Config) string
}{
	{"sources", "Where lplug looks, and the file types", helpSources},
	{"rules", "Rule files: formats and every setting", helpRules},
	{"scripts", "Writing hook programs in any language", func(*Config) string { return helpScripts }},
	{"events", "Neutral event names and each agent's native names", helpEvents},
	{"toggles", "Turning things on and off", func(*Config) string { return helpToggles }},
	{"settings", "Layers, settings.ini, env vars - with current values", helpSettings},
	{"kb", "Knowledge bases", helpKB},
	{"skills", "Skills from skills.d", func(*Config) string { return helpSkills }},
	{"commands", "Slash commands from commands.d, composable", func(*Config) string { return helpCommands }},
}

func usage(e *env) {
	c := e.config()
	fmt.Fprintf(e.stdout, "lplug %s - one hook dispatcher for every agent event\n\nUsage: lplug COMMAND [ARGS]\n\nCommands:\n", Version)
	w := tabwriter.NewWriter(e.stdout, 0, 2, 2, ' ', 0)
	for _, cmd := range commands {
		fmt.Fprintf(w, "  %s %s\t%s\n", cmd.name, cmd.args, cmd.summary)
	}
	w.Flush()
	fmt.Fprintln(e.stdout, "\nHelp topics (lplug help TOPIC):")
	w = tabwriter.NewWriter(e.stdout, 0, 2, 2, ' ', 0)
	for _, t := range topics {
		fmt.Fprintf(w, "  %s\t%s\n", t.name, t.summary)
	}
	w.Flush()
	fmt.Fprintf(e.stdout, `
Quick start:
  mkdir -p %[1]s/session-start.d
  echo 'Code very efficiently.' > %[1]s/session-start.d/50-ponytail.md
  lplug status
  lplug off ponytail          # in Claude Code, without a model turn: ! lplug off ponytail

Personal layer: %[1]s
State:          %[2]s
`, c.Layers[0].Dir, c.StateDir)
}

func runHelp(e *env, args []string) int {
	if len(args) == 0 {
		usage(e)
		return 0
	}
	cmd := findCommand(args[0])
	if cmd != nil {
		fmt.Fprintf(e.stdout, "Usage: lplug %s %s\n\n%s\n\n", cmd.name, cmd.args, cmd.summary)
		if cmd.help != "" {
			fmt.Fprintln(e.stdout, strings.TrimSpace(cmd.help))
			return 0
		}
	}
	// A command without long help (kb, skills) shows its topic, if it has one.
	for _, t := range topics {
		if t.name == args[0] || (args[0] == "env" && t.name == "settings") {
			fmt.Fprintln(e.stdout, strings.TrimSpace(t.text(e.config())))
			return 0
		}
	}
	if cmd != nil {
		return 0
	}
	fmt.Fprintf(e.stderr, "lplug: no command or topic %q - run `lplug help` for the list\n", args[0])
	return 1
}

const helpHook = `
Called by the agent once per event. Reads the agent's native payload on
stdin, runs every source configured for the event (lplug help sources),
merges the results and answers in the agent's own format.

  --agent NAME   adapter (default $LPLUG_AGENT, else claude-code)
  EVENT          the agent's native event name, e.g. PreToolUse

Merging: any block blocks the event (every reason is reported); any confirm
asks the user; context from every source is joined in order.

Try it by hand:
  echo '{"session_id":"t","tool_name":"Bash","tool_input":{"command":"ls"}}' \
    | LPLUG_DEBUG=1 lplug hook PreToolUse; echo "exit $?"
`

const helpToggles = `
Every file source, rule and skill can be switched without editing config.
Switches live in <state>/toggles, so your config files are never rewritten.

  lplug off ponytail        a rule id, file name (50-ponytail.md -> ponytail) or skill
  lplug off work            every rule in work.ini
  lplug off work:no-main    one rule in one file, when ids clash
  lplug off @strict         every rule with tags = strict
  lplug off 'git-*'         globs (* ? [..]) - quote them for the shell
  lplug on  ponytail        the most recent matching switch wins
  lplug reset ponytail      forget the switch; back to the default

In Claude Code, without spending a turn:  ! lplug off ponytail
`

const helpStatus = `
Lists every file source, rule and skill in every layer: whether it's on,
which toggle decided that, and WHERE it lives - personal (your layer) or
repo (a project layer, shared with anyone who gets the repo if committed).
Problems (parse errors, unknown keys, untrusted files) are listed at the end.
`

func helpSources(c *Config) string {
	var b strings.Builder
	b.WriteString(`
For each event, lplug reads from every layer (personal, then project):

  1. the event's sources: $LPLUG_<EVENT> (replaces all layers), else each
     layer's [sources] setting, else <layer>/<event>.d/
  2. rule sources: $LPLUG_RULES, else each layer's rules setting, else
     <layer>/rules.d/

Lists use ':' (';' on Windows). Directories are read in filename order, not
recursively; name files 10-x, 50-y to order them. Dotfiles and editor
backups are ignored. File sources from both layers run; a project rule
replaces a personal rule with the same id.

File types:
  executable                 run it, any language      -> lplug help scripts
  .md .txt                   injected as-is
  .ini .toml .json .conf     rule file                 -> lplug help rules

A file is used only if you (or root) own it and nobody else can write it.

Your layers now:
`)
	for _, l := range c.Layers {
		fmt.Fprintf(&b, "  %-8s %s\n", l.Label(), l.Dir)
	}
	return b.String()
}

func helpRules(*Config) string {
	var b strings.Builder
	b.WriteString(`
A rule file holds any number of rules; every format has the same shape:
top-level keys are rule ids, each with settings. Rules are consulted on
every event; each rule's "event" decides where it fires.

Settings:
`)
	w := tabwriter.NewWriter(&b, 0, 2, 2, ' ', 0)
	for _, k := range ruleKeys {
		fmt.Fprintf(w, "  %s\t%s\n", k.Name, k.Help)
	}
	w.Flush()
	b.WriteString(`
Actions apply in the order text, file, ask, run, confirm, block.
"every" counts only events that matched the rule's filters, per session.
confirm only means "ask the user" on tool_before; elsewhere it blocks.

INI (work.ini) - quote a value to keep " ;" or " #" in it. A value that
starts with a quote is a quoted string, so wrap a regex that starts with "
in single quotes:  unless = '"model"\s*:'
  [tests]
  event = tool_after
  tool  = Bash
  every = 5
  text  = Run the tests before saying you're done.

  [no-main]
  event  = tool_before
  when   = git push.*\bmain\b
  unless = --dry-run
  block  = Don't push to main; open a PR.
  tags   = strict

TOML (work.toml), flat tables only:  [tests]  event = "tool_after" ...
JSON (work.json):   { "tests": { "event": "tool_after", "every": 5, "text": "..." } }
HOCON (work.conf):  tests { event = tool_after, every = 5, text = "..." }

Check your files with: lplug status
`)
	return b.String()
}

const helpScripts = `
Any executable works, in any language:

  stdin    the agent's native hook payload (existing hooks work as-is)
  exit 0   stdout is injected as context; the agent's own hook JSON is understood
  exit 2   block the event; stderr is the reason
  other    logged and ignored - one broken script never stops the rest

Environment: LPLUG_AGENT, LPLUG_EVENT (neutral), LPLUG_NATIVE_EVENT,
LPLUG_SESSION_ID, LPLUG_TOOL_NAME, LPLUG_SOURCE, LPLUG_RULE, and
LPLUG_EVENT_FILE: the normalized event as JSON, the same for every agent.
Programs run in the session's working directory, with a timeout
(lplug help settings); the whole process group is killed on timeout.

  #!/bin/sh
  case "$LPLUG_TOOL_NAME" in Bash) ;; *) exit 0 ;; esac
  grep -q 'rm -rf /' && { echo "refusing rm -rf /" >&2; exit 2; }
  exit 0
`

func helpEvents(*Config) string {
	var b strings.Builder
	b.WriteString(`
Rules, env vars and directories use neutral names; rules also accept native
ones (case, '-' and '_' don't matter: tool_before = tool-before = PreToolUse).
`)
	for _, name := range agentNames() {
		a := adapters[name]
		native := map[string]string{}
		for nat, neu := range a.Events() {
			native[neu] = nat
		}
		fmt.Fprintf(&b, "\nAgent %s:\n", name)
		w := tabwriter.NewWriter(&b, 0, 2, 2, ' ', 0)
		fmt.Fprintln(w, "  NEUTRAL\tNATIVE\tENV VAR\tDIRECTORY\tCAN INJECT")
		for _, ev := range AllEvents {
			nat, inj := native[ev], "-"
			if nat == "" {
				nat = "-"
			} else if a.CanInject(Event{Name: ev, Native: nat}) {
				inj = "yes"
			}
			fmt.Fprintf(w, "  %s\t%s\t%s\t%s\t%s\n", ev, nat, eventEnv(ev), eventDir(ev), inj)
		}
		w.Flush()
	}
	b.WriteString(`
Other native events work under their snake_case name (PermissionRequest ->
permission_request). On stop events, "when" matches the agent's last message,
and a stop that a hook already continued is skipped, so rules can't loop.
`)
	return b.String()
}

func helpSettings(c *Config) string {
	var b strings.Builder
	b.WriteString(`
Precedence: environment variable > project settings.ini > personal settings.ini > default.

Layers:
  personal  $LPLUG_CONFIG_DIR, else $XDG_CONFIG_HOME/lplug (~/.config/lplug)
  project   the nearest <dir>/.config/lplug above where you work (not your home's)
Each may hold settings.ini; relative paths in a layer's files are relative to
its root (personal: the config dir; project: the project root).

  [lplug]
  state_dir = ~/work/lplug-state
  text_path = text:~/team/text
  rules     = rules.d:~/team/rules.d
  timeout   = 5s
  debug     = false

  [sources]
  session_start = session-start.d:~/team/session-start.d

Current values:
`)
	values := map[string]string{
		"config_dir": c.Layers[0].Dir, "state_dir": c.StateDir,
		"text_path": strings.Join(c.TextPath, ":"), "timeout": c.Timeout.String(),
		"debug": fmt.Sprint(c.Debug), "rules": srcPaths(c.RuleSources()),
	}
	w := tabwriter.NewWriter(&b, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "  SETTING\tENV VAR\tVALUE\tFROM")
	for _, k := range settingKeys {
		fmt.Fprintf(w, "  %s\t%s\t%s\t%s\n", k.Key, k.Env, values[k.Key], c.Origin[k.Key])
	}
	w.Flush()
	b.WriteString(`
Also: LPLUG_<EVENT> (sources for one event), LPLUG_KB_MAP (your knowledge-base
map), LPLUG_AGENT (default adapter). In Claude Code, set env vars for every
session in the "env" block of ~/.claude/settings.json.
`)
	return b.String()
}

func srcPaths(srcs []Src) string {
	var ps []string
	for _, s := range srcs {
		ps = append(ps, s.Path)
	}
	return strings.Join(ps, ":")
}

func helpKB(*Config) string {
	var b strings.Builder
	b.WriteString(`
Knowledge bases are listed in knowledge-bases.ini in each layer (yours can
move with $LPLUG_KB_MAP). [defaults] and every base merge key by key, the
project winning, so a project can add bases, change flags, or hide yours.

  [defaults]
  edits = suggest

  [project-blackbird]
  description = Architecture, decisions, glossary
  path    = wiki
  only_in = ~/work/blackbird

  [scripts]
  description = Reusable helper scripts
  path  = ~/.local/share/scripts
  kind  = scripts
  edits = apply

Keys:
`)
	w := tabwriter.NewWriter(&b, 0, 2, 2, ' ', 0)
	for _, k := range kbKeys {
		fmt.Fprintf(w, "  %s\t%s\n", k.Name, k.Help)
	}
	w.Flush()
	b.WriteString(`
To inject them, add a rule:  [kb]  event = session_start, subagent_start   run = lplug kb render
lplug kb list   shows every base, and whether it's shown in this folder.
`)
	return b.String()
}

const helpSkills = `
Put a skill folder (with its SKILL.md) in <layer>/skills.d/. On every
session start, and with 'lplug skills sync', lplug links each enabled skill
into the agent's skills dir (Claude Code: ~/.claude/skills).

- A project skill replaces a personal one with the same name.
- 'lplug off NAME' unlinks a skill at the next sync; 'lplug on NAME' brings it back.
- lplug only touches links pointing into a skills.d - never real folders.

  lplug skills list    every skill, on/off, personal or repo
  lplug skills sync    link and unlink now
`

const helpCommands = `
Put a slash command in <layer>/commands.d/NAME.md, in Claude Code's own
command format (front matter, then the prompt). A line that is only a call
to another lplug command runs that command's steps right there:

  ~/.config/lplug/commands.d/replay-transactions.md
    ---
    description: Replay a month of transactions
    argument-hint: "[month]"
    ---
    1. /download-transactions $1
    2. Replay the downloaded file against staging.
    3. /reconcile-transactions

On every session start, and with 'lplug commands sync', lplug builds each
enabled command, with every call filled in (recursively), into the agent's
commands dir (Claude Code: ~/.claude/commands). Edit the source in
commands.d, never the built file.

- Arguments: a call's arguments fill the called command's $ARGUMENTS and
  $1..$9. A call with none passes the top command's arguments through.
- A project command replaces a personal one with the same name.
- A loop (/a calls /b calls /a) or nesting deeper than 10 is an error: the
  command isn't built, and 'lplug status' and 'commands list' say why.
- /name that isn't an lplug command (a built-in, another plugin's) is left
  as written, for the agent.
- 'lplug off NAME' removes the built command at the next sync; other
  commands can still call it.
- Calls inside code fences are left alone.
- lplug only rewrites or removes files it built (they carry a marker line).

  lplug commands list         every command, on/off, what it calls, errors
  lplug commands show NAME    the built command, exactly as the agent gets it
  lplug commands edit NAME    open the source in $EDITOR (new: from a template), then sync
  lplug commands sync         build now
`
