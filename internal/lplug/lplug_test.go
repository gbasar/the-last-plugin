package lplug

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// fixture: an isolated home with a personal layer, state dir and a project.
type fixture struct {
	t                          *testing.T
	home, user, state, project string
}

func setup(t *testing.T) *fixture {
	home := t.TempDir()
	f := &fixture{t: t, home: home,
		user:    filepath.Join(home, ".config", "lplug"),
		state:   filepath.Join(home, ".local", "state", "lplug"),
		project: filepath.Join(home, "work", "proj")}
	t.Setenv("HOME", home)
	for _, k := range []string{"XDG_CONFIG_HOME", "XDG_STATE_HOME", "LPLUG_CONFIG_DIR", "LPLUG_STATE_DIR",
		"LPLUG_TEXT_PATH", "LPLUG_RULES", "LPLUG_KB_MAP", "LPLUG_TIMEOUT", "LPLUG_DEBUG", "CLAUDE_CONFIG_DIR"} {
		t.Setenv(k, "")
	}
	os.MkdirAll(f.project, 0o755)
	return f
}

// write puts a file at path (absolute, or relative to the personal layer).
func (f *fixture) write(path, body string, mode os.FileMode) {
	if !filepath.IsAbs(path) {
		path = filepath.Join(f.user, path)
	}
	os.MkdirAll(filepath.Dir(path), 0o755)
	if err := os.WriteFile(path, []byte(body), mode); err != nil {
		f.t.Fatal(err)
	}
	os.Chmod(path, mode) // umask-proof
}

// hook runs one event in cwd; returns exit code, injected context and stderr.
func (f *fixture) hook(native, cwd, payload string) (code int, ctx, stderr, ask string) {
	f.t.Helper()
	var out, errb bytes.Buffer
	d := &Dispatcher{A: claudeCode{}, C: LoadConfig(cwd)}
	d.C.Timeout = 2 * time.Second
	code = d.Run(native, strings.NewReader(payload), &out, &errb)
	if out.Len() > 0 {
		var o struct {
			H map[string]string `json:"hookSpecificOutput"`
		}
		if err := json.Unmarshal(out.Bytes(), &o); err != nil {
			f.t.Fatalf("bad stdout %q: %v", out.String(), err)
		}
		ctx, ask = o.H["additionalContext"], o.H["permissionDecisionReason"]
	}
	return code, ctx, strings.TrimSpace(errb.String()), ask
}

func TestFormatsAgree(t *testing.T) {
	f := setup(t)
	files := map[string]string{
		"r.ini":  "[tests]\nevent = tool_after ; comment\ntool = Bash\nevery = 5\ntext = \"Run the tests ; now\"\n",
		"r.toml": "[tests]\nevent = [\"tool_after\"]\ntool = \"Bash\"\nevery = 5\ntext = \"Run the tests ; now\"\n",
		"r.json": `{"tests":{"event":"tool_after","tool":"Bash","every":5,"text":"Run the tests ; now"}}`,
		"r.conf": "tests { event = tool_after, tool = Bash, every = 5, text = \"Run the tests ; now\" }\n",
	}
	for name, body := range files {
		f.write(name, body, 0o644)
		rules, warns, err := loadRules(filepath.Join(f.user, name))
		if err != nil || len(warns) > 0 || len(rules) != 1 {
			t.Fatalf("%s: rules=%d warns=%v err=%v", name, len(rules), warns, err)
		}
		r := rules[0]
		if r.ID != "tests" || r.Every != 5 || r.Text != "Run the tests ; now" || r.Tool.String() != "Bash" || r.Events[0] != "tool_after" {
			t.Errorf("%s: got %+v", name, r)
		}
	}
}

