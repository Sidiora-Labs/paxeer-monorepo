package blocksim

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/rand"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/ledger_db/block"
	memblockdb "github.com/Sidiora-Labs/Paxeer-X-Network/storage/ledger_db/block/mem_block_db"
)

// This fixture journals actual writes and pruning decisions before applying them
// to the production memory store. Closing a journal exercises real OS failures.
type repairM1160JournalStore struct {
	block.BlockDB
	writes *os.File
	prunes *os.File
}

func newRepairM1160JournalStore(t *testing.T) *repairM1160JournalStore {
	t.Helper()
	open := func(name string) *os.File {
		f, err := os.Create(filepath.Join(t.TempDir(), name))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = f.Close() })
		return f
	}
	return &repairM1160JournalStore{
		BlockDB: memblockdb.NewMemBlockDB(),
		writes:  open("writes.jsonl"),
		prunes:  open("prunes.jsonl"),
	}
}

func (s *repairM1160JournalStore) WriteBlock(ctx context.Context, blk *block.BinaryBlock) error {
	if err := json.NewEncoder(s.writes).Encode(blk); err != nil {
		return err
	}
	return s.BlockDB.WriteBlock(ctx, blk)
}

func (s *repairM1160JournalStore) Flush(ctx context.Context) error {
	if err := s.writes.Sync(); err != nil {
		return err
	}
	return s.BlockDB.Flush(ctx)
}

func (s *repairM1160JournalStore) Prune(ctx context.Context, height uint64) error {
	if err := json.NewEncoder(s.prunes).Encode(height); err != nil {
		return err
	}
	return s.BlockDB.Prune(ctx, height)
}

func (s *repairM1160JournalStore) Close(ctx context.Context) error {
	return errors.Join(s.writes.Close(), s.prunes.Close(), s.BlockDB.Close(ctx))
}

func newRepairM1160Runner(t *testing.T, db block.BlockDB) *BlockSim {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	config := DefaultBlocksimConfig()
	config.BytesPerTransaction = 8
	config.TransactionsPerBlock = 1
	config.ExtraBytesPerBlock = 8
	config.CannedRandomSize = minCannedRandomSize
	config.StagedBlockQueueSize = 1
	config.UnprunedBlocks = 4
	config.PruneIntervalBlocks = 2
	config.FlushIntervalBlocks = 0
	config.ConsoleUpdateIntervalSeconds = 3600
	config.ConsoleUpdateIntervalBlocks = 1_000_000
	now := time.Now()
	return &BlockSim{
		ctx: ctx, cancel: cancel, config: config, db: db,
		closeChan: make(chan struct{}), suspendChan: make(chan bool, 1),
		startTimestamp: now, lastConsoleUpdateTime: now,
		consoleUpdatePeriod: time.Hour,
	}
}

func repairM1160Generator(b *BlockSim, startHeight uint64) *BlockGenerator {
	return NewBlockGenerator(b.ctx, b.config, rand.NewCannedRandom(int(b.config.CannedRandomSize), b.config.Seed), startHeight)
}

func assertRepairM1160Result(t *testing.T, b *BlockSim, operation string, cause error) {
	t.Helper()
	err := b.BlockUntilHalted()
	if !errors.Is(err, cause) || !strings.Contains(err.Error(), operation) {
		t.Fatalf("halt result = %v; want %q wrapping %v", err, operation, cause)
	}
	if closeErr := b.Close(); closeErr != err {
		t.Fatalf("Close result = %v; want original error %v", closeErr, err)
	}
	var stderr bytes.Buffer
	if code := CLIExitCode(err, &stderr); code != 1 {
		t.Fatalf("CLI exit = %d; want 1", code)
	}
	if !strings.Contains(stderr.String(), operation) || !strings.Contains(stderr.String(), cause.Error()) {
		t.Fatalf("CLI diagnostic lost the operation or cause: %q", stderr.String())
	}
}

