package bench_test

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/pebbledb/mvcc"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/state_db/ss"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/tools/bench"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/tools/cmd/paxdb/benchmark"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/tools/utils"
	"github.com/spf13/cobra"
	dbm "github.com/tendermint/tm-db"
)

type repairM1178Command struct {
	name       string
	newCommand func() *cobra.Command
	dimensions []string
}

func repairM1178Commands() []repairM1178Command {
	return []repairM1178Command{
		{"write", benchmark.DBWriteCmd, []string{"concurrency", "num-versions", "batch-size"}},
		{"read", benchmark.DBRandomReadCmd, []string{"concurrency", "num-versions", "max-operations"}},
		{"forward", benchmark.DBIterationCmd, []string{"concurrency", "num-versions", "max-operations", "iteration-steps"}},
		{"reverse", benchmark.DBReverseIterationCmd, []string{"concurrency", "num-versions", "max-operations", "iteration-steps"}},
	}
}

func repairM1178CLI(t *testing.T, command repairM1178Command, input, home string, flags ...string) (string, any) {
	t.Helper()
	cmd := command.newCommand()
	args := []string{"--db-backend=pebbledb", "--raw-kv-input-dir=" + input, "--output-dir=" + home}
	for _, dimension := range command.dimensions {
		args = append(args, "--"+dimension+"=1")
	}
	cmd.SetArgs(append(args, flags...))
	return repairM1178Capture(t, func() {
		if err := cmd.Execute(); err != nil {
			panic(err)
		}
	})
}

func repairM1178Capture(t *testing.T, run func()) (string, any) {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	previous := os.Stdout
	os.Stdout = writer
	defer func() { os.Stdout = previous }()
	output := make(chan string, 1)
	go func() {
		var buffer bytes.Buffer
		_, _ = io.Copy(&buffer, reader)
		output <- buffer.String()
	}()
	var failure any
	func() {
		defer func() { failure = recover() }()
		run()
	}()
	os.Stdout = previous
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	result := <-output
	if err := reader.Close(); err != nil {
		t.Fatal(err)
	}
	return result, failure
}

