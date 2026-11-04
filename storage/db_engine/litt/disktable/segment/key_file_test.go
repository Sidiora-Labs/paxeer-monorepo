package segment

import (
	"fmt"
	"log/slog"
	"os"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadWriteKeys(t *testing.T) {
	t.Parallel()
	rand := util.NewTestRandom()
	logger := slog.Default()
	directory := t.TempDir()

	index := rand.Uint32()

	// Cycle through all four KeyKind values so the on-disk record layout is exercised end-to-end.
	// Index 0 has the implicit zero-value (KeyKindStandalone), confirming that a ScopedKey with
	// no explicit Kind round-trips as a Standalone primary.
	kinds := []types.KeyKind{
		types.KeyKindStandalone,
		types.KeyKindPrimary,
		types.KeyKindSecondary,
		types.KeyKindFinalSecondary,
	}

	keyCount := rand.Int32Range(100, 200)
	keys := make([]*types.ScopedKey, keyCount)
	for i := 0; i < int(keyCount); i++ {
		key := rand.VariableBytes(1, 100)
		address := types.NewAddress(
			rand.Uint32(),
			rand.Uint32(),
			uint8(rand.Uint32Range(0, 256)),
			rand.Uint32(),
		)
		// First record left implicit (Kind defaults to zero/Standalone); the rest rotate.
		var kind types.KeyKind
		if i != 0 {
			kind = kinds[i%len(kinds)]
		}
		keys[i] = &types.ScopedKey{Key: key, Address: address, Kind: kind}
	}

	segmentPath, err := NewSegmentPath(directory, "", "table")
	require.NoError(t, err)
	err = segmentPath.MakeDirectories(false)
	require.NoError(t, err)
	file, err := createKeyFile(logger, index, segmentPath, false, false)
	require.NoError(t, err)

	for _, key := range keys {
		err := file.write(key)
		require.NoError(t, err)
	}

	// Reading the file prior to sealing it is forbidden.
	_, err = file.readKeys()
	require.Error(t, err)

	err = file.seal()
	require.NoError(t, err)

	// Verify that file size is correctly reported.
	reportedSize := file.Size()
	stat, err := os.Stat(file.path())
	require.NoError(t, err)
	actualSize := uint64(stat.Size())
	require.Equal(t, actualSize, reportedSize)

	// Reading the file after sealing it is allowed.
	readKeys, err := file.readKeys()
	require.NoError(t, err)

	for i, key := range keys {
		assert.Equal(t, key, readKeys[i])
	}

	// Create a new in-memory instance from the on-disk file and verify that it behaves the same.
	file2, err := loadKeyFile(logger, index, []*SegmentPath{segmentPath}, LatestSegmentVersion)
	require.NoError(t, err)
	require.Equal(t, file.Size(), file2.Size())

	readKeys, err = file2.readKeys()
	require.NoError(t, err)
	for i, key := range keys {
		assert.Equal(t, key, readKeys[i])
	}

	// delete the file
	filePath := file.path()
	_, err = os.Stat(filePath)
	require.NoError(t, err)

	err = file.delete()
	require.NoError(t, err)

	_, err = os.Stat(filePath)
	require.True(t, os.IsNotExist(err))
}

func TestReadingTruncatedKeyFile(t *testing.T) {
	t.Parallel()
	rand := util.NewTestRandom()
	logger := slog.Default()
	directory := t.TempDir()

	index := rand.Uint32()

	keyCount := rand.Int32Range(100, 200)
	keys := make([]*types.ScopedKey, keyCount)
	for i := 0; i < int(keyCount); i++ {
		key := rand.VariableBytes(1, 100)
		address := types.NewAddress(
			rand.Uint32(),
			rand.Uint32(),
			uint8(rand.Uint32Range(0, 256)),
			rand.Uint32(),
		)
		keys[i] = &types.ScopedKey{Key: key, Address: address, Kind: types.KeyKind(i % 4)}
	}

	segmentPath, err := NewSegmentPath(directory, "", "table")
	require.NoError(t, err)
	err = segmentPath.MakeDirectories(false)
	require.NoError(t, err)
	file, err := createKeyFile(logger, index, segmentPath, false, false)
	require.NoError(t, err)

	for _, key := range keys {
		err := file.write(key)
		require.NoError(t, err)
	}

	err = file.seal()
	require.NoError(t, err)

	filePath := file.path()
	originalBytes, err := os.ReadFile(filePath)
	require.NoError(t, err)
	lastRecordSize := KeyRecordHeaderSize + len(keys[keyCount-1].Key) + types.AddressSerializedSize
	lastRecordStart := len(originalBytes) - lastRecordSize

	for kept := 0; kept < lastRecordSize; kept++ {
		t.Run(fmt.Sprintf("last_record_bytes_%d", kept), func(t *testing.T) {
			require.NoError(t, os.WriteFile(filePath, originalBytes[:lastRecordStart+kept], 0644))
			readKeys, err := file.readKeys()
			require.NoError(t, err)
			require.Len(t, readKeys, int(keyCount-1))
			for i, key := range keys[:keyCount-1] {
				assert.Equal(t, key.Key, readKeys[i].Key)
				assert.Equal(t, key.Kind, readKeys[i].Kind)
				assert.Equal(t, key.Address, readKeys[i].Address)
			}
		})
	}

	// delete the file
	_, err = os.Stat(filePath)
	require.NoError(t, err)

	err = file.delete()
	require.NoError(t, err)

	_, err = os.Stat(filePath)
	require.True(t, os.IsNotExist(err))
}

func TestSwappingKeyFile(t *testing.T) {
	t.Parallel()
	rand := util.NewTestRandom()
	logger := slog.Default()
	directory := t.TempDir()

	index := rand.Uint32()

	keyCount := rand.Int32Range(100, 200)
	keys := make([]*types.ScopedKey, keyCount)
	for i := 0; i < int(keyCount); i++ {
		key := rand.VariableBytes(1, 100)
		address := types.NewAddress(
			rand.Uint32(),
			rand.Uint32(),
			uint8(rand.Uint32Range(0, 256)),
			rand.Uint32(),
		)
		keys[i] = &types.ScopedKey{Key: key, Address: address}
	}

	segmentPath, err := NewSegmentPath(directory, "", "table")
	require.NoError(t, err)
	err = segmentPath.MakeDirectories(false)
	require.NoError(t, err)
	file, err := createKeyFile(logger, index, segmentPath, false, false)
	require.NoError(t, err)

	for _, key := range keys {
		err := file.write(key)
		require.NoError(t, err)
	}

	// Reading the file prior to sealing it is forbidden.
	_, err = file.readKeys()
	require.Error(t, err)

	err = file.seal()
	require.NoError(t, err)

	// Verify that file size is correctly reported.
	reportedSize := file.Size()
	stat, err := os.Stat(file.path())
	require.NoError(t, err)
	actualSize := uint64(stat.Size())
	require.Equal(t, actualSize, reportedSize)

	// Reading the file after sealing it is allowed.
	readKeys, err := file.readKeys()
	require.NoError(t, err)

	for i, key := range keys {
		assert.Equal(t, key, readKeys[i])
	}

	// Create a new in-memory instance from the on-disk file and verify that it behaves the same.
	file2, err := loadKeyFile(logger, index, []*SegmentPath{segmentPath}, LatestSegmentVersion)
	require.NoError(t, err)
	require.Equal(t, file.Size(), file2.Size())

	readKeys, err = file2.readKeys()
	require.NoError(t, err)
	for i, key := range keys {
		assert.Equal(t, key, readKeys[i])
	}

	// Create a new version of the key file that only contains the keys at even indices. The intention is to replace
	// the on-disk file with this new version.
	swapFile, err := createKeyFile(logger, index, segmentPath, true, false)
	require.NoError(t, err)
	for i := 0; i < int(keyCount); i += 2 {
		err := swapFile.write(keys[i])
		require.NoError(t, err)
	}
	err = swapFile.seal()
	require.NoError(t, err)

	// Verify that the swap file is present on disk.
	swapFilePath := swapFile.path()
	_, err = os.Stat(swapFilePath)
	require.NoError(t, err)

	// The swap file path should be different from the original file path.
	originalFilePath := file.path()
	require.NotEqual(t, swapFilePath, originalFilePath)

	// Replace the old file with the new one.
	err = swapFile.atomicSwap(false)
	require.NoError(t, err)

	// The old swap file should no longer be present.
	_, err = os.Stat(swapFilePath)
	require.True(t, os.IsNotExist(err))

	// The "regular" file should still be present.
	_, err = os.Stat(originalFilePath)
	require.NoError(t, err)

	// Verify that the file size is correctly reported after the swap.
	reportedSize = swapFile.Size()
	stat, err = os.Stat(swapFile.path())
	require.NoError(t, err)
	actualSize = uint64(stat.Size())
	require.Equal(t, actualSize, reportedSize)

	// Verify the contents of the new file. Reload it from disk just to ensure that we aren't "cheating" somehow.
	file2, err = loadKeyFile(logger, index, []*SegmentPath{segmentPath}, LatestSegmentVersion)
	require.NoError(t, err)
	readKeys, err = file2.readKeys()
	require.NoError(t, err)
	for i, key := range keys {
		if i%2 == 0 {
			assert.Equal(t, key, readKeys[i/2])
		}
	}

	// delete the file
	filePath := file.path()
	_, err = os.Stat(filePath)
	require.NoError(t, err)

	err = file.delete()
	require.NoError(t, err)

	_, err = os.Stat(filePath)
	require.True(t, os.IsNotExist(err))
}
