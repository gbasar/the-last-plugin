package lplug

// Rule files: INI, TOML, JSON or HOCON, one shape. Top-level keys are rule
// ids; each holds the settings in ruleKeys. Within a file, rules apply in id
// order; a later layer's rule replaces an earlier one with the same id.

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/gurkankaymak/hocon"
)

type Rule struct {
	ID, File, Description string
	Events, Agents, Tags  []string
	Tool, When, Unless    *regexp.Regexp
	Every                 int  // fire on every Nth matching event in a session
	Once                  bool // at most once per session
	Enabled               bool // default state; `lplug on|off` overrides
	// Actions, applied in this order:
	Text, FilePath, Ask, Run, Confirm, Block string
}

var ruleKeys = []struct{ Name, Help string }{
	{"event", "required; neutral or native event name(s), comma-separated or a list"},
	{"description", "shown in `lplug status`"},
	{"agent", "only for these agents (default: all)"},
	{"tags", "groups, toggled together with `lplug off @tag`"},
	{"enabled", "default on/off (default true); `lplug on|off` overrides"},
	{"tool", "regex the tool name must match (tool events)"},
	{"when", "regex the subject must match: the prompt, shell command, tool input JSON, or (stop) the agent's last message"},
	{"unless", "regex the subject must NOT match"},
	{"every", "fire on every Nth matching event in a session"},
	{"once", "fire at most once per session"},
	{"text", "action: inject this text"},
	{"file", "action: inject a file (relative names are looked up on the text path)"},
	{"ask", "action: ask the agent to state its intent (true, or your own question)"},
	{"run", "action: run a program: `lplug ...` (itself), a bare name (PATH), or a path"},
	{"confirm", "action: ask the user to approve the tool call, with this reason"},
	{"block", "action: block, with this reason"},
}

// loadRules parses one rule file. Invalid rules are skipped with a warning;
// an unparseable file is an error.
func loadRules(path string) ([]Rule, []string, error) {
	doc, err := parseRuleDoc(path)
	if err != nil {
		return nil, nil, err
	}
	ids := make([]string, 0, len(doc))
	for id := range doc {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var rules []Rule
	var warns []string
	for _, id := range ids {
		r, w, err := decodeRule(id, path, doc[id])
		warns = append(warns, w...)
		if err != nil {
			warns = append(warns, fmt.Sprintf("rule %q skipped: %v", id, err))
			continue
		}
		rules = append(rules, r)
	}
	return rules, warns, nil
}

func parseRuleDoc(path string) (map[string]map[string]any, error) {
	var doc map[string]any
	switch strings.ToLower(filepath.Ext(path)) {
	case ".ini", ".toml":
		// shortcut: TOML is read with the INI parser, so only flat [id] tables of
		// key = value work - all a rule file needs. Nested tables or multi-line
		// strings would need a real TOML parser.
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		sections, err := parseINI(data)
		if err != nil {
			return nil, err
		}
		doc = map[string]any{}
		for id, kv := range sections {
			t := map[string]any{}
			for k, v := range kv {
				t[k] = v
			}
			doc[id] = t
		}
	case ".json":
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(data, &doc); err != nil {
			return nil, err
		}
	case ".conf", ".hocon":
		cfg, err := hocon.ParseResource(path)
		if err != nil {
			return nil, err
		}
		m, ok := fromHOCON(cfg.GetRoot()).(map[string]any)
		if !ok {
			return nil, fmt.Errorf("top level must be an object")
		}
		doc = m
	default:
		return nil, fmt.Errorf("unsupported rule format %q", filepath.Ext(path))
	}
	out := make(map[string]map[string]any, len(doc))
	for id, v := range doc {
		t, ok := v.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%q must be a table of settings", id)
		}
		out[id] = t
	}
	return out, nil
}

func fromHOCON(v hocon.Value) any {
	switch t := v.(type) {
	case hocon.Object:
		m := make(map[string]any, len(t))
		for k, e := range t {
			m[k] = fromHOCON(e)
		}
		return m
	case hocon.Array:
		a := make([]any, len(t))
		for i, e := range t {
			a[i] = fromHOCON(e)
		}
		return a
	case hocon.String:
		return string(t)
	case hocon.Int:
		return int(t)
	case hocon.Boolean:
		return bool(t)
	case hocon.Null, nil:
		return nil
	default:
		return strings.Trim(t.String(), `"`)
	}
}

