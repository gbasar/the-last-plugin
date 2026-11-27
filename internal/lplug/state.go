package lplug

// State under $XDG_STATE_HOME/lplug: per-session counters and the toggles file.

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

const staleSessionAge = 7 * 24 * time.Hour

func sessionDir(stateDir, agent, session string) string {
	if session == "" {
		session = "no-session"
	}
	return filepath.Join(stateDir, "sessions", safeName(agent)+"-"+safeName(session))
}

// nth records token under key and returns its 1-based position. No locks:
// small O_APPEND writes are atomic, so concurrent hooks each land on their
// own line and every call gets a distinct number.
func nth(dir, key, token string) (int, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return 0, err
	}
	file := filepath.Join(dir, key+".count")
	f, err := os.OpenFile(file, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o600)
	if err != nil {
		return 0, err
	}
	_, err = f.WriteString(token + "\n")
	f.Close()
	if err != nil {
		return 0, err
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return 0, err
	}
	for n, line := range strings.Split(string(data), "\n") {
		if line == token {
			return n + 1, nil
		}
	}
	return 0, fmt.Errorf("counter %s: token not found", key)
}

// once claims key in dir; only the first caller gets true.
func once(dir, key string) (bool, error) {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return false, err
	}
	f, err := os.OpenFile(filepath.Join(dir, key+".once"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if os.IsExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, f.Close()
}

// pruneSessions deletes sessions idle for longer than age; crashed sessions
// never send session_end.
func pruneSessions(stateDir string, age time.Duration) {
	dir := filepath.Join(stateDir, "sessions")
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if fi, err := e.Info(); err == nil && time.Since(fi.ModTime()) > age {
			os.RemoveAll(filepath.Join(dir, e.Name()))
		}
	}
}

// ---- toggles: one "pattern on|off" per line; the most recently set
// matching line wins, so `off @work` then `on ponytail` leaves ponytail on.

type toggle struct {
	Pattern string
	On      bool
}

type toggles struct {
	File    string
	Entries []toggle
}

func loadToggles(stateDir string) *toggles {
	t := &toggles{File: filepath.Join(stateDir, "toggles")}
	data, _ := os.ReadFile(t.File)
	for _, line := range strings.Split(string(data), "\n") {
		if f := strings.Fields(line); len(f) == 2 && !strings.HasPrefix(f[0], "#") {
			t.Entries = append(t.Entries, toggle{f[0], f[1] == "on"})
		}
	}
	return t
}

// state returns the switch for an item known by names, and the pattern that
// decided it ("" = no toggle applies; use the item's default).
func (t *toggles) state(names []string) (on bool, by string) {
	for i := len(t.Entries) - 1; i >= 0; i-- {
		for _, n := range names {
			if ok, _ := path.Match(t.Entries[i].Pattern, n); ok {
				return t.Entries[i].On, t.Entries[i].Pattern
			}
		}
	}
	return false, ""
}

// set records pattern, moving it to the end so it wins.
func (t *toggles) set(pattern string, on bool) {
	t.reset(pattern)
	t.Entries = append(t.Entries, toggle{pattern, on})
}

// reset forgets pattern; reports whether it was set.
func (t *toggles) reset(pattern string) bool {
	kept, found := t.Entries[:0], false
	for _, e := range t.Entries {
		if e.Pattern == pattern {
			found = true
		} else {
			kept = append(kept, e)
		}
	}
	t.Entries = kept
	return found
}

// save writes atomically (temp file + rename).
func (t *toggles) save() error {
	if err := os.MkdirAll(filepath.Dir(t.File), 0o700); err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString("# lplug toggles: pattern on|off, last match wins. Edit with `lplug on|off|reset`.\n")
	for _, e := range t.Entries {
		fmt.Fprintf(&b, "%s %s\n", e.Pattern, map[bool]string{true: "on", false: "off"}[e.On])
	}
	tmp := t.File + ".tmp"
	if err := os.WriteFile(tmp, []byte(b.String()), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, t.File)
}

func validPattern(p string) error {
	if p == "" || strings.ContainsAny(p, " \t\n") {
		return fmt.Errorf("name must be non-empty, without spaces")
	}
	if _, err := path.Match(p, ""); err != nil {
		return fmt.Errorf("bad pattern %q: %v", p, err)
	}
	return nil
}
