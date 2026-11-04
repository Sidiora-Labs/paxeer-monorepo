package store_test

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"

	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/audit"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/auth/agent"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/backup"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/policy"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/policy/evm"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/server"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/store"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/transport"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/tss/dealershare"
)

const repair240Node = "node-1"
const repair240Account = "0x1111111111111111111111111111111111111111"

var repair240Participants = []string{"node-1", "node-2", "node-3", "node-4", "node-5"}

type repair240Fixture struct {
	st                 *store.Store
	log                *audit.Log
	nodeKey, backupKey []byte
	rec                store.ShareRecord
	plain              []byte
	identity           store.ImportIdentity
}

func repair240Bytes(t *testing.T, n int) []byte {
	t.Helper()
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return b
}

func repair240Open(t *testing.T, dir string, key []byte) *store.Store {
	t.Helper()
	st, err := store.Open(dir, key)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func repair240New(t *testing.T) *repair240Fixture {
	t.Helper()
	f := &repair240Fixture{nodeKey: repair240Bytes(t, store.KeySize), backupKey: repair240Bytes(t, store.KeySize)}
	f.st = repair240Open(t, t.TempDir(), f.nodeKey)
	var err error
	f.log, err = audit.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.log.Close() })
	// Use the real dealer and server codec, then the durable import producer.
	seed := repair240Bytes(t, ed25519.SeedSize)
	scalar, err := dealer.Ed25519ScalarFromSeed(seed)
	if err != nil {
		t.Fatal(err)
	}
	bundles, pub, err := dealer.Split(dealer.Ed25519, scalar, repair240Participants)
	if err != nil {
		t.Fatal(err)
	}
	publicKey, err := dealer.EncodeEd25519(pub)
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range bundles {
		defer dealer.Wipe(b.Share)
	}
	f.rec = store.ShareRecord{KeyID: "recovered-wallet", Curve: store.CurveEd25519, PublicKey: publicKey, Participants: repair240Participants}
	f.plain, err = json.Marshal(struct {
		Owner   string                 `json:"owner"`
		Account string                 `json:"account"`
		Bundle  server.ShareBundleJSON `json:"bundle"`
	}{Owner: "owner", Account: repair240Account, Bundle: server.EncodeBundle(bundles[0])})
	if err != nil {
		t.Fatal(err)
	}
	f.identity = store.ImportIdentity{CeremonyID: "ceremony", SessionID: "import-session", KeyID: f.rec.KeyID, Curve: f.rec.Curve, PublicKey: publicKey, Participants: repair240Participants, Threshold: 3, Owner: "owner", Account: repair240Account}
	if _, err := f.st.Import(f.rec, f.plain, f.identity); err != nil {
		t.Fatal(err)
	}
	entry, err := f.log.Append(audit.Entry{Kind: "keys.import", Subject: []byte(f.rec.KeyID), Decision: "allowed"})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.st.RecordImportAudit(f.rec.KeyID, f.identity.CeremonyID, entry.Sequence); err != nil {
		t.Fatal(err)
	}
	return f
}

func (f *repair240Fixture) stage(t *testing.T, decision string) store.CeremonyRefresh {
	t.Helper()
	state := store.CeremonyRefresh{CeremonyID: f.identity.CeremonyID, ImportSessionID: f.identity.SessionID, SessionID: "refresh-session", KeyID: f.rec.KeyID, Curve: f.rec.Curve, PublicKey: f.rec.PublicKey, Participants: repair240Participants, State: "running"}
	if err := f.st.SaveCeremonyRefresh(state); err != nil {
		t.Fatal(err)
	}
	next := f.rec
	next.Epoch++
	if err := f.st.PutStaged(next, f.plain); err != nil {
		t.Fatal(err)
	}
	state.State = "staged"
	if err := f.st.SaveCeremonyRefresh(state); err != nil {
		t.Fatal(err)
	}
	if decision != "" {
		if err := f.st.SetCeremonyRefreshDecision(f.rec.KeyID, state.SessionID, 1, decision); err != nil {
			t.Fatal(err)
		}
		state.Decision = decision
	}
	return state
}

