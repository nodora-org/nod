package main

import (
	"os"
	"path/filepath"

	"nod/governor"
)

func buildLayers(c common, localDir string) []governor.Layer {
	layers := []governor.Layer{{Dir: c.dir, Label: "global"}}
	if localDir != "" {
		layers = append(layers, governor.Layer{
			Dir: localDir, Label: "local", TightenOnly: !c.trust,
		})
	}
	return layers
}

func resolveLocal(explicit, start string) string {
	if explicit != "" {
		return explicit
	}
	return discoverLocal(start)
}

func discoverLocal(start string) string {
	if start == "" {
		start, _ = os.Getwd()
	}
	for dir := start; dir != ""; {
		if p := filepath.Join(dir, ".nod"); isDir(p) {
			return p
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return ""
}

func isDir(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}
