package store

import (
	"context"
	"crypto/sha256"
	"errors"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/jackc/pgx/v5"
)

func TestExactTransactionRetainsClaimsAcrossDatabaseWriteRefusals(t *testing.T) {
	for _, tc := range []struct {
		name, setup, want string
		advanceEpoch      bool
		conflict          bool
		foldError         bool
	}{
		{name: "invalid claim", want: "invalid_claim"},
		{name: "unavailable accepted identities", setup: `ALTER TABLE candles.chart_inbox RENAME TO chart_inbox_held`, want: "chart_inbox"},
		{name: "unavailable pool ownership", setup: `ALTER TABLE candles.chart_pool_fences RENAME TO chart_pool_fences_held`, want: "chart_pool_fences"},
		{name: "pool ownership write refused", setup: `CREATE TRIGGER refuse_pool_insert BEFORE INSERT ON candles.chart_pool_fences FOR EACH ROW EXECUTE FUNCTION candles.refuse_write()`, want: "write refused"},
		{name: "accepted identity write refused", setup: `CREATE TRIGGER refuse_inbox_insert BEFORE INSERT ON candles.chart_inbox FOR EACH ROW EXECUTE FUNCTION candles.refuse_write()`, want: "write refused"},
		{name: "owner epoch write refused", setup: `INSERT INTO candles.chart_pool_fences(chain_id,pool,epoch) VALUES('125','0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',1); CREATE TRIGGER refuse_pool_update BEFORE UPDATE ON candles.chart_pool_fences FOR EACH ROW EXECUTE FUNCTION candles.refuse_write()`, want: "write refused", advanceEpoch: true},
		{name: "frozen pool", setup: `INSERT INTO candles.chart_pool_fences(chain_id,pool,epoch,frozen) VALUES('125','0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',1,true)`, want: ErrIntegrity.Error()},
		{name: "stale pool owner", setup: `INSERT INTO candles.chart_pool_fences(chain_id,pool,epoch) VALUES('125','0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa',2)`, want: ErrStaleOwner.Error()},
		{name: "fold refusal", want: "fold refused", foldError: true},
		{name: "conflicting identity", want: ErrIntegrity.Error(), conflict: true},
		{name: "conflicting identity ownership write refused", setup: `CREATE TRIGGER refuse_pool_update BEFORE UPDATE ON candles.chart_pool_fences FOR EACH ROW EXECUTE FUNCTION candles.refuse_write()`, want: "write refused", conflict: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			pool := internaltest.OpenPool(t, internaltest.NewDatabase(t))
			ddl, err := ExactSchemaDDL("candles")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := pool.Exec(ctx, ddl); err != nil {
				t.Fatal(err)
			}
			if _, err := pool.Exec(ctx, `CREATE FUNCTION candles.refuse_write() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'write refused'; END $$`); err != nil {
				t.Fatal(err)
			}
			exact, err := NewExact(pool, "candles")
			if err != nil {
				t.Fatal(err)
			}
			address := "0x" + strings.Repeat("a", 40)
			fill := []byte(`{"price":"1"}`)
			digest := sha256.Sum256(fill)
			claim := Claim{ChainID: "125", Pool: address, Emitter: address, CodeEpoch: "epoch-1", TxHash: "0xwrite", LogIndex: "0", Epoch: 1, Digest: digest[:], Fill: fill}
			if tc.conflict {
				if err := exact.Transaction(ctx, claim, func(pgx.Tx, string) error { return nil }); err != nil {
					t.Fatal(err)
				}
			}
			if tc.setup != "" {
				if _, err := pool.Exec(ctx, tc.setup); err != nil {
					t.Fatal(err)
				}
			}
			if tc.name == "invalid claim" {
				claim.Epoch = 0
			}
			if tc.advanceEpoch {
				claim.Epoch = 2
			}
			if tc.conflict {
				claim.Pool = "0x" + strings.Repeat("b", 40)
			}
			fold := func(pgx.Tx, string) error { return nil }
			if tc.foldError {
				fold = func(pgx.Tx, string) error { return errors.New("fold refused") }
			}
			outer, beginErr := pool.Begin(ctx)
			if beginErr != nil {
				t.Fatal(beginErr)
			}
			outerErr := exact.TransactionTx(ctx, outer, claim, fold)
			if rollbackErr := outer.Rollback(ctx); rollbackErr != nil {
				t.Fatal(rollbackErr)
			}
			if tc.name == "frozen pool" || tc.conflict {
				var conflict *delivery.IntegrityFailure
				if !errors.As(outerErr, &conflict) || conflict.Class != delivery.IdentityConflict {
					t.Fatalf("outer identity conflict: %v", outerErr)
				}
			} else if outerErr == nil || !strings.Contains(outerErr.Error(), tc.want) {
				t.Fatalf("outer transaction result: %v", outerErr)
			}
			err = exact.Transaction(ctx, claim, fold)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("transaction result: %v", err)
			}
			if tc.name == "unavailable accepted identities" || tc.name == "unavailable pool ownership" {
				return
			}
			var accepted int
			if err := pool.QueryRow(ctx, `SELECT count(*) FROM candles.chart_inbox`).Scan(&accepted); err != nil || accepted != map[bool]int{false: 0, true: 1}[tc.conflict] {
				t.Fatalf("durable accepted identity count: %d, %v", accepted, err)
			}
		})
	}
}