func TestRepairM_1160_WorkloadErrorsReachCLI(t *testing.T) {
	for _, operation := range []string{"write", "flush", "prune"} {
		t.Run(operation, func(t *testing.T) {
			store := newRepairM1160JournalStore(t)
			b := newRepairM1160Runner(t, store)
			var cause error = os.ErrClosed
			startHeight := uint64(1)
			switch operation {
			case "write":
				if err := store.writes.Close(); err != nil {
					t.Fatal(err)
				}
			case "flush":
				// A pipe accepts the journal record but cannot be fsynced.
				reader, writer, err := os.Pipe()
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = reader.Close(); _ = writer.Close() })
				store.writes = writer
				cause = writer.Sync()
				if cause == nil {
					t.Fatal("pipe unexpectedly supports durable flushing")
				}
				cause = errors.Unwrap(cause)
				b.config.FlushIntervalBlocks = 1
			case "prune":
				if err := store.prunes.Close(); err != nil {
					t.Fatal(err)
				}
				startHeight = 6
			}
			b.generator = repairM1160Generator(b, startHeight)
			go b.run()
			select {
			case <-b.closeChan:
			case <-time.After(5 * time.Second):
				t.Fatal("workload failure did not halt the benchmark")
			}
			assertRepairM1160Result(t, b, "failed to "+operation, cause)
			wantBlocks := int64(1)
			if operation == "write" {
				wantBlocks = 0
			}
			if b.totalBlocksWritten != wantBlocks {
				t.Fatalf("wrote %d blocks after backend failure; want %d", b.totalBlocksWritten, wantBlocks)
			}
			_, present, err := store.GetBlockByHeight(context.Background(), startHeight)
			if err != nil || present != (wantBlocks == 1) {
				t.Fatalf("actual store state: present=%v, error=%v", present, err)
			}
		})
	}
}

func TestRepairM_1160_TeardownAndSuspendErrors(t *testing.T) {
	for _, operation := range []string{"flush during teardown", "close database", "flush on suspend"} {
		t.Run(operation, func(t *testing.T) {
			store := newRepairM1160JournalStore(t)
			b := newRepairM1160Runner(t, store)
			file := store.writes
			if operation == "close database" {
				file = store.prunes
			}
			if err := file.Close(); err != nil {
				t.Fatal(err)
			}
			if operation == "flush on suspend" {
				b.suspend()
				if b.ctx.Err() != context.Canceled {
					t.Fatal("failed suspension flush did not cancel the workload")
				}
			}
			b.teardown()
			assertRepairM1160Result(t, b, "failed to "+operation, os.ErrClosed)
		})
	}
}

func TestRepairM_1160_CleanupErrorReachesCLI(t *testing.T) {
	for _, cleanData := range []bool{true, false} {
		t.Run(map[bool]string{true: "data", false: "logs"}[cleanData], func(t *testing.T) {
			b := newRepairM1160Runner(t, memblockdb.NewMemBlockDB())
			path := filepath.Join(t.TempDir(), "not-a-directory")
			if err := os.WriteFile(path, []byte("preserve"), 0600); err != nil {
				t.Fatal(err)
			}
			b.config.DataDir, b.config.LogDir = path, path
			b.config.CleanDataOnExit, b.config.CleanLogsOnExit = cleanData, !cleanData
			b.teardown()
			err := b.BlockUntilHalted()
			var pathErr *os.PathError
			if !errors.As(err, &pathErr) || pathErr.Path != path {
				t.Fatalf("cleanup result = %v; want original filesystem cause for %s", err, path)
			}
			assertRepairM1160Result(t, b, "failed to clean", pathErr.Err)
		})
	}
}

func TestRepairM_1160_CleanShutdownAndConcurrentWaiters(t *testing.T) {
	b := newRepairM1160Runner(t, memblockdb.NewMemBlockDB())
	b.generator = repairM1160Generator(b, 1)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := b.BlockUntilHalted(); err != nil {
				t.Errorf("clean halt = %v", err)
			}
		}()
	}
	go b.run()
	if err := b.Close(); err != nil {
		t.Fatal(err)
	}
	waited := make(chan struct{})
	go func() { wg.Wait(); close(waited) }()
	select {
	case <-waited:
	case <-time.After(5 * time.Second):
		t.Fatal("shutdown failed to release all waiters")
	}
	var stderr bytes.Buffer
	if code := CLIExitCode(b.BlockUntilHalted(), &stderr); code != 0 || stderr.Len() != 0 {
		t.Fatalf("clean CLI result = %d, %q", code, stderr.String())
	}
}

