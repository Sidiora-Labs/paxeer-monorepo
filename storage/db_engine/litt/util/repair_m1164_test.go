package util

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func TestRepairM_1164_CacheMetricSeriesCollision(t *testing.T) {
	previous := otel.GetMeterProvider()
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	otel.SetMeterProvider(provider)
	t.Cleanup(func() { require.NoError(t, provider.Shutdown(context.Background())); otel.SetMeterProvider(previous) })
	weight := func(_ string, value string) uint64 { return uint64(len(value)) }
	shared := NewCacheMetrics("chunk_read")
	tableA := NewFIFOCache[string, string](2, weight, shared)
	tableB := NewFIFOCache[string, string](10, weight, shared)
	// A separately constructed reporter must aggregate with the shared one too.
	tableC := NewFIFOCache[string, string](10, weight, NewCacheMetrics("chunk_read"))
	write := NewFIFOCache[string, string](10, weight, NewCacheMetrics("chunk_write"))
	tableA.Put("a", "aa")
	tableB.Put("b", "bbb")
	tableB.Put("c", "cccc")
	tableC.Put("d", "d")
	write.Put("w", "wwwww")
	check := func(readCount, readWeight int64) {
		t.Helper()
		var data metricdata.ResourceMetrics
		require.NoError(t, reader.Collect(context.Background(), &data))
		got := map[string]map[string]int64{}
		for _, scope := range data.ScopeMetrics {
			for _, m := range scope.Metrics {
				if m.Name != "litt_chunk_cache_key_count" && m.Name != "litt_chunk_cache_weight_bytes" {
					continue
				}
				sum, ok := m.Data.(metricdata.Sum[int64])
				require.True(t, ok)
				require.False(t, sum.IsMonotonic)
				got[m.Name] = map[string]int64{}
				for _, point := range sum.DataPoints {
					attr, found := point.Attributes.Value("cache")
					require.True(t, found)
					got[m.Name][attr.AsString()] = point.Value
				}
			}
		}
		require.Equal(t, map[string]int64{"chunk_read": readCount, "chunk_write": 1}, got["litt_chunk_cache_key_count"])
		require.Equal(t, map[string]int64{"chunk_read": readWeight, "chunk_write": 5}, got["litt_chunk_cache_weight_bytes"])
	}
	check(4, 10)
	tableB.Put("b", "b") // replacement reduces weight without changing count
	check(4, 8)
	tableA.Put("next", "x") // evicts aa
	check(4, 7)
	tableB.SetMaxWeight(0) // resizing must subtract occupancy too
	check(2, 2)
	tableA.SetMaxWeight(0)
	tableC.SetMaxWeight(0)
	check(0, 0)
	runtime.KeepAlive([]any{tableA, tableB, tableC, write})
}

func TestRepairM_1164_FlushCoordinatorShutdown(t *testing.T) {
	for iteration := 0; iteration < 20; iteration++ {
		monitor := NewErrorMonitor(t.Context(), slog.Default(), nil)
		path := filepath.Join(t.TempDir(), "drained")
		file, err := os.Create(path)
		require.NoError(t, err)
		accepted := make(chan []byte, 3)
		drained, joined, release := make(chan struct{}), make(chan struct{}), make(chan struct{})
		workerErrors := make(chan error, 1)
		require.NoError(t, monitor.Go(func() {
			defer close(joined)
			var writeErr error
			for value := range accepted {
				if _, err := file.Write(value); err != nil {
					writeErr = err
				}
			}
			if err := file.Sync(); err != nil {
				writeErr = err
			}
			if err := file.Close(); err != nil {
				writeErr = err
			}
			workerErrors <- writeErr
			close(drained)
			<-monitor.ImmediateShutdownRequired()
			<-release
		}))
		expected := []byte(fmt.Sprintf("accepted-%d", iteration))
		accepted <- expected
		monitor.Shutdown()
		ok, err := monitor.IsOk()
		require.False(t, ok)
		require.Error(t, err)
		repair1164Wait(t, monitor.DrainRequired())
		select {
		case <-monitor.ImmediateShutdownRequired():
			t.Fatal("drain canceled accepted work")
		default:
		}
		close(accepted)
		repair1164Wait(t, drained)
		require.NoError(t, <-workerErrors)
		complete := make(chan struct{})
		go func() { monitor.CompleteShutdown(); close(complete) }()
		repair1164Wait(t, monitor.ImmediateShutdownRequired())
		select {
		case <-complete:
			t.Fatal("shutdown returned before worker joined")
		default:
		}
		close(release)
		repair1164Wait(t, complete)
		repair1164Wait(t, joined)
		require.Error(t, monitor.Go(func() { t.Error("worker admitted after stop") }))
		monitor.CompleteShutdown()
		reopened, err := os.Open(path)
		require.NoError(t, err)
		recovered, err := io.ReadAll(reopened)
		require.NoError(t, err)
		require.NoError(t, reopened.Close())
		require.Equal(t, expected, recovered)
	}
}