func TestRuleWarnings(t *testing.T) {
	f := setup(t)
	f.write("w.ini", "[a]\nevnet = x\ntext = y\n[b]\nevent = stop\nevery = zero\n[c]\nevent = stop\n", 0o644)
	rules, warns, _ := loadRules(filepath.Join(f.user, "w.ini"))
	if len(rules) != 1 || rules[0].ID != "c" || len(warns) != 4 {
		t.Fatalf("rules=%+v warns=%q", rules, warns)
	}
}

func TestCountersUniqueUnderConcurrency(t *testing.T) {
	dir := t.TempDir()
	const calls = 200
	seen := make([]int, calls)
	var wg sync.WaitGroup
	for i := range calls {
		wg.Add(1)
		go func() { defer wg.Done(); seen[i], _ = nth(dir, "k", fmt.Sprint("tok", i)) }()
	}
	wg.Wait()
	used := map[int]bool{}
	for _, n := range seen {
		if n < 1 || n > calls || used[n] {
			t.Fatalf("position %d duplicated or out of range", n)
		}
		used[n] = true
	}
}

func TestTogglesLastMatchWins(t *testing.T) {
	dir := t.TempDir()
	tg := loadToggles(dir)
	tg.set("@work", false)
	tg.set("ponytail", true)
	tg.save()
	tg = loadToggles(dir)
	if on, by := tg.state([]string{"ponytail", "@work"}); !on || by != "ponytail" {
		t.Fatalf("got %v %q", on, by)
	}
	if on, by := tg.state([]string{"tests", "@work"}); on || by != "@work" {
		t.Fatalf("got %v %q", on, by)
	}
}

func TestTextScriptsOrderAndTrust(t *testing.T) {
	f := setup(t)
	f.write("session-start.d/50-ponytail.md", "Code very efficiently.\n", 0o644)
	f.write("session-start.d/10-env.sh", "#!/bin/sh\necho \"$LPLUG_EVENT $LPLUG_NATIVE_EVENT\"\n", 0o755)
	f.write("session-start.d/20-broken.sh", "#!/bin/sh\nexit 7\n", 0o755)
	f.write("session-start.d/30-shared.md", "skipped: group-writable\n", 0o666)
	if code, ctx, _, _ := f.hook("SessionStart", f.project, `{"session_id":"s"}`); code != 0 || ctx != "session_start SessionStart\n\nCode very efficiently." {
		t.Fatalf("code=%d ctx=%q", code, ctx)
	}
}

func TestBlockConfirmUnless(t *testing.T) {
	f := setup(t)
	f.write("tool-before.d/guard.sh", "#!/bin/sh\ngrep -q 'rm -rf' && { echo 'script says no' >&2; exit 2; }\nexit 0\n", 0o755)
	f.write("rules.d/r.ini", "[no-rm]\nevent = tool_before\nwhen = rm -rf\nunless = --dry-run\nblock = rule says no\n"+
		"[scan]\nevent = tool_before\nwhen = nmap\nconfirm = really scan?\n", 0o644)
	if code, _, stderr, _ := f.hook("PreToolUse", f.project, `{"tool_name":"Bash","tool_input":{"command":"rm -rf /"}}`); code != 2 || stderr != "script says no\nrule says no" {
		t.Fatalf("block: code=%d stderr=%q", code, stderr)
	}
	f.write("tool-before.d/guard.sh", "#!/bin/sh\nexit 0\n", 0o755)
	if code, _, _, _ := f.hook("PreToolUse", f.project, `{"tool_name":"Bash","tool_input":{"command":"rm -rf x --dry-run"}}`); code != 0 {
		t.Fatalf("unless should skip the rule: code=%d", code)
	}
	if code, _, _, ask := f.hook("PreToolUse", f.project, `{"tool_name":"Bash","tool_input":{"command":"nmap host"}}`); code != 0 || ask != "really scan?" {
		t.Fatalf("confirm: code=%d ask=%q", code, ask)
	}
}

