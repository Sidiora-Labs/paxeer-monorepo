package main

import (
	"bytes"
	"errors"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"testing"
)

func TestRepairM_1186_InputDir(t *testing.T) {
	fixtureRoot, err := filepath.Abs("testdata")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name     string
		fixture  string
		relative bool
	}{
		{name: "absolute", fixture: "tags"},
		{name: "relative", fixture: "commented", relative: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			workingDir := filepath.Join(root, "caller")
			targetDir := filepath.Join(root, "target")
			for _, directory := range []string{workingDir, targetDir} {
				if err := os.Mkdir(directory, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			for directory, fixture := range map[string]string{workingDir: "basic", targetDir: tc.fixture} {
				source, err := os.ReadFile(filepath.Join(fixtureRoot, fixture, "metrics.go"))
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(directory, "metrics.go"), source, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			golden, err := os.ReadFile(filepath.Join(fixtureRoot, tc.fixture, "metrics.gen.go"))
			if err != nil {
				t.Fatal(err)
			}

			t.Chdir(workingDir)
			inputDir := targetDir
			if tc.relative {
				inputDir, err = filepath.Rel(workingDir, targetDir)
				if err != nil {
					t.Fatal(err)
				}
			}
			savedArgs, savedDir, savedStruct := os.Args, *dir, *strct
			t.Cleanup(func() {
				os.Args, *dir, *strct = savedArgs, savedDir, savedStruct
			})
			os.Args = []string{"metricsgen", "-dir", inputDir, "-struct", "Metrics"}
			main()

			outputPath := filepath.Join(targetDir, "metrics.gen.go")
			generated, err := os.ReadFile(outputPath)
			if err != nil {
				t.Fatal(err)
			}
			parsed, err := parser.ParseFile(token.NewFileSet(), outputPath, generated, parser.AllErrors)
			if err != nil {
				t.Fatal(err)
			}
			if parsed.Name.Name != tc.fixture {
				t.Errorf("generated package = %q, want selected package %q", parsed.Name.Name, tc.fixture)
			}
			if !bytes.Equal(generated, golden) {
				t.Errorf("generated constructors do not match selected fixture %s:\n%s", tc.fixture, generated)
			}
			if _, err := os.Stat(filepath.Join(workingDir, "metrics.gen.go")); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("caller directory must have no generated output; stat error = %v", err)
			}
		})
	}
}