func TestRepairM_1164_DeleteRelativeLink(t *testing.T) {
	root := t.TempDir()
	cwd := filepath.Join(root, "cwd")
	links := filepath.Join(root, "links")
	require.NoError(t, os.MkdirAll(cwd, 0700))
	require.NoError(t, os.MkdirAll(links, 0700))
	t.Chdir(cwd)
	require.NoError(t, os.WriteFile("backing", []byte("unrelated"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(links, "backing"), []byte("intended"), 0600))
	link := filepath.Join(links, "relative")
	require.NoError(t, os.Symlink("backing", link))
	require.NoError(t, DeepDelete(link))
	untouched, err := os.ReadFile("backing")
	require.NoError(t, err)
	require.Equal(t, "unrelated", string(untouched))
	require.NoFileExists(t, filepath.Join(links, "backing"))
	require.NoFileExists(t, link)

	disk := filepath.Join(root, "disk")
	live := filepath.Join(disk, "table", "segments", "0.values")
	backing := filepath.Join(disk, "table", "snapshot", "0.values")
	snapshot := filepath.Join(root, "snapshot", "table", "segments", "0.values")
	for _, path := range []string{live, backing, snapshot} {
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700))
	}
	require.NoError(t, os.WriteFile(live, []byte("live immutable data"), 0600))
	require.NoError(t, os.Link(live, backing))
	relative, err := filepath.Rel(filepath.Dir(snapshot), backing)
	require.NoError(t, err)
	require.NoError(t, os.Symlink(relative, snapshot))
	require.ErrorContains(t, DeepDelete(snapshot, links), "outside configured backing roots")
	require.FileExists(t, backing)
	require.NoError(t, DeepDelete(snapshot, disk))
	require.NoFileExists(t, backing)
	_, err = os.Lstat(snapshot)
	require.True(t, os.IsNotExist(err))
	recovered, err := os.ReadFile(live)
	require.NoError(t, err)
	require.Equal(t, "live immutable data", string(recovered))
	for _, invalid := range []string{live, filepath.Join(root, "other", "snapshot", "0.values")} {
		require.NoError(t, os.MkdirAll(filepath.Dir(invalid), 0700))
		if invalid != live {
			require.NoError(t, os.WriteFile(invalid, []byte("other table"), 0600))
		}
		require.NoError(t, os.Symlink(invalid, snapshot))
		require.ErrorContains(t, DeepDelete(snapshot), "outside its table backing tree")
		require.FileExists(t, invalid)
		require.NoError(t, os.Remove(snapshot))
	}
	// A snapshot-shaped path cannot escape through a symlink in its backing tree.
	require.NoError(t, os.Symlink(live, backing))
	require.NoError(t, os.Symlink(backing, snapshot))
	require.ErrorContains(t, DeepDelete(snapshot), "must be regular")
	require.FileExists(t, live)
	require.NoError(t, os.Remove(snapshot))
	require.NoError(t, os.Remove(backing))
	require.NoError(t, os.Remove(filepath.Dir(backing)))
	require.NoError(t, os.Symlink(filepath.Dir(live), filepath.Dir(backing)))
	require.NoError(t, os.Symlink(backing, snapshot))
	require.ErrorContains(t, DeepDelete(snapshot), "contains a symlink")
	require.FileExists(t, live)
}

