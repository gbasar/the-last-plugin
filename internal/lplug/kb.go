package lplug

// Knowledge bases: knowledge-bases.ini in each layer (the user layer's can be
// moved with $LPLUG_KB_MAP). [defaults] and each base merge key by key, the
// project winning. `lplug kb render` prints what the agent should know.

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const kbMapFile = "knowledge-bases.ini"

var kbKeys = []struct{ Name, Help string }{
	{"description", "what the base is for (shown to the agent)"},
	{"path", "the base's folder (relative: to the layer's root)"},
	{"kind", "wiki (default) or scripts: an index built from script headers"},
	{"index", "entry page, relative to path (default: index.md, README.md or _index.md)"},
	{"rules", "how to read and write the base (default: AGENTS.md, CLAUDE.md or SCHEMA.md)"},
	{"edits", "none | suggest (default) | apply"},
	{"preload", "what to inject at session start: full | outline (default) | none"},
	{"preload_limit", "characters per base (default 4000)"},
	{"only_in", "list only when working under these folders"},
	{"aliases", "other names for the base"},
	{"enabled", "false hides the base (e.g. a personal base inside a project)"},
}

var kbDefaults = map[string]string{
	"kind": "wiki", "edits": "suggest", "preload": "outline", "preload_limit": "4000", "enabled": "true",
}

var kbChoices = map[string][]string{
	"kind": {"wiki", "scripts"}, "edits": {"none", "suggest", "apply"}, "preload": {"full", "outline", "none"},
}

type KB struct {
	Name, Description, Path, Kind, Index, Rules, Edits, Preload string
	OnlyIn, Aliases                                             []string
	Limit                                                       int
	Enabled                                                     bool
	Layer                                                       *Layer // where `path` was set
}

type kbValue struct {
	v string
	l *Layer
}

func loadKBs(c *Config) ([]*KB, []string) {
	var warns []string
	known := map[string]bool{}
	for _, k := range kbKeys {
		known[k.Name] = true
	}
	defaults := map[string]kbValue{}
	for k, v := range kbDefaults {
		defaults[k] = kbValue{v, c.Layers[0]}
	}
	bases := map[string]map[string]kbValue{}
	var order []string
	for i, l := range c.Layers {
		file := filepath.Join(l.Dir, kbMapFile)
		if v := os.Getenv("LPLUG_KB_MAP"); v != "" && i == 0 {
			file = absPath(v)
		}
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}
		doc, err := parseINI(data)
		if err != nil {
			warns = append(warns, fmt.Sprintf("%s: %v", file, err))
			continue
		}
		names := make([]string, 0, len(doc))
		for n := range doc {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, name := range names {
			target := defaults
			if name != "defaults" {
				if bases[name] == nil {
					bases[name] = map[string]kbValue{}
					order = append(order, name)
				}
				target = bases[name]
			}
			for k, v := range doc[name] {
				if !known[k] {
					warns = append(warns, fmt.Sprintf("%s: [%s] unknown key %q", file, name, k))
				}
				target[k] = kbValue{v, l}
			}
		}
	}

	var kbs []*KB
	for _, name := range order {
		get := func(k string) kbValue {
			if v, ok := bases[name][k]; ok {
				return v
			}
			return defaults[k]
		}
		kb := &KB{Name: name, Description: get("description").v, Aliases: toList(get("aliases").v)}
		for k, dst := range map[string]*string{"kind": &kb.Kind, "edits": &kb.Edits, "preload": &kb.Preload} {
			*dst = get(k).v
			if !anyOf(kbChoices[k], func(c string) bool { return c == *dst }) {
				warns = append(warns, fmt.Sprintf("base %q: %s %q is not one of %s", name, k, *dst, strings.Join(kbChoices[k], ", ")))
				*dst = kbDefaults[k]
			}
		}
		kb.Enabled, _ = toBool(get("enabled").v, true)
		kb.Limit, _ = strconv.Atoi(get("preload_limit").v)
		if kb.Limit <= 0 {
			kb.Limit = 4000
		}
		p := get("path")
		if p.v == "" {
			warns = append(warns, fmt.Sprintf("base %q has no path", name))
			continue
		}
		kb.Path, kb.Layer = p.l.path(p.v), p.l
		if v := get("only_in"); v.v != "" {
			kb.OnlyIn = v.l.list(v.v)
		}
		kb.Index = kb.findFile(get("index").v, "index.md", "README.md", "_index.md")
		kb.Rules = kb.findFile(get("rules").v, "AGENTS.md", "CLAUDE.md", "SCHEMA.md")
		kbs = append(kbs, kb)
	}
	return kbs, warns
}

