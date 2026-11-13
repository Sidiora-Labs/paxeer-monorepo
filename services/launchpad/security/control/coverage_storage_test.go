package control

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLocalStoreSnapshotAndReadAuthority(t *testing.T) {
	authority := NewLocalCredentialAuthority()
	credential := strings.Repeat("l", 32)
	principal := Principal{ID: id(), CredentialID: id(), Role: "infrastructure", Scopes: []string{"platform", "service"}}
	if err := authority.Grant(credential, principal, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "control.json")
	if _, err := OpenLocalStore("", authority.Reader(), nil); !errors.Is(err, Invalid) {
		t.Fatalf("missing local path: %v", err)
	}
	if _, err := OpenLocalStore(path, nil, nil); !errors.Is(err, Invalid) {
		t.Fatalf("missing identity authority: %v", err)
	}
	if err := os.WriteFile(path, []byte("not-json"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenLocalStore(path, authority.Reader(), nil); err == nil {
		t.Fatal("malformed snapshot admitted")
	}
	if err := os.WriteFile(path, []byte(`{"Objects":null}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenLocalStore(path, authority.Reader(), nil); !errors.Is(err, Invalid) {
		t.Fatalf("missing snapshot objects: %v", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	store, err := OpenLocalStore(path, authority.Reader(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := OpenLocalStore(path, authority.Reader(), nil); err == nil {
		t.Fatal("concurrent store writer admitted")
	}
	if err := store.Install(context.Background()); !errors.Is(err, Invalid) {
		t.Fatalf("local migration: %v", err)
	}
	cap := Cap{Capability: "quote", Version: 1, Limit: 2, Effective: time.Now().Add(-time.Minute), Expires: time.Now().Add(time.Hour), Revision: 1}
	wire, err := json.Marshal(cap)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.local.save(map[string]json.RawMessage{controlCaps + "/quote": wire}, nil); err != nil {
		t.Fatal(err)
	}
	var loaded Cap
	if err := store.Read(context.Background(), credential, controlPlatform, "read capability", controlCaps, "quote", &loaded); err != nil || loaded.Limit != 2 {
		t.Fatalf("capability read: %+v, %v", loaded, err)
	}
	for _, sample := range []struct {
		name, scope, kind, key string
		out                    any
		want                   error
	}{
		{"invalid key", controlPlatform, controlCaps, "bad key", &loaded, Invalid},
		{"nil output", controlPlatform, controlCaps, "quote", nil, Invalid},
		{"nonpointer output", controlPlatform, controlCaps, "quote", loaded, Invalid},
		{"unlisted object", controlPlatform, "private", "quote", &loaded, Denied},
		{"other scope", "service", controlCaps, "quote", &loaded, Denied},
		{"missing capability", controlPlatform, controlCaps, "missing", &loaded, Missing},
	} {
		if err := store.Read(context.Background(), credential, sample.scope, "inspect local object", sample.kind, sample.key, sample.out); !errors.Is(err, sample.want) {
			t.Fatalf("%s: %v", sample.name, err)
		}
	}
	if err := store.CloseLocal(); err != nil {
		t.Fatal(err)
	}
	if err := store.CloseLocal(); !errors.Is(err, Invalid) {
		t.Fatalf("repeat close: %v", err)
	}
	if err := store.Read(context.Background(), credential, controlPlatform, "read closed store", controlCaps, "quote", &loaded); !errors.Is(err, Unavailable) {
		t.Fatalf("closed local read: %v", err)
	}
	reopened, err := OpenLocalStore(path, authority.Reader(), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := reopened.CloseLocal(); err != nil {
			t.Error(err)
		}
	})
	if err := reopened.Read(context.Background(), credential, controlPlatform, "read durable capability", controlCaps, "quote", &loaded); err != nil || loaded.Limit != cap.Limit {
		t.Fatalf("reopened capability: %+v, %v", loaded, err)
	}
}
