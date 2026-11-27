package lplug

// Commands: each layer's commands.d/<name>.md, a slash command in the agent's
// own format. A line whose whole content is a call to another lplug command,
// such as "1. /download-transactions $1", is replaced by that command's steps,
// recursively, so commands compose. lplug builds each enabled command into the
// agent's commands dir (Claude Code: ~/.claude/commands) on every session
// start and with `lplug commands sync`. Built files carry a marker line;
// lplug only ever rewrites or removes files that have it.

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	commandsDirName = "commands.d"
	builtMarker     = "<!-- lplug: built from "
	maxCallDepth    = 10
)

// callLine matches a line that is only a call: indent, optional list marker,
// /name, optional arguments.
var callLine = regexp.MustCompile(`^(\s*)((?:\d+[.)]|[-*+])\s+)?/([A-Za-z0-9][A-Za-z0-9_-]*)(?:\s+(.*?))?\s*$`)

// positional matches $1..$9 that aren't part of a longer name or number.
var positional = regexp.MustCompile(`\$([1-9])\b`)

type commandSource struct {
	Name, Path  string
	Layer       *Layer
	Front, Body string // front matter without its --- lines; the rest
}

// findCommands lists commands.d/*.md across layers; a project command
// replaces a personal one with the same name.
func findCommands(c *Config) []Item {
	at := map[string]int{}
	var out []Item
	for _, l := range c.Layers {
		files, _ := filepath.Glob(filepath.Join(l.Dir, commandsDirName, "*.md"))
		sort.Strings(files)
		for _, p := range files {
			name := strings.TrimSuffix(filepath.Base(p), ".md")
			it := Item{Path: p, Kind: kindCommand, Layer: l}
			if i, ok := at[name]; ok {
				out[i] = it
			} else {
				at[name] = len(out)
				out = append(out, it)
			}
		}
	}
	return out
}

func commandName(it Item) string { return strings.TrimSuffix(filepath.Base(it.Path), ".md") }

func loadCommandSources(c *Config) (map[string]*commandSource, error) {
	sources := map[string]*commandSource{}
	for _, it := range findCommands(c) {
		raw, err := os.ReadFile(it.Path)
		if err != nil {
			return nil, err
		}
		front, body := splitFrontMatter(string(raw))
		sources[commandName(it)] = &commandSource{Name: commandName(it), Path: it.Path, Layer: it.Layer, Front: front, Body: body}
	}
	return sources, nil
}

func splitFrontMatter(text string) (front, body string) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	if !strings.HasPrefix(text, "---\n") {
		return "", text
	}
	end := strings.Index(text[4:], "\n---")
	if end < 0 {
		return "", text
	}
	front = text[4 : 4+end]
	body = strings.TrimPrefix(text[4+end+4:], "\n")
	return front, body
}

// buildCommand returns the command as the agent should get it, and every
// lplug command it calls (directly or not), in first-call order.
func buildCommand(name string, sources map[string]*commandSource) (string, []string, error) {
	src := sources[name]
	if src == nil {
		return "", nil, fmt.Errorf("no command %q", name)
	}
	var calls []string
	body, err := expandSteps(src.Body, sources, []string{name}, &calls)
	if err != nil {
		return "", calls, err
	}
	var b strings.Builder
	if src.Front != "" {
		b.WriteString("---\n" + src.Front + "\n---\n")
	}
	fmt.Fprintf(&b, "%s%s. Edit that file, not this one. -->\n", builtMarker, src.Path)
	b.WriteString(body)
	return b.String(), calls, nil
}

// expandSteps replaces each call line in body with the called command's
// steps, nested under the line's list marker. stack is the call chain, for
// loop errors.
func expandSteps(body string, sources map[string]*commandSource, stack []string, calls *[]string) (string, error) {
	lines := strings.Split(body, "\n")
	inFence := false
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inFence = !inFence
		}
		m := callLine.FindStringSubmatch(line)
		if inFence || m == nil {
			continue
		}
		indent, marker, name, args := m[1], m[2], m[3], m[4]
		child := sources[name]
		if child == nil {
			continue // not ours: an agent built-in or another plugin's command
		}
		for _, s := range stack {
			if s == name {
				return "", fmt.Errorf("loop: /%s -> /%s", strings.Join(stack, " -> /"), name)
			}
		}
		if len(stack) >= maxCallDepth {
			return "", fmt.Errorf("calls nested deeper than %d: /%s", maxCallDepth, strings.Join(stack, " -> /"))
		}
		if !contains(*calls, name) {
			*calls = append(*calls, name)
		}
		childBody, err := expandSteps(withArguments(child.Body, args), sources, append(stack, name), calls)
		if err != nil {
			return "", err
		}
		heading := strings.TrimSpace("/" + name + " " + args)
		nested := indent + strings.Repeat(" ", len(marker))
		var b strings.Builder
		fmt.Fprintf(&b, "%s%s**%s**:", indent, marker, heading)
		for _, cl := range strings.Split(strings.TrimRight(strings.TrimLeft(childBody, "\n"), "\n"), "\n") {
			b.WriteString("\n")
			if cl != "" {
				b.WriteString(nested + cl)
			}
		}
		lines[i] = b.String()
	}
	return strings.Join(lines, "\n"), nil
}

