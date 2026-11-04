package main

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/disktable"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/disktable/segment"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/littbuilder"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/util"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// The SSH subprocess uses the actual CLI parser and handlers in this test
// executable, avoiding a second build or any replacement of the prune handler.
func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == "--repair-112-cli" {
		if err := buildCLIParser(slog.Default()).Run(append([]string{"litt"}, os.Args[2:]...)); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

type repair112SSH struct {
	user, key, knownHosts string
	port                  uint64
}

func repair112StartSSH(t *testing.T) repair112SSH {
	t.Helper()
	sshd, err := exec.LookPath("sshd")
	require.NoError(t, err, "the real SSH integration tests require openssh-server")
	_, err = exec.LookPath("rsync")
	require.NoError(t, err, "the real transfer tests require rsync")
	dir := t.TempDir()
	key := filepath.Join(dir, "identity")
	hostKey := filepath.Join(dir, "host")
	for _, file := range []string{key, hostKey} {
		output, err := exec.Command("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", file).CombinedOutput()
		require.NoError(t, err, "%s", output)
	}
	currentUser, err := user.Current()
	require.NoError(t, err)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	require.NoError(t, listener.Close())
	config := fmt.Sprintf(`ListenAddress 127.0.0.1
Port %d
HostKey %s
AuthorizedKeysFile %s
PidFile %s
StrictModes no
UsePAM no
PermitRootLogin yes
PasswordAuthentication no
KbdInteractiveAuthentication no
AuthenticationMethods publickey
LogLevel ERROR
`, port, hostKey, key+".pub", filepath.Join(dir, "sshd.pid"))
	configPath := filepath.Join(dir, "sshd_config")
	require.NoError(t, os.WriteFile(configPath, []byte(config), 0600))
	if os.Geteuid() == 0 {
		require.NoError(t, os.MkdirAll("/run/sshd", 0755))
	}
	logPath := filepath.Join(dir, "sshd.log")
	logFile, err := os.Create(logPath)
	require.NoError(t, err)
	server := exec.Command(sshd, "-D", "-e", "-f", configPath)
	server.Stdout, server.Stderr = logFile, logFile
	require.NoError(t, server.Start())
	t.Cleanup(func() {
		_ = server.Process.Kill()
		_ = server.Wait()
		_ = logFile.Close()
		if t.Failed() {
			data, _ := os.ReadFile(logPath)
			t.Logf("sshd: %s", data)
		}
	})
	address := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	require.Eventually(t, func() bool {
		conn, err := net.DialTimeout("tcp", address, 100*time.Millisecond)
		if err != nil {
			return false
		}
		_ = conn.Close()
		return true
	}, 5*time.Second, 20*time.Millisecond)
	publicData, err := os.ReadFile(hostKey + ".pub")
	require.NoError(t, err)
	publicKey, _, _, _, err := ssh.ParseAuthorizedKey(publicData)
	require.NoError(t, err)
	knownHosts := filepath.Join(dir, "known_hosts")
	require.NoError(t, os.WriteFile(knownHosts, []byte(knownhosts.Line([]string{address}, publicKey)+"\n"), 0600))
	return repair112SSH{user: currentUser.Username, key: key, knownHosts: knownHosts, port: uint64(port)}
}

func (s repair112SSH) args() []string {
	return []string{"--key", s.key, "--known-hosts", s.knownHosts, "--port", strconv.FormatUint(s.port, 10), "--threads", "2"}
}

func (s repair112SSH) connect(t *testing.T) *util.SSHSession {
	t.Helper()
	connection, err := util.NewSSHSession(slog.Default(), s.user, "127.0.0.1", s.port, s.key, s.knownHosts, false)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, connection.Close()) })
	return connection
}

func repair112Produce(t *testing.T, root, snapshot, table string, index uint32, sealTime time.Time) []string {
	t.Helper()
	paths, err := segment.BuildSegmentPaths([]string{root}, snapshot, table)
	require.NoError(t, err)
	require.NoError(t, paths[0].MakeDirectories(false))
	seg, err := segment.CreateSegment(slog.Default(), util.NewErrorMonitor(t.Context(), slog.Default(), nil), index, paths, snapshot != "", 2, false)
	require.NoError(t, err)
	for shard := 0; shard < 2; shard++ {
		_, _, err = seg.Write(&types.PutRequest{Key: []byte(fmt.Sprintf("key-%d-%d", index, shard)), Value: bytes.Repeat([]byte{byte(index + 1)}, 128)})
		require.NoError(t, err)
	}
	_, err = seg.Seal(sealTime)
	require.NoError(t, err)
	if snapshot != "" {
		require.NoError(t, seg.Snapshot())
	}
	return seg.GetFilePaths()
}

