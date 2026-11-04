package control

import (
	"context"
	"crypto/sha256"
	"reflect"
	"sync"
	"time"
)

// LocalCredentialAuthority separates the setup writer from a runtime reader.
// It is an in-memory credential store for explicitly selected local workflows;
// NewRuntime continues to require the independent PostgreSQL V2 owner views.
type LocalCredentialAuthority struct {
	mu          sync.Mutex
	credentials map[[32]byte]localCredential
}
type localCredential struct {
	Principal Principal
	Expires   time.Time
	Revoked   bool
}
type LocalCredentialReader struct{ authority *LocalCredentialAuthority }

func NewLocalCredentialAuthority() *LocalCredentialAuthority {
	return &LocalCredentialAuthority{credentials: map[[32]byte]localCredential{}}
}
func (a *LocalCredentialAuthority) Reader() *LocalCredentialReader { return &LocalCredentialReader{a} }
func (a *LocalCredentialAuthority) Grant(credential string, p Principal, expires time.Time) error {
	if len(credential) < 32 || len(credential) > 4096 || !uuidPattern.MatchString(p.ID) || !uuidPattern.MatchString(p.CredentialID) || !expires.After(time.Now()) || len(p.Scopes) == 0 || len(p.Scopes) > 32 {
		return Invalid
	}
	validRole := false
	for _, role := range []string{"user", "infrastructure", "finance", "moderator", "support"} {
		if p.Role == role {
			validRole = true
		}
	}
	if !validRole || (p.Role == "user" && !uuidPattern.MatchString(p.SessionID)) {
		return Invalid
	}
	for _, scope := range p.Scopes {
		if !key(scope) {
			return Invalid
		}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	hash := sha256.Sum256([]byte(credential))
	if _, ok := a.credentials[hash]; ok {
		return Conflict
	}
	p.Scopes = append([]string(nil), p.Scopes...)
	a.credentials[hash] = localCredential{Principal: p, Expires: expires}
	return nil
}
func (r *LocalCredentialReader) Resolve(ctx context.Context, credential string) (Principal, error) {
	if ctx.Err() != nil || len(credential) < 32 || len(credential) > 4096 {
		return Principal{}, Denied
	}
	a := r.authority
	a.mu.Lock()
	defer a.mu.Unlock()
	entry, ok := a.credentials[sha256.Sum256([]byte(credential))]
	if !ok || entry.Revoked || !entry.Expires.After(time.Now()) {
		return Principal{}, Denied
	}
	p := entry.Principal
	p.Scopes = append([]string(nil), p.Scopes...)
	return p, nil
}
func (r *LocalCredentialReader) Active(ctx context.Context, p Principal) bool {
	if ctx.Err() != nil {
		return false
	}
	a := r.authority
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, entry := range a.credentials {
		if !entry.Revoked && entry.Expires.After(time.Now()) && reflect.DeepEqual(entry.Principal, p) {
			return true
		}
	}
	return false
}
func (r *LocalCredentialReader) RevokeIdentity(ctx context.Context, identity string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	a := r.authority
	a.mu.Lock()
	defer a.mu.Unlock()
	for key, entry := range a.credentials {
		if entry.Principal.ID == identity {
			entry.Revoked = true
			a.credentials[key] = entry
		}
	}
	return nil
}
