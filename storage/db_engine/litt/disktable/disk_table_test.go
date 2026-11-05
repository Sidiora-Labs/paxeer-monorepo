package disktable

import (
	"fmt"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/disktable/keymap"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/disktable/segment"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/util"
	"github.com/stretchr/testify/require"
)

// This file contains tests that are specific to the disk table implementation. Other more general test scenarios
// are defined in litt/test/table_test.go.

type tableBuilder struct {
	name    string
	builder func(clock func() time.Time, name string, paths []string) (litt.ManagedTable, error)
}

// This test executes against different table implementations. This is useful for distinguishing between bugs that
// are present in an implementation, and bugs that are present in the test scenario itself.
var tableBuilders = []*tableBuilder{
	{
		name:    "MemKeyDiskTableSingleShard",
		builder: buildMemKeyDiskTableSingleShard,
	},
	{
		name:    "MemKeyDiskTableMultiShard",
		builder: buildMemKeyDiskTableMultiShard,
	},
	{
		name:    "PebbleDBKeyDiskTableSingleShard",
		builder: buildPebbleDBKeyDiskTableSingleShard,
	},
	{
		name:    "PebbleDBKeyDiskTableMultiShard",
		builder: buildPebbleDBKeyDiskTableMultiShard,
	},
}

func setupKeymapTypeFile(keymapPath string, keymapType keymap.KeymapType) (*keymap.KeymapTypeFile, error) {
	exists, err := keymap.KeymapFileExists(keymapPath)
	if err != nil {
		return nil, fmt.Errorf("failed to check if keymap file exists: %w", err)
	}
	var keymapTypeFile *keymap.KeymapTypeFile
	if exists {
		keymapTypeFile, err = keymap.LoadKeymapTypeFile(keymapPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load keymap type file: %w", err)
		}
	} else {
		err = os.MkdirAll(keymapPath, os.ModePerm)
		if err != nil {
			return nil, fmt.Errorf("failed to create keymap directory: %w", err)
		}
		keymapTypeFile = keymap.NewKeymapTypeFile(keymapPath, keymapType)
		err = keymapTypeFile.Write()
		if err != nil {
			return nil, fmt.Errorf("failed to create keymap type file: %w", err)
		}
	}

	return keymapTypeFile, nil
}

func buildMemKeyDiskTableSingleShard(
	clock func() time.Time,
	name string,
	paths []string) (litt.ManagedTable, error) {

	logger := slog.Default()

	keymapPath := filepath.Join(paths[0], keymap.KeymapDirectoryName)
	keymapTypeFile, err := setupKeymapTypeFile(keymapPath, keymap.MemKeymapType)
	if err != nil {
		return nil, fmt.Errorf("failed to load keymap type file: %w", err)
	}

	keys, _, err := keymap.NewMemKeymap(logger, "", true)
	if err != nil {
		return nil, fmt.Errorf("failed to create keymap: %w", err)
	}

	roots := make([]string, 0, len(paths))
	roots = append(roots, paths...)

	config, err := litt.DefaultConfig(paths...)
	if err != nil {
		return nil, fmt.Errorf("failed to create config: %w", err)
	}

	config.TargetSegmentFileSize = 100 // intentionally use a very small segment size
	config.GCPeriod = time.Millisecond
	config.Fsync = false

	runtimeConfig := litt.DefaultRuntimeConfig()
	runtimeConfig.Clock = clock
	runtimeConfig.Logger = logger

	table, err := NewDiskTable(
		config,
		runtimeConfig,
		name,
		litt.DefaultTableConfig(name),
		keys,
		keymapPath,
		keymapTypeFile,
		roots,
		true,
		nil)

	if err != nil {
		return nil, fmt.Errorf("failed to create disk table: %w", err)
	}

	return table, nil
}

func buildMemKeyDiskTableMultiShard(
	clock func() time.Time,
	name string,
	paths []string) (litt.ManagedTable, error) {

	logger := slog.Default()

	keymapPath := filepath.Join(paths[0], keymap.KeymapDirectoryName)
	keymapTypeFile, err := setupKeymapTypeFile(keymapPath, keymap.MemKeymapType)
	if err != nil {
		return nil, fmt.Errorf("failed to load keymap type file: %w", err)
	}

	keys, _, err := keymap.NewMemKeymap(logger, "", true)
	if err != nil {
		return nil, fmt.Errorf("failed to create keymap: %w", err)
	}

	config, err := litt.DefaultConfig(paths...)
	if err != nil {
		return nil, fmt.Errorf("failed to create config: %w", err)
	}

	config.TargetSegmentFileSize = 100 // intentionally use a very small segment size
	config.GCPeriod = time.Millisecond
	config.Fsync = false

	tableConfig := litt.DefaultTableConfig(name)
	tableConfig.ShardingFactor = 4

	runtimeConfig := litt.DefaultRuntimeConfig()
	runtimeConfig.Clock = clock
	runtimeConfig.Logger = logger

	table, err := NewDiskTable(
		config,
		runtimeConfig,
		name,
		tableConfig,
		keys,
		keymapPath,
		keymapTypeFile,
		paths,
		true,
		nil)

	if err != nil {
		return nil, fmt.Errorf("failed to create disk table: %w", err)
	}

	return table, nil
}

func buildPebbleDBKeyDiskTableSingleShard(
	clock func() time.Time,
	name string,
	paths []string) (litt.ManagedTable, error) {

	logger := slog.Default()
	keymapPath := filepath.Join(paths[0], keymap.KeymapDirectoryName)
	keymapTypeFile, err := setupKeymapTypeFile(keymapPath, keymap.UnsafePebbleDBKeymapType)
	if err != nil {
		return nil, fmt.Errorf("failed to load keymap type file: %w", err)
	}

	keys, _, err := keymap.NewUnsafePebbleDBKeymap(logger, keymapPath, false)
	if err != nil {
		return nil, fmt.Errorf("failed to create keymap: %w", err)
	}

	config, err := litt.DefaultConfig(paths...)
	if err != nil {
		return nil, fmt.Errorf("failed to create config: %w", err)
	}

	config.TargetSegmentFileSize = 100 // intentionally use a very small segment size
	config.GCPeriod = time.Millisecond
	config.Fsync = false

	runtimeConfig := litt.DefaultRuntimeConfig()
	runtimeConfig.Clock = clock
	runtimeConfig.Logger = logger

	table, err := NewDiskTable(
		config,
		runtimeConfig,
		name,
		litt.DefaultTableConfig(name),
		keys,
		keymapPath,
		keymapTypeFile,
		paths,
		false,
		nil)

	if err != nil {
		return nil, fmt.Errorf("failed to create disk table: %w", err)
	}

	return table, nil
}

func buildPebbleDBKeyDiskTableMultiShard(
	clock func() time.Time,
	name string,
	paths []string) (litt.ManagedTable, error) {

	logger := slog.Default()
	keymapPath := filepath.Join(paths[0], name, keymap.KeymapDirectoryName)
	keymapTypeFile, err := setupKeymapTypeFile(keymapPath, keymap.UnsafePebbleDBKeymapType)
	if err != nil {
		return nil, fmt.Errorf("failed to load keymap type file: %w", err)
	}

	keys, _, err := keymap.NewUnsafePebbleDBKeymap(logger, keymapPath, true)
	if err != nil {
		return nil, fmt.Errorf("failed to create keymap: %w", err)
	}

	config, err := litt.DefaultConfig(paths...)
	if err != nil {
		return nil, fmt.Errorf("failed to create config: %w", err)
	}

	config.TargetSegmentFileSize = 100 // intentionally use a very small segment size
	config.GCPeriod = time.Millisecond
	config.Fsync = false

	tableConfig := litt.DefaultTableConfig(name)
	tableConfig.ShardingFactor = 4

	runtimeConfig := litt.DefaultRuntimeConfig()
	runtimeConfig.Clock = clock
	runtimeConfig.Logger = logger

	table, err := NewDiskTable(
		config,
		runtimeConfig,
		name,
		tableConfig,
		keys,
		keymapPath,
		keymapTypeFile,
		paths,
		false,
		nil)

	if err != nil {
		return nil, fmt.Errorf("failed to create disk table: %w", err)
	}

	return table, nil
}

