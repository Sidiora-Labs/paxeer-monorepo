package tasks

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"golang.org/x/sys/unix"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/sei-protocol/seictl/sidecar/engine"
	seis3 "github.com/sei-protocol/seictl/sidecar/s3"
	"github.com/sei-protocol/seilog"
)

var restoreLog = seilog.NewLogger("seictl", "task", "snapshot-restore")

const restoreMarkerFile = ".sei-sidecar-snapshot-done"

// SnapshotHeightFile records the snapshot height the node was restored from.
// The result-export task uses this to know where to start exporting.
const SnapshotHeightFile = ".sei-sidecar-snapshot-height"

// snapshotHeightRe extracts the block height from S3 snapshot keys of the form
// <chainID>/state-sync/<height>.tar.gz. The leading "/" anchor prevents the
// regex from picking up trailing digits embedded in other path segments.
var snapshotHeightRe = regexp.MustCompile(`/(\d+)\.tar\.gz$`)

// SnapshotRestoreRequest holds the typed parameters for the snapshot-restore task.
// S3 bucket, region, and chain prefix are derived from the sidecar's environment.
// TargetHeight, when set, selects the highest available snapshot <= that height.
// When zero, the latest snapshot (from latest.txt) is used.
type SnapshotRestoreRequest struct {
	TargetHeight int64 `json:"targetHeight,omitempty"`
}

// SnapshotRestorer downloads and extracts a snapshot archive from S3.
type SnapshotRestorer struct {
	homeDir       string
	bucket        string
	region        string
	chainID       string
	clientFactory seis3.TransferClientFactory
	listerFactory seis3.ObjectListerFactory
}

// NewSnapshotRestorer creates a restorer targeting the given home directory.
// Bucket, region, and chainID are read from environment at construction time.
func NewSnapshotRestorer(homeDir, bucket, region, chainID string, clientFactory seis3.TransferClientFactory, listerFactory seis3.ObjectListerFactory) (*SnapshotRestorer, error) {
	if bucket == "" || region == "" || chainID == "" {
		return nil, fmt.Errorf("snapshot-restore: bucket, region, and chainID are required")
	}
	if clientFactory == nil {
		clientFactory = seis3.DefaultTransferClientFactory
	}
	if listerFactory == nil {
		listerFactory = seis3.DefaultObjectListerFactory
	}
	return &SnapshotRestorer{
		homeDir:       homeDir,
		bucket:        bucket,
		region:        region,
		chainID:       chainID,
		clientFactory: clientFactory,
		listerFactory: listerFactory,
	}, nil
}

// Handler returns an engine.TaskHandler for the snapshot-restore task.
func (r *SnapshotRestorer) Handler() engine.TaskHandler {
	return engine.TypedHandler(func(ctx context.Context, req SnapshotRestoreRequest) error {
		return r.Restore(ctx, req.TargetHeight)
	})
}

// Restore downloads and extracts the snapshot, skipping if the marker file exists.
// It lists objects under the chain's state-sync prefix and picks the highest
// snapshot height; when targetHeight > 0, the search is capped at that height.
func (r *SnapshotRestorer) Restore(ctx context.Context, targetHeight int64) error {
	if markerExists(r.homeDir, restoreMarkerFile) {
		restoreLog.Debug("already completed, skipping")
		return nil
	}

	if targetHeight < 0 {
		return fmt.Errorf("snapshot-restore: targetHeight must be >= 0, got %d", targetHeight)
	}

	prefix := r.chainID + "/state-sync/"

	client, err := r.clientFactory(ctx, r.region)
	if err != nil {
		return fmt.Errorf("building S3 transfer client: %w", err)
	}

	lister, err := r.listerFactory(ctx, r.region)
	if err != nil {
		return fmt.Errorf("building S3 lister: %w", err)
	}

	snapshotKey, err := resolveKeyForHeight(ctx, lister, r.bucket, prefix, r.region, targetHeight)
	if err != nil {
		return err
	}

	if snapshotKey == "" {
		return fmt.Errorf("snapshot-restore: resolved snapshot key is empty for %s in s3://%s/%s", r.chainID, r.bucket, prefix)
	}

	tmpDir := filepath.Join(r.homeDir, "tmp")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}

	tmpFile, err := os.CreateTemp(tmpDir, "snapshot-*.tar.gz")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	restoreLog.Info("downloading snapshot", "bucket", r.bucket, "key", snapshotKey, "dest", tmpPath)
	_, err = client.DownloadObject(ctx, &transfermanager.DownloadObjectInput{
		Bucket:   aws.String(r.bucket),
		Key:      aws.String(snapshotKey),
		WriterAt: boundedSnapshotWriter{file: tmpFile},
	})
	_ = tmpFile.Close()
	if err != nil {
		return seis3.ClassifyS3Error("snapshot-restore", r.bucket, snapshotKey, r.region, err)
	}

	destDir := filepath.Join(r.homeDir, "data", "snapshots")
	restoreLog.Info("extracting archive", "dest", destDir)
	if err := extractArchive(ctx, tmpPath, destDir); err != nil {
		return fmt.Errorf("extracting snapshot: %w", err)
	}

	if h := parseHeightFromKey(snapshotKey); h > 0 {
		if err := os.WriteFile(
			filepath.Join(r.homeDir, SnapshotHeightFile),
			[]byte(strconv.FormatInt(h, 10)),
			0o644,
		); err != nil {
			restoreLog.Warn("failed to write snapshot height file", "err", err)
		}
	}

	restoreLog.Info("restore complete")
	return writeMarker(r.homeDir, restoreMarkerFile)
}

