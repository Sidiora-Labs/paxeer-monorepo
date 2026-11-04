package runner

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

var repairM757RunFailure = flag.Bool("repair-m757-run-failure", false, "run the failing RunFile fixture in a child test process")

// The already-built test executable is a command fixture. Its stdout, stderr,
// exit and blocking behavior travel through the real OS process transport;
// no shell, Docker daemon or additional runtime is needed by these tests.
func TestMain(m *testing.M) {
	if len(os.Args) == 3 && os.Args[1] == "-c" && strings.HasPrefix(os.Args[2], "repair-m757:") {
		switch os.Args[2] {
		case "repair-m757:negative":
			fmt.Fprintln(os.Stdout, " 7 ")
			fmt.Fprintln(os.Stderr, "setup rejected")
			os.Exit(7)
		case "repair-m757:capture":
			fmt.Fprintln(os.Stdout, os.Getenv("CAPTURE"))
		case "repair-m757:prefix":
			fmt.Fprintln(os.Stdout, "PASS trailing output")
		case "repair-m757:leading":
			fmt.Fprintln(os.Stdout, "noise\nPASS")
		case "repair-m757:success":
			fmt.Fprintln(os.Stdout, "success")
			fmt.Fprintln(os.Stderr, "successful diagnostic")
		case "repair-m757:block":
			fmt.Fprintln(os.Stdout, "started")
			time.Sleep(10 * time.Second)
		default:
			fmt.Fprintln(os.Stderr, "unknown command fixture")
			os.Exit(64)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func repairM757Options(t *testing.T) []Option {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return []Option{WithContainer(""), WithShell(executable), WithCommandTimeout(3 * time.Second)}
}

func TestRepairM_757_RunFileExpectedStatusAndCapture(t *testing.T) {
	RunFile(t, filepath.Join("testdata", "repair_m757.yaml"), repairM757Options(t)...)
}

func TestRepairM_757_UnexpectedExitStatus(t *testing.T) {
	opts := newOptions(repairM757Options(t))
	out, err := execCmd(t, Input{Cmd: "repair-m757:negative"}, "", nil, opts)
	var exit *exec.ExitError
	if out != "7" || !errors.As(err, &exit) || exit.ExitCode() != 7 {
		t.Fatalf("output = %q, error = %v; want output 7 and exit 7", out, err)
	}
	if !strings.Contains(err.Error(), "setup rejected") || !strings.Contains(err.Error(), "expected 0") {
		t.Fatalf("missing status/stderr diagnostic: %v", err)
	}
	for _, input := range []Input{
		{Cmd: "repair-m757:negative", ExpectedExitCode: 8},
		{Cmd: "repair-m757:success", ExpectedExitCode: 7},
		{Cmd: "repair-m757:success", ExpectedExitCode: -1},
		{Cmd: "repair-m757:success", ExpectedExitCode: 256},
	} {
		if _, err := execCmd(t, input, "", nil, opts); err == nil {
			t.Fatalf("accepted unexpected status for %+v", input)
		}
	}
	out, err = execCmd(t, Input{Cmd: "repair-m757:success"}, "", nil, opts)
	if err != nil || out != "success" {
		t.Fatalf("successful command = %q, %v", out, err)
	}
	opts.Shell = filepath.Join(t.TempDir(), "missing-command")
	if _, err := execCmd(t, Input{ExpectedExitCode: 7}, "", nil, opts); err == nil {
		t.Fatal("launch failure satisfied a declared exit status")
	}
}

func TestRepairM_757_RunFileRejectsUnexpectedExit(t *testing.T) {
	if *repairM757RunFailure {
		RunFile(t, filepath.Join("testdata", "repair_m757_unexpected.yaml"), repairM757Options(t)...)
		return
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, executable, "-test.v", "-test.run=^TestRepairM_757_RunFileRejectsUnexpectedExit$", "-repair-m757-run-failure")
	output, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	if ctx.Err() != nil || !errors.As(err, &exit) || exit.ExitCode() != 1 {
		t.Fatalf("RunFile child exit = %v, context = %v, output = %s", err, ctx.Err(), output)
	}
	for _, diagnostic := range []string{"input[0] failed", "exit status 7 (expected 0)", "setup rejected", "--- FAIL:"} {
		if !strings.Contains(string(output), diagnostic) {
			t.Fatalf("missing %q in child result: %s", diagnostic, output)
		}
	}
}

func TestRepairM_757_CommandCancellation(t *testing.T) {
	t.Run("deadline cannot satisfy a negative status", func(t *testing.T) {
		opts := newOptions(repairM757Options(t))
		opts.CommandTimeout = 100 * time.Millisecond
		started := time.Now()
		_, err := execCmd(t, Input{Cmd: "repair-m757:block", ExpectedExitCode: 7}, "", nil, opts)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("deadline error = %v", err)
		}
		if elapsed := time.Since(started); elapsed > 2*time.Second {
			t.Fatalf("command cancellation took %v", elapsed)
		}
	})
	t.Run("caller cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		opts := newOptions(append(repairM757Options(t), WithContext(ctx)))
		timer := time.AfterFunc(100*time.Millisecond, cancel)
		defer timer.Stop()
		defer cancel()
		started := time.Now()
		_, err := execCmd(t, Input{Cmd: "repair-m757:block"}, "", nil, opts)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("caller cancellation = %v", err)
		}
		if elapsed := time.Since(started); elapsed > 2*time.Second {
			t.Fatalf("caller cancellation took %v", elapsed)
		}
	})
	t.Run("invalid timeout", func(t *testing.T) {
		opts := newOptions(append(repairM757Options(t), WithCommandTimeout(0)))
		if _, err := execCmd(t, Input{Cmd: "repair-m757:success"}, "", nil, opts); err == nil {
			t.Fatal("accepted an unbounded command timeout")
		}
	})
	if opts := newOptions(nil); opts.CommandTimeout <= 0 {
		t.Fatal("default command execution is unbounded")
	}
}

