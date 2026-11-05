package backup

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"log"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"

	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/audit"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/policy"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/policy/evm"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/replica"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/store"
)

const repairM147Account = "0x1111111111111111111111111111111111111111"
const repairM147NextShare = "refreshed secp256k1 share material"

type repairM147Recovery struct {
	at      time.Time
	policy  *policy.Policy
	receipt store.ImportReceipt
	refresh store.CeremonyRefresh
	staged  store.ShareRecord
	token   [32]byte
	request [32]byte
	pub     [32]byte
	nonce   [16]byte
	expires time.Time
}

func (f *fixture) putImported(t *testing.T, rec store.ShareRecord, share string) {
	t.Helper()
	identity := store.ImportIdentity{CeremonyID: "backup-ceremony", SessionID: "backup-import", KeyID: rec.KeyID, Curve: rec.Curve, PublicKey: rec.PublicKey, Participants: rec.Participants, Threshold: 3, Owner: "backup-owner", Account: repairM147Account}
	if _, err := f.store.Import(rec, []byte(share), identity); err != nil {
		t.Fatal(err)
	}
	entry, err := f.audit.Append(audit.Entry{Kind: "keys.import", Subject: []byte(rec.KeyID), Decision: "allowed"})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.store.RecordImportAudit(rec.KeyID, identity.CeremonyID, entry.Sequence); err != nil {
		t.Fatal(err)
	}
	f.shares[rec.KeyID] = []byte(share)
}

