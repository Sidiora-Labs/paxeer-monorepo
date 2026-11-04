package replica

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

const ledgerVersion = 1

var ErrLedger = errors.New("replica: ledger")

type LedgerEntry struct {
	Name      string    `json:"name"`
	Size      int64     `json:"size"`
	SHA256    string    `json:"sha256"`
	ShippedAt time.Time `json:"shipped_at"`
}

type ledgerFile struct {
	Version     int           `json:"version"`
	Entries     []LedgerEntry `json:"entries"`
	LastShipped time.Time     `json:"last_shipped,omitempty"`
}

type Ledger struct {
	path    string
	mu      sync.Mutex
	entries map[string]LedgerEntry
	last    time.Time
}

func OpenLedger(path string) (*Ledger, error) {
	l := &Ledger{path: path, entries: make(map[string]LedgerEntry)}
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return l, nil
	}
	if err != nil {
		return nil, fmt.Errorf("%w: read: %v", ErrLedger, err)
	}
	var file ledgerFile
	if err := json.Unmarshal(raw, &file); err != nil {
		return nil, fmt.Errorf("%w: undecodable", ErrLedger)
	}
	if file.Version != ledgerVersion {
		return nil, fmt.Errorf("%w: unsupported version %d", ErrLedger, file.Version)
	}
	l.last = file.LastShipped
	for _, e := range file.Entries {
		if err := validEntry(e); err != nil {
			return nil, err
		}
		if _, dup := l.entries[e.Name]; dup {
			return nil, fmt.Errorf("%w: %q appears twice", ErrLedger, e.Name)
		}
		l.entries[e.Name] = e
		if e.ShippedAt.After(l.last) {
			l.last = e.ShippedAt
		}
	}
	return l, nil
}

func validEntry(e LedgerEntry) error {
	if !validName(e.Name) {
		return fmt.Errorf("%w: invalid snapshot name %q", ErrLedger, e.Name)
	}
	if e.Size < 0 {
		return fmt.Errorf("%w: %q has a negative size", ErrLedger, e.Name)
	}
	if d, err := hex.DecodeString(e.SHA256); err != nil || len(d) != 32 {
		return fmt.Errorf("%w: %q has a malformed digest", ErrLedger, e.Name)
	}
	if e.ShippedAt.IsZero() {
		return fmt.Errorf("%w: %q has no shipping time", ErrLedger, e.Name)
	}
	return nil
}

func (l *Ledger) Lookup(name string) (LedgerEntry, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	e, ok := l.entries[name]
	return e, ok
}

