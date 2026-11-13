package store

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store/canonicaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/queue"
	"github.com/jackc/pgx/v5/pgxpool"
)

func lifecycleStore(t *testing.T) (*Store, *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	_, pool := canonicaltest.New(t)
	partitionDDL, err := delivery.PartitionSchemaDDL("indexer")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, partitionDDL); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `CREATE SCHEMA recovery`); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"60001", "190001", "440000"} {
		body, err := os.ReadFile(filepath.Join("../../../../../migrations", id+".sql"))
		if err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, string(body)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, `INSERT INTO recovery.recovery_control VALUES(true,1,'primary',1,repeat('0',64))`); err != nil {
		t.Fatal(err)
	}
	guard, err := queue.NewEpochGuard("recovery", 1, "primary")
	if err != nil {
		t.Fatal(err)
	}
	st, err := NewLifecycleStore(pool, guard.Assert)
	if err != nil {
		t.Fatal(err)
	}
	if !st.LifecycleEnabled() || !st.CanonicalEpochGuarded() {
		t.Fatal("lifecycle store lacks epoch fencing")
	}
	return st, pool
}
func lifecycleRaw() RawRecord {
	return RawRecord{TransactionHash: "0x" + strings.Repeat("1", 64), BlockHash: "0x" + strings.Repeat("2", 64), Emitter: "0x" + strings.Repeat("3", 40), Height: 101, Topics: []string{}, Data: []byte{1}}
}
func authorizeStoreOperation(t *testing.T, pool *pgxpool.Pool, action string) OperationScope {
	t.Helper()
	scope := OperationScope{AuthorizationID: "approved-" + action, Operator: "operator", Action: action, Target: "canonical", Generation: "g1", Revision: "v1", From: 100, To: 103}
	if _, err := pool.Exec(context.Background(), `INSERT INTO indexer.indexer_replay_authorizations VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,2,clock_timestamp()+interval '1 hour',NULL)`, scope.AuthorizationID, scope.Operator, strings.Repeat("a", 64), scope.Action, scope.Target, scope.Generation, scope.Revision, scope.From, scope.To); err != nil {
		t.Fatal(err)
	}
	return scope
}

