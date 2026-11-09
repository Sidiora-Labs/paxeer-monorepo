package backup

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/audit"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/replica"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/store"
)

type fixture struct {
	nodeKey   []byte
	backupKey []byte
	dataDir   string
	dir       string
	store     *store.Store
	audit     *audit.Log
	shares    map[string][]byte
	recovery  *repairM147Recovery
}

func randomKey(t *testing.T) []byte {
	t.Helper()
	k := make([]byte, store.KeySize)
	if _, err := rand.Read(k); err != nil {
		t.Fatal(err)
	}
	return k
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	root := t.TempDir()
	f := &fixture{
		nodeKey:   randomKey(t),
		backupKey: randomKey(t),
		dataDir:   filepath.Join(root, "data"),
		dir:       filepath.Join(root, "backup"),
		shares:    map[string][]byte{},
	}
	var err error
	if f.store, err = store.Open(f.dataDir, f.nodeKey); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.store.Close() })
	if f.audit, err = audit.Open(filepath.Join(f.dataDir, "audit")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.audit.Close() })
	secp := make([]byte, 33)
	secp[0] = 0x02
	secp[32] = 0x11
	ed := make([]byte, 32)
	ed[0] = 0x22
	f.putImported(t, store.ShareRecord{KeyID: "evm-key", Curve: store.CurveSecp256k1, PublicKey: secp, Participants: []string{"node-1", "node-2", "node-3", "node-4", "node-5"}}, "secp256k1 share material")
	f.put(t, store.ShareRecord{KeyID: "lx-key", Curve: store.CurveEd25519, PublicKey: ed, Epoch: 2, Participants: []string{"node-1", "node-2", "node-3", "node-4", "node-5"}}, "ed25519 share material")
	f.seedRecoveryState(t)
	return f
}

func (f *fixture) put(t *testing.T, rec store.ShareRecord, share string) {
	t.Helper()
	if err := f.store.Put(rec, []byte(share)); err != nil {
		t.Fatal(err)
	}
	f.shares[rec.KeyID] = []byte(share)
}

func (f *fixture) writer(t *testing.T, retain int, shipped Shipped) *Writer {
	t.Helper()
	w, err := New(Config{NodeID: "node-1", Dir: f.dir, BackupKey: f.backupKey, Retain: retain, Store: f.store, Audit: f.audit, Shipped: shipped, Logger: log.New(io.Discard, "", 0)})
	if err != nil {
		t.Fatal(err)
	}
	return w
}

func listDir(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		out = append(out, e.Name())
	}
	sort.Strings(out)
	return out
}

func names(seqs ...uint64) []string {
	out := make([]string, 0, len(seqs))
	for _, s := range seqs {
		out = append(out, Name("node-1", s))
	}
	return out
}

func (f *fixture) assertRestorable(t *testing.T, path string) {
	t.Helper()
	_, digest, err := fileDigest(path)
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "restore")
	if _, err := RestoreFile(path, hex.EncodeToString(digest[:]), "node-1", f.backupKey, target, f.nodeKey); err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(target, f.nodeKey)
	if err != nil {
		t.Fatalf("restore %s: %v", filepath.Base(path), err)
	}
	defer st.Close()
	for keyID, want := range f.shares {
		var got []byte
		if err := st.WithShare(keyID, func(p []byte) error { got = append(got, p...); return nil }); err != nil || !bytes.Equal(got, want) {
			t.Fatalf("restored share %s: %v", keyID, err)
		}
	}
	f.assertRecoveryState(t, st)
}

