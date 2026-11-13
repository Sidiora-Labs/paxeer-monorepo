package holdings

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestHolderTransactionsPreserveCommittedStateOnStorageRejection(t *testing.T) {
	ctx, pool, mint := canonicalLocalFixture(t)
	ddl, err := SchemaSQL("atomic_holders")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	repo, err := NewRepository(pool, "atomic_holders")
	if err != nil {
		t.Fatal(err)
	}
	if err = repo.Discover(ctx, mint.Token, 6, "10", true); err != nil {
		t.Fatal(err)
	}
	if got, err := repo.Apply(ctx, mint); err != nil || got.Revision != 1 {
		t.Fatalf("mint: %+v %v", got, err)
	}
	move := event(mint.Token, mint.To, addr(90), "10", 2)
	actor := ChainActor{ChainID: mint.ChainID, TransactionHash: mint.TransactionHash, BlockHash: mint.BlockHash, Address: addr(91)}
	classify := func(tx pgx.Tx) error {
		return repo.ClassifyTx(ctx, tx, mint.Token, mint.To, "external_wallet", strings.Repeat("a", 64), 1)
	}
	apply := func(tx pgx.Tx) error { _, e := repo.ApplyTx(ctx, tx, move); return e }
	checkpoint := func(tx pgx.Tx) error {
		_, e := repo.checkpointTx(ctx, tx, mint.Token, "10", mint.BlockHash, "holdings_v1")
		return e
	}
	distribution := func(tx pgx.Tx) error {
		_, e := repo.distributionTx(ctx, tx, Snapshot{Token: mint.Token, Supply: "100", Coverage: "complete"}, mint.To)
		return e
	}
	for _, tc := range []struct {
		name, setup, code string
		run               func(pgx.Tx) error
	}{
		{"transfer ledger unavailable", "ALTER TABLE atomic_holders.transfers RENAME TO saved_transfers", "42P01", apply},
		{"sender balances unavailable", "ALTER TABLE atomic_holders.balances RENAME TO saved_balances", "42P01", apply},
		{"recipient balances unavailable", "ALTER TABLE atomic_holders.balances RENAME TO saved_balances", "42P01", func(tx pgx.Tx) error {
			_, e := repo.ApplyTx(ctx, tx, event(mint.Token, Zero, addr(90), "10", 2))
			return e
		}},
		{"aggregate insert rejected", "ALTER TABLE atomic_holders.aggregates ADD CHECK (category <> 'unknown') NOT VALID", "23514", apply},
		{"recipient insert rejected", "ALTER TABLE atomic_holders.balances ADD CHECK (address <> '" + addr(90) + "') NOT VALID", "23514", apply},
		{"sender update rejected", "ALTER TABLE atomic_holders.balances ADD CHECK (balance >= 100) NOT VALID", "23514", apply},
		{"transfer insert rejected", "ALTER TABLE atomic_holders.transfers ADD CHECK (revision < 2) NOT VALID", "23514", apply},
		{"token update rejected", "ALTER TABLE atomic_holders.tokens ADD CHECK (revision < 2) NOT VALID", "23514", apply},
		{"provenance insert rejected", "ALTER TABLE atomic_holders.provenance ADD CHECK (false) NOT VALID", "23514", apply},
		{"incident insert rejected", "ALTER TABLE atomic_holders.incidents ADD CHECK (false) NOT VALID", "23514", func(tx pgx.Tx) error {
			_, e := repo.ApplyTx(ctx, tx, event(mint.Token, mint.To, addr(90), "101", 2))
			return e
		}},
		{"quarantine update rejected", "ALTER TABLE atomic_holders.tokens ADD CHECK (coverage <> 'quarantined') NOT VALID", "23514", func(tx pgx.Tx) error {
			_, e := repo.ApplyTx(ctx, tx, event(mint.Token, mint.To, addr(90), "101", 2))
			return e
		}},
		{"classification token unavailable", "ALTER TABLE atomic_holders.tokens RENAME TO saved_tokens", "42P01", classify},
		{"classification ledger unavailable", "ALTER TABLE atomic_holders.classifications RENAME TO saved_classifications", "42P01", classify},
		{"classification balances unavailable", "ALTER TABLE atomic_holders.balances RENAME TO saved_balances", "42P01", classify},
		{"classification insert rejected", "ALTER TABLE atomic_holders.classifications ADD CHECK (false) NOT VALID", "23514", classify},
		{"classification balance update rejected", "ALTER TABLE atomic_holders.balances ADD CHECK (category <> 'external_wallet') NOT VALID", "23514", classify},
		{"classification aggregate unavailable", "ALTER TABLE atomic_holders.aggregates RENAME TO saved_aggregates", "42P01", classify},
		{"classification addition rejected", "ALTER TABLE atomic_holders.aggregates ADD CHECK (category <> 'external_wallet') NOT VALID", "23514", classify},
		{"checkpoint token unavailable", "ALTER TABLE atomic_holders.tokens RENAME TO saved_tokens", "42P01", checkpoint},
		{"checkpoint balances unavailable", "ALTER TABLE atomic_holders.balances RENAME TO saved_balances", "42P01", checkpoint},
		{"checkpoint ledger unavailable", "ALTER TABLE atomic_holders.checkpoints RENAME TO saved_checkpoints", "42P01", checkpoint},
		{"distribution aggregate unavailable", "ALTER TABLE atomic_holders.aggregates RENAME TO saved_aggregates", "42P01", distribution},
		{"distribution leaders unavailable", "ALTER TABLE atomic_holders.balances RENAME TO saved_balances", "42P01", distribution},
		{"conservation authority unavailable", "ALTER TABLE atomic_holders.aggregates RENAME TO saved_aggregates", "42P01", func(tx pgx.Tx) error { return repo.conserveTx(ctx, tx, mint.Token, "100") }},
		{"actor ledger unavailable", "ALTER TABLE atomic_holders.transfers RENAME TO saved_transfers", "42P01", func(tx pgx.Tx) error { return repo.enrichActorTx(ctx, tx, mint, actor) }},
		{"actor provenance unavailable", "ALTER TABLE atomic_holders.provenance RENAME TO saved_provenance", "42P01", func(tx pgx.Tx) error { return repo.enrichActorTx(ctx, tx, mint, actor) }},
		{"actor provenance update rejected", "ALTER TABLE atomic_holders.provenance ADD CHECK (payload->>'actor' IS NULL) NOT VALID", "23514", func(tx pgx.Tx) error { return repo.enrichActorTx(ctx, tx, mint, actor) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = tx.Rollback(context.Background()) }()
			if _, err = tx.Exec(ctx, tc.setup); err != nil {
				t.Fatal(err)
			}
			err = tc.run(tx)
			var pgerr *pgconn.PgError
			if !errors.As(err, &pgerr) || pgerr.Code != tc.code {
				t.Fatalf("storage rejection: %v; want SQLSTATE %s", err, tc.code)
			}
			if err = tx.Rollback(ctx); err != nil {
				t.Fatal(err)
			}
			snapshot, err := repo.Snapshot(ctx, mint.Token, 10)
			if err != nil || snapshot.Supply != "100" || snapshot.Revision != 1 || len(snapshot.Balances) != 1 || snapshot.Balances[0].Raw != "100" {
				t.Fatalf("committed state changed: %+v %v", snapshot, err)
			}
		})
	}
}