func restartTest(t *testing.T, tableBuilder *tableBuilder) {
	rand := util.NewTestRandom()

	directory := t.TempDir()

	tableName := rand.String(8)
	table, err := tableBuilder.builder(time.Now, tableName, []string{directory})
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	require.Equal(t, tableName, table.Name())

	expectedValues := make(map[string][]byte)

	iterations := 1000
	restartIteration := iterations/2 + int(rand.Int64Range(-10, 10))

	for i := 0; i < iterations; i++ {

		// Somewhere in the middle of the test, restart the table.
		if i == restartIteration {
			ok, _ := table.(*DiskTable).errorMonitor.IsOk()
			require.True(t, ok)
			err = table.Close()
			require.NoError(t, err)

			table, err = tableBuilder.builder(time.Now, tableName, []string{directory})
			require.NoError(t, err)

			// Do a full scan of the table to verify that all expected values are still present.
			for expectedKey, expectedValue := range expectedValues {
				value, ok, err := table.Get([]byte(expectedKey))
				require.NoError(t, err)
				require.True(t, ok, "key %s not found", expectedKey)
				require.Equal(t, expectedValue, value)
			}

			// Try fetching a value that isn't in the table.
			_, ok, err := table.Get(rand.PrintableVariableBytes(32, 64))
			require.NoError(t, err)
			require.False(t, ok)
		}

		// Write some data.
		batchSize := rand.Int32Range(1, 10)

		if batchSize == 1 {
			key := rand.PrintableVariableBytes(32, 64)
			value := rand.PrintableVariableBytes(1, 128)
			err = table.Put(key, value)
			require.NoError(t, err)
			expectedValues[string(key)] = value
		} else {
			batch := make([]*types.PutRequest, 0, batchSize)
			for j := int32(0); j < batchSize; j++ {
				key := rand.PrintableVariableBytes(32, 64)
				value := rand.PrintableVariableBytes(1, 128)
				batch = append(batch, &types.PutRequest{Key: key, Value: value})
				expectedValues[string(key)] = value
			}
			err = table.PutBatch(batch)
			require.NoError(t, err)
		}

		// Once in a while, flush the table.
		if rand.BoolWithProbability(0.1) {
			err = table.Flush()
			require.NoError(t, err)
		}

		// Once in a while, sleep for a short time. For tables that do garbage collection, the garbage
		// collection interval has been configured to be 1ms. Sleeping 5ms should be enough to give
		// the garbage collector a chance to run.
		if rand.BoolWithProbability(0.01) {
			time.Sleep(5 * time.Millisecond)
		}

		// Once in a while, scan the table and verify that all expected values are present.
		// Don't do this every time for the sake of test runtime.
		if rand.BoolWithProbability(0.01) || i == iterations-1 /* always check on the last iteration */ {

			for expectedKey, expectedValue := range expectedValues {
				value, ok, err := table.Get([]byte(expectedKey))
				require.NoError(t, err)
				require.True(t, ok)
				require.Equal(t, expectedValue, value)
			}

			// Try fetching a value that isn't in the table.
			_, ok, err := table.Get(rand.PrintableVariableBytes(32, 64))
			require.NoError(t, err)
			require.False(t, ok)
		}
	}

	ok, _ := table.(*DiskTable).errorMonitor.IsOk()
	require.True(t, ok)
	err = table.Drop()
	require.NoError(t, err)

	// ensure that the test directory is empty
	entries, err := os.ReadDir(directory)
	require.NoError(t, err)
	require.Empty(t, entries)
}

func TestRestart(t *testing.T) {
	t.Parallel()
	for _, tb := range tableBuilders {
		t.Run(tb.name, func(t *testing.T) {
			restartTest(t, tb)
		})
	}
}

// This test deletes a random file from a middle segment. This is considered unrecoverable corruption, and should
// cause the table to fail to restart.
func middleFileMissingTest(t *testing.T, tableBuilder *tableBuilder, typeToDelete string) {
	rand := util.NewTestRandom()

	logger := slog.Default()

	directory := t.TempDir()

	tableName := rand.String(8)
	table, err := tableBuilder.builder(time.Now, tableName, []string{directory})
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	require.Equal(t, tableName, table.Name())

	expectedValues := make(map[string][]byte)

	// Fill the table with random data.
	iterations := 100
	for i := 0; i < iterations; i++ {
		batchSize := rand.Int32Range(1, 10)
		if batchSize == 1 {
			key := rand.PrintableVariableBytes(32, 64)
			value := rand.PrintableVariableBytes(1, 128)
			err = table.Put(key, value)
			require.NoError(t, err)
			expectedValues[string(key)] = value
		} else {
			batch := make([]*types.PutRequest, 0, batchSize)
			for j := int32(0); j < batchSize; j++ {
				key := rand.PrintableVariableBytes(32, 64)
				value := rand.PrintableVariableBytes(1, 128)
				batch = append(batch, &types.PutRequest{Key: key, Value: value})
				expectedValues[string(key)] = value
			}
			err = table.PutBatch(batch)
			require.NoError(t, err)
		}
	}

	// Stop the table
	ok, _ := table.(*DiskTable).errorMonitor.IsOk()
	require.True(t, ok)
	err = table.Close()
	require.NoError(t, err)

	errorMonitor := table.(*DiskTable).errorMonitor

	// Delete a file in the middle of the sequence of segments.
	segmentPath, err := segment.NewSegmentPath(directory, "", tableName)
	require.NoError(t, err)
	lowestSegmentIndex, highestSegmentIndex, _, err := segment.GatherSegmentFiles(
		logger,
		errorMonitor,
		[]*segment.SegmentPath{segmentPath},
		false,
		time.Now(),
		true,
		false)
	require.NoError(t, err)

	middleIndex := lowestSegmentIndex + (highestSegmentIndex-lowestSegmentIndex)/2

	filePath := ""
	if typeToDelete == "key" {
		filePath = fmt.Sprintf("%s/%s/segments/%d%s",
			directory, tableName, middleIndex, segment.KeyFileExtension)
	} else if typeToDelete == "value" {
		shardingFactor := table.(*DiskTable).getShardingFactor()
		shard := rand.Uint32Range(0, uint32(shardingFactor))
		filePath = fmt.Sprintf("%s/%s/segments/%d-%d%s",
			directory, tableName, middleIndex, shard, segment.ValuesFileExtension)
	} else {
		filePath = fmt.Sprintf("%s/%s/segments/%d%s",
			directory, tableName, middleIndex, segment.MetadataFileExtension)
	}

	exists, err := util.Exists(filePath)
	require.NoError(t, err)
	require.True(t, exists)

	err = os.Remove(filePath)
	require.NoError(t, err)

	// files in segments directory should not be changed as a result of the deletion
	files, err := os.ReadDir(fmt.Sprintf("%s/%s/segments", directory, tableName))
	require.NoError(t, err)

	// Restart the table. This should fail.
	table, err = tableBuilder.builder(time.Now, tableName, []string{directory})
	require.Error(t, err)
	require.Nil(t, table)

	// Ensure that no files were added or removed from the segments directory.
	filesAfterRestart, err := os.ReadDir(fmt.Sprintf("%s/%s/segments", directory, tableName))
	require.NoError(t, err)
	require.Equal(t, len(files), len(filesAfterRestart))
	filesSet := make(map[string]struct{})
	for _, file := range files {
		filesSet[file.Name()] = struct{}{}
	}
	for _, file := range filesAfterRestart {
		require.Contains(t, filesSet, file.Name())
	}
}

func TestMiddleFileMissing(t *testing.T) {
	t.Parallel()
	for _, tb := range tableBuilders {
		t.Run("key-"+tb.name, func(t *testing.T) {
			middleFileMissingTest(t, tb, "key")
		})
		t.Run("value-"+tb.name, func(t *testing.T) {
			middleFileMissingTest(t, tb, "value")
		})
		t.Run("metadata-"+tb.name, func(t *testing.T) {
			middleFileMissingTest(t, tb, "metadata")
		})
	}
}