func TestEveryOnceToggles(t *testing.T) {
	f := setup(t)
	f.write("rules.d/r.ini", "[tests]\nevent = tool_after\ntool = Bash\nevery = 2\ntext = run tests\ntags = work\n"+
		"[hi]\nevent = tool_after\nonce = true\ntext = hello\n", 0o644)
	want := []string{"hello", "", "run tests", "", "run tests"} // Read doesn't count toward every
	for i, tool := range []string{"Bash", "Read", "Bash", "Bash", "Bash"} {
		_, ctx, _, _ := f.hook("PostToolUse", f.project, fmt.Sprintf(`{"session_id":"s","tool_name":%q,"tool_use_id":"u%d"}`, tool, i))
		if ctx != want[i] {
			t.Fatalf("call %d: got %q want %q", i+1, ctx, want[i])
		}
	}
	tg := loadToggles(f.state)
	tg.set("@work", false)
	tg.save()
	for i := range 4 {
		if _, ctx, _, _ := f.hook("PostToolUse", f.project, fmt.Sprintf(`{"session_id":"s","tool_name":"Bash","tool_use_id":"x%d"}`, i)); ctx != "" {
			t.Fatalf("toggled-off rule fired: %q", ctx)
		}
	}
}

func TestNativeOutputAndTimeout(t *testing.T) {
	f := setup(t)
	f.write("tool-after.d/native.sh", "#!/bin/sh\necho '{\"hookSpecificOutput\":{\"additionalContext\":\"from native\"}}'\n", 0o755)
	f.write("tool-after.d/slow.sh", "#!/bin/sh\nsleep 5\necho late\n", 0o755)
	start := time.Now()
	if _, ctx, _, _ := f.hook("PostToolUse", f.project, `{}`); ctx != "from native" {
		t.Fatalf("ctx=%q", ctx)
	}
	if el := time.Since(start); el > 4*time.Second {
		t.Fatalf("took %s: the timed-out script's children weren't killed", el)
	}
}

func TestStopMatchesLastMessageAndNeverLoops(t *testing.T) {
	f := setup(t)
	f.write("rules.d/r.ini", "[handback]\nevent = stop\nwhen = (?i)can you run\ntext = do it yourself\n", 0o644)
	if _, ctx, _, _ := f.hook("Stop", f.project, `{"last_assistant_message":"Can you run this?"}`); ctx != "do it yourself" {
		t.Fatalf("ctx=%q", ctx)
	}
	if _, ctx, _, _ := f.hook("Stop", f.project, `{"last_assistant_message":"Can you run this?","stop_hook_active":true}`); ctx != "" {
		t.Fatalf("re-entrant stop must not inject: %q", ctx)
	}
}

func TestProjectLayer(t *testing.T) {
	f := setup(t)
	projLayer := filepath.Join(f.project, ".config", "lplug")
	f.write("rules.d/r.ini", "[greet]\nevent = session_start\nfile = greet.md\n[only-user]\nevent = session_start\ntext = user rule\n", 0o644)
	f.write("text/greet.md", "personal greeting", 0o644)
	f.write(filepath.Join(projLayer, "rules.d", "p.ini"), "[greet]\nevent = session_start\nfile = greet.md\n", 0o644)
	f.write(filepath.Join(projLayer, "text", "greet.md"), "project greeting", 0o644)
	f.write(filepath.Join(projLayer, "settings.ini"), "[lplug]\ntimeout = 3s\n", 0o644)
	f.write("settings.ini", "[lplug]\ntimeout = 9s\ndebug = true\n", 0o644)

	sub := filepath.Join(f.project, "src", "deep")
	os.MkdirAll(sub, 0o755)
	c := LoadConfig(sub)
	if len(c.Layers) != 2 || c.Timeout != 3*time.Second || !c.Debug {
		t.Fatalf("layers=%d timeout=%s debug=%v", len(c.Layers), c.Timeout, c.Debug)
	}
	t.Setenv("LPLUG_TIMEOUT", "1")
	if c := LoadConfig(sub); c.Timeout != time.Second || c.Origin["timeout"] != "env LPLUG_TIMEOUT" {
		t.Fatalf("env should win: %s from %s", c.Timeout, c.Origin["timeout"])
	}
	// The project's greet replaces the personal one in place; the project text dir is searched first.
	if _, ctx, _, _ := f.hook("SessionStart", sub, `{}`); ctx != "project greeting\n\nuser rule" {
		t.Fatalf("ctx=%q", ctx)
	}
	// Outside the project: only the personal layer.
	if _, ctx, _, _ := f.hook("SessionStart", f.home, `{}`); ctx != "personal greeting\n\nuser rule" {
		t.Fatalf("ctx=%q", ctx)
	}
}