func repairM147Request(t *testing.T, amount int64) policy.Request {
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

func (f *fixture) seedRecoveryState(t *testing.T) {
	t.Helper()
	r := &repairM147Recovery{at: time.Unix(1700000000, 0), expires: time.Now().Add(time.Hour)}
	f.recovery = r
	chain, rate := uint64(125), uint32(1)
	raw, err := json.Marshal(policy.Document{Version: policy.Version, Defaults: policy.Rules{ChainID: &chain, Kinds: []string{policy.KindEVMTransaction}, RatePerMinute: &rate, Caps: map[string]policy.Cap{policy.AssetNative: {PerTransaction: "10", Daily: "10"}}}})
	if err != nil {
		t.Fatal(err)
	}
	doc, err := policy.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	r.policy = policy.New(doc)
	ledger, err := policy.NewSpendLedger(f.store, func() time.Time { return r.at })
	if err != nil {
		t.Fatal(err)
	}
	if got := r.policy.Evaluate(repairM147Account, repairM147Request(t, 8), ledger.ForRequest("original-spend")); !got.Allowed {
		t.Fatalf("seed policy spend: %+v", got)
	}
	r.token, r.request, r.pub = sha256.Sum256([]byte("backup-token")), sha256.Sum256([]byte("backup-request")), sha256.Sum256([]byte("backup-agent"))
	copy(r.nonce[:], []byte("backup-nonce-147"))
	if fresh, err := f.store.UseTokenRequest(r.token, r.request, r.expires); err != nil || !fresh {
		t.Fatalf("seed token: fresh=%v error=%v", fresh, err)
	}
	if fresh, err := f.store.UseNonce(r.pub, r.nonce, r.expires); err != nil || !fresh {
		t.Fatalf("seed nonce: fresh=%v error=%v", fresh, err)
	}
	r.receipt, err = f.store.ImportReceipt("evm-key")
	if err != nil {
		t.Fatal(err)
	}
	id := r.receipt.Identity
	r.refresh = store.CeremonyRefresh{CeremonyID: id.CeremonyID, ImportSessionID: id.SessionID, SessionID: "backup-refresh", KeyID: id.KeyID, BaseEpoch: id.Epoch, Curve: id.Curve, PublicKey: id.PublicKey, Participants: id.Participants, State: "running"}
	if err := f.store.SaveCeremonyRefresh(r.refresh); err != nil {
		t.Fatal(err)
	}
	next, err := f.store.Get(id.KeyID)
	if err != nil {
		t.Fatal(err)
	}
	next.Epoch++
	if err := f.store.PutStaged(next, []byte(repairM147NextShare)); err != nil {
		t.Fatal(err)
	}
	r.staged, err = f.store.GetStaged(id.KeyID)
	if err != nil {
		t.Fatal(err)
	}
	r.refresh.State = "staged"
	if err := f.store.SaveCeremonyRefresh(r.refresh); err != nil {
		t.Fatal(err)
	}
}

func (f *fixture) assertRecoveryState(t *testing.T, st *store.Store) {
	t.Helper()
	r := f.recovery
	now := r.at
	ledger, err := policy.NewSpendLedger(st, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	account := policy.AccountKey(repairM147Account)
	spent, err := ledger.Spent(account, policy.AssetNative, now.Add(-policy.SpendWindow))
	if err != nil || spent.Cmp(big.NewInt(8)) != 0 {
		t.Fatalf("recovered spend: %v, %v", spent, err)
	}
	if count, err := ledger.Requests(account, now.Add(-policy.RateWindow)); err != nil || count != 1 {
		t.Fatalf("recovered request count: %d, %v", count, err)
	}
	if got := r.policy.Evaluate(repairM147Account, repairM147Request(t, 3), policy.ReadOnlyLedger(ledger.ForRequest("after-backup"))); got.Allowed || got.Code != policy.CodeRateLimited {
		t.Fatalf("recovered rate refusal: %+v", got)
	}
	now = now.Add(2 * policy.RateWindow)
	if got := r.policy.Evaluate(repairM147Account, repairM147Request(t, 3), policy.ReadOnlyLedger(ledger.ForRequest("after-backup"))); got.Allowed || got.Code != policy.CodeDailyCap {
		t.Fatalf("recovered daily refusal: %+v", got)
	}
	if got := r.policy.Evaluate(repairM147Account, repairM147Request(t, 2), policy.ReadOnlyLedger(ledger.ForRequest("remaining-budget"))); !got.Allowed {
		t.Fatalf("remaining budget refused: %+v", got)
	}
	now = r.at.Add(policy.SpendWindow + time.Second)
	if got := r.policy.Evaluate(repairM147Account, repairM147Request(t, 3), policy.ReadOnlyLedger(ledger.ForRequest("expired-budget"))); !got.Allowed {
		t.Fatalf("expired spend window refused: %+v", got)
	}
	if fresh, err := st.UseTokenRequest(r.token, r.request, r.expires); err != nil || fresh {
		t.Fatalf("recovered token replay: fresh=%v error=%v", fresh, err)
	}
	if fresh, err := st.UseNonce(r.pub, r.nonce, r.expires); err != nil || fresh {
		t.Fatalf("recovered nonce replay: fresh=%v error=%v", fresh, err)
	}
	if got, err := st.ImportReceipt(r.receipt.Identity.KeyID); err != nil || !reflect.DeepEqual(got, r.receipt) {
		t.Fatalf("recovered import receipt: %+v, %v", got, err)
	}
	if got, err := st.CeremonyRefresh(r.refresh.KeyID); err != nil || !reflect.DeepEqual(got, r.refresh) {
		t.Fatalf("recovered refresh: %+v, %v", got, err)
	}
	if got, err := st.GetStaged(r.refresh.KeyID); err != nil || !reflect.DeepEqual(got, r.staged) {
		t.Fatalf("recovered staged metadata: %+v, %v", got, err)
	}
}

func repairM147Signer(t *testing.T) (ssh.Signer, []byte) {
	t.Helper()
	_, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	block, err := ssh.MarshalPrivateKey(private, "")
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(private)
	if err != nil {
		t.Fatal(err)
	}
	return signer, pem.EncodeToMemory(block)
}

func repairM147Replica(t *testing.T, host ssh.Signer, authorized ssh.PublicKey) string {
	t.Helper()
	cfg := &ssh.ServerConfig{PublicKeyCallback: func(_ ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
		if !bytes.Equal(key.Marshal(), authorized.Marshal()) {
			return nil, errors.New("unauthorized replica key")
		}
		return &ssh.Permissions{}, nil
	}}
	cfg.AddHostKey(host)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	wg.Add(1)
	t.Cleanup(func() { ln.Close(); wg.Wait() })
	go func() {
		defer wg.Done()
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer conn.Close()
				_ = conn.SetDeadline(time.Now().Add(15 * time.Second))
				server, channels, requests, err := ssh.NewServerConn(conn, cfg)
				if err != nil {
					return
				}
				defer server.Close()
				go ssh.DiscardRequests(requests)
				for incoming := range channels {
					if incoming.ChannelType() != "session" {
						_ = incoming.Reject(ssh.UnknownChannelType, "session required")
						continue
					}
					channel, requests, err := incoming.Accept()
					if err != nil {
						return
					}
					func() {
						defer channel.Close()
						for request := range requests {
							var subsystem struct{ Name string }
							if request.Type != "subsystem" || ssh.Unmarshal(request.Payload, &subsystem) != nil || subsystem.Name != "sftp" {
								_ = request.Reply(false, nil)
								continue
							}
							_ = request.Reply(true, nil)
							go ssh.DiscardRequests(requests)
							sftpServer, err := sftp.NewServer(channel)
							if err != nil {
								return
							}
							defer sftpServer.Close()
							_ = sftpServer.Serve()
							return
						}
					}()
				}
			}()
		}
	}()
	return ln.Addr().String()
}

