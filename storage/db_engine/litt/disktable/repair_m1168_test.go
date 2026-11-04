package disktable

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/disktable/segment"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/types"
	"github.com/stretchr/testify/require"
)

func TestRepairM_1168_BoundaryIndexRange(t *testing.T) {
	for _, kind := range []BoundaryType{LowerBound, UpperBound} {
		t.Run(fmt.Sprint(kind), func(t *testing.T) {
			for _, index := range []uint32{0, 1, math.MaxUint32 - 1, math.MaxUint32} {
				t.Run(fmt.Sprint(index), func(t *testing.T) {
					root := t.TempDir()
					boundary, err := LoadBoundaryFile(kind, root)
					require.NoError(t, err)
					require.False(t, boundary.IsDefined())
					require.NoError(t, boundary.Update(index))
					encoded, err := os.ReadFile(boundary.Path())
					require.NoError(t, err)
					require.Equal(t, fmt.Sprintf("%d\n", index), string(encoded))
					reloaded, err := LoadBoundaryFile(kind, root)
					require.NoError(t, err)
					require.True(t, reloaded.IsDefined())
					require.Equal(t, index, reloaded.BoundaryIndex())
					require.NoError(t, os.WriteFile(boundary.Path(), []byte(" \t\r\n"+string(encoded)+"\t "), 0600))
					reloaded, err = LoadBoundaryFile(kind, root)
					require.NoError(t, err, "file loading must normalize surrounding whitespace")
					require.True(t, reloaded.IsDefined())
					require.Equal(t, index, reloaded.BoundaryIndex())
				})
			}
			for _, invalid := range []string{"-1", "-4294967296", "4294967296", "18446744073709551616", "", "1 2"} {
				t.Run("invalid-"+invalid, func(t *testing.T) {
					root := t.TempDir()
					boundary, err := LoadBoundaryFile(kind, root)
					require.NoError(t, err)
					require.NoError(t, boundary.Update(17))
					require.Error(t, boundary.deserialize([]byte(invalid)))
					require.Equal(t, uint32(17), boundary.BoundaryIndex(), "failed decoding must preserve the boundary")
					require.NoError(t, os.WriteFile(boundary.Path(), []byte(" \t"+invalid+"\r\n"), 0600))
					loaded, err := LoadBoundaryFile(kind, root)
					require.ErrorContains(t, err, "failed to parse boundary index")
					require.Nil(t, loaded, "invalid boundaries must not reach consumers")
				})
			}
		})
	}
}

func repairM1168SegmentBytes(t *testing.T, root, name string) map[string][]byte {
	t.Helper()
	directory := filepath.Join(root, name, segment.SegmentDirectory)
	entries, err := os.ReadDir(directory)
	require.NoError(t, err)
	contents := make(map[string][]byte, len(entries))
	for _, entry := range entries {
		data, err := os.ReadFile(filepath.Join(directory, entry.Name()))
		require.NoError(t, err)
		contents[entry.Name()] = data
	}
	return contents
}

func repairM1168Read(t *testing.T, table litt.ManagedTable, expected map[string]string, missing []string) {
	t.Helper()
	for key, want := range expected {
		value, exists, err := table.Get([]byte(key))
		require.NoError(t, err, key)
		require.True(t, exists, key)
		require.Equal(t, want, string(value), key)
	}
	for _, key := range missing {
		_, exists, err := table.Get([]byte(key))
		require.NoError(t, err, key)
		require.False(t, exists, key)
		exists, err = table.Exists([]byte(key))
		require.NoError(t, err, key)
		require.False(t, exists, key)
		_, exists, cached, err := table.(*DiskTable).CacheAwareGet([]byte(key), false)
		require.NoError(t, err, key)
		require.False(t, exists, key)
		require.False(t, cached, key)
		_, exists, err = table.(*DiskTable).keymap.Get([]byte(key))
		require.NoError(t, err, key)
		require.False(t, exists, key)
	}
	require.Equal(t, uint64(len(expected)), table.KeyCount())
}

func TestRepairM_1168_NilBatchRequest(t *testing.T) {
	for _, builder := range tableBuilders {
		t.Run(builder.name, func(t *testing.T) {
			root := t.TempDir()
			const name = "nil-batch"
			table, err := builder.builder(time.Now, name, []string{root})
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, table.Close()) })
			require.NoError(t, table.Put([]byte("existing"), []byte("original"),
				&types.SecondaryKey{Key: []byte("existing-alias"), Offset: 1, Length: 3}))
			require.NoError(t, table.Flush())
			before := repairM1168SegmentBytes(t, root, name)
			size := table.Size()
			expected := map[string]string{"existing": "original", "existing-alias": "rig"}
			missing := []string{"candidate", "parent", "alias", "end", "trailing"}
			plain := &types.PutRequest{Key: []byte("candidate"), Value: []byte("candidate-value")}
			group := &types.PutRequest{Key: []byte("parent"), Value: []byte("abcdef"),
				SecondaryKeys: []*types.SecondaryKey{
					{Key: []byte("alias"), Offset: 1, Length: 3},
					{Key: []byte("end"), Offset: 6},
				}}
			trailing := &types.PutRequest{Key: []byte("trailing"), Value: []byte("last")}
			for _, batch := range [][]*types.PutRequest{
				{nil}, {nil, plain, group}, {plain, group, nil}, {plain, group, nil, trailing},
			} {
				require.NotPanics(t, func() {
					err = table.PutBatch(batch)
				})
				require.ErrorContains(t, err, "nil batch request")
				repairM1168Read(t, table, expected, missing)
				table.(*DiskTable).unflushedDataCache.Range(func(key, value any) bool {
					t.Errorf("rejected batch populated the cache: %v", key)
					return true
				})
				require.Equal(t, size, table.Size())
				require.NoError(t, table.Flush(), "drain the real writer to expose any wrongly enqueued request")
				repairM1168Read(t, table, expected, missing)
				require.Equal(t, before, repairM1168SegmentBytes(t, root, name))
			}
			require.NoError(t, table.Close())
			table, err = builder.builder(time.Now, name, []string{root})
			require.NoError(t, err)
			repairM1168Read(t, table, expected, missing)
			require.NoError(t, table.PutBatch([]*types.PutRequest{plain, group, trailing}))
			expected["candidate"] = "candidate-value"
			expected["parent"] = "abcdef"
			expected["alias"] = "bcd"
			expected["end"] = ""
			expected["trailing"] = "last"
			repairM1168Read(t, table, expected, nil)
			require.NoError(t, table.Flush())
			require.NoError(t, table.Close())
			table, err = builder.builder(time.Now, name, []string{root})
			require.NoError(t, err)
			repairM1168Read(t, table, expected, nil)
		})
	}
}

func TestRepairM_1168_V3TruncationFixture(t *testing.T) {
	for _, builder := range tableBuilders {
		t.Run(builder.name, func(t *testing.T) {
			truncatedValueFileTest(t, builder)
		})
	}
}
