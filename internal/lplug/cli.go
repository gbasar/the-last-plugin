package lplug

// The command line. Commands are a table: a new one (edit, check...) is one
// entry, and help lists it automatically.

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/tabwriter"
)

// Version is set at build time: -ldflags "-X lplug/internal/lplug.Version=...".
var Version = "dev"

type env struct {
	stdin          io.Reader
	stdout, stderr io.Writer
	cfg            *Config
}

type command struct {
	name, args, summary, help string
	run                       func(e *env, args []string) int
}

var commands []command

func init() {
	toggleCmd := func(mode string) func(*env, []string) int {
		return func(e *env, a []string) int { return runToggle(e, a, mode) }
	}
	commands = []command{
		{"hook", "[--agent NAME] EVENT", "Run everything configured for an event (the agent calls this)", helpHook, runHook},
		{"on", "NAME...", "Turn items on", helpToggles, toggleCmd("on")},
		{"off", "NAME...", "Turn items off", helpToggles, toggleCmd("off")},
		{"reset", "NAME...", "Forget a toggle; back to the item's default", helpToggles, toggleCmd("reset")},
		{"status", "", "Every configured item, where it lives, whether it's on and why", helpStatus, runStatus},
		{"kb", "[render|list]", "Knowledge bases: what the agent is told, or a table", "", runKB},
		{"skills", "[sync|list]", "Link skills.d skills into the agent's skills dir", "", runSkills},
		{"commands", "[sync|list|show NAME|edit NAME]", "Build commands.d slash commands (they can call each other)", "", runCommands},
		{"help", "[COMMAND|TOPIC]", "Show help", "", runHelp},
		{"version", "", "Print the version", "", func(e *env, _ []string) int { fmt.Fprintln(e.stdout, "lplug", Version); return 0 }},
	}
}

// Main is the entry point; it returns the exit code.
func Main(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	e := &env{stdin: stdin, stdout: stdout, stderr: stderr}
	if len(args) == 0 {
		usage(e)
		return 0
	}
	name, rest := args[0], args[1:]
	switch name {
	case "-h", "--help":
		name = "help"
	case "-v", "--version":
		name = "version"
	}
	c := findCommand(name)
	if c == nil {
		fmt.Fprintf(stderr, "lplug: unknown command %q - run `lplug help` for the list\n", name)
		return 1
	}
	if len(rest) > 0 && (rest[0] == "-h" || rest[0] == "--help") {
		return runHelp(e, []string{name})
	}
	return c.run(e, rest)
}

func findCommand(name string) *command {
	for i := range commands {
		if commands[i].name == name {
			return &commands[i]
		}
	}
	return nil
}

func (e *env) config() *Config {
	if e.cfg == nil {
		e.cfg = LoadConfig("")
	}
	return e.cfg
}

func agentFlag(e *env, name string, args []string) (Adapter, []string, bool) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(e.stderr)
	def := os.Getenv("LPLUG_AGENT")
	if def == "" {
		def = defaultAgent
	}
	agent := fs.String("agent", def, "agent adapter: "+strings.Join(agentNames(), ", "))
	if fs.Parse(args) != nil {
		return nil, nil, false
	}
	a, ok := adapters[*agent]
	if !ok {
		fmt.Fprintf(e.stderr, "lplug: unknown agent %q (known: %s)\n", *agent, strings.Join(agentNames(), ", "))
	}
	return a, fs.Args(), ok
}

func runHook(e *env, args []string) int {
	a, rest, ok := agentFlag(e, "hook", args)
	if !ok {
		return 1
	}
	if len(rest) != 1 {
		fmt.Fprintln(e.stderr, "usage: lplug hook [--agent NAME] EVENT   (see `lplug help hook`)")
		return 1
	}
	return (&Dispatcher{A: a}).Run(rest[0], e.stdin, e.stdout, e.stderr)
}

func runToggle(e *env, args []string, mode string) int {
	if len(args) == 0 {
		fmt.Fprintf(e.stderr, "usage: lplug %s NAME...   (see `lplug help toggles`)\n", mode)
		return 1
	}
	c := e.config()
	t := loadToggles(c.StateDir)
	items, _ := collect(c, AllEvents)
	items = append(items, findSkills(c)...)
	items = append(items, findCommands(c)...)
	for _, p := range args {
		if err := validPattern(p); err != nil {
			fmt.Fprintln(e.stderr, "lplug:", err)
			return 1
		}
		if mode == "reset" {
			if !t.reset(p) {
				fmt.Fprintf(e.stderr, "lplug: %s had no toggle\n", p)
			}
			continue
		}
		t.set(p, mode == "on")
		probe := &toggles{Entries: []toggle{{p, true}}}
		n := 0
		for _, it := range items {
			if _, by := probe.state(it.Names()); by != "" {
				n++
			}
		}
		if n == 0 {
			fmt.Fprintf(e.stderr, "lplug: note: %q matches nothing configured right now (saved anyway)\n", p)
		} else {
			fmt.Fprintf(e.stdout, "%s %s (%d item%s)\n", p, mode, n, map[bool]string{true: "", false: "s"}[n == 1])
		}
	}
	if err := t.save(); err != nil {
		fmt.Fprintln(e.stderr, "lplug:", err)
		return 1
	}
	return 0
}