func TestRepairM_1164_PprofNetworkExposure(t *testing.T) {
	_, err := NewPprofProfilerWithConfig(PprofConfig{Address: "0.0.0.0:0"}, slog.Default())
	require.Error(t, err)
	_, err = NewPprofProfilerWithConfig(PprofConfig{Address: "localhost:0"}, slog.Default())
	require.Error(t, err)
	for _, protected := range []bool{false, true} {
		profiler := NewPprofProfiler("0", slog.Default())
		token := strings.Repeat("operator-secret-", 3)
		if protected {
			profiler, err = NewPprofProfilerWithConfig(PprofConfig{Address: "0.0.0.0:0", BearerToken: token}, slog.Default())
			require.NoError(t, err)
		}
		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan error, 1)
		go func() { done <- profiler.StartContext(ctx) }()
		t.Cleanup(cancel)
		require.Eventually(t, func() bool { return profiler.Addr() != nil }, 2*time.Second, 5*time.Millisecond)
		address := profiler.Addr().(*net.TCPAddr)
		if !protected {
			require.True(t, address.IP.IsLoopback())
		}
		profiler.mu.Lock()
		headerTimeout := profiler.server.ReadHeaderTimeout
		profiler.mu.Unlock()
		require.Equal(t, 10*time.Second, headerTimeout)
		url := "http://" + net.JoinHostPort("127.0.0.1", strconv.Itoa(address.Port)) + "/debug/pprof/"
		client := &http.Client{Timeout: 2 * time.Second}
		response, err := client.Get(url)
		require.NoError(t, err)
		if protected {
			require.Equal(t, http.StatusUnauthorized, response.StatusCode)
		} else {
			require.Equal(t, http.StatusOK, response.StatusCode)
		}
		require.NoError(t, response.Body.Close())
		if protected {
			request, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
			require.NoError(t, err)
			request.Header.Set("Authorization", "Bearer "+token)
			response, err = client.Do(request)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, response.StatusCode)
			require.NoError(t, response.Body.Close())
		}
		cancel()
		select {
		case err := <-done:
			require.NoError(t, err)
		case <-time.After(3 * time.Second):
			t.Fatal("profiler did not join shutdown")
		}
		connection, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(address.Port)), time.Second)
		if connection != nil {
			_ = connection.Close()
		}
		require.Error(t, err)
		require.NoError(t, profiler.Shutdown(t.Context()))
	}
}

// The peer speaks SSH using genuine host/client keys. Command responses are
// committed protocol transcripts; no process, shell, credential or endpoint is needed.
type repair1164Response struct {
	stdout, stderr string
	status         uint32
	stall, drop    bool
}
type repair1164SSHPeer struct {
	listener                             net.Listener
	key, trust                           string
	port                                 uint64
	host                                 ssh.Signer
	authorized                           ssh.PublicKey
	mu                                   sync.Mutex
	connections                          map[net.Conn]struct{}
	responses                            map[string]repair1164Response
	stallHandshake, stallOpen, stallExec bool
	started                              chan string
	stop                                 chan struct{}
	workers                              sync.WaitGroup
}

func repair1164Signer(t *testing.T) (ssh.Signer, []byte) {
	t.Helper()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	block, err := ssh.MarshalPrivateKey(key, "repair-m1164")
	require.NoError(t, err)
	signer, err := ssh.NewSignerFromKey(key)
	require.NoError(t, err)
	return signer, pem.EncodeToMemory(block)
}

