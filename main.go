package main

import (
	"context"
	"fmt"
	"os"

	"github.com/urfave/cli/v3"

	"nodora.org/nod/adapters"
	"nodora.org/nod/governor"
)

func main() {
	cmd := &cli.Command{
		Name:  "nod",
		Usage: "an extensible governor (allow/ask/deny) with agent hook adapters",
		Commands: []*cli.Command{
			{
				Name:      "decide",
				Usage:     "decide a single input against the policy and print the verdict + provenance",
				ArgsUsage: " ",
				Flags: append(policyFlags(),
					&cli.StringFlag{Name: "input", Aliases: []string{"i"}, Required: true, Usage: "JSON input file"},
					&cli.BoolFlag{Name: "quiet", Aliases: []string{"q"}, Usage: "print only the verdict (ALLOW/ASK/DENY)"},
					&cli.StringFlag{Name: "fail-on", Usage: "exit 1 when the verdict is this strict or stricter: ask|deny"},
				),
				Action: func(_ context.Context, cmd *cli.Command) error {
					c, err := policyCommon(cmd)
					if err != nil {
						return err
					}
					c.input = cmd.String("input")
					c.quiet = cmd.Bool("quiet")
					if fo := cmd.String("fail-on"); fo != "" {
						v, err := parseVerdict(fo)
						if err != nil {
							return fmt.Errorf("--fail-on %v", err)
						}
						c.failOn, c.failSet = v, true
					}
					return runDecide(c)
				},
			},
			{
				Name:  "check",
				Usage: "compile every ruleset and report errors (exit non-zero if any fail)",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "dir", Aliases: []string{"d"}, Value: defaultGlobalDir(), Usage: "global policy directory to check"},
					&cli.StringFlag{Name: "local", Usage: "also check a local .nod/ directory"},
				},
				Action: func(_ context.Context, cmd *cli.Command) error {
					return runCheck(cmd.String("dir"), cmd.String("local"))
				},
			},
			{
				Name:  "hook",
				Usage: "run as an agent hook: stdin=tool-call JSON, stdout=decision",
				Flags: append(policyFlags(),
					&cli.StringFlag{Name: "adapter", Value: "claude", Usage: "agent hook protocol adapter (available: " + adapters.Names() + ")"},
					&cli.StringFlag{Name: "log", Value: defaultAuditLog(), Usage: "audit log path"},
					&cli.BoolFlag{Name: "no-log", Usage: "disable the audit log"},
				),
				Action: func(_ context.Context, cmd *cli.Command) error {
					c, err := policyCommon(cmd)
					if err != nil {
						return err
					}
					a, err := adapters.Select(cmd.String("adapter"))
					if err != nil {
						return err
					}
					c.logf = cmd.String("log")
					if cmd.Bool("no-log") {
						c.logf = ""
					}
					runHook(c, a)
					return nil
				},
			},
		},
	}
	if err := cmd.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

type common struct {
	dir      string
	localDir string // extra tighten-only layer (.nod/), if set explicitly
	trust    bool   // treat the local layer as fully trusted (honor its Allow verdicts)
	def      governor.Verdict
	input    string
	quiet    bool             // decide: print only the verdict, no provenance
	failOn   governor.Verdict // decide: exit 1 when Verdict >= failOn (if failSet)
	failSet  bool
	logf     string
}

func policyFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{Name: "dir", Aliases: []string{"d"}, Value: defaultGlobalDir(), Usage: "global policy directory"},
		&cli.StringFlag{Name: "local", Aliases: []string{"l"}, Usage: "a local .nod/ directory of tighten-only rules"},
		&cli.BoolFlag{Name: "trust", Usage: "trust allow verdicts from the local .nod/ rules"},
		&cli.StringFlag{Name: "default", Value: "deny", Usage: "verdict when no rule matches: deny|ask|allow"},
	}
}

func policyCommon(cmd *cli.Command) (common, error) {
	c := common{
		dir:      cmd.String("dir"),
		localDir: cmd.String("local"),
		trust:    cmd.Bool("trust"),
	}
	def, err := parseVerdict(cmd.String("default"))
	if err != nil {
		return c, fmt.Errorf("--default %v", err)
	}
	c.def = def
	return c, nil
}

func parseVerdict(s string) (governor.Verdict, error) {
	switch s {
	case "deny":
		return governor.Deny, nil
	case "ask":
		return governor.Ask, nil
	case "allow":
		return governor.Allow, nil
	}
	return 0, fmt.Errorf("must be deny|ask|allow, got %q", s)
}
