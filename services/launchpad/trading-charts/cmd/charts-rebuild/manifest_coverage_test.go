package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRebuildReadsBoundedManifestDocument(t *testing.T) {
	for _, tc := range []struct {
		name, document, expected string
	}{
		{"unknown field", `[{"unknown":true}]`, "unknown field"},
		{"invalid JSON", `[`, "unexpected EOF"},
		{"trailing JSON", `[] {}`, "trailing manifest data"},
		{"empty manifest", `[]`, "rebuild runtime schema readiness"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repair1052Environment(t, true)
			path := filepath.Join(t.TempDir(), "manifests.json")
			if err := os.WriteFile(path, []byte(tc.document), 0600); err != nil {
				t.Fatal(err)
			}
			err := runArgs(repair1052Args(path))
			if err == nil || !strings.Contains(err.Error(), tc.expected) {
				t.Fatalf("manifest %q: %v", tc.name, err)
			}
		})
	}

	t.Run("oversized manifest", func(t *testing.T) {
		repair1052Environment(t, true)
		path := filepath.Join(t.TempDir(), "manifests.json")
		if err := os.WriteFile(path, []byte(strings.Repeat(" ", 16<<20+1)), 0600); err != nil {
			t.Fatal(err)
		}
		if err := runArgs(repair1052Args(path)); err == nil || !strings.Contains(err.Error(), "manifest file exceeds bound") {
			t.Fatalf("oversized manifest: %v", err)
		}
	})

	t.Run("missing manifest", func(t *testing.T) {
		repair1052Environment(t, true)
		path := filepath.Join(t.TempDir(), "missing.json")
		err := runArgs(repair1052Args(path))
		var pathError *os.PathError
		if !errors.As(err, &pathError) || pathError.Path != path {
			t.Fatalf("manifest path: %v", err)
		}
	})
}

func TestRebuildRunReadsProcessArguments(t *testing.T) {
	previous := os.Args
	os.Args = []string{"charts-rebuild"}
	t.Cleanup(func() { os.Args = previous })
	if err := run(); err == nil || !strings.Contains(err.Error(), "explicit approved manifests") {
		t.Fatalf("rebuild without approved inputs: %v", err)
	}
}
