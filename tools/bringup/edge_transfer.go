package bringup

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type EdgeTransfer struct {
	Version int
	Root    string
	Files   map[string]edgeFile
}

func edgeTransferPath(path string) bool {
	if path == "edge/generation" || path == "edge/rpc-down" || path == "edge/renewed" || strings.HasSuffix(path, ".lock") {
		return false
	}
	return strings.HasPrefix(path, "edge/") || strings.HasPrefix(path, "sites-available/") || strings.HasPrefix(path, "sites-enabled/") || path == "modules-enabled/99-edge-stream.conf"
}
func (s *EdgeState) Export(output io.Writer) error {
	return s.withLock(func() error {
		if _, e := s.journal(); e == nil {
			return errors.New("cannot transfer an unacknowledged edge generation")
		} else if !os.IsNotExist(e) {
			return e
		}
		files, e := s.files()
		if e != nil {
			return e
		}
		transfer := EdgeTransfer{Version: 1, Root: s.Root, Files: map[string]edgeFile{}}
		for path, file := range files {
			if edgeTransferPath(path) {
				transfer.Files[path] = file
			}
		}
		return json.NewEncoder(output).Encode(transfer)
	})
}

// Import stages the transferred generation under the same durable transaction
// that rerender validates and reloads. Receiver-local health is retained.
func (s *EdgeState) Import(input io.Reader) (bool, error) {
	var transfer EdgeTransfer
	d := json.NewDecoder(io.LimitReader(input, 32<<20))
	d.DisallowUnknownFields()
	if d.Decode(&transfer) != nil || transfer.Version != 1 || !filepath.IsAbs(transfer.Root) || transfer.Files == nil {
		return false, errors.New("edge transfer malformed")
	}
	if d.Decode(new(any)) != io.EOF {
		return false, errors.New("edge transfer trailing input")
	}
	manifest, ok := transfer.Files["edge/manifest"]
	if !ok || manifest.Link != "" {
		return false, errors.New("edge transfer manifest missing")
	}
	rows, e := ReadEdgeManifest(bytes.NewReader(manifest.Data), true)
	if e != nil {
		return false, e
	}
	names := map[string]bool{}
	for _, row := range rows {
		if row.Mode == "http" || row.Mode == "rpc" {
			names[row.Name+".conf"] = true
		}
	}
	proposed := map[string]edgeFile{}
	for path, file := range transfer.Files {
		if filepath.IsAbs(path) || filepath.Clean(path) != path || strings.HasPrefix(path, "../") || !edgeTransferPath(path) {
			return false, errors.New("edge transfer path refused")
		}
		if strings.HasPrefix(path, "sites-") {
			if !names[filepath.Base(path)] {
				return false, errors.New("edge transfer contains unregistered site")
			}
			if file.Link == "" && strings.SplitN(string(file.Data), "\n", 2)[0] != EdgeMarker {
				return false, errors.New("edge transfer site is not owned")
			}
		}
		if file.Link != "" {
			want := filepath.Join(transfer.Root, "sites-available", filepath.Base(path))
			if !strings.HasPrefix(path, "sites-enabled/") || file.Link != want {
				return false, errors.New("edge transfer symlink refused")
			}
			file.Link = filepath.Join(s.Root, "sites-available", filepath.Base(path))
		} else {
			file.Data = bytes.ReplaceAll(file.Data, []byte(transfer.Root), []byte(s.Root))
		}
		proposed[path] = file
	}
	changed := false
	e = s.withLock(func() error {
		if e := s.recover(); e != nil {
			return e
		}
		current, e := s.files()
		if e != nil {
			return e
		}
		for path, file := range proposed {
			old, ok := current[path]
			if !ok || old.Link != file.Link || old.Mode != file.Mode || !bytes.Equal(old.Data, file.Data) {
				changed = true
			}
		}
		for path := range current {
			if edgeTransferPath(path) {
				if _, ok := proposed[path]; !ok {
					changed = true
				}
			}
		}
		if !changed {
			return nil
		}
		if e = s.begin(); e != nil {
			return e
		}
		// Revoke using the transferred manifest before rendering the new generation.
		for path, file := range proposed {
			dest := filepath.Join(s.Root, path)
			if strings.HasPrefix(path, "sites-") {
				if _, e := os.Lstat(dest); e == nil {
					if _, owned := current[path]; !owned {
						return errors.New("edge import would overwrite unrelated site")
					}
				} else if !os.IsNotExist(e) {
					return e
				}
			}
			if e = os.MkdirAll(filepath.Dir(dest), 0755); e != nil {
				return e
			}
			if file.Link != "" {
				if e = os.Remove(dest); e != nil && !os.IsNotExist(e) {
					return e
				}
				e = os.Symlink(file.Link, dest)
				if e == nil {
					e = edgeSyncDir(filepath.Dir(dest))
				}
			} else {
				e = edgeAtomic(dest, file.Data, os.FileMode(file.Mode))
			}
			if e != nil {
				return e
			}
		}
		if e = s.reconcile(); e != nil {
			return e
		}
		for path := range current {
			if edgeTransferPath(path) && strings.HasPrefix(path, "edge/") {
				if _, ok := proposed[path]; !ok {
					if e = os.Remove(filepath.Join(s.Root, path)); e != nil && !os.IsNotExist(e) {
						return e
					}
				}
			}
		}
		if _, ok := proposed["modules-enabled/99-edge-stream.conf"]; !ok {
			if edgeOwned(filepath.Join(s.Root, "modules-enabled/99-edge-stream.conf")) {
				if e = os.Remove(filepath.Join(s.Root, "modules-enabled/99-edge-stream.conf")); e != nil {
					return e
				}
			}
		}
		return nil
	})
	return changed, e
}
