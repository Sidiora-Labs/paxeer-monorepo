package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// stageSourceHome copies the state and its actual config/wasm siblings into
// one disposable home. CopyFS refuses symlinks rather than letting a replay
// write through a copied link into the original home.
func stageSourceHome(source, destination string) error {
	data, err := resolveDataDir(source)
	if err != nil {
		return err
	}
	data, err = filepath.EvalSymlinks(data)
	if err != nil {
		return err
	}
	destination, err = filepath.Abs(destination)
	if err != nil {
		return err
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(destination))
	if err != nil {
		return err
	}
	destination = filepath.Join(parent, filepath.Base(destination))
	if _, err := os.Lstat(destination); !os.IsNotExist(err) {
		return fmt.Errorf("staging destination must not exist: %s", destination)
	}
	sources := map[string]string{"data": data}
	for _, name := range []string{"config", "wasm"} {
		sibling := filepath.Join(filepath.Dir(data), name)
		info, err := os.Stat(sibling)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return fmt.Errorf("source sibling is not a directory: %s", sibling)
		}
		sibling, err = filepath.EvalSymlinks(sibling)
		if err != nil {
			return err
		}
		sources[name] = sibling
	}
	for _, src := range sources {
		if withinDirectory(src, destination) || withinDirectory(destination, src) {
			return fmt.Errorf("staging destination %s overlaps source %s", destination, src)
		}
	}
	temporary, err := os.MkdirTemp(parent, ".upgrade-replay-source-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temporary)
	for _, name := range []string{"data", "config", "wasm"} {
		if src, ok := sources[name]; ok {
			if err := os.CopyFS(filepath.Join(temporary, name), os.DirFS(src)); err != nil {
				return fmt.Errorf("staging %s: %w", name, err)
			}
		}
	}
	return os.Rename(temporary, destination)
}

func withinDirectory(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	return err == nil && rel != ".." && !filepath.IsAbs(rel) && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
