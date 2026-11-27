package lplug

// Config: where lplug reads and writes, and how it runs.
//
// Layers, lowest to highest precedence (environment variables beat both):
//
//	user     $LPLUG_CONFIG_DIR, else $XDG_CONFIG_HOME/lplug (~/.config/lplug)
//	project  the nearest <dir>/.config/lplug above the working directory,
//	         never the home directory's own ~/.config/lplug
//
// Each layer may hold settings.ini:
//
//	[lplug]
//	state_dir = ~/work/lplug-state
//	text_path = text:~/team/text      ; searched for relative `file =` values
//	rules     = rules.d:~/team/rules.d
//	timeout   = 5s
//	debug     = false
//
//	[sources]
//	session_start = session-start.d:~/team/session-start.d
//
// Relative paths in a layer's files are relative to its root: the config
// dir for the user layer, the project root for a project layer.

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	settingsFile   = "settings.ini"
	defaultTimeout = 10 * time.Second
)

type Layer struct {
	Name     string // "user" or "project"
	Dir      string // the layer's lplug config dir
	Root     string // what relative paths in its files are relative to
	Settings map[string]map[string]string
}

// Label is how `status` shows where something lives.
func (l *Layer) Label() string {
	if l.Name == "project" {
		return "repo"
	}
	return "personal"
}

func (l *Layer) path(p string) string {
	p = expandHome(strings.TrimSpace(p))
	if !filepath.IsAbs(p) {
		p = filepath.Join(l.Root, p)
	}
	return p
}

func (l *Layer) list(v string) []string {
	var out []string
	for _, p := range splitList(v) {
		out = append(out, l.path(p))
	}
	return out
}

// Src is a configured file or directory and the layer it came from.
type Src struct {
	Path  string
	Layer *Layer
}

type Config struct {
	Layers   []*Layer // user first, then project: later layers win
	Cwd      string
	StateDir string
	TextPath []string
	Timeout  time.Duration
	Debug    bool
	Origin   map[string]string // setting -> where its value came from
	Warnings []string
}

// settingKeys documents every setting, in display order.
var settingKeys = []struct{ Key, Env, Help string }{
	{"config_dir", "LPLUG_CONFIG_DIR", "your personal layer (env only)"},
	{"state_dir", "LPLUG_STATE_DIR", "toggles, counters, lplug.log"},
	{"text_path", "LPLUG_TEXT_PATH", "dirs searched for relative `file =` values"},
	{"rules", "LPLUG_RULES", "rule sources consulted on every event"},
	{"timeout", "LPLUG_TIMEOUT", "per-program timeout: seconds, or a duration like 5s"},
	{"debug", "LPLUG_DEBUG", "log every decision to lplug.log"},
}

// LoadConfig resolves layers and settings for a working directory.
func LoadConfig(cwd string) *Config {
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	c := &Config{Cwd: cwd, Origin: map[string]string{}}

	userDir, from := filepath.Join(xdgDir("XDG_CONFIG_HOME", ".config"), "lplug"), "default (XDG)"
	if v := os.Getenv("LPLUG_CONFIG_DIR"); v != "" {
		userDir, from = absPath(v), "env LPLUG_CONFIG_DIR"
	}
	c.Origin["config_dir"] = from
	c.addLayer("user", userDir, userDir)
	if root := findProject(cwd, userDir); root != "" {
		c.addLayer("project", filepath.Join(root, ".config", "lplug"), root)
	}

	c.StateDir = filepath.Join(xdgDir("XDG_STATE_HOME", filepath.Join(".local", "state")), "lplug")
	c.Origin["state_dir"] = "default (XDG)"
	if v, from, l := c.setting("state_dir"); v != "" {
		c.StateDir, c.Origin["state_dir"] = c.resolve(v, l), from
	}

	// Text path: every layer contributes, the project's dirs first.
	if v := os.Getenv("LPLUG_TEXT_PATH"); v != "" {
		c.TextPath, c.Origin["text_path"] = absList(v), "env LPLUG_TEXT_PATH"
	} else {
		c.Origin["text_path"] = "layers"
		for i := len(c.Layers) - 1; i >= 0; i-- {
			l := c.Layers[i]
			if v := l.Settings["lplug"]["text_path"]; v != "" {
				c.TextPath = append(c.TextPath, l.list(v)...)
			} else {
				c.TextPath = append(c.TextPath, filepath.Join(l.Dir, "text"))
			}
		}
	}

	c.Timeout, c.Origin["timeout"] = defaultTimeout, "default"
	if v, from, _ := c.setting("timeout"); v != "" {
		if d, err := parseDuration(v); err == nil {
			c.Timeout, c.Origin["timeout"] = d, from
		} else {
			c.warn("timeout %q (%s): %v", v, from, err)
		}
	}

	c.Origin["debug"] = "default"
	if v, from, _ := c.setting("debug"); v != "" {
		c.Debug, c.Origin["debug"] = truthy(v), from
	}
	c.Origin["rules"] = "layers"
	if os.Getenv("LPLUG_RULES") != "" {
		c.Origin["rules"] = "env LPLUG_RULES"
	}
	return c
}

