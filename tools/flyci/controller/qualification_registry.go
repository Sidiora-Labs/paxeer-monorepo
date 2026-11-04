package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

type qualificationRegistry struct{ root string }

func qPrivate(path string, directory bool) error {
	clean := filepath.Clean(path)
	if !filepath.IsAbs(clean) {
		return errors.New("private path must be absolute")
	}
	parts := strings.Split(clean, string(os.PathSeparator))
	current := string(os.PathSeparator)
	for _, p := range parts {
		if p == "" {
			continue
		}
		if p == ".env" || strings.HasPrefix(p, ".env.") {
			return errors.New("credential files prohibited")
		}
		current = filepath.Join(current, p)
		s, e := os.Lstat(current)
		if e != nil {
			return e
		}
		if s.Mode()&os.ModeSymlink != 0 {
			return errors.New("symlink refused")
		}
	}
	s, e := os.Stat(clean)
	if e != nil {
		return e
	}
	st, ok := s.Sys().(*syscall.Stat_t)
	if !ok || int(st.Uid) != os.Geteuid() || s.Mode().Perm()&0077 != 0 {
		return errors.New("private ownership or permissions refused")
	}
	if directory {
		if !s.IsDir() {
			return errors.New("private directory required")
		}
	} else if !s.Mode().IsRegular() || st.Nlink != 1 {
		return errors.New("private single-link regular file required")
	}
	return nil
}
func qRead(path string) ([]byte, error) {
	if e := qPrivate(path, false); e != nil {
		return nil, e
	}
	f, e := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if e != nil {
		return nil, e
	}
	defer f.Close()
	s, e := f.Stat()
	if e != nil {
		return nil, e
	}
	if s.Size() > qualificationLimit {
		return nil, errors.New("private document exceeds bound")
	}
	b := make([]byte, s.Size())
	_, e = f.ReadAt(b, 0)
	if len(b) == 0 {
		e = errors.New("empty document")
	}
	return b, e
}
func qSyncDir(path string) error {
	f, e := os.Open(path)
	if e != nil {
		return e
	}
	defer f.Close()
	return f.Sync()
}
func qStageWrite(path string) (*os.File, error) {
	if e := qPrivate(filepath.Dir(path), true); e != nil {
		return nil, e
	}
	return os.CreateTemp(filepath.Dir(path), ".qualification-*.tmp")
}
func qPrepareWrite(f *os.File, b []byte) error {
	_, e := f.Write(b)
	if e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e != nil {
		return e
	}
	if ce != nil {
		return ce
	}
	return nil
}
func qPublishWrite(staged, path string) error {
	return unix.Renameat2(unix.AT_FDCWD, staged, unix.AT_FDCWD, path, unix.RENAME_NOREPLACE)
}
func qWrite(path string, b []byte) error {
	f, e := qStageWrite(path)
	if e != nil {
		return e
	}
	defer os.Remove(f.Name())
	if e = qPrepareWrite(f, b); e != nil {
		return e
	}
	if e = qPublishWrite(f.Name(), path); e != nil {
		return e
	}
	return qSyncDir(filepath.Dir(path))
}
func qStagedWrite(name string) bool {
	if !strings.HasPrefix(name, ".qualification-") || !strings.HasSuffix(name, ".tmp") {
		return false
	}
	number := strings.TrimSuffix(strings.TrimPrefix(name, ".qualification-"), ".tmp")
	if number == "" {
		return false
	}
	for _, c := range number {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// Only staging names are uncommitted. A final record, including dispatch_intent,
// is never discarded or rewritten, even when its caller did not see success.
// Callers hold the registry lock so recovery cannot remove a live writer's file.
func (r *qualificationRegistry) recoverRequest(id string) (bool, error) {
	dir := filepath.Join(r.root, id)
	if e := qPrivate(dir, true); e != nil {
		return false, e
	}
	entries, e := os.ReadDir(dir)
	if e != nil {
		return false, e
	}
	remaining := 0
	for _, entry := range entries {
		if !qStagedWrite(entry.Name()) {
			remaining++
			continue
		}
		path := filepath.Join(dir, entry.Name())
		if e = qPrivate(path, false); e != nil {
			return false, e
		}
		if e = os.Remove(path); e != nil {
			return false, e
		}
	}
	if e = qSyncDir(dir); e != nil {
		return false, e
	}
	if remaining != 0 {
		return false, nil
	}
	// No first record was published, so no dispatch could have been granted.
	if e = os.Remove(dir); e != nil {
		return false, e
	}
	return true, qSyncDir(r.root)
}
func newQualificationRegistry(root string) (*qualificationRegistry, error) {
	if e := qPrivate(root, true); e != nil {
		return nil, e
	}
	r := &qualificationRegistry{root}
	lock, e := r.lock()
	if e != nil {
		return nil, e
	}
	defer lock.Close()
	entries, e := os.ReadDir(root)
	if e != nil {
		return nil, e
	}
	for _, entry := range entries {
		if qDigest.MatchString(entry.Name()) {
			if _, e = r.recoverRequest(entry.Name()); e != nil {
				return nil, e
			}
		}
	}
	return r, nil
}
func (r *qualificationRegistry) lock() (*os.File, error) {
	f, e := os.OpenFile(filepath.Join(r.root, "registry.lock"), os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW, 0600)
	if e != nil {
		return nil, e
	}
	if e = qPrivate(f.Name(), false); e != nil {
		f.Close()
		return nil, e
	}
	if e = syscall.Flock(int(f.Fd()), syscall.LOCK_EX); e != nil {
		f.Close()
		return nil, e
	}
	return f, nil
}
func (r *qualificationRegistry) read(id string) (qualificationRecord, error) {
	var rec qualificationRecord
	if !qDigest.MatchString(id) {
		return rec, errors.New("invalid logical identity")
	}
	dir := filepath.Join(r.root, id)
	if e := qPrivate(dir, true); e != nil {
		return rec, e
	}
	entries, e := os.ReadDir(dir)
	if e != nil {
		return rec, e
	}
	previous := ""
	seq := int64(0)
	for _, entry := range entries {
		if qStagedWrite(entry.Name()) {
			if e = qPrivate(filepath.Join(dir, entry.Name()), false); e != nil {
				return rec, e
			}
			continue
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			return rec, errors.New("registry has unexpected entry")
		}
		seq++
		if entry.Name() != fmt.Sprintf("%020d.json", seq) {
			return rec, errors.New("registry sequence gap")
		}
		b, e := qRead(filepath.Join(dir, entry.Name()))
		if e != nil {
			return rec, e
		}
		var next qualificationRecord
		if e = qStrict(b, &next); e != nil {
			return rec, e
		}
		if next.Sequence != seq || next.Previous != previous || next.Request.LogicalID != id {
			return rec, errors.New("registry event binding failed")
		}
		if e = next.Request.validate(); e != nil {
			return rec, e
		}
		if seq == 1 {
			if next.State != "prepared" {
				return rec, errors.New("invalid initial state")
			}
		} else {
			nextBytes, err := qCanonical(next.Request)
			if err != nil {
				return rec, err
			}
			oldBytes, err := qCanonical(rec.Request)
			if err != nil {
				return rec, err
			}
			if !qTransition(rec.State, next.State) || !bytes.Equal(nextBytes, oldBytes) {
				return rec, errors.New("invalid registry transition")
			}
		}
		rec = next
		previous = qHash(b)
	}
	if seq == 0 {
		return rec, errors.New("empty registry is not recovery proof")
	}
	return rec, nil
}
func (r *qualificationRegistry) submit(req qualificationRequest) (qualificationRecord, error) {
	var rec qualificationRecord
	if e := req.validate(); e != nil {
		return rec, e
	}
	lock, e := r.lock()
	if e != nil {
		return rec, e
	}
	defer lock.Close()
	dir := filepath.Join(r.root, req.LogicalID)
	if _, e = os.Lstat(dir); e == nil {
		empty, err := r.recoverRequest(req.LogicalID)
		if err != nil {
			return rec, err
		}
		if !empty {
			old, err := r.read(req.LogicalID)
			if err == nil {
				oldBytes, e := qCanonical(old.Request)
				if e != nil {
					return old, e
				}
				newBytes, e := qCanonical(req)
				if e != nil {
					return old, e
				}
				if !bytes.Equal(oldBytes, newBytes) {
					err = errors.New("logical request payload changed")
				}
			}
			return old, err
		}
	} else if !os.IsNotExist(e) {
		return rec, e
	}
	if e = os.Mkdir(dir, 0700); e != nil {
		return rec, e
	}
	if e = qSyncDir(r.root); e != nil {
		return rec, e
	}
	rec = qualificationRecord{Request: req, Sequence: 1, State: "prepared", UpdatedAt: time.Now().UTC()}
	b, e := qCanonical(rec)
	if e != nil {
		return rec, e
	}
	e = qWrite(filepath.Join(dir, fmt.Sprintf("%020d.json", 1)), b)
	return rec, e
}
func (r *qualificationRegistry) advance(id, old, state string, run, attempt int64, detail string) (qualificationRecord, error) {
	var rec qualificationRecord
	lock, e := r.lock()
	if e != nil {
		return rec, e
	}
	defer lock.Close()
	rec, e = r.read(id)
	if e != nil {
		return rec, e
	}
	if rec.State != old || !qTransition(old, state) {
		return rec, errors.New("state changed or transition refused")
	}
	previous, e := qRead(filepath.Join(r.root, id, fmt.Sprintf("%020d.json", rec.Sequence)))
	if e != nil {
		return rec, e
	}
	rec.Previous = qHash(previous)
	rec.Sequence++
	rec.State = state
	rec.RunID = run
	rec.Attempt = attempt
	rec.Detail = detail
	rec.UpdatedAt = time.Now().UTC()
	b, e := qCanonical(rec)
	if e != nil {
		return rec, e
	}
	return rec, qWrite(filepath.Join(r.root, id, fmt.Sprintf("%020d.json", rec.Sequence)), b)
}
func (r *qualificationRegistry) list() ([]qualificationRecord, error) {
	entries, e := os.ReadDir(r.root)
	if e != nil {
		return nil, e
	}
	var out []qualificationRecord
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if !qDigest.MatchString(entry.Name()) {
			continue
		}
		rec, e := r.read(entry.Name())
		if e != nil {
			return nil, e
		}
		out = append(out, rec)
	}
	return out, nil
}
func (r *qualificationRegistry) protects(runID int64) bool {
	records, e := r.list()
	if e != nil {
		return true
	}
	for _, rec := range records {
		if rec.RunID == runID && rec.State != "completed_validated" && rec.State != "terminal_failed" {
			return true
		}
	}
	return false
}

func (r *qualificationRegistry) protectsAssigned(m machine, idx *jobIndex) bool {
	records, e := r.list()
	if e != nil {
		return true
	}
	active := false
	for _, rec := range records {
		if rec.State != "completed_validated" && rec.State != "terminal_failed" {
			active = true
		}
	}
	if !active {
		return false
	}
	var assigned *workflowJob
	for _, job := range idx.byID {
		if job.RunnerID > 0 && job.RunnerName == m.Name {
			if assigned != nil {
				return true
			}
			copy := job
			assigned = &copy
		}
	}
	if assigned == nil {
		return true
	}
	return r.protects(assigned.RunID)
}
