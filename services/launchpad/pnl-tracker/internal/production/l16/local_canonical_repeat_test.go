package l16

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
)

func TestLocalCanonicalFixtureRepeatsARefusedRange(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	root := coordinatortest.RepositoryRoot(t)
	env := append(os.Environ(), "GOWORK=off")
	binary := localCanonicalProducer.load(t, "canonical producer", func() (string, error) {
		return buildLocalCanonicalProducer(ctx, root, env)
	})
	cmd := exec.CommandContext(ctx, binary, "-test.count=1", "-test.v", "-test.run=^TestPnlCanonicalFixtureRepeatsARefusedRange$")
	cmd.Dir, cmd.Env = filepath.Join(root, "services/launchpad/indexer/internal/processor"), env
	started := time.Now()
	output, err := cmd.CombinedOutput()
	t.Logf("canonical range repeat case run: %s\n%s", time.Since(started), output)
	if err != nil {
		t.Fatalf("canonical range repeat case: %v", err)
	}
	for _, line := range strings.Split(string(output), "\n") {
		if strings.HasPrefix(line, "--- PASS: TestPnlCanonicalFixtureRepeatsARefusedRange (") {
			return
		}
	}
	t.Fatal("canonical range repeat case pass line missing")
}
