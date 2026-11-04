package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/disktable"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/disktable/keymap"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/disktable/segment"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/littbuilder"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/util"
	"github.com/stretchr/testify/require"
)

// The full rsync integration suite uses the supported OpenSSH fixture shared
// with repair_112_test.go. It requires sshd and rsync, with no Docker dependency.
func pushTest(t *testing.T, sourceDirs, destDirs uint64, verbose bool) {
	t.Helper()
	pushIntegrationTest(t, sourceDirs, destDirs, verbose, false)
}

func pushIntegrationTest(t *testing.T, sourceDirs, destDirs uint64, verbose, snapshot bool) {
	t.Helper()
	server := repair112StartSSH(t)
	random := util.NewTestRandom()
	parent := t.TempDir()
	var sources, destinations []string
	for index := uint64(0); index < sourceDirs; index++ {
		sources = append(sources, filepath.Join(parent, fmt.Sprintf("source-%d", index)))
	}
	for index := uint64(0); index < destDirs; index++ {
		destinations = append(destinations, filepath.Join(parent, fmt.Sprintf("destination-%d", index)))
	}
	config, err := litt.DefaultConfig(sources...)
	require.NoError(t, err)
	config.DoubleWriteProtection, config.Fsync, config.TargetSegmentFileSize = true, false, 1024
	snapshotRoot := filepath.Join(parent, "snapshot")
	if snapshot {
		config.SnapshotDirectory = snapshotRoot
	}
	tableConfig := litt.DefaultTableConfig("")
	tableConfig.ShardingFactor = uint8(sourceDirs + 2)
	db, err := littbuilder.NewDB(config)
	require.NoError(t, err)
	names := []string{"first", "second", "third"}
	tables := buildTables(t, db, tableConfig, names)
	expected := make(map[string]map[string][]byte)
	for _, name := range names {
		expected[name] = make(map[string][]byte)
	}
	for index := 0; index < 1024; index++ {
		table := tables[names[index%len(names)]]
		key, value := random.PrintableBytes(32), random.PrintableVariableBytes(10, 100)
		expected[table.Name()][string(key)] = value
		require.NoError(t, table.Put(key, value))
	}
	for _, table := range tables {
		require.NoError(t, table.Flush())
	}
	check := func(db litt.DB) {
		opened := buildTables(t, db, tableConfig, names)
		for name, entries := range expected {
			for key, expectedValue := range entries {
				actual, exists, err := opened[name].Get([]byte(key))
				require.NoError(t, err)
				require.True(t, exists)
				require.Equal(t, expectedValue, actual)
			}
		}
	}
	// A live source database cannot be transferred through its primary roots.
	err = push(slog.Default(), sources, destinations, server.user, "127.0.0.1", server.port, server.key, server.knownHosts, false, false, 2, 0, verbose)
	require.Error(t, err)
	for name, entries := range expected {
		for key, expectedValue := range entries {
			actual, exists, err := tables[name].Get([]byte(key))
			require.NoError(t, err)
			require.True(t, exists)
			require.Equal(t, expectedValue, actual)
		}
	}
	require.NoError(t, db.Close())
	transferSources := sources
	if snapshot {
		transferSources = []string{snapshotRoot}
	}
	args := []string{"litt", "push"}
	for _, root := range transferSources {
		args = append(args, "--src", root)
	}
	for _, root := range destinations {
		args = append(args, "--dst", root)
	}
	if !snapshot {
		args = append(args, "--no-gc")
	}
	args = append(args, server.args()...)
	args = append(args, server.user+"@127.0.0.1")
	if !snapshot {
		require.Error(t, push(slog.Default(), sources, destinations, server.user, "127.0.0.1", server.port, server.key, server.knownHosts, true, false, 2, 0, verbose))
	}
	require.NoError(t, buildCLIParser(slog.Default()).Run(args))
	for _, source := range sources {
		require.DirExists(t, source)
	}
	for _, destination := range destinations {
		require.DirExists(t, destination)
	}

	if !snapshot {
		// Corrupt, truncate and relocate real transferred files, then retry.
		// Existing placement must be retained without creating duplicates.
		var files []string
		for _, destination := range destinations {
			require.NoError(t, filepath.WalkDir(destination, func(path string, entry os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				extension := filepath.Ext(path)
				if extension == segment.MetadataFileExtension || extension == segment.KeyFileExtension || extension == segment.ValuesFileExtension {
					files = append(files, path)
				}
				return nil
			}))
		}
		require.NotEmpty(t, files)
		relocated := make(map[string]string)
		for index, file := range files {
			switch index % 4 {
			case 0:
				require.NoError(t, os.Remove(file))
			case 1:
				require.NoError(t, os.WriteFile(file, random.PrintableBytes(100), 0600))
			case 2:
				if len(destinations) > 1 {
					for destinationIndex, root := range destinations {
						relative, err := filepath.Rel(root, file)
						require.NoError(t, err)
						if strings.HasPrefix(relative, "..") {
							continue
						}
						target := filepath.Join(destinations[(destinationIndex+1)%len(destinations)], relative)
						require.NoError(t, os.MkdirAll(filepath.Dir(target), 0755))
						require.NoError(t, os.Rename(file, target))
						relocated[filepath.Base(target)] = target
						break
					}
				}
			}
		}
		require.NoError(t, buildCLIParser(slog.Default()).Run(args))
		for name, target := range relocated {
			require.FileExists(t, target)
			for _, root := range destinations {
				candidate := filepath.Join(root, filepath.Base(filepath.Dir(filepath.Dir(target))), segment.SegmentDirectory, name)
				if candidate != target {
					require.NoFileExists(t, candidate)
				}
			}
		}
	} else {
		for _, name := range names {
			bound, err := disktable.LoadBoundaryFile(disktable.LowerBound, filepath.Join(snapshotRoot, name))
			require.NoError(t, err)
			upper, err := disktable.LoadBoundaryFile(disktable.UpperBound, filepath.Join(snapshotRoot, name))
			require.NoError(t, err)
			require.True(t, bound.IsDefined())
			require.True(t, upper.IsDefined())
			require.Equal(t, upper.BoundaryIndex(), bound.BoundaryIndex())
			entries, err := os.ReadDir(filepath.Join(snapshotRoot, name, segment.SegmentDirectory))
			require.NoError(t, err)
			require.Empty(t, entries)
			for _, root := range sources {
				links, err := os.ReadDir(filepath.Join(root, name, segment.HardLinkDirectory))
				require.NoError(t, err)
				require.Empty(t, links)
			}
		}
	}
	// Recovery of the original DB must still read all data before destroying it.
	db, err = littbuilder.NewDB(config)
	require.NoError(t, err)
	check(db)
	require.NoError(t, db.Destroy())
	for _, root := range destinations {
		require.NoError(t, filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			require.NotEqual(t, keymap.KeymapDirectoryName, entry.Name(), "push must not copy keymaps")
			return nil
		}))
	}
	config.Paths, config.SnapshotDirectory = destinations, ""
	for reopen := 0; reopen < 2; reopen++ {
		db, err = littbuilder.NewDB(config)
		require.NoError(t, err)
		check(db)
		require.NoError(t, db.Close())
	}
}

