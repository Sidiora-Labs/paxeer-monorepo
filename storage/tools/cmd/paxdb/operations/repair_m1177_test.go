package operations

import (
	"bufio"
	"bytes"
	"compress/flate"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/proto"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/state_db/sc/memiavl"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/state_db/ss/backend"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/tools/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/wal"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func repairM1177OpenStore(t *testing.T, dir string) types.StateStore {
	t.Helper()
	cfg := config.DefaultStateStoreConfig()
	cfg.DBDirectory = dir
	cfg.KeepRecent = 0
	store, err := backend.ResolveBackend(cfg.Backend)(dir, cfg)
	require.NoError(t, err)
	return store
}

func repairM1177Changes(name, key, value string) []*proto.NamedChangeSet {
	return []*proto.NamedChangeSet{{Name: name, Changeset: proto.ChangeSet{
		Pairs: []*proto.KVPair{{Key: []byte(key), Value: []byte(value)}},
	}}}
}

func repairM1177Populate(t *testing.T, dir, value string) {
	t.Helper()
	store := repairM1177OpenStore(t, dir)
	for version := int64(1); version <= 4; version++ {
		require.NoError(t, store.ApplyChangesetSync(version, repairM1177Changes("sequencer", "key", fmt.Sprintf("%s-%d", value, version))))
	}
	require.NoError(t, store.Close())
}

func repairM1177Execute(cmd *cobra.Command, args ...string) (string, error) {
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func TestRepairM_1177_DumpSource(t *testing.T) {
	source, output := filepath.Join(t.TempDir(), "source"), filepath.Join(t.TempDir(), "output")
	repairM1177Populate(t, source, "source")
	repairM1177Populate(t, output, "sentinel")
	_, err := repairM1177Execute(DumpDbCmd(), "--db-backend", "pebbledb", "--db-dir", source, "--output-dir", output, "--module", "sequencer")
	require.NoError(t, err)
	data, err := os.ReadFile(filepath.Join(output, outputFileName))
	require.NoError(t, err)
	for version := 1; version <= 4; version++ {
		require.Contains(t, string(data), fmt.Sprintf("Key: %X Val: %X Version: %d\n", "key", fmt.Sprintf("source-%d", version), version))
	}
	require.Len(t, strings.Split(strings.TrimSpace(string(data)), "\n"), 4)
	require.NotContains(t, string(data), fmt.Sprintf("%X", "sentinel"))
	// Immediate reopen proves the command released its real backend handle.
	store := repairM1177OpenStore(t, source)
	require.Equal(t, int64(4), store.GetLatestVersion())
	require.NoError(t, store.Close())
	sentinel := repairM1177OpenStore(t, output)
	value, err := sentinel.Get("sequencer", 4, []byte("key"))
	require.NoError(t, err)
	require.Equal(t, []byte("sentinel-4"), value)
	require.NoError(t, sentinel.Close())
	require.NoDirExists(t, filepath.Join(output, "data"))
}

func TestRepairM_1177_MissingSource(t *testing.T) {
	root := t.TempDir()
	for _, source := range []string{filepath.Join(root, "missing"), t.TempDir()} {
		output := filepath.Join(root, "export")
		require.Error(t, DumpDbData("pebbledb", "", output, source))
		require.NoDirExists(t, output)
		require.Error(t, PruneDB("pebbledb", source, 1))
		require.NoFileExists(t, filepath.Join(source, "CURRENT"))
		require.NoDirExists(t, filepath.Join(source, "data"))
	}
	require.NoDirExists(t, filepath.Join(root, "missing"))
}

func repairM1177IAVL(t *testing.T, names []string, largeValue bool) string {
	t.Helper()
	dir := t.TempDir()
	db, err := memiavl.OpenDB(0, memiavl.Options{Dir: dir, CreateIfMissing: true, InitialStores: names})
	require.NoError(t, err)
	for _, name := range names {
		value := name + "-value"
		if largeValue {
			value = strings.Repeat("v", 1<<20)
		}
		require.NoError(t, db.ApplyChangeSet(name, proto.ChangeSet{Pairs: []*proto.KVPair{
			{Key: []byte("a"), Value: []byte(value)},
			{Key: []byte("z"), Value: []byte(name + "-last")},
		}}))
	}
	version, err := db.Commit()
	require.NoError(t, err)
	require.Equal(t, int64(1), version)
	require.NoError(t, db.Close())
	return dir
}

func repairM1177Stdout(t *testing.T, run func() error) (string, error) {
	t.Helper()
	file, err := os.CreateTemp(t.TempDir(), "stdout")
	require.NoError(t, err)
	previous := os.Stdout
	os.Stdout = file
	defer func() { os.Stdout = previous; require.NoError(t, file.Close()) }()
	runErr := run()
	_, err = file.Seek(0, io.SeekStart)
	require.NoError(t, err)
	data, err := io.ReadAll(file)
	require.NoError(t, err)
	return string(data), runErr
}

func TestRepairM_1177_ModuleInventory(t *testing.T) {
	names := []string{"sequencer", "lxphost", "custom-store"}
	dir := repairM1177IAVL(t, names, false)
	output := t.TempDir()
	_, err := repairM1177Execute(DumpIAVLCmd(), "--db-dir", dir, "--output-dir", output)
	require.NoError(t, err)
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(output, name))
		require.NoError(t, err)
		require.Contains(t, string(data), "Tree "+name+" has version 1")
		require.Contains(t, string(data), fmt.Sprintf("Key: %X, Value: %X", "a", name+"-value"))
		require.Contains(t, string(data), fmt.Sprintf("Key: %X, Value: %X", "z", name+"-last"))
	}
	report, err := repairM1177Stdout(t, func() error {
		_, err := repairM1177Execute(StateSizeCmd(), "--db-dir", dir)
		return err
	})
	require.NoError(t, err)
	for _, name := range names {
		size := 2 + len(name+"-value") + len(name+"-last")
		require.Contains(t, report, fmt.Sprintf("Module %s total numKeys:2, total keySize:2, total valueSize:%d, totalSize:%d", name, size-2, size))
	}
	filtered := t.TempDir()
	_, err = repairM1177Execute(DumpIAVLCmd(), "--db-dir", dir, "--output-dir", filtered, "--module", "lxphost")
	require.NoError(t, err)
	files, err := os.ReadDir(filtered)
	require.NoError(t, err)
	require.Len(t, files, 1)
	require.Equal(t, "lxphost", files[0].Name())
	report, err = repairM1177Stdout(t, func() error {
		_, err := repairM1177Execute(StateSizeCmd(), "--db-dir", dir, "--module", "lxphost")
		return err
	})
	require.NoError(t, err)
	require.Contains(t, report, "Module lxphost total numKeys:2")
	require.NotContains(t, report, "Module sequencer")
	require.NotContains(t, report, "Module custom-store")
}

