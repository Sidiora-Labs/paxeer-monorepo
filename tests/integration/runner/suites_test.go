package runner

import (
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

func TestSuitesCoverEveryModuleYAML(t *testing.T) {
	names := make(map[string]bool)
	registered := make(map[string]string)
	for _, suite := range Suites {
		if !strings.HasPrefix(suite.Name, "Test") || len(suite.Name) == len("Test") {
			t.Errorf("invalid Go test name %q", suite.Name)
		}
		if names[suite.Name] {
			t.Errorf("duplicate suite name %q", suite.Name)
		}
		names[suite.Name] = true
		if len(suite.Files) == 0 {
			t.Errorf("suite %q has no YAML files", suite.Name)
		}
		for _, file := range suite.Files {
			if !fs.ValidPath(file) || filepath.Ext(file) != ".yaml" {
				t.Errorf("suite %q has invalid YAML path %q", suite.Name, file)
			}
			if previous, ok := registered[file]; ok {
				t.Errorf("YAML %q is registered more than once: %q and %q", file, previous, suite.Name)
			}
			registered[file] = suite.Name
			if reason, ok := ExcludedSuites[file]; ok {
				t.Errorf("YAML %q is both registered in %q and excluded: %s", file, suite.Name, reason)
			}
		}
	}
	for file, reason := range ExcludedSuites {
		if !fs.ValidPath(file) || filepath.Ext(file) != ".yaml" {
			t.Errorf("exclusion has invalid YAML path %q", file)
		}
		if strings.TrimSpace(reason) == "" {
			t.Errorf("excluded YAML %q has no reason", file)
		}
	}

	discovered := make(map[string]bool)
	err := filepath.WalkDir("..", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".yaml" {
			return nil
		}
		relative, err := filepath.Rel("..", path)
		if err != nil {
			return err
		}
		file := filepath.ToSlash(relative)
		discovered[file] = true
		_, wired := registered[file]
		_, excluded := ExcludedSuites[file]
		if !wired && !excluded {
			t.Errorf("YAML %q must be registered in Suites or have a reason in ExcludedSuites", file)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk integration YAML files: %v", err)
	}
	if len(discovered) == 0 {
		t.Fatal("no integration YAML files found")
	}
	for file, suite := range registered {
		if !discovered[file] {
			t.Errorf("suite %q references missing YAML %q", suite, file)
		}
	}
	for file := range ExcludedSuites {
		if !discovered[file] {
			t.Errorf("exclusion references missing YAML %q", file)
		}
	}
}