// This test deletes a random file from the first segment. This is considered recoverable, since it can happen
// if the table crashes during garbage collection.
func initialFileMissingTest(t *testing.T, tableBuilder *tableBuilder, typeToDelete string) {
	rand := util.NewTestRandom()

	logger := slog.Default()
	directory := t.TempDir()

	tableName := rand.String(8)
	table, err := tableBuilder.builder(time.Now, tableName, []string{directory})
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	require.Equal(t, tableName, table.Name())

	expectedValues := make(map[string][]byte)

	// Fill the table with random data.
	iterations := 100
	for i := 0; i < iterations; i++ {
		batchSize := rand.Int32Range(1, 10)
		if batchSize == 1 {
			key := rand.PrintableVariableBytes(32, 64)
			value := rand.PrintableVariableBytes(1, 128)
			err = table.Put(key, value)
			require.NoError(t, err)
			expectedValues[string(key)] = value
		} else {
			batch := make([]*types.PutRequest, 0, batchSize)
			for j := int32(0); j < batchSize; j++ {
				key := rand.PrintableVariableBytes(32, 64)
				value := rand.PrintableVariableBytes(1, 128)
				batch = append(batch, &types.PutRequest{Key: key, Value: value})
				expectedValues[string(key)] = value
			}
			err = table.PutBatch(batch)
			require.NoError(t, err)
		}
	}

	// Stop the table
	ok, _ := table.(*DiskTable).errorMonitor.IsOk()
	require.True(t, ok)
	err = table.Close()
	require.NoError(t, err)

	segmentPath, err := segment.NewSegmentPath(directory, "", tableName)
	require.NoError(t, err)
	lowestSegmentIndex, _, segments, err := segment.GatherSegmentFiles(
		logger,
		table.(*DiskTable).errorMonitor,
		[]*segment.SegmentPath{segmentPath},
		false,
		time.Now(),
		true,
		false)
	require.NoError(t, err)

	// All keys in the initial segment are expected to be missing after the restart.
	missingKeys := make(map[string]struct{})
	segmentKeys, err := segments[lowestSegmentIndex].GetKeys()
	require.NoError(t, err)
	for _, key := range segmentKeys {
		missingKeys[string(key.Key)] = struct{}{}
	}

	// Delete a file in the initial segment.
	filePath := ""
	if typeToDelete == "key" {
		filePath = fmt.Sprintf("%s/%s/segments/%d%s",
			directory, tableName, lowestSegmentIndex, segment.KeyFileExtension)
	} else if typeToDelete == "value" {
		shardingFactor := table.(*DiskTable).getShardingFactor()
		shard := rand.Uint32Range(0, uint32(shardingFactor))
		filePath = fmt.Sprintf(
			"%s/%s/segments/%d-%d%s",
			directory, tableName, lowestSegmentIndex, shard, segment.ValuesFileExtension)
	} else {
		filePath = fmt.Sprintf("%s/%s/segments/%d%s",
			directory, tableName, lowestSegmentIndex, segment.MetadataFileExtension)
	}
	exists, err := util.Exists(filePath)
	require.NoError(t, err)
	require.True(t, exists)

	err = os.Remove(filePath)
	require.NoError(t, err)

	// Restart the table.
	table, err = tableBuilder.builder(time.Now, tableName, []string{directory})
	require.NoError(t, err)

	// Check the data in the table.
	for expectedKey, expectedValue := range expectedValues {
		if _, expectedToBeMissing := missingKeys[expectedKey]; expectedToBeMissing {
			_, ok, err := table.Get([]byte(expectedKey))
			require.NoError(t, err)
			require.False(t, ok)
		} else {
			value, ok, err := table.Get([]byte(expectedKey))
			require.NoError(t, err)
			require.True(t, ok)
			require.Equal(t, expectedValue, value)
		}
	}

	// Remove the missing values from the expected values map. Simplifies following checks.
	for key := range missingKeys {
		delete(expectedValues, key)
	}

	// Make additional modifications to the table to ensure that it is still working.
	for i := 0; i < iterations; i++ {

		// Write some data.
		batchSize := rand.Int32Range(1, 10)

		if batchSize == 1 {
			key := rand.PrintableVariableBytes(32, 64)
			value := rand.PrintableVariableBytes(1, 128)
			err = table.Put(key, value)
			require.NoError(t, err)
			expectedValues[string(key)] = value
		} else {
			batch := make([]*types.PutRequest, 0, batchSize)
			for j := int32(0); j < batchSize; j++ {
				key := rand.PrintableVariableBytes(32, 64)
				value := rand.PrintableVariableBytes(1, 128)
				batch = append(batch, &types.PutRequest{Key: key, Value: value})
				expectedValues[string(key)] = value
			}
			err = table.PutBatch(batch)
			require.NoError(t, err)
		}

		// Once in a while, flush the table.
		if rand.BoolWithProbability(0.1) {
			err = table.Flush()
			require.NoError(t, err)
		}

		// Once in a while, sleep for a short time. For tables that do garbage collection, the garbage
		// collection interval has been configured to be 1ms. Sleeping 5ms should be enough to give
		// the garbage collector a chance to run.
		if rand.BoolWithProbability(0.01) {
			time.Sleep(5 * time.Millisecond)
		}

		// Once in a while, scan the table and verify that all expected values are present.
		// Don't do this every time for the sake of test runtime.
		if rand.BoolWithProbability(0.01) || i == iterations-1 /* always check on the last iteration */ {
			for expectedKey, expectedValue := range expectedValues {
				value, ok, err := table.Get([]byte(expectedKey))
				require.NoError(t, err)
				require.True(t, ok)
				require.Equal(t, expectedValue, value)
			}

			// Try fetching a value that isn't in the table.
			_, ok, err := table.Get(rand.PrintableVariableBytes(32, 64))
			require.NoError(t, err)
			require.False(t, ok)
		}
	}

	ok, _ = table.(*DiskTable).errorMonitor.IsOk()
	require.True(t, ok)
	err = table.Drop()
	require.NoError(t, err)

	// ensure that the test directory is empty
	entries, err := os.ReadDir(directory)
	require.NoError(t, err)
	require.Empty(t, entries)
}

func TestInitialFileMissing(t *testing.T) {
	t.Parallel()
	for _, tb := range tableBuilders {
		t.Run("key-"+tb.name, func(t *testing.T) {
			initialFileMissingTest(t, tb, "key")
		})
		t.Run("value-"+tb.name, func(t *testing.T) {
			initialFileMissingTest(t, tb, "value")
		})
		t.Run("metadata-"+tb.name, func(t *testing.T) {
			initialFileMissingTest(t, tb, "metadata")
		})
	}
}

// This test deletes a random file from the last segment. This can happen if the table crashes prior to the
// last segment being flushed.
func lastFileMissingTest(t *testing.T, tableBuilder *tableBuilder, typeToDelete string) {
	rand := util.NewTestRandom()

	logger := slog.Default()
	directory := t.TempDir()

	tableName := rand.String(8)
	table, err := tableBuilder.builder(time.Now, tableName, []string{directory})
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	require.Equal(t, tableName, table.Name())

	expectedValues := make(map[string][]byte)

	// Fill the table with random data.
	iterations := 100
	for i := 0; i < iterations; i++ {
		batchSize := rand.Int32Range(1, 10)
		if batchSize == 1 {
			key := rand.PrintableVariableBytes(32, 64)
			value := rand.PrintableVariableBytes(1, 128)
			err = table.Put(key, value)
			require.NoError(t, err)
			expectedValues[string(key)] = value
		} else {
			batch := make([]*types.PutRequest, 0, batchSize)
			for j := int32(0); j < batchSize; j++ {
				key := rand.PrintableVariableBytes(32, 64)
				value := rand.PrintableVariableBytes(1, 128)
				batch = append(batch, &types.PutRequest{Key: key, Value: value})
				expectedValues[string(key)] = value
			}
			err = table.PutBatch(batch)
			require.NoError(t, err)
		}
	}

	// Stop the table
	ok, _ := table.(*DiskTable).errorMonitor.IsOk()
	require.True(t, ok)
	err = table.Close()
	require.NoError(t, err)

	segmentPath, err := segment.NewSegmentPath(directory, "", tableName)
	require.NoError(t, err)
	_, highestSegmentIndex, segments, err := segment.GatherSegmentFiles(
		logger,
		table.(*DiskTable).errorMonitor,
		[]*segment.SegmentPath{segmentPath},
		false,
		time.Now(),
		true,
		false)
	require.NoError(t, err)

	// All keys in the final segment are expected to be missing after the restart.
	missingKeys := make(map[string]struct{})
	segmentKeys, err := segments[highestSegmentIndex].GetKeys()
	require.NoError(t, err)
	for _, key := range segmentKeys {
		missingKeys[string(key.Key)] = struct{}{}
	}

	// Delete a file in the final segment.
	filePath := ""
	if typeToDelete == "key" {
		filePath = fmt.Sprintf("%s/%s/segments/%d%s",
			directory, tableName, highestSegmentIndex, segment.KeyFileExtension)
	} else if typeToDelete == "value" {
		shardingFactor := table.(*DiskTable).getShardingFactor()
		shard := rand.Uint32Range(0, uint32(shardingFactor))
		filePath = fmt.Sprintf("%s/%s/segments/%d-%d%s",
			directory, tableName, highestSegmentIndex, shard, segment.ValuesFileExtension)
	} else {
		filePath = fmt.Sprintf("%s/%s/segments/%d%s",
			directory, tableName, highestSegmentIndex, segment.MetadataFileExtension)
	}
	exists, err := util.Exists(filePath)
	require.NoError(t, err)
	require.True(t, exists)

	err = os.Remove(filePath)
	require.NoError(t, err)

	// Restart the table.
	table, err = tableBuilder.builder(time.Now, tableName, []string{directory})
	require.NoError(t, err)

	// Manually remove the keys from the last segment from the keymap. If this happens in reality (as opposed
	// to the files being artificially deleted in this test), the keymap will not hold any value that has not
	// yet been durably flushed to disk.
	for key := range missingKeys {
		err = table.(*DiskTable).keymap.Delete([]*types.ScopedKey{{Key: []byte(key)}})
		require.NoError(t, err)
	}

	// Check the data in the table.
	for expectedKey, expectedValue := range expectedValues {
		if _, expectedToBeMissing := missingKeys[expectedKey]; expectedToBeMissing {
			_, ok, err := table.Get([]byte(expectedKey))
			require.NoError(t, err)
			require.False(t, ok)
		} else {
			value, ok, err := table.Get([]byte(expectedKey))
			require.NoError(t, err)
			require.True(t, ok)
			require.Equal(t, expectedValue, value)
		}
	}

	// Remove the missing values from the expected values map. Simplifies following checks.
	for key := range missingKeys {
		delete(expectedValues, key)
	}

	// Make additional modifications to the table to ensure that it is still working.
	for i := 0; i < iterations; i++ {

		// Write some data.
		batchSize := rand.Int32Range(1, 10)

		if batchSize == 1 {
			key := rand.PrintableVariableBytes(32, 64)
			value := rand.PrintableVariableBytes(1, 128)
			err = table.Put(key, value)
			require.NoError(t, err)
			expectedValues[string(key)] = value
		} else {
			batch := make([]*types.PutRequest, 0, batchSize)
			for j := int32(0); j < batchSize; j++ {
				key := rand.PrintableVariableBytes(32, 64)
				value := rand.PrintableVariableBytes(1, 128)
				batch = append(batch, &types.PutRequest{Key: key, Value: value})
				expectedValues[string(key)] = value
			}
			err = table.PutBatch(batch)
			require.NoError(t, err)
		}

		// Once in a while, flush the table.
		if rand.BoolWithProbability(0.1) {
			err = table.Flush()
			require.NoError(t, err)
		}

		// Once in a while, sleep for a short time. For tables that do garbage collection, the garbage
		// collection interval has been configured to be 1ms. Sleeping 5ms should be enough to give
		// the garbage collector a chance to run.
		if rand.BoolWithProbability(0.01) {
			time.Sleep(5 * time.Millisecond)
		}

		// Once in a while, scan the table and verify that all expected values are present.
		// Don't do this every time for the sake of test runtime.
		if rand.BoolWithProbability(0.01) || i == iterations-1 /* always check on the last iteration */ {
			for expectedKey, expectedValue := range expectedValues {
				value, ok, err := table.Get([]byte(expectedKey))
				require.NoError(t, err)
				require.True(t, ok)
				require.Equal(t, expectedValue, value)
			}

			// Try fetching a value that isn't in the table.
			_, ok, err := table.Get(rand.PrintableVariableBytes(32, 64))
			require.NoError(t, err)
			require.False(t, ok)
		}
	}

	ok, _ = table.(*DiskTable).errorMonitor.IsOk()
	require.True(t, ok)
	err = table.Drop()
	require.NoError(t, err)

	// ensure that the test directory is empty
	entries, err := os.ReadDir(directory)
	require.NoError(t, err)
	require.Empty(t, entries)
}

