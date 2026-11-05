package litt_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt"
	benchmarkconfig "github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/benchmark/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/disktable/keymap"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/disktable/segment"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/littbuilder"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/util"
	"github.com/stretchr/testify/require"
)

func repairM1166Read(t *testing.T, filename string) []byte {
	t.Helper()
	data, err := os.ReadFile(filename)
	require.NoError(t, err)
	return data
}

func repairM1166Block(t *testing.T, section, language string) string {
	t.Helper()
	_, rest, ok := strings.Cut(section, "```"+language+"\n")
	require.True(t, ok, "missing %s code block", language)
	block, _, ok := strings.Cut(rest, "```")
	require.True(t, ok, "unterminated code block")
	return block
}

func repairM1166Runtime() *litt.RuntimeConfig {
	runtime := litt.DefaultRuntimeConfig()
	runtime.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	return runtime
}

func repairM1166Open(t *testing.T, config *litt.Config, tableConfig litt.TableConfig) (litt.DB, litt.Table) {
	t.Helper()
	database, err := littbuilder.NewDB(config, repairM1166Runtime())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, database.Close()) })
	table, err := database.BuildTable(tableConfig)
	require.NoError(t, err)
	return database, table
}

func repairM1166Get(t *testing.T, table litt.Table, key, expected []byte) {
	t.Helper()
	value, exists, err := table.Get(key)
	require.NoError(t, err)
	require.True(t, exists)
	require.Equal(t, expected, value)
}

func TestRepairM_1166_CLIDocInvalidJSON(t *testing.T) {
	doc := string(repairM1166Read(t, "docs/littdb_cli.md"))
	_, section, ok := strings.Cut(doc, "## `litt benchmark`")
	require.True(t, ok)
	documented := []byte(repairM1166Block(t, section, "json"))
	fixture := repairM1166Read(t, "benchmark/config/basic-config.json")
	// Compare the actual published bytes, allowing only whitespace at the block edges.
	require.Equal(t, bytes.TrimSpace(fixture), bytes.TrimSpace(documented))
	filename := filepath.Join(t.TempDir(), "documented benchmark.json")
	require.NoError(t, os.WriteFile(filename, documented, 0600))
	loaded, err := benchmarkconfig.LoadConfig(filename)
	require.NoError(t, err, "the complete documented example must load unchanged")
	basic, err := benchmarkconfig.LoadConfig("benchmark/config/basic-config.json")
	require.NoError(t, err)
	require.Equal(t, basic, loaded)
	require.Equal(t, []string{"~/benchmark/volume1", "~/benchmark/volume2", "~/benchmark/volume3"}, loaded.LittConfig.Paths)
	require.Equal(t, "~/snapshot", loaded.LittConfig.SnapshotDirectory)
	require.Equal(t, float64(1024), loaded.MaximumWriteThroughputMB)
	require.Equal(t, float64(1), loaded.MetricsLoggingPeriodSeconds)
	require.NoError(t, loaded.LittConfig.Validate())
}