// withArguments fills a called command's $ARGUMENTS and $1..$9 from the call's
// arguments. A call with no arguments leaves them as they are, so they take
// the arguments of the command the user ran: pass-through by default.
func withArguments(body, args string) string {
	if strings.TrimSpace(args) == "" {
		return body
	}
	fields := splitArguments(args)
	body = strings.ReplaceAll(body, "$ARGUMENTS", args)
	return positional.ReplaceAllStringFunc(body, func(p string) string {
		n := int(p[1] - '0')
		if n <= len(fields) {
			return fields[n-1]
		}
		return ""
	})
}

// splitArguments splits on spaces, keeping "quoted parts" together.
func splitArguments(s string) []string {
	var out []string
	var cur strings.Builder
	quote := rune(0)
	for _, r := range s {
		switch {
		case quote != 0 && r == quote:
			quote = 0
		case quote == 0 && (r == '"' || r == '\''):
			quote = r
		case quote == 0 && (r == ' ' || r == '\t'):
			if cur.Len() > 0 {
				out = append(out, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteRune(r)
		}
	}
	if cur.Len() > 0 {
		out = append(out, cur.String())
	}
	return out
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// commandReport is one command's build result, for sync, list and status.
type commandReport struct {
	Item  Item
	On    bool
	Calls []string
	Err   error
	Text  string
}

func buildAllCommands(c *Config) ([]commandReport, error) {
	sources, err := loadCommandSources(c)
	if err != nil {
		return nil, err
	}
	t := loadToggles(c.StateDir)
	var out []commandReport
	for _, it := range findCommands(c) {
		on, _ := it.enabled(t)
		text, calls, err := buildCommand(commandName(it), sources)
		out = append(out, commandReport{Item: it, On: on, Calls: calls, Err: err, Text: text})
	}
	return out, nil
}

// syncCommands makes target hold exactly lplug's enabled commands that build.
// skipped are names taken by files lplug didn't build; failed are commands
// that don't build (their old built file is removed, never left stale).
func syncCommands(c *Config, target string) (written, removed, skipped, failed []string, err error) {
	reports, err := buildAllCommands(c)
	if err != nil {
		return
	}
	want := map[string]string{}
	for _, r := range reports {
		switch {
		case !r.On:
		case r.Err != nil:
			failed = append(failed, fmt.Sprintf("%s: %v", commandName(r.Item), r.Err))
		default:
			want[commandName(r.Item)] = r.Text
		}
	}
	existing, _ := filepath.Glob(filepath.Join(target, "*.md"))
	if len(want) == 0 && len(existing) == 0 {
		return
	}
	if err = os.MkdirAll(target, 0o755); err != nil {
		return
	}
	for _, p := range existing { // drop our files that are stale, off or broken
		name := strings.TrimSuffix(filepath.Base(p), ".md")
		if _, ok := want[name]; !ok && builtByLplug(p) {
			if os.Remove(p) == nil {
				removed = append(removed, name)
			}
		}
	}
	names := make([]string, 0, len(want))
	for n := range want {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, name := range names {
		p := filepath.Join(target, name+".md")
		if old, readErr := os.ReadFile(p); readErr == nil {
			if string(old) == want[name] {
				continue
			}
			if !builtByLplug(p) {
				skipped = append(skipped, name)
				continue
			}
		}
		if err = os.WriteFile(p, []byte(want[name]), 0o644); err != nil {
			return
		}
		written = append(written, name)
	}
	return
}

func builtByLplug(p string) bool {
	raw, err := os.ReadFile(p)
	return err == nil && strings.Contains(string(raw), builtMarker)
}

// newCommandTemplate is what `lplug commands edit NAME` starts a new command with.
const newCommandTemplate = `---
description: %s
argument-hint: "[args]"
---
1. First step. A line that is only /other-command (with optional args) runs
   that lplug command's steps here.
2. Next step. $ARGUMENTS is everything after the command; $1, $2 are its words.
`