func TestStateSizeFullScan(t *testing.T) {
	contractA := strings.Repeat("AB", 20)
	contractB := strings.Repeat("CD", 20)
	storageKey := func(address, slot byte) []byte {
		key := append([]byte{0x03}, bytes.Repeat([]byte{address}, 20)...)
		return append(key, bytes.Repeat([]byte{slot}, 32)...)
	}
	changes := map[string]proto.ChangeSet{
		"evm": {Pairs: []*proto.KVPair{
			{Key: []byte{0x01, 'a'}, Value: []byte("one")},
			{Key: []byte{0x02, 'b', 'c'}, Value: []byte("four")},
			{Key: storageKey(0xAB, 1), Value: []byte("aa")},
			{Key: storageKey(0xAB, 2), Value: []byte("bbbb")},
			{Key: storageKey(0xCD, 1), Value: []byte("ccccccc")},
			{Key: []byte{0x0A, 'd'}, Value: []byte("five!")},
		}},
		"sequencer": {Pairs: []*proto.KVPair{
			{Key: []byte{0x01, 'x'}, Value: []byte("cat")},
			{Key: []byte{0x01, 'y', 'z'}, Value: []byte("four")},
			{Key: storageKey(0xAB, 1), Value: []byte("other")},
		}},
	}
	want := map[string]*ModuleResult{
		"evm": {
			ModuleName: "evm", TotalNumKeys: 6, TotalKeySize: 166, TotalValueSize: 25, TotalSize: 191,
			PrefixSizes: map[string]*utils.PrefixSize{
				"01": {KeySize: 2, ValueSize: 3, TotalSize: 5, KeyCount: 1},
				"02": {KeySize: 3, ValueSize: 4, TotalSize: 7, KeyCount: 1},
				"03": {KeySize: 159, ValueSize: 13, TotalSize: 172, KeyCount: 3},
				"0A": {KeySize: 2, ValueSize: 5, TotalSize: 7, KeyCount: 1},
			},
			ContractSizes: map[string]*utils.ContractSizeEntry{
				contractA: {Address: contractA, TotalSize: 112, KeyCount: 2},
				contractB: {Address: contractB, TotalSize: 60, KeyCount: 1},
			},
		},
		"sequencer": {
			ModuleName: "sequencer", TotalNumKeys: 3, TotalKeySize: 58, TotalValueSize: 12, TotalSize: 70,
			PrefixSizes: map[string]*utils.PrefixSize{
				"01": {KeySize: 5, ValueSize: 7, TotalSize: 12, KeyCount: 2},
				"03": {KeySize: 53, ValueSize: 5, TotalSize: 58, KeyCount: 1},
			},
			ContractSizes: map[string]*utils.ContractSizeEntry{},
		},
	}
	wantConsole := func(t *testing.T, names []string) string {
		t.Helper()
		var output strings.Builder
		for _, name := range names {
			result := want[name]
			fmt.Fprintf(&output, "Module %s total numKeys:%d, total keySize:%d, total valueSize:%d, totalSize:%d (0.00 MB)\n",
				name, result.TotalNumKeys, result.TotalKeySize, result.TotalValueSize, result.TotalSize)
			prefixJSON, err := json.MarshalIndent(result.PrefixSizes, "", "  ")
			require.NoError(t, err)
			fmt.Fprintf(&output, "Module %s prefix breakdown (key/value/total bytes and key count per prefix byte): %s\n", name, prefixJSON)
			if name == "evm" {
				output.WriteString("\nDetailed breakdown for 0x03 prefix (top 2 contracts by total size):\n")
				fmt.Fprintf(&output, "%-42s %15s %10s\n", "Contract Address", "Total Size", "Key Count")
				fmt.Fprintln(&output, strings.Repeat("-", 70))
				fmt.Fprintf(&output, "0x%-40s %15d %10d\n", contractA, 112, 2)
				fmt.Fprintf(&output, "0x%-40s %15d %10d\n", contractB, 60, 1)
			}
		}
		return output.String()
	}
	checkDatabase := func(t *testing.T, db *memiavl.DB, snapshot bool) {
		t.Helper()
		for _, name := range []string{"evm", "sequencer"} {
			tree := db.TreeByName(name)
			require.NotNil(t, tree)
			tree.ScanPostOrder(func(node memiavl.Node) bool {
				if snapshot {
					require.IsType(t, memiavl.PersistedNode{}, node)
				} else {
					require.IsType(t, &memiavl.MemNode{}, node)
				}
				return false
			})
			require.Equal(t, want[name], collectModuleStats(tree, name))
		}
		for _, selection := range []string{"", "evm", "sequencer"} {
			t.Run("module="+selection, func(t *testing.T) {
				names := []string{"evm", "sequencer"}
				if selection != "" {
					names = []string{selection}
				}
				results := collectAllModuleData(selection, db)
				require.Len(t, results, len(names))
				for _, name := range names {
					expected := want[name]
					require.Equal(t, expected, results[name])
					analysis := createStateSizeAnalysis(1, name, results[name])
					var prefixes map[string]*utils.PrefixSize
					require.NoError(t, json.Unmarshal([]byte(analysis.PrefixBreakdown), &prefixes))
					require.Equal(t, expected.PrefixSizes, prefixes)
					var contracts []utils.ContractSizeEntry
					require.NoError(t, json.Unmarshal([]byte(analysis.ContractBreakdown), &contracts))
					wantContracts := make([]utils.ContractSizeEntry, 0, len(expected.ContractSizes))
					for _, entry := range expected.ContractSizes {
						wantContracts = append(wantContracts, *entry)
					}
					require.ElementsMatch(t, wantContracts, contracts)
					require.Equal(t, &utils.StateSizeAnalysis{
						BlockHeight: 1, ModuleName: name,
						TotalNumKeys: expected.TotalNumKeys, TotalKeySize: expected.TotalKeySize,
						TotalValueSize: expected.TotalValueSize, TotalSize: expected.TotalSize,
						PrefixBreakdown: analysis.PrefixBreakdown, ContractBreakdown: analysis.ContractBreakdown,
					}, analysis)
				}
				output, err := repairM1177Stdout(t, func() error {
					printResultsToConsole(results)
					return nil
				})
				require.NoError(t, err)
				require.Equal(t, wantConsole(t, names), output)
			})
		}
	}
	dir := t.TempDir()
	db, err := memiavl.OpenDB(0, memiavl.Options{Dir: dir, CreateIfMissing: true, InitialStores: []string{"evm", "sequencer"}})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	for name, changeSet := range changes {
		require.NoError(t, db.ApplyChangeSet(name, changeSet))
	}
	t.Run("mutable", func(t *testing.T) { checkDatabase(t, db, false) })
	version, err := db.Commit()
	require.NoError(t, err)
	require.Equal(t, int64(1), version)
	require.NoError(t, db.RewriteSnapshot(context.Background()))
	require.NoError(t, db.Close())
	db, err = memiavl.OpenDB(0, memiavl.Options{Dir: dir, ZeroCopy: true})
	require.NoError(t, err)
	require.Equal(t, int64(1), db.Version())
	require.Equal(t, int64(1), db.SnapshotVersion())
	t.Run("snapshot", func(t *testing.T) { checkDatabase(t, db, true) })
	require.NoError(t, db.Close())
	for _, selection := range []string{"", "evm", "sequencer"} {
		names := []string{"evm", "sequencer"}
		args := []string{"--db-dir", dir}
		if selection != "" {
			names = []string{selection}
			args = append(args, "--module", selection)
		}
		output, err := repairM1177Stdout(t, func() error {
			_, err := repairM1177Execute(StateSizeCmd(), args...)
			return err
		})
		require.NoError(t, err)
		require.Contains(t, output, "Finished opening db at height 1 (requested: 0)")
		require.True(t, strings.HasSuffix(output, wantConsole(t, names)), "%s", output)
		require.Equal(t, len(names), strings.Count(output, "total numKeys:"))
	}
}