func (f *repair240Fixture) snapshot(t *testing.T) backup.Snapshot {
	t.Helper()
	writer, err := backup.New(backup.Config{NodeID: repair240Node, Dir: t.TempDir(), BackupKey: f.backupKey, Retain: 1, Store: f.st, Audit: f.log})
	if err != nil {
		t.Fatal(err)
	}
	snap, err := writer.WriteSnapshot("repair-240")
	if err != nil {
		t.Fatal(err)
	}
	return snap
}

func (f *repair240Fixture) restore(t *testing.T, snap backup.Snapshot) *store.Store {
	t.Helper()
	if err := f.st.Close(); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "restored")
	result, err := backup.RestoreFile(snap.Path, hex.EncodeToString(snap.SHA256[:]), repair240Node, f.backupKey, dir, f.nodeKey)
	if err != nil {
		t.Fatal(err)
	}
	if result.Shares != 1 || result.AuditSequence == 0 {
		t.Fatalf("restore result: %+v", result)
	}
	// RestoreFile closes its store; the production consumer opens it anew.
	return repair240Open(t, dir, f.nodeKey)
}

func repair240Policy(t *testing.T, rate uint32) *policy.Policy {
	t.Helper()
	chain := uint64(125)
	doc := policy.Document{Version: policy.Version, Defaults: policy.Rules{ChainID: &chain, Kinds: []string{policy.KindEVMTransaction}, RatePerMinute: &rate, Caps: map[string]policy.Cap{policy.AssetNative: {PerTransaction: "10", Daily: "10"}}}}
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := policy.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return policy.New(parsed)
}

func repair240Request(t *testing.T, amount int64) policy.Request {
	t.Helper()
	to := common.HexToAddress("0x2222222222222222222222222222222222222222")
	tx := types.NewTx(&types.DynamicFeeTx{ChainID: big.NewInt(125), GasTipCap: big.NewInt(1), GasFeeCap: big.NewInt(2), Gas: 21000, To: &to, Value: big.NewInt(amount)})
	raw, err := tx.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	view, err := evm.DecodeTransaction(raw, big.NewInt(125))
	if err != nil {
		t.Fatal(err)
	}
	return policy.Request{Kind: policy.KindEVMTransaction, View: view}
}

func TestRepair_240_SpendAndRateLimitsSurviveBackupRestart(t *testing.T) {
	f := repair240New(t)
	now := time.Now()
	clock := func() time.Time { return now }
	ledger, err := policy.NewSpendLedger(f.st, clock)
	if err != nil {
		t.Fatal(err)
	}
	p := repair240Policy(t, 1)
	if got := p.Evaluate(repair240Account, repair240Request(t, 8), ledger.ForRequest("first")); !got.Allowed {
		t.Fatalf("initial spend: %+v", got)
	}
	restored := f.restore(t, f.snapshot(t))
	recovered, err := policy.NewSpendLedger(restored, clock)
	if err != nil {
		t.Fatal(err)
	}
	if got := p.Evaluate(repair240Account, repair240Request(t, 3), recovered.ForRequest("second")); got.Allowed || got.Code != policy.CodeRateLimited {
		t.Fatalf("recovered rate cap: %+v", got)
	}
	now = now.Add(2 * policy.RateWindow)
	if got := p.Evaluate(repair240Account, repair240Request(t, 3), recovered.ForRequest("third")); got.Allowed || got.Code != policy.CodeDailyCap {
		t.Fatalf("recovered daily cap: %+v", got)
	}
	spent, err := recovered.Spent(policy.AccountKey(repair240Account), policy.AssetNative, now.Add(-policy.SpendWindow))
	if err != nil || spent.Cmp(big.NewInt(8)) != 0 {
		t.Fatalf("recovered spend = %v, %v", spent, err)
	}
	// The original request remains idempotent and usage expires on its original window.
	if err := recovered.Apply(policy.AccountKey(repair240Account), "first", []policy.Spend{{Asset: policy.AssetNative, Amount: big.NewInt(8)}}, now); err != nil {
		t.Fatal(err)
	}
	spent, err = recovered.Spent(policy.AccountKey(repair240Account), policy.AssetNative, now.Add(-policy.SpendWindow))
	if err != nil || spent.Cmp(big.NewInt(8)) != 0 {
		t.Fatalf("duplicate spend = %v, %v", spent, err)
	}
	now = now.Add(policy.SpendWindow)
	if got := p.Evaluate(repair240Account, repair240Request(t, 3), recovered.ForRequest("fourth")); !got.Allowed {
		t.Fatalf("expired window: %+v", got)
	}
}

