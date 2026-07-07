package main

import (
	"fmt"

	"nodora.org/nod/governor"
)

func runCheck(dir, localDir string) error {
	localDir = resolveLocal(localDir, "")
	results, err := governor.CheckLayers(buildLayers(common{dir: dir}, localDir))
	if err != nil {
		return err
	}
	if len(results) == 0 {
		fmt.Printf("no rulesets found in %s\n", dir)
		return nil
	}

	failed := 0
	for _, r := range results {
		if r.Err != nil {
			failed++
			fmt.Printf("✗ %-12s %-34s %v\n", r.Layer, r.File, r.Err)
		} else {
			fmt.Printf("✓ %-12s %-34s %d rule(s)\n", r.Layer, r.File, len(r.Rules))
		}
	}

	fmt.Printf("\n%d ruleset(s) checked, %d failed\n", len(results), failed)
	if failed > 0 {
		return fmt.Errorf("%d ruleset(s) failed to compile", failed)
	}
	return nil
}