func (f *fixture) shipAndRestore(t *testing.T) (*store.Store, string, []byte) {
	t.Helper()
	host, _ := repairM147Signer(t)
	client, private := repairM147Signer(t)
	addr := repairM147Replica(t, host, client.PublicKey())
	root, remote := t.TempDir(), t.TempDir()
	keyPath, hostPath := filepath.Join(root, "client-key"), filepath.Join(root, "host-key")
	if err := os.WriteFile(keyPath, private, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(hostPath, ssh.MarshalAuthorizedKey(host.PublicKey()), 0o600); err != nil {
		t.Fatal(err)
	}
	shipper, err := replica.New(replica.Config{Addr: addr, User: "backup-test", KeyFile: keyPath, HostKeyFile: hostPath, RemoteDir: remote, Interval: replica.MinInterval, SnapshotDir: f.dir, LedgerPath: filepath.Join(root, replica.LedgerFileName)}, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	snap, err := f.writer(t, 1, shipper.Ledger()).WriteSnapshot("recovery-proof")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if count, err := shipper.Ship(ctx); err != nil || count != 1 {
		t.Fatalf("ship real snapshot: count=%d error=%v", count, err)
	}
	ledger, err := replica.OpenLedger(filepath.Join(root, replica.LedgerFileName))
	if err != nil {
		t.Fatal(err)
	}
	entry, ok := ledger.Lookup(snap.Name)
	digest := hex.EncodeToString(snap.SHA256[:])
	if !ok || entry.Size != snap.Size || entry.SHA256 != digest || entry.ShippedAt.IsZero() {
		t.Fatalf("persisted shipment receipt: %+v, found=%v", entry, ok)
	}
	sshClient, err := ssh.Dial("tcp", addr, &ssh.ClientConfig{User: "backup-test", Auth: []ssh.AuthMethod{ssh.PublicKeys(client)}, HostKeyCallback: ssh.FixedHostKey(host.PublicKey()), Timeout: 10 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer sshClient.Close()
	remoteClient, err := sftp.NewClient(sshClient)
	if err != nil {
		t.Fatal(err)
	}
	defer remoteClient.Close()
	file, err := remoteClient.Open(filepath.Join(remote, snap.Name))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(file)
	closeErr := file.Close()
	if err != nil || closeErr != nil {
		t.Fatalf("download shipped snapshot: %v, %v", err, closeErr)
	}
	if int64(len(raw)) != snap.Size || sha256.Sum256(raw) != snap.SHA256 {
		t.Fatal("downloaded snapshot differs from writer digest or size")
	}
	if got := listDir(t, remote); !reflect.DeepEqual(got, []string{snap.Name}) {
		t.Fatalf("replica publication left unexpected files: %v", got)
	}
	for _, plain := range f.shares {
		if bytes.Contains(raw, plain) {
			t.Fatal("snapshot exposed committed share")
		}
	}
	if bytes.Contains(raw, []byte(repairM147NextShare)) {
		t.Fatal("snapshot exposed staged share")
	}
	download := filepath.Join(root, snap.Name)
	if err := os.WriteFile(download, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := f.store.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.audit.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(snap.Path); err != nil {
		t.Fatal(err)
	}
	target, nodeKey := filepath.Join(t.TempDir(), "restored"), f.nodeKey
	result, err := RestoreFile(download, entry.SHA256, "node-1", f.backupKey, target, nodeKey)
	if err != nil {
		t.Fatal(err)
	}
	if result.Name != snap.Name || result.SHA256 != snap.SHA256 || result.Shares != len(f.shares) || result.AuditSequence != 1 {
		t.Fatalf("restore result: %+v", result)
	}
	st := repairM147Open(t, target, nodeKey)
	for key, want := range f.shares {
		if err := st.WithShare(key, func(got []byte) error {
			if !bytes.Equal(got, want) {
				t.Fatalf("restored share %s differs", key)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	f.assertRecoveryState(t, st)
	return st, target, nodeKey
}

func repairM147Open(t *testing.T, dir string, key []byte) *store.Store {
	t.Helper()
	st, err := store.Open(dir, key)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func TestRepairM_147_ShippedSnapshotRecoversPolicyAndReplay(t *testing.T) {
	f := newFixture(t)
	f.assertRecoveryState(t, f.store)
	st, target, key := f.shipAndRestore(t)
	r := f.recovery
	ledger, err := policy.NewSpendLedger(st, func() time.Time { return r.at })
	if err != nil {
		t.Fatal(err)
	}
	if err := ledger.Apply(policy.AccountKey(repairM147Account), "original-spend", []policy.Spend{{Asset: policy.AssetNative, Amount: big.NewInt(8)}}, r.at); err != nil {
		t.Fatal(err)
	}
	request := sha256.Sum256([]byte("distinct-request"))
	if fresh, err := st.UseTokenRequest(r.token, request, r.expires); err != nil || !fresh {
		t.Fatalf("distinct request refused after restore: %v, %v", fresh, err)
	}
	nonce := r.nonce
	nonce[0] ^= 1
	if fresh, err := st.UseNonce(r.pub, nonce, r.expires); err != nil || !fresh {
		t.Fatalf("distinct nonce refused after restore: %v, %v", fresh, err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	again := repairM147Open(t, target, key)
	f.assertRecoveryState(t, again)
	if fresh, err := again.UseTokenRequest(r.token, request, r.expires); err != nil || fresh {
		t.Fatalf("post-restore token use lost on restart: %v, %v", fresh, err)
	}
	if fresh, err := again.UseNonce(r.pub, nonce, r.expires); err != nil || fresh {
		t.Fatalf("post-restore nonce use lost on restart: %v, %v", fresh, err)
	}
}

func TestRepairM_147_ShippedSnapshotContinuesCeremony(t *testing.T) {
	for _, decision := range []string{"undecided", "commit", "abort"} {
		t.Run(decision, func(t *testing.T) {
			f := newFixture(t)
			r := f.recovery
			if decision != "undecided" {
				if err := f.store.SetCeremonyRefreshDecision(r.refresh.KeyID, r.refresh.SessionID, 1, decision); err != nil {
					t.Fatal(err)
				}
				r.refresh.Decision = decision
			}
			st, target, key := f.shipAndRestore(t)
			if _, err := st.Import(store.ShareRecord{KeyID: r.receipt.Identity.KeyID, Curve: r.receipt.Identity.Curve, PublicKey: r.receipt.Identity.PublicKey, Participants: r.receipt.Identity.Participants}, f.shares[r.receipt.Identity.KeyID], r.receipt.Identity); err != nil {
				t.Fatalf("restored import cannot be retried: %v", err)
			}
			conflict := r.receipt.Identity
			conflict.SessionID = "conflicting-import"
			if _, err := st.Import(store.ShareRecord{KeyID: conflict.KeyID, Curve: conflict.Curve, PublicKey: conflict.PublicKey, Participants: conflict.Participants}, f.shares[conflict.KeyID], conflict); !errors.Is(err, store.ErrInvalid) {
				t.Fatalf("conflicting import accepted: %v", err)
			}
			if decision == "abort" {
				if err := st.SetCeremonyRefreshDecision(r.refresh.KeyID, r.refresh.SessionID, 1, "commit"); !errors.Is(err, store.ErrInvalid) {
					t.Fatalf("restored abort reversed: %v", err)
				}
				if err := st.DiscardStaged(r.refresh.KeyID); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := st.SetCeremonyRefreshDecision(r.refresh.KeyID, "foreign-session", 1, "commit"); !errors.Is(err, store.ErrInvalid) {
					t.Fatalf("foreign refresh session accepted: %v", err)
				}
				if err := st.SetCeremonyRefreshDecision(r.refresh.KeyID, r.refresh.SessionID, 1, "commit"); err != nil {
					t.Fatal(err)
				}
				if err := st.SetCeremonyRefreshDecision(r.refresh.KeyID, r.refresh.SessionID, 1, "abort"); !errors.Is(err, store.ErrInvalid) {
					t.Fatalf("restored commit reversed: %v", err)
				}
				if err := st.CommitStaged(r.refresh.KeyID, 1); err != nil {
					t.Fatalf("resume staged commit: %v", err)
				}
				lg, err := audit.Open(filepath.Join(target, "audit"))
				if err != nil {
					t.Fatal(err)
				}
				entry, err := lg.Append(audit.Entry{Kind: "keys.refresh", Subject: []byte(r.refresh.KeyID), Decision: "allowed"})
				closeErr := lg.Close()
				if err != nil || closeErr != nil {
					t.Fatalf("audit continued refresh: %v, %v", err, closeErr)
				}
				r.refresh.Decision, r.refresh.State, r.refresh.AuditSequence = "commit", "complete", entry.Sequence
				if err := st.SaveCeremonyRefresh(r.refresh); err != nil {
					t.Fatal(err)
				}
			}
			if err := st.Close(); err != nil {
				t.Fatal(err)
			}
			again := repairM147Open(t, target, key)
			if got, err := again.CeremonyRefresh(r.refresh.KeyID); err != nil || !reflect.DeepEqual(got, r.refresh) {
				t.Fatalf("continued ceremony not durable: %+v, %v", got, err)
			}
			if _, err := again.GetStaged(r.refresh.KeyID); !errors.Is(err, store.ErrNotFound) {
				t.Fatalf("continued ceremony left a stage: %v", err)
			}
			wantEpoch, wantShare := uint64(1), []byte(repairM147NextShare)
			if decision == "abort" {
				wantEpoch, wantShare = 0, f.shares[r.refresh.KeyID]
			}
			if rec, err := again.Get(r.refresh.KeyID); err != nil || rec.Epoch != wantEpoch {
				t.Fatalf("continued share epoch: %+v, %v", rec, err)
			}
			if err := again.WithShare(r.refresh.KeyID, func(got []byte) error {
				if !bytes.Equal(got, wantShare) {
					t.Fatal("continued share material differs")
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
		})
	}
}