func TestRepairM_1166_ConfigSignedBounds(t *testing.T) {
	minInt := -int(^uint(0)>>1) - 1
	for _, tc := range []struct {
		name  string
		field string
		set   func(*litt.Config)
	}{
		{"channel_zero", "control channel size", func(c *litt.Config) { c.ControlChannelSize = 0 }},
		{"channel_negative", "control channel size", func(c *litt.Config) { c.ControlChannelSize = -1 }},
		{"channel_minimum_int", "control channel size", func(c *litt.Config) { c.ControlChannelSize = minInt }},
		{"gc_zero", "gc period", func(c *litt.Config) { c.GCPeriod = 0 }},
		{"gc_negative", "gc period", func(c *litt.Config) { c.GCPeriod = -time.Nanosecond }},
		{"gc_minimum_duration", "gc period", func(c *litt.Config) { c.GCPeriod = time.Duration(math.MinInt64) }},
		{"metrics_zero", "metrics update interval", func(c *litt.Config) { c.MetricsUpdateInterval = 0 }},
		{"metrics_negative", "metrics update interval", func(c *litt.Config) { c.MetricsUpdateInterval = -time.Nanosecond }},
		{"metrics_minimum_duration", "metrics update interval", func(c *litt.Config) { c.MetricsUpdateInterval = time.Duration(math.MinInt64) }},
		{"flush_negative", "minimum flush interval", func(c *litt.Config) { c.MinimumFlushInterval = -time.Nanosecond }},
		{"flush_minimum_duration", "minimum flush interval", func(c *litt.Config) { c.MinimumFlushInterval = time.Duration(math.MinInt64) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			config, err := litt.DefaultConfig(filepath.Join(root, "data"))
			require.NoError(t, err)
			config.SnapshotDirectory = filepath.Join(root, "snapshots")
			config.MetricsEnabled = true
			config.MetricsPort = 0
			tc.set(config)
			require.ErrorContains(t, config.Validate(), tc.field)
			// A direct caller must get a validation error before directories, locks or metrics start.
			database, err := littbuilder.NewDB(config, repairM1166Runtime())
			if database != nil {
				t.Cleanup(func() { require.NoError(t, database.Close()) })
			}
			require.Nil(t, database)
			require.ErrorContains(t, err, "error checking config")
			require.ErrorContains(t, err, tc.field)
			entries, err := os.ReadDir(root)
			require.NoError(t, err)
			require.Empty(t, entries, "invalid config must not acquire filesystem resources")

			// The same signed values must fail at the JSON producer/loader boundary, too.
			benchmark := benchmarkconfig.DefaultBenchmarkConfig()
			benchmark.LittConfig = config
			encoded, err := json.Marshal(benchmark)
			require.NoError(t, err)
			filename := filepath.Join(t.TempDir(), "invalid.json")
			require.NoError(t, os.WriteFile(filename, encoded, 0600))
			loaded, err := benchmarkconfig.LoadConfig(filename)
			require.Nil(t, loaded)
			require.ErrorContains(t, err, tc.field)
			entries, err = os.ReadDir(root)
			require.NoError(t, err)
			require.Empty(t, entries)
		})
	}

	t.Run("positive_boundaries", func(t *testing.T) {
		config, err := litt.DefaultConfig(filepath.Join(t.TempDir(), "data"))
		require.NoError(t, err)
		config.ControlChannelSize = 1
		config.GCPeriod = time.Nanosecond
		config.MetricsEnabled = true
		config.MetricsUpdateInterval = time.Nanosecond
		config.MinimumFlushInterval = time.Nanosecond
		require.NoError(t, config.Validate())
		config.ControlChannelSize = int(^uint(0) >> 1)
		config.GCPeriod = time.Duration(math.MaxInt64)
		config.MetricsUpdateInterval = time.Duration(math.MaxInt64)
		config.MinimumFlushInterval = time.Duration(math.MaxInt64)
		require.NoError(t, config.Validate())
	})

	for _, ignoredInterval := range []time.Duration{0, -time.Nanosecond, time.Duration(math.MinInt64)} {
		t.Run(fmt.Sprintf("disabled_metrics_%d", ignoredInterval), func(t *testing.T) {
			config, err := litt.DefaultConfig(filepath.Join(t.TempDir(), "data"))
			require.NoError(t, err)
			config.KeymapType = keymap.MemKeymapType
			config.ControlChannelSize = 1
			config.MetricsEnabled = false
			config.MetricsUpdateInterval = ignoredInterval
			config.MinimumFlushInterval = 0 // documented immediate-flush mode remains valid
			require.NoError(t, config.Validate())
			tableConfig := litt.DefaultTableConfig("disabled")
			database, table := repairM1166Open(t, config, tableConfig)
			require.NoError(t, table.Put([]byte("key"), []byte("value")))
			require.NoError(t, table.Flush())
			require.NoError(t, database.Close())
			_, reopened := repairM1166Open(t, config, tableConfig)
			repairM1166Get(t, reopened, []byte("key"), []byte("value"))
		})
	}
}

func repairM1166FieldDoc(t *testing.T, filename, typeName, fieldName string) string {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), filename, nil, parser.ParseComments)
	require.NoError(t, err)
	var doc string
	ast.Inspect(file, func(node ast.Node) bool {
		typ, ok := node.(*ast.TypeSpec)
		if !ok || typ.Name.Name != typeName {
			return true
		}
		var fields *ast.FieldList
		switch body := typ.Type.(type) {
		case *ast.InterfaceType:
			fields = body.Methods
		case *ast.StructType:
			fields = body.Fields
		}
		require.NotNil(t, fields)
		for _, field := range fields.List {
			for _, name := range field.Names {
				if name.Name == fieldName {
					doc = field.Doc.Text()
				}
			}
		}
		return false
	})
	require.NotEmpty(t, doc, "%s.%s must have documentation", typeName, fieldName)
	return doc
}