func TestLastFileMissing(t *testing.T) {
	t.Parallel()
	for _, tb := range tableBuilders {
		t.Run("key-"+tb.name, func(t *testing.T) {
			lastFileMissingTest(t, tb, "key")
		})
		t.Run("value-"+tb.name, func(t *testing.T) {
			lastFileMissingTest(t, tb, "value")
		})
		t.Run("metadata-"+tb.name, func(t *testing.T) {
			lastFileMissingTest(t, tb, "metadata")
		})
	}
}

// This test simulates the scenario where a key file is truncated.
func truncatedKeyFileTest(t *testing.T, tableBuilder *tableBuilder) {
	rand := util.NewTestRandom()

	logger := slog.Default()
	directory := t.TempDir()

	tableName := rand.String(8)
	table, err := tableBuilder.builder(time.Now, tableName, []string{directory})
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	require.Equal(t, tableName, table.Name())

	expectedValues := make(map[string][]byte)

	// Fill the table with random data.
	iterations := 100
	for i := 0; i < iterations; i++ {
		batchSize := rand.Int32Range(1, 10)
		if batchSize == 1 {
			key := rand.PrintableVariableBytes(32, 64)
			value := rand.PrintableVariableBytes(1, 128)
			err = table.Put(key, value)
			require.NoError(t, err)
			expectedValues[string(key)] = value
		} else {
			batch := make([]*types.PutRequest, 0, batchSize)
			for j := int32(0); j < batchSize; j++ {
				key := rand.PrintableVariableBytes(32, 64)
				value := rand.PrintableVariableBytes(1, 128)
				batch = append(batch, &types.PutRequest{Key: key, Value: value})
				expectedValues[string(key)] = value
			}
			err = table.PutBatch(batch)
			require.NoError(t, err)
		}
	}

	err = table.Flush()
	require.NoError(t, err)

	// If the last segment is empty, write a final value to make it non-empty. This test isn't interesting
	// if there is no data to be truncated.
	segmentPath, err := segment.NewSegmentPath(directory, "", tableName)
	require.NoError(t, err)
	_, highestSegmentIndex, _, err := segment.GatherSegmentFiles(
		logger,
		table.(*DiskTable).errorMonitor,
		[]*segment.SegmentPath{segmentPath},
		false,
		time.Now(),
		true,
		false)
	require.NoError(t, err)
	keyFileName := fmt.Sprintf("%s/%s/segments/%d%s",
		directory, tableName, highestSegmentIndex, segment.KeyFileExtension)
	keyFileBytes, err := os.ReadFile(keyFileName)
	require.NoError(t, err)

	if len(keyFileBytes) == 0 {
		key := rand.PrintableVariableBytes(32, 64)
		value := rand.PrintableVariableBytes(1, 64)
		err = table.Put(key, value)
		require.NoError(t, err)
		expectedValues[string(key)] = value
	}

	// Stop the table
	ok, _ := table.(*DiskTable).errorMonitor.IsOk()
	require.True(t, ok)
	err = table.Close()
	require.NoError(t, err)

	_, highestSegmentIndex, segments, err := segment.GatherSegmentFiles(
		logger,
		table.(*DiskTable).errorMonitor,
		[]*segment.SegmentPath{segmentPath},
		false,
		time.Now(),
		true,
		false)
	require.NoError(t, err)

	// Truncate the last key file.
	keysInLastFile, err := segments[highestSegmentIndex].GetKeys()
	require.NoError(t, err)

	keyFileName = fmt.Sprintf("%s/%s/segments/%d%s",
		directory, tableName, highestSegmentIndex, segment.KeyFileExtension)
	keyFileBytes, err = os.ReadFile(keyFileName)
	require.NoError(t, err)

	bytesRemaining := int32(0)
	if len(keyFileBytes) > 1 {
		bytesRemaining = rand.Int32Range(1, int32(len(keyFileBytes)))
	}

	keyFileBytes = keyFileBytes[:bytesRemaining]
	err = os.WriteFile(keyFileName, keyFileBytes, 0644)
	require.NoError(t, err)

	keysInLastFileAfterTruncate, err := segments[highestSegmentIndex].GetKeys()
	require.NoError(t, err)

	missingKeyCount := len(keysInLastFile) - len(keysInLastFileAfterTruncate)
	require.True(t, missingKeyCount > 0)
	remainingKeyCount := len(keysInLastFileAfterTruncate)

	missingKeys := make(map[string]struct{})
	for i := 0; i < missingKeyCount; i++ {
		missingKeys[string(keysInLastFile[remainingKeyCount+i].Key)] = struct{}{}
	}

	// Mark the last segment as non-sealed. This will be the case if the file is truncated.
	metadataFileName := fmt.Sprintf("%s/%s/segments/%d%s",
		directory, tableName, highestSegmentIndex, segment.MetadataFileExtension)
	metadataBytes, err := os.ReadFile(metadataFileName)
	require.NoError(t, err)
	// The last byte of the metadata file is the sealed flag.
	metadataBytes[len(metadataBytes)-1] = 0
	err = os.WriteFile(metadataFileName, metadataBytes, 0644)
	require.NoError(t, err)

	// Restart the table.
	table, err = tableBuilder.builder(time.Now, tableName, []string{directory})
	require.NoError(t, err)

	// Manually remove the keys from the last segment from the keymap. If this happens in reality (as opposed
	// to the files being artificially deleted in this test), the keymap will not hold any value that has not
	// yet been durably flushed to disk.
	for key := range missingKeys {
		err = table.(*DiskTable).keymap.Delete([]*types.ScopedKey{{Key: []byte(key)}})
		require.NoError(t, err)
	}

	// Check the data in the table.
	for expectedKey, expectedValue := range expectedValues {
		if _, expectedToBeMissing := missingKeys[expectedKey]; expectedToBeMissing {
			_, ok, err := table.Get([]byte(expectedKey))
			require.NoError(t, err)
			require.False(t, ok)
		} else {
			value, ok, err := table.Get([]byte(expectedKey))
			require.NoError(t, err)
			require.True(t, ok)
			require.Equal(t, expectedValue, value)
		}
	}

	// Remove the missing values from the expected values map. Simplifies following checks.
	for key := range missingKeys {
		delete(expectedValues, key)
	}

	// Make additional modifications to the table to ensure that it is still working.
	for i := 0; i < iterations; i++ {

		// Write some data.
		batchSize := rand.Int32Range(1, 10)

		if batchSize == 1 {
			key := rand.PrintableVariableBytes(32, 64)
			value := rand.PrintableVariableBytes(1, 128)
			err = table.Put(key, value)
			require.NoError(t, err)
			expectedValues[string(key)] = value
		} else {
			batch := make([]*types.PutRequest, 0, batchSize)
			for j := int32(0); j < batchSize; j++ {
				key := rand.PrintableVariableBytes(32, 64)
				value := rand.PrintableVariableBytes(1, 128)
				batch = append(batch, &types.PutRequest{Key: key, Value: value})
				expectedValues[string(key)] = value
			}
			err = table.PutBatch(batch)
			require.NoError(t, err)
		}

		// Once in a while, flush the table.
		if rand.BoolWithProbability(0.1) {
			err = table.Flush()
			require.NoError(t, err)
		}

		// Once in a while, sleep for a short time. For tables that do garbage collection, the garbage
		// collection interval has been configured to be 1ms. Sleeping 5ms should be enough to give
		// the garbage collector a chance to run.
		if rand.BoolWithProbability(0.01) {
			time.Sleep(5 * time.Millisecond)
		}

		// Once in a while, scan the table and verify that all expected values are present.
		// Don't do this every time for the sake of test runtime.
		if rand.BoolWithProbability(0.01) || i == iterations-1 /* always check on the last iteration */ {
			for expectedKey, expectedValue := range expectedValues {
				value, ok, err := table.Get([]byte(expectedKey))
				require.NoError(t, err)
				require.True(t, ok)
				require.Equal(t, expectedValue, value)
			}

			// Try fetching a value that isn't in the table.
			_, ok, err := table.Get(rand.PrintableVariableBytes(32, 64))
			require.NoError(t, err)
			require.False(t, ok)
		}
	}

	ok, _ = table.(*DiskTable).errorMonitor.IsOk()
	require.True(t, ok)
	err = table.Drop()
	require.NoError(t, err)

	// ensure that the test directory is empty
	entries, err := os.ReadDir(directory)
	require.NoError(t, err)
	require.Empty(t, entries)
}

func TestTruncatedKeyFile(t *testing.T) {
	t.Parallel()
	for _, tb := range tableBuilders {
		t.Run(tb.name, func(t *testing.T) {
			truncatedKeyFileTest(t, tb)
		})
	}
}

