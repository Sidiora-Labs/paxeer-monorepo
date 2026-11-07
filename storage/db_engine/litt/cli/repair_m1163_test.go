package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"math"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/disktable"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/disktable/segment"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/litt/util"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// This peer implements SSH exec with filesystem operations in-process. The
// component under test remains the production parser, push engine and SSHSession.
type repairM1163SSH struct {
	key, knownHosts string
	port            uint64
	live            atomic.Int64
	accepted        atomic.Int64
	channels        atomic.Int64
	beforeExec      func(string) error
}

func repairM1163StartSSH(t *testing.T, beforeExec func(string) error) *repairM1163SSH {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	signer, err := ssh.NewSignerFromKey(private)
	require.NoError(t, err)
	block, err := ssh.MarshalPrivateKey(private, "repair fixture")
	require.NoError(t, err)
	dir := t.TempDir()
	server := &repairM1163SSH{key: filepath.Join(dir, "identity"), knownHosts: filepath.Join(dir, "known_hosts"), beforeExec: beforeExec}
	require.NoError(t, os.WriteFile(server.key, pem.EncodeToMemory(block), 0600))
	authorized, err := ssh.NewPublicKey(public)
	require.NoError(t, err)
	config := &ssh.ServerConfig{PublicKeyCallback: func(_ ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
		if !bytes.Equal(key.Marshal(), authorized.Marshal()) {
			return nil, fmt.Errorf("unauthorized public key")
		}
		return nil, nil
	}}
	config.AddHostKey(signer)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	server.port = uint64(listener.Addr().(*net.TCPAddr).Port)
	require.NoError(t, os.WriteFile(server.knownHosts, []byte(knownhosts.Line([]string{listener.Addr().String()}, signer.PublicKey())+"\n"), 0600))
	var peers sync.WaitGroup
	var sockets sync.Map
	acceptDone := make(chan struct{})
	go func() {
		defer close(acceptDone)
		for {
			socket, err := listener.Accept()
			if err != nil {
				return
			}
			sockets.Store(socket, socket)
			peers.Add(1)
			go func() {
				defer peers.Done()
				defer sockets.Delete(socket)
				defer socket.Close()
				connection, channels, requests, err := ssh.NewServerConn(socket, config)
				if err != nil {
					return
				}
				server.live.Add(1)
				server.accepted.Add(1)
				defer server.live.Add(-1)
				defer connection.Close()
				go ssh.DiscardRequests(requests)
				var sessions sync.WaitGroup
				for next := range channels {
					if next.ChannelType() != "session" {
						_ = next.Reject(ssh.UnknownChannelType, "session required")
						continue
					}
					channel, requests, err := next.Accept()
					if err != nil {
						continue
					}
					sessions.Add(1)
					server.channels.Add(1)
					go func() {
						defer sessions.Done()
						defer server.channels.Add(-1)
						defer channel.Close()
						for request := range requests {
							if request.Type != "exec" {
								_ = request.Reply(false, nil)
								continue
							}
							var payload struct{ Command string }
							if err := ssh.Unmarshal(request.Payload, &payload); err != nil {
								_ = request.Reply(false, nil)
								return
							}
							_ = request.Reply(true, nil)
							err := server.exec(payload.Command, channel)
							status := uint32(0)
							if err != nil {
								status = 1
								_, _ = fmt.Fprintln(channel.Stderr(), err)
							}
							_, _ = channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{status}))
							return
						}
					}()
				}
				sessions.Wait()
			}()
		}
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		<-acceptDone
		sockets.Range(func(key, _ any) bool { _ = key.(net.Conn).Close(); return true })
		peers.Wait()
	})
	return server
}

