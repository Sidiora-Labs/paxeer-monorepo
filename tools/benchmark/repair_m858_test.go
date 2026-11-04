package benchmark_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// Run the production start block unchanged. Initialization (and its build,
// Python and jq dependencies) is deliberately outside this launcher's proof.
func repairM858StartBlock(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("benchmark.sh")
	if err != nil {
		t.Fatal(err)
	}
	script := string(data)
	marker := `if [ "$BENCHMARK_PHASE" = "start" ] || [ "$BENCHMARK_PHASE" = "all" ]; then`
	if strings.Count(script, marker) != 1 {
		t.Fatal("production start block must be unique")
	}
	start := strings.Index(script, marker)
	if !strings.HasPrefix(script, "#!/bin/bash\n# require success for commands\nset -e\n") {
		t.Fatal("launcher must retain its production errexit setting")
	}
	return "set -e\n" + script[start:]
}

func repairM858Command(t *testing.T, home, peer, mode string) *exec.Cmd {
	t.Helper()
	fixture, err := filepath.Abs(filepath.Join("testdata", "m858", peer+".sh"))
	if err != nil {
		t.Fatal(err)
	}
	// The peer is a committed transcript interpreted by the same production
	// shell. No node binary or subprocess toolchain is built or invoked.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	cmd := exec.CommandContext(ctx, "/bin/bash", "--noprofile", "--norc", "-c", repairM858StartBlock(t))
	cmd.Env = []string{
		"PATH=", "LC_ALL=C", "BENCHMARK_PHASE=start", "PORT_OFFSET=0",
		"PAXD=" + fixture, "PAX_HOME=" + home,
		"BENCHMARK_CONFIG=fixture scenario with spaces.json", "BENCHMARK_TXS_PER_BATCH=19",
		"DEBUG=" + strconv.FormatBool(mode == "debug"), "LOG_FILE=",
	}
	if mode == "file" {
		cmd.Env[len(cmd.Env)-1] = "LOG_FILE=" + filepath.Join(home, "node.log")
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.WaitDelay = time.Second
	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
	})
	return cmd
}

func repairM858Status(t *testing.T, err error) int {
	t.Helper()
	if err == nil {
		return 0
	}
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("launcher failed without an exit status: %v", err)
	}
	return exit.ExitCode()
}

func repairM858AssertInvocation(t *testing.T, home string, pid int) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(home, "invocation"))
	want := "start\n--chain-id\npax-chain\n--home\n" + home + "\nfixture scenario with spaces.json\n19\n"
	if err != nil || string(data) != want {
		t.Fatalf("node invocation = %q, %v; want %q", data, err, want)
	}
	data, err = os.ReadFile(filepath.Join(home, "node.pid"))
	if err != nil || string(data) != fmt.Sprintln(pid) {
		t.Fatalf("node did not replace launcher pid %d: %q, %v", pid, data, err)
	}
}

func TestRepairM_858_NodeFailureAfterMatchingLine(t *testing.T) {
	for _, mode := range []string{"filtered", "debug", "file"} {
		t.Run(mode, func(t *testing.T) {
			home := t.TempDir()
			cmd := repairM858Command(t, home, "matching-failure", mode)
			output, err := cmd.CombinedOutput()
			if status := repairM858Status(t, err); status != 37 {
				t.Fatalf("exit = %d, want 37; output: %s", status, output)
			}
			repairM858AssertInvocation(t, home, cmd.Process.Pid)
			if mode == "file" {
				output, err = os.ReadFile(filepath.Join(home, "node.log"))
				if err != nil {
					t.Fatal(err)
				}
			}
			if !bytes.Contains(output, []byte("Benchmark execution started\n")) {
				t.Fatalf("matching node line lost: %s", output)
			}
			if mode == "filtered" && bytes.Contains(output, []byte("fatal: execution failed")) {
				t.Fatalf("unmatched diagnostic leaked: %s", output)
			}
			if mode != "filtered" && !bytes.Contains(output, []byte("fatal: execution failed\n")) {
				t.Fatalf("unfiltered stderr lost: %s", output)
			}
		})
	}
}