// This test simulates the scenario where a value file is truncated.
func truncatedValueFileTest(t *testing.T, tableBuilder *tableBuilder) {
	for _, remaining := range []uint32{0, 1, 9} {
		t.Run(fmt.Sprintf("tail-bytes-%d", remaining), func(t *testing.T) {
			truncatedValueFileAt(t, tableBuilder, remaining)
		})
	}
}

func truncatedValueFileAt(t *testing.T, tableBuilder *tableBuilder, remaining uint32) {
	rand := util.NewTestRandom()
	logger := slog.Default()
	directory := t.TempDir()
	tableName := "truncation"
	table, err := tableBuilder.builder(time.Now, tableName, []string{directory})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, table.Close()) })
	require.Equal(t, tableName, table.Name())
	require.NoError(t, table.Put([]byte("baseline"), []byte("durable"),
		&types.SecondaryKey{Key: []byte("baseline-alias"), Offset: 1, Length: 3}))
	require.NoError(t, table.Flush())
	disk := table.(*DiskTable)
	index := disk.controlLoop.threadsafeHighestSegmentIndex.Load() + 1
	shards := disk.getShardingFactor()
	require.NoError(t, table.Close())

	// Produce the next segment in two durability stages. Only the first stage's
	// returned addresses reach the real keymap; the interrupted tail is unacknowledged.
	monitor := util.NewErrorMonitor(t.Context(), logger, nil)
	seg, err := segment.CreateSegment(logger, monitor, index, disk.segmentPaths, false, shards, true)
	require.NoError(t, err)
	expectedValues := map[string][]byte{"baseline": []byte("durable"), "baseline-alias": []byte("ura")}
	for shard := uint8(0); shard < shards; shard++ {
		primary := fmt.Sprintf("keep-%d", shard)
		prefix := primary + "-prefix"
		end := primary + "-end"
		value := []byte("abcdefgh")
		_, _, err = seg.Write(&types.PutRequest{Key: []byte(primary), Value: value,
			SecondaryKeys: []*types.SecondaryKey{
				{Key: []byte(prefix), Length: 2},
				{Key: []byte(end), Offset: uint32(len(value))},
			}})
		require.NoError(t, err)
		expectedValues[primary] = value
		expectedValues[prefix] = value[:2]
		expectedValues[end] = []byte{}
	}
	wait, err := seg.Flush()
	require.NoError(t, err)
	committedKeys, err := wait()
	require.NoError(t, err)
	require.Len(t, committedKeys, int(shards)*3)
	if disk.keymapTypeFile.Type() != keymap.MemKeymapType {
		keys, _, err := keymap.NewUnsafePebbleDBKeymap(logger, disk.keymapPath, false)
		require.NoError(t, err)
		err = keys.Put(committedKeys)
		stopErr := keys.Stop()
		require.NoError(t, err)
		require.NoError(t, stopErr)
	}

	tail := &types.PutRequest{Key: []byte("torn"), Value: []byte("0123456789"),
		SecondaryKeys: []*types.SecondaryKey{
			{Key: []byte("torn-prefix"), Length: 1},
			{Key: []byte("torn-empty")},
		}}
	_, _, err = seg.Write(tail)
	require.NoError(t, err)
	tailKeys, err := seg.Seal(time.Now())
	require.NoError(t, err)
	require.Len(t, tailKeys, 3)
	require.Equal(t, types.KeyKindPrimary, tailKeys[0].Kind)
	require.Equal(t, types.KeyKindSecondary, tailKeys[1].Kind)
	require.Equal(t, types.KeyKindFinalSecondary, tailKeys[2].Kind)
	address := tailKeys[0].Address
	require.Equal(t, uint8(0), address.ShardID())
	valueFileName := filepath.Join(directory, tableName, segment.SegmentDirectory,
		fmt.Sprintf("%d-%d%s", index, address.ShardID(), segment.ValuesFileExtension))
	valueFileBytes, err := os.ReadFile(valueFileName)
	require.NoError(t, err)
	require.Equal(t, []byte("abcdefgh0123456789"), valueFileBytes, "V3 stores raw bytes without a length prefix")
	length := uint64(address.Offset()) + uint64(remaining)
	requiredLength := uint64(address.Offset()) + uint64(address.ValueSize())
	require.Greater(t, requiredLength, length)
	for _, key := range committedKeys {
		if key.Address.ShardID() == address.ShardID() {
			end := uint64(key.Address.Offset()) + uint64(key.Address.ValueSize())
			require.LessOrEqual(t, end, length, "complete group must fit at the raw V3 boundary")
		}
	}
	require.NoError(t, os.Truncate(valueFileName, int64(length)))
	metadataFileName := filepath.Join(directory, tableName, segment.SegmentDirectory,
		fmt.Sprintf("%d%s", index, segment.MetadataFileExtension))
	metadataBytes, err := os.ReadFile(metadataFileName)
	require.NoError(t, err)
	metadataBytes[len(metadataBytes)-1] = 0
	require.NoError(t, os.WriteFile(metadataFileName, metadataBytes, 0644))

	// Inspect the actual open result twice, including its rewritten key file.
	for restart := 0; restart < 2; restart++ {
		table, err = tableBuilder.builder(time.Now, tableName, []string{directory})
		require.NoError(t, err)
		require.Equal(t, uint64(len(expectedValues)), table.KeyCount())
		for key, expected := range expectedValues {
			value, exists, err := table.Get([]byte(key))
			require.NoError(t, err, key)
			require.True(t, exists, key)
			require.Equal(t, expected, value, key)
		}
		for _, key := range tailKeys {
			_, exists, err := table.Get(key.Key)
			require.NoError(t, err)
			require.False(t, exists, "the whole torn group must be absent: %s", key.Key)
			exists, err = table.Exists(key.Key)
			require.NoError(t, err)
			require.False(t, exists)
		}
		recovered, err := table.(*DiskTable).controlLoop.segments[index].GetKeys()
		require.NoError(t, err)
		require.Equal(t, committedKeys, recovered, "recovery must retain exactly the complete groups")
		if restart == 0 {
			require.NoError(t, table.Close())
		}
	}
	iterations := 100
	// Make additional modifications to the table to ensure that it is still working.
	for i := 0; i < iterations; i++ {

		// Write some data.
		batchSize := rand.Int32Range(1, 10)

		if batchSize == 1 {
			key := rand.PrintableVariableBytes(32, 64)
			value := rand.PrintableVariableBytes(1, 128)
			err = table.Put(key, value)
			require.NoError(t, err)
			expectedValues[string(key)] = value
		} else {
			batch := make([]*types.PutRequest, 0, batchSize)
			for j := int32(0); j < batchSize; j++ {
				key := rand.PrintableVariableBytes(32, 64)
				value := rand.PrintableVariableBytes(1, 128)
				batch = append(batch, &types.PutRequest{Key: key, Value: value})
				expectedValues[string(key)] = value
			}
			err = table.PutBatch(batch)
			require.NoError(t, err)
		}

		// Once in a while, flush the table.
		if rand.BoolWithProbability(0.1) {
			err = table.Flush()
			require.NoError(t, err)
		}

		// Once in a while, sleep for a short time. For tables that do garbage collection, the garbage
		// collection interval has been configured to be 1ms. Sleeping 5ms should be enough to give
		// the garbage collector a chance to run.
		if rand.BoolWithProbability(0.01) {
			time.Sleep(5 * time.Millisecond)
		}

		// Once in a while, scan the table and verify that all expected values are present.
		// Don't do this every time for the sake of test runtime.
		if rand.BoolWithProbability(0.01) || i == iterations-1 /* always check on the last iteration */ {
			for expectedKey, expectedValue := range expectedValues {
				value, ok, err := table.Get([]byte(expectedKey))
				require.NoError(t, err)
				require.True(t, ok)
				require.Equal(t, expectedValue, value)
			}

			// Try fetching a value that isn't in the table.
			_, ok, err := table.Get(rand.PrintableVariableBytes(32, 64))
			require.NoError(t, err)
			require.False(t, ok)
		}
	}

	ok, _ := table.(*DiskTable).errorMonitor.IsOk()
	require.True(t, ok)
	err = table.Drop()
	require.NoError(t, err)

	// ensure that the test directory is empty
	entries, err := os.ReadDir(directory)
	require.NoError(t, err)
	require.Empty(t, entries)
}

func TestTruncatedValueFile(t *testing.T) {
	t.Parallel()
	for _, tb := range tableBuilders {
		t.Run(tb.name, func(t *testing.T) {
			truncatedValueFileTest(t, tb)
		})
	}
}

