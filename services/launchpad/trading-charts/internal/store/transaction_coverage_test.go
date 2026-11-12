package store

import (
	"context"
	"crypto/sha256"
	"errors"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/jackc/pgx/v5"
)

func TestExactTransactionDeduplicatesAndFencesOwners(t *testing.T) {
	ctx := context.Background()
	pool := internaltest.OpenPool(t, internaltest.NewDatabase(t))
	ddl, err := ExactSchemaDDL("candles")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	exact, err := NewExact(pool, "candles")
	if err != nil {
		t.Fatal(err)
	}
	address := "0x" + strings.Repeat("a", 40)
	fill := []byte(`{"price":"1"}`)
	digest := sha256.Sum256(fill)
	claim := Claim{ChainID: "125", Pool: address, Emitter: address, CodeEpoch: "epoch-1", TxHash: "0xfirst", LogIndex: "0", Epoch: 1, Digest: digest[:], Fill: fill}
	folds := 0
	fold := func(tx pgx.Tx, schema string) error {
		folds++
		_, err := tx.Exec(ctx, `INSERT INTO `+schema+`.chart_exact(chain_id,pool,interval,start,snapshot) VALUES('125',$1,'1m',60,'{}')`, address)
		return err
	}
	if err := exact.Transaction(ctx, claim, fold); err != nil {
		t.Fatalf("first claim: %v", err)
	}
	if err := exact.Transaction(ctx, claim, fold); err != nil || folds != 1 {
		t.Fatalf("duplicate claim: %v, folds %d", err, folds)
	}
	var candles int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM candles.chart_exact WHERE pool=$1`, address).Scan(&candles); err != nil || candles != 1 {
		t.Fatalf("committed candle count: %d, %v", candles, err)
	}
	newer := claim
	newer.TxHash = "0xsecond"
	newer.Epoch = 2
	updateFold := func(tx pgx.Tx, schema string) error {
		tag, err := tx.Exec(ctx, `UPDATE `+schema+`.chart_exact SET snapshot='{"revision":"2"}' WHERE chain_id='125' AND pool=$1 AND interval='1m' AND start=60`, address)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return ErrIntegrity
		}
		return nil
	}
	if err := exact.Transaction(ctx, newer, updateFold); err != nil {
		t.Fatalf("higher owner epoch: %v", err)
	}
	stale := claim
	stale.TxHash = "0xthird"
	if err := exact.Transaction(ctx, stale, updateFold); !errors.Is(err, ErrStaleOwner) {
		t.Fatalf("stale owner: %v", err)
	}
	conflict := claim
	conflict.Epoch = 2
	conflict.Fill = []byte(`{"price":"2"}`)
	conflictDigest := sha256.Sum256(conflict.Fill)
	conflict.Digest = conflictDigest[:]
	if err := exact.Transaction(ctx, conflict, updateFold); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("conflicting canonical fill: %v", err)
	}
	var frozen bool
	if err := pool.QueryRow(ctx, `SELECT frozen FROM candles.chart_pool_fences WHERE chain_id='125' AND pool=$1`, address).Scan(&frozen); err != nil || !frozen {
		t.Fatalf("pool fence: %v, %v", frozen, err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	other := "0x" + strings.Repeat("b", 40)
	if err := exact.FreezePoolsTx(ctx, tx, []string{other, other}, accounting.Digest(strings.Repeat("c", 64))); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT frozen FROM candles.chart_pool_fences WHERE chain_id='125' AND pool=$1`, other).Scan(&frozen); err != nil || !frozen {
		t.Fatalf("explicit pool freeze: %v, %v", frozen, err)
	}
}

func TestExactTransactionTxBindsIdentityAndOwner(t *testing.T) {
	ctx := context.Background()
	pool := internaltest.OpenPool(t, internaltest.NewDatabase(t))
	ddl, err := ExactSchemaDDL("candles")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	exact, err := NewExact(pool, "candles")
	if err != nil {
		t.Fatal(err)
	}
	address := "0x" + strings.Repeat("a", 40)
	body := []byte(`{"price":"1"}`)
	digest := sha256.Sum256(body)
	claim := Claim{ChainID: "125", Pool: address, Emitter: address, CodeEpoch: "epoch-1", TxHash: "0xfirst", LogIndex: "0", Epoch: 1, Digest: digest[:], Fill: body}
	folds := 0
	fold := func(_ pgx.Tx, schema string) error {
		if schema != "candles" {
			return ErrIntegrity
		}
		folds++
		return nil
	}
	apply := func(c Claim, effect func(pgx.Tx, string) error) error {
		t.Helper()
		tx, e := pool.Begin(ctx)
		if e != nil {
			t.Fatal(e)
		}
		e = exact.TransactionTx(ctx, tx, c, effect)
		if e != nil {
			_ = tx.Rollback(ctx)
			return e
		}
		return tx.Commit(ctx)
	}
	if err := apply(claim, fold); err != nil || folds != 1 {
		t.Fatalf("first canonical claim: %d, %v", folds, err)
	}
	if err := apply(claim, fold); err != nil || folds != 1 {
		t.Fatalf("idempotent canonical claim: %d, %v", folds, err)
	}
	conflict := claim
	conflict.Pool = "0x" + strings.Repeat("b", 40)
	if err := apply(conflict, fold); !errors.Is(err, ErrIntegrity) || folds != 1 {
		t.Fatalf("pool identity conflict: %d, %v", folds, err)
	}
	newer := claim
	newer.TxHash, newer.Epoch = "0xnewer", 2
	if err := apply(newer, fold); err != nil || folds != 2 {
		t.Fatalf("new owner claim: %d, %v", folds, err)
	}
	stale := claim
	stale.TxHash = "0xstale"
	if err := apply(stale, fold); !errors.Is(err, ErrStaleOwner) || folds != 2 {
		t.Fatalf("stale owner: %d, %v", folds, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE candles.chart_pool_fences SET frozen=true WHERE pool=$1`, address); err != nil {
		t.Fatal(err)
	}
	frozen := newer
	frozen.TxHash = "0xfrozen"
	if err := apply(frozen, fold); !errors.Is(err, ErrIntegrity) || folds != 2 {
		t.Fatalf("frozen owner: %d, %v", folds, err)
	}
	invalid := claim
	invalid.Epoch = 0
	if err := apply(invalid, fold); err == nil || err.Error() != "invalid_claim" {
		t.Fatalf("invalid owner claim: %v", err)
	}
	if err := exact.TransactionTx(ctx, nil, claim, fold); err == nil || err.Error() != "transaction and fold required" {
		t.Fatalf("missing transaction: %v", err)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM candles.chart_inbox`).Scan(&count); err != nil || count != 2 {
		t.Fatalf("committed identities: %d, %v", count, err)
	}
}