func TestRepair_240_ReplayAndOpaqueAuthorityRecordsSurviveBackupRestart(t *testing.T) {
	f := repair240New(t)
	var token, request, pub [32]byte
	var nonce [16]byte
	copy(token[:], repair240Bytes(t, 32))
	copy(request[:], repair240Bytes(t, 32))
	copy(pub[:], repair240Bytes(t, 32))
	copy(nonce[:], repair240Bytes(t, 16))
	expires := time.Now().Add(time.Hour)
	if fresh, err := f.st.UseTokenRequest(token, request, expires); err != nil || !fresh {
		t.Fatalf("first token use: %v %v", fresh, err)
	}
	if fresh, err := f.st.UseNonce(pub, nonce, expires); err != nil || !fresh {
		t.Fatalf("first nonce use: %v %v", fresh, err)
	}
	// PersistSigned is the production monotonic authority-record writer.
	if err := agent.PersistSigned(f.st, "authority", "42", "signed-authority-token"); err != nil {
		t.Fatal(err)
	}
	opaque := []byte{0, 255, 1, 128, 0}
	if err := f.st.UpdateRecord(store.RecordKind("future-durable-kind"), "binary-id", func([]byte) ([]byte, error) { return bytes.Clone(opaque), nil }); err != nil {
		t.Fatal(err)
	}
	restored := f.restore(t, f.snapshot(t))
	if fresh, err := restored.UseTokenRequest(token, request, expires); err != nil || fresh {
		t.Fatalf("replayed token admitted: %v %v", fresh, err)
	}
	if fresh, err := restored.UseNonce(pub, nonce, expires); err != nil || fresh {
		t.Fatalf("replayed nonce admitted: %v %v", fresh, err)
	}
	request[0] ^= 1
	if fresh, err := restored.UseTokenRequest(token, request, expires); err != nil || !fresh {
		t.Fatalf("distinct request refused: %v %v", fresh, err)
	}
	if err := agent.PersistSigned(restored, "authority", "41", "stale-authority-token"); !errors.Is(err, agent.ErrAuthority) {
		t.Fatalf("authority sequence rollback admitted: %v", err)
	}
	if err := restored.WithRecord(store.RecordKind("future-durable-kind"), "binary-id", func(raw []byte) error {
		if !bytes.Equal(raw, opaque) {
			t.Fatalf("opaque record changed: %x", raw)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// Start the actual server with a real transport and mutually authenticated TLS.
// No peer protocol is invoked by describe; recovery runs in server.New/initPeer.
func repair240Server(t *testing.T, st *store.Store, lg *audit.Log) (*http.Client, string) {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: repair240Node}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}}
	der, err := x509.CreateCertificate(rand.Reader, template, template, public, private)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		t.Fatal(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	dir := t.TempDir()
	certPath, keyPath := filepath.Join(dir, "node.pem"), filepath.Join(dir, "node.key")
	if err := os.WriteFile(certPath, certPEM, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0600); err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	pin := transport.SPKIHash(cert)
	tr, err := transport.New(transport.Config{SelfID: repair240Node, CertFile: certPath, KeyFile: keyPath, CAFile: certPath, Peers: []transport.Peer{{ID: repair240Node, Address: "https://127.0.0.1", SPKISHA256: hex.EncodeToString(pin[:])}}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tr.Close() })
	clients, err := server.LoadClientAuthorities(certPath, certPath)
	if err != nil {
		t.Fatal(err)
	}
	ledger, err := policy.NewSpendLedger(st, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	srv, err := server.New(server.Options{NodeID: repair240Node, ChainID: 125, Ceremony: true, Participants: repair240Participants, Store: st, Audit: lg, Transport: tr, Policy: repair240Policy(t, 100), Ledger: ledger, Clients: clients})
	if err != nil {
		t.Fatal(err)
	}
	pair, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(cert)
	https := httptest.NewUnstartedServer(srv.Handler())
	https.TLS = &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{pair}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: roots}
	https.StartTLS()
	t.Cleanup(https.Close)
	client := &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: roots, Certificates: []tls.Certificate{pair}}}}
	t.Cleanup(client.CloseIdleConnections)
	return client, https.URL
}