func TestRepairM_1166_TableAPISizeDoc(t *testing.T) {
	for _, method := range []string{"Put", "PutBatch", "Get"} {
		doc := repairM1166FieldDoc(t, "table.go", "Table", method)
		require.Contains(t, doc, "65,535 bytes (math.MaxUint16)")
		require.Contains(t, doc, "4,294,967,295 bytes (math.MaxUint32)")
	}
	for _, field := range []string{"Key", "SecondaryKeys"} {
		require.Contains(t, repairM1166FieldDoc(t, "types/put_request.go", "PutRequest", field), "65,535 bytes (math.MaxUint16)")
	}
	require.Contains(t, repairM1166FieldDoc(t, "types/put_request.go", "PutRequest", "Value"), "4,294,967,295 bytes (math.MaxUint32)")
	require.Contains(t, repairM1166FieldDoc(t, "types/secondary_key.go", "SecondaryKey", "Key"), "65,535 bytes (math.MaxUint16)")
	readme := string(repairM1166Read(t, "README.md"))
	require.Contains(t, readme, "primary and secondary keys up to 65,535 bytes (2^16 - 1)")
	require.Contains(t, readme, "values up to 4,294,967,295 bytes (2^32 - 1, `math.MaxUint32`)")
	require.Contains(t, readme, "Key           []byte // at most 65,535 bytes (math.MaxUint16)")
	require.Contains(t, readme, "Value         []byte // at most 4,294,967,295 bytes (math.MaxUint32)")
	require.Equal(t, math.MaxUint16, segment.MaxKeyLength)

	// The exact value limit is representable in the real address codec without a 4 GiB allocation.
	address := types.NewAddress(7, 0, 2, math.MaxUint32)
	decoded, err := types.DeserializeAddress(address.Serialize())
	require.NoError(t, err)
	require.Equal(t, address, decoded)
	require.Equal(t, uint32(math.MaxUint32), decoded.ValueSize())

	for _, keymapType := range []keymap.KeymapType{keymap.MemKeymapType, keymap.PebbleDBKeymapType} {
		t.Run(string(keymapType), func(t *testing.T) {
			config, err := litt.DefaultConfig(filepath.Join(t.TempDir(), "data"))
			require.NoError(t, err)
			config.KeymapType = keymapType
			tableConfig := litt.DefaultTableConfig("limits")
			database, table := repairM1166Open(t, config, tableConfig)
			value := []byte("primary-value")
			primary := bytes.Repeat([]byte("p"), math.MaxUint16)
			alias := bytes.Repeat([]byte("s"), math.MaxUint16)
			batchKey := bytes.Repeat([]byte("b"), math.MaxUint16)
			batchAlias := bytes.Repeat([]byte("a"), math.MaxUint16)
			secondary := func(key []byte) *types.SecondaryKey { return &types.SecondaryKey{Key: key, Offset: 2, Length: 4} }
			require.NoError(t, table.Put(primary, value, secondary(alias)))
			require.NoError(t, table.PutBatch([]*types.PutRequest{{Key: batchKey, Value: value, SecondaryKeys: []*types.SecondaryKey{secondary(batchAlias)}}}))
			tooLong := bytes.Repeat([]byte("x"), math.MaxUint16+1)
			require.Error(t, table.Put(tooLong, value))
			require.Error(t, table.Put([]byte("rejected-put"), value, secondary(tooLong)))
			for _, bad := range []*types.PutRequest{
				{Key: tooLong, Value: value},
				{Key: []byte("rejected-batch"), Value: value, SecondaryKeys: []*types.SecondaryKey{secondary(tooLong)}},
			} {
				require.Error(t, table.PutBatch([]*types.PutRequest{{Key: []byte("must-not-leak"), Value: value}, bad}))
			}
			require.Equal(t, uint64(4), table.KeyCount())
			for _, key := range [][]byte{tooLong, []byte("rejected-put"), []byte("rejected-batch"), []byte("must-not-leak")} {
				_, exists, err := table.Get(key)
				require.NoError(t, err)
				require.False(t, exists)
			}
			require.NoError(t, table.Flush())
			repairM1166Get(t, table, primary, value)
			repairM1166Get(t, table, alias, value[2:6])
			require.NoError(t, database.Close())
			_, reopened := repairM1166Open(t, config, tableConfig)
			require.Equal(t, uint64(4), reopened.KeyCount())
			for _, key := range [][]byte{primary, batchKey} {
				repairM1166Get(t, reopened, key, value)
			}
			for _, key := range [][]byte{alias, batchAlias} {
				repairM1166Get(t, reopened, key, value[2:6])
			}
		})
	}
}

