package segment

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/disktable/keymap"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/util"
	"github.com/stretchr/testify/require"
)

const repair110WriterEnv = "LITT_REPAIR_110_WRITER"

type repair110WriterConfig struct {
	Root string
	Mode string
}

func repair110Paths(t *testing.T, root string) []*SegmentPath {
	t.Helper()
	paths, err := BuildSegmentPaths([]string{filepath.Join(root, "a"), filepath.Join(root, "b")}, "", "table")
	require.NoError(t, err)
	for _, p := range paths {
		require.NoError(t, p.MakeDirectories(true))
	}
	return paths
}

func repair110Request() *types.PutRequest {
	return &types.PutRequest{
		Key: []byte("primary"), Value: []byte("prefix-payload-suffix"),
		SecondaryKeys: []*types.SecondaryKey{
			{Key: []byte("alias"), Offset: 0, Length: 21},
			{Key: []byte("slice"), Offset: 7, Length: 7},
		},
	}
}

// The child deliberately exits without sealing/closing the segment after an
// acknowledged flush. Recovery cannot depend on deferred cleanup in this process.
func repair110CrashWriter(t *testing.T, cfg repair110WriterConfig) {
	t.Helper()
	logger := slog.Default()
	paths := repair110Paths(t, cfg.Root)
	seg, err := CreateSegment(logger, util.NewErrorMonitor(t.Context(), logger, nil), 0, paths, false, 2, true)
	require.NoError(t, err)
	require.True(t, seg.keys.fsync)
	if cfg.Mode != "initial" {
		_, _, err = seg.Write(repair110Request())
		require.NoError(t, err)
	}
	if cfg.Mode == "seal" {
		keys, err := seg.Seal(time.Now())
		require.NoError(t, err)
		require.Len(t, keys, 3)
	} else {
		wait, err := seg.Flush()
		require.NoError(t, err)
		keys, err := wait()
		require.NoError(t, err)
		if cfg.Mode == "initial" {
			require.Empty(t, keys)
		} else {
			require.Len(t, keys, 3)
		}
		if cfg.Mode == "rewrite" {
			// A crash can persist a primary without its group's closing record.
			// Use the real key codec to leave that unacknowledged tail behind.
			require.NoError(t, seg.keys.write(&types.ScopedKey{
				Key: []byte("torn"), Kind: types.KeyKindPrimary,
				Address: types.NewAddress(0, 21, 0, 1),
			}))
			require.NoError(t, seg.keys.flush())
		}
	}
	os.Exit(0)
}

func repair110RunWriter(t *testing.T, root, mode string) {
	t.Helper()
	cfg, err := json.Marshal(repair110WriterConfig{Root: root, Mode: mode})
	require.NoError(t, err)
	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestRepair_110_KeyDurabilityRestart$", "-test.count=1")
	cmd.Env = append(os.Environ(), repair110WriterEnv+"="+string(cfg))
	output, err := cmd.CombinedOutput()
	require.NoError(t, err, "%s", output)
}