func repair240Describe(t *testing.T, client *http.Client, url, key string) server.DescribeResponse {
	t.Helper()
	raw, err := json.Marshal(server.DescribeRequest{SessionID: "describe-recovery", KeyID: key})
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.Post(url+server.PathDescribe, "application/json", bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("describe status %d: %s", response.StatusCode, body)
	}
	var described server.DescribeResponse
	if err := json.Unmarshal(body, &described); err != nil {
		t.Fatal(err)
	}
	return described
}

func TestRepair_240_InterruptedCeremonyRecoversThroughRestoredServer(t *testing.T) {
	for _, decision := range []string{"", "commit", "abort"} {
		t.Run("decision="+decision, func(t *testing.T) {
			f := repair240New(t)
			state := f.stage(t, decision)
			before, err := f.st.ImportReceipt(f.rec.KeyID)
			if err != nil {
				t.Fatal(err)
			}
			restored := f.restore(t, f.snapshot(t))
			client, url := repair240Server(t, restored, f.log)
			described := repair240Describe(t, client, url, f.rec.KeyID)
			wantState := "staged"
			if decision == "abort" {
				wantState = "aborted"
			}
			if described.CeremonyID != f.identity.CeremonyID || described.ImportSessionID != f.identity.SessionID || described.ImportDigest != hex.EncodeToString(before.Identity.MaterialDigest[:]) || described.RefreshSessionID != state.SessionID || described.RefreshState != wantState || described.Epoch != 0 || described.Owner != f.identity.Owner || described.Account != f.identity.Account {
				t.Fatalf("restored ceremony: %+v", described)
			}
			after, err := restored.ImportReceipt(f.rec.KeyID)
			if err != nil || !reflect.DeepEqual(before, after) {
				t.Fatalf("import receipt changed: %+v %v", after, err)
			}
			retried, err := restored.Import(f.rec, f.plain, f.identity)
			if err != nil || !reflect.DeepEqual(before, retried) {
				t.Fatalf("import retry lost identity/audit: %+v %v", retried, err)
			}
			conflict := f.identity
			conflict.SessionID = "conflicting-import"
			if _, err := restored.Import(f.rec, f.plain, conflict); !errors.Is(err, store.ErrInvalid) {
				t.Fatalf("conflicting import admitted: %v", err)
			}
			if decision == "abort" {
				if _, err := restored.GetStaged(f.rec.KeyID); !errors.Is(err, store.ErrNotFound) {
					t.Fatalf("aborted stage survived startup: %v", err)
				}
				if err := restored.SetCeremonyRefreshDecision(f.rec.KeyID, state.SessionID, 1, "commit"); !errors.Is(err, store.ErrInvalid) {
					t.Fatalf("abort flipped to commit: %v", err)
				}
			} else {
				stage, err := restored.GetStaged(f.rec.KeyID)
				if err != nil || stage.Epoch != 1 {
					t.Fatalf("recoverable stage lost at startup: %+v %v", stage, err)
				}
				if err := restored.SetCeremonyRefreshDecision(f.rec.KeyID, state.SessionID, 1, "commit"); err != nil {
					t.Fatal(err)
				}
				if err := restored.CommitStaged(f.rec.KeyID, 1); err != nil {
					t.Fatal(err)
				}
				if err := restored.WithShare(f.rec.KeyID, func(raw []byte) error {
					if !bytes.Equal(raw, f.plain) {
						t.Fatal("committed stage material differs")
					}
					return nil
				}); err != nil {
					t.Fatal(err)
				}
				if got := repair240Describe(t, client, url, f.rec.KeyID); got.Epoch != 1 {
					t.Fatalf("recovered commit epoch: %+v", got)
				}
			}
		})
	}
}