// This test simulates the scenario where keys have not been flushed to the key store. The important thing
// is to ensure that garbage collection doesn't explode when it encounters keys that are not in the key store.
func unflushedKeysTest(t *testing.T, tableBuilder *tableBuilder) {
	rand := util.NewTestRandom()

	logger := slog.Default()
	directory := t.TempDir()

	tableName := rand.String(8)
	table, err := tableBuilder.builder(time.Now, tableName, []string{directory})
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	require.Equal(t, tableName, table.Name())

	expectedValues := make(map[string][]byte)

	// Fill the table with random data.
	iterations := 100
	for i := 0; i < iterations; i++ {
		batchSize := rand.Int32Range(1, 10)
		if batchSize == 1 {
			key := rand.PrintableVariableBytes(32, 64)
			value := rand.PrintableVariableBytes(1, 128)
			err = table.Put(key, value)
			require.NoError(t, err)
			expectedValues[string(key)] = value
		} else {
			batch := make([]*types.PutRequest, 0, batchSize)
			for j := int32(0); j < batchSize; j++ {
				key := rand.PrintableVariableBytes(32, 64)
				value := rand.PrintableVariableBytes(1, 128)
				batch = append(batch, &types.PutRequest{Key: key, Value: value})
				expectedValues[string(key)] = value
			}
			err = table.PutBatch(batch)
			require.NoError(t, err)
		}
	}

	err = table.Flush()
	require.NoError(t, err)

	// If the last segment is empty, write a final value to make it non-empty. This test isn't interesting
	// if there is no data left unflushed.
	segmentPath, err := segment.NewSegmentPath(directory, "", tableName)
	require.NoError(t, err)
	_, highestSegmentIndex, _, err := segment.GatherSegmentFiles(
		logger,
		table.(*DiskTable).errorMonitor,
		[]*segment.SegmentPath{segmentPath},
		false,
		time.Now(),
		true,
		false)
	require.NoError(t, err)
	keyFileName := fmt.Sprintf("%s/%s/segments/%d%s",
		directory, tableName, highestSegmentIndex, segment.KeyFileExtension)
	keyFileBytes, err := os.ReadFile(keyFileName)
	require.NoError(t, err)
	if len(keyFileBytes) == 0 {
		key := rand.PrintableVariableBytes(32, 64)
		value := rand.PrintableVariableBytes(1, 64)
		err = table.Put(key, value)
		require.NoError(t, err)
		expectedValues[string(key)] = value
	}

	// Stop the table
	ok, _ := table.(*DiskTable).errorMonitor.IsOk()
	require.True(t, ok)
	err = table.Close()
	require.NoError(t, err)

	_, highestSegmentIndex, segments, err := segment.GatherSegmentFiles(
		logger,
		table.(*DiskTable).errorMonitor,
		[]*segment.SegmentPath{segmentPath},
		false,
		time.Now(),
		true,
		false)
	require.NoError(t, err)

	// Identify keys in the last file. These will be removed from the keymap to simulate keys that have not
	// been flushed to the key store.
	keysInLastFile, err := segments[highestSegmentIndex].GetKeys()
	require.NoError(t, err)

	missingKeys := make(map[string]struct{})
	for _, key := range keysInLastFile {
		missingKeys[string(key.Key)] = struct{}{}
	}

	// Mark the last segment as non-sealed. This will be the case if the file is truncated.
	metadataFileName := fmt.Sprintf("%s/%s/segments/%d%s",
		directory, tableName, highestSegmentIndex, segment.MetadataFileExtension)
	metadataBytes, err := os.ReadFile(metadataFileName)
	require.NoError(t, err)
	// The last byte of the metadata file is the sealed flag.
	metadataBytes[len(metadataBytes)-1] = 0
	err = os.WriteFile(metadataFileName, metadataBytes, 0644)
	require.NoError(t, err)

	// Restart the table.
	table, err = tableBuilder.builder(time.Now, tableName, []string{directory})
	require.NoError(t, err)

	// Manually remove the keys from the last segment from the keymap. If this happens in reality (as opposed
	// to the files being artificially deleted in this test), the keymap will not hold any value that has not
	// yet been durably flushed to disk.
	for key := range missingKeys {
		err = table.(*DiskTable).keymap.Delete([]*types.ScopedKey{{Key: []byte(key)}})
		require.NoError(t, err)
	}

	// Check the data in the table.
	for expectedKey, expectedValue := range expectedValues {
		if _, expectedToBeMissing := missingKeys[expectedKey]; expectedToBeMissing {
			_, ok, err := table.Get([]byte(expectedKey))
			require.NoError(t, err)
			require.False(t, ok)
		} else {
			value, ok, err := table.Get([]byte(expectedKey))
			require.NoError(t, err)
			require.True(t, ok)
			require.Equal(t, expectedValue, value)
		}
	}

	// Remove the missing values from the expected values map. Simplifies following checks.
	for key := range missingKeys {
		delete(expectedValues, key)
	}

	// Make additional modifications to the table to ensure that it is still working.
	for i := 0; i < iterations; i++ {

		// Write some data.
		batchSize := rand.Int32Range(1, 10)

		if batchSize == 1 {
			key := rand.PrintableVariableBytes(32, 64)
			value := rand.PrintableVariableBytes(1, 128)
			err = table.Put(key, value)
			require.NoError(t, err)
			expectedValues[string(key)] = value
		} else {
			batch := make([]*types.PutRequest, 0, batchSize)
			for j := int32(0); j < batchSize; j++ {
				key := rand.PrintableVariableBytes(32, 64)
				value := rand.PrintableVariableBytes(1, 128)
				batch = append(batch, &types.PutRequest{Key: key, Value: value})
				expectedValues[string(key)] = value
			}
			err = table.PutBatch(batch)
			require.NoError(t, err)
		}

		// Once in a while, flush the table.
		if rand.BoolWithProbability(0.1) {
			err = table.Flush()
			require.NoError(t, err)
		}

		// Once in a while, sleep for a short time. For tables that do garbage collection, the garbage
		// collection interval has been configured to be 1ms. Sleeping 5ms should be enough to give
		// the garbage collector a chance to run.
		if rand.BoolWithProbability(0.01) {
			time.Sleep(5 * time.Millisecond)
		}

		// Once in a while, scan the table and verify that all expected values are present.
		// Don't do this every time for the sake of test runtime.
		if rand.BoolWithProbability(0.01) || i == iterations-1 /* always check on the last iteration */ {
			for expectedKey, expectedValue := range expectedValues {
				value, ok, err := table.Get([]byte(expectedKey))
				require.NoError(t, err)
				require.True(t, ok)
				require.Equal(t, expectedValue, value)
			}

			// Try fetching a value that isn't in the table.
			_, ok, err := table.Get(rand.PrintableVariableBytes(32, 64))
			require.NoError(t, err)
			require.False(t, ok)
		}
	}

	// Enable a TTL for the table. The goal is to force the keys that were removed from the keymap artificially to
	// become eligible for garbage collection.
	err = table.SetTTL(1 * time.Millisecond)
	require.NoError(t, err)

	// Sleep for a short time to allow the TTL to expire, and to give the garbage collector a chance to
	// do bad things if it is going to. Nothing bad should happen if the GC is implemented correctly.
	time.Sleep(50 * time.Millisecond)

	ok, _ = table.(*DiskTable).errorMonitor.IsOk()
	require.True(t, ok)
	err = table.Drop()
	require.NoError(t, err)

	// ensure that the test directory is empty
	entries, err := os.ReadDir(directory)
	require.NoError(t, err)
	require.Empty(t, entries)
}

func TestUnflushedKeys(t *testing.T) {
	t.Parallel()
	for _, tb := range tableBuilders {
		t.Run(tb.name, func(t *testing.T) {
			unflushedKeysTest(t, tb)
		})
	}
}

func settingsResetOnRestartTest(t *testing.T, tableBuilder *tableBuilder) {
	rand := util.NewTestRandom()

	directory := t.TempDir()

	tableName := rand.String(8)
	table, err := tableBuilder.builder(time.Now, tableName, []string{directory})
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}
	require.Equal(t, tableName, table.Name())

	// Capture the initial settings, which come from the table config supplied at creation time.
	initialTTL := (table.(*DiskTable)).getTTL()
	initialShardingFactor := (table.(*DiskTable)).getShardingFactor()

	// Change the settings at runtime.
	ttl := time.Hour + time.Duration(rand.Int63n(1000))*time.Millisecond
	err = table.SetTTL(ttl)
	require.NoError(t, err)
	shardingFactor := uint8(rand.Uint32Range(1, 100))
	err = table.SetShardingFactor(shardingFactor)
	require.NoError(t, err)

	// Stop the table
	ok, _ := table.(*DiskTable).errorMonitor.IsOk()
	require.True(t, ok)
	err = table.Close()
	require.NoError(t, err)

	// Restart the table.
	table, err = tableBuilder.builder(time.Now, tableName, []string{directory})
	require.NoError(t, err)

	// Settings are not persisted to disk, so after a restart they revert to the values supplied at creation
	// time rather than the values set at runtime.
	require.Equal(t, initialTTL, (table.(*DiskTable)).getTTL())
	require.Equal(t, initialShardingFactor, (table.(*DiskTable)).getShardingFactor())

	err = table.Drop()
	require.NoError(t, err)
}

func TestSettingsResetOnRestart(t *testing.T) {
	t.Parallel()
	for _, tb := range tableBuilders {
		t.Run(tb.name, func(t *testing.T) {
			settingsResetOnRestartTest(t, tb)
		})
	}
}