func (c *Config) addLayer(name, dir, root string) {
	l := &Layer{Name: name, Dir: dir, Root: root}
	path := filepath.Join(dir, settingsFile)
	if data, err := os.ReadFile(path); err == nil {
		if l.Settings, err = parseINI(data); err != nil {
			c.warn("%s: %v (ignored)", path, err)
		}
		c.checkSettings(path, l.Settings)
	}
	c.Layers = append(c.Layers, l)
}

func (c *Config) checkSettings(path string, doc map[string]map[string]string) {
	known := map[string]bool{}
	for _, k := range settingKeys[1:] { // config_dir can't be set from inside itself
		known[k.Key] = true
	}
	for section, kv := range doc {
		switch section {
		case "lplug":
			for k := range kv {
				if !known[k] {
					c.warn("%s: [lplug] unknown key %q", path, k)
				}
			}
		case "sources":
		default:
			c.warn("%s: unknown section [%s]", path, section)
		}
	}
}

// setting: env > project [lplug] > user [lplug]. layer is nil for env values.
func (c *Config) setting(key string) (value, from string, layer *Layer) {
	env := "LPLUG_" + strings.ToUpper(key)
	if v := os.Getenv(env); v != "" {
		return v, "env " + env, nil
	}
	for i := len(c.Layers) - 1; i >= 0; i-- {
		l := c.Layers[i]
		if v := l.Settings["lplug"][key]; v != "" {
			return v, l.Label() + " " + settingsFile, l
		}
	}
	return "", "", nil
}

func (c *Config) resolve(v string, l *Layer) string {
	if l == nil {
		return absPath(v)
	}
	return l.path(v)
}

// EventSources: $LPLUG_<EVENT> replaces everything; otherwise every layer's
// [sources] entry for the event, or its <event>.d directory.
func (c *Config) EventSources(ev string) []Src {
	if v := os.Getenv(eventEnv(ev)); v != "" {
		return c.envSrcs(v)
	}
	var out []Src
	for _, l := range c.Layers {
		if v := l.Settings["sources"][ev]; v != "" {
			for _, p := range l.list(v) {
				out = append(out, Src{p, l})
			}
		} else {
			out = append(out, Src{filepath.Join(l.Dir, eventDir(ev)), l})
		}
	}
	return out
}

// RuleSources: $LPLUG_RULES, else every layer's `rules` setting or rules.d.
func (c *Config) RuleSources() []Src {
	if v := os.Getenv("LPLUG_RULES"); v != "" {
		return c.envSrcs(v)
	}
	var out []Src
	for _, l := range c.Layers {
		if v := l.Settings["lplug"]["rules"]; v != "" {
			for _, p := range l.list(v) {
				out = append(out, Src{p, l})
			}
		} else {
			out = append(out, Src{filepath.Join(l.Dir, "rules.d"), l})
		}
	}
	return out
}

// envSrcs: paths from env vars count as personal.
func (c *Config) envSrcs(v string) []Src {
	var out []Src
	for _, p := range absList(v) {
		out = append(out, Src{p, c.Layers[0]})
	}
	return out
}

// ResolveText finds a `file =` value: absolute and ~ paths as-is, relative
// ones in the first text_path dir that has them, then the rule's own layer.
func (c *Config) ResolveText(p string, l *Layer) string {
	p = expandHome(p)
	if filepath.IsAbs(p) {
		return p
	}
	for _, dir := range c.TextPath {
		if cand := filepath.Join(dir, p); exists(cand) {
			return cand
		}
	}
	return l.path(p)
}

