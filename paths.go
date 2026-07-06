package main

import (
	"os"
	"path/filepath"
)

func nodHome() string {
	if h := os.Getenv("NOD_HOME"); h != "" {
		return h
	}
	if h, err := os.UserHomeDir(); err == nil {
		return filepath.Join(h, ".nod")
	}
	return ".nod" // fallback
}

func defaultGlobalDir() string { return filepath.Join(nodHome(), "rulesets") }

func defaultAuditLog() string { return filepath.Join(nodHome(), "audit.jsonl") }
