package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ethereum/go-ethereum/common"
)

var artifactNames = []string{"export.json", "fetch.json", "heights.json"}

type publication struct {
	paths       []string
	current     string
	generations string
}

type priorArtifact struct {
	Present bool        `json:"present"`
	Digest  common.Hash `json:"digest"`
}

type generationManifest struct {
	Version   int                      `json:"version"`
	Digests   map[string]common.Hash   `json:"digests"`
	Bootstrap map[string]priorArtifact `json:"bootstrap,omitempty"`
}

type stagedPublication struct {
	publication *publication
	directory   string
	manifest    generationManifest
}

func newPublication(export string) *publication {
	dir := filepath.Dir(export)
	base := filepath.Base(export)
	return &publication{
		paths:       []string{export, strings.TrimSuffix(export, ".json") + ".height", filepath.Join(dir, "v6120-halt-heights.json")},
		current:     filepath.Join(dir, "."+base+".current"),
		generations: filepath.Join(dir, "."+base+".generations"),
	}
}

func optionalFile(path string) ([]byte, error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	return raw, err
}

func (p *publication) open() (export, record, heights []byte, err error) {
	target, err := os.Readlink(p.current)
	if errors.Is(err, os.ErrNotExist) {
		export, err = os.ReadFile(p.paths[0])
		if err != nil {
			return
		}
		record, err = optionalFile(p.paths[1])
		if err != nil {
			return
		}
		heights, err = optionalFile(p.paths[2])
		return
	}
	if err != nil {
		return nil, nil, nil, err
	}
	// A current pointer always names a complete immutable sibling generation.
	prefix := filepath.Base(p.generations) + string(filepath.Separator)
	if !strings.HasPrefix(target, prefix) || filepath.Dir(target) != filepath.Base(p.generations) || !strings.HasPrefix(filepath.Base(target), "gen-") {
		return nil, nil, nil, fmt.Errorf("invalid generation pointer %q", target)
	}
	dir := filepath.Join(filepath.Dir(p.current), target)
	raw, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return nil, nil, nil, err
	}
	var manifest generationManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return nil, nil, nil, err
	}
	if manifest.Version != 1 || len(manifest.Digests) != len(artifactNames) {
		return nil, nil, nil, fmt.Errorf("invalid generation manifest")
	}
	contents := make([][]byte, len(artifactNames))
	for i, name := range artifactNames {
		path := filepath.Join(dir, name)
		info, err := os.Lstat(path)
		if err != nil {
			return nil, nil, nil, err
		}
		if !info.Mode().IsRegular() {
			return nil, nil, nil, fmt.Errorf("generation artifact %s is not a regular file", name)
		}
		contents[i], err = os.ReadFile(path)
		if err != nil {
			return nil, nil, nil, err
		}
		if digest(contents[i]) != manifest.Digests[name] {
			return nil, nil, nil, fmt.Errorf("generation artifact %s digest mismatch", name)
		}
	}
	var rec fetchRecord
	if err := json.Unmarshal(contents[1], &rec); err != nil {
		return nil, nil, nil, err
	}
	if err := rec.validate(); err != nil {
		return nil, nil, nil, err
	}
	if rec.ExportDigest != digest(contents[0]) || rec.HeightsDigest != digest(contents[2]) {
		return nil, nil, nil, fmt.Errorf("mixed snapshot generation")
	}
	if err := p.installAliases(manifest); err != nil {
		return nil, nil, nil, err
	}
	return contents[0], contents[1], contents[2], nil
}

func (p *publication) aliasTarget(i int) string {
	return filepath.Join(filepath.Base(p.current), artifactNames[i])
}