func TestRepair_110_KeyDurabilityRestart(t *testing.T) {
	if cfgJSON := os.Getenv(repair110WriterEnv); cfgJSON != "" {
		var cfg repair110WriterConfig
		require.NoError(t, json.Unmarshal([]byte(cfgJSON), &cfg))
		repair110CrashWriter(t, cfg)
		return
	}
	for _, mode := range []string{"flush", "seal", "rewrite"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			repair110RunWriter(t, root, mode)
			paths := repair110Paths(t, root)
			logger := slog.Default()
			monitor := util.NewErrorMonitor(t.Context(), logger, nil)
			low, high, segments, err := GatherSegmentFiles(logger, monitor, paths, false, time.Now(), true, true)
			require.NoError(t, err)
			require.Equal(t, uint32(0), low)
			require.Equal(t, uint32(0), high)
			require.Len(t, segments, 1)
			recovered := segments[0]
			require.True(t, recovered.IsSealed())
			keys, err := recovered.GetKeys()
			require.NoError(t, err)
			require.Len(t, keys, 3)
			require.Equal(t, uint32(3), recovered.KeyCount())
			require.Equal(t, []types.KeyKind{types.KeyKindPrimary, types.KeyKindSecondary, types.KeyKindFinalSecondary},
				[]types.KeyKind{keys[0].Kind, keys[1].Kind, keys[2].Kind})
			if mode == "rewrite" {
				// Recovery's replacement file must inherit the production sync mode.
				require.True(t, recovered.keys.fsync)
				require.Nil(t, recovered.keys.file)
				_, err = os.Stat(recovered.keys.path() + util.SwapFileExtension)
				require.ErrorIs(t, err, os.ErrNotExist)
			}

			// Rebuild both real keymap implementations solely from recovered keys.
			for _, kind := range []string{"memory", "durable"} {
				t.Run(kind, func(t *testing.T) {
					var build keymap.BuildKeymap = keymap.NewMemKeymap
					if kind == "durable" {
						build = keymap.NewPebbleDBKeymap
					}
					keymapPath := filepath.Join(t.TempDir(), "rebuilt")
					km, reload, err := build(logger, keymapPath, true)
					require.NoError(t, err)
					require.True(t, reload)
					require.NoError(t, km.Put(keys))
					require.NoError(t, km.Stop())
					km, reload, err = build(logger, keymapPath, true)
					require.NoError(t, err)
					t.Cleanup(func() { require.NoError(t, km.Stop()) })
					if kind == "memory" {
						require.True(t, reload)
						require.NoError(t, km.Put(keys))
					} else {
						require.False(t, reload)
					}
					for key, value := range map[string]string{
						"primary": "prefix-payload-suffix", "alias": "prefix-payload-suffix", "slice": "payload",
					} {
						address, found, err := km.Get([]byte(key))
						require.NoError(t, err)
						require.True(t, found, key)
						data, err := recovered.Read([]byte(key), address)
						require.NoError(t, err)
						require.Equal(t, value, string(data))
					}
					_, found, err := km.Get([]byte("torn"))
					require.NoError(t, err)
					require.False(t, found)
				})
			}
			// Reopen again to prove the recovery rewrite and metadata are stable.
			again, err := LoadSegment(logger, monitor, 0, paths, false, time.Now(), true)
			require.NoError(t, err)
			againKeys, err := again.GetKeys()
			require.NoError(t, err)
			require.Equal(t, keys, againKeys)
		})
	}
}

func TestRepair_110_KeySyncFailure(t *testing.T) {
	for _, operation := range []string{"flush", "seal"} {
		t.Run(operation, func(t *testing.T) {
			logger := slog.Default()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			monitor := util.NewErrorMonitor(ctx, logger, nil)
			paths := repair110Paths(t, t.TempDir())
			seg, err := CreateSegment(logger, monitor, 0, paths, false, 2, true)
			require.NoError(t, err)
			_, _, err = seg.Write(repair110Request())
			require.NoError(t, err)
			wait, err := seg.Flush()
			require.NoError(t, err)
			keys, err := wait()
			require.NoError(t, err)
			require.Len(t, keys, 3)

			// The buffer is empty, so flushing it cannot notice a closed descriptor.
			// Only the actual fsync can detect this durability-boundary fault.
			require.NoError(t, seg.keys.file.Close())
			if operation == "seal" {
				keys, err = seg.Seal(time.Now())
			} else {
				wait, err = seg.Flush()
				require.NoError(t, err)
				keys, err = wait()
			}
			require.Error(t, err)
			require.Empty(t, keys, "a failed sync must not acknowledge keys")
			ok, failure := monitor.IsOk()
			require.False(t, ok)
			require.ErrorIs(t, failure, os.ErrClosed)
			require.ErrorContains(t, failure, "sync key file")
			metadata, err := loadMetadataFile(0, paths, true)
			require.NoError(t, err)
			require.False(t, metadata.sealed, "failed key sync must not publish sealed metadata")
			if operation == "flush" {
				// No further writes are queued; close the value handles after the wait.
				for _, shard := range seg.shards {
					require.NoError(t, shard.file.Close())
				}
			}
		})
	}
}