func TestKnowledgeBases(t *testing.T) {
	f := setup(t)
	f.write(filepath.Join(f.home, "kb", "me", "index.md"), "# Me\nintro text\n- [Prefs](prefs.md)\n", 0o644)
	f.write(filepath.Join(f.home, "kb", "me", "AGENTS.md"), "rules", 0o644)
	f.write(filepath.Join(f.home, "scripts", "count-errors"), "#!/bin/sh\n# Purpose: count ERROR lines\n# Usage: count-errors DIR\n", 0o755)
	f.write("knowledge-bases.ini", "[defaults]\nedits = suggest\n"+
		"[me]\ndescription = Personal notes\npath = ~/kb/me\n"+
		"[scripts]\ndescription = Helper scripts\npath = ~/scripts\nkind = scripts\npreload = full\n", 0o644)
	f.write(filepath.Join(f.project, ".config", "lplug", "knowledge-bases.ini"),
		"[defaults]\nedits = none\n[me]\nenabled = false\n[proj]\ndescription = Project wiki\npath = wiki\n", 0o644)
	os.MkdirAll(filepath.Join(f.project, "wiki"), 0o755)

	home := renderKBs(LoadConfig(f.home))
	for _, want := range []string{"- me: Personal notes", "index index.md · rules AGENTS.md · edits: suggest",
		"    # Me", "    - [Prefs](prefs.md)", "- count-errors: count ERROR lines", "usage: count-errors DIR"} {
		if !strings.Contains(home, want) {
			t.Errorf("home render missing %q:\n%s", want, home)
		}
	}
	if strings.Contains(home, "intro text") {
		t.Error("outline should drop plain text")
	}
	proj := renderKBs(LoadConfig(f.project))
	if strings.Contains(proj, "- me:") || !strings.Contains(proj, "- proj (this project): Project wiki") || !strings.Contains(proj, "edits: none") {
		t.Errorf("project render:\n%s", proj)
	}
}

func TestSkillsSync(t *testing.T) {
	f := setup(t)
	f.write("skills.d/ponytail-simple/SKILL.md", "---\nname: ponytail-simple\n---\n", 0o644)
	f.write("skills.d/other/SKILL.md", "---\nname: other\n---\n", 0o644)
	target := filepath.Join(f.home, ".claude", "skills")
	f.write(filepath.Join(target, "mine", "SKILL.md"), "not lplug's", 0o644)
	c := LoadConfig(f.home)
	if linked, _, _, err := syncSkills(c, target); err != nil || strings.Join(linked, ",") != "other,ponytail-simple" {
		t.Fatalf("linked=%v err=%v", linked, err)
	}
	tg := loadToggles(c.StateDir)
	tg.set("other", false)
	tg.save()
	if _, removed, _, _ := syncSkills(c, target); strings.Join(removed, ",") != "other" {
		t.Fatalf("removed=%v", removed)
	}
	if !exists(filepath.Join(target, "mine", "SKILL.md")) {
		t.Fatal("a skill lplug doesn't manage was touched")
	}
}

