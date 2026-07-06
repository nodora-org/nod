package main

import (
	"io"
	"os"

	"nod/adapters"
	"nod/governor"

	"nodora.org/nodora/pkg/core"
)

func runHook(c common, a adapters.Adapter) {
	payload, err := io.ReadAll(os.Stdin)
	if err != nil {
		a.Emit(deferOutcome("nod: could not read hook payload: " + err.Error()))
		return
	}
	req, err := a.Parse(payload)
	if err != nil {
		a.Emit(deferOutcome("nod: could not parse hook payload: " + err.Error()))
		return
	}

	localDir := resolveLocal(c.localDir, req.Start)
	g, err := governor.LoadLayers(buildLayers(c, localDir), c.def)
	if err != nil {
		a.Emit(deferOutcome("nod: governor load error: " + err.Error()))
		return
	}

	o := evaluate(g, req.Input)
	if c.logf != "" {
		appendAudit(c.logf, req, o)
	}
	a.Emit(o)
}

func deferOutcome(msg string) adapters.Outcome {
	return adapters.Outcome{Reason: msg}
}

func evaluate(g *governor.Governor, input core.ValueMap) adapters.Outcome {
	d := g.Evaluate(input)
	o := adapters.Outcome{}
	if v, explicit := d.Explicit(); explicit {
		o.Explicit, o.Verdict = true, v
		if w, ok := winning(d, v); ok {
			o.Ruleset, o.Rule, o.Reason = w.Ruleset, w.Rule, w.Reason
			if o.Reason == "" {
				o.Reason = w.Signal
			}
		}
	}
	return o
}

func winning(d governor.Decision, v governor.Verdict) (governor.Contribution, bool) {
	for _, c := range d.Contributions {
		if c.CarriesVerdict() && c.Verdict == v {
			return c, true
		}
	}
	return governor.Contribution{}, false
}