func repair112AssertRecovered(t *testing.T, roots []string, tableName string, indexes ...uint32) {
	t.Helper()
	config, err := litt.DefaultConfig(roots...)
	require.NoError(t, err)
	config.Fsync = false
	config.DoubleWriteProtection = true
	for reopen := 0; reopen < 2; reopen++ {
		db, err := littbuilder.NewDB(config)
		require.NoError(t, err)
		table, err := db.BuildTable(litt.DefaultTableConfig(tableName))
		require.NoError(t, err)
		for _, index := range indexes {
			for shard := 0; shard < 2; shard++ {
				actual, exists, err := table.Get([]byte(fmt.Sprintf("key-%d-%d", index, shard)))
				require.NoError(t, err)
				require.True(t, exists)
				require.Equal(t, bytes.Repeat([]byte{byte(index + 1)}, 128), actual)
			}
		}
		require.NoError(t, db.Close())
	}
}

func repair112CheckTransfers(t *testing.T, files, destinations []string, table string) {
	t.Helper()
	for _, file := range files {
		name := filepath.Base(file)
		destination, err := determineDestination(name, destinations)
		require.NoError(t, err)
		expected, err := os.ReadFile(file)
		require.NoError(t, err)
		for _, root := range destinations {
			target := filepath.Join(root, table, segment.SegmentDirectory, name)
			if root == destination {
				actual, err := os.ReadFile(target)
				require.NoError(t, err)
				require.Equal(t, expected, actual)
			} else {
				require.NoFileExists(t, target)
			}
		}
	}
}

func TestRepair_112_PushDestinationFlag(t *testing.T) {
	server := repair112StartSSH(t)
	for _, flag := range []string{"--dst", "-d"} {
		t.Run(flag, func(t *testing.T) {
			root := t.TempDir()
			files := repair112Produce(t, root, "", "push_table", 0, time.Now())
			destinations := []string{filepath.Join(t.TempDir(), "first"), filepath.Join(t.TempDir(), "second")}
			args := []string{"litt", "push", "--src", root, flag, destinations[0], flag, destinations[1], "--no-gc"}
			args = append(args, server.args()...)
			args = append(args, server.user+"@127.0.0.1")
			require.NoError(t, buildCLIParser(slog.Default()).Run(args))
			repair112CheckTransfers(t, files, destinations, "push_table")
			repair112AssertRecovered(t, destinations, "push_table", 0)
		})
	}
}

func TestRepair_112_SyncDestinationFlag(t *testing.T) {
	server := repair112StartSSH(t)
	root := t.TempDir()
	oldFiles := repair112Produce(t, root, "", "sync_table", 0, time.Now().Add(-2*time.Hour))
	freshFiles := repair112Produce(t, root, "", "sync_table", 1, time.Now())
	destinations := []string{filepath.Join(t.TempDir(), "first"), filepath.Join(t.TempDir(), "second")}
	binary := repair112RemoteCLI(t)
	args := []string{"litt", "sync", "--src", root, "--dst", destinations[0], "--dst", destinations[1], "--no-gc", "--period", "1", "--max-age", "3600", "--litt-binary", binary}
	args = append(args, server.args()...)
	args = append(args, server.user+"@127.0.0.1")
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- buildCLIParser(slog.Default()).RunContext(ctx, args) }()
	require.Eventually(t, func() bool {
		for _, file := range freshFiles {
			destination, err := determineDestination(filepath.Base(file), destinations)
			if err != nil {
				return false
			}
			if _, err := os.Stat(filepath.Join(destination, "sync_table", segment.SegmentDirectory, filepath.Base(file))); err != nil {
				return false
			}
		}
		for _, file := range oldFiles {
			for _, destination := range destinations {
				if _, err := os.Stat(filepath.Join(destination, "sync_table", segment.SegmentDirectory, filepath.Base(file))); !os.IsNotExist(err) {
					return false
				}
			}
		}
		return true
	}, 15*time.Second, 30*time.Millisecond)
	cancel()
	require.NoError(t, <-done)
	repair112CheckTransfers(t, freshFiles, destinations, "sync_table")
	repair112AssertRecovered(t, destinations, "sync_table", 1)
}