func TestPush1to1(t *testing.T)     { pushTest(t, 1, 1, false) }
func TestPush1toN(t *testing.T)     { pushTest(t, 1, 4, false) }
func TestPushNto1(t *testing.T)     { pushTest(t, 4, 1, false) }
func TestPushNtoN(t *testing.T)     { pushTest(t, 4, 4, true) }
func TestPushSnapshot(t *testing.T) { pushIntegrationTest(t, 3, 3, false, true) }

func pushInterruptedParticipantsTest(t *testing.T) {
	t.Helper()
	root, snapshot := t.TempDir(), t.TempDir()
	files := repair112Produce(t, root, snapshot, "interrupt_table", 0, time.Now())
	upper, err := disktable.LoadBoundaryFile(disktable.UpperBound, filepath.Join(snapshot, "interrupt_table"))
	require.NoError(t, err)
	require.NoError(t, upper.Update(0))
	sourceDirectory := filepath.Join(snapshot, "interrupt_table", segment.SegmentDirectory)
	heldDirectory := filepath.Join(snapshot, "interrupt_table", "held")
	destination := t.TempDir()
	var interrupted atomic.Bool
	server := repairM1163StartSSH(t, func(command string) error {
		if strings.HasPrefix(command, "mkdir -p ") && interrupted.CompareAndSwap(false, true) {
			// Pause access to all files after planning, before the transfer
			// participants open them. Rsync's real preflight reports the errors.
			return os.Rename(sourceDirectory, heldDirectory)
		}
		return nil
	})
	output := &repairM1163Log{}
	logger := slog.New(slog.NewTextHandler(output, nil))
	err = push(logger, []string{snapshot}, []string{destination}, "fixture", "127.0.0.1", server.port, server.key, server.knownHosts, true, false, 2, 0, false)
	require.ErrorContains(t, err, "error detected during transfer")
	require.True(t, interrupted.Load())
	server.assertClosed(t)
	output.mu.Lock()
	logged := output.Buffer.String()
	output.mu.Unlock()
	require.Equal(t, len(files), strings.Count(logged, "msg=\"monitor encountered an unrecoverable error\""),
		"all participants must finish before push returns")
	require.Equal(t, len(files), strings.Count(logged, "failed to resolve source:"),
		"every participant must report the real source-resolution failure before a subprocess starts")
	lower, err := disktable.LoadBoundaryFile(disktable.LowerBound, filepath.Join(snapshot, "interrupt_table"))
	require.NoError(t, err)
	require.False(t, lower.IsDefined(), "an interrupted attempt cannot acknowledge reclamation")
	for _, file := range files {
		require.FileExists(t, filepath.Join(heldDirectory, filepath.Base(file)))
		require.FileExists(t, filepath.Join(root, "interrupt_table", segment.HardLinkDirectory, filepath.Base(file)))
		require.NoFileExists(t, filepath.Join(destination, "interrupt_table", segment.SegmentDirectory, filepath.Base(file)))
	}
	require.NoError(t, os.Rename(heldDirectory, sourceDirectory))
	repair112AssertRecovered(t, []string{root}, "interrupt_table", 0)
}

func TestPushInterruptedMultiFile(t *testing.T) { pushInterruptedParticipantsTest(t) }
