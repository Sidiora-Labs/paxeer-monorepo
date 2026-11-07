package main

import (
	"context"
	"fmt"
	"log/slog"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/disktable"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/disktable/segment"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/util"
	"github.com/urfave/cli/v2"
)

func pushCommand(ctx *cli.Context) error {
	if ctx.NArg() < 1 {
		return fmt.Errorf("not enough arguments provided, must provide USER@HOST")
	}

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

	destinations := ctx.StringSlice(dstFlag.Name)
	if len(destinations) == 0 {
		return fmt.Errorf("no destinations provided")
	}

	userHost := ctx.Args().First()
	parts := strings.Split(userHost, "@")
	if len(parts) != 2 {
		return fmt.Errorf("invalid USER@HOST format: %s", userHost)
	}
	user := parts[0]
	host := parts[1]

	port := ctx.Uint64("port")

	keyPath := ctx.String("key")
	keyPath, err := util.SanitizePath(keyPath)
	if err != nil {
		return fmt.Errorf("invalid key path: %s", keyPath)
	}

	knownHosts := ctx.String(knownHostsFileFlag.Name)

	deleteAfterTransfer := !ctx.Bool("no-gc")
	threads := ctx.Uint64("threads")
	verbose := !ctx.Bool("quiet")
	throttleMB := ctx.Float64("throttle")

	return push(
		logger,
		sources,
		destinations,
		user,
		host,
		port,
		keyPath,
		knownHosts,
		deleteAfterTransfer,
		true,
		threads,
		throttleMB,
		verbose)
}

// push uses rsync to transfer LittDB data to the remote location(s)
func push(
	logger *slog.Logger,
	sources []string,
	destinations []string,
	user string,
	host string,
	port uint64,
	keyPath string,
	knownHosts string,
	deleteAfterTransfer bool,
	fsync bool,
	threads uint64,
	throttleMB float64,
	verbose bool) error {

	if len(sources) == 0 {
		return fmt.Errorf("no source paths provided")
	}
	if len(destinations) == 0 {
		return fmt.Errorf("no destination paths provided")
	}
	if threads == 0 {
		return fmt.Errorf("threads must be greater than 0")
	}

	// split bandwidth between workers
	throttleMB /= float64(threads)

	// Lock source files. It would be nice to also lock the remote directories, but that's tricky given that
	// we are interacting with the remote machine via SSH and rsync.
	releaseSourceLocks, err := util.LockDirectories(logger, sources, util.LockfileName, fsync)
	if err != nil {
		return fmt.Errorf("failed to lock source directories: %w", err)
	}
	defer releaseSourceLocks()

	// Create an SSH session to the remote host.
	connection, err := util.NewSSHSession(logger, user, host, port, keyPath, knownHosts, verbose)
	if err != nil {
		return fmt.Errorf("failed to create SSH session to %s@%s port %d: %w", user, host, port, err)
	}

	defer func() {
		if err := connection.Close(); err != nil {
			logger.Error("Failed to close SSH session", "error", err)
		}
	}()

	tables, err := lsPaths(logger, sources, false, fsync)
	if err != nil {
		return fmt.Errorf("failed to list tables in source paths %v: %w", sources, err)
	}

	for _, tableName := range tables {
		err = pushTable(
			logger,
			tableName,
			sources,
			destinations,
			connection,
			deleteAfterTransfer,
			fsync,
			throttleMB,
			threads,
		)

		if err != nil {
			return fmt.Errorf("failed to push table %s: %w", tableName, err)
		}
	}

	return nil
}

// Figure out which files are already present at the destination(s). Although these files may be partial, we always
// want to preserve any pre-existing arrangements of files at the destination(s).
//
// The returned map is a map from file name (e.g. 1234.metadata) to the destination path (e.g. /path/to/remote/dir).
func mapExistingFiles(
	destinations []string,
	tableName string,
	connection *util.SSHSession) (map[string]string, error) {

	existingFiles := make(map[string]string)

	extensions := []string{segment.MetadataFileExtension, segment.KeyFileExtension, segment.ValuesFileExtension}

	for _, dest := range destinations {
		tableDestination := path.Join(dest, tableName, segment.SegmentDirectory)
		filePaths, err := connection.FindFiles(tableDestination, extensions)
		if err != nil {
			return nil, fmt.Errorf("failed to list files in destination %s: %w", dest, err)
		}

		for _, filePath := range filePaths {
			// Extract the file name from the path.
			fileName := path.Base(filePath)

			util.MapDoesNotContainKey(existingFiles, fileName,
				"duplicate file found: %s and %s", fileName, existingFiles[fileName])
			existingFiles[fileName] = dest
		}
	}

	return existingFiles, nil
}