func restartWithMultipleStorageDirectoriesTest(t *testing.T, tableBuilder *tableBuilder) {
	rand := util.NewTestRandom()

	directoryCount := rand.Uint32Range(5, 10)

	directory := t.TempDir()
	directories := make([]string, 0, directoryCount)
	for i := uint32(0); i < directoryCount; i++ {
		directories = append(directories, path.Join(directory, fmt.Sprintf("dir%d", i)))
	}

	tableName := rand.String(8)
	table, err := tableBuilder.builder(time.Now, tableName, directories)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	require.Equal(t, tableName, table.Name())

	expectedValues := make(map[string][]byte)

	iterations := 1000
	restartIteration := iterations/2 + int(rand.Int64Range(-10, 10))

	for i := 0; i < iterations; i++ {

		// Somewhere in the middle of the test, restart the table.
		if i == restartIteration {
			ok, _ := table.(*DiskTable).errorMonitor.IsOk()
			require.True(t, ok)
			err = table.Close()
			require.NoError(t, err)

			// Shuffle around the segment files. This should not cause problems.
			files := make([]string, 0)
			for _, dir := range directories {
				segmentDir := path.Join(dir, tableName, "segments")

				entries, err := os.ReadDir(segmentDir)
				require.NoError(t, err)
				for _, entry := range entries {
					files = append(files, path.Join(dir, tableName, "segments", entry.Name()))
				}
			}
			for _, file := range files {
				destination := path.Join(
					directories[rand.Uint32Range(0, uint32(len(directories)))],
					tableName,
					"segments",
					path.Base(file))
				err = os.Rename(file, destination)
				require.NoError(t, err)
			}

			table, err = tableBuilder.builder(time.Now, tableName, directories)
			require.NoError(t, err)

			// Change the sharding factor. This should not cause problems.
			shardingFactor := uint8(rand.Uint32Range(1, 10))
			err = table.SetShardingFactor(shardingFactor)
			require.NoError(t, err)

			// Do a full scan of the table to verify that all expected values are still present.
			for expectedKey, expectedValue := range expectedValues {
				value, ok, err := table.Get([]byte(expectedKey))
				require.NoError(t, err)
				require.True(t, ok)
				require.Equal(t, expectedValue, value)
			}

			// Try fetching a value that isn't in the table.
			_, ok, err := table.Get(rand.PrintableVariableBytes(32, 64))
			require.NoError(t, err)
			require.False(t, ok)
		}

		// Write some data.
		batchSize := rand.Int32Range(1, 10)

		if batchSize == 1 {
			key := rand.PrintableVariableBytes(32, 64)
			value := rand.PrintableVariableBytes(1, 128)
			err = table.Put(key, value)
			require.NoError(t, err)
			expectedValues[string(key)] = value
		} else {
			batch := make([]*types.PutRequest, 0, batchSize)
			for j := int32(0); j < batchSize; j++ {
				key := rand.PrintableVariableBytes(32, 64)
				value := rand.PrintableVariableBytes(1, 128)
				batch = append(batch, &types.PutRequest{Key: key, Value: value})
				expectedValues[string(key)] = value
			}
			err = table.PutBatch(batch)
			require.NoError(t, err)
		}

		// Once in a while, flush the table.
		if rand.BoolWithProbability(0.1) {
			err = table.Flush()
			require.NoError(t, err)
		}

		// Once in a while, sleep for a short time. For tables that do garbage collection, the garbage
		// collection interval has been configured to be 1ms. Sleeping 5ms should be enough to give
		// the garbage collector a chance to run.
		if rand.BoolWithProbability(0.01) {
			time.Sleep(5 * time.Millisecond)
		}

		// Once in a while, scan the table and verify that all expected values are present.
		// Don't do this every time for the sake of test runtime.
		if rand.BoolWithProbability(0.01) || i == iterations-1 /* always check on the last iteration */ {

			for expectedKey, expectedValue := range expectedValues {
				value, ok, err := table.Get([]byte(expectedKey))
				require.NoError(t, err)
				require.True(t, ok)
				require.Equal(t, expectedValue, value)
			}

			// Try fetching a value that isn't in the table.
			_, ok, err := table.Get(rand.PrintableVariableBytes(32, 64))
			require.NoError(t, err)
			require.False(t, ok)
		}
	}

	ok, _ := table.(*DiskTable).errorMonitor.IsOk()
	require.True(t, ok)
	err = table.Drop()
	require.NoError(t, err)

	// ensure that the test directories are empty
	for _, dir := range directories {
		entries, err := os.ReadDir(dir)
		require.NoError(t, err)
		require.Empty(t, entries)
	}
}

func TestRestartWithMultipleStorageDirectories(t *testing.T) {
	t.Parallel()
	for _, tb := range tableBuilders {
		t.Run(tb.name, func(t *testing.T) {
			restartWithMultipleStorageDirectoriesTest(t, tb)
		})
	}
}

// changingShardingFactorTest checks the number of shards in a particular segment and compares it to the expected
// number of shards in the segment.
func checkShardsInSegment(
	t *testing.T,
	roots []string,
	segmentIndex uint32,
	expectedShardCount uint8) {

	// For each shard, there should be exactly one value file in the format <segmentIndex>-<shardIndex>.value
	expectedValueFiles := make(map[string]struct{})
	for i := uint8(0); i < expectedShardCount; i++ {
		expectedValueFiles[fmt.Sprintf("%d-%d.values", segmentIndex, i)] = struct{}{}
	}

	discoveredShardFiles := make(map[string]struct{})
	for _, root := range roots {
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			fileName := filepath.Base(path)
			if _, ok := expectedValueFiles[fileName]; ok {
				discoveredShardFiles[fileName] = struct{}{}
			}

			return nil
		})
		require.NoError(t, err)
	}

	require.Equal(t, expectedValueFiles, discoveredShardFiles)
}

// changingShardingFactorTest checks the number of shards in the latest segment.
func checkShardsInSegments(
	t *testing.T,
	roots []string,
	expectedShardCounts map[uint32]uint8) {

	for segmentIndex, expectedShardCount := range expectedShardCounts {
		checkShardsInSegment(t, roots, segmentIndex, expectedShardCount)
	}
}

// getLatestSegmentIndex returns the index of the latest segment in the table.
func getLatestSegmentIndex(table litt.Table) uint32 {
	return (table.(*DiskTable)).controlLoop.threadsafeHighestSegmentIndex.Load()
}

func changingShardingFactorTest(t *testing.T, tableBuilder *tableBuilder) {
	rand := util.NewTestRandom()

	directory := t.TempDir()
	rootCount := rand.Uint32Range(1, 5)
	roots := make([]string, 0, rootCount)
	for i := uint32(0); i < rootCount; i++ {
		roots = append(roots, path.Join(directory, fmt.Sprintf("root%d", i)))
	}

	tableName := rand.String(8)
	table, err := tableBuilder.builder(time.Now, tableName, roots)
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	require.Equal(t, tableName, table.Name())

	// Contains the expected number of shards in various segments. We won't check all segments, just the segments
	// immediately before and immediately after a sharding factor change.
	expectedShardCounts := make(map[uint32]uint8)

	// Before data is written, change the sharding factor to a random value.
	expectedShardCounts[getLatestSegmentIndex(table)] = table.(*DiskTable).getShardingFactor()
	shardingFactor := uint8(rand.Uint32Range(2, 10))
	err = table.SetShardingFactor(shardingFactor)
	require.NoError(t, err)
	err = table.Flush()
	require.NoError(t, err)
	expectedShardCounts[getLatestSegmentIndex(table)] = shardingFactor

	expectedValues := make(map[string][]byte)

	iterations := 1000
	restartIteration := iterations/2 + int(rand.Int64Range(-10, 10))

	for i := 0; i < iterations; i++ {

		// Somewhere in the middle of the test, restart the table.
		if i == restartIteration {
			expectedShardCounts[getLatestSegmentIndex(table)] = shardingFactor

			ok, _ := table.(*DiskTable).errorMonitor.IsOk()
			require.True(t, ok)
			err = table.Close()
			require.NoError(t, err)

			table, err = tableBuilder.builder(time.Now, tableName, roots)
			require.NoError(t, err)

			// Sharding factor is not persisted across restarts (see TestSettingsResetOnRestart). The
			// restarted table's new segment uses the builder's configured factor, not the runtime value
			// previously set via SetShardingFactor, so re-sync our expectation to the table's actual factor.
			shardingFactor = table.(*DiskTable).getShardingFactor()
			expectedShardCounts[getLatestSegmentIndex(table)] = shardingFactor

			// Do a full scan of the table to verify that all expected values are still present.
			for expectedKey, expectedValue := range expectedValues {
				value, ok, err := table.Get([]byte(expectedKey))
				require.NoError(t, err)
				require.True(t, ok, "key %s not found", expectedKey)
				require.Equal(t, expectedValue, value)
			}

			// Try fetching a value that isn't in the table.
			_, ok, err := table.Get(rand.PrintableVariableBytes(32, 64))
			require.NoError(t, err)
			require.False(t, ok)
		}

		// Write some data.
		batchSize := rand.Int32Range(1, 10)

		if batchSize == 1 {
			key := rand.PrintableVariableBytes(32, 64)
			value := rand.PrintableVariableBytes(1, 128)
			err = table.Put(key, value)
			require.NoError(t, err)
			expectedValues[string(key)] = value
		} else {
			batch := make([]*types.PutRequest, 0, batchSize)
			for j := int32(0); j < batchSize; j++ {
				key := rand.PrintableVariableBytes(32, 64)
				value := rand.PrintableVariableBytes(1, 128)
				batch = append(batch, &types.PutRequest{Key: key, Value: value})
				expectedValues[string(key)] = value
			}
			err = table.PutBatch(batch)
			require.NoError(t, err)
		}

		// Once in a while, change the sharding factor to a random value.
		if rand.BoolWithProbability(0.01) {
			expectedShardCounts[getLatestSegmentIndex(table)] = shardingFactor
			shardingFactor = uint8(rand.Uint32Range(1, 10))
			err = table.SetShardingFactor(shardingFactor)
			require.NoError(t, err)
			err = table.Flush()
			require.NoError(t, err)
			expectedShardCounts[getLatestSegmentIndex(table)] = shardingFactor
		}

		// Once in a while, flush the table.
		if rand.BoolWithProbability(0.1) {
			err = table.Flush()
			require.NoError(t, err)
		}

		// Once in a while, sleep for a short time. For tables that do garbage collection, the garbage
		// collection interval has been configured to be 1ms. Sleeping 5ms should be enough to give
		// the garbage collector a chance to run.
		if rand.BoolWithProbability(0.01) {
			time.Sleep(5 * time.Millisecond)
		}

		// Once in a while, scan the table and verify that all expected values are present.
		// Don't do this every time for the sake of test runtime.
		if rand.BoolWithProbability(0.01) || i == iterations-1 /* always check on the last iteration */ {

			for expectedKey, expectedValue := range expectedValues {
				value, ok, err := table.Get([]byte(expectedKey))
				require.NoError(t, err)
				require.True(t, ok)
				require.Equal(t, expectedValue, value)
			}

			// Try fetching a value that isn't in the table.
			_, ok, err := table.Get(rand.PrintableVariableBytes(32, 64))
			require.NoError(t, err)
			require.False(t, ok)
		}
	}

	ok, _ := table.(*DiskTable).errorMonitor.IsOk()
	require.True(t, ok)

	err = table.Close()
	require.NoError(t, err)

	checkShardsInSegments(t, roots, expectedShardCounts)
}

