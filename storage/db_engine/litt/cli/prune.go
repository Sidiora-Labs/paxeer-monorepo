package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/disktable"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/disktable/keymap"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/disktable/segment"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/util"
	"github.com/urfave/cli/v2"
)

// pruneCommand can be used to remove data from a LittDB instance/snapshot.
func pruneCommand(ctx *cli.Context) error {

	logger := slog.Default()

	sources := ctx.StringSlice("src")
	if len(sources) == 0 {
		return fmt.Errorf("no sources provided")
	}
	for i, src := range sources {
		var err error
		sources[i], err = util.SanitizePath(src)
		if err != nil {
			return fmt.Errorf("invalid source path: %s", src)
		}
	}

	tables := ctx.StringSlice("table")

	maxAgeSeconds := ctx.Uint64("max-age")

	return prune(logger, sources, tables, maxAgeSeconds, true)
}

// prune deletes data from a littDB database/snapshot.
func prune(logger *slog.Logger, sources []string, allowedTables []string, maxAgeSeconds uint64, fsync bool) error {
	allowedTablesSet := make(map[string]struct{})
	for _, table := range allowedTables {
		allowedTablesSet[table] = struct{}{}
	}

	// Forbid touching tables in active use.
	releaseLocks, err := util.LockDirectories(logger, sources, util.LockfileName, fsync)
	if err != nil {
		return fmt.Errorf("failed to acquire locks on paths %v: %w", sources, err)
	}
	defer releaseLocks()

	// Determine which tables to prune.
	var tables []string
	foundTables, err := lsPaths(logger, sources, false, fsync)
	if err != nil {
		return fmt.Errorf("failed to list tables in paths %v: %w", sources, err)
	}
	if len(allowedTables) == 0 {
		tables = foundTables
	} else {
		for _, table := range foundTables {
			if _, ok := allowedTablesSet[table]; ok {
				tables = append(tables, table)
			}
		}
	}

	// Prune each table.
	for _, table := range tables {
		bytesDeleted, err := pruneTable(logger, sources, table, maxAgeSeconds, fsync)
		if err != nil {
			return fmt.Errorf("failed to prune table %s in paths %v: %w", table, sources, err)
		}

		logger.Info("Deleted from table",
			"bytes", util.PrettyPrintBytes(bytesDeleted),
			"table", table,
		)
	}

	return nil
}

// pruneTable performs offline garbage collection on a LittDB database/snapshot.
func pruneTable(
	logger *slog.Logger,
	sources []string,
	tableName string,
	maxAgeSeconds uint64,
	fsync bool) (uint64, error) {

	errorMonitor := util.NewErrorMonitor(context.Background(), logger, nil)

	segmentPaths, err := segment.BuildSegmentPaths(sources, "", tableName)
	if err != nil {
		return 0, fmt.Errorf("failed to build segment paths for table %s at paths %v: %w",
			tableName, sources, err)
	}

	lowestSegmentIndex, highestSegmentIndex, segments, isSnapshot, err := gatherPruneSegments(
		logger, errorMonitor, segmentPaths, sources, tableName, fsync)
	if err != nil {
		return 0, err
	}
	if len(segments) == 0 {
		if isSnapshot {
			return 0, nil
		}
		return 0, fmt.Errorf("no segments found for table %s at paths %v", tableName, sources)
	}

	// Delete old segments.
	bytesDeleted := uint64(0)
	deletedSegments := make([]*segment.Segment, 0)
	for index := uint64(lowestSegmentIndex); index <= uint64(highestSegmentIndex); index++ {
		seg := segments[uint32(index)]
		segmentAge := time.Since(seg.GetSealTime())

		if segmentAge < time.Duration(maxAgeSeconds)*time.Second { //nolint:gosec // CLI flag bounded
			// We've pruned all segments that we can.
			break
		}

		deletedSegments = append(deletedSegments, seg)
		bytesDeleted += seg.Size()
		seg.Release()
	}

	// Wait for deletion to complete.
	for _, seg := range deletedSegments {
		err = seg.BlockUntilFullyDeleted()
		if err != nil {
			return 0, fmt.Errorf("failed to block until segment %d is fully deleted: %w",
				seg.SegmentIndex(), err)
		}
	}

	if ok, err := errorMonitor.IsOk(); !ok {
		return 0, fmt.Errorf("error monitor reports errors: %w", err)
	}

	if isSnapshot {
		// This is a snapshot. Write a lower bound file to tell the DB not to re-snapshot files than have been pruned.
		err = writeLowerBoundFile(sources[0], tableName, deletedSegments)
		if err != nil {
			return 0, fmt.Errorf("failed to write lower bound file for table %s at path %s: %w",
				tableName, sources[0], err)
		}
	} else {
		// If we are doing GC on a table that isn't a snapshot, then we need to delete the snapshots/keymap
		// for the table. The DB will automatically rebuild the snapshots directory & keymap on the next startup.
		err = deleteSnapshots(sources, tableName)
		if err != nil {
			return 0, fmt.Errorf("failed to delete snapshots/keymap for table %s at paths %v: %w",
				tableName, sources, err)
		}
	}

	return bytesDeleted, nil
}