func TestRepairM_1177_IAVLFailures(t *testing.T) {
	dir := repairM1177IAVL(t, []string{"sequencer"}, true)
	db, err := memiavl.OpenDB(0, memiavl.Options{Dir: dir})
	require.NoError(t, err)
	defer func() { require.NoError(t, db.Close()) }()
	tree := db.TreeByName("sequencer")
	t.Run("header", func(t *testing.T) {
		full, err := os.OpenFile("/dev/full", os.O_WRONLY, 0)
		require.NoError(t, err)
		err = writeIAVLTree(full, "sequencer", tree)
		require.ErrorIs(t, err, syscall.ENOSPC)
		_, err = full.Write([]byte("closed"))
		require.ErrorIs(t, err, os.ErrClosed)
	})
	t.Run("scan", func(t *testing.T) {
		reader, writer, err := os.Pipe()
		require.NoError(t, err)
		done := make(chan error, 1)
		go func() {
			_, readErr := bufio.NewReader(reader).ReadString('\n')
			done <- errors.Join(readErr, reader.Close())
		}()
		err = writeIAVLTree(writer, "sequencer", tree)
		require.ErrorIs(t, err, syscall.EPIPE)
		require.NoError(t, <-done)
		_, err = writer.Write([]byte("closed"))
		require.ErrorIs(t, err, os.ErrClosed)
	})
	t.Run("close", func(t *testing.T) {
		full, err := os.OpenFile("/dev/full", os.O_WRONLY, 0)
		require.NoError(t, err)
		defer func() { require.NoError(t, full.Close()) }()
		// The real compressor buffers this small export until Close, whose final
		// flush then fails on the real destination. No writer is replaced by a fake.
		compressed, err := flate.NewWriter(full, flate.BestSpeed)
		require.NoError(t, err)
		small := memiavl.New(0)
		small.Set([]byte("key"), []byte("value"))
		require.ErrorIs(t, writeIAVLTree(compressed, "sequencer", small), syscall.ENOSPC)
	})
	t.Run("publish", func(t *testing.T) {
		output := t.TempDir()
		// A nonempty directory at the final name rejects rename on the real export path.
		require.NoError(t, os.Mkdir(filepath.Join(output, "sequencer"), 0o755))
		sentinel := filepath.Join(output, "sequencer", "previous-backup")
		require.NoError(t, os.WriteFile(sentinel, []byte("preserved"), 0o600))
		report, err := repairM1177Stdout(t, func() error { return DumpIAVLData("sequencer", db, output) })
		require.Error(t, err)
		require.NotContains(t, report, "Finished dumping")
		data, err := os.ReadFile(sentinel)
		require.NoError(t, err)
		require.Equal(t, "preserved", string(data))
		files, err := os.ReadDir(output)
		require.NoError(t, err)
		require.Len(t, files, 1, "failed exports must remove staging files")
		require.True(t, files[0].IsDir(), "no partial backup was published")
	})
}

