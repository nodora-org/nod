package governor

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"nodora.org/nodora/pkg/compiler"
	"nodora.org/nodora/pkg/nir"
)

const cacheVersion = "1"

type compiledFile struct {
	File    string       `json:"file"`
	Ruleset *nir.Ruleset `json:"ruleset,omitempty"`
	Err     string       `json:"err,omitempty"`
}

type layerCache struct {
	Version string         `json:"version"`
	Sig     string         `json:"sig"`
	Files   []compiledFile `json:"files"`
}

func compileLayer(dir string) ([]compiledFile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	type srcFile struct {
		name string
		size int64
		mod  int64
	}
	var srcs []srcFile
	statOK := true
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".ruleset" {
			continue
		}
		info, err := e.Info()
		if err != nil {
			// can't build a trustworthy signature; still compile, just don't cache
			statOK = false
			srcs = append(srcs, srcFile{name: e.Name()})
			continue
		}
		srcs = append(srcs, srcFile{e.Name(), info.Size(), info.ModTime().UnixNano()})
	}
	sort.Slice(srcs, func(i, j int) bool { return srcs[i].name < srcs[j].name })

	// build the signature and locate the layer's cache entry
	h := sha256.New()
	h.Write([]byte(cacheVersion))
	for _, s := range srcs {
		fmt.Fprintf(h, "\x00%s\x00%d\x00%d", s.name, s.size, s.mod)
	}
	sig := hex.EncodeToString(h.Sum(nil))

	cacheRoot, cacheable := cacheDir()
	cacheable = cacheable && statOK
	var cachePath string
	if cacheable {
		cachePath = filepath.Join(cacheRoot, "layer-"+dirKey(dir)+".json")
		if data, err := os.ReadFile(cachePath); err == nil {
			var lc layerCache
			if json.Unmarshal(data, &lc) == nil && lc.Version == cacheVersion && lc.Sig == sig {
				return lc.Files, nil
			}
			// stale or corrupt: fall through and recompile
		}
	}

	files := make([]compiledFile, 0, len(srcs))
	for _, s := range srcs {
		cf := compiledFile{File: s.name}
		switch src, err := os.ReadFile(filepath.Join(dir, s.name)); {
		case err != nil:
			cf.Err = err.Error()
		default:
			if rs, err := compiler.NewCompiler().Compile(string(src)); err != nil {
				cf.Err = "compile error: " + err.Error()
			} else {
				cf.Ruleset = rs
			}
		}
		files = append(files, cf)
	}

	if cacheable {
		writeLayerCache(cacheRoot, cachePath, layerCache{Version: cacheVersion, Sig: sig, Files: files})
	}
	return files, nil
}

func dirKey(dir string) string {
	abs, err := filepath.Abs(dir)
	if err != nil {
		abs = dir
	}
	sum := sha256.Sum256([]byte(abs))
	return hex.EncodeToString(sum[:])
}

func writeLayerCache(root, path string, lc layerCache) {
	data, err := json.Marshal(lc)
	if err != nil {
		return
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return
	}
	tmp, err := os.CreateTemp(root, "tmp-*.json")
	if err != nil {
		return
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
	}
}

func cacheDir() (string, bool) {
	if h := os.Getenv("NOD_HOME"); h != "" {
		return filepath.Join(h, "cache"), true
	}
	if h, err := os.UserHomeDir(); err == nil {
		return filepath.Join(h, ".nod", "cache"), true
	}
	return "", false
}
