package lplug

import (
	"encoding/json"
	"strings"
)

// Neutral event names. Rules, env vars and directories use these; each
// adapter maps its agent's native names onto them (`lplug help events`).
const (
	SessionStart  = "session_start"
	PromptSubmit  = "prompt_submit"
	ToolBefore    = "tool_before"
	ToolAfter     = "tool_after"
	ToolFailed    = "tool_failed"
	SubagentStart = "subagent_start"
	Notification  = "notification"
	Stop          = "stop"
	SubagentStop  = "subagent_stop"
	CompactBefore = "compact_before"
	SessionEnd    = "session_end"
)

// AllEvents in lifecycle order.
var AllEvents = []string{
	SessionStart, PromptSubmit, ToolBefore, ToolAfter, ToolFailed, SubagentStart,
	Notification, Stop, SubagentStop, CompactBefore, SessionEnd,
}

// Event is one hook invocation, normalized.
type Event struct {
	Agent          string          `json:"agent"`
	Name           string          `json:"event"`
	Native         string          `json:"native_event"`
	SessionID      string          `json:"session_id,omitempty"`
	ToolName       string          `json:"tool_name,omitempty"`
	ToolUseID      string          `json:"tool_use_id,omitempty"`
	Subject        string          `json:"subject,omitempty"` // what `when` / `unless` match
	Cwd            string          `json:"cwd,omitempty"`
	TranscriptPath string          `json:"transcript_path,omitempty"`
	Reentrant      bool            `json:"reentrant,omitempty"` // a stop already continued by a hook
	Raw            json.RawMessage `json:"raw,omitempty"`       // the agent's original payload
}

// Is matches a neutral or native name, ignoring case, '_' and '-'.
func (e Event) Is(name string) bool {
	k := eventKey(name)
	return k == eventKey(e.Name) || k == eventKey(e.Native)
}

func eventKey(s string) string {
	return strings.NewReplacer("_", "", "-", "", " ", "").Replace(strings.ToLower(s))
}

// eventEnv: LPLUG_TOOL_BEFORE. eventDir: tool-before.d.
func eventEnv(ev string) string { return "LPLUG_" + strings.ToUpper(ev) }
func eventDir(ev string) string { return strings.ReplaceAll(ev, "_", "-") + ".d" }

// Result is what all sources together decided for one event.
type Result struct {
	Context  []string // text for the agent's context
	Confirms []string // ask the user before going ahead (tool_before)
	Reasons  []string // why it was blocked
	Blocked  bool
}

func (r *Result) AddContext(s string) {
	if s = strings.TrimRight(s, "\r\n"); strings.TrimSpace(s) != "" {
		r.Context = append(r.Context, s)
	}
}

func (r *Result) Confirm(reason string) {
	if reason = strings.TrimSpace(reason); reason != "" {
		r.Confirms = append(r.Confirms, reason)
	}
}

// Block: any one source blocking blocks the event.
func (r *Result) Block(reason string) {
	r.Blocked = true
	if reason = strings.TrimSpace(reason); reason != "" {
		r.Reasons = append(r.Reasons, reason)
	}
}

func (r *Result) Merge(o Result) {
	r.Context = append(r.Context, o.Context...)
	r.Confirms = append(r.Confirms, o.Confirms...)
	r.Reasons = append(r.Reasons, o.Reasons...)
	r.Blocked = r.Blocked || o.Blocked
}