func TestRepairM_1160_PruneRetentionAndInterval(t *testing.T) {
	store := newRepairM1160JournalStore(t)
	b := newRepairM1160Runner(t, store)
	defer b.teardown()
	g := repairM1160Generator(b, 1)
	for height := uint64(1); height <= 10; height++ {
		blk := g.NextBlock()
		if blk == nil || blk.Height != height {
			t.Fatalf("producer at height %d returned %v", height, blk)
		}
		b.handleNextBlock(blk)
		if b.err != nil {
			t.Fatal(b.err)
		}
		if height <= b.config.UnprunedBlocks {
			info, err := store.prunes.Stat()
			if err != nil || info.Size() != 0 {
				t.Fatalf("height %d: pruning ran before the retention window was exceeded: %v", height, err)
			}
		}
		lowest := uint64(1)
		lastPrune := height - height%b.config.PruneIntervalBlocks
		if lastPrune > b.config.UnprunedBlocks {
			lowest = lastPrune - b.config.UnprunedBlocks
		}
		lo, err := store.GetLowestBlockHeight(b.ctx)
		if err != nil || lo != lowest {
			t.Fatalf("height %d: lowest = %d, %v; want %d", height, lo, err, lowest)
		}
		for retainedHeight := uint64(1); retainedHeight <= height; retainedHeight++ {
			stored, ok, err := store.GetBlockByHeight(b.ctx, retainedHeight)
			if err != nil || ok != (retainedHeight >= lowest) {
				t.Fatalf("after height %d, height %d: present=%v error=%v; oldest retained=%d", height, retainedHeight, ok, err, lowest)
			}
			if ok {
				byHash, found, err := store.GetBlockByHash(b.ctx, stored.Hash)
				if err != nil || !found || byHash.Height != retainedHeight {
					t.Fatalf("retained block hash lookup = %v, %v, %v", byHash, found, err)
				}
			}
		}
	}
}

func TestRepairM_1160_FailedBuildNeverLaunchesStaleExecutables(t *testing.T) {
	script, err := os.ReadFile("blocksim.sh")
	if err != nil {
		t.Fatal(err)
	}
	for _, withConfig := range []bool{false, true} {
		t.Run(map[bool]string{false: "benchmark", true: "logger-and-benchmark"}[withConfig], func(t *testing.T) {
			dir := t.TempDir()
			write := func(name, content string, mode os.FileMode) {
				t.Helper()
				if err := os.WriteFile(filepath.Join(dir, name), []byte(content), mode); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Mkdir(filepath.Join(dir, "bin"), 0700); err != nil {
				t.Fatal(err)
			}
			write("blocksim.sh", string(script), 0700)
			write("config.json", "{}", 0600)
			write("Makefile", ".PHONY: build\nbuild:\n\t@exit 37\n", 0600)
			write("bin/blocksim", "#!/bin/bash\nprintf 'benchmark launched\\n' >> \"${0%/*}/../launched\"\n", 0700)
			write("bin/configure-logger", "#!/bin/bash\nprintf 'logger launched\\n' >> \"${0%/*}/../launched\"\n", 0700)
			args := []string{"--noprofile", "--norc", filepath.Join(dir, "blocksim.sh")}
			if withConfig {
				args = append(args, filepath.Join(dir, "config.json"))
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, "/bin/bash", args...)
			cmd.Dir = dir
			// No inherited credentials, build settings, or shell startup hooks.
			cmd.Env = []string{"PATH=/usr/bin:/bin"}
			output, err := cmd.CombinedOutput()
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) || exitErr.ExitCode() != 2 {
				t.Fatalf("launcher result = %v, output=%s; want make's failure status 2", err, output)
			}
			if !bytes.Contains(output, []byte("37")) {
				t.Fatalf("build failure cause was lost: %s", output)
			}
			if _, err := os.Stat(filepath.Join(dir, "launched")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("a stale executable was launched: marker stat = %v, output=%s", err, output)
			}
		})
	}
}