func TestRepairM_1177_PruneNamedSource(t *testing.T) {
	source := filepath.Join(t.TempDir(), "source")
	repairM1177Populate(t, source, "source")
	// This is precisely the unintended home-relative path used by the old command.
	sentinelDir := filepath.Join(source, "data", "state_store", "cosmos", "pebbledb")
	repairM1177Populate(t, sentinelDir, "sentinel")
	_, err := repairM1177Execute(PruneCmd(), "--db-backend", "pebbledb", "--db-dir", source, "--version", "2")
	require.NoError(t, err)
	for _, dir := range []string{source, sentinelDir} {
		store := repairM1177OpenStore(t, dir)
		var versions []int64
		_, err := store.RawIterate("sequencer", func(key, value []byte, version int64) bool {
			versions = append(versions, version)
			return false
		})
		require.NoError(t, err)
		if dir == source {
			require.Equal(t, int64(3), store.GetEarliestVersion())
			require.ElementsMatch(t, []int64{2, 3, 4}, versions)
			value, err := store.Get("sequencer", 4, []byte("key"))
			require.NoError(t, err)
			require.Equal(t, []byte("source-4"), value)
		} else {
			require.Equal(t, int64(0), store.GetEarliestVersion())
			require.ElementsMatch(t, []int64{1, 2, 3, 4}, versions)
			value, err := store.Get("sequencer", 1, []byte("key"))
			require.NoError(t, err)
			require.Equal(t, []byte("sentinel-1"), value)
		}
		require.Equal(t, int64(4), store.GetLatestVersion())
		require.NoError(t, store.Close())
	}
	for _, version := range []int64{-1, 0, 1, 4, 5} {
		require.Error(t, PruneDB("pebbledb", source, version))
	}
	store := repairM1177OpenStore(t, source)
	require.Equal(t, int64(3), store.GetEarliestVersion())
	require.Equal(t, int64(4), store.GetLatestVersion())
	require.NoError(t, store.Close())
}

