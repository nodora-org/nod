package governor

import (
	"errors"
	"fmt"
	"os"
	"sort"

	"nodora.org/nodora/pkg/core"
	"nodora.org/nodora/pkg/evaluator"
	_ "nodora.org/nodora/pkg/registry/all"
)

// verdict is the combined outcome
// a higher value is more restrictive and wins during combination (deny-wins)
type Verdict int

const (
	Allow Verdict = iota // least restrictive
	Ask                  // ask the user first
	Deny                 // most restrictive
)

func (v Verdict) String() string {
	switch v {
	case Allow:
		return "ALLOW"
	case Ask:
		return "ASK"
	case Deny:
		return "DENY"
	default:
		return "?"
	}
}

var verdictSignals = map[string]Verdict{
	"Allow": Allow,
	"Ask":   Ask,
	"Deny":  Deny,
}

type Contribution struct {
	Layer   string
	Ruleset string
	Rule    string
	Signal  string
	Reason  string
	Verdict Verdict // set when CarriesVerdict() — Kind "verdict" or "error"
	Kind    string  // "verdict" | "error" | "log" | "unhandled" | "suppressed"
}

func (c Contribution) CarriesVerdict() bool {
	return c.Kind == "verdict" || c.Kind == "error"
}

type Decision struct {
	Verdict       Verdict
	Contributions []Contribution
}

type Layer struct {
	Dir         string
	Label       string // shown in provenance, e.g. "global" or "local"
	TightenOnly bool   // marks a layer as lower-trust
}

type policy struct {
	layer       string
	tightenOnly bool
	file        string
	ev          *evaluator.Evaluator
	rules       []string
}

type Governor struct {
	Default  Verdict
	LoadErrs []string // compile/read failures
	policies []policy
}

func Load(dir string, def Verdict) (*Governor, error) {
	return LoadLayers([]Layer{{Dir: dir, Label: "global"}}, def)
}

func LoadLayers(layers []Layer, def Verdict) (*Governor, error) {
	g := &Governor{Default: def}
	for _, ly := range layers {
		files, err := compileLayer(ly.Dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}

		for _, cf := range files {
			if cf.Err != "" {
				g.LoadErrs = append(g.LoadErrs, fmt.Sprintf("%s/%s: %s", ly.Label, cf.File, cf.Err))
				continue
			}
			ev := evaluator.NewEvaluator(cf.Ruleset)
			rules := ev.GetRuleNames()
			sort.Strings(rules) // deterministic within a file too
			g.policies = append(g.policies, policy{
				layer: ly.Label, tightenOnly: ly.TightenOnly,
				file: cf.File, ev: ev, rules: rules,
			})
		}
	}
	return g, nil
}

type CheckResult struct {
	Layer string
	File  string
	Rules []string
	Err   error
}

func CheckLayers(layers []Layer) ([]CheckResult, error) {
	var out []CheckResult
	for _, ly := range layers {
		files, err := compileLayer(ly.Dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		for _, cf := range files {
			r := CheckResult{Layer: ly.Label, File: cf.File}
			if cf.Err != "" {
				r.Err = errors.New(cf.Err)
			} else {
				r.Rules = evaluator.NewEvaluator(cf.Ruleset).GetRuleNames()
				sort.Strings(r.Rules)
			}
			out = append(out, r)
		}
	}
	return out, nil
}

func (g *Governor) Count() int { return len(g.policies) }

func (g *Governor) Evaluate(input core.ValueMap) Decision {
	d := Decision{Verdict: g.Default}
	spoke := false
	worst := Allow

	for _, p := range g.policies {
		for _, rn := range p.rules {
			res, err := p.ev.EvaluateRule(rn, input)
			if err != nil {
				// an evaluator that blows up is treated as the most restrictive
				// possible answer rather than silently ignored — it is an error,
				// not a signal, so it carries no Signal name
				d.Contributions = append(d.Contributions, Contribution{
					Layer: p.layer, Ruleset: p.file, Rule: rn,
					Reason: err.Error(), Verdict: Deny, Kind: "error",
				})
				spoke, worst = true, Deny
				continue
			}
			for _, sig := range res.Signals {
				c := Contribution{Layer: p.layer, Ruleset: p.file, Rule: rn, Signal: sig.Name, Reason: firstArg(sig.Args)}
				v, verdict := verdictSignals[sig.Name]
				switch {
				case sig.Name == "Log":
					c.Kind = "log"
				case verdict && p.tightenOnly && v == Allow:
					// a lower-trust layer may only tighten, never grant
					c.Kind = "suppressed"
					c.Verdict = Allow
				case verdict:
					c.Kind = "verdict"
					c.Verdict = v
					spoke = true
					if v > worst {
						worst = v
					}
				default:
					c.Kind = "unhandled"
				}
				d.Contributions = append(d.Contributions, c)
			}
		}
	}

	if spoke {
		d.Verdict = worst
	}
	return d
}

func (d Decision) Explicit() (Verdict, bool) {
	found := false
	worst := Allow
	for _, c := range d.Contributions {
		if c.CarriesVerdict() {
			found = true
			if c.Verdict > worst {
				worst = c.Verdict
			}
		}
	}
	return worst, found
}

func firstArg(args []any) string {
	if len(args) == 0 {
		return ""
	}
	return fmt.Sprint(args[0])
}
