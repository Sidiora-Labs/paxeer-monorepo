package segment

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/util"
	"github.com/stretchr/testify/require"
)

func repairM1162Path(t *testing.T) *SegmentPath {
	t.Helper()
	p, err := NewSegmentPath(t.TempDir(), "", "table")
	require.NoError(t, err)
	require.NoError(t, p.MakeDirectories(false))
	return p
}

func repairM1162DescriptorCount(t *testing.T, p *SegmentPath) int {
	t.Helper()
	entries, err := os.ReadDir("/proc/self/fd")
	require.NoError(t, err)
	count := 0
	for _, entry := range entries {
		target, err := os.Readlink(filepath.Join("/proc/self/fd", entry.Name()))
		if os.IsNotExist(err) {
			continue
		}
		require.NoError(t, err)
		if strings.HasPrefix(target, p.SegmentDirectory()+"/") {
			count++
		}
	}
	return count
}

func TestRepairM_1162_Rotation(t *testing.T) {
	for _, durable := range []bool{false, true} {
		t.Run(fmt.Sprint(durable), func(t *testing.T) {
			p := repairM1162Path(t)
			paths := []*SegmentPath{p}
			logger := slog.Default()
			monitor := util.NewErrorMonitor(t.Context(), logger, nil)
			handles := make([]*os.File, 0, 32)
			for index := uint32(0); index < 32; index++ {
				seg, err := CreateSegment(logger, monitor, index, paths, false, 2, durable)
				require.NoError(t, err)
				handle := seg.keys.file
				handles = append(handles, handle)
				require.Equal(t, 3, repairM1162DescriptorCount(t, p))
				request := &types.PutRequest{
					Key: []byte(fmt.Sprintf("primary-%d", index)), Value: []byte("payload"),
					SecondaryKeys: []*types.SecondaryKey{
						{Key: []byte("alias"), Offset: 0, Length: 7},
						{Key: []byte("slice"), Offset: 1, Length: 3},
					},
				}
				_, _, err = seg.Write(request)
				require.NoError(t, err)
				wait, err := seg.Flush()
				require.NoError(t, err)
				flushed, err := wait()
				require.NoError(t, err)
				require.Len(t, flushed, 3)
				_, err = handle.Stat()
				require.NoError(t, err, "an ordinary flush must retain the writable handle")
				_, _, err = seg.Write(&types.PutRequest{Key: []byte("standalone"), Value: []byte("second")})
				require.NoError(t, err)
				sealed, err := seg.Seal(time.Now())
				require.NoError(t, err)
				require.Len(t, sealed, 1)
				require.Nil(t, seg.keys.file)
				require.Nil(t, seg.keys.writer)
				for _, owned := range handles {
					_, err := owned.Stat()
					require.ErrorIs(t, err, os.ErrClosed)
				}
				require.Zero(t, repairM1162DescriptorCount(t, p))
				reopened, err := LoadSegment(logger, monitor, index, paths, false, time.Now(), durable)
				require.NoError(t, err)
				keys, err := reopened.GetKeys()
				require.NoError(t, err)
				expected := append(flushed, sealed...)
				require.Equal(t, expected, keys)
				for i, value := range []string{"payload", "payload", "ayl", "second"} {
					actual, err := reopened.Read(keys[i].Key, keys[i].Address)
					require.NoError(t, err)
					require.Equal(t, []byte(value), actual)
				}
				require.Zero(t, repairM1162DescriptorCount(t, p))
			}
		})
	}
}

