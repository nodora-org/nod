package adapters

import (
	"fmt"
	"sort"
	"strings"

	"nod/governor"

	"nodora.org/nodora/pkg/core"
)

type Adapter interface {
	Parse(payload []byte) (Request, error)
	Emit(o Outcome)
}

type Request struct {
	Input core.ValueMap
	Start string

	// audit metadata
	Tool      string
	ToolInput map[string]any
	Session   string
}

type Outcome struct {
	Verdict  governor.Verdict
	Explicit bool // false = no rule matched; nod defers to the agent's own flow
	Ruleset  string
	Rule     string
	Reason   string
}

func (o Outcome) ReasonText() string {
	if o.Reason != "" {
		return o.Reason
	}
	return "no policy matched this tool call"
}

// nod's word for an outcome: allow | ask | deny | defer
func DecisionLabel(o Outcome) string {
	if !o.Explicit {
		return "defer"
	}
	return verdictWord(o.Verdict)
}

func verdictWord(v governor.Verdict) string {
	switch v {
	case governor.Allow:
		return "allow"
	case governor.Ask:
		return "ask"
	case governor.Deny:
		return "deny"
	}
	return "?"
}

func formatReason(ruleset, rule, reason string) string {
	if ruleset == "" {
		return reason
	}
	src := ruleset
	if rule != "" {
		src += ":" + rule
	}
	return src + ": " + reason
}

var registry = map[string]Adapter{}

func register(name string, a Adapter) { registry[name] = a }

func Select(name string) (Adapter, error) {
	if a, ok := registry[name]; ok {
		return a, nil
	}
	return nil, fmt.Errorf("unknown --adapter %q (available: %s)", name, Names())
}

func Names() string {
	names := make([]string, 0, len(registry))
	for n := range registry {
		names = append(names, n)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}
