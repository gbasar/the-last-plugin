package lplug

// The dispatcher: collect every source for one event, run it, merge.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const defaultAsk = "State in one line what you are doing and why."

type Kind int

const (
	kindSkip    Kind = iota
	kindExec         // executable: run it
	kindText         // .md / .txt: inject verbatim
	kindRules        // .ini .toml .json .conf .hocon: rule file
	kindSkill        // a skills.d entry (status only)
	kindCommand      // a commands.d entry (status only)
)

func (k Kind) String() string {
	return [...]string{"skip", "exec", "text", "rule", "skill", "command"}[k]
}

func classify(path string, fi os.FileInfo) Kind {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".ini", ".toml", ".json", ".conf", ".hocon":
		return kindRules
	case ".md", ".txt":
		return kindText
	}
	if isExecutable(path, fi) {
		return kindExec
	}
	return kindSkip
}

// Item is one switchable thing: a file source, a rule or a skill.
type Item struct {
	Path   string
	Kind   Kind
	Layer  *Layer
	Rule   *Rule
	Events []string // file sources: the events whose dirs/lists hold it
}

// Names are what toggle patterns match, most specific first:
// file "50-ponytail.md" -> ponytail, 50-ponytail.md;
// rule -> file:id, id, file, @tag...; skill -> its name.
func (it Item) Names() []string {
	file := baseName(it.Path)
	switch {
	case it.Rule != nil:
		names := []string{file + ":" + it.Rule.ID, it.Rule.ID, file}
		for _, t := range it.Rule.Tags {
			names = append(names, "@"+t)
		}
		return names
	case it.Kind == kindSkill:
		return []string{filepath.Base(it.Path)}
	case it.Kind == kindCommand:
		return []string{commandName(it)}
	}
	return []string{file, filepath.Base(it.Path)}
}

func (it Item) enabled(t *toggles) (on bool, by string) {
	if on, by := t.state(it.Names()); by != "" {
		return on, by
	}
	return it.Rule == nil || it.Rule.Enabled, ""
}

// baseName: "50-ponytail.md" -> "ponytail".
func baseName(p string) string {
	n := strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
	if t := strings.TrimLeft(n, "0123456789"); t != n && len(t) > 1 && (t[0] == '-' || t[0] == '_') {
		n = t[1:]
	}
	return n
}

// expand turns sources into trusted files: dirs become their files in name
// order (non-recursive; dotfiles and editor backups skipped), duplicates go.
func expand(srcs []Src, warns *[]string) []Src {
	seen := map[string]bool{}
	var out []Src
	add := func(p string, l *Layer) {
		if seen[p] {
			return
		}
		seen[p] = true
		fi, err := os.Stat(p)
		if err != nil || fi.IsDir() {
			return
		}
		if ok, why := trusted(fi); !ok {
			*warns = append(*warns, fmt.Sprintf("SKIP untrusted (%s): %s", why, p))
			return
		}
		out = append(out, Src{p, l})
	}
	for _, s := range srcs {
		if !isDir(s.Path) {
			add(s.Path, s.Layer)
			continue
		}
		entries, _ := os.ReadDir(s.Path) // sorted by name
		for _, e := range entries {
			if n := e.Name(); !strings.HasPrefix(n, ".") && !strings.HasSuffix(n, "~") && !strings.HasSuffix(n, ".swp") {
				add(filepath.Join(s.Path, n), s.Layer)
			}
		}
	}
	return out
}

// collect gathers the items for some events: file sources in order, then
// every rule, a later layer's rule replacing an earlier one with the same id.
func collect(c *Config, events []string) (items []Item, warns []string) {
	fileAt := map[string]int{}
	var ruleFiles []Src
	seenRules := map[string]bool{}
	addRules := func(s Src) {
		if !seenRules[s.Path] {
			seenRules[s.Path] = true
			ruleFiles = append(ruleFiles, s)
		}
	}
	for _, ev := range events {
		for _, s := range expand(c.EventSources(ev), &warns) {
			fi, _ := os.Stat(s.Path)
			switch kind := classify(s.Path, fi); {
			case kind == kindRules:
				addRules(s)
			case kind == kindSkip:
			case fileAt[s.Path] > 0:
				items[fileAt[s.Path]-1].Events = append(items[fileAt[s.Path]-1].Events, ev)
			default:
				items = append(items, Item{Path: s.Path, Kind: kind, Layer: s.Layer, Events: []string{ev}})
				fileAt[s.Path] = len(items)
			}
		}
	}
	for _, s := range expand(c.RuleSources(), &warns) {
		addRules(s)
	}
	ruleAt := map[string]int{}
	for _, s := range ruleFiles {
		rules, w, err := loadRules(s.Path)
		for _, m := range w {
			warns = append(warns, s.Path+": "+m)
		}
		if err != nil {
			warns = append(warns, fmt.Sprintf("FAIL rules %s: %v", s.Path, err))
			continue
		}
		for i := range rules {
			it := Item{Path: s.Path, Kind: kindRules, Layer: s.Layer, Rule: &rules[i]}
			if j, ok := ruleAt[rules[i].ID]; ok {
				items[j] = it
			} else {
				ruleAt[rules[i].ID] = len(items)
				items = append(items, it)
			}
		}
	}
	return items, warns
}