func repair1164Peer(t *testing.T, responses map[string]repair1164Response, stall string) *repair1164SSHPeer {
	t.Helper()
	host, _ := repair1164Signer(t)
	client, key := repair1164Signer(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	root := t.TempDir()
	p := &repair1164SSHPeer{listener: listener, key: filepath.Join(root, "key 'with spaces\n$(literal)"), trust: filepath.Join(root, "known 'hosts\n$(literal)"), port: uint64(listener.Addr().(*net.TCPAddr).Port), host: host, authorized: client.PublicKey(), connections: map[net.Conn]struct{}{}, responses: responses, started: make(chan string, 10), stop: make(chan struct{}), stallHandshake: stall == "handshake", stallOpen: stall == "open", stallExec: stall == "exec"}
	require.NoError(t, os.WriteFile(p.key, key, 0600))
	require.NoError(t, os.WriteFile(p.trust, []byte(knownhosts.Line([]string{listener.Addr().String()}, host.PublicKey())+"\n"), 0600))
	p.workers.Add(1)
	go func() {
		defer p.workers.Done()
		for {
			connection, err := listener.Accept()
			if err != nil {
				return
			}
			p.mu.Lock()
			select {
			case <-p.stop:
				p.mu.Unlock()
				_ = connection.Close()
				return
			default:
			}
			p.connections[connection] = struct{}{}
			signer := p.host
			p.mu.Unlock()
			p.workers.Add(1)
			go p.serve(connection, signer)
		}
	}()
	t.Cleanup(func() {
		close(p.stop)
		_ = p.listener.Close()
		p.mu.Lock()
		for connection := range p.connections {
			_ = connection.Close()
		}
		p.mu.Unlock()
		p.workers.Wait()
	})
	return p
}

func (p *repair1164SSHPeer) serve(connection net.Conn, signer ssh.Signer) {
	defer p.workers.Done()
	defer func() { _ = connection.Close(); p.mu.Lock(); delete(p.connections, connection); p.mu.Unlock() }()
	if p.stallHandshake {
		<-p.stop
		return
	}
	config := &ssh.ServerConfig{PublicKeyCallback: func(metadata ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
		if metadata.User() != "backup" || !bytes.Equal(key.Marshal(), p.authorized.Marshal()) {
			return nil, fmt.Errorf("unauthorized key")
		}
		return nil, nil
	}}
	config.AddHostKey(signer)
	server, channels, requests, err := ssh.NewServerConn(connection, config)
	if err != nil {
		return
	}
	defer server.Close()
	done := make(chan struct{})
	p.workers.Add(2)
	go func() { defer p.workers.Done(); _ = server.Wait(); close(done) }()
	go func() { defer p.workers.Done(); ssh.DiscardRequests(requests) }()
	for incoming := range channels {
		if incoming.ChannelType() != "session" {
			_ = incoming.Reject(ssh.UnknownChannelType, "unsupported")
			continue
		}
		if p.stallOpen {
			select {
			case p.started <- "open":
			default:
			}
			<-done
			return
		}
		channel, channelRequests, err := incoming.Accept()
		if err != nil {
			return
		}
		p.workers.Add(1)
		go func() {
			defer p.workers.Done()
			defer channel.Close()
			for request := range channelRequests {
				if request.Type != "exec" {
					_ = request.Reply(false, nil)
					continue
				}
				var payload struct{ Command string }
				if ssh.Unmarshal(request.Payload, &payload) != nil {
					_ = request.Reply(false, nil)
					return
				}
				select {
				case p.started <- payload.Command:
				default:
				}
				if p.stallExec {
					<-done
					return
				}
				_ = request.Reply(true, nil)
				response, ok := p.responses[payload.Command]
				if !ok {
					response = repair1164Response{stderr: "unrecognized transcript: " + payload.Command, status: 127}
				}
				if response.drop {
					_ = server.Close()
					return
				}
				if response.stall {
					<-done
					return
				}
				_, _ = io.WriteString(channel, response.stdout)
				_, _ = io.WriteString(channel.Stderr(), response.stderr)
				_, _ = channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{response.status}))
				return
			}
		}()
	}
}

func (p *repair1164SSHPeer) connect(t *testing.T, ctx context.Context, options ...SSHTimeouts) *SSHSession {
	t.Helper()
	session, err := NewSSHSessionContext(ctx, slog.Default(), "backup", "127.0.0.1", p.port, p.key, p.trust, false, options...)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, session.Close()) })
	return session
}

func repair1164Wait(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("operation did not terminate")
	}
}