func TestQuarantineRetriesWaiversAndSourceRepair(t *testing.T) {
	ctx := context.Background()
	st, pool := lifecycleStore(t)
	raw := lifecycleRaw()
	poison := PoisonRecord{ChainID: 125, StreamID: "canonical", Raw: raw, ErrorCode: "arity", ABIRevision: "abi-v1", Requirement: "accounting", Required: true}
	if due, err := st.PoisonRetryDue(ctx, 125, "canonical", raw.TransactionHash, raw.LogIndex); err != nil || !due {
		t.Fatalf("unknown poison retry: %v %v", due, err)
	}
	for n := 0; n < 12; n++ {
		if err := st.RetainPoison(ctx, poison); err != nil {
			t.Fatal(err)
		}
	}
	var attempts int
	var operator bool
	if err := pool.QueryRow(ctx, `SELECT attempt_count,operator_required FROM indexer.indexer_quarantine`).Scan(&attempts, &operator); err != nil || attempts != 10 || !operator {
		t.Fatalf("bounded poison retry %d/%v: %v", attempts, operator, err)
	}
	if due, err := st.PoisonRetryDue(ctx, 125, "canonical", raw.TransactionHash, raw.LogIndex); err != nil || due {
		t.Fatalf("exhausted poison retry: %v %v", due, err)
	}
	changed := poison
	changed.RawDigest = strings.Repeat("0", 64)
	if err := st.RetainPoison(ctx, changed); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("changed poison digest: %v", err)
	}
	changed = poison
	changed.ErrorCode = "bounds"
	if err := st.RetainPoison(ctx, changed); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("changed poison classification: %v", err)
	}
	changed = poison
	changed.Raw.Data = []byte{2}
	if err := st.RetainPoison(ctx, changed); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("changed immutable raw: %v", err)
	}
	changed = poison
	changed.StreamID = "optional"
	changed.Required = false
	if err := st.RetainPoison(ctx, changed); err != nil {
		t.Fatal(err)
	}
	var degraded int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM indexer.indexer_capability_degradation`).Scan(&degraded); err != nil || degraded != 1 {
		t.Fatalf("optional degradation %d: %v", degraded, err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = checkPoisonTx(ctx, tx, 125, "canonical", 101, 101); !errors.Is(err, ErrRequiredPoison) {
		t.Fatalf("required poison not fenced: %v", err)
	}
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	digest, err := RawDigest(raw)
	if err != nil {
		t.Fatal(err)
	}
	waiver := authorizeStoreOperation(t, pool, "waive")
	if err = st.CheckOperationAuthorization(ctx, waiver); err != nil {
		t.Fatal(err)
	}
	if err = st.WaivePoison(ctx, waiver, digest, poison.Requirement, "approved bounded input", time.Now().Add(10*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err = st.WaivePoison(ctx, waiver, digest, poison.Requirement, "approved bounded input", time.Now().Add(10*time.Minute)); err == nil {
		t.Fatal("duplicate waiver accepted")
	}
	tx, err = pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = checkPoisonTx(ctx, tx, 125, "canonical", 101, 101); err != nil {
		t.Fatal(err)
	}
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	repair := authorizeStoreOperation(t, pool, "repair")
	if err = st.ResolvePoison(ctx, repair, digest, raw.BlockHash); err == nil {
		t.Fatal("uncommitted source repair accepted")
	}
	if _, err = pool.Exec(ctx, `INSERT INTO indexer.indexer_block_completeness VALUES(125,'canonical',101,$1,1,true,true,clock_timestamp())`, raw.BlockHash); err != nil {
		t.Fatal(err)
	}
	if err = st.ResolvePoison(ctx, repair, digest, raw.BlockHash); err != nil {
		t.Fatal(err)
	}
	var resolution string
	if err = pool.QueryRow(ctx, `SELECT resolution FROM indexer.indexer_quarantine WHERE stream_id='canonical'`).Scan(&resolution); err != nil || resolution != "repaired" {
		t.Fatalf("source repair state %s: %v", resolution, err)
	}
	tx, err = pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	canonical := CanonicalTx{Tx: tx, Store: st}
	if err = canonical.ResolveSourceQualifiedPoisonTx(ctx, CanonicalBatch{ChainID: 125, StreamID: "optional", Records: []RawRecord{raw}}); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if due, err := st.PoisonRetryDue(ctx, 125, "optional", raw.TransactionHash, raw.LogIndex); err != nil || due {
		t.Fatalf("repaired input scheduled: %v %v", due, err)
	}
	if _, err = pool.Exec(ctx, `UPDATE indexer.indexer_replay_authorizations SET revoked_at=clock_timestamp() WHERE authorization_id=$1`, waiver.AuthorizationID); err != nil {
		t.Fatal(err)
	}
	if err = st.CheckOperationAuthorization(ctx, waiver); err == nil {
		t.Fatal("revoked operation authorized")
	}
}

func TestCommittedHeadsAndReadinessRequireCurrentEpochAndLease(t *testing.T) {
	ctx := context.Background()
	st, pool := lifecycleStore(t)
	raw := lifecycleRaw()
	if err := st.RetainCommittedHead(ctx, "canonical", 101, raw.BlockHash, "source"); err != nil {
		t.Fatal(err)
	}
	if err := st.RetainCommittedHead(ctx, "canonical", 101, raw.BlockHash, "source"); err != nil {
		t.Fatal(err)
	}
	if err := st.RetainCommittedHead(ctx, "canonical", 101, "0x"+strings.Repeat("4", 64), "source"); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("conflicting committed head %v", err)
	}
	if err := st.RetainCommittedHead(ctx, "canonical", 102, raw.BlockHash, "source"); err != nil {
		t.Fatal(err)
	}
	if err := st.QuarantineCommittedHead(ctx, "canonical"); err != nil {
		t.Fatal(err)
	}
	var quarantined int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM indexer.indexer_committed_heads WHERE integrity='quarantined'`).Scan(&quarantined); err != nil || quarantined != 2 {
		t.Fatalf("quarantined heads %d: %v", quarantined, err)
	}
	fence, err := st.AcquireLease(ctx, 125, "canonical", "worker", 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO indexer.indexer_ingest_watermarks VALUES(125,'canonical',100,100)`); err != nil {
		t.Fatal(err)
	}
	if err = st.IngestReady(ctx, 125, "canonical", "worker", fence); err != nil {
		t.Fatal(err)
	}
	if err = st.RenewLease(ctx, 125, "canonical", "worker", fence, 30*time.Second); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `UPDATE indexer.indexer_ingest_watermarks SET typed_complete_height=99`); err != nil {
		t.Fatal(err)
	}
	if _, err = st.GetCanonicalWatermark(ctx, 125, "canonical"); !errors.Is(err, ErrWatermark) {
		t.Fatalf("diverged watermark %v", err)
	}
	if err = st.IngestReady(ctx, 125, "canonical", "worker", fence); !errors.Is(err, ErrFenced) {
		t.Fatalf("diverged readiness %v", err)
	}
	if err = st.ReleaseLease(ctx, 125, "canonical", "worker", fence); err != nil {
		t.Fatal(err)
	}
	if err = st.IngestReady(ctx, 125, "canonical", "worker", fence); !errors.Is(err, ErrFenced) {
		t.Fatalf("released readiness %v", err)
	}
	if err = st.RenewLease(ctx, 125, "canonical", "worker", fence, 30*time.Second); !errors.Is(err, ErrFenced) {
		t.Fatalf("expired lease revived %v", err)
	}
	if _, err = pool.Exec(ctx, `UPDATE recovery.recovery_control SET epoch=2`); err != nil {
		t.Fatal(err)
	}
	if err = st.QuarantineCommittedHead(ctx, "canonical"); err == nil {
		t.Fatal("stale epoch wrote quarantine")
	}
	if err = st.IngestReady(ctx, 125, "canonical", "worker", fence); err == nil {
		t.Fatal("stale epoch ready")
	}
}
