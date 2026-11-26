package l16

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"
)

var localCanonicalProgramsDir string

var localCanonicalProducer, localCanonicalIdentityProgram localCanonicalProgram

type localCanonicalProgram struct {
	once   sync.Once
	binary string
	err    error
}

func TestMain(m *testing.M) {
	var err error
	localCanonicalProgramsDir, err = os.MkdirTemp("", "pnl-canonical-programs-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	code := m.Run()
	if err = os.RemoveAll(localCanonicalProgramsDir); err != nil {
		fmt.Fprintln(os.Stderr, err)
		code = 1
	}
	os.Exit(code)
}

func (p *localCanonicalProgram) load(t *testing.T, name string, build func() (string, error)) string {
	t.Helper()
	p.once.Do(func() {
		started := time.Now()
		p.binary, p.err = build()
		t.Logf("%s build: %s", name, time.Since(started))
	})
	if p.err != nil {
		t.Fatalf("%s build: %v", name, p.err)
	}
	return p.binary
}

func buildLocalCanonicalProgram(ctx context.Context, dir string, env []string, args ...string) error {
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir, cmd.Env = dir, env
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%w\n%s", err, output)
	}
	return nil
}

func buildLocalCanonicalProducer(ctx context.Context, root string, env []string) (string, error) {
	dir, err := os.MkdirTemp(localCanonicalProgramsDir, "producer-")
	if err != nil {
		return "", err
	}
	source, err := filepath.Abs("testdata/local_canonical/indexer_test.go.overlay")
	if err != nil {
		return "", err
	}
	indexer := filepath.Join(root, "services/launchpad/indexer")
	overlay, err := json.Marshal(struct{ Replace map[string]string }{map[string]string{filepath.Join(indexer, "internal/processor/pnl_canonical_fixture_test.go"): source}})
	if err != nil {
		return "", err
	}
	overlayPath := filepath.Join(dir, "overlay.json")
	if err = os.WriteFile(overlayPath, overlay, 0600); err != nil {
		return "", err
	}
	binary := filepath.Join(dir, "indexer.test")
	if err = buildLocalCanonicalProgram(ctx, indexer, env, "test", "-c", "-overlay", overlayPath, "-o", binary, "./internal/processor"); err != nil {
		return "", err
	}
	return binary, nil
}

func buildLocalCanonicalIdentity(ctx context.Context, root string, env []string) (string, error) {
	source := filepath.Join(root, "services/launchpad/gateway")
	dir, err := os.MkdirTemp(localCanonicalProgramsDir, "identity-")
	if err != nil {
		return "", err
	}
	module, err := os.ReadFile(filepath.Join(source, "go.mod"))
	if err != nil {
		return "", err
	}
	const gateway = "github.com/Sidiora-Technologies/KindleLaunch/media/gateway"
	mod := strings.Replace(string(module), "module "+gateway, "module "+gateway+"/pnlfixture", 1)
	relative := regexp.MustCompile(`(?m)(=>\s+)(\.[^\s]+)`)
	mod = relative.ReplaceAllStringFunc(mod, func(line string) string {
		parts := relative.FindStringSubmatch(line)
		return parts[1] + filepath.Clean(filepath.Join(source, parts[2]))
	})
	mod += "\nrequire " + gateway + " v0.0.0\nreplace " + gateway + " => " + source + "\n"
	if err = os.WriteFile(filepath.Join(dir, "go.mod"), []byte(mod), 0600); err != nil {
		return "", err
	}
	sum, err := os.ReadFile(filepath.Join(source, "go.sum"))
	if err != nil {
		return "", err
	}
	if err = os.WriteFile(filepath.Join(dir, "go.sum"), sum, 0600); err != nil {
		return "", err
	}
	raw, err := os.ReadFile("testdata/local_canonical/identity.go.overlay")
	if err != nil {
		return "", err
	}
	if err = os.WriteFile(filepath.Join(dir, "main.go"), raw, 0600); err != nil {
		return "", err
	}
	binary := filepath.Join(dir, "identity")
	if err = buildLocalCanonicalProgram(ctx, dir, env, "build", "-mod=mod", "-o", binary, "."); err != nil {
		return "", err
	}
	return binary, nil
}
