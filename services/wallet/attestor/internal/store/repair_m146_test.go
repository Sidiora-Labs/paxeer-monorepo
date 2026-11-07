package store_test

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"math/big"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/policy"
	"github.com/Sidiora-Labs/Paxeer-X-Network/human/wallet/attestor/internal/store"
)

func TestRepairM_146_SnapshotRestoreRecordsAndRefusals(t *testing.T) {
	for _, decision := range []string{"", "commit", "abort"} {
		name := decision
		if name == "" {
			name = "undecided"
		}
		t.Run(name, func(t *testing.T) {
			f := repair240New(t)
			refresh := f.stage(t, decision)
			now := time.Now()
			clock := func() time.Time { return now }
			ledger, err := policy.NewSpendLedger(f.st, clock)
			if err != nil {
				t.Fatal(err)
			}
			ratePolicy := repair240Policy(t, 1)
			spendPolicy := repair240Policy(t, 100)
			if got := ratePolicy.Evaluate(repair240Account, repair240Request(t, 8), ledger.ForRequest("original")); !got.Allowed {
				t.Fatalf("seed real ledger through policy: %+v", got)
			}
			token := sha256.Sum256([]byte("repair-m146-token"))
			request := sha256.Sum256([]byte("repair-m146-request"))
			pub := sha256.Sum256([]byte("repair-m146-agent"))
			nonce := [16]byte{1, 4, 6}
			expires := now.Add(time.Hour)
			if fresh, err := f.st.UseTokenRequest(token, request, expires); err != nil || !fresh {
				t.Fatalf("seed token replay: %v, %v", fresh, err)
			}
			if fresh, err := f.st.UseNonce(pub, nonce, expires); err != nil || !fresh {
				t.Fatalf("seed nonce replay: %v, %v", fresh, err)
			}
			conflict := f.identity
			conflict.Owner = "different-owner"
			assertRefusals := func(st *store.Store) {
				t.Helper()
				live, err := policy.NewSpendLedger(st, clock)
				if err != nil {
					t.Fatal(err)
				}
				if got := ratePolicy.Evaluate(repair240Account, repair240Request(t, 3), live.ForRequest("next-rate")); got.Allowed || got.Code != policy.CodeRateLimited {
					t.Fatalf("rate refusal changed: %+v", got)
				}
				if got := spendPolicy.Evaluate(repair240Account, repair240Request(t, 3), live.ForRequest("next-spend")); got.Allowed || got.Code != policy.CodeDailyCap {
					t.Fatalf("daily spend refusal changed: %+v", got)
				}
				spent, err := live.Spent(policy.AccountKey(repair240Account), policy.AssetNative, now.Add(-policy.SpendWindow))
				if err != nil || spent == nil || spent.Cmp(big.NewInt(8)) != 0 {
					t.Fatalf("rolling spend = %v, %v; want 8", spent, err)
				}
				requests, err := live.Requests(policy.AccountKey(repair240Account), now.Add(-policy.RateWindow))
				if err != nil || requests != 2 {
					t.Fatalf("rolling requests = %d, %v; want the original and daily-refused requests", requests, err)
				}
				if fresh, err := st.UseTokenRequest(token, request, expires); err != nil || fresh {
					t.Fatalf("token replay refusal changed: %v, %v", fresh, err)
				}
				if fresh, err := st.UseNonce(pub, nonce, expires); err != nil || fresh {
					t.Fatalf("nonce replay refusal changed: %v, %v", fresh, err)
				}
				if _, err := st.Import(f.rec, f.plain, conflict); !errors.Is(err, store.ErrInvalid) {
					t.Fatalf("conflicting import identity accepted: %v", err)
				}
				if err := st.SetCeremonyRefreshDecision(f.rec.KeyID, "wrong-session", 1, "commit"); !errors.Is(err, store.ErrInvalid) {
					t.Fatalf("wrong ceremony session accepted: %v", err)
				}
				if err := st.CommitStaged(f.rec.KeyID, 2); !errors.Is(err, store.ErrStageEpoch) {
					t.Fatalf("wrong staged epoch accepted: %v", err)
				}
				if decision != "" {
					opposite := "commit"
					if decision == "commit" {
						opposite = "abort"
					}
					if err := st.SetCeremonyRefreshDecision(f.rec.KeyID, refresh.SessionID, 1, opposite); !errors.Is(err, store.ErrInvalid) {
						t.Fatalf("conflicting ceremony decision accepted: %v", err)
					}
				}
			}
			readRecords := func(st *store.Store) map[store.RecordKind][]byte {
				t.Helper()
				records := make(map[store.RecordKind][]byte)
				for _, kind := range []store.RecordKind{store.RecordLedger, store.RecordCeremony, store.RecordRefresh} {
					id := f.rec.KeyID
					if kind == store.RecordLedger {
						id = policy.AccountKey(repair240Account)
					}
					if err := st.WithRecord(kind, id, func(plain []byte) error {
						records[kind] = bytes.Clone(plain)
						return nil
					}); err != nil {
						t.Fatalf("read %s: %v", kind, err)
					}
				}
				return records
			}
			assertRefusals(f.st)
			wantRecords := readRecords(f.st)
			wantShares, err := f.st.List()
			if err != nil {
				t.Fatal(err)
			}
			wantStage, err := f.st.GetStaged(f.rec.KeyID)
			if err != nil {
				t.Fatal(err)
			}
			wantReceipt, err := f.st.ImportReceipt(f.rec.KeyID)
			if err != nil || wantReceipt.AuditSequence == 0 {
				t.Fatalf("audited import receipt: %+v, %v", wantReceipt, err)
			}
			assertRecords := func(st *store.Store) {
				t.Helper()
				if !reflect.DeepEqual(wantRecords, readRecords(st)) {
					t.Fatal("ledger or ceremony record bytes changed")
				}
				if got, err := st.List(); err != nil || !reflect.DeepEqual(wantShares, got) {
					t.Fatalf("committed metadata changed: %+v, %v", got, err)
				}
				if got, err := st.GetStaged(f.rec.KeyID); err != nil || !reflect.DeepEqual(wantStage, got) {
					t.Fatalf("staged metadata changed: %+v, %v", got, err)
				}
				if got, err := st.ImportReceipt(f.rec.KeyID); err != nil || !reflect.DeepEqual(wantReceipt, got) {
					t.Fatalf("import receipt changed: %+v, %v", got, err)
				}
				if got, err := st.CeremonyRefresh(f.rec.KeyID); err != nil || !reflect.DeepEqual(refresh, got) {
					t.Fatalf("ceremony state changed: %+v, %v", got, err)
				}
				if err := st.WithShare(f.rec.KeyID, func(plain []byte) error {
					if !bytes.Equal(plain, f.plain) {
						t.Fatal("committed share plaintext changed")
					}
					return nil
				}); err != nil {
					t.Fatal(err)
				}
			}
			var snapshot bytes.Buffer
			if err := f.st.Snapshot(&snapshot, f.backupKey, repair240Node); err != nil {
				t.Fatal(err)
			}
			if err := f.st.Close(); err != nil {
				t.Fatal(err)
			}
			dir := filepath.Join(t.TempDir(), "restored")
			restored, err := store.Restore(bytes.NewReader(snapshot.Bytes()), f.backupKey, repair240Node, dir, f.nodeKey)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = restored.Close() })
			assertRecords(restored)
			assertRefusals(restored)
			if err := restored.Close(); err != nil {
				t.Fatal(err)
			}
			reopened := repair240Open(t, dir, f.nodeKey)
			assertRecords(reopened)
			assertRefusals(reopened)
			assertRecords(reopened)
			if receipt, err := reopened.Import(f.rec, f.plain, f.identity); err != nil || !reflect.DeepEqual(wantReceipt, receipt) {
				t.Fatalf("idempotent import changed: %+v, %v", receipt, err)
			}
			if decision == "abort" {
				if err := reopened.DiscardStaged(f.rec.KeyID); err != nil {
					t.Fatal(err)
				}
				if err := reopened.CommitStaged(f.rec.KeyID, 1); !errors.Is(err, store.ErrNotFound) {
					t.Fatalf("discarded stage committed: %v", err)
				}
				if got, err := reopened.Get(f.rec.KeyID); err != nil || got.Epoch != 0 {
					t.Fatalf("abort advanced committed epoch: %+v, %v", got, err)
				}
			} else {
				if err := reopened.SetCeremonyRefreshDecision(f.rec.KeyID, refresh.SessionID, 1, "commit"); err != nil {
					t.Fatal(err)
				}
				if err := reopened.CommitStaged(f.rec.KeyID, 1); err != nil {
					t.Fatal(err)
				}
				if got, err := reopened.Get(f.rec.KeyID); err != nil || got.Epoch != 1 {
					t.Fatalf("commit did not advance epoch: %+v, %v", got, err)
				}
				if _, err := reopened.GetStaged(f.rec.KeyID); !errors.Is(err, store.ErrNotFound) {
					t.Fatalf("commit retained stage: %v", err)
				}
				if err := reopened.WithShare(f.rec.KeyID, func(plain []byte) error {
					if !bytes.Equal(plain, f.plain) {
						t.Fatal("recovered staged share plaintext changed")
					}
					return nil
				}); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