func TestRepairM_1177_ReplayBounds(t *testing.T) {
	dir := t.TempDir()
	store := repairM1177OpenStore(t, dir)
	require.NoError(t, store.Close())
	stream, err := wal.NewChangelogWAL(filepath.Join(dir, "changelog"), wal.Config{})
	require.NoError(t, err)
	for i := 1; i <= 5; i++ {
		require.NoError(t, stream.Write(proto.ChangelogEntry{Version: int64(100 + i), Changesets: repairM1177Changes("sequencer", fmt.Sprintf("key-%d", i), fmt.Sprintf("value-%d", i))}))
	}
	require.NoError(t, stream.Close())
	args := []string{"--db-dir", dir, "--start-offset", "2", "--end-offset", "4"}
	expected := "Offset: 2, Height: 102\nOffset: 3, Height: 103\nOffset: 4, Height: 104\n"
	output, err := repairM1177Execute(ReplayChangelogCmd(), args...)
	require.NoError(t, err)
	require.Equal(t, expected, output)
	store = repairM1177OpenStore(t, dir)
	require.Equal(t, int64(0), store.GetLatestVersion(), "dry run mutated the DB")
	require.NoError(t, store.Close())
	output, err = repairM1177Execute(ReplayChangelogCmd(), append(args, "--no-dry-run")...)
	require.NoError(t, err)
	for i := 1; i <= 5; i++ {
		if i >= 2 && i <= 4 {
			require.Contains(t, output, fmt.Sprintf("Offset: %d, Height: %d\nRe-applied changeset for height %d\n", i, 100+i, 100+i))
		} else {
			require.NotContains(t, output, fmt.Sprintf("Offset: %d,", i))
		}
	}
	// Reopen the actual MVCC consumer; automatic composite WAL recovery is not
	// part of bounded CLI replay and would deliberately recover the other entries.
	store = repairM1177OpenStore(t, dir)
	require.Equal(t, int64(104), store.GetLatestVersion())
	for i := 1; i <= 5; i++ {
		value, err := store.Get("sequencer", 104, []byte(fmt.Sprintf("key-%d", i)))
		require.NoError(t, err)
		if i >= 2 && i <= 4 {
			require.Equal(t, []byte(fmt.Sprintf("value-%d", i)), value)
		} else {
			require.Empty(t, value)
		}
	}
	require.NoError(t, store.Close())
	// A subsequent command has no stale process-global store from the apply run.
	output, err = repairM1177Execute(ReplayChangelogCmd(), args...)
	require.NoError(t, err)
	require.Equal(t, expected, output)
	for _, bounds := range [][]string{
		{"--start-offset", "4", "--end-offset", "2"},
		{"--start-offset", "-1"},
		{"--end-offset", "18446744073709551616"},
		{"--start-offset", "2", "--end-offset", "6"},
		{"--start-offset", "18446744073709551615"},
	} {
		_, err := repairM1177Execute(ReplayChangelogCmd(), append([]string{"--db-dir", dir, "--no-dry-run"}, bounds...)...)
		require.Error(t, err)
	}
	output, err = repairM1177Execute(ReplayChangelogCmd(), "--db-dir", dir, "--start-offset", "5", "--end-offset", "5")
	require.NoError(t, err)
	require.Equal(t, "Offset: 5, Height: 105\n", output)
	// Every run closed the WAL; a real producer can immediately reopen it.
	stream, err = wal.NewChangelogWAL(filepath.Join(dir, "changelog"), wal.Config{})
	require.NoError(t, err)
	last, err := stream.LastOffset()
	require.NoError(t, err)
	require.Equal(t, uint64(5), last)
	require.NoError(t, stream.Close())
	store = repairM1177OpenStore(t, dir)
	require.Equal(t, int64(104), store.GetLatestVersion(), "invalid bounds mutated the DB")
	require.NoError(t, store.Close())
}

