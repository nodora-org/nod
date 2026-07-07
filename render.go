package main

import (
	"fmt"

	"nodora.org/nod/governor"
)

const provFormat = "%-16s %-14s %-8s %-32s %s\n"

func printDecision(d governor.Decision) {
	fmt.Printf("VERDICT: %s\n\n", d.Verdict)
	if len(d.Contributions) == 0 {
		fmt.Println("  (no signals emitted, decision is the default)")
		return
	}

	fmt.Printf(provFormat, "SIGNAL", "VERDICT", "SCOPE", "SOURCE", "REASON")
	for _, c := range d.Contributions {
		fmt.Printf(provFormat, signalOf(c), verdictOf(c), c.Layer, source(c), c.Reason)
	}
}

func signalOf(c governor.Contribution) string {
	if c.Signal == "" {
		return "—"
	}
	return c.Signal
}

func verdictOf(c governor.Contribution) string {
	switch c.Kind {
	case "verdict":
		return c.Verdict.String()
	case "error":
		return c.Verdict.String() + " (error)"
	case "log":
		return "—"
	default:
		return "(" + c.Kind + ")"
	}
}

func source(c governor.Contribution) string {
	if c.Rule == "" {
		return c.Ruleset
	}
	return c.Ruleset + ":" + c.Rule
}
