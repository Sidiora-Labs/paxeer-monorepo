package analytics_test

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/analytics"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/risk"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/jackc/pgx/v5"
)

func TestReactionStorageRejectionsRollBackVoteAndOutbox(t *testing.T) {
	a := phase2Authority(t)
	ctx := context.Background()
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	wallet := strings.ToLower(crypto.PubkeyToAddress(key.PublicKey).Hex())
	i := analytics.Intent{ChainID: "125", Pool: "0x" + strings.Repeat("7", 40), Wallet: wallet, Reaction: "hot", Nonce: strings.Repeat("a", 64), Expiry: time.Now().Add(2 * time.Minute).Unix()}
	canonical, err := i.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	message := append([]byte("KindleLaunch reaction v1\n"), canonical...)
	signature, err := crypto.Sign(accounts.TextHash(message), key)
	if err != nil {
		t.Fatal(err)
	}
	verify := func(_ context.Context, actor string, body []byte) error {
		if !auth.VerifyWalletSignature(actor, string(body), "0x"+hex.EncodeToString(signature)) {
			return auth.ErrScopeDenied
		}
		return nil
	}
	for _, tc := range []struct{ name, sql, want string }{
		{"wallet unavailable", "ALTER TABLE " + a.Schema + ".reaction_wallets RENAME TO saved_wallets", "does not exist"},
		{"nonce unavailable", "ALTER TABLE " + a.Schema + ".reaction_nonces RENAME TO saved_nonces", "does not exist"},
		{"vote unavailable", "ALTER TABLE " + a.Schema + ".reaction_votes RENAME TO saved_votes", "does not exist"},
		{"pool revision unavailable", "ALTER TABLE " + a.Schema + ".reaction_pools RENAME TO saved_pools", "does not exist"},
		{"wallet insert rejected", "ALTER TABLE " + a.Schema + ".reaction_wallets ADD CHECK(false) NOT VALID", "check constraint"},
		{"pool insert rejected", "ALTER TABLE " + a.Schema + ".reaction_pools ADD CHECK(false) NOT VALID", "check constraint"},
		{"vote insert rejected", "ALTER TABLE " + a.Schema + ".reaction_votes ADD CHECK(false) NOT VALID", "check constraint"},
		{"nonce insert rejected", "ALTER TABLE " + a.Schema + ".reaction_nonces ADD CHECK(false) NOT VALID", "check constraint"},
		{"outbox insert rejected", "ALTER TABLE " + a.Schema + ".reaction_outbox ADD CHECK(false) NOT VALID", "check constraint"},
		{"wallet timestamp rejected", "ALTER TABLE " + a.Schema + ".reaction_wallets ADD CHECK(last_change IS NULL) NOT VALID", "check constraint"},
		{"revision exhausted", "INSERT INTO " + a.Schema + ".reaction_votes VALUES('" + i.Pool + "','" + wallet + "','hot',9223372036854775807)", "revision"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := a.Pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			if _, err = tx.Exec(ctx, tc.sql); err != nil {
				t.Fatal(err)
			}
			intent := i
			authorization := verify
			if tc.name == "revision exhausted" {
				intent.ExpectedRevision = 1<<63 - 1
				raw, err := intent.Canonical()
				if err != nil {
					t.Fatal(err)
				}
				signed, err := crypto.Sign(accounts.TextHash(append([]byte("KindleLaunch reaction v1\n"), raw...)), key)
				if err != nil {
					t.Fatal(err)
				}
				authorization = func(_ context.Context, actor string, body []byte) error {
					if !auth.VerifyWalletSignature(actor, string(body), "0x"+hex.EncodeToString(signed)) {
						return auth.ErrScopeDenied
					}
					return nil
				}
			}
			if _, err = a.ApplyInTx(ctx, tx, intent, authorization); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("reaction rejection: %v, want %s", err, tc.want)
			}
		})
	}
	public, err := a.ReadReactions(ctx, i.Pool)
	if err != nil || public.Revision != "0" || public.Counts["hot"] != 0 {
		t.Fatalf("uncommitted reaction retained: %+v %v", public, err)
	}
	for _, table := range []string{"reaction_votes", "reaction_pools"} {
		t.Run("public "+table, func(t *testing.T) {
			if _, err := a.Pool.Exec(ctx, "ALTER TABLE "+a.Schema+"."+table+" RENAME TO saved_reactions"); err != nil {
				t.Fatal(err)
			}
			if _, err := a.ReadReactions(ctx, i.Pool); err == nil || !strings.Contains(err.Error(), "does not exist") {
				t.Fatalf("reaction authority: %v", err)
			}
			if _, err := a.Pool.Exec(ctx, "ALTER TABLE "+a.Schema+".saved_reactions RENAME TO "+table); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestAnalyticsProjectionStorageFailuresPreserveCommittedSnapshots(t *testing.T) {
	a := phase2Authority(t)
	ctx := context.Background()
	fill := repair1038Fixture(t).Source[0]
	projector, err := analytics.NewProjector(a, risk.ProjectionModelV1{}, analytics.PostgresRiskInputs{Authority: a})
	if err != nil {
		t.Fatal(err)
	}
	wm := analytics.Watermark{ChainID: "125", Height: fill.Position.Height, BlockHash: fill.Position.BlockHash, ProjectionRevision: "1", ObservedAt: time.Now().UTC().Format("2006-01-02T15:04:05.000Z"), Integrity: "healthy"}
	snapshot := analytics.Snapshot{Pool: fill.PoolAddress, Model: "pressure-v1", Watermark: wm, CalculatedAt: time.Now().Unix(), Payload: json.RawMessage(`{"value":"1"}`)}
	for _, tc := range []struct {
		name, sql, want string
		run             func(pgx.Tx) error
	}{
		{"trade read", "ALTER TABLE " + a.Schema + ".analytics_trades RENAME TO saved_trades", "does not exist", func(tx pgx.Tx) error { return projector.ApplyFillTx(ctx, tx, fill, time.Now().UTC()) }},
		{"progress read", "ALTER TABLE " + a.Schema + ".analytics_progress RENAME TO saved_progress", "does not exist", func(tx pgx.Tx) error { return projector.ApplyFillTx(ctx, tx, fill, time.Now().UTC()) }},
		{"trade insert", "ALTER TABLE " + a.Schema + ".analytics_trades ADD CHECK(false) NOT VALID", "check constraint", func(tx pgx.Tx) error { return projector.ApplyFillTx(ctx, tx, fill, time.Now().UTC()) }},
		{"progress insert", "ALTER TABLE " + a.Schema + ".analytics_progress ADD CHECK(false) NOT VALID", "check constraint", func(tx pgx.Tx) error { return projector.ApplyFillTx(ctx, tx, fill, time.Now().UTC()) }},
		{"risk read", "ALTER TABLE " + a.Schema + ".analytics_risk_inputs RENAME TO saved_risk", "does not exist", func(tx pgx.Tx) error { return projector.ApplyFillTx(ctx, tx, fill, time.Now().UTC()) }},
		{"snapshot read", "ALTER TABLE " + a.Schema + ".analytics_snapshots RENAME TO saved_snapshots", "does not exist", func(tx pgx.Tx) error { return a.PersistSnapshot(ctx, tx, snapshot) }},
		{"snapshot insert", "ALTER TABLE " + a.Schema + ".analytics_snapshots ADD CHECK(false) NOT VALID", "check constraint", func(tx pgx.Tx) error { return a.PersistSnapshot(ctx, tx, snapshot) }},
		{"projection snapshot insert", "ALTER TABLE " + a.Schema + ".analytics_snapshots ADD CHECK(false) NOT VALID", "check constraint", func(tx pgx.Tx) error { return projector.ApplyFillTx(ctx, tx, fill, time.Now().UTC()) }},
		{"projection outbox insert", "ALTER TABLE " + a.Schema + ".analytics_outbox ADD CHECK(false) NOT VALID", "check constraint", func(tx pgx.Tx) error { return projector.ApplyFillTx(ctx, tx, fill, time.Now().UTC()) }},
		{"risk input read", "ALTER TABLE " + a.Schema + ".analytics_risk_inputs RENAME TO saved_inputs", "does not exist", func(tx pgx.Tx) error {
			return a.PersistRiskInputsTx(ctx, tx, fill.PoolAddress, wm, analytics.RiskInputs{HolderCount: -1, UniqueTraders24h: -1})
		}},
		{"risk input write", "ALTER TABLE " + a.Schema + ".analytics_risk_inputs ADD CHECK(false) NOT VALID", "check constraint", func(tx pgx.Tx) error {
			return a.PersistRiskInputsTx(ctx, tx, fill.PoolAddress, wm, analytics.RiskInputs{HolderCount: -1, UniqueTraders24h: -1})
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := a.Pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			if _, err = tx.Exec(ctx, tc.sql); err != nil {
				t.Fatal(err)
			}
			if err = tc.run(tx); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("analytics storage: %v, want %s", err, tc.want)
			}
		})
	}
	var n int
	if err = a.Pool.QueryRow(ctx, "SELECT count(*) FROM "+a.Schema+".analytics_trades").Scan(&n); err != nil || n != 0 {
		t.Fatalf("rejected projection committed: %d %v", n, err)
	}
	tx, err := a.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	for _, run := range []func() error{func() error { return projector.ApplyFillTx(ctx, tx, fill, time.Now().UTC()) }, func() error { return a.PersistSnapshot(ctx, tx, snapshot) }, func() error { return a.PersistRiskInputsTx(ctx, tx, fill.PoolAddress, wm, analytics.RiskInputs{}) }, func() error {
		_, e := (analytics.PostgresRiskInputs{Authority: a}).LoadRiskInputs(ctx, tx, fill.PoolAddress, wm)
		return e
	}, func() error {
		_, e := (analytics.CanonicalRiskInputs{Authority: a}).LoadRiskInputs(ctx, tx, fill.PoolAddress, wm)
		return e
	}} {
		if err := run(); !errors.Is(err, pgx.ErrTxClosed) {
			t.Fatalf("closed analytics transaction: %v", err)
		}
	}
	a.Pool = internaltest.NewPostgres(t)
	a.Pool.Close()
	if _, err := a.ReadReactions(ctx, fill.PoolAddress); err == nil || !strings.Contains(err.Error(), "closed") {
		t.Fatalf("closed reaction authority: %v", err)
	}
}