func (s *repairM1163SSH) exec(command string, output io.Writer) error {
	if s.beforeExec != nil {
		if err := s.beforeExec(command); err != nil {
			return err
		}
	}
	if strings.HasPrefix(command, "find '") && strings.HasSuffix(command, "' -type f -print0") {
		root := strings.TrimSuffix(strings.TrimPrefix(command, "find '"), "' -type f -print0")
		root = strings.ReplaceAll(root, "'\"'\"'", "'")
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.Type().IsRegular() {
				_, err = fmt.Fprintf(output, "%s\x00", path)
			}
			return err
		})
		if os.IsNotExist(err) {
			return fmt.Errorf("find: '%s': No such file or directory", root)
		}
		return err
	}
	if strings.HasPrefix(command, "mkdir -p -- '") && strings.HasSuffix(command, "'") {
		root := strings.TrimSuffix(strings.TrimPrefix(command, "mkdir -p -- '"), "'")
		root = strings.ReplaceAll(root, "'\"'\"'", "'")
		return os.MkdirAll(root, 0755)
	}
	return fmt.Errorf("unsupported fixture command: %s", command)
}

func (s *repairM1163SSH) args() []string {
	return []string{"--key", s.key, "--known-hosts", s.knownHosts, "--port", strconv.FormatUint(s.port, 10), "--threads", "2", "--quiet"}
}

func (s *repairM1163SSH) assertClosed(t *testing.T) {
	t.Helper()
	require.Eventually(t, func() bool { return s.live.Load() == 0 && s.channels.Load() == 0 }, 3*time.Second, time.Millisecond,
		"push must release its SSH client after all participants finish")
}

func repairM1163Segment(t *testing.T, root, snapshot, table string, index uint32) (*segment.Segment, *segment.SegmentPath) {
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
	_, err = seg.Seal(time.Now().Add(-2 * time.Hour))
	require.NoError(t, err)
	return seg, paths[0]
}

func TestRepairM_1163_PruneUnpublishedOrphans(t *testing.T) {
	for firstLink := 0; firstLink < 4; firstLink++ {
		t.Run(fmt.Sprintf("first-link-%d", firstLink), func(t *testing.T) {
			root, snapshot := t.TempDir(), t.TempDir()
			old := repair112Produce(t, root, snapshot, "prune_table", 0, time.Now().Add(-2*time.Hour))
			upper, err := disktable.LoadBoundaryFile(disktable.UpperBound, filepath.Join(snapshot, "prune_table"))
			require.NoError(t, err)
			require.NoError(t, upper.Update(0))
			seg, publisher := repairM1163Segment(t, root, snapshot, "prune_table", 1)
			files := seg.GetFilePaths()
			require.Len(t, files, 4)
			// Start with every link type, then qualify every intermediate set.
			files[0], files[firstLink] = files[firstLink], files[0]
			var linked []string
			for _, file := range files {
				require.NoError(t, publisher.Snapshot(filepath.Base(file)))
				linked = append(linked, file)
				require.NoError(t, prune(slog.Default(), []string{snapshot}, nil, 3600, false))
				for _, published := range old {
					require.NoFileExists(t, filepath.Join(snapshot, "prune_table", segment.SegmentDirectory, filepath.Base(published)))
				}
				for _, pending := range linked {
					require.FileExists(t, filepath.Join(snapshot, "prune_table", segment.SegmentDirectory, filepath.Base(pending)))
					require.FileExists(t, filepath.Join(root, "prune_table", segment.HardLinkDirectory, filepath.Base(pending)))
				}
				lower, err := disktable.LoadBoundaryFile(disktable.LowerBound, filepath.Join(snapshot, "prune_table"))
				require.NoError(t, err)
				require.True(t, lower.IsDefined())
				require.Equal(t, uint32(0), lower.BoundaryIndex())
			}
			require.NoError(t, upper.Update(1))
			require.NoError(t, prune(slog.Default(), []string{snapshot}, nil, 3600, false))
			lower, err := disktable.LoadBoundaryFile(disktable.LowerBound, filepath.Join(snapshot, "prune_table"))
			require.NoError(t, err)
			require.Equal(t, uint32(1), lower.BoundaryIndex())
			for _, file := range files {
				require.NoFileExists(t, filepath.Join(snapshot, "prune_table", segment.SegmentDirectory, filepath.Base(file)))
				require.FileExists(t, file, "snapshot pruning must retain the database file")
			}
			repair112AssertRecovered(t, []string{root}, "prune_table", 0, 1)
		})
	}
}