func TestRepair_110_KeyDurabilityMode(t *testing.T) {
	for _, sync := range []bool{false, true} {
		t.Run(fmt.Sprint(sync), func(t *testing.T) {
			paths := repair110Paths(t, t.TempDir())
			keys, err := createKeyFile(slog.Default(), 0, paths[0], false, sync)
			require.NoError(t, err)
			require.Equal(t, sync, keys.fsync)
			require.NoError(t, keys.write(&types.ScopedKey{
				Key: []byte("key"), Address: types.NewAddress(0, 0, 0, 1),
			}))
			require.NoError(t, keys.writer.Flush())
			require.NoError(t, keys.file.Close())
			err = keys.flush()
			if sync {
				require.ErrorIs(t, err, os.ErrClosed)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestRepair_110_InitialSegmentRecovery(t *testing.T) {
	// Each bit represents one of metadata, keys, shard 0, and shard 1.
	// Enumerate every persisted subset, including empty and fully created sets.
	for mask := 0; mask < 16; mask++ {
		for _, clean := range []bool{false, true} {
			t.Run(fmt.Sprintf("files_%04b/clean_%t", mask, clean), func(t *testing.T) {
				root := t.TempDir()
				repair110RunWriter(t, root, "initial")
				paths := repair110Paths(t, root)
				files := []string{
					filepath.Join(paths[0].SegmentDirectory(), "0"+MetadataFileExtension),
					filepath.Join(paths[0].SegmentDirectory(), "0"+KeyFileExtension),
					filepath.Join(paths[1].SegmentDirectory(), "0-0"+ValuesFileExtension),
					filepath.Join(paths[0].SegmentDirectory(), "0-1"+ValuesFileExtension),
				}
				for bit, file := range files {
					if mask&(1<<bit) == 0 {
						require.NoError(t, os.Remove(file))
					}
				}
				logger := slog.Default()
				monitor := util.NewErrorMonitor(t.Context(), logger, nil)
				low, high, segments, err := GatherSegmentFiles(logger, monitor, paths, false, time.Now(), clean, true)
				require.NoError(t, err)
				require.Zero(t, low)
				require.Zero(t, high)
				if mask == 15 {
					require.Len(t, segments, 1)
					require.True(t, segments[0].IsSealed())
					keys, err := segments[0].GetKeys()
					require.NoError(t, err)
					require.Empty(t, keys)
					return
				}
				require.Empty(t, segments)
				for bit, file := range files {
					_, err := os.Stat(file)
					if !clean && mask&(1<<bit) != 0 {
						require.NoError(t, err)
					} else {
						require.ErrorIs(t, err, os.ErrNotExist)
					}
				}
				// Startup with cleanup must permit creating and using segment zero.
				_, _, segments, err = GatherSegmentFiles(logger, monitor, paths, false, time.Now(), true, true)
				require.NoError(t, err)
				require.Empty(t, segments)
				seg, err := CreateSegment(logger, monitor, 0, paths, false, 2, true)
				require.NoError(t, err)
				_, _, err = seg.Write(repair110Request())
				require.NoError(t, err)
				keys, err := seg.Seal(time.Now())
				require.NoError(t, err)
				require.Len(t, keys, 3)
				_, _, segments, err = GatherSegmentFiles(logger, monitor, paths, false, time.Now(), true, true)
				require.NoError(t, err)
				require.Len(t, segments, 1)
				recoveredKeys, err := segments[0].GetKeys()
				require.NoError(t, err)
				require.Equal(t, keys, recoveredKeys)
			})
		}
	}
}

func TestRepair_110_HealthySegmentSurvivors(t *testing.T) {
	for _, damaged := range [][]uint32{{0, 2}, {0, 1, 2}, {1}} {
		t.Run(fmt.Sprint(damaged), func(t *testing.T) {
			logger := slog.Default()
			monitor := util.NewErrorMonitor(t.Context(), logger, nil)
			paths := repair110Paths(t, t.TempDir())
			for index := uint32(0); index < 3; index++ {
				seg, err := CreateSegment(logger, monitor, index, paths, false, 2, true)
				require.NoError(t, err)
				_, _, err = seg.Write(repair110Request())
				require.NoError(t, err)
				_, err = seg.Seal(time.Now())
				require.NoError(t, err)
			}
			for _, index := range damaged {
				require.NoError(t, os.Remove(filepath.Join(paths[0].SegmentDirectory(), fmt.Sprintf("%d.keys", index))))
			}
			low, high, segments, err := GatherSegmentFiles(logger, monitor, paths, false, time.Now(), true, true)
			if len(damaged) != 2 {
				// Losing an internal segment remains unrecoverable corruption.
				require.ErrorContains(t, err, "missing key file for segment 1")
				return
			}
			require.NoError(t, err)
			require.Equal(t, uint32(1), low)
			require.Equal(t, uint32(1), high)
			require.Len(t, segments, 1)
			keys, err := segments[1].GetKeys()
			require.NoError(t, err)
			require.Len(t, keys, 3)
			for _, key := range keys {
				value, err := segments[1].Read(key.Key, key.Address)
				require.NoError(t, err)
				require.NotEmpty(t, value)
			}
		})
	}
}
