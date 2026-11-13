package internaltest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
)

type ChainFixture struct{ RPC, Pool, Token, Admin, Alice, Bob string }

func NewAnvilFixture(t *testing.T, ctx context.Context, root string) (*rpc.Client, ChainFixture) {
	t.Helper()
	lock, err := os.OpenFile(filepath.Join(os.TempDir(), "pnl-tracker-anvil-21800.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = syscall.Flock(int(lock.Fd()), syscall.LOCK_UN); _ = lock.Close() })
	for {
		err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			break
		}
		if err != syscall.EWOULDBLOCK {
			t.Fatal(err)
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(10 * time.Millisecond):
		}
	}
	listener, err := net.Listen("tcp", "127.0.0.1:21800")
	if err != nil {
		t.Fatalf("owned Anvil port occupied; refusing to reuse a peer: %v", err)
	}
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	_, err = exec.LookPath("anvil")
	if err != nil {
		t.Fatalf("services/launchpad/pnl-tracker/internal/production/l18/fixture.cjs requires a local Anvil executable: %v", err)
	}
	// The existing fixture requires this exact loopback endpoint and verifies
	// the PID, start time, and socket ownership before every chain mutation.
	cmd := exec.CommandContext(ctx, "anvil", "--host", "127.0.0.1", "--port", "21800", "--chain-id", "125", "--silent")
	log, err := os.CreateTemp(t.TempDir(), "anvil-*.log")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = log.Close() }()
	cmd.Stdout, cmd.Stderr = log, log
	if err := cmd.Start(); err != nil {
		t.Fatalf("real Anvil required on PATH: %v", err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	stat, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", cmd.Process.Pid))
	if err != nil {
		t.Fatal(err)
	}
	fields := strings.Fields(string(stat[strings.LastIndexByte(string(stat), ')')+1:]))
	identity, _ := json.Marshal(map[string]any{"pid": cmd.Process.Pid, "starttime": fields[19]})
	t.Setenv("L18_ANVIL_IDENTITY", string(identity))
	r, err := rpc.DialContext(ctx, "http://127.0.0.1:21800")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(r.Close)
	ready := false
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); {
		var chain hexutil.Uint64
		if err := r.CallContext(ctx, &chain, "eth_chainId"); err == nil && chain == 125 {
			ready = true
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !ready {
		t.Fatal("owned real chain did not become ready")
	}
	fixture := exec.CommandContext(ctx, "node", "services/launchpad/pnl-tracker/internal/production/l18/fixture.cjs")
	fixture.Dir = root
	fixture.Env = append(os.Environ(), "L18_ANVIL_IDENTITY="+string(identity))
	var stderr bytes.Buffer
	fixture.Stderr = &stderr
	raw, err := fixture.Output()
	if err != nil {
		t.Fatalf("actual contract fixture: %v: %s", err, stderr.String())
	}
	var result ChainFixture
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	result.Alice, result.Bob, result.Pool = strings.ToLower(result.Alice), strings.ToLower(result.Bob), strings.ToLower(result.Pool)
	return r, result
}