func TestHolderPersistedStateIntegrityAndQuarantine(t *testing.T) {
	ctx, pool, mint := canonicalLocalFixture(t)
	ddl, err := SchemaSQL("state_holders")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	repo, err := NewRepository(pool, "state_holders")
	if err != nil {
		t.Fatal(err)
	}
	if err = repo.DiscoverCanonical(ctx, mint.Token, "canonical"); err != nil {
		t.Fatal(err)
	}
	result, err := repo.ApplyCanonicalTransfer(ctx, mint, "canonical")
	if err != nil || result.Revision != 1 || result.Incident {
		t.Fatalf("canonical mint: %+v %v", result, err)
	}
	move := event(mint.Token, mint.To, addr(90), "10", 2)
	actor := ChainActor{ChainID: mint.ChainID, TransactionHash: mint.TransactionHash, BlockHash: mint.BlockHash, Address: addr(91)}
	for _, tc := range []struct {
		name, setup string
		run         func(pgx.Tx) error
		want        error
	}{
		{"saved transfer JSON", "UPDATE state_holders.tokens SET last_event='\"invalid\"'", func(tx pgx.Tx) error { _, e := repo.applyTx(ctx, tx, move); return e }, nil},
		{"checkpoint position JSON", "UPDATE state_holders.tokens SET last_event='\"invalid\"'", func(tx pgx.Tx) error {
			_, e := repo.checkpointTx(ctx, tx, mint.Token, "10", mint.BlockHash, "holdings_v1")
			return e
		}, nil},
		{"checkpoint quarantined", "UPDATE state_holders.tokens SET coverage='quarantined'", func(tx pgx.Tx) error {
			_, e := repo.checkpointTx(ctx, tx, mint.Token, "10", mint.BlockHash, "holdings_v1")
			return e
		}, ErrIntegrity},
		{"checkpoint sum differs", "UPDATE state_holders.balances SET balance=99", func(tx pgx.Tx) error {
			_, e := repo.checkpointTx(ctx, tx, mint.Token, "10", mint.BlockHash, "holdings_v1")
			return e
		}, ErrIntegrity},
		{"aggregate sum differs", "UPDATE state_holders.aggregates SET raw=99", func(tx pgx.Tx) error { return repo.conserveTx(ctx, tx, mint.Token, "100") }, ErrIntegrity},
		{"aggregate population underflow", "UPDATE state_holders.aggregates SET population=0", func(tx pgx.Tx) error { return repo.aggregateDeltaTx(ctx, tx, mint.Token, "unknown", "100", -1) }, ErrIntegrity},
		{"aggregate amount", "", func(tx pgx.Tx) error { return repo.aggregateDeltaTx(ctx, tx, mint.Token, "unknown", "01", 1) }, nil},
		{"aggregate direction", "", func(tx pgx.Tx) error { return repo.aggregateDeltaTx(ctx, tx, mint.Token, "unknown", "100", 0) }, ErrIntegrity},
		{"distribution creator", "", func(tx pgx.Tx) error {
			_, e := repo.distributionTx(ctx, tx, Snapshot{Token: mint.Token, Supply: "100", Coverage: "complete"}, "invalid")
			return e
		}, nil},
		{"actor transfer digest", "UPDATE state_holders.transfers SET digest='" + strings.Repeat("f", 64) + "'", func(tx pgx.Tx) error { return repo.enrichActorTx(ctx, tx, mint, actor) }, ErrConflict},
		{"actor saved JSON", "UPDATE state_holders.provenance SET payload='\"invalid\"'", func(tx pgx.Tx) error { return repo.enrichActorTx(ctx, tx, mint, actor) }, nil},
		{"actor saved position", "UPDATE state_holders.provenance SET payload=jsonb_set(payload,'{recipient}',to_jsonb('" + addr(99) + "'::text))", func(tx pgx.Tx) error { return repo.enrichActorTx(ctx, tx, mint, actor) }, ErrIntegrity},
		{"actor malformed event", "", func(tx pgx.Tx) error { return repo.enrichActorTx(ctx, tx, Transfer{}, actor) }, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = tx.Rollback(ctx) }()
			if tc.setup != "" {
				if _, err = tx.Exec(ctx, tc.setup); err != nil {
					t.Fatal(err)
				}
			}
			err = tc.run(tx)
			if err == nil || (tc.want != nil && !errors.Is(err, tc.want)) {
				t.Fatalf("persisted integrity: %v, want %v", err, tc.want)
			}
		})
	}
	first, err := repo.Checkpoint(ctx, mint.Token, "10", mint.BlockHash, "holdings_v1")
	if err != nil {
		t.Fatal(err)
	}
	second, err := repo.Checkpoint(ctx, mint.Token, "10", mint.BlockHash, "holdings_v1")
	if err != nil || first.Digest != second.Digest || first.Revision != second.Revision {
		t.Fatalf("checkpoint identity: %+v %+v %v", first, second, err)
	}
	if _, err = pool.Exec(ctx, "UPDATE state_holders.checkpoints SET digest=$1", strings.Repeat("f", 64)); err != nil {
		t.Fatal(err)
	}
	if _, err = repo.Checkpoint(ctx, mint.Token, "10", mint.BlockHash, "holdings_v1"); !errors.Is(err, ErrConflict) {
		t.Fatalf("changed checkpoint: %v", err)
	}
	if _, err = pool.Exec(ctx, "UPDATE state_holders.tokens SET coverage='quarantined'"); err != nil {
		t.Fatal(err)
	}
	incident, err := repo.Apply(ctx, move)
	if !errors.Is(err, ErrIntegrity) || !incident.Incident {
		t.Fatalf("quarantine mutation: %+v %v", incident, err)
	}
	batch, err := repo.CatchUpCanonical(ctx, mint.Token, "canonical", 10)
	if err != nil || !batch.Incident || batch.Applied != 0 {
		t.Fatalf("quarantine catchup: %+v %v", batch, err)
	}
}
