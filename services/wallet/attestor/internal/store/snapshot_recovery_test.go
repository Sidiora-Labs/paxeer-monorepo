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

const snapshotRecoveryNode = "node-1"
const snapshotRecoveryAccount = "0x1111111111111111111111111111111111111111"

var snapshotRecoveryParticipants = []string{"node-1", "node-2", "node-3", "node-4", "node-5"}

type snapshotRecoveryFixture struct {
	st                 *store.Store
	log                *audit.Log
	nodeKey, backupKey []byte
	rec                store.ShareRecord
	plain              []byte
	identity           store.ImportIdentity
}

func snapshotRecoveryBytes(t *testing.T, n int) []byte {
	t.Helper()
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return b
}

func snapshotRecoveryOpen(t *testing.T, dir string, key []byte) *store.Store {
	t.Helper()
	st, err := store.Open(dir, key)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func snapshotRecoveryNew(t *testing.T) *snapshotRecoveryFixture {
	t.Helper()
	f := &snapshotRecoveryFixture{nodeKey: snapshotRecoveryBytes(t, store.KeySize), backupKey: snapshotRecoveryBytes(t, store.KeySize)}
	f.st = snapshotRecoveryOpen(t, t.TempDir(), f.nodeKey)
	var err error
	f.log, err = audit.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.log.Close() })
	// Use the real dealer and server codec, then the durable import producer.
	seed := snapshotRecoveryBytes(t, ed25519.SeedSize)
	scalar, err := dealer.Ed25519ScalarFromSeed(seed)
	if err != nil {
		t.Fatal(err)
	}
	bundles, pub, err := dealer.Split(dealer.Ed25519, scalar, snapshotRecoveryParticipants)
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
	f.rec = store.ShareRecord{KeyID: "recovered-wallet", Curve: store.CurveEd25519, PublicKey: publicKey, Participants: snapshotRecoveryParticipants}
	f.plain, err = json.Marshal(struct {
		Owner   string                 `json:"owner"`
		Account string                 `json:"account"`
		Bundle  server.ShareBundleJSON `json:"bundle"`
	}{Owner: "owner", Account: snapshotRecoveryAccount, Bundle: server.EncodeBundle(bundles[0])})
	if err != nil {
		t.Fatal(err)
	}
	f.identity = store.ImportIdentity{CeremonyID: "ceremony", SessionID: "import-session", KeyID: f.rec.KeyID, Curve: f.rec.Curve, PublicKey: publicKey, Participants: snapshotRecoveryParticipants, Threshold: 3, Owner: "owner", Account: snapshotRecoveryAccount}
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

func (f *snapshotRecoveryFixture) stage(t *testing.T, decision string) store.CeremonyRefresh {
	t.Helper()
	state := store.CeremonyRefresh{CeremonyID: f.identity.CeremonyID, ImportSessionID: f.identity.SessionID, SessionID: "refresh-session", KeyID: f.rec.KeyID, Curve: f.rec.Curve, PublicKey: f.rec.PublicKey, Participants: snapshotRecoveryParticipants, State: "running"}
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

func (f *snapshotRecoveryFixture) snapshot(t *testing.T) backup.Snapshot {
	t.Helper()
	writer, err := backup.New(backup.Config{NodeID: snapshotRecoveryNode, Dir: t.TempDir(), BackupKey: f.backupKey, Retain: 1, Store: f.st, Audit: f.log})
	if err != nil {
		t.Fatal(err)
	}
	snap, err := writer.WriteSnapshot("repair-240")
	if err != nil {
		t.Fatal(err)
	}
	return snap
}

func (f *snapshotRecoveryFixture) restore(t *testing.T, snap backup.Snapshot) *store.Store {
	t.Helper()
	if err := f.st.Close(); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "restored")
	result, err := backup.RestoreFile(snap.Path, hex.EncodeToString(snap.SHA256[:]), snapshotRecoveryNode, f.backupKey, dir, f.nodeKey)
	if err != nil {
		t.Fatal(err)
	}
	if result.Shares != 1 || result.AuditSequence == 0 {
		t.Fatalf("restore result: %+v", result)
	}
	// RestoreFile closes its store; the production consumer opens it anew.
	return snapshotRecoveryOpen(t, dir, f.nodeKey)
}

