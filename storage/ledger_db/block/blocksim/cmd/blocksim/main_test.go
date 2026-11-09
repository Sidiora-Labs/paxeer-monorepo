package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/ledger_db/block/blocksim"
)

func TestIssue1700CLICleanup(t *testing.T) {
	for _, failCleanup := range []bool{true, false} {
		t.Run(map[bool]string{true: "cleanup failure", false: "clean shutdown"}[failCleanup], func(t *testing.T) {
			stdout, err := os.CreateTemp(t.TempDir(), "stdout")
			if err != nil {
				t.Fatal(err)
			}
			defer stdout.Close()
			stderr, err := os.CreateTemp(t.TempDir(), "stderr")
			if err != nil {
				t.Fatal(err)
			}
			defer stderr.Close()
			originalStdout, originalStderr := os.Stdout, os.Stderr
			os.Stdout, os.Stderr = stdout, stderr
			defer func() { os.Stdout, os.Stderr = originalStdout, originalStderr }()

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			config := blocksim.DefaultBlocksimConfig()
			config.DataDir = filepath.Join(t.TempDir(), "data")
			config.LogDir = t.TempDir()
			config.BytesPerTransaction = 8
			config.TransactionsPerBlock = 1
			config.ExtraBytesPerBlock = 8
			config.CannedRandomSize = 1 << 20
			config.StagedBlockQueueSize = 1
			config.BackgroundMetricsScrapeInterval = 0
			config.MaxBlocksPerSecond = 1
			config.ConsoleUpdateIntervalSeconds = 3600
			config.ConsoleUpdateIntervalBlocks = 1_000_000
			config.EnableSuspension = false
			config.CleanDataOnExit = true
			config.CleanLogsOnExit = true
			if err := config.Validate(); err != nil {
				t.Fatal(err)
			}
			logPath := filepath.Join(config.LogDir, "benchmark.log")
			if err := os.WriteFile(logPath, []byte("cleanup required"), 0600); err != nil {
				t.Fatal(err)
			}
			bs, err := blocksim.NewBlockSim(ctx, config, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer bs.Close()
			var cause error
			if failCleanup {
				if err := os.Remove(config.DataDir); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(config.DataDir, []byte("preserve"), 0600); err != nil {
					t.Fatal(err)
				}
				_, cause = os.ReadDir(config.DataDir)
				if cause == nil {
					t.Fatal("reading a file as a directory unexpectedly succeeded")
				}
			}
			result := make(chan error, 1)
			go func() { result <- waitForBenchmark(bs) }()
			cancel()
			var resultErr error
			select {
			case resultErr = <-result:
			case <-time.After(5 * time.Second):
				t.Fatal("waitForBenchmark did not complete cleanup")
			}
			if haltErr := bs.BlockUntilHalted(); resultErr != haltErr {
				t.Fatalf("waitForBenchmark = %v; want original halt error %v", resultErr, haltErr)
			}
			if closeErr := bs.Close(); closeErr != resultErr {
				t.Fatalf("Close = %v; want original result %v", closeErr, resultErr)
			}
			code := blocksim.CLIExitCode(resultErr, os.Stderr)
			read := func(file *os.File) string {
				t.Helper()
				data, err := os.ReadFile(file.Name())
				if err != nil {
					t.Fatal(err)
				}
				return string(data)
			}
			out, diagnostic := read(stdout), read(stderr)
			output := out + diagnostic
			if _, err := os.Stat(logPath); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("cleanup did not finish removing logs: %v", err)
			}
			if !failCleanup {
				if resultErr != nil || code != 0 || diagnostic != "" || strings.Contains(out, "failed to") {
					t.Fatalf("clean result = %v, exit %d, stdout %q, stderr %q", resultErr, code, out, diagnostic)
				}
				return
			}
			var pathErr, causePathErr *os.PathError
			if !errors.As(cause, &causePathErr) || !errors.As(resultErr, &pathErr) || pathErr.Path != config.DataDir || !errors.Is(resultErr, causePathErr.Err) {
				t.Fatalf("cleanup result = %v; want original filesystem cause %v", resultErr, cause)
			}
			if !strings.Contains(resultErr.Error(), "failed to clean data directory on exit") || code != 1 {
				t.Fatalf("cleanup result = %v, exit %d; want original operation and exit 1", resultErr, code)
			}
			if diagnostic != "Error: "+resultErr.Error()+"\n" || strings.Count(output, resultErr.Error()) != 1 || strings.Count(output, pathErr.Err.Error()) != 1 || strings.Count(output, "failed to") != 1 {
				t.Fatalf("cleanup failure must appear exactly once: stdout %q, stderr %q", out, diagnostic)
			}
			if strings.Contains(output, "Error closing blocksim") || strings.Contains(output, "failed to close database") {
				t.Fatalf("cleanup failure was relabeled as a close failure: %q", output)
			}
			if data, err := os.ReadFile(config.DataDir); err != nil || string(data) != "preserve" {
				t.Fatalf("cleanup changed the obstructing file: %q, %v", data, err)
			}
		})
	}
}
