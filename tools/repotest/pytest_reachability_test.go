package repotest_test

import (
	"io/fs"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestEveryPytestSuiteIsReferenced(t *testing.T) {
	buildPaths := []string{"Makefile", "chain.mk"}
	workflows, err := filepath.Glob(".github/workflows/*.yml")
	if err != nil {
		t.Fatalf("list workflows: %v", err)
	}
	buildPaths = append(buildPaths, workflows...)
	for _, root := range []string{"tools", "scripts"} {
		buildPaths = append(buildPaths, reachabilityFiles(t, root, ".sh", ".py", ".mk")...)
	}
	continuation := regexp.MustCompile(`\\\r?\n[ \t]*`)
	buildSources := make([]string, 0, len(buildPaths))
	for _, path := range buildPaths {
		buildSources = append(buildSources, continuation.ReplaceAllString(string(reachabilityRead(t, path)), " "))
	}

	var suites []string
	err = filepath.WalkDir("tests", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		path = filepath.ToSlash(path)
		if entry.IsDir() {
			if path == "tests/bridge" || path == "tests/daemon" || (filepath.Dir(path) == "tests" && strings.HasPrefix(entry.Name(), "explorer")) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(entry.Name(), "test_") && strings.HasSuffix(entry.Name(), ".py") {
			suites = append(suites, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scan pytest suites: %v", err)
	}
	if len(suites) == 0 {
		t.Fatal("no pytest suites found under tests/")
	}

	for _, path := range suites {
		fileReference := regexp.MustCompile(`(^|[^A-Za-z0-9_./-])` + regexp.QuoteMeta(path) + `($|[^A-Za-z0-9_./-])`)
		var directoryReferences []*regexp.Regexp
		for dir := filepath.ToSlash(filepath.Dir(path)); dir != "."; dir = filepath.ToSlash(filepath.Dir(dir)) {
			directoryReferences = append(directoryReferences, regexp.MustCompile(
				`(?:^|[^A-Za-z0-9_./-])pytest["']?(?:[ \t,][^\r\n;&|]*)?[ \t,]["']?(?:\./)?`+
					regexp.QuoteMeta(dir)+`/?["']?(?:$|[ \t\r\n,\])])`))
			directoryReferences = append(directoryReferences, regexp.MustCompile(
				`(?:^|[^A-Za-z0-9_./-])python3[ \t]+-m[ \t]+unittest[ \t]+discover(?:[ \t]+[^\r\n;&|]*)?[ \t]+-s[ \t]+["']?(?:\./)?`+
					regexp.QuoteMeta(dir)+`/?["']?(?:$|[ \t\r\n,\])])`))
		}
		referenced := false
		for _, source := range buildSources {
			if fileReference.MatchString(source) {
				referenced = true
				break
			}
			for _, directoryReference := range directoryReferences {
				if directoryReference.MatchString(source) {
					referenced = true
					break
				}
			}
			if referenced {
				break
			}
		}
		if !referenced {
			t.Errorf("%s has no exact path reference or containing directory passed to pytest or python3 -m unittest discover -s in Makefile, chain.mk, .github/workflows/*.yml, tools/ or scripts/", path)
		}
	}
}
