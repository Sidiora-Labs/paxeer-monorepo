package api_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestBufLintWorkspace(t *testing.T) {
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("locate repository root: %v", err)
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}

	t.Run("lint", func(t *testing.T) {
		binDir := t.TempDir()
		install := exec.CommandContext(t.Context(), "go", "install", "github.com/bufbuild/buf/cmd/buf@v1.58.0")
		install.Dir = root
		install.Env = append(os.Environ(), "GOBIN="+binDir)
		if output, err := install.CombinedOutput(); err != nil {
			t.Fatalf("install pinned buf CLI: %v\n%s", err, output)
		} else if len(output) != 0 {
			t.Logf("install pinned buf CLI:\n%s", output)
		}
		cmd := exec.CommandContext(t.Context(), filepath.Join(binDir, "buf"), "lint")
		cmd.Dir = root
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("buf lint: %v\n%s", err, output)
		}
		if len(output) != 0 {
			t.Errorf("buf lint: expected empty output, got:\n%s", output)
		}
	})

	t.Run("ignore_directories", func(t *testing.T) {
		data, err := os.ReadFile(filepath.Join(root, "buf.yaml"))
		if err != nil {
			t.Fatalf("read buf.yaml: %v", err)
		}
		var config struct {
			Lint struct {
				Ignore []string `yaml:"ignore"`
			} `yaml:"lint"`
		}
		if err := yaml.Unmarshal(data, &config); err != nil {
			t.Fatalf("parse buf.yaml: %v", err)
		}
		for _, ignore := range config.Lint.Ignore {
			if !filepath.IsLocal(ignore) {
				t.Errorf("lint.ignore %q must be a path under the repository root", ignore)
				continue
			}
			path, err := filepath.EvalSymlinks(filepath.Join(root, ignore))
			if err != nil {
				t.Errorf("resolve lint.ignore %q: %v", ignore, err)
				continue
			}
			relative, err := filepath.Rel(root, path)
			if err != nil || !filepath.IsLocal(relative) {
				t.Errorf("lint.ignore %q resolves outside the repository root: %s (%v)", ignore, path, err)
				continue
			}
			info, err := os.Stat(path)
			if err != nil {
				t.Errorf("stat lint.ignore %q: %v", ignore, err)
				continue
			}
			if !info.IsDir() {
				t.Errorf("lint.ignore %q must name an existing directory", ignore)
			}
		}
	})
}
