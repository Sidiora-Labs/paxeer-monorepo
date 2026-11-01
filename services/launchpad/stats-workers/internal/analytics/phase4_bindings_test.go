package analytics_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestPhase4CandidateSourcesExistAtRepositoryRoot(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("current source location unavailable")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "../../../../.."))
	if len(phase4CandidateSources) == 0 {
		t.Fatal("candidate sources must not be empty")
	}
	for _, path := range phase4CandidateSources {
		t.Run(path, func(t *testing.T) {
			info, err := os.Stat(filepath.Join(root, path))
			if err != nil {
				t.Fatalf("candidate source %q at repository root: %v", path, err)
			}
			if !info.Mode().IsRegular() {
				t.Fatalf("candidate source %q is not a regular file", path)
			}
		})
	}
}
