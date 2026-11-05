package replica

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"

	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/store"
)

func TestRepairM_148_LedgerLongHistory(t *testing.T) {
	f := newFixture(t)
	s := f.shipper()
	const history, retain = 256, 8
	st, err := store.Open(filepath.Join(f.dir, "history-store"), bytes.Repeat([]byte{1}, store.KeySize))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	var firstRaw []byte
	var firstEntry LedgerEntry
	last := time.Unix(1_000_000, 0).UTC()
	s.now = func() time.Time { return last }
	for i := 0; i < history; i++ {
		name := fmt.Sprintf("snapshot-%04d.bin", i)
		var raw bytes.Buffer
		if err := st.Snapshot(&raw, bytes.Repeat([]byte{2}, store.KeySize), "node-1"); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(f.snapshotDir, name), raw.Bytes(), 0o600); err != nil {
			t.Fatal(err)
		}
		last = last.Add(time.Minute)
		if n, err := s.Ship(context.Background()); err != nil || n != 1 {
			t.Fatalf("shipment %d = %d, %v", i, n, err)
		}
		entry, ok := s.Ledger().Lookup(name)
		if !ok || entry.Size != int64(raw.Len()) || entry.SHA256 != sha256Hex(raw.Bytes()) {
			t.Fatalf("shipment %d acknowledgement = %+v, %v", i, entry, ok)
		}
		if i == 0 {
			firstRaw, firstEntry = append([]byte(nil), raw.Bytes()...), entry
		}
		if err := s.Ledger().Record(entry); !errors.Is(err, ErrLedger) {
			t.Fatalf("active duplicate accepted: %v", err)
		}
		// Match local retention's acknowledgement check before removing a snapshot.
		if i >= retain {
			oldName := fmt.Sprintf("snapshot-%04d.bin", i-retain)
			if _, ok := s.Ledger().Lookup(oldName); !ok {
				t.Fatalf("retiring unacknowledged %s", oldName)
			}
			if err := os.Remove(filepath.Join(f.snapshotDir, oldName)); err != nil {
				t.Fatal(err)
			}
		}
		if n, err := s.Ship(context.Background()); err != nil || n != 0 {
			t.Fatalf("retention pass %d = %d, %v", i, n, err)
		}
		want := i + 1
		if want > retain {
			want = retain
		}
		if n := len(s.Ledger().Entries()); n != want {
			t.Fatalf("history %d: active index %d, want %d", i, n, want)
		}
		rawIndex, err := os.ReadFile(s.cfg.LedgerPath)
		if err != nil {
			t.Fatal(err)
		}
		if len(rawIndex) > 4096 {
			t.Fatalf("active index grew with history: %d bytes", len(rawIndex))
		}
		if i > 0 && i%37 == 0 {
			s = f.shipper() // All Ship calls have ended and closed their transports.
			s.now = func() time.Time { return last }
			if !s.State().LastShipped.Equal(last) {
				t.Fatal("restart lost last-shipped state")
			}
		}
	}
	if got := f.srv.writes.Load(); got != history {
		t.Fatalf("remote writes %d, want %d", got, history)
	}
	if !bytes.Equal(f.remoteBytes(firstEntry.Name), firstRaw) {
		t.Fatal("retirement changed the remote snapshot")
	}
	if _, ok := s.Ledger().Lookup(firstEntry.Name); ok {
		t.Fatal("retired entry is still active")
	}
	for i := 0; i < history-retain; i++ {
		name := fmt.Sprintf("snapshot-%04d.bin", i)
		e, ok, err := s.Ledger().Archived(name)
		if err != nil || !ok || e.Name != name {
			t.Fatalf("archived acknowledgement %s = %+v, %v, %v", name, e, ok, err)
		}
	}
	// Retire all remaining files, including the newest acknowledgement.
	for _, entry := range s.Ledger().Entries() {
		if err := os.Remove(filepath.Join(f.snapshotDir, entry.Name)); err != nil {
			t.Fatal(err)
		}
	}
	if n, err := s.Ship(context.Background()); err != nil || n != 0 {
		t.Fatalf("final retirement = %d, %v", n, err)
	}
	s = f.shipper()
	if len(s.Ledger().Entries()) != 0 || !s.State().LastShipped.Equal(last) {
		t.Fatalf("empty-index recovery = %+v, %v", s.State(), s.Ledger().Entries())
	}
	if err := s.Ledger().Record(firstEntry); !errors.Is(err, ErrLedger) {
		t.Fatalf("archived duplicate accepted after restart: %v", err)
	}
	// Reappearing identical snapshots use archived proof without another upload.
	if err := os.WriteFile(filepath.Join(f.snapshotDir, firstEntry.Name), firstRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	if n, err := s.Ship(context.Background()); err != nil || n != 0 {
		t.Fatalf("archived duplicate shipment = %d, %v", n, err)
	}
	if f.srv.writes.Load() != history {
		t.Fatal("archived duplicate uploaded again")
	}
	changed := append([]byte(nil), firstRaw...)
	changed[len(changed)-1] ^= 1
	if err := os.WriteFile(filepath.Join(f.snapshotDir, firstEntry.Name), changed, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Ship(context.Background()); !errors.Is(err, ErrLedger) {
		t.Fatalf("changed archived snapshot accepted: %v", err)
	}
	if f.srv.writes.Load() != history {
		t.Fatal("conflicting archived duplicate reached transport")
	}
}

func TestRepairM_148_RetirementDurability(t *testing.T) {
	f := newFixture(t)
	f.writeSnapshot("retired.bin")
	f.writeSnapshot("retained.bin")
	s := f.shipper()
	if n, err := s.Ship(context.Background()); err != nil || n != 2 {
		t.Fatalf("Ship = %d, %v", n, err)
	}
	entry, _ := s.Ledger().Lookup("retired.bin")
	active, err := os.ReadFile(s.cfg.LedgerPath)
	if err != nil {
		t.Fatal(err)
	}
	// Neither age nor repeated passes may evict an acknowledged local snapshot.
	s.now = func() time.Time { return time.Unix(4_000_000_000, 0) }
	if err := s.Ledger().RetireAbsent(f.snapshotDir); err != nil {
		t.Fatal(err)
	}
	if len(s.Ledger().Entries()) != 2 {
		t.Fatal("retirement pruned a present local snapshot")
	}
	if err := s.Ledger().RetireAbsent(filepath.Join(f.dir, "missing")); !errors.Is(err, ErrLedger) {
		t.Fatalf("missing directory is retirement: %v", err)
	}
	if len(s.Ledger().Entries()) != 2 {
		t.Fatal("missing directory pruned acknowledgements")
	}
	if err := os.Remove(filepath.Join(f.snapshotDir, entry.Name)); err != nil {
		t.Fatal(err)
	}
	// A real filesystem failure must leave the active acknowledgement durable.
	if err := os.WriteFile(s.cfg.LedgerPath+".archive", []byte("blocked"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Ship(context.Background()); !errors.Is(err, ErrLedger) {
		t.Fatalf("archive failure hidden: %v", err)
	}
	reopened, err := OpenLedger(s.cfg.LedgerPath)
	if err != nil || len(reopened.Entries()) != 2 {
		t.Fatalf("failed archive lost active state: %v, %v", reopened, err)
	}
	if err := os.Remove(s.cfg.LedgerPath + ".archive"); err != nil {
		t.Fatal(err)
	}
	if n, err := s.Ship(context.Background()); err != nil || n != 0 {
		t.Fatalf("archive retry = %d, %v", n, err)
	}
	archived, ok, err := s.Ledger().Archived(entry.Name)
	if err != nil || !ok || archived != entry {
		t.Fatalf("archive = %+v, %v, %v", archived, ok, err)
	}
	// Reopen an interrupted retirement: archive persisted, old active index intact.
	if err := os.WriteFile(s.cfg.LedgerPath, active, 0o600); err != nil {
		t.Fatal(err)
	}
	s = f.shipper()
	if n, err := s.Ship(context.Background()); err != nil || n != 0 {
		t.Fatalf("interrupted retirement recovery = %d, %v", n, err)
	}
	if len(s.Ledger().Entries()) != 1 {
		t.Fatal("recovery did not finish retirement")
	}
	if _, ok := s.Ledger().Lookup("retained.bin"); !ok {
		t.Fatal("recovery pruned a present acknowledged snapshot")
	}
	if f.srv.writes.Load() != 2 {
		t.Fatal("recovery uploaded acknowledged snapshots again")
	}
	// Broken archived evidence fails closed rather than forgetting a shipment.
	if err := os.WriteFile(s.Ledger().archivePath(entry.Name), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := s.Ledger().Record(entry); !errors.Is(err, ErrLedger) {
		t.Fatalf("corrupt archive treated as absent: %v", err)
	}
}

// This peer runs the real SSH and SFTP servers over loopback. Its channel reader
// pauses a selected client packet, emulating a peer that stops servicing SFTP
// while continuing to send SSH traffic. No shipper or ledger method is replaced.
type repairM148StallChannel struct {
	ssh.Channel
	packet       byte
	pending      []byte
	reached      chan struct{}
	disconnected <-chan struct{}
	once         sync.Once
}

func (c *repairM148StallChannel) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if len(c.pending) == 0 {
		var header [4]byte
		if _, err := io.ReadFull(c.Channel, header[:]); err != nil {
			return 0, err
		}
		length := binary.BigEndian.Uint32(header[:])
		if length == 0 || length > 1<<20 {
			return 0, fmt.Errorf("invalid SFTP packet length %d", length)
		}
		frame := make([]byte, 4+int(length))
		copy(frame, header[:])
		if _, err := io.ReadFull(c.Channel, frame[4:]); err != nil {
			return 0, err
		}
		if frame[4] == c.packet {
			c.once.Do(func() { close(c.reached) })
			<-c.disconnected
			return 0, io.EOF
		}
		c.pending = frame
	}
	n := copy(p, c.pending)
	c.pending = c.pending[n:]
	return n, nil
}

func repairM148Peer(t *testing.T, f *fixture, stage string) (string, <-chan struct{}, <-chan struct{}) {
	t.Helper()
	reached, disconnected, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	cfg := &ssh.ServerConfig{PublicKeyCallback: func(_ ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
		if !bytes.Equal(key.Marshal(), f.clientKey.PublicKey().Marshal()) {
			return nil, errors.New("unauthorized key")
		}
		return &ssh.Permissions{}, nil
	}}
	cfg.AddHostKey(f.hostKey)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var accepted net.Conn
	t.Cleanup(func() {
		ln.Close()
		mu.Lock()
		if accepted != nil {
			accepted.Close()
		}
		mu.Unlock()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("stalled SSH peer did not stop")
		}
	})
	go func() {
		defer close(done)
		raw, err := ln.Accept()
		if err != nil {
			return
		}
		mu.Lock()
		accepted = raw
		mu.Unlock()
		defer raw.Close()
		conn, chans, reqs, err := ssh.NewServerConn(raw, cfg)
		if err != nil {
			t.Errorf("SSH server handshake: %v", err)
			return
		}
		defer conn.Close()
		go ssh.DiscardRequests(reqs)
		go func() { conn.Wait(); close(disconnected) }()
		go func() {
			ticker := time.NewTicker(20 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-disconnected:
					return
				case <-ticker.C:
					conn.SendRequest("keepalive@repair-fixture", false, nil)
				}
			}
		}()
		for newCh := range chans {
			if newCh.ChannelType() != "session" {
				newCh.Reject(ssh.UnknownChannelType, "session only")
				continue
			}
			if stage == "channel" {
				close(reached)
				<-disconnected
				return
			}
			ch, requests, err := newCh.Accept()
			if err != nil {
				return
			}
			defer ch.Close()
			for req := range requests {
				if req.Type != "subsystem" || len(req.Payload) < 4 || string(req.Payload[4:]) != "sftp" {
					req.Reply(false, nil)
					continue
				}
				if stage == "subsystem" {
					close(reached)
					<-disconnected
					return
				}
				req.Reply(true, nil)
				go ssh.DiscardRequests(requests)
				packets := map[string]byte{"init": 1, "stat": 17, "upload": 6, "readback": 5, "rename": 18}
				channel := &repairM148StallChannel{Channel: ch, packet: packets[stage], reached: reached, disconnected: disconnected}
				server, err := sftp.NewServer(channel)
				if err != nil {
					t.Errorf("SFTP server: %v", err)
					return
				}
				defer server.Close()
				if err := server.Serve(); err != nil && !errors.Is(err, io.EOF) {
					select {
					case <-disconnected:
					default:
						t.Errorf("SFTP server: %v", err)
					}
				}
				return
			}
		}
	}()
	return ln.Addr().String(), reached, disconnected
}

func TestRepairM_148_StalledPeer(t *testing.T) {
	for _, stage := range []string{"channel", "subsystem", "init", "stat", "upload", "readback", "rename"} {
		for _, action := range []string{"cancel", "operation-deadline", "caller-deadline"} {
			t.Run(stage+"/"+action, func(t *testing.T) {
				f := newFixture(t)
				f.writeSnapshot("stalled.bin")
				addr, reached, disconnected := repairM148Peer(t, f, stage)
				s := f.shipper()
				s.cfg.Addr = addr
				s.cfg.RemoteDir = t.TempDir()
				s.cfg.OperationTimeout = 5 * time.Second
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				want := error(context.Canceled)
				if action == "operation-deadline" {
					s.cfg.OperationTimeout = 400 * time.Millisecond
					want = context.DeadlineExceeded
				}
				if action == "caller-deadline" {
					var deadlineCancel context.CancelFunc
					ctx, deadlineCancel = context.WithTimeout(ctx, 400*time.Millisecond)
					defer deadlineCancel()
					want = context.DeadlineExceeded
				}
				type result struct {
					n   int
					err error
				}
				resultCh := make(chan result, 1)
				started := time.Now()
				go func() { n, err := s.Ship(ctx); resultCh <- result{n, err} }()
				select {
				case <-reached:
				case result := <-resultCh:
					t.Fatalf("Ship ended before real peer reached %s: %+v", stage, result)
				case <-time.After(2 * time.Second):
					t.Fatalf("real peer never reached %s", stage)
				}
				if action == "cancel" {
					cancel()
				}
				select {
				case result := <-resultCh:
					if result.n != 0 || !errors.Is(result.err, want) {
						t.Fatalf("Ship = %d, %v; want 0, %v", result.n, result.err, want)
					}
				case <-time.After(2 * time.Second):
					t.Fatal("Ship remained blocked on a stalled SSH/SFTP peer")
				}
				if time.Since(started) > 2*time.Second {
					t.Fatal("operation exceeded bound")
				}
				select {
				case <-disconnected:
				case <-time.After(time.Second):
					t.Fatal("underlying SSH connection was not closed")
				}
				if len(s.Ledger().Entries()) != 0 || !s.State().LastShipped.IsZero() || s.State().ErrorClass == "" {
					t.Fatalf("stalled shipment acknowledged: %+v, %v", s.State(), s.Ledger().Entries())
				}
				if _, err := os.Stat(filepath.Join(s.cfg.RemoteDir, "stalled.bin")); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("stalled shipment published a final snapshot: %v", err)
				}
				// A later attempt can use the same shipper after the timed-out transport closes.
				s.cfg.Addr, s.cfg.RemoteDir = f.srv.addr, remoteDir
				s.cfg.OperationTimeout = 5 * time.Second
				if n, err := s.Ship(context.Background()); err != nil || n != 1 {
					t.Fatalf("retry after %s/%s = %d, %v", stage, action, n, err)
				}
				if s.State().ErrorClass != "" || len(s.Ledger().Entries()) != 1 {
					t.Fatal("healthy retry did not clear timeout state")
				}
			})
		}
	}
}

