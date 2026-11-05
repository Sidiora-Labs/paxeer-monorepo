package control

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

// LocalObjects is an explicitly selected, single-process object store. Runtime
// never substitutes it for PostgreSQL. Snapshots include denial audits and retry
// claims, and are synced and atomically replaced before a mutation succeeds.
type LocalObjects struct {
	mu      sync.Mutex
	path    string
	lock    *os.File
	closed  bool
	objects map[string]json.RawMessage
	audits  []localAudit
}
type localAudit struct {
	Actor, Role, Operation, Scope, Reason, Result string
	At                                            time.Time
}
type localSnapshot struct {
	Objects map[string]json.RawMessage
	Audits  []localAudit
}
type localContextKey struct{}

func OpenLocalStore(path string, identity Identity, verifier EvidenceVerifier) (*Store, error) {
	if path == "" || identity == nil {
		return nil, Invalid
	}
	lock, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		lock.Close()
		return nil, err
	}
	l := &LocalObjects{path: path, lock: lock, objects: map[string]json.RawMessage{}}
	raw, err := os.ReadFile(path)
	if err == nil {
		var snapshot localSnapshot
		err = json.Unmarshal(raw, &snapshot)
		if err == nil && snapshot.Objects == nil {
			err = Invalid
		}
		l.objects = snapshot.Objects
		l.audits = snapshot.Audits
	}
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		lock.Close()
		return nil, err
	}
	return &Store{local: l, identity: identity, verifier: verifier}, nil
}
func (s *Store) CloseLocal() error {
	if s.local == nil {
		return Invalid
	}
	l := s.local
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return Invalid
	}
	l.closed = true
	return l.lock.Close()
}
func (l *LocalObjects) save(objects map[string]json.RawMessage, audits []localAudit) error {
	raw, err := json.Marshal(localSnapshot{objects, audits})
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(l.path), ".control-objects-")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if err = file.Chmod(0600); err == nil {
		_, err = file.Write(raw)
	}
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err = os.Rename(file.Name(), l.path); err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(l.path))
	if err != nil {
		return err
	}
	defer dir.Close()
	if err = dir.Sync(); err != nil {
		return err
	}
	l.objects = objects
	l.audits = audits
	return nil
}
func (s *Store) runLocal(ctx context.Context, credential, op, scope, reason string, fn func(*transaction) error) error {
	l := s.local
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.closed {
		return Unavailable
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	p, err := s.identity.Resolve(ctx, credential)
	if err != nil {
		return Denied
	}
	objects := make(map[string]json.RawMessage, len(l.objects))
	for k, v := range l.objects {
		objects[k] = v
	}
	t := &transaction{store: s, now: time.Now(), actor: p, local: objects, ctx: ctx}
	t.ctx = context.WithValue(ctx, localContextKey{}, t)
	err = s.mutate(t, credential, op, scope, fn)
	var fault Fault
	if err != nil && !errors.As(err, &fault) {
		return err
	}
	result := "ok"
	if err != nil {
		objects = l.objects
		result = string(fault)
	}
	audits := append(append([]localAudit(nil), l.audits...), localAudit{p.ID, p.Role, op, scope, reason, result, t.now})
	if saveErr := l.save(objects, audits); saveErr != nil {
		return saveErr
	}
	return err
}

// mutate is shared by the PostgreSQL and local transaction boundaries.
func (s *Store) mutate(t *transaction, credential, op, scope string, fn func(*transaction) error) error {
	revoked, err := t.quarantined(t.actor.ID)
	if err != nil {
		return err
	}
	if !permission(t.actor, op, scope) || revoked {
		return Denied
	}
	if err = fn(t); err != nil {
		return err
	}
	current, err := s.identity.Resolve(contextFor(t), credential)
	if err != nil || current.ID != t.actor.ID || !permission(current, op, scope) || !s.identity.Active(contextFor(t), t.actor) {
		return Denied
	}
	return nil
}
func (s *Store) readObject(ctx context.Context, kind, k string, out any) error {
	if t, ok := ctx.Value(localContextKey{}).(*transaction); ok && t.store == s {
		return t.get(kind, k, out)
	}
	if s.local != nil {
		l := s.local
		l.mu.Lock()
		defer l.mu.Unlock()
		if l.closed {
			return Unavailable
		}
		raw, ok := l.objects[kind+"/"+k]
		if !ok {
			return Missing
		}
		return json.Unmarshal(raw, out)
	}
	var raw []byte
	if err := s.pool.QueryRow(ctx, "SELECT body FROM "+s.schema+".objects WHERE kind=$1 AND id=$2", kind, k).Scan(&raw); err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}
func (t *transaction) revokeIdentity(identity string) error {
	if t.local != nil {
		authority, ok := t.store.identity.(interface {
			RevokeIdentity(context.Context, string) error
		})
		if !ok {
			return Unavailable
		}
		return authority.RevokeIdentity(contextFor(t), identity)
	}
	_, err := t.Exec(contextFor(t), "UPDATE "+t.store.schema+".authority_credentials SET revoked_at=clock_timestamp() WHERE actor_id::text=$1 AND revoked_at IS NULL", identity)
	return err
}