func TestRepairM_1177_PruneStoreInventory(t *testing.T) {
	dir := t.TempDir()
	names := []string{"sequencer", "lxphost", "custom-store", "x"}
	store := repairM1177OpenStore(t, dir)
	for version := int64(1); version <= 5; version++ {
		for _, name := range names {
			require.NoError(t, store.ApplyChangesetSync(version, repairM1177Changes(name, "key", fmt.Sprintf("%s-%d", name, version))))
		}
	}
	require.NoError(t, store.Close())
	for _, height := range []int64{2, 3} {
		_, err := repairM1177Execute(PruneCmd(), "--db-backend", "pebbledb", "--db-dir", dir, "--version", fmt.Sprint(height))
		require.NoError(t, err)
		store = repairM1177OpenStore(t, dir)
		require.Equal(t, height+1, store.GetEarliestVersion())
		require.Equal(t, int64(5), store.GetLatestVersion())
		for _, name := range names {
			var versions []int64
			_, err := store.RawIterate(name, func(key, value []byte, version int64) bool {
				versions = append(versions, version)
				return false
			})
			require.NoError(t, err)
			var expected []int64
			for version := height; version <= 5; version++ {
				expected = append(expected, version)
			}
			require.ElementsMatch(t, expected, versions, name)
			value, err := store.Get(name, height+1, []byte("key"))
			require.NoError(t, err)
			require.Equal(t, []byte(fmt.Sprintf("%s-%d", name, height+1)), value)
		}
		require.NoError(t, store.Close())
	}
}
