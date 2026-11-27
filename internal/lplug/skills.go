package lplug

// Skills: each layer's skills.d/<name>/SKILL.md, linked into the agent's
// skills dir (Claude Code: ~/.claude/skills) on every session start and by
// `lplug skills sync`. A project skill replaces a personal one of the same
// name; toggled-off skills are unlinked. lplug only ever touches links that
// point into a skills.d dir - never real folders or anyone else's links.

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func findSkills(c *Config) []Item {
	at := map[string]int{}
	var out []Item
	for _, l := range c.Layers {
		dir := filepath.Join(l.Dir, "skills.d")
		entries, _ := os.ReadDir(dir)
		for _, e := range entries {
			p := filepath.Join(dir, e.Name())
			if !isDir(p) || !exists(filepath.Join(p, "SKILL.md")) {
				continue
			}
			it := Item{Path: p, Kind: kindSkill, Layer: l}
			if i, ok := at[e.Name()]; ok {
				out[i] = it
			} else {
				at[e.Name()] = len(out)
				out = append(out, it)
			}
		}
	}
	return out
}

// syncSkills makes target hold exactly lplug's enabled skills. skipped are
// names taken by something lplug doesn't manage.
func syncSkills(c *Config, target string) (linked, removed, skipped []string, err error) {
	t := loadToggles(c.StateDir)
	want := map[string]string{}
	for _, it := range findSkills(c) {
		if on, _ := it.enabled(t); on {
			want[filepath.Base(it.Path)] = it.Path
		}
	}
	entries, _ := os.ReadDir(target)
	if len(want) == 0 && len(entries) == 0 {
		return
	}
	if err = os.MkdirAll(target, 0o755); err != nil {
		return
	}
	for _, e := range entries { // drop our links that are stale or switched off
		link := filepath.Join(target, e.Name())
		if dest, err := os.Readlink(link); err == nil && ownedLink(dest) && want[e.Name()] != dest {
			if os.Remove(link) == nil {
				removed = append(removed, e.Name())
			}
		}
	}
	names := make([]string, 0, len(want))
	for n := range want {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, name := range names {
		link := filepath.Join(target, name)
		if dest, err := os.Readlink(link); err == nil && dest == want[name] {
			continue
		}
		if _, err := os.Lstat(link); err == nil {
			skipped = append(skipped, name)
			continue
		}
		if err = os.Symlink(want[name], link); err != nil {
			return
		}
		linked = append(linked, name)
	}
	return
}

func ownedLink(dest string) bool { return strings.Contains(filepath.ToSlash(dest), "/skills.d/") }
