package holdings

import (
	"context"
	"os/exec"
	"testing"
	"time"
)

func TestPhase4CommandCancellationStopsProcessGroup(t *testing.T) {
	shell, err := exec.LookPath("sh")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	command, stop := phase4Command(ctx, true, shell, "-c", "sleep 10")
	defer stop()
	if err := command.Start(); err != nil {
		cancel()
		t.Fatal(err)
	}
	started := time.Now()
	cancel()
	if err := command.Wait(); err == nil {
		t.Fatal("cancelled command completed normally")
	}
	if elapsed := time.Since(started); elapsed > 2*time.Second {
		t.Fatalf("cancelled process group remained active for %s", elapsed)
	}
}