func (c *Config) warn(format string, a ...any) {
	c.Warnings = append(c.Warnings, fmt.Sprintf(format, a...))
}

// findProject walks up from cwd to the nearest dir holding .config/lplug,
// stopping before the home dir (its ~/.config/lplug is the user layer).
func findProject(cwd, userDir string) string {
	home, _ := os.UserHomeDir()
	for dir := cwd; dir != "" && dir != home; {
		if cand := filepath.Join(dir, ".config", "lplug"); cand != userDir && isDir(cand) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

// ---- INI: one dialect for rule files, TOML-style rule files, settings and
// knowledge-base maps.
//
//	[section]
//	key = value            ; unquoted: " ;" or " #" starts an inline comment
//	key = "a \"quoted\"\n"   double quotes take \n \t \" \\ escapes
//	key = 'literal ; text'   single quotes are taken as-is

func parseINI(data []byte) (map[string]map[string]string, error) {
	out := map[string]map[string]string{}
	var cur map[string]string
	for i, line := range strings.Split(string(data), "\n") {
		n, l := i+1, strings.TrimSpace(line)
		switch {
		case l == "" || l[0] == '#' || l[0] == ';':
			continue
		case l[0] == '[':
			name := strings.TrimSpace(strings.TrimSuffix(l[1:], "]"))
			if !strings.HasSuffix(l, "]") || name == "" {
				return nil, fmt.Errorf("line %d: bad section header", n)
			}
			if _, dup := out[name]; dup {
				return nil, fmt.Errorf("line %d: duplicate section [%s]", n, name)
			}
			cur = map[string]string{}
			out[name] = cur
			continue
		}
		k, v, ok := strings.Cut(l, "=")
		if !ok {
			return nil, fmt.Errorf("line %d: expected key = value", n)
		}
		if cur == nil {
			return nil, fmt.Errorf("line %d: %q is outside any [section]", n, strings.TrimSpace(k))
		}
		v, err := iniValue(strings.TrimSpace(v))
		if err != nil {
			return nil, fmt.Errorf("line %d: %v", n, err)
		}
		cur[strings.TrimSpace(k)] = v
	}
	return out, nil
}

func iniValue(v string) (string, error) {
	if len(v) >= 2 && (v[0] == '"' || v[0] == '\'') {
		end := strings.LastIndexByte(v, v[0])
		if end <= 0 {
			return "", fmt.Errorf("unterminated quote")
		}
		if v[0] == '\'' {
			return v[1:end], nil
		}
		return strconv.Unquote(v[:end+1])
	}
	for _, c := range []string{" ;", "\t;", " #", "\t#"} {
		if j := strings.Index(v, c); j >= 0 {
			v = strings.TrimSpace(v[:j])
		}
	}
	return v, nil
}

// ---- small path and value helpers

// xdgDir: $env if set to an absolute path (relative values are invalid per
// the XDG spec), else ~/fallback. The same on every OS, on purpose.
func xdgDir(env, fallback string) string {
	if v := os.Getenv(env); filepath.IsAbs(v) {
		return v
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, fallback)
}

func expandHome(p string) string {
	if p != "~" && !strings.HasPrefix(p, "~/") && !strings.HasPrefix(p, `~\`) {
		return p
	}
	home, _ := os.UserHomeDir()
	if p == "~" {
		return home
	}
	return filepath.Join(home, p[2:])
}

func absPath(p string) string {
	p = expandHome(strings.TrimSpace(p))
	if abs, err := filepath.Abs(p); err == nil {
		return abs
	}
	return p
}

func absList(v string) []string {
	var out []string
	for _, p := range splitList(v) {
		out = append(out, absPath(p))
	}
	return out
}

// splitList splits on the OS list separator: ':' on Unix, ';' on Windows.
func splitList(v string) []string {
	var out []string
	for _, p := range filepath.SplitList(v) {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func parseDuration(v string) (time.Duration, error) {
	if d, err := time.ParseDuration(v); err == nil {
		return d, nil
	}
	secs, err := strconv.ParseFloat(v, 64)
	if err != nil || secs <= 0 {
		return 0, fmt.Errorf("want seconds or a duration like 5s")
	}
	return time.Duration(secs * float64(time.Second)), nil
}

func truthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

func exists(p string) bool { _, err := os.Stat(p); return err == nil }

func isDir(p string) bool { fi, err := os.Stat(p); return err == nil && fi.IsDir() }
