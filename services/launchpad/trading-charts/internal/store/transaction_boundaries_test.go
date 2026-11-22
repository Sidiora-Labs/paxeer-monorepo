package store

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/jackc/pgx/v5"
)

func boundaryExactStore(t *testing.T) *ExactStore {
	t.Helper()
	pool := internaltest.OpenPool(t, internaltest.NewDatabase(t))
	for _, schema := range []func(string) (string, error){ExactSchemaDDL, SeriesSchemaDDL, RebuildSchemaDDL} {
		ddl, err := schema("candles")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(context.Background(), ddl); err != nil {
			t.Fatal(err)
		}
	}
	exact, err := NewExact(pool, "candles")
	if err != nil {
		t.Fatal(err)
	}
	return exact
}

func TestSeriesAuthorityPreservesAttemptsAndWatermarksOnRejection(t *testing.T) {
	ctx := context.Background()
	exact := boundaryExactStore(t)
	address := "0x" + strings.Repeat("a", 40)
	digest, corrected := strings.Repeat("a", 64), strings.Repeat("b", 64)
	if err := exact.RecordAttempt(ctx, "charts", "reprocess_next", "publication", digest, 1, "received", false); err != nil {
		t.Fatal(err)
	}
	if err := exact.RecordAttempt(ctx, "charts", "reprocess_next", "publication", corrected, 1, "retry", false); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("conflicting attempt digest: %v", err)
	}
	var count int
	var stored string
	if err := exact.Pool.QueryRow(ctx, `SELECT attempt_count,wire_digest FROM candles.chart_delivery_attempts`).Scan(&count, &stored); err != nil || count != 1 || stored != digest {
		t.Fatalf("attempt integrity: %d, %s, %v", count, stored, err)
	}
	if _, err := exact.Pool.Exec(ctx, `UPDATE candles.chart_delivery_attempts SET attempt_count=10`); err != nil {
		t.Fatal(err)
	}
	bounded, cancel := context.WithTimeout(ctx, 250*time.Millisecond)
	defer cancel()
	if err := exact.WaitRetryBackoff(bounded, "charts", "reprocess_next", "publication"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("bounded retry wait: %v", err)
	}
	tx, err := exact.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if revision, err := exact.AdvanceSeriesTx(ctx, tx, address, 100, 180, false); err != nil || revision != 1 {
		t.Fatalf("first series clock: %d, %v", revision, err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	tx, err = exact.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if revision, err := exact.AdvanceSeriesTx(ctx, tx, address, 99, 180, false); revision != 0 || !errors.Is(err, ErrSourceWatermarkRegression) {
		t.Fatalf("regressing source clock: %d, %v", revision, err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := exact.Pool.Exec(ctx, `ALTER TABLE candles.chart_delivery_attempts RENAME TO chart_delivery_attempts_held`); err != nil {
		t.Fatal(err)
	}
	if err := exact.AuthorizeReprocess(ctx, "charts", "reprocess_next", "publication", digest, corrected, "accounting-fill-v2", "operator"); err == nil || !strings.Contains(err.Error(), "chart_delivery_attempts") {
		t.Fatalf("missing attempt authority: %v", err)
	}
	tx, err = exact.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := exact.AdvanceSeriesTx(ctx, tx, address, 101, 181, false); err == nil || !strings.Contains(err.Error(), "chart_delivery_attempts") {
		t.Fatalf("missing completeness authority: %v", err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := exact.Pool.Exec(ctx, `ALTER TABLE candles.chart_delivery_attempts_held RENAME TO chart_delivery_attempts; ALTER TABLE candles.chart_series_clock RENAME TO chart_series_clock_held`); err != nil {
		t.Fatal(err)
	}
	tx, err = exact.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := exact.AdvanceSeriesTx(ctx, tx, address, 101, 181, false); err == nil || !strings.Contains(err.Error(), "chart_series_clock") {
		t.Fatalf("missing series clock: %v", err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestReprocessAuditRollsBackRejectedWrites(t *testing.T) {
	for _, deferred := range []bool{false, true} {
		t.Run(map[bool]string{false: "insert", true: "commit"}[deferred], func(t *testing.T) {
			ctx := context.Background()
			exact := boundaryExactStore(t)
			oldDigest, newDigest := strings.Repeat("a", 64), strings.Repeat("b", 64)
			if err := exact.RecordAttempt(ctx, "charts", "reprocess_next", "publication", oldDigest, 1, "received", false); err != nil {
				t.Fatal(err)
			}
			trigger := `CREATE TRIGGER reject_audit BEFORE INSERT ON candles.chart_reprocess_audit FOR EACH ROW EXECUTE FUNCTION candles.reject_audit()`
			if deferred {
				trigger = `CREATE CONSTRAINT TRIGGER reject_audit AFTER INSERT ON candles.chart_reprocess_audit DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION candles.reject_audit()`
			}
			if _, err := exact.Pool.Exec(ctx, `CREATE FUNCTION candles.reject_audit() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'audit write refused'; END $$; `+trigger); err != nil {
				t.Fatal(err)
			}
			if err := exact.AuthorizeReprocess(ctx, "charts", "reprocess_next", "publication", oldDigest, newDigest, "accounting-fill-v2", "operator"); err == nil || !strings.Contains(err.Error(), "audit write refused") {
				t.Fatalf("rejected audit transaction: %v", err)
			}
			var count int
			if err := exact.Pool.QueryRow(ctx, `SELECT count(*) FROM candles.chart_reprocess_audit`).Scan(&count); err != nil || count != 0 {
				t.Fatalf("uncommitted audit: %d, %v", count, err)
			}
		})
	}
}

func TestPoolFenceRejectsEpochAndFreezeWritesAtomically(t *testing.T) {
	for _, outer := range []bool{false, true} {
		for _, conflict := range []bool{false, true} {
			name := map[bool]string{false: "owned", true: "outer"}[outer] + "/" + map[bool]string{false: "epoch", true: "freeze"}[conflict]
			t.Run(name, func(t *testing.T) {
				ctx := context.Background()
				exact := boundaryExactStore(t)
				claim := Claim{ChainID: "125", Pool: "0x" + strings.Repeat("a", 40), Emitter: "emitter", CodeEpoch: "v1", TxHash: "transaction", LogIndex: "0", Epoch: 1, Digest: bytes.Repeat([]byte{1}, 32), Fill: []byte(`{"value":1}`)}
				fold := func(tx pgx.Tx, q string) error {
					_, err := tx.Exec(ctx, `INSERT INTO `+q+`.chart_exact VALUES('125',$1,'1m',60,'{}')`, claim.Pool)
					return err
				}
				if err := exact.Transaction(ctx, claim, fold); err != nil {
					t.Fatal(err)
				}
				if _, err := exact.Pool.Exec(ctx, `CREATE FUNCTION candles.reject_fence() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'fence write refused'; END $$; CREATE TRIGGER reject_fence BEFORE UPDATE ON candles.chart_pool_fences FOR EACH ROW EXECUTE FUNCTION candles.reject_fence()`); err != nil {
					t.Fatal(err)
				}
				if conflict {
					claim.Digest = bytes.Repeat([]byte{2}, 32)
					claim.Fill = []byte(`{"value":2}`)
				} else {
					claim.Epoch = 2
				}
				var observed error
				if outer {
					tx, err := exact.Pool.Begin(ctx)
					if err != nil {
						t.Fatal(err)
					}
					observed = exact.TransactionTx(ctx, tx, claim, fold)
					if conflict {
						var failure *delivery.IntegrityFailure
						if !errors.As(observed, &failure) {
							t.Fatalf("typed canonical conflict: %v", observed)
						}
						observed = exact.FreezePoolsTx(ctx, tx, failure.AffectedPools, failure.EvidenceDigest)
					}
					if err := tx.Rollback(ctx); err != nil {
						t.Fatal(err)
					}
				} else {
					observed = exact.Transaction(ctx, claim, fold)
				}
				if observed == nil || !strings.Contains(observed.Error(), "fence write refused") {
					t.Fatalf("rejected fence mutation: %v", observed)
				}
				var epoch int
				var frozen bool
				if err := exact.Pool.QueryRow(ctx, `SELECT epoch,frozen FROM candles.chart_pool_fences`).Scan(&epoch, &frozen); err != nil || epoch != 1 || frozen {
					t.Fatalf("fence state: %d, %t, %v", epoch, frozen, err)
				}
				var count int
				if err := exact.Pool.QueryRow(ctx, `SELECT count(*) FROM candles.chart_exact`).Scan(&count); err != nil || count != 1 {
					t.Fatalf("preserved candle: %d, %v", count, err)
				}
			})
		}
	}
}

func TestReplayReferenceRejectsUnavailableStorage(t *testing.T) {
	for _, statement := range []string{
		`ALTER TABLE candles.chart_replay_references RENAME TO chart_replay_references_held`,
		`CREATE FUNCTION candles.reject_reference() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'reference write refused'; END $$; CREATE TRIGGER reject_reference BEFORE INSERT ON candles.chart_replay_references FOR EACH ROW EXECUTE FUNCTION candles.reject_reference()`,
	} {
		ctx := context.Background()
		exact := boundaryExactStore(t)
		if _, err := exact.Pool.Exec(ctx, statement); err != nil {
			t.Fatal(err)
		}
		if err := exact.HoldReplayRange(ctx, "reader", "0x"+strings.Repeat("a", 40), 0, 120, time.Minute); err == nil {
			t.Fatal("unavailable replay reference storage accepted")
		}
	}
}

func TestRetentionCommitRejectionPreservesCandles(t *testing.T) {
	for _, archived := range []bool{false, true} {
		t.Run(map[bool]string{false: "incident", true: "deletion"}[archived], func(t *testing.T) {
			ctx := context.Background()
			exact := boundaryExactStore(t)
			address := "0x" + strings.Repeat("a", 40)
			if _, err := exact.Pool.Exec(ctx, `INSERT INTO candles.chart_exact VALUES('125',$1,'1m',60,'{}')`, address); err != nil {
				t.Fatal(err)
			}
			relation, event := "chart_recoverability_incidents", "INSERT"
			if archived {
				if _, err := exact.Pool.Exec(ctx, `INSERT INTO candles.chart_archive_checkpoints(pool,from_time,to_time,manifest_digest) VALUES($1,0,120,$2)`, address, strings.Repeat("b", 64)); err != nil {
					t.Fatal(err)
				}
				relation, event = "chart_exact", "DELETE"
			}
			if _, err := exact.Pool.Exec(ctx, `CREATE FUNCTION candles.reject_retention() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'retention commit refused'; END $$; CREATE CONSTRAINT TRIGGER reject_retention AFTER `+event+` ON candles.`+relation+` DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION candles.reject_retention()`); err != nil {
				t.Fatal(err)
			}
			if removed, err := exact.DeleteRetainedRange(ctx, address, "1m", 0, 120); removed != 0 || err == nil || !strings.Contains(err.Error(), "retention commit refused") {
				t.Fatalf("rejected retention commit: %d, %v", removed, err)
			}
			var count int
			if err := exact.Pool.QueryRow(ctx, `SELECT count(*) FROM candles.chart_exact`).Scan(&count); err != nil || count != 1 {
				t.Fatalf("preserved retained candle: %d, %v", count, err)
			}
			if err := exact.Pool.QueryRow(ctx, `SELECT count(*) FROM candles.chart_recoverability_incidents`).Scan(&count); err != nil || count != 0 {
				t.Fatalf("rolled back incident: %d, %v", count, err)
			}
		})
	}
}