// ---- the hook run

type Dispatcher struct {
	A Adapter
	C *Config // nil: loaded for the event's working directory

	ev        Event
	toggles   *toggles
	eventFile string
	logf      *os.File
}

// Run handles one hook invocation and returns the exit code.
func (d *Dispatcher) Run(native string, stdin io.Reader, stdout, stderr io.Writer) int {
	defer d.close()
	raw, _ := io.ReadAll(stdin) // always drain stdin, so the agent never sees EPIPE
	ev, err := d.A.Parse(native, raw)
	d.ev = ev
	if d.C == nil {
		d.C = LoadConfig(ev.Cwd)
	}
	if err != nil {
		d.log("WARN %v", err)
	}
	if ev.Reentrant { // a stop a hook already continued: never loop
		return 0
	}
	switch ev.Name {
	case SessionStart:
		pruneSessions(d.C.StateDir, staleSessionAge)
		if dir := d.A.SkillsDir(); dir != "" {
			if _, _, _, err := syncSkills(d.C, dir); err != nil {
				d.log("WARN skills sync: %v", err)
			}
		}
		if dir := d.A.CommandsDir(); dir != "" {
			_, _, _, failed, err := syncCommands(d.C, dir)
			if err != nil {
				d.log("WARN commands sync: %v", err)
			}
			for _, f := range failed {
				d.log("WARN command not built: %s", f)
			}
		}
	case SessionEnd:
		defer os.RemoveAll(sessionDir(d.C.StateDir, ev.Agent, ev.SessionID))
	}

	items, warns := collect(d.C, []string{ev.Name})
	for _, w := range append(d.C.Warnings, warns...) {
		if strings.HasPrefix(w, "SKIP") || strings.HasPrefix(w, "FAIL") {
			d.log("%s", w)
		} else {
			d.debug("%s", w)
		}
	}
	if len(items) == 0 { // fast path: nothing configured for this event
		return 0
	}
	d.toggles = loadToggles(d.C.StateDir)

	var res Result
	for _, it := range items {
		if it.Rule != nil && !it.Rule.Matches(ev) {
			continue
		}
		if on, by := it.enabled(d.toggles); !on {
			d.debug("off (%s): %s", by, it.Names()[0])
			continue
		}
		switch it.Kind {
		case kindText:
			res.Merge(d.readText(it.Path))
		case kindExec:
			res.Merge(d.runProgram(it.Path, nil, ""))
		case kindRules:
			if d.gate(it.Rule) {
				res.Merge(d.fire(it))
			}
		}
	}
	if len(res.Context) > 0 && !res.Blocked && !d.A.CanInject(ev) {
		d.log("DROP context: %s cannot inject context on %s", ev.Agent, ev.Native)
	}
	return d.A.Emit(ev, res, stdout, stderr)
}

// fire applies a matched rule's actions in order: text, file, ask, run, confirm, block.
func (d *Dispatcher) fire(it Item) Result {
	r := it.Rule
	d.debug("fire: rule %s (%s)", r.ID, it.Path)
	var res Result
	res.AddContext(r.Text)
	if r.FilePath != "" {
		res.Merge(d.readText(d.C.ResolveText(r.FilePath, it.Layer)))
	}
	if r.Ask != "" {
		res.AddContext(r.Ask + " Begin that line with `INTENT:`.")
	}
	if r.Run != "" {
		args := strings.Fields(r.Run)
		if prog, err := d.resolveProgram(args[0], it.Layer); err != nil {
			d.log("FAIL rule %s: %v", r.ID, err)
		} else {
			res.Merge(d.runProgram(prog, args[1:], r.ID))
		}
	}
	res.Confirm(r.Confirm)
	if r.Block != "" {
		res.Block(r.Block)
	}
	return res
}