func TestRepairM_757_RegexPrefixTransport(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "repair_m757_regex.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []TestCase
	if err := yaml.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	want := []bool{true, false, false, false, true}
	if len(cases) != len(want) {
		t.Fatalf("regex cases = %d, want %d", len(cases), len(want))
	}
	for i, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			if len(tc.Inputs) != 1 || len(tc.Verifiers) != 1 {
				t.Fatal("fixture must drive one producer and verifier")
			}
			input := tc.Inputs[0]
			output, err := execCmd(t, input, "", nil, newOptions(repairM757Options(t)))
			if err != nil {
				t.Fatal(err)
			}
			env := map[string]string{input.Env: output}
			if err := verify(tc.Verifiers[0], env); (err == nil) != want[i] {
				t.Fatalf("verify(%q, %q) = %v; want match %v", tc.Verifiers[0].Expr, output, err, want[i])
			}
		})
	}
	if err := verify(Verifier{Type: "regex", Result: "MISSING", Expr: ".*"}, nil); err == nil {
		t.Fatal("regex accepted missing captured output")
	}
	if err := verify(Verifier{Type: "regex", Result: "OUTPUT", Expr: "["}, map[string]string{"OUTPUT": "PASS"}); err == nil {
		t.Fatal("regex accepted invalid pattern")
	}
}

func TestRepairM_757_ExpressionPrecedence(t *testing.T) {
	TestEvalExprPrecedence(t)
	TestEvalExprRejectsDanglingOperators(t)
	TestEvalExprOrPropagatesEvaluationError(t)
	if err := verify(Verifier{Type: "eval", Expr: "2 + 3 * 4 == 20"}, nil); err == nil {
		t.Fatal("eval verifier accepted incorrect mixed arithmetic")
	}
	if err := verify(Verifier{Type: "eval", Expr: "4 + == 4"}, nil); err == nil {
		t.Fatal("eval verifier accepted a dangling operator")
	}
}