// resolveKeyForHeight lists snapshot objects under prefix and returns the key
// with the highest parsed height. When targetHeight > 0 it caps the search at
// that height; targetHeight == 0 picks the highest available snapshot.
func resolveKeyForHeight(ctx context.Context, lister seis3.ObjectLister, bucket, prefix, region string, targetHeight int64) (string, error) {
	var bestHeight int64
	var bestKey string

	var continuationToken *string
	for {
		output, err := lister.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:            aws.String(bucket),
			Prefix:            aws.String(prefix),
			ContinuationToken: continuationToken,
		})
		if err != nil {
			return "", seis3.ClassifyS3Error("snapshot-restore", bucket, prefix, region, err)
		}

		for _, obj := range output.Contents {
			if obj.Key == nil {
				continue
			}
			h := parseHeightFromKey(*obj.Key)
			if h <= 0 {
				continue
			}
			if targetHeight > 0 && h > targetHeight {
				continue
			}
			if h > bestHeight {
				bestHeight = h
				bestKey = *obj.Key
			}
		}

		if !aws.ToBool(output.IsTruncated) {
			break
		}
		continuationToken = output.NextContinuationToken
	}

	if bestKey == "" {
		if targetHeight > 0 {
			return "", fmt.Errorf("no snapshot found at or below height %d in s3://%s/%s", targetHeight, bucket, prefix)
		}
		return "", fmt.Errorf("no snapshots found in s3://%s/%s", bucket, prefix)
	}

	restoreLog.Info("resolved snapshot",
		"targetHeight", targetHeight, "snapshotHeight", bestHeight, "key", bestKey)
	return bestKey, nil
}

func parseHeightFromKey(key string) int64 {
	m := snapshotHeightRe.FindStringSubmatch(key)
	if len(m) < 2 {
		return 0
	}
	h, err := strconv.ParseInt(m[1], 10, 64)
	if err != nil {
		return 0
	}
	return h
}

// extractArchive opens a .tar.gz file and extracts it to destDir.
func extractArchive(ctx context.Context, archivePath, destDir string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("opening archive: %w", err)
	}
	defer func() { _ = f.Close() }()

	return extractTarStream(ctx, f, destDir)
}

const (
	maxSnapshotCompressedBytes int64 = 64 << 30
	maxSnapshotFileBytes       int64 = 64 << 30
	maxSnapshotExpandedBytes   int64 = 256 << 30
	maxSnapshotEntries               = 1_000_000
)

type boundedSnapshotWriter struct{ file *os.File }

func (w boundedSnapshotWriter) WriteAt(p []byte, off int64) (int, error) {
	if off < 0 || off > maxSnapshotCompressedBytes || int64(len(p)) > maxSnapshotCompressedBytes-off {
		return 0, fmt.Errorf("compressed snapshot exceeds size limit")
	}
	return w.file.WriteAt(p, off)
}

// An archive is fully validated in a fresh sibling directory before adoption.
func extractTarStream(ctx context.Context, r io.Reader, destDir string) error {
	parent, err := openConfinedDir(filepath.Dir(destDir), true)
	if err != nil {
		return err
	}
	defer parent.Close()
	name := filepath.Base(destDir)
	if !plainFilename(name) {
		return fmt.Errorf("invalid snapshot destination")
	}
	var existing unix.Stat_t
	err = unix.Fstatat(int(parent.Fd()), name, &existing, unix.AT_SYMLINK_NOFOLLOW)
	hasDest := err == nil
	if err != nil && err != unix.ENOENT {
		return err
	}
	if hasDest && existing.Mode&unix.S_IFMT != unix.S_IFDIR {
		return fmt.Errorf("snapshot destination must be a directory, not a symlink")
	}
	stage := fmt.Sprintf(".snapshot-%x", randomFileID())
	if err := unix.Mkdirat(int(parent.Fd()), stage, 0700); err != nil {
		return err
	}
	stagePath := filepath.Join(filepath.Dir(destDir), stage)
	defer os.RemoveAll(stagePath)
	root, err := openConfinedDir(stagePath, false)
	if err != nil {
		return err
	}
	defer root.Close()
	if err := extractConfinedSnapshot(ctx, r, root); err != nil {
		return err
	}
	if err := root.Sync(); err != nil {
		return err
	}
	if hasDest {
		if err := unix.Renameat2(int(parent.Fd()), stage, int(parent.Fd()), name, unix.RENAME_EXCHANGE); err != nil {
			return fmt.Errorf("atomically adopting snapshot: %w", err)
		}
	} else if err := unix.Renameat2(int(parent.Fd()), stage, int(parent.Fd()), name, unix.RENAME_NOREPLACE); err != nil {
		return err
	}
	return parent.Sync()
}