func TestWriteSnapshotAtomicNameSequence(t *testing.T) {
	f := newFixture(t)
	w := f.writer(t, 10, nil)
	if err := os.WriteFile(filepath.Join(f.dir, "."+Name("node-1", 9)+".123.tmp"), []byte("partial"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.dir, Name("node-7", 40)), []byte("another node"), 0o600); err != nil {
		t.Fatal(err)
	}
	for i := uint64(1); i <= 3; i++ {
		snap, err := w.WriteSnapshot("keys.generate")
		if err != nil {
			t.Fatal(err)
		}
		if snap.Sequence != i || snap.Name != Name("node-1", i) || snap.Path != filepath.Join(f.dir, snap.Name) {
			t.Fatalf("snapshot %d = %+v", i, snap)
		}
		raw, err := os.ReadFile(snap.Path)
		if err != nil {
			t.Fatal(err)
		}
		if sha256.Sum256(raw) != snap.SHA256 || int64(len(raw)) != snap.Size || raw[0] != store.SnapshotVersion {
			t.Fatalf("snapshot %d digest, size or version does not match the file", i)
		}
		f.assertRestorable(t, snap.Path)
	}
	if Name("node-1", 1) != "snapshot-node-1-000000000001.snap" {
		t.Fatalf("name format %s", Name("node-1", 1))
	}
	want := append([]string{"." + Name("node-1", 9) + ".123.tmp"}, names(1, 2, 3)...)
	want = append(want, Name("node-7", 40))
	sort.Strings(want)
	if got := listDir(t, f.dir); !reflect.DeepEqual(got, want) {
		t.Fatalf("directory %v, want %v", got, want)
	}
	for _, n := range listDir(t, f.dir) {
		if strings.HasPrefix(n, ".") && n != "."+Name("node-1", 9)+".123.tmp" {
			t.Fatalf("temporary file %s left behind", n)
		}
	}

	again := f.writer(t, 10, nil)
	snap, err := again.WriteSnapshot("interval")
	if err != nil {
		t.Fatal(err)
	}
	if snap.Sequence != 4 || snap.Name != Name("node-1", 4) {
		t.Fatalf("restarted writer continued at %+v", snap)
	}
	if state := again.State(); state.LastWritten.IsZero() || state.LastError != "" || state.Failures != 0 {
		t.Fatalf("state after a write %+v", state)
	}
}

func TestParseNameAndNodeID(t *testing.T) {
	node, seq, ok := ParseName("snapshot-node-1-000000000042.snap")
	if !ok || node != "node-1" || seq != 42 {
		t.Fatalf("ParseName = %q %d %v", node, seq, ok)
	}
	for _, bad := range []string{"snapshot-node-1-42.snap", "snapshot-node-1-000000000000.snap", "snapshot--000000000001.snap", "snapshot-node-1-000000000001.bin", "other-node-1-000000000001.snap", ".snapshot-node-1-000000000001.snap.1.tmp", "snapshot-node/1-000000000001.snap"} {
		if _, _, ok := ParseName(bad); ok {
			t.Fatalf("ParseName accepted %q", bad)
		}
	}
	for _, id := range []string{"", "node 1", "node/1", strings.Repeat("n", 65)} {
		if ValidNodeID(id) {
			t.Fatalf("ValidNodeID accepted %q", id)
		}
	}
	f := newFixture(t)
	if _, err := New(Config{NodeID: "node/1", Dir: f.dir, BackupKey: f.backupKey, Retain: 1, Store: f.store, Audit: f.audit}); !errors.Is(err, ErrConfig) {
		t.Fatalf("invalid node id: %v", err)
	}
	if _, err := New(Config{NodeID: "node-1", Dir: f.dir, BackupKey: f.backupKey, Retain: 0, Store: f.store, Audit: f.audit}); !errors.Is(err, ErrConfig) {
		t.Fatalf("zero retention: %v", err)
	}
	if _, err := New(Config{NodeID: "node-1", Dir: f.dir, BackupKey: f.backupKey[:16], Retain: 1, Store: f.store, Audit: f.audit}); !errors.Is(err, ErrConfig) {
		t.Fatalf("short backup key: %v", err)
	}
}