// Push the data in a single table to the remote location(s).
func pushTable(
	logger *slog.Logger,
	tableName string,
	sources []string,
	destinations []string,
	connection *util.SSHSession,
	deleteAfterTransfer bool,
	fsync bool,
	throttleMB float64,
	threads uint64) error {

	// Figure out where data currently exists at the destination(s). We don't want this operation to cause a file
	// to exist in multiple places.
	existingFilesMap, err := mapExistingFiles(destinations, tableName, connection)
	if err != nil {
		return fmt.Errorf("failed to map existing files at destinations: %w", err)
	}

	segmentPaths, err := segment.BuildSegmentPaths(sources, "", tableName)
	if err != nil {
		return fmt.Errorf("failed to build segment paths for table %s at paths %v: %w", tableName, sources, err)
	}

	errorMonitor := util.NewErrorMonitor(context.Background(), logger, nil)

	// Gather segment files to send.
	lowestSegmentIndex, highestSegmentIndex, segments, err := segment.GatherSegmentFiles(
		logger,
		errorMonitor,
		segmentPaths,
		false,
		time.Now(),
		false,
		fsync)
	if err != nil {
		return fmt.Errorf("failed to gather segment files for table %s at paths %v: %w",
			tableName, sources, err)
	}

	if len(segments) == 0 {
		logger.Info("No segments found for table", "table", tableName)
		return nil
	}

	// Special handling if we are transferring data from a snapshot.
	isSnapshot, err := segments[lowestSegmentIndex].IsSnapshot()
	if err != nil {
		return fmt.Errorf("failed to check if segment %d is a snapshot: %w", lowestSegmentIndex, err)
	}
	if isSnapshot {
		if len(sources) > 1 {
			return fmt.Errorf("table %s is a snapshot, but source more than one source directories found: %v",
				tableName, sources)
		}

		boundaryFile, err := disktable.LoadBoundaryFile(disktable.UpperBound, path.Join(sources[0], tableName))
		if err != nil {
			return fmt.Errorf("failed to load boundary file for table %s at path %s: %w",
				tableName, sources[0], err)
		}

		if !boundaryFile.IsDefined() {
			return nil
		}
		if boundaryFile.BoundaryIndex() < highestSegmentIndex {
			highestSegmentIndex = boundaryFile.BoundaryIndex()
		}
	} else if deleteAfterTransfer {
		return fmt.Errorf("--no-gc is required when pushing a non-snapshot table")
	}

	// Ensure the remote segment directories exists.
	for _, dest := range destinations {
		segmentDir := path.Join(dest, tableName, segment.SegmentDirectory)
		err = connection.Mkdirs(segmentDir)
		if err != nil {
			return fmt.Errorf("failed to create segment directory %s at destination %s: %w",
				segmentDir, dest, err)
		}
	}

	type transfer struct {
		source string
		target string
	}
	var transfers []transfer
	publishedSegments := make(map[uint32]*segment.Segment)
	for index := uint64(lowestSegmentIndex); index <= uint64(highestSegmentIndex); index++ {
		i := uint32(index)
		seg, exists := segments[i]
		if !exists {
			return fmt.Errorf("missing published segment %d", i)
		}
		for _, filePath := range seg.GetFilePaths() {
			fileName := path.Base(filePath)
			destination, exists := existingFilesMap[fileName]
			if !exists {
				destination, err = determineDestination(fileName, destinations)
				if err != nil {
					return fmt.Errorf("failed to determine destination for file %s: %w", fileName, err)
				}
			}
			transfers = append(transfers, transfer{
				source: filePath,
				target: path.Join(destination, tableName, segment.SegmentDirectory, fileName),
			})
		}
		publishedSegments[i] = seg
	}

	rsyncLimiter := make(chan struct{}, threads)
	var rsyncs sync.WaitGroup
	defer rsyncs.Wait()
	for _, file := range transfers {
		rsyncLimiter <- struct{}{}
		rsyncs.Add(1)
		go func(file transfer) {
			defer rsyncs.Done()
			defer func() { <-rsyncLimiter }()
			if transferErr := connection.Rsync(file.source, file.target, throttleMB); transferErr != nil {
				errorMonitor.Panic(transferErr)
			}
		}(file)
	}
	rsyncs.Wait()

	if ok, err := errorMonitor.IsOk(); !ok {
		return fmt.Errorf("error detected during transfer: %w", err)
	}
	// Every file in this set has now been acknowledged by rsync. Segments
	// observed beyond the captured publication boundary remain publisher-owned.
	acknowledgedSegments := publishedSegments

	// Now that we have transferred the files, we can delete them if requested.
	if deleteAfterTransfer {
		util.True(isSnapshot, "we should have already returned an error if this is a non-snapshot table")

		err = deleteLocalSegments(acknowledgedSegments, tableName, true, sources, lowestSegmentIndex)
		if err != nil {
			return fmt.Errorf("failed to delete segments after transfer: %w", err)
		}
	}

	return nil
}

// Deletes local segments after they have been successfully transferred to the remote destination(s).
func deleteLocalSegments(
	segments map[uint32]*segment.Segment,
	tableName string,
	isSnapshot bool,
	sources []string,
	lowestSegmentIndex uint32) error {

	var boundaryFile *disktable.BoundaryFile
	first := uint64(lowestSegmentIndex)
	if isSnapshot {
		var err error
		boundaryFile, err = disktable.LoadBoundaryFile(disktable.LowerBound, path.Join(sources[0], tableName))
		if err != nil {
			return fmt.Errorf("failed to load boundary file for table %s at path %s: %w",
				tableName, sources[0], err)
		}
		if boundaryFile.IsDefined() {
			first = uint64(boundaryFile.BoundaryIndex()) + 1
		}
	}

	// Reclaim only the contiguous acknowledged prefix. Keeping later segments
	// preserves the unacknowledged gap for the next consumer invocation.
	var prefix []*segment.Segment
	for index := first; index <= uint64(^uint32(0)); index++ {
		seg, exists := segments[uint32(index)]
		if !exists {
			break
		}
		prefix = append(prefix, seg)
	}
	for _, seg := range prefix {
		seg.Release()
	}
	for _, seg := range prefix {
		if err := seg.BlockUntilFullyDeleted(); err != nil {
			return fmt.Errorf("failed to delete segment %d for table %s: %w", seg.SegmentIndex(), tableName, err)
		}
	}

	if isSnapshot && len(prefix) > 0 {
		if err := boundaryFile.Update(prefix[len(prefix)-1].SegmentIndex()); err != nil {
			return fmt.Errorf("failed to update boundary file for table %s at path %s: %w",
				tableName, sources[0], err)
		}
	}
	return nil
}