func (l *Ledger) Entries() []LedgerEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]LedgerEntry, 0, len(l.entries))
	for _, e := range l.entries {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (l *Ledger) LastShipped() time.Time {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.last
}

func (l *Ledger) Record(e LedgerEntry) error {
	if err := validEntry(e); err != nil {
		return err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, dup := l.entries[e.Name]; dup {
		return fmt.Errorf("%w: %q is already recorded", ErrLedger, e.Name)
	}
	if _, ok, err := l.Archived(e.Name); err != nil {
		return err
	} else if ok {
		return fmt.Errorf("%w: %q is already archived", ErrLedger, e.Name)
	}
	last := l.last
	if e.ShippedAt.After(last) {
		last = e.ShippedAt
	}
	file := ledgerFile{Version: ledgerVersion, LastShipped: last, Entries: make([]LedgerEntry, 0, len(l.entries)+1)}
	for _, existing := range l.entries {
		file.Entries = append(file.Entries, existing)
	}
	file.Entries = append(file.Entries, e)
	sort.Slice(file.Entries, func(i, j int) bool { return file.Entries[i].Name < file.Entries[j].Name })
	if err := writeAtomic(l.path, file); err != nil {
		return err
	}
	l.entries[e.Name] = e
	l.last = last
	return nil
}

// archivePath uses a sharded, on-disk index. Archives are not loaded at startup
// or rewritten for later shipments; each acknowledgement is retained indefinitely.
func (l *Ledger) archivePath(name string) string {
	digest := sha256.Sum256([]byte(name))
	key := hex.EncodeToString(digest[:])
	return filepath.Join(l.path+".archive", key[:2], key+".json")
}

// Archived reads one retired acknowledgement without growing the active index.
func (l *Ledger) Archived(name string) (LedgerEntry, bool, error) {
	if !validName(name) {
		return LedgerEntry{}, false, fmt.Errorf("%w: invalid snapshot name %q", ErrLedger, name)
	}
	raw, err := os.ReadFile(l.archivePath(name))
	if errors.Is(err, fs.ErrNotExist) {
		return LedgerEntry{}, false, nil
	}
	if err != nil {
		return LedgerEntry{}, false, fmt.Errorf("%w: read archive: %v", ErrLedger, err)
	}
	var file ledgerFile
	if err := json.Unmarshal(raw, &file); err != nil || file.Version != ledgerVersion || len(file.Entries) != 1 || file.Entries[0].Name != name {
		return LedgerEntry{}, false, fmt.Errorf("%w: invalid archive for %q", ErrLedger, name)
	}
	e := file.Entries[0]
	if err := validEntry(e); err != nil {
		return LedgerEntry{}, false, err
	}
	return e, true, nil
}

// RetireAbsent archives only acknowledged snapshots whose local removal has
// completed. A missing or unreadable snapshot directory is never retirement.
// Archive fsync precedes active-index replacement so either crash ordering
// preserves the acknowledgement. Retrying after an archive-only crash is safe.
func (l *Ledger) RetireAbsent(snapshotDir string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	dir, err := os.Open(snapshotDir)
	if err != nil {
		return fmt.Errorf("%w: open snapshot directory: %v", ErrLedger, err)
	}
	defer dir.Close()
	info, err := dir.Stat()
	if err != nil || !info.IsDir() {
		return fmt.Errorf("%w: snapshot directory unavailable", ErrLedger)
	}
	retained := make([]LedgerEntry, 0, len(l.entries))
	retired := make([]string, 0)
	for name, e := range l.entries {
		_, err := os.Lstat(filepath.Join(snapshotDir, name))
		if err == nil {
			retained = append(retained, e)
			continue
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("%w: inspect local snapshot: %v", ErrLedger, err)
		}
		archived, ok, err := l.Archived(name)
		if err != nil {
			return err
		}
		if ok {
			if archived.Name != e.Name || archived.Size != e.Size || archived.SHA256 != e.SHA256 || !archived.ShippedAt.Equal(e.ShippedAt) {
				return fmt.Errorf("%w: conflicting archive for %q", ErrLedger, name)
			}
		} else if err := writeAtomic(l.archivePath(name), ledgerFile{Version: ledgerVersion, Entries: []LedgerEntry{e}}); err != nil {
			return err
		}
		retired = append(retired, name)
	}
	if len(retired) == 0 {
		return nil
	}
	// Persist the local removals before acknowledging retirement in the ledger.
	if err := dir.Sync(); err != nil {
		return fmt.Errorf("%w: sync snapshot retirement: %v", ErrLedger, err)
	}
	for _, p := range []string{l.path + ".archive", filepath.Dir(l.path)} {
		if err := syncLedgerDir(p); err != nil {
			return err
		}
	}
	sort.Slice(retained, func(i, j int) bool { return retained[i].Name < retained[j].Name })
	if err := writeAtomic(l.path, ledgerFile{Version: ledgerVersion, Entries: retained, LastShipped: l.last}); err != nil {
		return err
	}
	for _, name := range retired {
		delete(l.entries, name)
	}
	return nil
}

func syncLedgerDir(path string) error {
	d, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("%w: open directory: %v", ErrLedger, err)
	}
	defer d.Close()
	if err := d.Sync(); err != nil {
		return fmt.Errorf("%w: sync directory: %v", ErrLedger, err)
	}
	return nil
}

func writeAtomic(path string, file ledgerFile) error {
	raw, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return fmt.Errorf("%w: encode: %v", ErrLedger, err)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("%w: create directory: %v", ErrLedger, err)
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("%w: create: %v", ErrLedger, err)
	}
	tmpName := tmp.Name()
	fail := func(err error) error {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("%w: write: %v", ErrLedger, err)
	}
	if _, err := tmp.Write(append(raw, '\n')); err != nil {
		return fail(err)
	}
	if err := tmp.Sync(); err != nil {
		return fail(err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("%w: write: %v", ErrLedger, err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("%w: rename: %v", ErrLedger, err)
	}
	d, err := os.Open(dir)
	if err != nil {
		return fmt.Errorf("%w: sync directory: %v", ErrLedger, err)
	}
	defer d.Close()
	if err := d.Sync(); err != nil {
		return fmt.Errorf("%w: sync directory: %v", ErrLedger, err)
	}
	return nil
}