func TestSnapshotEventInAuditChain(t *testing.T) {
	f := newFixture(t)
	w := f.writer(t, 5, nil)
	snap, err := w.WriteSnapshot("keys.refresh")
	if err != nil {
		t.Fatal(err)
	}
	seq, _ := f.audit.Head()
	if snap.AuditSequence == 0 || snap.AuditSequence != seq {
		t.Fatalf("audit sequence %d, head %d", snap.AuditSequence, seq)
	}
	if err := f.audit.Verify(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(snap.Path)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(raw)
	logged, err := os.ReadFile(filepath.Join(f.dataDir, "audit", audit.FileName))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{SnapshotEvent, "written", "name=" + snap.Name, "sha256=" + hex.EncodeToString(digest[:]), "node=node-1", "trigger=keys.refresh"} {
		if !bytes.Contains(logged, []byte(want)) {
			t.Fatalf("audit log lacks %q", want)
		}
	}
	for _, share := range f.shares {
		if bytes.Contains(logged, share) || bytes.Contains(raw, share) {
			t.Fatal("share material appears in the audit log or the snapshot")
		}
	}
}

func TestFailedWriteIsCountedAndReported(t *testing.T) {
	f := newFixture(t)
	w := f.writer(t, 5, nil)
	if _, err := w.WriteSnapshot("keys.generate"); err != nil {
		t.Fatal(err)
	}
	before, _ := f.audit.Head()
	f.store.Close()
	if _, err := w.WriteSnapshot("keys.import"); err == nil {
		t.Fatal("snapshot of a closed store succeeded")
	}
	state := w.State()
	if state.Failures != 1 || state.LastError == "" || state.LastWritten.IsZero() {
		t.Fatalf("state after a failed write %+v", state)
	}
	if got := listDir(t, f.dir); !reflect.DeepEqual(got, names(1)) {
		t.Fatalf("failed write left %v", got)
	}
	after, _ := f.audit.Head()
	if after != before+1 {
		t.Fatalf("failed write not audited: head %d then %d", before, after)
	}
	logged, err := os.ReadFile(filepath.Join(f.dataDir, "audit", audit.FileName))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(logged, []byte("trigger=keys.import error=")) {
		t.Fatal("audit log lacks the failed snapshot entry")
	}
}

func TestRetainWithoutLedgerDeletesByCount(t *testing.T) {
	f := newFixture(t)
	w := f.writer(t, 2, nil)
	for i := 0; i < 5; i++ {
		if _, err := w.WriteSnapshot("interval"); err != nil {
			t.Fatal(err)
		}
	}
	if got := listDir(t, f.dir); !reflect.DeepEqual(got, names(4, 5)) {
		t.Fatalf("directory %v, want %v", got, names(4, 5))
	}
	deleted, err := w.Retain()
	if err != nil || len(deleted) != 0 {
		t.Fatalf("second retention deleted %v: %v", deleted, err)
	}
}

func TestRetainWithLedgerDeletesOnlyShipped(t *testing.T) {
	f := newFixture(t)
	ledger, err := replica.OpenLedger(filepath.Join(f.dataDir, replica.LedgerFileName))
	if err != nil {
		t.Fatal(err)
	}
	w := f.writer(t, 2, ledger)
	var snaps []Snapshot
	for i := 0; i < 4; i++ {
		snap, err := w.WriteSnapshot("interval")
		if err != nil {
			t.Fatal(err)
		}
		snaps = append(snaps, snap)
	}
	if got := listDir(t, f.dir); !reflect.DeepEqual(got, names(1, 2, 3, 4)) {
		t.Fatalf("unshipped snapshots were deleted: %v", got)
	}
	record := func(s Snapshot, digest string) {
		t.Helper()
		if err := ledger.Record(replica.LedgerEntry{Name: s.Name, Size: s.Size, SHA256: digest, ShippedAt: time.Now().UTC()}); err != nil {
			t.Fatal(err)
		}
	}
	record(snaps[0], hex.EncodeToString(snaps[0].SHA256[:]))
	record(snaps[1], strings.Repeat("ab", 32))
	record(snaps[2], hex.EncodeToString(snaps[2].SHA256[:]))
	if _, err := w.WriteSnapshot("interval"); err != nil {
		t.Fatal(err)
	}
	if got := listDir(t, f.dir); !reflect.DeepEqual(got, names(2, 4, 5)) {
		t.Fatalf("directory %v, want %v", got, names(2, 4, 5))
	}
	state := w.State()
	if state.Failures != 1 || !strings.Contains(state.LastError, Name("node-1", 2)) {
		t.Fatalf("a snapshot that differs from its shipped copy was not reported: %+v", state)
	}
	record(snaps[3], hex.EncodeToString(snaps[3].SHA256[:]))
	if _, err := w.WriteSnapshot("interval"); err != nil {
		t.Fatal(err)
	}
	if got := listDir(t, f.dir); !reflect.DeepEqual(got, names(2, 5, 6)) {
		t.Fatalf("directory %v, want %v", got, names(2, 5, 6))
	}
	deleted, err := w.Retain()
	if len(deleted) != 0 || err == nil || !strings.Contains(err.Error(), Name("node-1", 2)) {
		t.Fatalf("retention deleted %v with error %v", deleted, err)
	}
}

func TestRetainReappearingArchivedSnapshots(t *testing.T) {
	f := newFixture(t)
	host, _ := repairM147Signer(t)
	client, private := repairM147Signer(t)
	peer := repairM147Replica(t, host, client.PublicKey())
	addr, connections := retainReplicaConnections(t, peer)
	root, remote := t.TempDir(), t.TempDir()
	keyPath, hostPath := filepath.Join(root, "client-key"), filepath.Join(root, "host-key")
	if err := os.WriteFile(keyPath, private, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(hostPath, ssh.MarshalAuthorizedKey(host.PublicKey()), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := replica.Config{Addr: addr, User: "backup-test", KeyFile: keyPath, HostKeyFile: hostPath, RemoteDir: remote, Interval: replica.MinInterval, SnapshotDir: f.dir, LedgerPath: filepath.Join(root, replica.LedgerFileName)}
	var shipper *replica.Shipper
	var w *Writer
	reopen := func() {
		t.Helper()
		var err error
		shipper, err = replica.New(cfg, log.New(io.Discard, "", 0))
		if err != nil {
			t.Fatal(err)
		}
		w = f.writer(t, 1, shipper.Ledger())
	}
	reopen()
	read := func(path string) []byte {
		t.Helper()
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	restore := func(s Snapshot, raw []byte) {
		t.Helper()
		if err := os.WriteFile(s.Path, raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write := func() Snapshot {
		t.Helper()
		s, err := w.write("archived-retention")
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	remoteCopies := make(map[string][]byte)
	uploads := 0
	ship := func(want int) {
		t.Helper()
		before := connections.Load()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		n, err := shipper.Ship(ctx)
		if err != nil || n != want {
			t.Fatalf("Ship = %d, %v; want %d uploads", n, err, want)
		}
		uploads += n
		if want == 0 && connections.Load() != before {
			t.Fatal("duplicate or retirement pass contacted the replica")
		}
		if connections.Load() != int64(uploads) {
			t.Fatalf("replica connections = %d, want one per new upload (%d)", connections.Load(), uploads)
		}
	}
	assertRemote := func() {
		t.Helper()
		if got := listDir(t, remote); len(got) != len(remoteCopies) {
			t.Fatalf("remote files = %v, want %d published snapshots", got, len(remoteCopies))
		}
		for name, want := range remoteCopies {
			if !bytes.Equal(read(filepath.Join(remote, name)), want) {
				t.Fatalf("remote snapshot %s changed", name)
			}
		}
		if uploads != len(remoteCopies) || connections.Load() != int64(uploads) {
			t.Fatalf("uploads = %d, connections = %d, unique remote snapshots = %d", uploads, connections.Load(), len(remoteCopies))
		}
	}
	assertLocal := func(snaps ...Snapshot) {
		t.Helper()
		want := make([]string, 0, len(snaps))
		for _, s := range snaps {
			want = append(want, s.Name)
		}
		sort.Strings(want)
		if got := listDir(t, f.dir); !reflect.DeepEqual(got, want) {
			t.Fatalf("local snapshots = %v, want %v", got, want)
		}
	}
	retain := func(want ...string) {
		t.Helper()
		deleted, err := w.Retain()
		if err != nil || !reflect.DeepEqual(deleted, want) {
			t.Fatalf("Retain = %v, %v; want %v", deleted, err, want)
		}
	}
	assertActive := func(s Snapshot) {
		t.Helper()
		entries := shipper.Ledger().Entries()
		if len(entries) != 1 || entries[0].Name != s.Name {
			t.Fatalf("active ledger = %+v, want only %s", entries, s.Name)
		}
		if size := len(read(cfg.LedgerPath)); size > 512 {
			t.Fatalf("one-snapshot active ledger grew with retired history: %d bytes", size)
		}
	}

	unshipped, err := w.WriteSnapshot("never-shipped")
	if err != nil {
		t.Fatal(err)
	}
	unshippedRaw := read(unshipped.Path)
	if err := os.Remove(unshipped.Path); err != nil {
		t.Fatal(err)
	}
	a := write()
	aRaw := read(a.Path)
	ship(1)
	remoteCopies[a.Name] = aRaw
	aEntry, ok := shipper.Ledger().Lookup(a.Name)
	if !ok {
		t.Fatal("snapshot A was not actively acknowledged")
	}
	if err := shipper.Ledger().Record(aEntry); !errors.Is(err, replica.ErrLedger) {
		t.Fatalf("active duplicate acknowledgment accepted: %v", err)
	}
	b := write()
	bRaw := read(b.Path)
	ship(1)
	remoteCopies[b.Name] = bRaw
	activeBeforeArchive := read(cfg.LedgerPath)
	assertLocal(a, b)
	retain(a.Name)
	assertLocal(b)
	ship(0)
	assertActive(b)
	archived, ok, err := shipper.Ledger().Archived(a.Name)
	if err != nil || !ok || archived != aEntry {
		t.Fatalf("archived A = %+v, %v, %v; want %+v", archived, ok, err, aEntry)
	}
	for pass := 0; pass < 4; pass++ {
		if pass > 0 {
			reopen()
		}
		restore(a, aRaw)
		before := read(cfg.LedgerPath)
		ship(0)
		if pass >= 2 {
			reopen()
		}
		if _, ok := shipper.Ledger().Lookup(a.Name); ok {
			t.Fatal("recognizing archived A repopulated the active ledger")
		}
		if err := shipper.Ledger().Record(aEntry); !errors.Is(err, replica.ErrLedger) {
			t.Fatalf("archived duplicate acknowledgment accepted on pass %d: %v", pass, err)
		}
		retain(a.Name)
		assertLocal(b)
		retain()
		ship(0)
		assertActive(b)
		if !bytes.Equal(read(cfg.LedgerPath), before) {
			t.Fatal("recognition or retention rewrote the active ledger")
		}
		assertRemote()
	}

	const history = 64
	latest := b
	var validArchived Snapshot
	var validArchivedRaw []byte
	for i := 2; i < history; i++ {
		next := write()
		raw := read(next.Path)
		ship(1)
		remoteCopies[next.Name] = raw
		if entries := shipper.Ledger().Entries(); len(entries) != 2 {
			t.Fatalf("history %d: active entries = %d, want two locally present acknowledgments", i, len(entries))
		}
		retain(latest.Name)
		latest = next
		ship(0)
		assertLocal(latest)
		assertActive(latest)
		if i == 2 {
			validArchived, validArchivedRaw = next, raw
		}
		if i%8 == 0 {
			reopen()
			assertActive(latest)
		}
	}
	if uploads != history {
		t.Fatalf("history uploaded %d snapshots, want %d", uploads, history)
	}
	reopen()
	restore(a, aRaw)
	ship(0)
	reopen()
	retain(a.Name)
	assertActive(latest)
	assertLocal(latest)
	assertRemote()

	restore(unshipped, unshippedRaw)
	f.assertRestorable(t, unshipped.Path)
	retain()
	assertLocal(unshipped, latest)
	if _, ok := shipper.Ledger().Lookup(unshipped.Name); ok {
		t.Fatal("unshipped snapshot acquired an active acknowledgment")
	}
	if _, ok, err := shipper.Ledger().Archived(unshipped.Name); ok || err != nil {
		t.Fatalf("unshipped snapshot acquired archived evidence: %v, %v", ok, err)
	}
	changedDigest := append([]byte(nil), aRaw...)
	changedDigest[len(changedDigest)-1] ^= 1
	changedSize := append(append([]byte(nil), aRaw...), 0)
	for _, changed := range [][]byte{changedDigest, changedSize} {
		restore(a, changed)
		reopen()
		before := connections.Load()
		if n, err := shipper.Ship(context.Background()); n != 0 || !errors.Is(err, replica.ErrLedger) || !strings.Contains(err.Error(), a.Name) {
			t.Fatalf("changed archived snapshot shipped: count=%d error=%v", n, err)
		}
		if connections.Load() != before {
			t.Fatal("changed archived snapshot reached the remote transport")
		}
		deleted, err := w.Retain()
		if len(deleted) != 0 || err == nil || !strings.Contains(err.Error(), a.Name) || !strings.Contains(err.Error(), "differs from the shipped copy") {
			t.Fatalf("changed archived snapshot retention = %v, %v", deleted, err)
		}
		assertLocal(unshipped, a, latest)
		if !bytes.Equal(read(a.Path), changed) || !bytes.Equal(read(unshipped.Path), unshippedRaw) {
			t.Fatal("retention changed a protected local snapshot")
		}
		assertActive(latest)
		assertRemote()
	}

	restore(a, aRaw)
	restore(b, bRaw)
	restore(validArchived, validArchivedRaw)
	for _, s := range []Snapshot{a, b} {
		key := sha256.Sum256([]byte(s.Name))
		digest := hex.EncodeToString(key[:])
		archivePath := filepath.Join(cfg.LedgerPath+".archive", digest[:2], digest+".json")
		if err := os.WriteFile(archivePath, []byte("{"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	reopen()
	before := connections.Load()
	if n, err := shipper.Ship(context.Background()); n != 0 || !errors.Is(err, replica.ErrLedger) {
		t.Fatalf("invalid archive accepted by shipping: count=%d error=%v", n, err)
	}
	if connections.Load() != before {
		t.Fatal("invalid archive reached the remote transport")
	}
	deleted, err := w.Retain()
	if !reflect.DeepEqual(deleted, []string{validArchived.Name}) || !errors.Is(err, replica.ErrLedger) || !strings.Contains(err.Error(), a.Name) || !strings.Contains(err.Error(), b.Name) {
		t.Fatalf("invalid archives did not accumulate retention errors while retaining valid files: deleted=%v error=%v", deleted, err)
	}
	assertLocal(unshipped, a, b, latest)
	if !bytes.Equal(read(a.Path), aRaw) || !bytes.Equal(read(b.Path), bRaw) || !bytes.Equal(read(unshipped.Path), unshippedRaw) {
		t.Fatal("archive errors changed protected snapshots")
	}
	assertActive(latest)
	assertRemote()
	if err := os.WriteFile(cfg.LedgerPath, activeBeforeArchive, 0o600); err != nil {
		t.Fatal(err)
	}
	reopen()
	retain(a.Name, b.Name)
	assertLocal(unshipped, latest)
	assertRemote()
}

func retainReplicaConnections(t *testing.T, peer string) (string, *atomic.Int64) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	connections := new(atomic.Int64)
	var wg sync.WaitGroup
	wg.Add(1)
	t.Cleanup(func() { ln.Close(); wg.Wait() })
	go func() {
		defer wg.Done()
		for {
			local, err := ln.Accept()
			if err != nil {
				return
			}
			connections.Add(1)
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer local.Close()
				remote, err := net.DialTimeout("tcp", peer, 10*time.Second)
				if err != nil {
					t.Errorf("connect real replica: %v", err)
					return
				}
				defer remote.Close()
				_ = local.SetDeadline(time.Now().Add(15 * time.Second))
				_ = remote.SetDeadline(time.Now().Add(15 * time.Second))
				done := make(chan struct{})
				go func() {
					_, _ = io.Copy(remote, local)
					remote.Close()
					close(done)
				}()
				_, _ = io.Copy(local, remote)
				local.Close()
				<-done
			}()
		}
	}()
	return ln.Addr().String(), connections
}

func TestRestoreFileIntoEmptyDirectory(t *testing.T) {
	f := newFixture(t)
	w := f.writer(t, 5, nil)
	snap, err := w.WriteSnapshot("keys.addshare")
	if err != nil {
		t.Fatal(err)
	}
	digest := hex.EncodeToString(snap.SHA256[:])
	target := filepath.Join(t.TempDir(), "restored")
	res, err := RestoreFile(snap.Path, strings.ToUpper(digest), "node-1", f.backupKey, target, f.nodeKey)
	if err != nil {
		t.Fatal(err)
	}
	if res.Name != snap.Name || res.SHA256 != snap.SHA256 || res.Shares != 2 || res.AuditSequence != 1 {
		t.Fatalf("restore result %+v", res)
	}
	st, err := store.Open(target, f.nodeKey)
	if err != nil {
		t.Fatal(err)
	}
	want, err := f.store.List()
	if err != nil {
		t.Fatal(err)
	}
	got, err := st.List()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, got) {
		t.Fatalf("restored records differ:\nwant %+v\ngot  %+v", want, got)
	}
	for keyID, share := range f.shares {
		var plain []byte
		if err := st.WithShare(keyID, func(p []byte) error { plain = append(plain, p...); return nil }); err != nil || !bytes.Equal(plain, share) {
			t.Fatalf("restored share %s: %v", keyID, err)
		}
	}
	f.assertRecoveryState(t, st)
	st.Close()
	logged, err := os.ReadFile(filepath.Join(target, "audit", audit.FileName))
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range []string{RestoreEvent, "name=" + snap.Name, "sha256=" + digest, "node=node-1", "shares=2"} {
		if !bytes.Contains(logged, []byte(w)) {
			t.Fatalf("restore audit lacks %q", w)
		}
	}
}

func TestRestoreFileRefusals(t *testing.T) {
	f := newFixture(t)
	w := f.writer(t, 5, nil)
	snap, err := w.WriteSnapshot("keys.import")
	if err != nil {
		t.Fatal(err)
	}
	digest := hex.EncodeToString(snap.SHA256[:])
	fresh := func() string { return filepath.Join(t.TempDir(), "data") }
	assertEmpty := func(dir string) {
		t.Helper()
		entries, err := os.ReadDir(dir)
		if err == nil && len(entries) != 0 {
			t.Fatalf("refused restore left %d entries in %s", len(entries), dir)
		}
	}

	dir := fresh()
	if _, err := RestoreFile(snap.Path, strings.Repeat("00", 32), "node-1", f.backupKey, dir, f.nodeKey); !errors.Is(err, ErrDigestMismatch) {
		t.Fatalf("wrong digest: %v", err)
	}
	assertEmpty(dir)
	if _, err := RestoreFile(snap.Path, "abcd", "node-1", f.backupKey, fresh(), f.nodeKey); !errors.Is(err, ErrDigestFormat) {
		t.Fatalf("malformed digest: %v", err)
	}

	dir = fresh()
	if _, err := RestoreFile(snap.Path, digest, "node-2", f.backupKey, dir, f.nodeKey); !errors.Is(err, store.ErrSnapshotNode) {
		t.Fatalf("foreign node by name: %v", err)
	}
	assertEmpty(dir)
	neutral := filepath.Join(t.TempDir(), "copied.snap")
	raw, err := os.ReadFile(snap.Path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(neutral, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	dir = fresh()
	if _, err := RestoreFile(neutral, digest, "node-2", f.backupKey, dir, f.nodeKey); !errors.Is(err, store.ErrSnapshotNode) {
		t.Fatalf("foreign node by header: %v", err)
	}
	assertEmpty(dir)

	used := fresh()
	if err := os.MkdirAll(used, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(used, "shares.db"), []byte("existing"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := RestoreFile(snap.Path, digest, "node-1", f.backupKey, used, f.nodeKey); !errors.Is(err, store.ErrDataDirInUse) {
		t.Fatalf("non-empty directory: %v", err)
	}
	if existing, err := os.ReadFile(filepath.Join(used, "shares.db")); err != nil || string(existing) != "existing" {
		t.Fatal("refused restore changed the existing data directory")
	}

	dir = fresh()
	if _, err := RestoreFile(snap.Path, digest, "node-1", randomKey(t), dir, f.nodeKey); !errors.Is(err, store.ErrSnapshotAuth) {
		t.Fatalf("wrong backup key: %v", err)
	}
	assertEmpty(dir)

	tampered := append([]byte(nil), raw...)
	tampered[len(tampered)-1] ^= 0x01
	sum := sha256.Sum256(tampered)
	tamperedPath := filepath.Join(t.TempDir(), snap.Name)
	if err := os.WriteFile(tamperedPath, tampered, 0o600); err != nil {
		t.Fatal(err)
	}
	dir = fresh()
	if _, err := RestoreFile(tamperedPath, hex.EncodeToString(sum[:]), "node-1", f.backupKey, dir, f.nodeKey); !errors.Is(err, store.ErrSnapshotAuth) {
		t.Fatalf("tampered snapshot: %v", err)
	}
	assertEmpty(dir)
}
