package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
	"github.com/jackc/pgx/v5/pgconn"
)

func editBundleDocument(t *testing.T, directory, name string, edit func(map[string]any)) {
	t.Helper()
	path := filepath.Join(directory, name)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	edit(document)
	raw, err = json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, path, raw)
}

func TestBundleAdmissionRequiresCompleteReviewedDocuments(t *testing.T) {
	for _, name := range []string{"registry.json", "grants.json", "legacy-history-aliases.json"} {
		for _, mode := range []string{"missing", "malformed"} {
			t.Run(name+"/"+mode, func(t *testing.T) {
				directory, _ := approvedBundle(t, "admission", false, "")
				path := filepath.Join(directory, name)
				want := name + " is unavailable"
				if mode == "missing" {
					if err := os.Remove(path); err != nil {
						t.Fatal(err)
					}
				} else {
					writeTestFile(t, path, []byte("{"))
					want = name + " is invalid"
				}
				if _, err := loadBundle(directory); err == nil || err.Error() != want {
					t.Fatalf("document admission: %v, want %s", err, want)
				}
			})
		}
	}
	for _, tc := range []struct {
		name, file, want string
		approval, legacy bool
		edit             func(map[string]any)
	}{
		{"registry shape", "registry.json", "registry.json requires an approved digest and complete dependency order", true, false, func(d map[string]any) { d["schema_version"] = 1 }},
		{"duplicate registration", "registry.json", "registry.json requires an approved digest and complete dependency order", true, false, func(d map[string]any) { a := d["registrations"].([]any); a[1] = a[0] }},
		{"duplicate application", "registry.json", "registry.json requires an approved digest and complete dependency order", true, false, func(d map[string]any) { a := d["application_order"].([]any); a[1] = a[0] }},
		{"missing predecessor", "registry.json", "registry.json requires an approved digest and complete dependency order", true, false, func(d map[string]any) {
			d["registrations"].([]any)[0].(map[string]any)["predecessor_ids"] = []any{999999}
		}},
		{"unapproved registry", "registry.json", "registry.json requires an approved digest and complete dependency order", false, false, func(d map[string]any) { d["approval_digest"] = "invalid" }},
		{"approval binding", "registry.json", "registry.json approval digest differs from its registrations", false, false, func(d map[string]any) { d["state"] = "approved" }},
		{"grants version", "grants.json", "grants.json requires version 2 and runtime roles", false, false, func(d map[string]any) { d["schema_version"] = 1 }},
		{"grant authority", "grants.json", "grants.json contains invalid roles or ownership", false, false, func(d map[string]any) { d["roles"].([]any)[0].(map[string]any)["owner_role"] = "invalid role" }},
		{"legacy version", "legacy-history-aliases.json", "legacy aliases require version 2", false, false, func(d map[string]any) { d["schema_version"] = 1 }},
		{"legacy service", "legacy-history-aliases.json", "legacy history has no reviewed service and owner", false, true, func(d map[string]any) { d["histories"].([]any)[0].(map[string]any)["service"] = "unregistered" }},
		{"legacy order", "legacy-history-aliases.json", "legacy aliases require exact reviewed fingerprints and ordered versions", false, true, func(d map[string]any) {
			d["histories"].([]any)[0].(map[string]any)["sources"].([]any)[0].(map[string]any)["original_version"] = 0
		}},
		{"legacy source binding", "legacy-history-aliases.json", "legacy source is unavailable or its checksum differs", false, true, func(d map[string]any) {
			d["histories"].([]any)[0].(map[string]any)["sources"].([]any)[0].(map[string]any)["source_sha256"] = strings.Repeat("0", 64)
		}},
		{"legacy up binding", "legacy-history-aliases.json", "legacy up SQL checksum differs", false, true, func(d map[string]any) {
			d["histories"].([]any)[0].(map[string]any)["sources"].([]any)[0].(map[string]any)["up_sha256"] = strings.Repeat("0", 64)
		}},
		{"schema owner", "registry.json", "registered schema has no reviewed owner", true, false, func(d map[string]any) { d["registrations"].([]any)[0].(map[string]any)["schema"] = "unregistered" }},
		{"migration interface", "registry.json", "migration 440000 requires matching approval, interface, reader and safety metadata", false, false, func(d map[string]any) {
			d["registrations"].([]any)[0].(map[string]any)["interface_digest"] = strings.Repeat("0", 64)
		}},
		{"online plan required", "registry.json", "registered migration requires an exact online index plan", true, false, func(d map[string]any) {
			d["registrations"].([]any)[0].(map[string]any)["transaction_mode"] = "nontransactional"
		}},
		{"extension identifier", "registry.json", "invalid extension prerequisite", true, false, func(d map[string]any) {
			d["extension_prerequisites"] = []any{map[string]any{"extension": "invalid extension", "schema": "public"}}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			directory, _ := approvedBundle(t, "admission", tc.legacy, strings.Repeat("a", 64))
			editBundleDocument(t, directory, tc.file, tc.edit)
			if _, err := loadBundleMode(directory, tc.approval); err == nil || err.Error() != tc.want {
				t.Fatalf("bundle admission: %v, want %s", err, tc.want)
			}
		})
	}
}