func extractConfinedSnapshot(ctx context.Context, r io.Reader, root *os.File) error {
	gzr, err := gzip.NewReader(io.LimitReader(r, maxSnapshotCompressedBytes+1))
	if err != nil {
		return err
	}
	defer gzr.Close()
	tr := tar.NewReader(gzr)
	var expanded int64
	entries := 0
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		header, err := tr.Next()
		if err == io.EOF {
			// Consume the trailer too, so a corrupt gzip checksum cannot be adopted.
			remaining, err := io.Copy(io.Discard, io.LimitReader(gzr, 1<<20))
			if err != nil {
				return err
			}
			if remaining > 0 {
				return fmt.Errorf("unexpected content after tar archive")
			}
			return nil
		}
		if err != nil {
			return err
		}
		entries++
		if entries > maxSnapshotEntries {
			return fmt.Errorf("snapshot has too many entries")
		}
		if header.Size < 0 || header.Size > maxSnapshotFileBytes || header.Size > maxSnapshotExpandedBytes-expanded {
			return fmt.Errorf("snapshot exceeds expanded size limit")
		}
		expanded += header.Size
		if header.Typeflag != tar.TypeDir && header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA {
			return fmt.Errorf("snapshot links and special files are forbidden: %q", header.Name)
		}
		if filepath.IsAbs(header.Name) || strings.ContainsAny(header.Name, "\\\x00") {
			return fmt.Errorf("invalid archive path %q", header.Name)
		}
		parts := strings.Split(header.Name, "/")
		for _, part := range parts {
			if part == ".." {
				return fmt.Errorf("archive path traverses parent")
			}
		}
		clean := filepath.Clean(header.Name)
		if clean == "." {
			if header.Typeflag == tar.TypeDir {
				continue
			}
			return fmt.Errorf("invalid archive file")
		}
		if !filepath.IsLocal(clean) {
			return fmt.Errorf("archive path escapes destination")
		}
		dir, err := snapshotParent(root, filepath.Dir(clean))
		if err != nil {
			return err
		}
		name := filepath.Base(clean)
		if header.Typeflag == tar.TypeDir {
			err = unix.Mkdirat(int(dir.Fd()), name, 0700)
			if err == unix.EEXIST {
				fd, e := unix.Openat(int(dir.Fd()), name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
				err = e
				if e == nil {
					unix.Close(fd)
				}
			}
		} else {
			err = extractConfinedFile(ctx, tr, dir, name, header.Size)
		}
		dir.Close()
		if err != nil {
			return err
		}
	}
}

func snapshotParent(root *os.File, path string) (*os.File, error) {
	fd, err := unix.Dup(int(root.Fd()))
	if err != nil {
		return nil, err
	}
	if path != "." {
		for _, part := range strings.Split(path, string(filepath.Separator)) {
			if !plainFilename(part) {
				unix.Close(fd)
				return nil, fmt.Errorf("invalid directory component")
			}
			if err := unix.Mkdirat(fd, part, 0700); err != nil && err != unix.EEXIST {
				unix.Close(fd)
				return nil, err
			}
			next, err := unix.Openat(fd, part, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
			unix.Close(fd)
			if err != nil {
				return nil, err
			}
			fd = next
		}
	}
	return os.NewFile(uintptr(fd), path), nil
}

func extractConfinedFile(ctx context.Context, r io.Reader, dir *os.File, name string, size int64) error {
	fd, err := unix.Openat(int(dir.Fd()), name, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0600)
	if err != nil {
		return err
	}
	f := os.NewFile(uintptr(fd), name)
	defer f.Close()
	buf := make([]byte, 64<<10)
	for size > 0 {
		if err := ctx.Err(); err != nil {
			return err
		}
		n := int64(len(buf))
		if size < n {
			n = size
		}
		got, err := io.ReadFull(r, buf[:n])
		if err != nil {
			return err
		}
		if _, err := f.Write(buf[:got]); err != nil {
			return err
		}
		size -= int64(got)
	}
	return f.Sync()
}

// isInsideDir checks that target is within or equal to baseDir.
func isInsideDir(target, baseDir string) bool {
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return false
	}
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return false
	}
	if absTarget == absBase {
		return true
	}
	rel, err := filepath.Rel(absBase, absTarget)
	if err != nil {
		return false
	}
	return len(rel) > 0 && rel[0] != '.'
}

func markerExists(homeDir, name string) bool {
	_, err := os.Stat(filepath.Join(homeDir, name))
	return err == nil
}

func writeMarker(homeDir, name string) error {
	path := filepath.Join(homeDir, name)
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("writing marker file %s: %w", path, err)
	}
	return f.Close()
}