func TestChangingShardingFactor(t *testing.T) {
	t.Parallel()
	for _, tb := range tableBuilders {
		t.Run(tb.name, func(t *testing.T) {
			changingShardingFactorTest(t, tb)
		})
	}
}

// verifies that the size reported by the table matches the actual size of the table on disk
func tableSizeTest(t *testing.T, tableBuilder *tableBuilder) {
	rand := util.NewTestRandom()

	directory := t.TempDir()

	startTime := rand.Time()

	var fakeTime atomic.Pointer[time.Time]
	fakeTime.Store(&startTime)

	clock := func() time.Time {
		return *fakeTime.Load()
	}

	tableName := rand.String(8)
	table, err := tableBuilder.builder(clock, tableName, []string{directory})
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	ttlSeconds := rand.Int32Range(20, 30)
	ttl := time.Duration(ttlSeconds) * time.Second
	err = table.SetTTL(ttl)
	require.NoError(t, err)

	require.Equal(t, tableName, table.Name())

	expectedValues := make(map[string][]byte)
	creationTimes := make(map[string]time.Time)
	expiredValues := make(map[string][]byte)

	iterations := 1000
	for i := 0; i < iterations; i++ {

		// Advance the clock.
		now := *fakeTime.Load()
		secondsToAdvance := rand.Float64Range(0.0, 1.0)
		newTime := now.Add(time.Duration(secondsToAdvance * float64(time.Second)))
		fakeTime.Store(&newTime)

		// Write some data.
		batchSize := rand.Int32Range(1, 10)

		if batchSize == 1 {
			key := rand.PrintableVariableBytes(32, 64)
			value := rand.PrintableVariableBytes(1, 128)
			err = table.Put(key, value)
			require.NoError(t, err)
			expectedValues[string(key)] = value
			creationTimes[string(key)] = newTime
		} else {
			batch := make([]*types.PutRequest, 0, batchSize)
			for j := int32(0); j < batchSize; j++ {
				key := rand.PrintableVariableBytes(32, 64)
				value := rand.PrintableVariableBytes(1, 128)
				batch = append(batch, &types.PutRequest{Key: key, Value: value})
				expectedValues[string(key)] = value
				creationTimes[string(key)] = newTime
			}
			err = table.PutBatch(batch)
			require.NoError(t, err)
		}

		// Once in a while, flush the table.
		if rand.BoolWithProbability(0.1) {
			err = table.Flush()
			require.NoError(t, err)
		}

		// Once in a while, change the TTL. To avoid introducing test flakiness, only decrease the TTL
		// (increasing the TTL risks causing the expected deletions as tracked by this test to get out
		// of sync with what the table is doing)
		if rand.BoolWithProbability(0.01) {
			ttlSeconds -= 1
			ttl = time.Duration(ttlSeconds) * time.Second
			err = table.SetTTL(ttl)
			require.NoError(t, err)
		}

		// Once in a while, pause for a brief moment to give the garbage collector a chance to do work in the
		// background. This is not required for the test to pass.
		if rand.BoolWithProbability(0.01) {
			time.Sleep(5 * time.Millisecond)
		}

		// Once in a while, scan the table and verify that all expected values are present.
		// Don't do this every time for the sake of test runtime.
		if rand.BoolWithProbability(0.01) || i == iterations-1 /* always check on the last iteration */ {

			// Force garbage collection to run in order to remove expired values from counts.
			err = table.Flush()
			require.NoError(t, err)
			err = (table).(*DiskTable).RunGC()
			require.NoError(t, err)

			// Remove expired values from the expected values.
			newlyExpiredKeys := make([]string, 0)
			for key, creationTime := range creationTimes {
				age := newTime.Sub(creationTime)
				if age > ttl {
					newlyExpiredKeys = append(newlyExpiredKeys, key)
				}
			}
			for _, key := range newlyExpiredKeys {
				expiredValues[key] = expectedValues[key]
				delete(expectedValues, key)
				delete(creationTimes, key)
			}

			// Check the keys that are expected to still be in the table
			for expectedKey, expectedValue := range expectedValues {
				value, ok, err := table.Get([]byte(expectedKey))
				require.NoError(t, err)
				require.True(t, ok, "key %s not found in table", expectedKey)
				require.Equal(t, expectedValue, value)
			}

			// Try fetching a value that isn't in the table.
			_, ok, err := table.Get(rand.PrintableVariableBytes(32, 64))
			require.NoError(t, err)
			require.False(t, ok)

			for key, expectedValue := range expiredValues {
				value, ok, err := table.Get([]byte(key))
				require.NoError(t, err)
				if !ok {
					// value is not present in the table
					continue
				}

				// If the value has not yet been deleted, it should at least return the expected value.
				require.Equal(t, expectedValue, value, "unexpected value for key %s", key)

			}
		}
	}

	err = table.Flush()
	require.NoError(t, err)
	err = table.RunGC()
	require.NoError(t, err)

	// disable garbage collection
	err = table.SetTTL(0)
	require.NoError(t, err)
	err = table.Flush()
	require.NoError(t, err)

	// Write some data that won't expire, just to be sure that the table is not empty.
	for i := 0; i < 10; i++ {
		key := rand.PrintableVariableBytes(32, 64)
		value := rand.PrintableVariableBytes(1, 128)
		err = table.Put(key, value)
		require.NoError(t, err)
		expectedValues[string(key)] = value
	}

	err = table.Flush()
	require.NoError(t, err)

	reportedSize := table.Size()
	reportedKeyCount := table.KeyCount()

	// The exact key count is hard to predict for the sake of this unit test, since GC is "lazy" and may not
	// immediately remove all values that are legal to be removed. But at the very least, all unexpired
	// values should be present, and the key count should not exceed the number of total inserted values.
	require.GreaterOrEqual(t, reportedKeyCount, uint64(len(expectedValues)))
	require.LessOrEqual(t, reportedKeyCount, uint64(len(expectedValues)+len(expiredValues)))

	err = table.Close()
	require.NoError(t, err)

	// Walk the "directory" file tree and calculate the actual size of the table.
	// There is some asynchrony in file deletion, so we retry a reasonable number of times.
	util.AssertEventuallyTrue(t, func() bool {
		actualSize := uint64(0)

		err = filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				// files may be deleted in the middle of the walk
				return nil
			}
			if info.IsDir() {
				// directory sizes are not factored into the table size
				return nil
			}
			if strings.Contains(path, "keymap") {
				// table size does not currently include the keymap size
				return nil
			}
			actualSize += uint64(info.Size())
			return nil
		})
		require.NoError(t, err)
		return actualSize == reportedSize
	}, time.Second)

	// Restart the table. The size should be accurately reported.
	table, err = tableBuilder.builder(clock, tableName, []string{directory})
	require.NoError(t, err)

	newReportedSize := table.Size()
	newReportedKeyCount := table.KeyCount()

	// New size should be greater than the old size, since GC is disabled and
	// we will have started a new segment upon restart.
	require.LessOrEqual(t, reportedSize, newReportedSize)

	// The number of keys should be the same as before.
	require.Equal(t, reportedKeyCount, newReportedKeyCount)

	err = table.Close()
	require.NoError(t, err)

	// Walk the "directory" file tree and calculate the actual size of the table.
	// There is some asynchrony in file deletion, so we retry a reasonable number of times.
	util.AssertEventuallyTrue(t, func() bool {
		actualSize := uint64(0)
		err = filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				// files may be deleted in the middle of the walk
				return nil
			}
			if info.IsDir() {
				// directory sizes are not factored into the table size
				return nil
			}
			if strings.Contains(path, "keymap") {
				// table size does not currently include the keymap size
				return nil
			}

			actualSize += uint64(info.Size())
			return nil
		})
		require.NoError(t, err)

		return actualSize == newReportedSize
	}, time.Second)
}

func TestTableSize(t *testing.T) {
	t.Parallel()
	for _, tb := range tableBuilders {
		t.Run(tb.name, func(t *testing.T) {
			tableSizeTest(t, tb)
		})
	}
}
