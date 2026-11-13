package store

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/jackc/pgx/v5"
)

func accountingLifecycleFill() accounting.FillV1 {
	f := accounting.FillV1{Version: accounting.FillVersion, Position: accounting.SourcePosition{ChainID: "125", Height: "101", TransactionIndex: "0", LogIndex: "0", TransactionHash: "0x" + strings.Repeat("1", 64), BlockHash: "0x" + strings.Repeat("2", 64)}, ActualEmitter: "0x" + strings.Repeat("3", 40), PoolAddress: "0x" + strings.Repeat("4", 40), Token: accounting.Asset{Address: "0x" + strings.Repeat("5", 40), Decimals: 18}, Settlement: accounting.Asset{Address: "0x" + strings.Repeat("6", 40), Decimals: 6}, Side: "buy", TokenAmountRaw: "1000000000000000000", USDLAmountRaw: "5000000", PaidInputRaw: "5000000", FeeRaw: "15000", InputDerivationQuality: "receipt_incomplete", FeeDisposition: "distributed_usdl", Attribution: accounting.Attribution{TransactionSender: "0x" + strings.Repeat("7", 40), Quality: "unavailable", Evidence: []accounting.EvidenceRef{}}, Execution: accounting.ExecutionPolicy{Quality: "unavailable", Evidence: []accounting.EvidenceRef{}}, Provenance: accounting.Provenance{DeploymentID: "private-125", ABIRevision: "sidiora-v1", EmitterCodeEpoch: "epoch-1", PoolCodeEpoch: "epoch-1", ReceiptSHA256: strings.Repeat("8", 64), HeaderSHA256: strings.Repeat("9", 64)}, BlockTimestampSeconds: "1700000000"}
	f.SourceFillID = accounting.SourceFillIdentity(f.Position, f.Provenance.DeploymentID)
	return f
}
func TestAccountingFillAndDeferredIntentPreserveCanonicalIdentity(t *testing.T) {
	ctx := context.Background()
	st, pool := lifecycleStore(t)
	fill := accountingLifecycleFill()
	if err := accounting.ValidateFillV1(fill); err != nil {
		t.Fatal(err)
	}
	fence, err := st.AcquireLease(ctx, 125, "canonical", "worker", 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO indexer.indexer_ingest_watermarks VALUES(125,'canonical',100,100)`); err != nil {
		t.Fatal(err)
	}
	raw := lifecycleRaw()
	batch := CanonicalBatch{ChainID: 125, StreamID: "canonical", Owner: "worker", BatchID: "source-fill", Fence: fence, PreviousHeight: 100, Blocks: []BlockCompleteness{{Height: 101, Hash: raw.BlockHash, RawCount: 1, FetchComplete: true, RequiredDecodeComplete: true}}, Records: []RawRecord{raw}}
	expectedDigest, err := accounting.FillDigest(fill)
	if err != nil {
		t.Fatal(err)
	}
	if err = st.CommitProjectedCanonicalTx(ctx, batch, func(ctx context.Context, tx CanonicalTx) ([]OutboxIntent, error) {
		digest, err := tx.InsertAccountingFill(ctx, fill)
		if err != nil {
			return nil, err
		}
		if digest != expectedDigest {
			return nil, errors.New("fill digest changed")
		}
		return []OutboxIntent{{TransactionHash: raw.TransactionHash, Kind: "Swap", Payload: []byte(`{"amount":"5000000"}`)}}, nil
	}); err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	canonical := CanonicalTx{Tx: tx, Store: st}
	if digest, err := canonical.InsertAccountingFill(ctx, fill); err != nil || digest != expectedDigest {
		t.Fatalf("duplicate fill digest %s %v", digest, err)
	}
	changed := fill
	changed.TokenAmountRaw = "2000000000000000000"
	if _, err = canonical.InsertAccountingFill(ctx, changed); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("changed source fact %v", err)
	}
	changed = fill
	changed.Position.Height = "102"
	if _, err = canonical.InsertAccountingFill(ctx, changed); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("changed raw height %v", err)
	}
	if _, err = (CanonicalTx{}).InsertAccountingFill(ctx, fill); err == nil {
		t.Fatal("accounting fill without transaction accepted")
	}
	invalid := fill
	invalid.Side = "unknown"
	if _, err = canonical.InsertAccountingFill(ctx, invalid); err == nil {
		t.Fatal("invalid fill accepted")
	}
	intent := OutboxIntent{TransactionHash: raw.TransactionHash, Kind: accounting.InventorySubject, Payload: []byte(`{"inventory":"source-bound"}`)}
	if err = canonical.InsertCommittedOutboxIntent(ctx, batch, intent); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("future inventory input %v", err)
	}
	batch.PreviousHeight = 101
	if err = canonical.InsertCommittedOutboxIntent(ctx, batch, intent); err != nil {
		t.Fatal(err)
	}
	if err = canonical.InsertCommittedOutboxIntent(ctx, batch, intent); err != nil {
		t.Fatal(err)
	}
	different := intent
	different.Payload = []byte(`{"inventory":"changed"}`)
	if err = canonical.InsertCommittedOutboxIntent(ctx, batch, different); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("changed deferred intent %v", err)
	}
	different = intent
	different.Kind = "Swap"
	if err = canonical.InsertCommittedOutboxIntent(ctx, batch, different); err == nil {
		t.Fatal("unapproved deferred facet accepted")
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	batch.PreviousHeight = 100
	if matched, err := st.ReconcileCanonicalCommit(ctx, batch); err != nil || !matched {
		t.Fatalf("deferred intent reconciliation %v %v", matched, err)
	}
	measured, err := st.MeasureCanonicalRange(ctx, "canonical", 101, 101)
	if err != nil || measured.AccountingFillCount != 1 || measured.ProjectionCount != 1 || measured.TypedCount != 2 || measured.RawCount != 1 {
		t.Fatalf("committed accounting measurement %+v %v", measured, err)
	}
	for _, tc := range []struct{ name, sql string }{
		{"block completeness", `UPDATE indexer.indexer_block_completeness SET fetch_complete=false`},
		{"raw count", `UPDATE indexer.indexer_block_completeness SET raw_count=2`},
		{"raw topics", `UPDATE indexer.indexer_raw_logs SET topics='[1]'::jsonb`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = tx.Exec(ctx, tc.sql); err != nil {
				t.Fatal(err)
			}
			if err = tx.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			if _, err = st.MeasureCanonicalRange(ctx, "canonical", 101, 101); err == nil {
				t.Fatal("incomplete measurement accepted")
			}
			switch tc.name {
			case "block completeness":
				_, err = pool.Exec(ctx, `UPDATE indexer.indexer_block_completeness SET fetch_complete=true`)
			case "raw count":
				_, err = pool.Exec(ctx, `UPDATE indexer.indexer_block_completeness SET raw_count=1`)
			case "raw topics":
				_, err = pool.Exec(ctx, `UPDATE indexer.indexer_raw_logs SET topics='[]'::jsonb`)
			}
			if err != nil {
				t.Fatal(err)
			}
		})
	}
	if _, err = st.GetPoolByAddress(ctx, fill.PoolAddress); err != nil {
		t.Fatal(err)
	}
	if found, err := st.GetPoolByNftID(ctx, 101); err != nil || found != nil {
		t.Fatalf("unknown nft pool %+v %v", found, err)
	}
	for _, bounds := range []CanonicalBatch{{}, {ChainID: 125}} {
		if err = st.CommitProjectedCanonical(ctx, bounds, nil); err == nil {
			t.Fatal("missing canonical projector accepted")
		}
		if err = st.CommitProjectedCanonicalTx(ctx, bounds, nil); err == nil {
			t.Fatal("missing transaction projector accepted")
		}
	}
	if _, err = st.GetCanonicalWatermark(ctx, 1, "canonical"); !errors.Is(err, ErrWatermark) {
		t.Fatal(err)
	}
	if _, err = st.GetCanonicalWatermark(ctx, 125, ""); !errors.Is(err, ErrWatermark) {
		t.Fatal(err)
	}
	if _, err = NewCanonicalStore(nil, nil); err == nil {
		t.Fatal("absent recovery authority accepted")
	}
	if (*Store)(nil).CanonicalEpochGuarded() || (*Store)(nil).LifecycleEnabled() {
		t.Fatal("absent store is ready")
	}
	for _, operation := range []func() error{func() error { return (&canonicalEpochTx{}).Commit(ctx) }, func() error { _, err := (&canonicalEpochDB{}).BeginTx(ctx, pgx.TxOptions{}); return err }} {
		if err = operation(); err == nil {
			t.Fatal("absent epoch transaction accepted")
		}
	}
}
