package types_test

import (
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/disktable/segment"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/util"
	"github.com/stretchr/testify/require"
)

type repairM1173Value struct {
	offset uint32
	shard  uint8
	value  []byte
}

// Addresses come from the production writer and persisted key-file decoder.
func repairM1173Segment(t *testing.T) (*segment.Segment, []*segment.SegmentPath, map[string]repairM1173Value) {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	monitor := util.NewErrorMonitor(t.Context(), logger, nil)
	paths, err := segment.BuildSegmentPaths([]string{t.TempDir()}, "", "address")
	require.NoError(t, err)
	require.NoError(t, paths[0].MakeDirectories(false))
	s, err := segment.CreateSegment(logger, monitor, 7, paths, false, 2, false)
	require.NoError(t, err)

	// These first bytes resemble a length prefix but are part of the value.
	guard := []byte{0, 0, 0, 2, 'a', 'b'}
	for _, request := range []*types.PutRequest{
		{Key: []byte("guard"), Value: guard},
		{Key: []byte("other-shard"), Value: []byte("other")},
		{
			Key: []byte("primary"), Value: []byte("prefix-payload-suffix"),
			SecondaryKeys: []*types.SecondaryKey{
				{Key: []byte("slice"), Offset: 7, Length: 7},
				{Key: []byte("alias"), Offset: 0, Length: 21},
				{Key: []byte("empty-end"), Offset: 21, Length: 0},
			},
		},
	} {
		_, _, err := s.Write(request)
		require.NoError(t, err)
	}
	durable, err := s.Seal(time.Unix(1, 0))
	require.NoError(t, err)
	require.Len(t, durable, 6)
	require.True(t, s.IsSealed())

	expected := map[string]repairM1173Value{
		"guard":       {offset: 0, shard: 0, value: guard},
		"other-shard": {offset: 0, shard: 1, value: []byte("other")},
		"primary":     {offset: 6, shard: 0, value: []byte("prefix-payload-suffix")},
		"slice":       {offset: 13, shard: 0, value: []byte("payload")},
		"alias":       {offset: 6, shard: 0, value: []byte("prefix-payload-suffix")},
		"empty-end":   {offset: 27, shard: 0, value: []byte{}},
	}
	return s, paths, expected
}

func repairM1173Reopen(t *testing.T, paths []*segment.SegmentPath) *segment.Segment {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	monitor := util.NewErrorMonitor(t.Context(), logger, nil)
	s, err := segment.LoadSegment(logger, monitor, 7, paths, false, time.Unix(2, 0), false)
	require.NoError(t, err)
	require.True(t, s.IsSealed())
	return s
}

func TestRepairM_1173_AddressRawByteSpan(t *testing.T) {
	s, paths, expected := repairM1173Segment(t)
	files := s.GetValueFilePaths()
	require.Len(t, files, 2)
	for shard, want := range [][]byte{
		append([]byte{0, 0, 0, 2, 'a', 'b'}, []byte("prefix-payload-suffix")...),
		[]byte("other"),
	} {
		raw, err := os.ReadFile(files[shard])
		require.NoError(t, err)
		require.Equal(t, want, raw, "shard %d must contain only raw primary bytes", shard)
	}

	s = repairM1173Reopen(t, paths)
	keys, err := s.GetKeys()
	require.NoError(t, err)
	require.Len(t, keys, len(expected))
	reader := s.NewReader()
	t.Cleanup(func() { require.NoError(t, reader.Close()) })
	seen := make(map[string]bool)
	for _, key := range keys {
		name := string(key.Key)
		want, ok := expected[name]
		require.True(t, ok, "unexpected persisted key %q", name)
		require.False(t, seen[name], "duplicate persisted key %q", name)
		seen[name] = true
		address := key.Address
		require.Equal(t, uint32(7), address.Index(), name)
		require.Equal(t, want.offset, address.Offset(), name)
		require.Equal(t, want.shard, address.ShardID(), name)
		require.Equal(t, uint32(len(want.value)), address.ValueSize(), name)

		wire := address.Serialize()
		require.Equal(t, 13, types.AddressSerializedSize)
		require.Len(t, wire, 13)
		require.Equal(t, uint32(7), binary.BigEndian.Uint32(wire[0:4]))
		require.Equal(t, want.offset, binary.BigEndian.Uint32(wire[4:8]))
		require.Equal(t, want.shard, wire[8])
		require.Equal(t, uint32(len(want.value)), binary.BigEndian.Uint32(wire[9:13]))
		decoded, err := types.DeserializeAddress(wire)
		require.NoError(t, err)
		require.Equal(t, address, decoded)
		value, err := s.Read(key.Key, decoded)
		require.NoError(t, err, name)
		require.Equal(t, want.value, value, name)
		value, err = reader.Read(decoded)
		require.NoError(t, err, name)
		require.Equal(t, want.value, value, name)
	}
}

func TestRepairM_1173_TruncationUsesExactSpan(t *testing.T) {
	// Endpoints are raw offsets: guard ends at 6, slice at 20, primary at 27.
	for _, remaining := range []uint32{6, 20, 26, 27} {
		t.Run(fmt.Sprintf("bytes_%d", remaining), func(t *testing.T) {
			s, paths, expected := repairM1173Segment(t)
			require.NoError(t, os.Truncate(s.GetValueFilePaths()[0], int64(remaining)))
			s = repairM1173Reopen(t, paths)
			keys, err := s.GetKeys()
			require.NoError(t, err)
			require.Len(t, keys, len(expected))
			reader := s.NewReader()
			t.Cleanup(func() { require.NoError(t, reader.Close()) })
			for _, key := range keys {
				name := string(key.Key)
				want, ok := expected[name]
				require.True(t, ok, name)
				value, readErr := s.Read(key.Key, key.Address)
				buffered, bufferedErr := reader.Read(key.Address)
				if want.shard == 0 && want.offset+uint32(len(want.value)) > remaining {
					require.ErrorContains(t, readErr, "out of bounds", name)
					require.ErrorContains(t, bufferedErr, "out of bounds", name)
					require.Nil(t, value, name)
					require.Nil(t, buffered, name)
				} else {
					require.NoError(t, readErr, name)
					require.NoError(t, bufferedErr, name)
					require.Equal(t, want.value, value, name)
					require.Equal(t, want.value, buffered, name)
				}
			}
		})
	}
}
