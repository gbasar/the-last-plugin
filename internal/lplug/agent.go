package lplug

// Adapters translate between lplug's neutral model and each agent's native
// hook protocol. Adding an agent = one more type implementing Adapter,
// registered in `adapters`.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

type Adapter interface {
	Name() string                                   // id for --agent / LPLUG_AGENT
	Events() map[string]string                      // native -> neutral
	Parse(native string, raw []byte) (Event, error) // native payload -> Event
	NativeOutput(stdout []byte) (Result, bool)      // a script's output in the agent's own format
	CanInject(ev Event) bool                        // can context reach the model on this event?
	Emit(ev Event, r Result, stdout, stderr io.Writer) int
	SkillsDir() string   // where the agent discovers skills ("" = none)
	CommandsDir() string // where the agent discovers slash commands ("" = none)
}

const defaultAgent = "claude-code"

var adapters = map[string]Adapter{"claude-code": claudeCode{}}

func agentNames() []string {
	var names []string
	for n := range adapters {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// neutralName maps a native event; unmapped events keep a snake_case name.
func neutralName(a Adapter, native string) string {
	if n, ok := a.Events()[native]; ok {
		return n
	}
	var b strings.Builder
	for i, r := range native {
		if unicode.IsUpper(r) && i > 0 {
			b.WriteByte('_')
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

// ---- Claude Code: JSON on stdin; exit 2 + stderr blocks;
// hookSpecificOutput carries additionalContext and permission decisions.

type claudeCode struct{}

var ccEvents = map[string]string{
	"SessionStart": SessionStart, "SessionEnd": SessionEnd,
	"UserPromptSubmit": PromptSubmit, "PreToolUse": ToolBefore,
	"PostToolUse": ToolAfter, "PostToolUseFailure": ToolFailed,
	"SubagentStart": SubagentStart, "Notification": Notification,
	"Stop": Stop, "SubagentStop": SubagentStop, "PreCompact": CompactBefore,
}

// Events where additionalContext reaches the model. On Stop/SubagentStop
// it makes the conversation continue.
var ccInject = map[string]bool{
	"SessionStart": true, "UserPromptSubmit": true, "PreToolUse": true,
	"PostToolUse": true, "SubagentStart": true, "Stop": true, "SubagentStop": true,
}

func (claudeCode) Name() string              { return "claude-code" }
func (claudeCode) Events() map[string]string { return ccEvents }

func (claudeCode) SkillsDir() string   { return filepath.Join(claudeConfigDir(), "skills") }
func (claudeCode) CommandsDir() string { return filepath.Join(claudeConfigDir(), "commands") }

func claudeConfigDir() string {
	if dir := os.Getenv("CLAUDE_CONFIG_DIR"); dir != "" {
		return dir
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude")
}

func (c claudeCode) Parse(native string, raw []byte) (Event, error) {
	ev := Event{Agent: c.Name(), Name: neutralName(c, native), Native: native}
	if raw = bytes.TrimSpace(raw); len(raw) == 0 {
		return ev, nil
	}
	ev.Raw = raw
	var in struct {
		SessionID      string          `json:"session_id"`
		Cwd            string          `json:"cwd"`
		TranscriptPath string          `json:"transcript_path"`
		ToolName       string          `json:"tool_name"`
		ToolUseID      string          `json:"tool_use_id"`
		ToolInput      json.RawMessage `json:"tool_input"`
		Prompt         string          `json:"prompt"`
		Message        string          `json:"message"`
		LastMessage    string          `json:"last_assistant_message"`
		StopHookActive bool            `json:"stop_hook_active"`
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		return ev, fmt.Errorf("parse %s payload: %w", native, err)
	}
	ev.SessionID, ev.Cwd, ev.TranscriptPath = in.SessionID, in.Cwd, in.TranscriptPath
	ev.ToolName, ev.ToolUseID, ev.Reentrant = in.ToolName, in.ToolUseID, in.StopHookActive
	switch {
	case in.Prompt != "":
		ev.Subject = in.Prompt
	case in.LastMessage != "": // stop events: match what the agent just said
		ev.Subject = in.LastMessage
	case len(in.ToolInput) > 0: // shell tools: the command; others: the input JSON
		var cmd struct {
			Command string `json:"command"`
		}
		var b bytes.Buffer
		if json.Unmarshal(in.ToolInput, &cmd) == nil && cmd.Command != "" {
			ev.Subject = cmd.Command
		} else if json.Compact(&b, in.ToolInput) == nil {
			ev.Subject = b.String()
		}
	default:
		ev.Subject = in.Message
	}
	return ev, nil
}

func (claudeCode) NativeOutput(stdout []byte) (Result, bool) {
	var r Result
	var o struct {
		Decision string `json:"decision"`
		Reason   string `json:"reason"`
		Specific *struct {
			AdditionalContext string `json:"additionalContext"`
			Permission        string `json:"permissionDecision"`
			PermissionReason  string `json:"permissionDecisionReason"`
		} `json:"hookSpecificOutput"`
	}
	stdout = bytes.TrimSpace(stdout)
	if len(stdout) == 0 || stdout[0] != '{' || json.Unmarshal(stdout, &o) != nil ||
		(o.Decision == "" && o.Specific == nil) {
		return r, false
	}
	if o.Decision == "block" {
		r.Block(o.Reason)
	}
	if s := o.Specific; s != nil {
		switch s.Permission {
		case "deny":
			r.Block(s.PermissionReason)
		case "ask":
			r.Confirm(s.PermissionReason)
		}
		r.AddContext(s.AdditionalContext)
	}
	return r, true
}

func (claudeCode) CanInject(ev Event) bool { return ccInject[ev.Native] || ev.Native == "PreCompact" }

func (claudeCode) Emit(ev Event, r Result, stdout, stderr io.Writer) int {
	// Only PreToolUse can ask the user; anywhere else a confirm blocks, to be safe.
	if len(r.Confirms) > 0 && ev.Native != "PreToolUse" {
		r.Blocked, r.Reasons = true, append(r.Reasons, r.Confirms...)
	}
	if r.Blocked {
		reason := strings.Join(r.Reasons, "\n")
		if reason == "" {
			reason = "blocked by lplug"
		}
		fmt.Fprintln(stderr, reason)
		return 2
	}
	out := map[string]string{"hookEventName": ev.Native}
	if ctx := strings.Join(r.Context, "\n\n"); ctx != "" {
		if ev.Native == "PreCompact" { // plain stdout becomes compaction instructions
			fmt.Fprintln(stdout, ctx)
			return 0
		}
		if ccInject[ev.Native] {
			out["additionalContext"] = ctx
		}
	}
	if len(r.Confirms) > 0 {
		out["permissionDecision"] = "ask"
		out["permissionDecisionReason"] = strings.Join(r.Confirms, "\n")
	}
	if len(out) > 1 {
		json.NewEncoder(stdout).Encode(map[string]any{"hookSpecificOutput": out})
	}
	return 0
}