func TestCommandsCompose(t *testing.T) {
	f := setup(t)
	f.write("commands.d/download-transactions.md", "---\ndescription: Download\n---\nDownload transactions for $1 into $2.\n", 0o644)
	f.write("commands.d/replay-transactions.md", "---\ndescription: Replay\nargument-hint: \"[month]\"\n---\n1. /download-transactions $1 \"out dir\"\n2. Replay them.\n3. /compact\n", 0o644)
	f.write("commands.d/nightly.md", "Every night:\n- /replay-transactions\n```\n/replay-transactions\n```\n", 0o644)
	sources, err := loadCommandSources(LoadConfig(f.home))
	if err != nil {
		t.Fatal(err)
	}

	text, calls, err := buildCommand("replay-transactions", sources)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"---\ndescription: Replay\nargument-hint: \"[month]\"\n---\n" + builtMarker,
		"1. **/download-transactions $1 \"out dir\"**:\n   Download transactions for $1 into out dir.\n2. Replay them.",
		"3. /compact", // not ours: left for the agent
	} {
		if !strings.Contains(text, want) {
			t.Errorf("replay missing %q in:\n%s", want, text)
		}
	}
	if strings.Join(calls, ",") != "download-transactions" {
		t.Errorf("calls=%v", calls)
	}

	text, calls, _ = buildCommand("nightly", sources)
	if strings.Join(calls, ",") != "replay-transactions,download-transactions" ||
		!strings.Contains(text, "- **/replay-transactions**:\n  1. **/download-transactions $1 \"out dir\"**:\n     Download") ||
		!strings.Contains(text, "```\n/replay-transactions\n```") {
		t.Errorf("nested build, calls=%v:\n%s", calls, text)
	}
}

func TestCommandsLoopAndSync(t *testing.T) {
	f := setup(t)
	f.write("commands.d/a.md", "1. /b\n", 0o644)
	f.write("commands.d/b.md", "1. /a\n", 0o644)
	f.write("commands.d/ok.md", "Just do it.\n", 0o644)
	f.write("commands.d/off.md", "Not today.\n", 0o644)
	f.write(filepath.Join(f.project, ".config", "lplug", "commands.d", "ok.md"), "Repo version.\n", 0o644)
	c := LoadConfig(f.project)
	if _, _, err := buildCommand("a", mustSources(t, c)); err == nil || !strings.Contains(err.Error(), "loop: /a -> /b -> /a") {
		t.Fatalf("want loop error, got %v", err)
	}

	target := filepath.Join(f.home, ".claude", "commands")
	f.write(filepath.Join(target, "mine.md"), "not lplug's", 0o644)
	f.write(filepath.Join(target, "off.md"), "not lplug's either", 0o644)
	tg := loadToggles(c.StateDir)
	tg.set("off", false)
	tg.save()
	written, _, skipped, failed, err := syncCommands(c, target)
	if err != nil || strings.Join(written, ",") != "ok" || len(skipped) != 0 || len(failed) != 2 {
		t.Fatalf("written=%v skipped=%v failed=%v err=%v", written, skipped, failed, err)
	}
	if raw, _ := os.ReadFile(filepath.Join(target, "ok.md")); !strings.Contains(string(raw), "Repo version.") {
		t.Errorf("project command should win:\n%s", raw)
	}
	for _, p := range []string{"mine.md", "off.md"} {
		if raw, _ := os.ReadFile(filepath.Join(target, p)); strings.Contains(string(raw), builtMarker) {
			t.Errorf("%s: a file lplug didn't build was touched", p)
		}
	}

	tg.set("ok", false)
	tg.save()
	if _, removed, _, _, _ := syncCommands(c, target); strings.Join(removed, ",") != "ok" || exists(filepath.Join(target, "ok.md")) {
		t.Fatalf("switched-off command not removed: %v", removed)
	}
}

func mustSources(t *testing.T, c *Config) map[string]*commandSource {
	t.Helper()
	s, err := loadCommandSources(c)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