func TestSchemaFailuresExposeOnlyStableReasons(t *testing.T) {
	for _, tc := range []struct {
		cause error
		want  string
	}{
		{&pgconn.PgError{Code: "23514", Message: "private-value"}, "operation (SQLSTATE 23514)"},
		{&pgconn.PgError{Code: "private-value", Message: "private-value"}, "operation"},
		{context.Canceled, "operation: operation cancelled"},
		{context.DeadlineExceeded, "operation: operation cancelled"},
		{production.ErrMigrationInProgress, "operation: migration in progress"},
	} {
		if err := failure("operation", tc.cause); err.Error() != tc.want {
			t.Fatalf("stable failure: %v", err)
		}
	}
	if err := readinessFailure(errors.New("private-value")); err.Error() != "database readiness refused" {
		t.Fatal(err)
	}
	for _, reason := range []string{"runtime role authority mismatch", "expected schema absent"} {
		if err := readinessFailure(errors.New(reason)); err.Error() != reason {
			t.Fatal(err)
		}
	}
}

func TestSchemaDocumentPublicationCleansUnpublishedFiles(t *testing.T) {
	directory := t.TempDir()
	blocked := filepath.Join(directory, "blocked")
	if err := os.Mkdir(blocked, 0700); err != nil {
		t.Fatal(err)
	}
	if err := writeDocument(filepath.Join(directory, "missing", "expected.json"), []byte("document")); err == nil || err.Error() != "expected schema output cannot be created" {
		t.Fatalf("missing directory: %v", err)
	}
	if err := writeDocument(blocked, []byte("document")); err == nil || err.Error() != "expected schema output publication failed" {
		t.Fatalf("directory destination: %v", err)
	}
	for _, mode := range []string{"create", "publish"} {
		t.Run(mode, func(t *testing.T) {
			first := filepath.Join(directory, "first.json")
			second := filepath.Join(directory, "missing", "second.json")
			want := "schema document output cannot be created"
			if mode == "publish" {
				first, second, want = blocked, filepath.Join(directory, "second.json"), "schema document publication failed"
			}
			err := writeDocumentsAtomically([]struct {
				path string
				raw  []byte
			}{{first, []byte("first")}, {second, []byte("second")}})
			if err == nil || err.Error() != want {
				t.Fatalf("publication: %v", err)
			}
			for _, path := range []string{filepath.Join(directory, "first.json"), filepath.Join(directory, "second.json")} {
				if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("unpublished document exists: %s: %v", path, err)
				}
			}
		})
	}
	files, err := os.ReadDir(directory)
	if err != nil || len(files) != 1 || files[0].Name() != "blocked" {
		t.Fatalf("temporary output retained: %v %v", files, err)
	}
}