func repairM1178RawInput(t *testing.T, keys ...string) string {
	t.Helper()
	dir := t.TempDir()
	file, err := utils.CreateFile(dir, "fixture.kv")
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range keys {
		for _, field := range [][]byte{[]byte(key), []byte("value-" + key)} {
			if err := binary.Write(file, binary.LittleEndian, uint32(len(field))); err != nil {
				t.Fatal(err)
			}
			if _, err := file.Write(field); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	return dir
}

func repairM1178Open(t *testing.T, home string) types.StateStore {
	t.Helper()
	cfg := config.DefaultStateStoreConfig()
	cfg.KeepRecent = 0
	db, err := ss.NewStateStore(home, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	return db
}

func repairM1178Key(key string) string {
	return fmt.Sprintf(mvcc.StorePrefixTpl, "") + key
}

func repairM1178Seed(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	keys := make([]string, 64)
	for i := range keys {
		keys[i] = repairM1178Key(fmt.Sprintf("key-%03d", i))
	}
	input := repairM1178RawInput(t, keys...)
	db := repairM1178Open(t, home)
	output, failure := repairM1178Capture(t, func() {
		bench.BenchmarkDBWrite(db, input, 1, 3, 5)
	})
	if failure != nil {
		t.Fatalf("fixture write failed: %v\n%s", failure, output)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	return home
}

func repairM1178RequireError(t *testing.T, failure any, message string) {
	t.Helper()
	if _, ok := failure.(error); !ok || !strings.Contains(fmt.Sprint(failure), message) {
		t.Fatalf("expected explicit operator error %q, got %#v", message, failure)
	}
}

func TestRepairM_1178_UncheckedDimensionsCLI(t *testing.T) {
	input := repairM1178RawInput(t, repairM1178Key("key-032"))
	for _, command := range repairM1178Commands() {
		for _, dimension := range command.dimensions {
			for _, value := range []int{0, -1} {
				t.Run(fmt.Sprintf("%s/%s/%d", command.name, dimension, value), func(t *testing.T) {
					_, failure := repairM1178CLI(t, command, input, t.TempDir(), "--"+dimension+"="+strconv.Itoa(value))
					repairM1178RequireError(t, failure, "benchmark: "+dimension+" must be positive")
				})
			}
		}
	}
}

func TestRepairM_1178_EmptyInput(t *testing.T) {
	for _, input := range []string{t.TempDir(), repairM1178RawInput(t)} {
		for _, command := range repairM1178Commands() {
			t.Run(command.name+"/"+filepath.Base(input), func(t *testing.T) {
				_, failure := repairM1178CLI(t, command, input, t.TempDir())
				repairM1178RequireError(t, failure, "benchmark: input dataset is empty")
			})
		}
	}
}

func TestRepairM_1178_SharedEntryPoints(t *testing.T) {
	db := repairM1178Open(t, t.TempDir())
	input := repairM1178RawInput(t, repairM1178Key("key-032"))
	for _, value := range []int{0, -1} {
		cases := []struct {
			name      string
			dimension string
			run       func()
		}{
			{"write-concurrency", "concurrency", func() { bench.BenchmarkDBWrite(db, input, 1, value, 1) }},
			{"write-versions", "num-versions", func() { bench.BenchmarkDBWrite(db, input, value, 1, 1) }},
			{"write-batch", "batch-size", func() { bench.BenchmarkDBWrite(db, input, 1, 1, value) }},
			{"read-concurrency", "concurrency", func() { bench.BenchmarkDBRead(db, input, 1, value, 1) }},
			{"read-versions", "num-versions", func() { bench.BenchmarkDBRead(db, input, value, 1, 1) }},
			{"read-operations", "max-operations", func() { bench.BenchmarkDBRead(db, input, 1, 1, int64(value)) }},
			{"forward-concurrency", "concurrency", func() { bench.BenchmarkDBForwardIteration(db, input, 1, value, 1, 1) }},
			{"forward-versions", "num-versions", func() { bench.BenchmarkDBForwardIteration(db, input, value, 1, 1, 1) }},
			{"forward-operations", "max-operations", func() { bench.BenchmarkDBForwardIteration(db, input, 1, 1, int64(value), 1) }},
			{"forward-steps", "iteration-steps", func() { bench.BenchmarkDBForwardIteration(db, input, 1, 1, 1, value) }},
			{"reverse-concurrency", "concurrency", func() { bench.BenchmarkDBReverseIteration(db, input, 1, value, 1, 1) }},
			{"reverse-versions", "num-versions", func() { bench.BenchmarkDBReverseIteration(db, input, value, 1, 1, 1) }},
			{"reverse-operations", "max-operations", func() { bench.BenchmarkDBReverseIteration(db, input, 1, 1, int64(value), 1) }},
			{"reverse-steps", "iteration-steps", func() { bench.BenchmarkDBReverseIteration(db, input, 1, 1, 1, value) }},
		}
		for _, tc := range cases {
			t.Run(fmt.Sprintf("%s/%d", tc.name, value), func(t *testing.T) {
				_, failure := repairM1178Capture(t, tc.run)
				repairM1178RequireError(t, failure, "benchmark: "+tc.dimension+" must be positive")
			})
		}
	}
	if db.GetLatestVersion() != 0 {
		t.Fatal("invalid workloads modified the database")
	}
}

func TestRepairM_1178_IterationBoundsCLI(t *testing.T) {
	input := repairM1178RawInput(t, repairM1178Key("key-032"))
	for _, command := range repairM1178Commands()[2:] {
		for _, bound := range []int{1, 3} {
			t.Run(fmt.Sprintf("%s/%d", command.name, bound), func(t *testing.T) {
				home := repairM1178Seed(t)
				output, failure := repairM1178CLI(t, command, input, home,
					"--max-operations=9", "--concurrency=3", "--iteration-steps="+strconv.Itoa(bound))
				if failure != nil {
					t.Fatalf("benchmark command failed: %v\n%s", failure, output)
				}
				label := "Total Prefixes Iterated: "
				if command.name == "reverse" {
					label = "Total Prefixes Reverse-Iterated: "
				}
				if !strings.Contains(output, fmt.Sprintf("%s%d\n", label, 9*bound)) {
					t.Fatalf("expected exactly %d real database steps:\n%s", 9*bound, output)
				}
				// The command released its database lock as well as its iterators.
				db := repairM1178Open(t, home)
				value, err := db.Get("", 1, []byte(repairM1178Key("key-032")))
				if err != nil || string(value) != "value-"+repairM1178Key("key-032") {
					t.Fatalf("stored fixture changed: value=%q error=%v", value, err)
				}
			})
		}
	}
}

// These observers delegate every operation to the real StateStore/iterator.
// They measure lifetime and traversal without substituting storage behavior.
type repairM1178ObservedStore struct {
	types.StateStore
	mu                              sync.Mutex
	active, maximum, opened, closed int
	activeBeforeOpen                []int
	stepsAtClose                    []int
}

type repairM1178ObservedIterator struct {
	dbm.Iterator
	store *repairM1178ObservedStore
	steps int
}

func (db *repairM1178ObservedStore) ReverseIterator(storeKey string, version int64, start, end []byte) (dbm.Iterator, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.activeBeforeOpen = append(db.activeBeforeOpen, db.active)
	it, err := db.StateStore.ReverseIterator(storeKey, version, start, end)
	if err != nil {
		return nil, err
	}
	db.opened++
	db.active++
	if db.active > db.maximum {
		db.maximum = db.active
	}
	return &repairM1178ObservedIterator{Iterator: it, store: db}, nil
}

func (it *repairM1178ObservedIterator) Next() {
	it.Iterator.Next()
	it.steps++
}

func (it *repairM1178ObservedIterator) Close() error {
	err := it.Iterator.Close()
	it.store.mu.Lock()
	defer it.store.mu.Unlock()
	it.store.active--
	it.store.closed++
	it.store.stepsAtClose = append(it.store.stepsAtClose, it.steps)
	return err
}

func TestRepairM_1178_ReverseIteratorLifetime(t *testing.T) {
	input := repairM1178RawInput(t, repairM1178Key("key-032"))
	for _, concurrency := range []int{1, 4} {
		t.Run(strconv.Itoa(concurrency), func(t *testing.T) {
			db := &repairM1178ObservedStore{StateStore: repairM1178Open(t, repairM1178Seed(t))}
			output, failure := repairM1178Capture(t, func() {
				bench.BenchmarkDBReverseIteration(db, input, 1, concurrency, 48, 3)
			})
			if failure != nil {
				t.Fatalf("reverse benchmark failed: %v\n%s", failure, output)
			}
			if db.opened != 48 || db.closed != 48 || db.active != 0 || db.maximum > concurrency {
				t.Fatalf("iterators retained across operations: opened=%d closed=%d active=%d maximum=%d workers=%d",
					db.opened, db.closed, db.active, db.maximum, concurrency)
			}
			if len(db.stepsAtClose) != 48 {
				t.Fatalf("missing operation measurements: %d", len(db.stepsAtClose))
			}
			for i, steps := range db.stepsAtClose {
				if steps != 3 {
					t.Fatalf("operation %d traversed %d keys, want 3", i, steps)
				}
			}
			if concurrency == 1 {
				for i, active := range db.activeBeforeOpen {
					if active != 0 {
						t.Fatalf("operation %d began with %d previous iterators still open", i, active)
					}
				}
			}
			if !strings.Contains(output, "Total Prefixes Reverse-Iterated: 144\n") {
				t.Fatalf("incorrect reverse benchmark summary:\n%s", output)
			}
		})
	}
}

func TestRepairM_1178_EmptySummaries(t *testing.T) {
	input := repairM1178RawInput(t, repairM1178Key("key-032"))
	for _, populated := range []bool{false, true} {
		home := t.TempDir()
		if populated {
			home = repairM1178Seed(t)
		}
		db := repairM1178Open(t, home)
		// A version below the retained boundary produces a real empty iterator,
		// including on backends whose unbounded empty namespace hits metadata.
		if err := db.SetEarliestVersion(2, true); err != nil {
			t.Fatal(err)
		}
		for _, reverse := range []bool{false, true} {
			t.Run(fmt.Sprintf("populated=%t/reverse=%t", populated, reverse), func(t *testing.T) {
				seekInput := input
				if populated {
					key := "zzz"
					if reverse {
						key = "aaa"
					}
					seekInput = repairM1178RawInput(t, repairM1178Key(key))
				}
				output, failure := repairM1178Capture(t, func() {
					if reverse {
						bench.BenchmarkDBReverseIteration(db, seekInput, 1, 2, 5, 3)
					} else {
						bench.BenchmarkDBForwardIteration(db, seekInput, 1, 2, 5, 3)
					}
				})
				if failure != nil || !strings.Contains(output, "unavailable (no keys iterated)") ||
					!strings.Contains(output, "Iterated: 0\n") || strings.Contains(output, "NaN") || strings.Contains(output, "+Inf") {
					t.Fatalf("empty traversal summary is unsafe: failure=%v\n%s", failure, output)
				}
			})
		}
	}
}

func TestRepairM_1178_WriteReadRoundTrip(t *testing.T) {
	keys := []string{"key-001", "key-002", "key-003", "key-004", "key-005", "key-006", "key-007"}
	input := repairM1178RawInput(t, keys...)
	db := repairM1178Open(t, t.TempDir())
	output, failure := repairM1178Capture(t, func() {
		bench.BenchmarkDBWrite(db, input, 2, 3, 2)
		bench.BenchmarkDBRead(db, input, 2, 3, 17)
	})
	if failure != nil || !strings.Contains(output, "Total Successfully Read 17\n") {
		t.Fatalf("valid workload failed: %v\n%s", failure, output)
	}
	for version := int64(1); version <= 2; version++ {
		for _, key := range keys {
			value, err := db.Get("", version, []byte(key))
			if err != nil || string(value) != "value-"+key {
				t.Fatalf("version=%d key=%q value=%q error=%v", version, key, value, err)
			}
		}
	}
}
