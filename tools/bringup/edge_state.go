package bringup

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
)

const EdgeMarker = "# rendered by tools/bringup/edge.sh; edit the manifest through it, not this file"

type edgeFile struct {
	Data []byte
	Link string
	Mode uint32
}
type edgeJournal struct {
	Generation string
	Files      map[string]edgeFile
}

// EdgeState owns only the edge state and marker-owned nginx files. The lock
// spans the caller's render, validation and reload, including separate CLI calls.
type EdgeState struct{ Root string }

func edgeAtomic(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".edge-write-")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err = f.Chmod(mode); err == nil {
		_, err = f.Write(data)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err = os.Rename(name, path); err != nil {
		return err
	}
	return edgeSyncDir(filepath.Dir(path))
}
func edgeSyncDir(path string) error {
	f, e := os.Open(path)
	if e != nil {
		return e
	}
	defer f.Close()
	return f.Sync()
}
func OpenEdgeState(root string) (*EdgeState, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(root, 0755); err != nil {
		return nil, err
	}
	return &EdgeState{Root: root}, nil
}
func (s *EdgeState) Close() error {
	if s.Root == "" {
		return errors.New("edge state closed")
	}
	s.Root = ""
	return nil
}
func (s *EdgeState) withLock(action func() error) error {
	if s.Root == "" {
		return errors.New("edge state closed")
	}
	f, e := os.OpenFile(filepath.Join(s.Root, ".edge.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return e
	}
	defer f.Close()
	if e = syscall.Flock(int(f.Fd()), syscall.LOCK_EX); e != nil {
		return e
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	return action()
}
func edgeOwned(path string) bool {
	b, e := os.ReadFile(path)
	return e == nil && strings.SplitN(string(b), "\n", 2)[0] == EdgeMarker
}
func (s *EdgeState) files() (map[string]edgeFile, error) {
	files := map[string]edgeFile{}
	add := func(path string) error {
		info, e := os.Lstat(path)
		if e != nil {
			return e
		}
		rel, e := filepath.Rel(s.Root, path)
		if e != nil {
			return e
		}
		record := edgeFile{Mode: uint32(info.Mode().Perm())}
		if info.Mode()&os.ModeSymlink != 0 {
			record.Link, e = os.Readlink(path)
		} else if info.Mode().IsRegular() {
			record.Data, e = os.ReadFile(path)
		} else {
			return errors.New("edge snapshot contains special file")
		}
		if e != nil {
			return e
		}
		files[rel] = record
		return nil
	}
	state := filepath.Join(s.Root, "edge")
	if _, e := os.Lstat(state); e == nil {
		if e = filepath.WalkDir(state, func(path string, d os.DirEntry, e error) error {
			if e != nil {
				return e
			}
			if d.IsDir() {
				return nil
			}
			return add(path)
		}); e != nil {
			return nil, e
		}
	} else if !os.IsNotExist(e) {
		return nil, e
	}
	// Capture links before deleting targets; dangling links are owned only when
	// the last committed generation explicitly recorded that exact link.
	previous := map[string]edgeFile{}
	if b, e := os.ReadFile(filepath.Join(s.Root, ".edge-owned.json")); e == nil {
		if e = json.Unmarshal(b, &previous); e != nil {
			return nil, e
		}
	} else if !os.IsNotExist(e) {
		return nil, e
	}
	for _, dir := range []string{"sites-available", "sites-enabled"} {
		entries, e := os.ReadDir(filepath.Join(s.Root, dir))
		if os.IsNotExist(e) {
			continue
		}
		if e != nil {
			return nil, e
		}
		for _, entry := range entries {
			path := filepath.Join(s.Root, dir, entry.Name())
			rel := filepath.Join(dir, entry.Name())
			owned := edgeOwned(path)
			if !owned && entry.Type()&os.ModeSymlink != 0 {
				target, e := os.Readlink(path)
				if e != nil {
					return nil, e
				}
				old, ok := previous[rel]
				owned = ok && old.Link == target
			}
			if owned {
				if e = add(path); e != nil {
					return nil, e
				}
			}
		}
	}
	if _, e := os.Lstat(filepath.Join(s.Root, ".edge-owned.json")); e == nil {
		if e = add(filepath.Join(s.Root, ".edge-owned.json")); e != nil {
			return nil, e
		}
	} else if !os.IsNotExist(e) {
		return nil, e
	}
	path := filepath.Join(s.Root, "modules-enabled", "99-edge-stream.conf")
	if edgeOwned(path) {
		if e := add(path); e != nil {
			return nil, e
		}
	}
	return files, nil
}
func (s *EdgeState) journal() (edgeJournal, error) {
	var j edgeJournal
	b, e := os.ReadFile(filepath.Join(s.Root, ".edge-transaction.json"))
	if e != nil {
		return j, e
	}
	e = json.Unmarshal(b, &j)
	if e == nil && (j.Generation == "" || j.Files == nil) {
		e = errors.New("invalid edge transaction")
	}
	return j, e
}
func (s *EdgeState) restore(j edgeJournal) error {
	current, e := s.files()
	if e != nil {
		return e
	}
	for path := range current {
		if e = os.Remove(filepath.Join(s.Root, path)); e != nil && !os.IsNotExist(e) {
			return e
		}
	}
	paths := make([]string, 0, len(j.Files))
	for path := range j.Files {
		if filepath.IsAbs(path) || filepath.Clean(path) != path || path == ".." || strings.HasPrefix(path, "../") {
			return errors.New("invalid edge snapshot path")
		}
		paths = append(paths, path)
	}
	sort.Strings(paths)
	for _, path := range paths {
		record := j.Files[path]
		path = filepath.Join(s.Root, path)
		if e = os.MkdirAll(filepath.Dir(path), 0755); e != nil {
			return e
		}
		if record.Link != "" {
			if e = os.Remove(path); e != nil && !os.IsNotExist(e) {
				return e
			}
			e = os.Symlink(record.Link, path)
			if e == nil {
				e = edgeSyncDir(filepath.Dir(path))
			}
		} else {
			e = edgeAtomic(path, record.Data, os.FileMode(record.Mode))
		}
		if e != nil {
			return e
		}
	}
	for path := range current {
		if e = edgeSyncDir(filepath.Dir(filepath.Join(s.Root, path))); e != nil && !os.IsNotExist(e) {
			return e
		}
	}
	return s.clearJournal()
}
func (s *EdgeState) clearJournal() error {
	if e := os.Remove(filepath.Join(s.Root, ".edge-transaction.json")); e != nil {
		return e
	}
	return edgeSyncDir(s.Root)
}
func (s *EdgeState) recover() error {
	j, e := s.journal()
	if os.IsNotExist(e) {
		return nil
	}
	if e != nil {
		return e
	}
	b, e := os.ReadFile(filepath.Join(s.Root, "edge", "generation"))
	if e != nil && !os.IsNotExist(e) {
		return e
	}
	if strings.TrimSpace(string(b)) == j.Generation {
		return s.clearJournal()
	}
	return s.restore(j)
}
func (s *EdgeState) Recover() error { return s.withLock(s.recover) }
func (s *EdgeState) Begin() error   { return s.withLock(s.begin) }
func (s *EdgeState) begin() error {
	if e := s.recover(); e != nil {
		return e
	}
	files, e := s.files()
	if e != nil {
		return e
	}
	var id [16]byte
	if _, e = rand.Read(id[:]); e != nil {
		return e
	}
	b, e := json.Marshal(edgeJournal{hex.EncodeToString(id[:]), files})
	if e != nil {
		return e
	}
	return edgeAtomic(filepath.Join(s.Root, ".edge-transaction.json"), b, 0600)
}

// Finish accepts the reload acknowledgement. A refused validation or reload
// restores the prior generation before returning to the shell's failure path.
func (s *EdgeState) Finish(acknowledged bool) error {
	return s.withLock(func() error {
		j, e := s.journal()
		if e != nil {
			return e
		}
		if !acknowledged {
			return s.restore(j)
		}
		files, e := s.files()
		if e != nil {
			return e
		}
		for path, record := range files {
			if record.Link != "" {
				continue
			}
			f, e := os.OpenFile(filepath.Join(s.Root, path), os.O_RDONLY, 0)
			if e != nil {
				return e
			}
			e = f.Sync()
			f.Close()
			if e != nil {
				return e
			}
		}
		owned := map[string]edgeFile{}
		for path, record := range files {
			if strings.HasPrefix(path, "sites-") {
				owned[path] = record
			}
		}
		b, e := json.Marshal(owned)
		if e != nil {
			return e
		}
		if e = edgeAtomic(filepath.Join(s.Root, ".edge-owned.json"), b, 0600); e != nil {
			return e
		}
		for _, dir := range []string{"edge", "edge/stream", "sites-available", "sites-enabled", "modules-enabled"} {
			if e = edgeSyncDir(filepath.Join(s.Root, dir)); e != nil && !os.IsNotExist(e) {
				return e
			}
		}
		if e = edgeAtomic(filepath.Join(s.Root, "edge", "generation"), []byte(j.Generation+"\n"), 0600); e != nil {
			return e
		}
		return s.clearJournal()
	})
}

// Reconcile revokes only marker-owned HTTP sites absent from the transferred
// manifest, before nginx validation. Both available files and enabled links go.
func (s *EdgeState) Reconcile() error { return s.withLock(s.reconcile) }
func (s *EdgeState) reconcile() error {
	b, e := os.ReadFile(filepath.Join(s.Root, "edge", "manifest"))
	if e != nil {
		return e
	}
	rows, e := ReadEdgeManifest(strings.NewReader(string(b)), true)
	if e != nil {
		return e
	}
	keep := map[string]bool{}
	for _, r := range rows {
		if r.Mode == "http" || r.Mode == "rpc" {
			keep[r.Name+".conf"] = true
		}
	}
	files, e := s.files()
	if e != nil {
		return e
	}
	for path := range files {
		if (strings.HasPrefix(path, "sites-available/") || strings.HasPrefix(path, "sites-enabled/")) && !keep[filepath.Base(path)] {
			if e = os.Remove(filepath.Join(s.Root, path)); e != nil && !os.IsNotExist(e) {
				return e
			}
		}
	}
	for _, dir := range []string{"sites-available", "sites-enabled"} {
		if e = edgeSyncDir(filepath.Join(s.Root, dir)); e != nil && !os.IsNotExist(e) {
			return e
		}
	}
	return nil
}
func EdgeGeneration(root string) (string, error) {
	b, e := os.ReadFile(filepath.Join(root, "edge", "generation"))
	if e != nil {
		return "", e
	}
	if len(strings.TrimSpace(string(b))) != 32 {
		return "", fmt.Errorf("invalid edge generation")
	}
	return strings.TrimSpace(string(b)), nil
}

// ReloadResult consumes nginx's actual validation and reload exit statuses.
// A negative reload status means validation refused before a reload was tried.
func (s *EdgeState) ReloadResult(validation, reload int) error {
	if validation < 0 || reload < -1 || validation == 0 && reload < 0 {
		return errors.New("invalid edge reload result")
	}
	if validation != 0 || reload != 0 {
		if e := s.Finish(false); e != nil {
			return e
		}
		if validation != 0 {
			return errors.New("nginx validation refused; previous generation restored")
		}
		return errors.New("nginx reload refused; previous generation restored")
	}
	return s.Finish(true)
}
