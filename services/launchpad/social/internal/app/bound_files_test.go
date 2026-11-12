package app

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSocialBoundFilesAndIdentifiers(t *testing.T) {
	path := filepath.Join(t.TempDir(), "runtime.json")
	raw := []byte(`{"version":2}`)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(raw))
	loaded, err := socialBoundBytes(path, digest)
	if err != nil || string(loaded) != string(raw) {
		t.Fatalf("bound runtime file = %q: %v", loaded, err)
	}
	if _, err := socialBoundBytes(path, strings.Repeat("0", 64)); err == nil {
		t.Fatal("different digest accepted")
	}
	if _, err := socialBoundBytes(filepath.Join(t.TempDir(), "absent"), digest); err == nil {
		t.Fatal("missing bound file accepted")
	}
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readSocialBoundFile(path); err == nil {
		t.Fatal("empty bound file accepted")
	}
	identifier, err := socialIdentifier("chat", "interaction_comments")
	if err != nil || identifier.Sanitize() != `"chat"."interaction_comments"` {
		t.Fatalf("bound identifier = %v: %v", identifier, err)
	}
	for _, name := range []string{"", "Chat", "chat;drop", strings.Repeat("a", 64)} {
		if _, err := socialIdentifier(name, "interaction_comments"); err == nil {
			t.Fatalf("identifier %q accepted", name)
		}
	}
}
