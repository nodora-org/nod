package adapters

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"

	"nodora.org/nodora/pkg/core"
)

func init() { register("claude", claudeAdapter{}) }

type claudeAdapter struct{}

type claudePreToolUse struct {
	SessionID string         `json:"session_id"`
	CWD       string         `json:"cwd"`
	ToolName  string         `json:"tool_name"`
	ToolInput map[string]any `json:"tool_input"`
}

func (claudeAdapter) Parse(payload []byte) (Request, error) {
	var ev claudePreToolUse
	if err := json.Unmarshal(payload, &ev); err != nil {
		return Request{}, err
	}
	start := os.Getenv("CLAUDE_PROJECT_DIR")
	if start == "" {
		start = ev.CWD
	}
	return Request{
		Input:     normalizeClaudeToolCall(ev.ToolName, ev.ToolInput),
		Start:     start,
		Tool:      ev.ToolName,
		ToolInput: ev.ToolInput,
		Session:   ev.SessionID,
	}, nil
}

func (claudeAdapter) Emit(o Outcome) {
	out := map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":            "PreToolUse",
			"permissionDecision":       DecisionLabel(o),
			"permissionDecisionReason": formatReason(o.Ruleset, o.Rule, o.ReasonText()),
		},
	}
	b, _ := json.Marshal(out)
	fmt.Println(string(b))
}

func normalizeClaudeToolCall(name string, in map[string]any) core.ValueMap {
	get := func(k string) any {
		if in == nil {
			return nil
		}
		return in[k]
	}
	raw := map[string]any{"tool_name": name, "tool_input": in}

	switch strings.ToLower(name) {
	case "bash":
		raw["tool"], raw["cmd"] = "bash", get("command")
	case "write":
		raw["tool"], raw["path"], raw["content"] = "write", get("file_path"), get("content")
	case "edit", "multiedit":
		raw["tool"], raw["path"] = "edit", get("file_path")
	case "notebookedit":
		raw["tool"], raw["path"] = "edit", get("notebook_path")
	case "read":
		raw["tool"], raw["path"] = "read", get("file_path")
	case "grep":
		raw["tool"], raw["path"] = "grep", get("path")
	case "glob":
		raw["tool"], raw["path"] = "glob", get("path")
	case "webfetch":
		raw["tool"], raw["url"] = "webfetch", get("url")
		if u, ok := get("url").(string); ok {
			if parsed, err := url.Parse(u); err == nil {
				raw["host"] = parsed.Hostname()
			}
		}
	case "websearch":
		raw["tool"] = "websearch"
	default:
		raw["tool"] = strings.ToLower(name)
	}

	vm, _ := core.NewValueMap(raw)
	return vm
}
