package main

import (
	"fmt"
	"os"

	"nodora.org/nod/governor"
)

func runDecide(c common) error {
	localDir := resolveLocal(c.localDir, "")
	g, err := governor.LoadLayers(buildLayers(c, localDir), c.def)
	if err != nil {
		return err
	}
	printLoadErrs(g)

	input := readInput(c.input)
	d := g.Evaluate(input)

	if c.quiet {
		fmt.Println(d.Verdict)
	} else {
		fmt.Printf("input:   %s\n", c.input)
		fmt.Printf("loaded:  %d ruleset(s) (default: %s)\n\n", g.Count(), c.def)
		printDecision(d)
	}

	if c.failSet && d.Verdict >= c.failOn {
		os.Exit(1)
	}
	return nil
}