func decodeRule(id, file string, m map[string]any) (Rule, []string, error) {
	r := Rule{ID: id, File: file, Enabled: true}
	var warns []string
	known := map[string]bool{}
	for _, k := range ruleKeys {
		known[k.Name] = true
	}
	for k := range m {
		if !known[k] {
			warns = append(warns, fmt.Sprintf("rule %q: unknown key %q", id, k))
		}
	}
	if r.Events = toList(m["event"]); len(r.Events) == 0 {
		return r, warns, fmt.Errorf("missing event")
	}
	r.Agents, r.Tags, r.Description = toList(m["agent"]), toList(m["tags"]), toStr(m["description"])
	var err error
	for key, re := range map[string]**regexp.Regexp{"tool": &r.Tool, "when": &r.When, "unless": &r.Unless} {
		if s := toStr(m[key]); s != "" {
			if *re, err = regexp.Compile(s); err != nil {
				return r, warns, fmt.Errorf("%s: %w", key, err)
			}
		}
	}
	if s := toStr(m["every"]); s != "" {
		if r.Every, err = strconv.Atoi(s); err != nil || r.Every < 1 {
			return r, warns, fmt.Errorf("every: want a positive number, got %q", s)
		}
	}
	if r.Once, err = toBool(m["once"], false); err != nil {
		return r, warns, fmt.Errorf("once: %w", err)
	}
	if r.Enabled, err = toBool(m["enabled"], true); err != nil {
		return r, warns, fmt.Errorf("enabled: %w", err)
	}
	r.Text, r.FilePath, r.Run = toStr(m["text"]), toStr(m["file"]), toStr(m["run"])
	r.Confirm, r.Block = toStr(m["confirm"]), toStr(m["block"])
	if r.Ask = toStr(m["ask"]); r.Ask != "" {
		if on, err := toBool(r.Ask, true); err == nil {
			r.Ask = map[bool]string{true: defaultAsk, false: ""}[on]
		}
	}
	if r.Text+r.FilePath+r.Ask+r.Run+r.Confirm+r.Block == "" {
		warns = append(warns, fmt.Sprintf("rule %q has no action", id))
	}
	return r, warns, nil
}

// Matches checks event, agent, tool, when and unless. Counting and toggles
// are the dispatcher's job.
func (r *Rule) Matches(ev Event) bool {
	return anyOf(r.Events, ev.Is) &&
		(len(r.Agents) == 0 || anyOf(r.Agents, func(a string) bool { return a == ev.Agent })) &&
		(r.Tool == nil || r.Tool.MatchString(ev.ToolName)) &&
		(r.When == nil || r.When.MatchString(ev.Subject)) &&
		(r.Unless == nil || !r.Unless.MatchString(ev.Subject))
}

// stateKey names the rule's counters; the file hash keeps equal ids apart.
func (r *Rule) stateKey() string {
	h := sha1.Sum([]byte(r.File))
	return hex.EncodeToString(h[:4]) + "-" + safeName(r.ID)
}

func anyOf(xs []string, f func(string) bool) bool {
	for _, x := range xs {
		if f(x) {
			return true
		}
	}
	return false
}

func safeName(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '-' || r == '_' || ('a' <= r && r <= 'z') || ('A' <= r && r <= 'Z') || ('0' <= r && r <= '9') {
			return r
		}
		return '_'
	}, s)
}

func toStr(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(t)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	}
	return fmt.Sprint(v)
}

// toList takes a list, "a, b", or a TOML-style ["a", "b"] read as text.
func toList(v any) []string {
	var parts []string
	if a, ok := v.([]any); ok {
		for _, e := range a {
			parts = append(parts, toStr(e))
		}
	} else {
		s := toStr(v)
		if strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") {
			s = s[1 : len(s)-1]
		}
		parts = strings.Split(s, ",")
	}
	var out []string
	for _, p := range parts {
		if p = strings.Trim(strings.TrimSpace(p), `"'`); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func toBool(v any, def bool) (bool, error) {
	if b, ok := v.(bool); ok {
		return b, nil
	}
	switch strings.ToLower(toStr(v)) {
	case "":
		return def, nil
	case "true", "yes", "on", "1":
		return true, nil
	case "false", "no", "off", "0":
		return false, nil
	}
	return def, fmt.Errorf("want true or false, got %q", toStr(v))
}