func runStatus(e *env, _ []string) int {
	c := e.config()
	t := loadToggles(c.StateDir)
	items, warns := collect(c, AllEvents)
	items = append(items, findSkills(c)...)
	items = append(items, findCommands(c)...)

	fmt.Fprintln(e.stdout, "Layers:")
	for _, l := range c.Layers {
		note := ""
		if l.Name == "project" {
			note = "   (shared with anyone who gets this repo, if committed)"
		}
		fmt.Fprintf(e.stdout, "  %-8s  %s%s\n", l.Label(), l.Dir, note)
	}
	fmt.Fprintf(e.stdout, "  %-8s  %s\n\n", "state", c.StateDir)
	if len(items) == 0 {
		fmt.Fprintf(e.stdout, "Nothing configured. Try:\n  mkdir -p %[1]s/session-start.d\n  echo 'Code very efficiently.' > %[1]s/session-start.d/50-ponytail.md\n\n", c.Layers[0].Dir)
	}
	w := tabwriter.NewWriter(e.stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "STATE\tNAME\tKIND\tWHERE\tEVENTS\tDETAIL")
	for _, it := range items {
		on, by := it.enabled(t)
		state := map[bool]string{true: "on", false: "off"}[on]
		if by != "" {
			state += " (" + by + ")"
		}
		name, events, detail := it.Names()[0], strings.Join(it.Events, ","), ""
		if r := it.Rule; r != nil {
			name, events, detail = r.ID, strings.Join(r.Events, ","), ruleDetail(r)
		}
		if it.Kind == kindCommand {
			detail = commandDetail(c, it)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", state, name, it.Kind, it.Layer.Label(), events, detail)
	}
	w.Flush()
	if all := append(c.Warnings, warns...); len(all) > 0 {
		fmt.Fprintln(e.stdout, "\nWarnings:")
		for _, m := range all {
			fmt.Fprintln(e.stdout, "  "+m)
		}
	}
	if len(t.Entries) > 0 {
		fmt.Fprintln(e.stdout, "\nToggles (last match wins):")
		for _, en := range t.Entries {
			fmt.Fprintf(e.stdout, "  %-3s %s\n", map[bool]string{true: "on", false: "off"}[en.On], en.Pattern)
		}
	}
	return 0
}

func ruleDetail(r *Rule) string {
	var parts []string
	if r.Every > 1 {
		parts = append(parts, fmt.Sprintf("every %d", r.Every))
	}
	if r.Once {
		parts = append(parts, "once")
	}
	for _, t := range r.Tags {
		parts = append(parts, "@"+t)
	}
	if r.Description != "" {
		parts = append(parts, r.Description)
	}
	return strings.Join(parts, " ")
}

func runKB(e *env, args []string) int {
	c := e.config()
	if len(args) == 0 || args[0] == "render" {
		fmt.Fprint(e.stdout, renderKBs(c))
		return 0
	}
	if args[0] != "list" {
		fmt.Fprintln(e.stderr, "usage: lplug kb [render|list]   (see `lplug help kb`)")
		return 1
	}
	kbs, warns := loadKBs(c)
	w := tabwriter.NewWriter(e.stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tSHOWN HERE\tKIND\tEDITS\tPRELOAD\tWHERE\tPATH")
	for _, kb := range kbs {
		shown := map[bool]string{true: "yes", false: "no"}[kb.Enabled && kb.applies(c.Cwd)]
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n", kb.Name, shown, kb.Kind, kb.Edits, kb.Preload, kb.Layer.Label(), kb.Path)
	}
	w.Flush()
	for _, m := range warns {
		fmt.Fprintln(e.stdout, "warning:", m)
	}
	return 0
}

func runSkills(e *env, args []string) int {
	a, rest, ok := agentFlag(e, "skills", args)
	if !ok {
		return 1
	}
	c := e.config()
	if len(rest) > 0 && rest[0] == "list" {
		t := loadToggles(c.StateDir)
		for _, it := range findSkills(c) {
			on, _ := it.enabled(t)
			fmt.Fprintf(e.stdout, "%-3s  %-8s  %s\n", map[bool]string{true: "on", false: "off"}[on], it.Layer.Label(), it.Path)
		}
		return 0
	}
	target := a.SkillsDir()
	linked, removed, skipped, err := syncSkills(c, target)
	for _, n := range linked {
		fmt.Fprintf(e.stdout, "linked   %s\n", n)
	}
	for _, n := range removed {
		fmt.Fprintf(e.stdout, "unlinked %s\n", n)
	}
	for _, n := range skipped {
		fmt.Fprintf(e.stdout, "skipped  %s (already in %s, not managed by lplug)\n", n, target)
	}
	if err != nil {
		fmt.Fprintln(e.stderr, "lplug:", err)
		return 1
	}
	if len(linked)+len(removed)+len(skipped) == 0 {
		fmt.Fprintln(e.stdout, "skills up to date in", target)
	}
	return 0
}

func runCommands(e *env, args []string) int {
	a, rest, ok := agentFlag(e, "commands", args)
	if !ok {
		return 1
	}
	c := e.config()
	sub := "sync"
	if len(rest) > 0 {
		sub, rest = rest[0], rest[1:]
	}
	switch sub {
	case "list":
		reports, err := buildAllCommands(c)
		if err != nil {
			fmt.Fprintln(e.stderr, "lplug:", err)
			return 1
		}
		if len(reports) == 0 {
			fmt.Fprintf(e.stdout, "No commands yet. Start one with: lplug commands edit NAME\n(they live in %s)\n", filepath.Join(c.Layers[0].Dir, commandsDirName))
			return 0
		}
		w := tabwriter.NewWriter(e.stdout, 0, 2, 2, ' ', 0)
		fmt.Fprintln(w, "STATE\tCOMMAND\tWHERE\tDETAIL\tSOURCE")
		for _, r := range reports {
			fmt.Fprintf(w, "%s\t/%s\t%s\t%s\t%s\n", map[bool]string{true: "on", false: "off"}[r.On],
				commandName(r.Item), r.Item.Layer.Label(), commandDetail(c, r.Item), r.Item.Path)
		}
		w.Flush()
		return 0
	case "show":
		if len(rest) != 1 {
			fmt.Fprintln(e.stderr, "usage: lplug commands show NAME")
			return 1
		}
		sources, err := loadCommandSources(c)
		if err != nil {
			fmt.Fprintln(e.stderr, "lplug:", err)
			return 1
		}
		text, _, err := buildCommand(strings.TrimPrefix(rest[0], "/"), sources)
		if err != nil {
			fmt.Fprintln(e.stderr, "lplug:", err)
			return 1
		}
		fmt.Fprint(e.stdout, text)
		return 0
	case "edit":
		if len(rest) != 1 {
			fmt.Fprintln(e.stderr, "usage: lplug commands edit NAME")
			return 1
		}
		return editCommand(e, c, a, strings.TrimPrefix(rest[0], "/"))
	case "sync":
		return syncCommandsReport(e, c, a)
	}
	fmt.Fprintln(e.stderr, "usage: lplug commands [sync|list|show NAME|edit NAME]   (see `lplug help commands`)")
	return 1
}

// commandDetail: what a command calls, or why it doesn't build.
func commandDetail(c *Config, it Item) string {
	sources, err := loadCommandSources(c)
	if err != nil {
		return "error: " + err.Error()
	}
	_, calls, err := buildCommand(commandName(it), sources)
	switch {
	case err != nil:
		return "NOT BUILT: " + err.Error()
	case len(calls) > 0:
		return "calls /" + strings.Join(calls, ", /")
	}
	return ""
}

func syncCommandsReport(e *env, c *Config, a Adapter) int {
	target := a.CommandsDir()
	if target == "" {
		fmt.Fprintf(e.stderr, "lplug: %s has no commands dir\n", a.Name())
		return 1
	}
	written, removed, skipped, failed, err := syncCommands(c, target)
	for _, n := range written {
		fmt.Fprintf(e.stdout, "built    /%s\n", n)
	}
	for _, n := range removed {
		fmt.Fprintf(e.stdout, "removed  /%s\n", n)
	}
	for _, n := range skipped {
		fmt.Fprintf(e.stdout, "skipped  /%s (already in %s, not built by lplug)\n", n, target)
	}
	for _, f := range failed {
		fmt.Fprintf(e.stdout, "FAILED   /%s\n", f)
	}
	if err != nil {
		fmt.Fprintln(e.stderr, "lplug:", err)
		return 1
	}
	if len(written)+len(removed)+len(skipped)+len(failed) == 0 {
		fmt.Fprintln(e.stdout, "commands up to date in", target)
	}
	if len(failed) > 0 {
		return 1
	}
	return 0
}

// editCommand opens a command's source in $VISUAL/$EDITOR (a new one starts
// from a template in the personal layer), then rebuilds.
func editCommand(e *env, c *Config, a Adapter, name string) int {
	path := ""
	for _, it := range findCommands(c) {
		if commandName(it) == name {
			path = it.Path
		}
	}
	if path == "" {
		path = filepath.Join(c.Layers[0].Dir, commandsDirName, name+".md")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			fmt.Fprintln(e.stderr, "lplug:", err)
			return 1
		}
		if err := os.WriteFile(path, []byte(fmt.Sprintf(newCommandTemplate, "What /"+name+" does")), 0o644); err != nil {
			fmt.Fprintln(e.stderr, "lplug:", err)
			return 1
		}
		fmt.Fprintln(e.stdout, "new command:", path)
	}
	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "vi"
	}
	cmd := exec.Command("sh", "-c", editor+` "$1"`, "sh", path) // $EDITOR may hold flags
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, e.stdout, e.stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(e.stderr, "lplug: editor:", err)
		return 1
	}
	return syncCommandsReport(e, c, a)
}