func TestRepairM_858_NodeExitWithoutMatchingLines(t *testing.T) {
	for _, tc := range []struct {
		peer   string
		status int
	}{{"quiet-success", 0}, {"quiet-failure", 29}} {
		t.Run(tc.peer, func(t *testing.T) {
			home := t.TempDir()
			cmd := repairM858Command(t, home, tc.peer, "filtered")
			output, err := cmd.CombinedOutput()
			if status := repairM858Status(t, err); status != tc.status {
				t.Fatalf("exit = %d, want %d; output: %s", status, tc.status, output)
			}
			repairM858AssertInvocation(t, home, cmd.Process.Pid)
			for _, line := range []string{"ordinary node output", "fatal: startup failed"} {
				if bytes.Contains(output, []byte(line)) {
					t.Fatalf("unmatched line leaked: %s", output)
				}
			}
		})
	}
}

func TestRepairM_858_LogTransport(t *testing.T) {
	for _, mode := range []string{"filtered", "debug", "file"} {
		t.Run(mode, func(t *testing.T) {
			home := t.TempDir()
			cmd := repairM858Command(t, home, "log-transcript", mode)
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("launcher: %v; output: %s", err, output)
			}
			if mode == "file" {
				if bytes.Contains(output, []byte("benchmark tps=19")) {
					t.Fatalf("file output leaked to terminal: %s", output)
				}
				output, err = os.ReadFile(filepath.Join(home, "node.log"))
				if err != nil {
					t.Fatal(err)
				}
			}
			for _, line := range []string{"benchmark tps=19\n", "Benchmark complete\n", "deployed contract\n", "transitioning scenario\n", "benchmark stderr\n", "benchmark final line without newline"} {
				if !bytes.Contains(output, []byte(line)) {
					t.Fatalf("node output missing %q: %s", line, output)
				}
			}
			for _, line := range []string{"ordinary node output\n", "unrelated diagnostic\n"} {
				if bytes.Contains(output, []byte(line)) != (mode != "filtered") {
					t.Fatalf("%s output mishandled %q: %s", mode, line, output)
				}
			}
		})
	}
}

func TestRepairM_858_SignalsReachNode(t *testing.T) {
	for _, mode := range []string{"filtered", "debug", "file"} {
		for _, tc := range []struct {
			signal syscall.Signal
			name   string
			status int
		}{{syscall.SIGTERM, "TERM", 143}, {syscall.SIGINT, "INT", 130}} {
			t.Run(mode+"/"+tc.name, func(t *testing.T) {
				home := t.TempDir()
				cmd := repairM858Command(t, home, "signal", mode)
				input, err := cmd.StdinPipe()
				if err != nil {
					t.Fatal(err)
				}
				defer input.Close()
				var output bytes.Buffer
				cmd.Stdout, cmd.Stderr = &output, &output
				if err := cmd.Start(); err != nil {
					t.Fatal(err)
				}
				done := make(chan error, 1)
				go func() { done <- cmd.Wait() }()
				timer := time.NewTimer(5 * time.Second)
				defer timer.Stop()
				ticker := time.NewTicker(5 * time.Millisecond)
				defer ticker.Stop()
			ready:
				for {
					if _, err := os.Stat(filepath.Join(home, "ready")); err == nil {
						break ready
					} else if !os.IsNotExist(err) {
						t.Fatal(err)
					}
					select {
					case err := <-done:
						t.Fatalf("node exited before signal: %v; output: %s", err, output.Bytes())
					case <-timer.C:
						t.Fatal("node did not become ready")
					case <-ticker.C:
					}
				}
				repairM858AssertInvocation(t, home, cmd.Process.Pid)
				if err := cmd.Process.Signal(tc.signal); err != nil {
					t.Fatal(err)
				}
				select {
				case err := <-done:
					if status := repairM858Status(t, err); status != tc.status {
						t.Fatalf("exit = %d, want %d; output: %s", status, tc.status, output.Bytes())
					}
				case <-timer.C:
					t.Fatal("launcher did not deliver signal to node")
				}
				data, err := os.ReadFile(filepath.Join(home, "signal"))
				if err != nil || string(data) != tc.name+"\n" {
					t.Fatalf("node signal = %q, %v; want %s", data, err, tc.name)
				}
			})
		}
	}
}