func TestRepair_112_SnapshotReclaimRange(t *testing.T) {
	server := repair112StartSSH(t)
	root, snapshot := t.TempDir(), t.TempDir()
	first := repair112Produce(t, root, snapshot, "snapshot_table", 0, time.Now())
	upper, err := disktable.LoadBoundaryFile(disktable.UpperBound, filepath.Join(snapshot, "snapshot_table"))
	require.NoError(t, err)
	require.NoError(t, upper.Update(0))
	second := repair112Produce(t, root, snapshot, "snapshot_table", 1, time.Now())
	publish, published := make(chan struct{}), make(chan error, 1)
	// Pause the real producer between Snapshot and publication, exactly where
	// the consumer can see a complete but unpublished segment.
	go func() {
		<-publish
		published <- upper.Update(1)
	}()
	defer func() {
		select {
		case <-publish:
		default:
			close(publish)
		}
	}()
	destination := filepath.Join(t.TempDir(), "backup")
	args := []string{"litt", "push", "--src", snapshot, "--dst", destination}
	args = append(args, server.args()...)
	args = append(args, server.user+"@127.0.0.1")
	require.NoError(t, buildCLIParser(slog.Default()).Run(args))
	repair112CheckTransfers(t, first, []string{destination}, "snapshot_table")
	for _, file := range first {
		require.NoFileExists(t, filepath.Join(snapshot, "snapshot_table", segment.SegmentDirectory, filepath.Base(file)))
	}
	for _, file := range second {
		require.FileExists(t, filepath.Join(snapshot, "snapshot_table", segment.SegmentDirectory, filepath.Base(file)))
		require.FileExists(t, filepath.Join(root, "snapshot_table", segment.HardLinkDirectory, filepath.Base(file)))
		require.NoFileExists(t, filepath.Join(destination, "snapshot_table", segment.SegmentDirectory, filepath.Base(file)))
	}
	lower, err := disktable.LoadBoundaryFile(disktable.LowerBound, filepath.Join(snapshot, "snapshot_table"))
	require.NoError(t, err)
	require.True(t, lower.IsDefined())
	require.Equal(t, uint32(0), lower.BoundaryIndex())
	close(publish)
	require.NoError(t, <-published)

	// Restart the database producer: recovery must honour the acknowledged
	// lower bound while retaining and republishing the next segment.
	config, err := litt.DefaultConfig(root)
	require.NoError(t, err)
	config.Fsync, config.DoubleWriteProtection, config.SnapshotDirectory = false, true, snapshot
	db, err := littbuilder.NewDB(config)
	require.NoError(t, err)
	_, err = db.BuildTable(litt.DefaultTableConfig("snapshot_table"))
	require.NoError(t, err)
	require.NoError(t, db.Close())
	for _, file := range first {
		require.NoFileExists(t, filepath.Join(snapshot, "snapshot_table", segment.SegmentDirectory, filepath.Base(file)))
	}
	for _, file := range second {
		require.FileExists(t, filepath.Join(snapshot, "snapshot_table", segment.SegmentDirectory, filepath.Base(file)))
	}

	// A fresh parser and SSH connection resume the snapshot consumer.
	require.NoError(t, buildCLIParser(slog.Default()).Run(args))
	repair112CheckTransfers(t, second, []string{destination}, "snapshot_table")
	lower, err = disktable.LoadBoundaryFile(disktable.LowerBound, filepath.Join(snapshot, "snapshot_table"))
	require.NoError(t, err)
	require.Equal(t, uint32(1), lower.BoundaryIndex())
	repair112AssertRecovered(t, []string{destination}, "snapshot_table", 0, 1)
}

func TestRepair_112_SnapshotReclaimPrefix(t *testing.T) {
	root, snapshot := t.TempDir(), t.TempDir()
	files := make([][]string, 3)
	for index := range files {
		files[index] = repair112Produce(t, root, snapshot, "prefix_table", uint32(index), time.Now())
	}
	paths, err := segment.BuildSegmentPaths([]string{snapshot}, "", "prefix_table")
	require.NoError(t, err)
	_, _, segments, err := segment.GatherSegmentFiles(slog.Default(), util.NewErrorMonitor(t.Context(), slog.Default(), nil), paths, false, time.Now(), false, false)
	require.NoError(t, err)
	require.NoError(t, deleteLocalSegments(map[uint32]*segment.Segment{0: segments[0], 2: segments[2]}, "prefix_table", true, []string{snapshot}, 0))
	lower, err := disktable.LoadBoundaryFile(disktable.LowerBound, filepath.Join(snapshot, "prefix_table"))
	require.NoError(t, err)
	require.Equal(t, uint32(0), lower.BoundaryIndex())
	for index, paths := range files {
		for _, file := range paths {
			link := filepath.Join(snapshot, "prefix_table", segment.SegmentDirectory, filepath.Base(file))
			if index == 0 {
				require.NoFileExists(t, link)
			} else {
				require.FileExists(t, link)
			}
		}
	}
	require.NoError(t, deleteLocalSegments(map[uint32]*segment.Segment{1: segments[1], 2: segments[2]}, "prefix_table", true, []string{snapshot}, 1))
	lower, err = disktable.LoadBoundaryFile(disktable.LowerBound, filepath.Join(snapshot, "prefix_table"))
	require.NoError(t, err)
	require.Equal(t, uint32(2), lower.BoundaryIndex())
}

