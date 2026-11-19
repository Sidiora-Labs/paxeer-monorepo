package rolecheck

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
)

func TestRegisteredSchemasUsesRepositoryRegistry(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join(coordinatortest.RepositoryRoot(t), "migrations/registry.json"))
	if err != nil {
		t.Fatal(err)
	}
	var registry struct {
		Registrations []struct {
			Service string `json:"service"`
			Schema  string `json:"schema"`
		} `json:"registrations"`
	}
	if err := json.Unmarshal(raw, &registry); err != nil {
		t.Fatal(err)
	}
	want := make(map[string]string)
	for _, registration := range registry.Registrations {
		want[registration.Service] = registration.Schema
	}
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "migrations"), 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "migrations/registry.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "registry.json"), []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := registeredSchemas(root)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("registered schemas = %v, %v; want %v", got, err, want)
	}
	for _, tc := range []struct {
		name, old, replacement, message string
	}{
		{"conflicting schemas", `"registrations": [`, `"registrations": [{"service":"media/livestream","schema":"other"},`, "service media/livestream has multiple schemas"},
		{"missing runtime service", `"media/livestream"`, `"media/other"`, "runtime schema binding differs for media/livestream"},
		{"different runtime schema", `"livestream"`, `"other"`, "runtime schema binding differs for media/livestream"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			changed := strings.ReplaceAll(string(raw), tc.old, tc.replacement)
			if changed == string(raw) {
				t.Fatal("registry must contain the selected binding")
			}
			if err := os.WriteFile(path, []byte(changed), 0600); err != nil {
				t.Fatal(err)
			}
			got, err := registeredSchemas(root)
			if got != nil || err == nil || err.Error() != tc.message {
				t.Fatalf("invalid registry = %v, %v; want %s", got, err, tc.message)
			}
		})
	}
}

func TestRegisteredSchemasReturnsReadAndDecodeErrors(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "migrations/registry.json")
	got, err := registeredSchemas(root)
	var pathErr *os.PathError
	if got != nil || !errors.Is(err, os.ErrNotExist) || !errors.As(err, &pathErr) || pathErr.Path != path {
		t.Fatalf("missing registry = %v, %v; want missing file %s", got, err, path)
	}
	if err := os.Mkdir(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"registrations":`), 0600); err != nil {
		t.Fatal(err)
	}
	got, err = registeredSchemas(root)
	var syntaxErr *json.SyntaxError
	if got != nil || !errors.As(err, &syntaxErr) {
		t.Fatalf("malformed registry = %v, %v; want JSON syntax error", got, err)
	}
}