func TestRepairM_1162_SealErrors(t *testing.T) {
	for _, fault := range []string{"flush", "sync", "close"} {
		t.Run(fault, func(t *testing.T) {
			p := repairM1162Path(t)
			keys, err := createKeyFile(slog.Default(), 0, p, false, fault == "sync")
			require.NoError(t, err)
			require.NoError(t, keys.file.Close())
			if fault == "flush" {
				keys.file, err = os.Open(keys.path())
				require.NoError(t, err)
				keys.writer = bufio.NewWriter(keys.file)
				require.NoError(t, keys.write(&types.ScopedKey{Key: []byte("buffered"), Address: types.NewAddress(0, 1, 2, 3)}))
			}
			handle := keys.file
			t.Cleanup(func() { _ = handle.Close() })
			err = keys.seal()
			require.ErrorContains(t, err, fault+" key file")
			if fault == "sync" {
				require.ErrorIs(t, err, os.ErrClosed)
				require.ErrorContains(t, err, "close key file")
			}
			_, err = handle.Stat()
			require.ErrorIs(t, err, os.ErrClosed, "seal must close even when flushing fails")
			require.Nil(t, keys.file)
			require.Nil(t, keys.writer)
			require.ErrorContains(t, keys.write(&types.ScopedKey{Key: []byte("later")}), "sealed")
			require.ErrorContains(t, keys.flush(), "sealed")
			require.ErrorContains(t, keys.seal(), "already sealed")
			require.Zero(t, repairM1162DescriptorCount(t, p))
		})
	}
}

func TestRepairM_1162_ConstructorErrors(t *testing.T) {
	p := repairM1162Path(t)
	logger := slog.Default()
	monitor := util.NewErrorMonitor(t.Context(), logger, nil)
	for index := uint32(0); index < 32; index++ {
		valuePath := filepath.Join(p.SegmentDirectory(), fmt.Sprintf("%d-0%s", index, ValuesFileExtension))
		require.NoError(t, os.WriteFile(valuePath, []byte("existing"), 0600))
		seg, err := CreateSegment(logger, monitor, index, []*SegmentPath{p}, false, 1, true)
		require.Nil(t, seg)
		require.ErrorContains(t, err, "already exists")
		require.Zero(t, repairM1162DescriptorCount(t, p), "failed segment construction must release the key handle")
	}
}

func TestRepairM_1162_TruncatedKeys(t *testing.T) {
	p := repairM1162Path(t)
	inputs := []*types.ScopedKey{
		{Key: []byte{}, Kind: types.KeyKindStandalone, Address: types.NewAddress(1, 2, 3, 4)},
		{Key: []byte{0, 255, 0, 128}, Kind: types.KeyKindPrimary, Address: types.NewAddress(12345, 67890, 254, 65537)},
		{Key: []byte("secondary"), Kind: types.KeyKindSecondary, Address: types.NewAddress(9, 8, 7, 6)},
		{Key: []byte("final"), Kind: types.KeyKindFinalSecondary, Address: types.NewAddress(0xffffffff, 0xfffffffe, 255, 0xfffffffd)},
		{Key: []byte("tail"), Kind: types.KeyKindStandalone, Address: types.NewAddress(5, 4, 3, 2)},
	}
	keys, err := createKeyFile(slog.Default(), 0, p, false, true)
	require.NoError(t, err)
	ends := make([]int, len(inputs))
	for i, key := range inputs {
		require.NoError(t, keys.write(key))
		ends[i] = int(keys.Size())
	}
	require.NoError(t, keys.seal())
	original, err := os.ReadFile(keys.path())
	require.NoError(t, err)
	for cut := 0; cut <= len(original); cut++ {
		t.Run(fmt.Sprintf("bytes_%d", cut), func(t *testing.T) {
			require.NoError(t, os.WriteFile(keys.path(), original[:cut], 0600))
			reopened, err := loadKeyFile(slog.Default(), 0, []*SegmentPath{p}, LatestSegmentVersion)
			require.NoError(t, err)
			actual, err := reopened.readKeys()
			require.NoError(t, err)
			count := 0
			for _, end := range ends {
				if end <= cut {
					count++
				}
			}
			require.Len(t, actual, count)
			for i, key := range inputs[:count] {
				require.Equal(t, key.Key, actual[i].Key)
				require.Equal(t, key.Kind, actual[i].Kind)
				require.Equal(t, key.Address.Index(), actual[i].Address.Index())
				require.Equal(t, key.Address.Offset(), actual[i].Address.Offset())
				require.Equal(t, key.Address.ShardID(), actual[i].Address.ShardID())
				require.Equal(t, key.Address.ValueSize(), actual[i].Address.ValueSize())
			}
		})
	}
}