func TestRepairM_1166_CLIDocRestoreMode(t *testing.T) {
	doc := string(repairM1166Read(t, "docs/littdb_cli.md"))
	_, section, ok := strings.Cut(doc, "### Restoring from a Backup\n")
	require.True(t, ok)
	section, _, _ = strings.Cut(section, "\n### ")
	command := strings.Fields(strings.ReplaceAll(repairM1166Block(t, section, "bash"), "\\\n", " "))
	require.Equal(t, []string{"litt", "push"}, command[:2])
	var sources, destinations []string
	noGC := false
	for i := 2; i < len(command)-1; i++ {
		switch command[i] {
		case "--no-gc":
			noGC = true
		case "--src":
			i++
			sources = append(sources, command[i])
		case "--dst":
			i++
			destinations = append(destinations, command[i])
		default:
			t.Fatalf("unexpected restore option %q", command[i])
		}
	}
	require.True(t, noGC, "ordinary archives require --no-gc")
	require.Equal(t, []string{"/backup1", "/backup2", "/backup3"}, sources)
	require.Equal(t, []string{"/restore0", "/restore1"}, destinations)
	require.Equal(t, "user@target-host", command[len(command)-1])
	require.Contains(t, section, "Stop the target application")
	require.Contains(t, section, "Keep the target stopped until the transfer succeeds")
	require.Contains(t, section, "preserves every archive source file")
	// Tie the documented option to the real CLI's admission and deletion branches.
	pushSource := string(repairM1166Read(t, "cli/push.go"))
	require.Contains(t, pushSource, `deleteAfterTransfer := !ctx.Bool("no-gc")`)
	require.Contains(t, pushSource, "} else if deleteAfterTransfer {\n\t\treturn fmt.Errorf(\"--no-gc is required when pushing a non-snapshot table\")")
	require.Contains(t, pushSource, "if deleteAfterTransfer {\n\t\tutil.True(isSnapshot")

	root := t.TempDir()
	for i, source := range sources {
		sources[i] = filepath.Join(root, filepath.Base(source))
	}
	for i, destination := range destinations {
		destinations[i] = filepath.Join(root, filepath.Base(destination))
	}
	config, err := litt.DefaultConfig(sources...)
	require.NoError(t, err)
	config.KeymapType = keymap.MemKeymapType
	config.MaxSegmentKeyCount = 4
	tableConfig := litt.DefaultTableConfig("archive")
	tableConfig.ShardingFactor = 3
	database, table := repairM1166Open(t, config, tableConfig)
	expected := make(map[string][]byte)
	for i := 0; i < 6; i++ {
		key := []byte(fmt.Sprintf("primary-%d", i))
		alias := []byte(fmt.Sprintf("alias-%d", i))
		value := []byte(fmt.Sprintf("value-%d-payload", i))
		require.NoError(t, table.Put(key, value, &types.SecondaryKey{Key: alias, Offset: 2, Length: 5}))
		expected[string(key)] = value
		expected[string(alias)] = value[2:7]
	}
	require.NoError(t, table.Flush())
	require.NoError(t, database.Close()) // stopped source, as documented

	// Discover the actual ordinary segment archive using the same scanner as pushTable.
	paths, err := segment.BuildSegmentPaths(sources, "", tableConfig.Name)
	require.NoError(t, err)
	monitor := util.NewErrorMonitor(context.Background(), repairM1166Runtime().Logger, nil)
	_, _, segments, err := segment.GatherSegmentFiles(repairM1166Runtime().Logger, monitor, paths, false, time.Now(), false, true)
	require.NoError(t, err)
	require.NotEmpty(t, segments)
	archiveHashes := make(map[string][sha256.Size]byte)
	usedSources := make(map[string]bool)
	copied := 0
	for _, seg := range segments {
		isSnapshot, err := seg.IsSnapshot()
		require.NoError(t, err)
		require.False(t, isSnapshot, "a received archive consists of ordinary files")
		for _, filename := range seg.GetFilePaths() {
			archiveHashes[filename] = sha256.Sum256(repairM1166Read(t, filename))
			usedSources[filepath.Dir(filepath.Dir(filepath.Dir(filename)))] = true
			info, err := os.Lstat(filename)
			require.NoError(t, err)
			require.True(t, info.Mode().IsRegular())
			// The file transport is local and preserves the real producer's bytes. No SSH/rsync process is needed.
			destination := filepath.Join(destinations[copied%len(destinations)], tableConfig.Name, segment.SegmentDirectory, filepath.Base(filename))
			require.NoError(t, util.CopyRegularFile(filename, destination, true))
			copied++
		}
	}
	require.Len(t, usedSources, len(sources), "every documented source root contributes archive files")
	for _, destination := range destinations {
		_, err := os.Stat(filepath.Join(destination, tableConfig.Name, keymap.KeymapDirectoryName))
		require.ErrorIs(t, err, os.ErrNotExist, "the restore fixture contains segments, not a prebuilt keymap")
	}
	restoredConfig, err := litt.DefaultConfig(destinations...)
	require.NoError(t, err)
	restoredConfig.KeymapType = keymap.MemKeymapType
	for restart := 0; restart < 2; restart++ {
		restoredDB, restoredTable := repairM1166Open(t, restoredConfig, tableConfig)
		require.Equal(t, uint64(len(expected)), restoredTable.KeyCount())
		for key, value := range expected {
			repairM1166Get(t, restoredTable, []byte(key), value)
		}
		require.NoError(t, restoredDB.Close())
	}
	for filename, hash := range archiveHashes {
		require.Equal(t, hash, sha256.Sum256(repairM1166Read(t, filename)), "archive remains retained: %s", filename)
	}
}
