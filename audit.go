package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"nod/adapters"
)

type auditRecord struct {
	TS        string         `json:"ts"`
	Session   string         `json:"session,omitempty"`
	ToolName  string         `json:"tool_name"`
	ToolInput map[string]any `json:"tool_input"`
	Decision  string         `json:"decision"`
	Ruleset   string         `json:"ruleset,omitempty"`
	Rule      string         `json:"rule,omitempty"`
	Reason    string         `json:"reason"`
}

func appendAudit(path string, req adapters.Request, o adapters.Outcome) {
	b, err := json.Marshal(auditRecord{
		TS:        time.Now().UTC().Format(time.RFC3339),
		Session:   req.Session,
		ToolName:  req.Tool,
		ToolInput: req.ToolInput,
		Decision:  adapters.DecisionLabel(o),
		Ruleset:   o.Ruleset,
		Rule:      o.Rule,
		Reason:    o.ReasonText(),
	})
	if err != nil {
		return
	}
	if dir := filepath.Dir(path); dir != "" {
		os.MkdirAll(dir, 0o755)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	f.Write(append(b, '\n'))
}