func repair112RemoteCLI(t *testing.T) string {
	t.Helper()
	executable, err := os.Executable()
	require.NoError(t, err)
	binary := filepath.Join(t.TempDir(), "litt binary's entry")
	wrapper := "#!/bin/sh\nexec " + quoteRemoteArgument(executable) + " --repair-112-cli \"$@\"\n"
	require.NoError(t, os.WriteFile(binary, []byte(wrapper), 0700))
	return binary
}

func TestRepair_112_RemotePruneSources(t *testing.T) {
	server := repair112StartSSH(t)
	connection := server.connect(t)
	parent := t.TempDir()
	destinations := []string{filepath.Join(parent, "backup one"), filepath.Join(parent, "backup' two;$(printf injected)")}
	var oldFiles, freshFiles [][]string
	for index, destination := range destinations {
		table := fmt.Sprintf("prune_%d", index)
		paths, err := segment.BuildSegmentPaths(destinations, "", table)
		require.NoError(t, err)
		for _, segmentPath := range paths {
			require.NoError(t, segmentPath.MakeDirectories(false))
		}
		oldFiles = append(oldFiles, repair112Produce(t, destination, "", table, 0, time.Now().Add(-2*time.Hour)))
		freshFiles = append(freshFiles, repair112Produce(t, destination, "", table, 1, time.Now()))
	}
	engine := newSyncEngine(t.Context(), slog.Default(), nil, destinations, server.user, "127.0.0.1", server.port, server.key, server.knownHosts, false, false, 2, 0, time.Second, 3600, repair112RemoteCLI(t), false)
	defer engine.Stop()
	stdout, stderr, err := connection.Exec(engine.pruneCommand())
	require.NoError(t, err, "stdout=%s stderr=%s", stdout, stderr)
	for index, destination := range destinations {
		for _, file := range oldFiles[index] {
			require.NoFileExists(t, file)
		}
		for _, file := range freshFiles[index] {
			require.FileExists(t, file)
		}
		repair112AssertRecovered(t, []string{destination}, fmt.Sprintf("prune_%d", index), 1)
	}
}

func TestRepair_112_SnapshotUnpublished(t *testing.T) {
	server := repair112StartSSH(t)
	root, snapshot := t.TempDir(), t.TempDir()
	files := repair112Produce(t, root, snapshot, "unpublished_table", 0, time.Now())
	destination := filepath.Join(t.TempDir(), "backup")
	args := []string{"litt", "push", "--src", snapshot, "--dst", destination}
	args = append(args, server.args()...)
	args = append(args, server.user+"@127.0.0.1")
	require.NoError(t, buildCLIParser(slog.Default()).Run(args))
	lower, err := disktable.LoadBoundaryFile(disktable.LowerBound, filepath.Join(snapshot, "unpublished_table"))
	require.NoError(t, err)
	require.False(t, lower.IsDefined())
	for _, file := range files {
		require.FileExists(t, filepath.Join(snapshot, "unpublished_table", segment.SegmentDirectory, filepath.Base(file)))
		require.NoFileExists(t, filepath.Join(destination, "unpublished_table", segment.SegmentDirectory, filepath.Base(file)))
	}
	upper, err := disktable.LoadBoundaryFile(disktable.UpperBound, filepath.Join(snapshot, "unpublished_table"))
	require.NoError(t, err)
	require.NoError(t, upper.Update(0))
	require.NoError(t, buildCLIParser(slog.Default()).Run(args))
	repair112CheckTransfers(t, files, []string{destination}, "unpublished_table")
	repair112AssertRecovered(t, []string{destination}, "unpublished_table", 0)
}
