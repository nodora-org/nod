package main

import (
	"encoding/json"
	"fmt"
	"os"

	"nodora.org/nod/governor"

	"nodora.org/nodora/pkg/core"
)

func readInput(path string) core.ValueMap {
	b, err := os.ReadFile(path)
	if err != nil {
		fatal("read %s: %v", path, err)
	}
	var m core.ValueMap
	if err := json.Unmarshal(b, &m); err != nil {
		fatal("parse %s: %v", path, err)
	}
	if m == nil {
		m = core.ValueMap{}
	}
	return m
}

func printLoadErrs(g *governor.Governor) {
	for _, e := range g.LoadErrs {
		fmt.Fprintf(os.Stderr, "warning: %s (skipped)\n", e)
	}
}

func fatal(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", a...)
	os.Exit(1)
}