func TestRepairM_1164_SSHHostVerification(t *testing.T) {
	peer := repair1164Peer(t, map[string]repair1164Response{"identity": {stdout: "verified"}}, "")
	_, err := NewSSHSession(slog.Default(), "backup", "127.0.0.1", peer.port, peer.key, "", false)
	require.ErrorContains(t, err, "known-hosts trust source")
	empty := filepath.Join(t.TempDir(), "empty")
	require.NoError(t, os.WriteFile(empty, nil, 0600))
	_, err = NewSSHSessionContext(t.Context(), slog.Default(), "backup", "127.0.0.1", peer.port, peer.key, empty, false)
	require.Error(t, err)
	require.ErrorContains(t, err, "key is unknown")
	invalid := filepath.Join(t.TempDir(), "invalid")
	require.NoError(t, os.WriteFile(invalid, []byte("not a host key\n"), 0600))
	_, err = NewSSHSessionContext(t.Context(), slog.Default(), "backup", "127.0.0.1", peer.port, peer.key, invalid, false)
	require.ErrorContains(t, err, "parse known hosts")
	session := peer.connect(t, t.Context())
	stdout, _, err := session.Exec("identity")
	require.NoError(t, err)
	require.Equal(t, "verified", stdout)
	// Changing/removing the original trust/key files cannot diverge data-channel
	// trust from the authenticated control connection.
	originalTrust, err := os.ReadFile(peer.trust)
	require.NoError(t, err)
	require.NoError(t, os.Remove(peer.key))
	require.NoError(t, os.WriteFile(peer.trust, nil, 0600))
	frozen, err := os.ReadFile(session.knownHostsPath)
	require.NoError(t, err)
	require.Equal(t, originalTrust, frozen)
	dataSession, err := NewSSHSessionContext(t.Context(), slog.Default(), "backup", "127.0.0.1", peer.port, session.keyPath, session.knownHostsPath, false)
	require.NoError(t, err)
	require.NoError(t, dataSession.Close())
	changed, _ := repair1164Signer(t)
	peer.mu.Lock()
	peer.host = changed
	peer.mu.Unlock()
	_, err = NewSSHSessionContext(t.Context(), slog.Default(), "backup", "127.0.0.1", peer.port, session.keyPath, session.knownHostsPath, false)
	require.ErrorContains(t, err, "key mismatch")
	privateDir := filepath.Dir(session.keyPath)
	require.NoError(t, session.Close())
	require.NoDirExists(t, privateDir)
}

func TestRepairM_1164_SSHCancellation(t *testing.T) {
	t.Run("handshake", func(t *testing.T) {
		peer := repair1164Peer(t, nil, "handshake")
		ctx, cancel := context.WithTimeout(t.Context(), 80*time.Millisecond)
		defer cancel()
		started := time.Now()
		_, err := NewSSHSessionContext(ctx, slog.Default(), "backup", "127.0.0.1", peer.port, peer.key, peer.trust, false)
		require.Error(t, err)
		require.Less(t, time.Since(started), 2*time.Second)
	})
	for _, stall := range []string{"open", "exec", "running"} {
		t.Run(stall, func(t *testing.T) {
			peer := repair1164Peer(t, map[string]repair1164Response{"stall": {stall: true}}, stall)
			session := peer.connect(t, t.Context(), SSHTimeouts{Command: 80 * time.Millisecond})
			started := time.Now()
			_, _, err := session.ExecContext(t.Context(), "stall")
			require.ErrorIs(t, err, context.DeadlineExceeded)
			require.Less(t, time.Since(started), 2*time.Second)
			require.NoError(t, session.Close())
		})
	}
	t.Run("parent-and-close-join", func(t *testing.T) {
		peer := repair1164Peer(t, map[string]repair1164Response{"stall": {stall: true}}, "")
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		session := peer.connect(t, ctx)
		result := make(chan error, 1)
		go func() { _, _, err := session.Exec("stall"); result <- err }()
		select {
		case <-peer.started:
		case <-time.After(2 * time.Second):
			t.Fatal("command never started")
		}
		cancel()
		require.NoError(t, session.Close())
		select {
		case err := <-result:
			require.Error(t, err)
		case <-time.After(time.Second):
			t.Fatal("Close left an active command")
		}
		require.Error(t, session.Rsync("unused", "unused", 0))
	})
	t.Run("transfer-context", func(t *testing.T) {
		peer := repair1164Peer(t, nil, "")
		session := peer.connect(t, t.Context())
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		require.ErrorIs(t, session.RsyncContext(ctx, "unused", "unused", 0), context.Canceled)
		_, err := NewSSHSessionContext(t.Context(), slog.Default(), "backup", "127.0.0.1", peer.port, peer.key, peer.trust, false, SSHTimeouts{Transfer: -time.Second})
		require.Error(t, err)
	})
}