func snapshotRecoveryPolicy(t *testing.T, rate uint32) *policy.Policy {
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

func snapshotRecoveryRequest(t *testing.T, amount int64) policy.Request {
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

func TestSnapshotRecoverySpendAndRateLimitsSurviveBackupRestart(t *testing.T) {
	f := snapshotRecoveryNew(t)
	now := time.Now()
	clock := func() time.Time { return now }
	ledger, err := policy.NewSpendLedger(f.st, clock)
	if err != nil {
		t.Fatal(err)
	}
	p := snapshotRecoveryPolicy(t, 1)
	if got := p.Evaluate(snapshotRecoveryAccount, snapshotRecoveryRequest(t, 8), ledger.ForRequest("first")); !got.Allowed {
		t.Fatalf("initial spend: %+v", got)
	}
	restored := f.restore(t, f.snapshot(t))
	recovered, err := policy.NewSpendLedger(restored, clock)
	if err != nil {
		t.Fatal(err)
	}
	if got := p.Evaluate(snapshotRecoveryAccount, snapshotRecoveryRequest(t, 3), recovered.ForRequest("second")); got.Allowed || got.Code != policy.CodeRateLimited {
		t.Fatalf("recovered rate cap: %+v", got)
	}
	now = now.Add(2 * policy.RateWindow)
	if got := p.Evaluate(snapshotRecoveryAccount, snapshotRecoveryRequest(t, 3), recovered.ForRequest("third")); got.Allowed || got.Code != policy.CodeDailyCap {
		t.Fatalf("recovered daily cap: %+v", got)
	}
	spent, err := recovered.Spent(policy.AccountKey(snapshotRecoveryAccount), policy.AssetNative, now.Add(-policy.SpendWindow))
	if err != nil || spent.Cmp(big.NewInt(8)) != 0 {
		t.Fatalf("recovered spend = %v, %v", spent, err)
	}
	// The original request remains idempotent and usage expires on its original window.
	if err := recovered.Apply(policy.AccountKey(snapshotRecoveryAccount), "first", []policy.Spend{{Asset: policy.AssetNative, Amount: big.NewInt(8)}}, now); err != nil {
		t.Fatal(err)
	}
	spent, err = recovered.Spent(policy.AccountKey(snapshotRecoveryAccount), policy.AssetNative, now.Add(-policy.SpendWindow))
	if err != nil || spent.Cmp(big.NewInt(8)) != 0 {
		t.Fatalf("duplicate spend = %v, %v", spent, err)
	}
	now = now.Add(policy.SpendWindow)
	if got := p.Evaluate(snapshotRecoveryAccount, snapshotRecoveryRequest(t, 3), recovered.ForRequest("fourth")); !got.Allowed {
		t.Fatalf("expired window: %+v", got)
	}
}

func TestSnapshotRecoveryReplayAndOpaqueAuthorityRecordsSurviveBackupRestart(t *testing.T) {
	f := snapshotRecoveryNew(t)
	var token, request, pub [32]byte
	var nonce [16]byte
	copy(token[:], snapshotRecoveryBytes(t, 32))
	copy(request[:], snapshotRecoveryBytes(t, 32))
	copy(pub[:], snapshotRecoveryBytes(t, 32))
	copy(nonce[:], snapshotRecoveryBytes(t, 16))
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
func snapshotRecoveryServer(t *testing.T, st *store.Store, lg *audit.Log) (*http.Client, string) {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: snapshotRecoveryNode}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}}
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
	tr, err := transport.New(transport.Config{SelfID: snapshotRecoveryNode, CertFile: certPath, KeyFile: keyPath, CAFile: certPath, Peers: []transport.Peer{{ID: snapshotRecoveryNode, Address: "https://127.0.0.1", SPKISHA256: hex.EncodeToString(pin[:])}}})
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
	srv, err := server.New(server.Options{NodeID: snapshotRecoveryNode, ChainID: 125, Ceremony: true, Participants: snapshotRecoveryParticipants, Store: st, Audit: lg, Transport: tr, Policy: snapshotRecoveryPolicy(t, 100), Ledger: ledger, Clients: clients})
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

func snapshotRecoveryDescribe(t *testing.T, client *http.Client, url, key string) server.DescribeResponse {
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

func TestSnapshotRecoveryInterruptedCeremonyRecoversThroughRestoredServer(t *testing.T) {
	for _, decision := range []string{"", "commit", "abort"} {
		t.Run("decision="+decision, func(t *testing.T) {
			f := snapshotRecoveryNew(t)
			state := f.stage(t, decision)
			before, err := f.st.ImportReceipt(f.rec.KeyID)
			if err != nil {
				t.Fatal(err)
			}
			restored := f.restore(t, f.snapshot(t))
			client, url := snapshotRecoveryServer(t, restored, f.log)
			described := snapshotRecoveryDescribe(t, client, url, f.rec.KeyID)
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
				if got := snapshotRecoveryDescribe(t, client, url, f.rec.KeyID); got.Epoch != 1 {
					t.Fatalf("recovered commit epoch: %+v", got)
				}
			}
		})
	}
}

type snapshotRecoveryEntry struct {
	Key   []byte `json:"key"`
	Value []byte `json:"value"`
}
type snapshotRecoveryBucket struct {
	Name    string                  `json:"name"`
	Entries []snapshotRecoveryEntry `json:"entries"`
}
type snapshotRecoveryBody struct {
	Buckets []snapshotRecoveryBucket `json:"buckets"`
}

// Negative tests edit authenticated output from the real producer. They never
// substitute a transport or store implementation for the component under test.
func snapshotRecoveryEnvelope(t *testing.T, blob, backupKey []byte, mutate func(*snapshotRecoveryBody)) []byte {
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
	var body snapshotRecoveryBody
	if err := json.Unmarshal(plain, &body); err != nil {
		t.Fatal(err)
	}
	mutate(&body)
	plain, err = json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	nonce := snapshotRecoveryBytes(t, store.NonceSize)
	out := append(bytes.Clone(header), nonce...)
	return aead.Seal(out, nonce, plain, ad)
}

func TestSnapshotRecoveryRestoreRejectsIncompleteOrInconsistentStateAtomically(t *testing.T) {
	f := snapshotRecoveryNew(t)
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
		mutate func(*snapshotRecoveryBody)
	}{
		{"missing-manifest-bucket", func(b *snapshotRecoveryBody) { b.Buckets = b.Buckets[:3] }},
		{"duplicate-bucket", func(b *snapshotRecoveryBody) { b.Buckets[3] = b.Buckets[0] }},
		{"unknown-bucket", func(b *snapshotRecoveryBody) { b.Buckets[3].Name = "unrecognized" }},
		{"duplicate-entry", func(b *snapshotRecoveryBody) {
			b.Buckets[0].Entries = append(b.Buckets[0].Entries, b.Buckets[0].Entries[0])
		}},
		{"orphaned-ceremony-and-stage", func(b *snapshotRecoveryBody) { b.Buckets[0].Entries = []snapshotRecoveryEntry{} }},
		{"lost-committed-stage", func(b *snapshotRecoveryBody) { b.Buckets[1].Entries = []snapshotRecoveryEntry{} }},
		{"missing-import-receipt", func(b *snapshotRecoveryBody) {
			entries := b.Buckets[2].Entries[:0]
			for _, e := range b.Buckets[2].Entries {
				if !bytes.HasPrefix(e.Key, []byte(string(store.RecordCeremony)+"\x00")) {
					entries = append(entries, e)
				}
			}
			b.Buckets[2].Entries = entries
		}},
		{"misbound-stage-key", func(b *snapshotRecoveryBody) { b.Buckets[1].Entries[0].Key = []byte("another-key") }},
		{"unauthenticated-stage", func(b *snapshotRecoveryBody) {
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
		{"unauthenticated-record", func(b *snapshotRecoveryBody) { b.Buckets[2].Entries[0].Value[store.NonceSize] ^= 1 }},
		{"malformed-replay", func(b *snapshotRecoveryBody) { b.Buckets[3].Entries[0].Value = []byte{1} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bad := snapshotRecoveryEnvelope(t, blob, f.backupKey, tc.mutate)
			dir := filepath.Join(t.TempDir(), "candidate")
			st, err := store.Restore(bytes.NewReader(bad), f.backupKey, snapshotRecoveryNode, dir, f.nodeKey)
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
			st, err = store.Restore(bytes.NewReader(blob), f.backupKey, snapshotRecoveryNode, dir, f.nodeKey)
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

func TestSnapshotRecoveryVersionAuthenticationAndEmptyLifecycle(t *testing.T) {
	key, backupKey := snapshotRecoveryBytes(t, store.KeySize), snapshotRecoveryBytes(t, store.KeySize)
	st := snapshotRecoveryOpen(t, t.TempDir(), key)
	var out bytes.Buffer
	if err := st.Snapshot(&out, backupKey, snapshotRecoveryNode); err != nil {
		t.Fatal(err)
	}
	blob := bytes.Clone(out.Bytes())
	if blob[0] != 3 || store.SnapshotVersion != 3 {
		t.Fatalf("full-lifecycle format version: %d", blob[0])
	}
	if owner, err := store.SnapshotNodeID(blob); err != nil || owner != snapshotRecoveryNode {
		t.Fatalf("owner: %q %v", owner, err)
	}
	recovered, err := store.Restore(bytes.NewReader(blob), backupKey, snapshotRecoveryNode, t.TempDir(), key)
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
		if _, err := store.Restore(bytes.NewReader(old), backupKey, snapshotRecoveryNode, t.TempDir(), key); !errors.Is(err, store.ErrSnapshotVersion) {
			t.Fatalf("unsafe legacy format %d: %v", version, err)
		}
	}
	for _, pos := range []int{3 + len(snapshotRecoveryNode), len(blob) - 1} {
		bad := bytes.Clone(blob)
		bad[pos] ^= 1
		if _, err := store.Restore(bytes.NewReader(bad), backupKey, snapshotRecoveryNode, t.TempDir(), key); !errors.Is(err, store.ErrSnapshotAuth) {
			t.Fatalf("tampered envelope: %v", err)
		}
	}
	if _, err := store.Restore(bytes.NewReader(blob), backupKey, "node-2", t.TempDir(), key); !errors.Is(err, store.ErrSnapshotNode) {
		t.Fatalf("foreign node: %v", err)
	}
}

func TestSnapshotRecoveryRefusesInconsistentCeremony(t *testing.T) {
	f := snapshotRecoveryNew(t)
	f.stage(t, "commit")
	// Simulate a key lost after a durable commit decision. The writer must fail
	// closed rather than produce an apparently usable partial recovery point.
	if err := f.st.Delete(f.rec.KeyID); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := f.st.Snapshot(&out, f.backupKey, snapshotRecoveryNode); !errors.Is(err, store.ErrSnapshotFormat) {
		t.Fatalf("orphaned ceremony snapshot: %v", err)
	}
	if out.Len() != 0 {
		t.Fatal("invalid lifecycle snapshot emitted bytes")
	}
}