// Snapshot discovery must not load or clean files still owned by the publisher.
func gatherPruneSegments(
	logger *slog.Logger,
	monitor *util.ErrorMonitor,
	paths []*segment.SegmentPath,
	sources []string,
	tableName string,
	fsync bool,
) (uint32, uint32, map[uint32]*segment.Segment, bool, error) {
	isSnapshot := false
	var names []string
	for _, segmentPath := range paths {
		entries, err := os.ReadDir(segmentPath.SegmentDirectory())
		if err != nil {
			return 0, 0, nil, false, fmt.Errorf("failed to discover segment files: %w", err)
		}
		for _, entry := range entries {
			extension := path.Ext(entry.Name())
			if extension != segment.MetadataFileExtension && extension != segment.KeyFileExtension &&
				extension != segment.ValuesFileExtension {
				continue
			}
			names = append(names, entry.Name())
			isSnapshot = isSnapshot || entry.Type()&os.ModeSymlink != 0
		}
		for _, boundaryName := range []string{disktable.UpperBoundFileName, disktable.LowerBoundFileName} {
			_, err := os.Lstat(path.Join(path.Dir(segmentPath.SegmentDirectory()), boundaryName))
			if err == nil {
				isSnapshot = true
			} else if !os.IsNotExist(err) {
				return 0, 0, nil, false, err
			}
		}
	}
	if !isSnapshot {
		low, high, segments, err := segment.GatherSegmentFiles(
			logger, monitor, paths, false, time.Now(), true, fsync)
		return low, high, segments, false, err
	}
	if len(sources) != 1 {
		return 0, 0, nil, true, fmt.Errorf("snapshot directory cannot be spread across multiple sources")
	}
	parent := path.Join(sources[0], tableName)
	upper, err := disktable.LoadBoundaryFile(disktable.UpperBound, parent)
	if err != nil {
		return 0, 0, nil, true, err
	}
	segments := make(map[uint32]*segment.Segment)
	if !upper.IsDefined() {
		return 0, 0, segments, true, nil
	}
	lower, err := disktable.LoadBoundaryFile(disktable.LowerBound, parent)
	if err != nil {
		return 0, 0, nil, true, err
	}
	high := upper.BoundaryIndex()
	first := uint64(high) + 1
	for _, name := range names {
		prefix := strings.SplitN(strings.TrimSuffix(name, path.Ext(name)), "-", 2)[0]
		index, err := strconv.ParseUint(prefix, 10, 32)
		if err != nil {
			return 0, 0, nil, true, fmt.Errorf("invalid segment file %s: %w", name, err)
		}
		if index <= uint64(high) && (!lower.IsDefined() || index > uint64(lower.BoundaryIndex())) && index < first {
			first = index
		}
	}
	// A defined lower bound makes a missing published prefix an error, rather
	// than silently accepting a gap and advancing reclamation past it.
	if lower.IsDefined() {
		first = uint64(lower.BoundaryIndex()) + 1
	}
	if first > uint64(high) {
		return 0, 0, segments, true, nil
	}
	for index := first; index <= uint64(high); index++ {
		seg, err := segment.LoadSegment(logger, monitor, uint32(index), paths, false, time.Now(), fsync)
		if err != nil {
			return 0, 0, nil, true, fmt.Errorf("failed to load published segment %d: %w", index, err)
		}
		segments[uint32(index)] = seg
	}
	return uint32(first), high, segments, true, nil
}

// Updates the lower bound file after segments have been deleted.
func writeLowerBoundFile(snapshotRoot string, tableName string, deletedSegments []*segment.Segment) error {
	if len(deletedSegments) == 0 {
		// No segments were deleted, no need to write a lower bound file.
		return nil
	}
	lowerBoundFile, err := disktable.LoadBoundaryFile(disktable.LowerBound, path.Join(snapshotRoot, tableName))
	if err != nil {
		return fmt.Errorf("failed to load boundary file for table %s at path %s: %w",
			tableName, snapshotRoot, err)
	}
	err = lowerBoundFile.Update(deletedSegments[len(deletedSegments)-1].SegmentIndex())
	if err != nil {
		return fmt.Errorf("failed to update lower bound file for table %s at path %s: %w",
			tableName, snapshotRoot, err)
	}

	return nil
}

// deletes the snapshot directories in all sources for the given table
func deleteSnapshots(sources []string, tableName string) error {
	for _, source := range sources {
		snapshotsPath := path.Join(source, tableName, segment.HardLinkDirectory)
		exists, err := util.Exists(snapshotsPath)
		if err != nil {
			return fmt.Errorf("failed to check if snapshots path %s exists: %w", snapshotsPath, err)
		}
		if exists {
			err = os.RemoveAll(snapshotsPath)
			if err != nil {
				return fmt.Errorf("failed to remove snapshots path %s: %w", snapshotsPath, err)
			}
		}

		keymapPath := path.Join(source, tableName, keymap.KeymapDirectoryName)
		exists, err = util.Exists(keymapPath)
		if err != nil {
			return fmt.Errorf("failed to check if keymap path %s exists: %w", keymapPath, err)
		}
		if exists {
			err = os.RemoveAll(keymapPath)
			if err != nil {
				return fmt.Errorf("failed to remove keymap path %s: %w", keymapPath, err)
			}
		}
	}

	return nil
}