func TestRepairM_1164_SSHPathEncoding(t *testing.T) {
	const root = "/srv/space 'quote\" $(touch /tmp/owned)\nroot"
	const quoted = "'/srv/space '\"'\"'quote\" $(touch /tmp/owned)\nroot'"
	file := root + "/line\nbreak.values"
	peer := repair1164Peer(t, map[string]repair1164Response{
		"mkdir -p -- " + quoted:                     {},
		"find " + quoted + " -type f -print0":       {stdout: file + "\x00" + root + "/ignored.txt\x00"},
		"find './-option' -type f -print0":          {},
		"mkdir -p -- 'not-a-directory'":             {stderr: "File exists", status: 1},
		"find '/permission-denied' -type f -print0": {stderr: "Permission denied", status: 1},
		"find '/malformed' -type f -print0":         {stdout: "/malformed/file.values\n"},
		"find '/missing' -type f -print0":           {stderr: "find: /missing: No such file or directory\n", status: 1},
	}, "")
	session := peer.connect(t, t.Context())
	require.Equal(t, quoted, QuoteRemoteArgument(root))
	require.Equal(t, "''", QuoteRemoteArgument(""))
	require.NoError(t, session.Mkdirs(root))
	files, err := session.FindFiles(root, []string{".values"})
	require.NoError(t, err)
	require.Equal(t, []string{file}, files)
	files, err = session.FindFiles("-option", []string{".values"})
	require.NoError(t, err)
	require.Empty(t, files)
	files, err = session.FindFiles("/missing", []string{".values"})
	require.NoError(t, err)
	require.Empty(t, files)
	require.Error(t, session.Mkdirs("not-a-directory"))
	_, err = session.FindFiles("/permission-denied", []string{".values"})
	require.Error(t, err)
	_, err = session.FindFiles("/malformed", []string{".values"})
	require.ErrorContains(t, err, "NUL terminated")
	require.Error(t, session.Mkdirs("bad\x00path"))
	_, err = session.FindFiles("bad\x00path", nil)
	require.Error(t, err)

	sourceDir := t.TempDir()
	source := filepath.Join(sourceDir, "source 'quote\" $(literal)\nfile")
	require.NoError(t, os.WriteFile(source, []byte("source remains literal"), 0600))
	link := filepath.Join(sourceDir, "relative-link")
	require.NoError(t, os.Symlink(filepath.Base(source), link))
	destination := "-remote 'quote\" $(literal)\nfile"
	cmd, err := session.rsyncCommand(t.Context(), link, destination, 1.5)
	require.NoError(t, err)
	require.Equal(t, []string{"--", source, "backup@127.0.0.1:" + destination}, cmd.Args[len(cmd.Args)-3:])
	require.Contains(t, cmd.Args, "--fsync")
	require.Contains(t, cmd.Args, "--secluded-args")
	require.Contains(t, cmd.Args, "--bwlimit=1536")
	require.NotNil(t, cmd.Cancel)
	require.Equal(t, 5*time.Second, cmd.WaitDelay)
	require.True(t, cmd.SysProcAttr.Setpgid)
	var shell string
	for i, arg := range cmd.Args {
		if arg == "-e" {
			shell = cmd.Args[i+1]
			break
		}
	}
	words := repair1164RsyncWords(t, shell)
	require.Equal(t, "ssh", words[0])
	require.Contains(t, words, "StrictHostKeyChecking=yes")
	require.Contains(t, words, "GlobalKnownHostsFile=/dev/null")
	require.Contains(t, words, "UserKnownHostsFile=\""+session.knownHostsPath+"\"")
	require.Contains(t, words, session.keyPath)
	require.NotContains(t, shell, "StrictHostKeyChecking=no")
	// The parser receives each local SSH argument intact, even with nested quotes.
	for _, value := range []string{"", "spaces 'single' \"double\"", "$(literal)\nnew line"} {
		require.Equal(t, []string{value}, repair1164RsyncWords(t, quoteRsyncArgument(value)))
	}
	_, err = session.rsyncCommand(t.Context(), source, "bad\x00path", 0)
	require.Error(t, err)
}