// Validate every alias before repairing a bootstrap interrupted after the
// pointer commit. Only the exact pre-publication files may be replaced.
func (p *publication) installAliases(manifest generationManifest) error {
	replace := make([]bool, len(p.paths))
	for i, path := range p.paths {
		info, err := os.Lstat(path)
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			if target != p.aliasTarget(i) {
				return fmt.Errorf("mixed snapshot alias %s", path)
			}
			continue
		}
		prior, allowed := manifest.Bootstrap[artifactNames[i]]
		if errors.Is(err, os.ErrNotExist) && allowed && !prior.Present {
			replace[i] = true
			continue
		}
		if err != nil {
			return err
		}
		if !allowed || !prior.Present || !info.Mode().IsRegular() {
			return fmt.Errorf("unexpected snapshot sidecar %s", path)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if digest(raw) != prior.Digest {
			return fmt.Errorf("mixed snapshot sidecar %s", path)
		}
		replace[i] = true
	}
	for i, needed := range replace {
		if needed {
			if err := atomicSymlink(p.aliasTarget(i), p.paths[i]); err != nil {
				return err
			}
		}
	}
	return syncDirectory(filepath.Dir(p.current))
}

func syncedFile(path string, raw []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(raw); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func syncDirectory(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func atomicSymlink(target, path string) error {
	dir, err := os.MkdirTemp(filepath.Dir(path), ".publish-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	tmp := filepath.Join(dir, "link")
	if err := os.Symlink(target, tmp); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	return syncDirectory(filepath.Dir(path))
}

func (p *publication) stage(export, record, heights []byte) (*stagedPublication, error) {
	var rec fetchRecord
	if err := json.Unmarshal(record, &rec); err != nil {
		return nil, err
	}
	if err := rec.validate(); err != nil {
		return nil, err
	}
	if rec.ExportDigest != digest(export) || rec.HeightsDigest != digest(heights) {
		return nil, fmt.Errorf("cannot stage mixed snapshot artifacts")
	}
	manifest := generationManifest{Version: 1, Digests: map[string]common.Hash{}, Bootstrap: map[string]priorArtifact{}}
	if _, err := os.Lstat(p.current); errors.Is(err, os.ErrNotExist) {
		for i, path := range p.paths {
			info, err := os.Lstat(path)
			if errors.Is(err, os.ErrNotExist) {
				manifest.Bootstrap[artifactNames[i]] = priorArtifact{}
				continue
			}
			if err != nil {
				return nil, err
			}
			if !info.Mode().IsRegular() {
				return nil, fmt.Errorf("cannot adopt snapshot alias %s", path)
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return nil, err
			}
			manifest.Bootstrap[artifactNames[i]] = priorArtifact{Present: true, Digest: digest(raw)}
		}
	} else if err != nil {
		return nil, err
	} else {
		// Reopen also checks aliases and the old generation before replacing it.
		_, previousRaw, _, err := p.open()
		if err != nil {
			return nil, err
		}
		var previous fetchRecord
		if err := json.Unmarshal(previousRaw, &previous); err != nil {
			return nil, err
		}
		previous.ExportDigest = rec.ExportDigest
		if previous.snapshotBinding != rec.snapshotBinding {
			return nil, fmt.Errorf("cannot replace generation with a different snapshot")
		}
	}
	if err := os.MkdirAll(p.generations, 0o755); err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp(p.generations, "gen-")
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(dir, 0o755); err != nil {
		return nil, err
	}
	for i, raw := range [][]byte{export, record, heights} {
		name := artifactNames[i]
		if err := syncedFile(filepath.Join(dir, name), raw); err != nil {
			return nil, err
		}
		manifest.Digests[name] = digest(raw)
	}
	raw, err := marshal(manifest)
	if err != nil {
		return nil, err
	}
	if err := syncedFile(filepath.Join(dir, "manifest.json"), append(raw, '\n')); err != nil {
		return nil, err
	}
	for _, path := range []string{dir, p.generations, filepath.Dir(p.current)} {
		if err := syncDirectory(path); err != nil {
			return nil, err
		}
	}
	return &stagedPublication{publication: p, directory: dir, manifest: manifest}, nil
}

func (s *stagedPublication) commitPointer() error {
	target := filepath.Join(filepath.Base(s.publication.generations), filepath.Base(s.directory))
	return atomicSymlink(target, s.publication.current)
}

func (p *publication) publish(export, record, heights []byte) error {
	staged, err := p.stage(export, record, heights)
	if err != nil {
		return err
	}
	if err := staged.commitPointer(); err != nil {
		return err
	}
	return p.installAliases(staged.manifest)
}
