//go:build cgo

package lxphost

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGuarantorHeaderIncludersCompile(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}
	for _, header := range []string{"lxp_guarantor.h", "lxp_paxeer.h", "lx_batch.h"} {
		t.Run(header, func(t *testing.T) {
			path := filepath.Join(root, "include", "layerx", header)
			if _, err := os.Lstat(path); !os.IsNotExist(err) {
				t.Fatalf("retired header must be absent: %s (stat error: %v)", path, err)
			}
		})
	}
	for _, header := range []string{"lxp_tools.h", "lxp_bridge.h", "lxp_verify.h"} {
		t.Run(header, func(t *testing.T) {
			cmd := exec.Command("cc", "-std=c17", "-fsyntax-only", "-Iinclude", "-Ibuild/generated", "-x", "c", "-")
			cmd.Dir = root
			cmd.Stdin = strings.NewReader("#include \"layerx/" + header + "\"\n")
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("compile %s alone: %v\n%s", header, err, output)
			}
		})
	}
}