type repair240Entry struct {
	Key   []byte `json:"key"`
	Value []byte `json:"value"`
}
type repair240Bucket struct {
	Name    string           `json:"name"`
	Entries []repair240Entry `json:"entries"`
}
type repair240Body struct {
	Buckets []repair240Bucket `json:"buckets"`
}

// Negative tests edit authenticated output from the real producer. They never
// substitute a transport or store implementation for the component under test.
func repair240Envelope(t *testing.T, blob, backupKey []byte, mutate func(*repair240Body)) []byte {
	t.Helper()
	hl := 3 + int(binary.BigEndian.Uint16(blob[1:3]))
	header := blob[:hl]
	const domain = "paxeer-attestor-backup-v3"
	key, err := hkdf.Key(sha256.New, backupKey, nil, domain, store.KeySize)
	if err != nil {
		t.Fatal(err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	ad := append(bytes.Clone(header), domain...)
	plain, err := aead.Open(nil, blob[hl:hl+store.NonceSize], blob[hl+store.NonceSize:], ad)
	if err != nil {
		t.Fatal(err)
	}
	var body repair240Body
	if err := json.Unmarshal(plain, &body); err != nil {
		t.Fatal(err)
	}
	mutate(&body)
	plain, err = json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	nonce := repair240Bytes(t, store.NonceSize)
	out := append(bytes.Clone(header), nonce...)
	return aead.Seal(out, nonce, plain, ad)
}

func TestRepair_240_RestoreRejectsIncompleteOrInconsistentStateAtomically(t *testing.T) {
	f := repair240New(t)
	f.stage(t, "commit")
	var pub [32]byte
	var nonce [16]byte
	if fresh, err := f.st.UseNonce(pub, nonce, time.Now().Add(time.Hour)); err != nil || !fresh {
		t.Fatalf("nonce: %v %v", fresh, err)
	}
	snap := f.snapshot(t)
	blob, err := os.ReadFile(snap.Path)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		mutate func(*repair240Body)
	}{
		{"missing-manifest-bucket", func(b *repair240Body) { b.Buckets = b.Buckets[:3] }},
		{"duplicate-bucket", func(b *repair240Body) { b.Buckets[3] = b.Buckets[0] }},
		{"unknown-bucket", func(b *repair240Body) { b.Buckets[3].Name = "unrecognized" }},
		{"duplicate-entry", func(b *repair240Body) { b.Buckets[0].Entries = append(b.Buckets[0].Entries, b.Buckets[0].Entries[0]) }},
		{"orphaned-ceremony-and-stage", func(b *repair240Body) { b.Buckets[0].Entries = []repair240Entry{} }},
		{"lost-committed-stage", func(b *repair240Body) { b.Buckets[1].Entries = []repair240Entry{} }},
		{"missing-import-receipt", func(b *repair240Body) {
			entries := b.Buckets[2].Entries[:0]
			for _, e := range b.Buckets[2].Entries {
				if !bytes.HasPrefix(e.Key, []byte(string(store.RecordCeremony)+"\x00")) {
					entries = append(entries, e)
				}
			}
			b.Buckets[2].Entries = entries
		}},
		{"misbound-stage-key", func(b *repair240Body) { b.Buckets[1].Entries[0].Key = []byte("another-key") }},
		{"unauthenticated-stage", func(b *repair240Body) {
			var rec store.ShareRecord
			if err := json.Unmarshal(b.Buckets[1].Entries[0].Value, &rec); err != nil {
				t.Fatal(err)
			}
			rec.Ciphertext[store.NonceSize] ^= 1
			var err error
			b.Buckets[1].Entries[0].Value, err = json.Marshal(rec)
			if err != nil {
				t.Fatal(err)
			}
		}},
		{"unauthenticated-record", func(b *repair240Body) { b.Buckets[2].Entries[0].Value[store.NonceSize] ^= 1 }},
		{"malformed-replay", func(b *repair240Body) { b.Buckets[3].Entries[0].Value = []byte{1} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bad := repair240Envelope(t, blob, f.backupKey, tc.mutate)
			dir := filepath.Join(t.TempDir(), "candidate")
			st, err := store.Restore(bytes.NewReader(bad), f.backupKey, repair240Node, dir, f.nodeKey)
			if err == nil {
				_ = st.Close()
				t.Fatal("invalid snapshot admitted a signing store")
			}
			if !errors.Is(err, store.ErrSnapshotFormat) && !errors.Is(err, store.ErrInvalid) && !errors.Is(err, store.ErrAuthFailed) {
				t.Fatalf("unexpected refusal: %v", err)
			}
			if _, err := os.Stat(filepath.Join(dir, store.FileName)); !os.IsNotExist(err) {
				t.Fatalf("failed transaction left database: %v", err)
			}
			// A failed candidate must not prevent a subsequent complete recovery.
			st, err = store.Restore(bytes.NewReader(blob), f.backupKey, repair240Node, dir, f.nodeKey)
			if err != nil {
				t.Fatalf("valid retry after rollback: %v", err)
			}
			defer st.Close()
			if stage, err := st.GetStaged(f.rec.KeyID); err != nil || stage.Epoch != 1 {
				t.Fatalf("retry stage: %+v %v", stage, err)
			}
		})
	}
}

func TestRepair_240_SnapshotVersionAuthenticationAndEmptyLifecycle(t *testing.T) {
	key, backupKey := repair240Bytes(t, store.KeySize), repair240Bytes(t, store.KeySize)
	st := repair240Open(t, t.TempDir(), key)
	var out bytes.Buffer
	if err := st.Snapshot(&out, backupKey, repair240Node); err != nil {
		t.Fatal(err)
	}
	blob := bytes.Clone(out.Bytes())
	if blob[0] != 3 || store.SnapshotVersion != 3 {
		t.Fatalf("full-lifecycle format version: %d", blob[0])
	}
	if owner, err := store.SnapshotNodeID(blob); err != nil || owner != repair240Node {
		t.Fatalf("owner: %q %v", owner, err)
	}
	recovered, err := store.Restore(bytes.NewReader(blob), backupKey, repair240Node, t.TempDir(), key)
	if err != nil {
		t.Fatal(err)
	}
	if keys, err := recovered.List(); err != nil || len(keys) != 0 {
		t.Fatalf("empty lifecycle: %+v %v", keys, err)
	}
	if err := recovered.Close(); err != nil {
		t.Fatal(err)
	}
	for _, version := range []byte{1, 2} {
		old := bytes.Clone(blob)
		old[0] = version
		if _, err := store.Restore(bytes.NewReader(old), backupKey, repair240Node, t.TempDir(), key); !errors.Is(err, store.ErrSnapshotVersion) {
			t.Fatalf("unsafe legacy format %d: %v", version, err)
		}
	}
	for _, pos := range []int{3 + len(repair240Node), len(blob) - 1} {
		bad := bytes.Clone(blob)
		bad[pos] ^= 1
		if _, err := store.Restore(bytes.NewReader(bad), backupKey, repair240Node, t.TempDir(), key); !errors.Is(err, store.ErrSnapshotAuth) {
			t.Fatalf("tampered envelope: %v", err)
		}
	}
	if _, err := store.Restore(bytes.NewReader(blob), backupKey, "node-2", t.TempDir(), key); !errors.Is(err, store.ErrSnapshotNode) {
		t.Fatalf("foreign node: %v", err)
	}
}

func TestRepair_240_SnapshotRefusesInconsistentCeremony(t *testing.T) {
	f := repair240New(t)
	f.stage(t, "commit")
	// Simulate a key lost after a durable commit decision. The writer must fail
	// closed rather than produce an apparently usable partial recovery point.
	if err := f.st.Delete(f.rec.KeyID); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := f.st.Snapshot(&out, f.backupKey, repair240Node); !errors.Is(err, store.ErrSnapshotFormat) {
		t.Fatalf("orphaned ceremony snapshot: %v", err)
	}
	if out.Len() != 0 {
		t.Fatal("invalid lifecycle snapshot emitted bytes")
	}
}