func TestRepairM_148_RunCancellationAndQueuedShip(t *testing.T) {
	f := newFixture(t)
	f.writeSnapshot("run.bin")
	addr, reached, disconnected := repairM148Peer(t, f, "subsystem")
	s := f.shipper()
	s.cfg.Addr = addr
	s.cfg.OperationTimeout = 5 * time.Second
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() { defer close(done); s.Run(ctx) }()
	select {
	case <-reached:
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not reach stalled subsystem")
	}
	queued, cancelQueued := context.WithCancel(context.Background())
	cancelQueued()
	result := make(chan error, 1)
	go func() { _, err := s.Ship(queued); result <- err }()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("queued cancellation = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("canceled Ship waited for another shipping pass")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run did not shut down on cancellation")
	}
	select {
	case <-disconnected:
	case <-time.After(time.Second):
		t.Fatal("Run left the SSH connection open")
	}
}

func TestRepairM_148_TimeoutConfigAndLegacyLedger(t *testing.T) {
	f := newFixture(t)
	env := f.env()
	load := func() (*Config, error) {
		return LoadConfig(func(k string) string { return env[k] }, f.snapshotDir, f.dataDir)
	}
	cfg, err := load()
	if err != nil || cfg.OperationTimeout != DefaultOperationTimeout {
		t.Fatalf("default operation timeout: %+v, %v", cfg, err)
	}
	env[EnvOperationTimeout] = "750ms"
	cfg, err = load()
	if err != nil || cfg.OperationTimeout != 750*time.Millisecond {
		t.Fatalf("configured operation timeout: %+v, %v", cfg, err)
	}
	for _, value := range []string{"0s", "-1s", "forever"} {
		env[EnvOperationTimeout] = value
		if _, err := load(); !errors.Is(err, ErrConfig) {
			t.Fatalf("invalid timeout %q accepted: %v", value, err)
		}
	}
	cfg.OperationTimeout = -time.Second
	if _, err := New(*cfg, log.New(io.Discard, "", 0)); !errors.Is(err, ErrConfig) {
		t.Fatalf("negative programmatic timeout accepted: %v", err)
	}
	cfg.OperationTimeout = 0
	s, err := New(*cfg, log.New(io.Discard, "", 0))
	if err != nil || s.cfg.OperationTimeout != DefaultOperationTimeout {
		t.Fatalf("programmatic default = %v, %v", s, err)
	}
	entry := LedgerEntry{Name: "legacy.bin", Size: 42, SHA256: hex.EncodeToString(bytes.Repeat([]byte{3}, 32)), ShippedAt: time.Unix(1_000_000, 0).UTC()}
	// Encode the original v1 shape, without the new high-water metadata.
	raw, err := json.Marshal(struct {
		Version int           `json:"version"`
		Entries []LedgerEntry `json:"entries"`
	}{1, []LedgerEntry{entry}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(f.dataDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg.LedgerPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	l, err := OpenLedger(cfg.LedgerPath)
	if err != nil || !l.LastShipped().Equal(entry.ShippedAt) {
		t.Fatalf("legacy ledger recovery = %v, %v", l, err)
	}
	if err := l.RetireAbsent(f.snapshotDir); err != nil {
		t.Fatal(err)
	}
	l, err = OpenLedger(cfg.LedgerPath)
	if err != nil || len(l.Entries()) != 0 || !l.LastShipped().Equal(entry.ShippedAt) {
		t.Fatalf("legacy retirement recovery = %v, %v", l, err)
	}
	archived, ok, err := l.Archived(entry.Name)
	if err != nil || !ok || archived != entry {
		t.Fatalf("legacy archive = %+v, %v, %v", archived, ok, err)
	}
}
