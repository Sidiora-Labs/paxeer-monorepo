//go:build cgo

package lxphost

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestKernelHeadersCompileWithoutRetiredHeaders(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	includeDir := filepath.Join(root, "include", "layerx")
	for _, header := range []string{"lxp_da.h", "lxp_replica.h", "lxp_sequencer.h"} {
		t.Run(header, func(t *testing.T) {
			path := filepath.Join(includeDir, header)
			if _, err := os.Lstat(path); !os.IsNotExist(err) {
				t.Fatalf("retired header must be absent: %s (stat error: %v)", path, err)
			}
		})
	}
	headers, err := os.ReadDir(includeDir)
	if err != nil {
		t.Fatalf("list kernel headers: %v", err)
	}
	count := 0
	for _, entry := range headers {
		header := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(header, ".h") || header == "lxp_daemon.h" {
			continue
		}
		count++
		t.Run(header, func(t *testing.T) {
			cmd := exec.Command("cc", "-std=c17", "-fsyntax-only", "-Iinclude", "-Ibuild/generated", "-x", "c", "-")
			cmd.Dir = root
			cmd.Stdin = strings.NewReader("#include \"layerx/" + header + "\"\n")
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("compile %s alone: %v\n%s", header, err, output)
			}
		})
	}
	if count == 0 {
		t.Fatal("no kernel headers found")
	}
}