func TestRepairM_858_CustomHomeProducerConsumer(t *testing.T) {
	f := repair430NewFixture(t)
	output, err := f.run(f.target, "")
	if err != nil {
		t.Fatal(err)
	}
	home := strings.TrimSuffix(output, "\n")
	if home != f.target {
		t.Fatalf("validated home = %q, want %q", home, f.target)
	}
	data, err := os.ReadFile("benchmark.sh")
	if err != nil {
		t.Fatal(err)
	}
	script := string(data)
	initStart := strings.Index(script, `"$PAXD" init demo --chain-id pax-chain --home "$PAX_HOME"`)
	initEnd := strings.Index(script, "# add validator information to genesis file")
	if initStart < 0 || initEnd <= initStart {
		t.Fatal("cannot locate production key-to-genesis initialization")
	}
	peer, err := filepath.Abs("testdata/m858/init-transcript.sh")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(home, "config"), 0700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	initialize := exec.CommandContext(ctx, "/bin/bash", "--noprofile", "--norc", "-c", "set -e\n"+script[initStart:initEnd])
	initialize.Env = []string{"PATH=", "PAX_HOME=" + home, "PAXD=" + peer, "keyname=admin"}
	initOutput, err := initialize.CombinedOutput()
	if err != nil {
		t.Fatalf("production initialization: %v; output: %s", err, initOutput)
	}
	genesis, err := os.ReadFile(filepath.Join(home, "config", "genesis.json"))
	wantGenesis := "initialized pax-chain\naccount pax1benchmarkfixture 100000000000000000000uhpx,100000000000000000000uusdc,100000000000000000000uatom\n"
	if err != nil || string(genesis) != wantGenesis {
		t.Fatalf("genesis consumer = %q, %v; want %q", genesis, err, wantGenesis)
	}
	for name, want := range map[string]string{"key": "admin\n", "gentx": "admin 7000000000000000uhpx\n"} {
		got, err := os.ReadFile(filepath.Join(home, name))
		if err != nil || string(got) != want {
			t.Fatalf("selected-home %s = %q, %v; want %q", name, got, err, want)
		}
	}
	// The launch consumer must receive the same path emitted by the real home
	// producer, including spaces, rather than a late-moved default node home.
	cmd := repairM858Command(t, home, "quiet-success", "filtered")
	nodeOutput, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("start: %v; output: %s", err, nodeOutput)
	}
	repairM858AssertInvocation(t, home, cmd.Process.Pid)
	f.assertProtected(t)

	for _, forbidden := range []string{"~/.pax", "$HOME/.pax/config", `mv "$HOME/.pax"`, `mv ~/.pax`} {
		if strings.Contains(script, forbidden) {
			t.Fatalf("split initialization home remains: %s", forbidden)
		}
	}
	for _, line := range strings.Split(script, "\n") {
		if strings.Contains(line, `"$PAXD"`) && !strings.Contains(line, `"${PAXD_ARGS[@]}"`) && !strings.Contains(line, `PAXD_BIN=`) && !strings.Contains(line, `--home "$PAX_HOME"`) {
			t.Fatalf("paxd command does not use selected home: %s", line)
		}
		if strings.Contains(line, "/config/genesis.json") && !strings.Contains(line, `"$PAX_HOME"/config/genesis.json`) {
			t.Fatalf("genesis edit does not use selected home: %s", line)
		}
	}
	prepare := strings.Index(script, "PAX_HOME=$(")
	export := strings.Index(script, "export PAX_HOME")
	init := strings.Index(script, `"$PAXD" init demo --chain-id pax-chain --home "$PAX_HOME"`)
	accounts := strings.Index(script, `PAXD_BIN="$PAXD" python3  "$REPO_ROOT/tools/benchmark/loadtest/scripts/populate_genesis_accounts.py"`)
	if prepare < 0 || export <= prepare || init <= export || accounts <= init {
		t.Fatal("selected home must be prepared and exported before node and account initialization")
	}
	producer, err := os.ReadFile("loadtest/scripts/populate_genesis_accounts.py")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{`os.environ.get("PAX_HOME"`, `"--home", pax_home`, `os.path.join(pax_home, "config/genesis.json")`} {
		if !strings.Contains(string(producer), required) {
			t.Fatalf("account producer no longer consumes selected home: %s", required)
		}
	}
}
