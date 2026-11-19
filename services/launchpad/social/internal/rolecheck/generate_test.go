package rolecheck

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
)

func TestStatementLocationsPreserveAndRelocateRuntimeSQL(t *testing.T) {
	root := statementGeneratorRoot(t)
	path := filepath.Join(root, "services/launchpad/social/internal/rolecheck/statements.json")
	original := readGeneratorStatements(t, path)
	const selection = "services/launchpad/social/internal/app/app.go:socialConnectedRuntime.publicResource"
	if err := WriteStatementLocations(root, selection); err != nil {
		t.Fatal(err)
	}
	if got := readGeneratorStatements(t, path); !reflect.DeepEqual(got, original) {
		t.Fatal("refresh changed the runtime SQL inventory")
	}
	index := -1
	for i, statement := range original {
		if statement.Function == "socialConnectedRuntime.publicResource" {
			index = i
			break
		}
	}
	if index < 0 {
		t.Fatal("public resource SELECT absent from inventory")
	}
	relocated := append([]Statement(nil), original...)
	relocated[index].Function = "socialConnectedRuntime.previousPublicResource"
	relocated[index].Ordinal = 2
	relocated[index].ID = "app/app.go:socialConnectedRuntime.previousPublicResource:2"
	writeGeneratorStatements(t, path, relocated)
	if err := WriteStatementLocations(root); err != nil {
		t.Fatal(err)
	}
	if got := readGeneratorStatements(t, path); !reflect.DeepEqual(got, original) {
		t.Fatal("relocation changed SQL or its dependency description")
	}
}

func TestStatementLocationsRejectInvalidInventoryWithoutWriting(t *testing.T) {
	root := statementGeneratorRoot(t)
	path := filepath.Join(root, "services/launchpad/social/internal/rolecheck/statements.json")
	original := readGeneratorStatements(t, path)
	for _, tc := range []struct {
		name       string
		selection  string
		statements func() []Statement
		want       string
	}{
		{"selection format", "invalid", func() []Statement { return original }, "invalid source selection"},
		{"selection file", "services/launchpad/social/internal/app/absent.go:missing", func() []Statement { return original }, "no such file"},
		{"selection function", "services/launchpad/social/internal/app/app.go:missing", func() []Statement { return original }, "function absent"},
		{"duplicate identity", "", func() []Statement {
			return append(append([]Statement(nil), original...), original[0])
		}, "duplicate statement identity"},
		{"SQL mismatch", "", func() []Statement {
			values := append([]Statement(nil), original...)
			values[0].SQL = "SELECT 1 AS absent_runtime_statement"
			return values
		}, "expected one SQL location, found 0"},
		{"source file", "", func() []Statement {
			values := append([]Statement(nil), original...)
			values[0].File = "services/launchpad/social/internal/app/absent.go"
			return values
		}, "no such file"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			writeGeneratorStatements(t, path, tc.statements())
			before, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			selections := []string{}
			if tc.selection != "" {
				selections = append(selections, tc.selection)
			}
			if err := WriteStatementLocations(root, selections...); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("inventory rejection = %v; want %s", err, tc.want)
			}
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(after) != string(before) {
				t.Fatal("rejected inventory was overwritten")
			}
		})
	}
}

func statementGeneratorRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	repository := coordinatortest.RepositoryRoot(t)
	for _, module := range []string{"services/launchpad/social", "sdk/launchpad/shared"} {
		destination := filepath.Join(root, module)
		if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.CopyFS(destination, os.DirFS(filepath.Join(repository, module))); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func readGeneratorStatements(t *testing.T, path string) []Statement {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var statements []Statement
	if err := json.Unmarshal(raw, &statements); err != nil {
		t.Fatal(err)
	}
	return statements
}

func writeGeneratorStatements(t *testing.T, path string, statements []Statement) {
	t.Helper()
	raw, err := json.Marshal(statements)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
}