// resolveProgram: "lplug" is this binary; a bare name is looked up on PATH;
// anything else is a path relative to the rule's layer.
func (d *Dispatcher) resolveProgram(name string, l *Layer) (string, error) {
	switch {
	case name == "lplug":
		return os.Executable()
	case !strings.ContainsAny(name, `/\`) && !strings.HasPrefix(name, "~"):
		return exec.LookPath(name)
	}
	return l.path(name), nil
}

// gate applies every/once. Only events that already matched are counted.
func (d *Dispatcher) gate(r *Rule) bool {
	dir, key := sessionDir(d.C.StateDir, d.ev.Agent, d.ev.SessionID), r.stateKey()
	if r.Every > 1 {
		token := d.ev.Name + ":" + d.ev.ToolUseID
		if d.ev.ToolUseID == "" {
			token = fmt.Sprintf("%s:%d-%d", d.ev.Name, os.Getpid(), time.Now().UnixNano())
		}
		n, err := nth(dir, key, token)
		if err != nil {
			d.log("WARN counter %s: %v", r.ID, err)
			return false
		}
		if n%r.Every != 0 {
			d.debug("count %d/%d: rule %s", n, r.Every, r.ID)
			return false
		}
	}
	if r.Once {
		first, err := once(dir, key)
		if err != nil {
			d.log("WARN once %s: %v", r.ID, err)
		}
		return first
	}
	return true
}

func (d *Dispatcher) readText(p string) Result {
	var res Result
	fi, err := os.Stat(p)
	if err != nil {
		d.log("FAIL read %s: %v", p, err)
		return res
	}
	if ok, why := trusted(fi); !ok {
		d.log("SKIP untrusted (%s): %s", why, p)
		return res
	}
	b, _ := os.ReadFile(p)
	res.AddContext(string(b))
	return res
}

// runProgram protocol: stdin = the agent's native payload; exit 0 = stdout is
// context (or the agent's own hook JSON); exit 2 = block, stderr is the
// reason; anything else is logged and ignored.
func (d *Dispatcher) runProgram(p string, args []string, rule string) Result {
	var res Result
	self, _ := os.Executable()
	if fi, err := os.Stat(p); err != nil {
		d.log("FAIL %s: %v", p, err)
		return res
	} else if ok, why := trusted(fi); !ok && p != self {
		d.log("SKIP untrusted (%s): %s", why, p)
		return res
	}
	ctx, cancel := context.WithTimeout(context.Background(), d.C.Timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, p, args...)
	cmd.Stdin = bytes.NewReader(d.ev.Raw)
	cmd.Env = append(os.Environ(), d.scriptEnv(p, rule)...)
	if isDir(d.C.Cwd) {
		cmd.Dir = d.C.Cwd
	}
	killGroup(cmd)
	cmd.WaitDelay = time.Second // last resort if something still holds the pipes
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()

	var exitErr *exec.ExitError
	code := 0
	switch {
	case ctx.Err() == context.DeadlineExceeded:
		d.log("TIMEOUT after %s: %s", d.C.Timeout, p)
		return res
	case errors.As(err, &exitErr):
		code = exitErr.ExitCode()
	case err != nil:
		d.log("FAIL %s: %v", p, err)
		return res
	}
	switch code {
	case 0:
		if native, ok := d.A.NativeOutput(out.Bytes()); ok {
			return native
		}
		res.AddContext(out.String())
	case 2:
		reason := strings.TrimSpace(errb.String())
		if reason == "" {
			reason = "blocked by " + filepath.Base(p)
		}
		res.Block(reason)
	default:
		d.log("FAIL rc=%d: %s: %.500s", code, p, strings.TrimSpace(errb.String()))
	}
	return res
}

func (d *Dispatcher) scriptEnv(p, rule string) []string {
	env := []string{
		"LPLUG_AGENT=" + d.ev.Agent, "LPLUG_EVENT=" + d.ev.Name,
		"LPLUG_NATIVE_EVENT=" + d.ev.Native, "LPLUG_SESSION_ID=" + d.ev.SessionID,
		"LPLUG_TOOL_NAME=" + d.ev.ToolName, "LPLUG_SOURCE=" + p, "LPLUG_RULE=" + rule,
	}
	if d.eventFile == "" { // the normalized event, written once per run
		if f, err := os.CreateTemp("", "lplug-event-*.json"); err == nil {
			json.NewEncoder(f).Encode(d.ev)
			f.Close()
			d.eventFile = f.Name()
		}
	}
	return append(env, "LPLUG_EVENT_FILE="+d.eventFile)
}

func (d *Dispatcher) log(format string, a ...any) {
	if d.logf == nil {
		os.MkdirAll(d.C.StateDir, 0o700)
		f, err := os.OpenFile(filepath.Join(d.C.StateDir, "lplug.log"), os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o600)
		if err != nil {
			return
		}
		d.logf = f
	}
	fmt.Fprintf(d.logf, "%s [%s/%s] %s\n", time.Now().Format("2006-01-02 15:04:05"),
		d.ev.Agent, d.ev.Name, fmt.Sprintf(format, a...))
}

func (d *Dispatcher) debug(format string, a ...any) {
	if d.C.Debug {
		d.log(format, a...)
	}
}

func (d *Dispatcher) close() {
	if d.eventFile != "" {
		os.Remove(d.eventFile)
	}
	if d.logf != nil {
		d.logf.Close()
	}
}