func TestRepairM_1163_PruneBeforeFirstPublication(t *testing.T) {
	for firstLink := 0; firstLink < 4; firstLink++ {
		t.Run(fmt.Sprint(firstLink), func(t *testing.T) {
			root, snapshot := t.TempDir(), t.TempDir()
			seg, publisher := repairM1163Segment(t, root, snapshot, "initial_table", 0)
			file := seg.GetFilePaths()[firstLink]
			require.NoError(t, publisher.Snapshot(filepath.Base(file)))
			require.NoError(t, prune(slog.Default(), []string{snapshot}, nil, 0, false))
			require.FileExists(t, filepath.Join(snapshot, "initial_table", segment.SegmentDirectory, filepath.Base(file)))
			lower, err := disktable.LoadBoundaryFile(disktable.LowerBound, filepath.Join(snapshot, "initial_table"))
			require.NoError(t, err)
			require.False(t, lower.IsDefined())
		})
	}
}

func TestRepairM_1163_TableInfoMapOrder(t *testing.T) {
	root, snapshot := t.TempDir(), t.TempDir()
	const published = 4
	var expectedSize uint64
	var oldest, newest time.Time
	for index := 0; index <= published; index++ {
		sealTime := time.Now().Add(time.Duration(index-10) * time.Hour)
		if index == 0 {
			oldest = sealTime
		}
		if index == published-1 {
			newest = sealTime
		}
		files := repair112Produce(t, root, snapshot, "info_table", uint32(index), sealTime)
		if index < published {
			for _, file := range files {
				info, err := os.Stat(file)
				require.NoError(t, err)
				expectedSize += uint64(info.Size())
			}
		}
	}
	upper, err := disktable.LoadBoundaryFile(disktable.UpperBound, filepath.Join(snapshot, "info_table"))
	require.NoError(t, err)
	require.NoError(t, upper.Update(published-1))
	for iteration := 0; iteration < 32; iteration++ {
		info, err := tableInfo(slog.Default(), "info_table", []string{snapshot}, false)
		require.NoError(t, err)
		require.True(t, info.IsSnapshot)
		require.Equal(t, uint64(published*2), info.KeyCount)
		require.Equal(t, expectedSize, info.Size)
		require.Equal(t, uint32(0), info.LowestSegmentIndex)
		require.Equal(t, uint32(published-1), info.HighestSegmentIndex)
		require.True(t, oldest.Equal(info.OldestSegmentSealTime))
		require.True(t, newest.Equal(info.NewestSegmentSealTime))
	}
}

func TestRepairM_1163_SyncPeriodBounds(t *testing.T) {
	maxSeconds := uint64(math.MaxInt64 / int64(time.Second))
	cases := []struct {
		name, value string
		seconds     uint64
		valid       bool
	}{
		{"default", "", 300, true},
		{"zero", "0", 0, false},
		{"nondefault", "7", 7, true},
		{"maximum", strconv.FormatUint(maxSeconds, 10), maxSeconds, true},
		{"duration-overflow", strconv.FormatUint(maxSeconds+1, 10), maxSeconds + 1, false},
		{"signed-overflow", "9223372036854775808", 1 << 63, false},
		{"unsigned-maximum", "18446744073709551615", math.MaxUint64, false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			app := buildCLIParser(slog.Default())
			var observed time.Duration
			var periodErr error
			var beforeCalled bool
			for _, command := range app.Commands {
				if command.Name == "sync" {
					command.Before = func(ctx *cli.Context) error {
						beforeCalled = true
						require.Equal(t, test.seconds, ctx.Uint64("period"))
						observed, periodErr = syncPeriod(ctx)
						return nil
					}
				}
			}
			args := []string{"litt", "sync", "--src", t.TempDir(), "--dst", t.TempDir(), "--key", filepath.Join(t.TempDir(), "identity"), "--known-hosts", filepath.Join(t.TempDir(), "known_hosts")}
			if test.value != "" {
				args = append(args, "--period", test.value)
			}
			args = append(args, "fixture@127.0.0.1")
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			err := app.RunContext(ctx, args)
			require.True(t, beforeCalled, "must exercise the registered parser")
			if test.valid {
				require.NoError(t, periodErr)
				require.NoError(t, err, "real handler and ticker must accept the period")
				require.Equal(t, time.Duration(test.seconds)*time.Second, observed)
			} else {
				require.Error(t, periodErr)
				require.ErrorContains(t, err, "period must be between")
			}
		})
	}
}

