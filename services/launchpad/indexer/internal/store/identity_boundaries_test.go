package store

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/db/sqlcdb"
	"github.com/jackc/pgx/v5"
)

func TestPoolIdentityQueriesPreserveRegisteredAddressesAndNFTs(t *testing.T) {
	ctx := context.Background()
	st, pool := lifecycleStore(t)
	raw := lifecycleRaw()
	nft := int64(42)
	row := sqlcdb.InsertPoolParams{PoolAddress: raw.Emitter, TokenAddress: "0x" + strings.Repeat("5", 40), Creator: raw.Emitter, Optical: raw.Emitter, PoolID: "0x" + strings.Repeat("6", 64), NftID: &nft, CreatedAt: 1700000000, CreatedBlock: 101, TxHash: raw.TransactionHash}
	if err := st.InsertPool(ctx, row); err != nil {
		t.Fatal(err)
	}
	for _, query := range []func(context.Context) (*sqlcdb.IndexerPool, error){
		func(c context.Context) (*sqlcdb.IndexerPool, error) {
			return st.GetPoolByAddress(c, strings.ToUpper(raw.Emitter))
		},
		func(c context.Context) (*sqlcdb.IndexerPool, error) { return st.GetPoolByNftID(c, nft) },
	} {
		got, err := query(ctx)
		if err != nil || got == nil || got.PoolAddress != row.PoolAddress || got.PoolID != row.PoolID || got.NftID == nil || *got.NftID != nft || got.TokenAddress != row.TokenAddress {
			t.Fatalf("registered pool identity: %+v, %v", got, err)
		}
		canceled, cancel := context.WithCancel(ctx)
		cancel()
		if got, err := query(canceled); got != nil || !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled pool identity lookup: %+v, %v", got, err)
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if err := st.InsertPool(canceled, row); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled pool insert: %v", err)
	}
	if _, err := pool.Exec(ctx, `ALTER TABLE indexer.pools ALTER COLUMN created_at TYPE text USING created_at::text; UPDATE indexer.pools SET created_at='invalid'`); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreationSnapshotCandidates(ctx, 125, 1); err == nil || !strings.Contains(err.Error(), "scan") {
		t.Fatalf("invalid stored creation timestamp: %v", err)
	}
}

func TestCanonicalCommitRechecksEpochAndRejectsUnboundOperations(t *testing.T) {
	ctx := context.Background()
	st, pool := lifecycleStore(t)
	db := st.db.(*canonicalEpochDB)
	tx, err := db.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `UPDATE recovery.recovery_control SET epoch=epoch+1`); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err == nil {
		t.Fatal("changed recovery epoch committed")
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	var epoch int64
	if err := pool.QueryRow(ctx, `SELECT epoch FROM recovery.recovery_control`).Scan(&epoch); err != nil || epoch != 1 {
		t.Fatalf("refused epoch transaction persisted: %d, %v", epoch, err)
	}
	var absent *Store
	if err := absent.IngestReady(ctx, 125, "canonical", "owner", 1); !errors.Is(err, ErrFenced) {
		t.Fatalf("absent readiness authority: %v", err)
	}
	if err := absent.CheckOperationAuthorization(ctx, OperationScope{}); err == nil {
		t.Fatal("absent operation store admitted")
	}
	if _, err := authorizeOperation(ctx, nil, OperationScope{}); err == nil {
		t.Fatal("unbounded operation admitted")
	}
	if err := st.WaivePoison(ctx, OperationScope{}, "", "", "", time.Time{}); err == nil {
		t.Fatal("incomplete waiver admitted")
	}
	if sameIntents([]OutboxIntent{{TransactionHash: "transaction", Kind: "event", Payload: []byte(`{`)}}, []OutboxIntent{{TransactionHash: "transaction", Kind: "event", Payload: []byte(`{}`)}}) {
		t.Fatal("malformed intent payload compared equal")
	}
	if reconciledWithDeferredInventory([]OutboxIntent{{}}, nil, 0) {
		t.Fatal("mismatched outbox cardinality reconciled")
	}
}