// findFile: the given name relative to the base, else the first default that exists.
func (kb *KB) findFile(given string, defaults ...string) string {
	if given != "" {
		return filepath.Join(kb.Path, given)
	}
	for _, d := range defaults {
		if p := filepath.Join(kb.Path, d); exists(p) {
			return p
		}
	}
	return ""
}

func (kb *KB) applies(cwd string) bool {
	return len(kb.OnlyIn) == 0 || anyOf(kb.OnlyIn, func(dir string) bool {
		return cwd == dir || strings.HasPrefix(cwd, dir+string(filepath.Separator))
	})
}

// renderKBs is what gets injected: every enabled base that applies here.
func renderKBs(c *Config) string {
	kbs, _ := loadKBs(c)
	var b strings.Builder
	for _, kb := range kbs {
		if !kb.Enabled || !kb.applies(c.Cwd) {
			continue
		}
		if b.Len() == 0 {
			b.WriteString("# Knowledge bases\n")
		}
		where := ""
		if kb.Layer.Name == "project" {
			where = " (this project)"
		}
		fmt.Fprintf(&b, "\n- %s%s: %s\n  path %s", kb.Name, where, kb.Description, kb.Path)
		if kb.Index != "" {
			fmt.Fprintf(&b, " · index %s", filepath.Base(kb.Index))
		}
		if kb.Rules != "" {
			fmt.Fprintf(&b, " · rules %s", filepath.Base(kb.Rules))
		}
		fmt.Fprintf(&b, " · edits: %s\n", kb.Edits)
		if body := kb.preload(); body != "" {
			for _, line := range strings.Split(body, "\n") {
				b.WriteString("    " + line + "\n")
			}
		}
	}
	return b.String()
}

// preload is the part of a base injected at session start.
// shortcut: no cache - reading an index takes well under a millisecond;
// cache in $XDG_CACHE_HOME/lplug if indexes ever get huge.
func (kb *KB) preload() string {
	var text string
	switch {
	case kb.Preload == "none":
		return ""
	case kb.Kind == "scripts":
		text = scriptIndex(kb.Path, kb.Preload == "full")
	case kb.Index != "":
		data, _ := os.ReadFile(kb.Index)
		text = strings.TrimSpace(string(data))
		if kb.Preload == "outline" {
			text = outline(text)
		}
	}
	if len(text) > kb.Limit {
		cut := strings.LastIndexByte(text[:kb.Limit], '\n')
		if cut <= 0 {
			cut = kb.Limit
		}
		text = text[:cut] + fmt.Sprintf("\n… cut at %d characters; open the index for the rest.", kb.Limit)
	}
	return text
}

// outline keeps an index's headings and link lines: a topic map.
func outline(text string) string {
	var keep []string
	for _, line := range strings.Split(text, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "#") || strings.Contains(t, "](") || strings.Contains(t, "[[") {
			keep = append(keep, strings.TrimRight(line, " \t\r"))
		}
	}
	return strings.Join(keep, "\n")
}

// scriptIndex lists a script library from each script's header comment:
// Purpose (always), Usage and Tags (full).
func scriptIndex(dir string, full bool) string {
	entries, _ := os.ReadDir(dir)
	var lines []string
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		h := scriptHeader(filepath.Join(dir, e.Name()))
		line := "- " + e.Name()
		if h["purpose"] != "" {
			line += ": " + h["purpose"]
		} else {
			line += " (no Purpose: header)"
		}
		if full && h["usage"] != "" {
			line += "\n  usage: " + h["usage"]
		}
		if full && h["tags"] != "" {
			line += "\n  tags: " + h["tags"]
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

// scriptHeader reads "Purpose:", "Usage:", "Tags:", "Tested:" from the
// first comment lines, whatever the comment style.
func scriptHeader(path string) map[string]string {
	h := map[string]string{}
	data, err := os.ReadFile(path)
	if err != nil {
		return h
	}
	for i, line := range strings.Split(string(data), "\n") {
		if i > 25 {
			break
		}
		t := strings.TrimLeft(line, "#/-;\"' \t")
		if k, v, ok := strings.Cut(t, ":"); ok {
			if key := strings.ToLower(strings.TrimSpace(k)); key == "purpose" || key == "usage" || key == "tags" || key == "tested" {
				h[key] = strings.TrimSpace(v)
			}
		}
	}
	return h
}