// This is the receiver-side -e grammar, independent of the command producer:
// quoted words retain whitespace; a doubled delimiter represents one quote.
func repair1164RsyncWords(t *testing.T, input string) []string {
	t.Helper()
	var words []string
	for len(input) > 0 {
		input = strings.TrimLeft(input, " \t\n")
		if input == "" {
			break
		}
		var word strings.Builder
		var quote byte
		for len(input) > 0 {
			c := input[0]
			input = input[1:]
			if quote != 0 {
				if c == quote {
					if len(input) > 0 && input[0] == quote {
						word.WriteByte(c)
						input = input[1:]
					} else {
						quote = 0
					}
				} else {
					word.WriteByte(c)
				}
			} else if c == '\'' || c == '"' {
				quote = c
			} else if c == ' ' || c == '\t' || c == '\n' {
				break
			} else {
				word.WriteByte(c)
			}
		}
		require.Zero(t, quote, "unterminated rsync shell word")
		words = append(words, word.String())
	}
	return words
}

func TestRepairM_1164_SSHDurabilityFailures(t *testing.T) {
	const destination = "/backup/space 'quote\nfile"
	const command = "sync -f -- '/backup/space '\"'\"'quote\nfile' && printf 'litt-durable-v1\\n'"
	cases := []struct {
		name      string
		response  repair1164Response
		wantError string
	}{
		{"acknowledged", repair1164Response{stdout: "litt-durable-v1\n"}, ""},
		{"writeback-error", repair1164Response{stderr: "Input/output error", status: 1}, "remote durable commit failed"},
		{"missing-ack", repair1164Response{}, "missing remote durable acknowledgment"},
		{"partial-ack", repair1164Response{stdout: "litt-durable-v1"}, "missing remote durable acknowledgment"},
		{"lost-connection", repair1164Response{drop: true}, "remote durable commit failed"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			peer := repair1164Peer(t, map[string]repair1164Response{command: test.response}, "")
			session := peer.connect(t, t.Context())
			source := filepath.Join(t.TempDir(), "retained-snapshot")
			require.NoError(t, os.WriteFile(source, []byte("retain until every acknowledgment succeeds"), 0600))
			err := session.acknowledgeDurable(t.Context(), destination)
			if test.wantError == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, test.wantError)
			}
			content, err := os.ReadFile(source)
			require.NoError(t, err)
			require.Equal(t, "retain until every acknowledgment succeeds", string(content))
			// A failed source lookup fails in the real transfer path before starting
			// any subprocess and cannot reclaim or modify snapshot data.
			require.Error(t, session.Rsync(source+"-missing", destination, 0))
		})
	}
}

func TestRepairM_1164_SSHTestsRunnable(t *testing.T) {
	peer := repair1164Peer(t, map[string]repair1164Response{"fixture-command": {stdout: "stdout\n", stderr: "stderr\n"}, "failed-command": {stderr: "rejected", status: 42}}, "")
	session := peer.connect(t, t.Context())
	out, diagnostic, err := session.Exec("fixture-command")
	require.NoError(t, err)
	require.Equal(t, "stdout\n", out)
	require.Equal(t, "stderr\n", diagnostic)
	_, diagnostic, err = session.Exec("failed-command")
	require.Error(t, err)
	require.Equal(t, "rejected", diagnostic)
	var exit *ssh.ExitError
	require.True(t, errors.As(err, &exit))
	require.Equal(t, 42, exit.ExitStatus())
	require.NoError(t, session.Close())
	require.NoError(t, session.Close())
	_, _, err = session.Exec("fixture-command")
	require.Error(t, err)
}