func TestRepairM_1163_RebasePathFixture(t *testing.T) {
	rebaseTest(t, 2, 3, 1, false, false)
}

func TestRepairM_1163_RebaseOverlapFixture(t *testing.T) {
	t.Run("move", func(t *testing.T) { rebaseTest(t, 4, 4, 2, false, false) })
	t.Run("preserve rejected before mutation", rebasePreserveOverlapTest)
}

func TestRepairM_1163_PushConnectionClose(t *testing.T) {
	root := t.TempDir()
	server := repairM1163StartSSH(t, nil)
	engine := newSyncEngine(t.Context(), slog.Default(), []string{root}, []string{t.TempDir()}, "fixture", "127.0.0.1", server.port, server.key, server.knownHosts, false, false, 2, 0, time.Second, 0, "litt", false)
	defer engine.Stop()
	for attempt := int64(1); attempt <= 8; attempt++ {
		engine.sync()
		require.Eventually(t, func() bool { return server.accepted.Load() == attempt }, 3*time.Second, time.Millisecond)
		server.assertClosed(t)
	}
	// An authenticated connection must also close when local table listing fails.
	invalidRoot := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(invalidRoot, "invalid_table"), 0755))
	require.NoError(t, os.Symlink("segments", filepath.Join(invalidRoot, "invalid_table", "segments")))
	err := push(slog.Default(), []string{invalidRoot}, []string{t.TempDir()}, "fixture", "127.0.0.1", server.port, server.key, server.knownHosts, false, false, 2, 0, false)
	require.ErrorContains(t, err, "failed to list tables")
	server.assertClosed(t)
}

func TestRepairM_1163_PushDeclaredDestinationFlags(t *testing.T) {
	for _, flag := range []string{"--dst", "-d"} {
		t.Run(flag, func(t *testing.T) {
			root := t.TempDir()
			repair112Produce(t, root, "", "flag_table", 0, time.Now())
			destinations := []string{filepath.Join(t.TempDir(), "first"), filepath.Join(t.TempDir(), "second")}
			var mu sync.Mutex
			var queried []string
			server := repairM1163StartSSH(t, func(command string) error {
				mu.Lock()
				queried = append(queried, command)
				mu.Unlock()
				return nil
			})
			args := []string{"litt", "push", "--src", root, flag, destinations[0], flag, destinations[1]}
			args = append(args, server.args()...)
			args = append(args, "fixture@127.0.0.1")
			err := buildCLIParser(slog.Default()).Run(args)
			// The real handler reaches both exact remote destinations before
			// rejecting destructive transfer of a non-snapshot database.
			require.ErrorContains(t, err, "--no-gc is required")
			server.assertClosed(t)
			mu.Lock()
			require.Equal(t, []string{
				fmt.Sprintf("find '%s' -type f -print0", filepath.Join(destinations[0], "flag_table", segment.SegmentDirectory)),
				fmt.Sprintf("find '%s' -type f -print0", filepath.Join(destinations[1], "flag_table", segment.SegmentDirectory)),
			}, queried)
			mu.Unlock()
			repair112AssertRecovered(t, []string{root}, "flag_table", 0)
		})
	}
}

type repairM1163Log struct {
	mu sync.Mutex
	bytes.Buffer
}

func (b *repairM1163Log) Write(data []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.Buffer.Write(data)
}

func TestRepairM_1163_PushInterruptedParticipants(t *testing.T) {
	pushInterruptedParticipantsTest(t)
}
