package holdings

import (
	"errors"
	"strings"
	"testing"
)

func TestCanonicalDiscoveryAndCatchupValidatePersistedAuthority(t *testing.T) {
	for _, tc := range []struct {
		name, setup, kind string
		want              error
	}{
		{"discovery payload type", "UPDATE indexer.indexer_canonical_creations SET payload='[]'", "discover", nil},
		{"discovery metadata absent", "UPDATE indexer.indexer_canonical_creations SET payload='{}'", "discover", ErrUnknownToken},
		{"discovery supply", "UPDATE indexer.indexer_canonical_creations SET payload=jsonb_set(payload,'{metadata,totalSupply}','\"01\"')", "discover", nil},
		{"discovery initial supply binding", "UPDATE canonical_bounds.tokens SET expected_initial_supply=101,creation_transaction_hash='0x" + strings.Repeat("f", 64) + "'", "discover", ErrConflict},
		{"catchup watermark absent", "DELETE FROM indexer.indexer_ingest_watermarks", "catchup", nil},
		{"catchup invalid bound", "", "bound", nil},
		{"catchup bound overflow", "", "overflow", ErrOrder},
		{"catchup saved JSON", "UPDATE canonical_bounds.tokens SET last_event='[]'", "catchup", nil},
		{"catchup saved future position", "UPDATE canonical_bounds.tokens SET last_event='{" + `"height":"11","transactionIndex":"0","logIndex":"0"` + "}'", "catchup", ErrOrder},
		{"catchup raw authority differs", "UPDATE indexer.indexer_raw_logs SET data=decode('01','hex') WHERE log_index=1", "catchup", ErrIntegrity},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, pool, mint := canonicalLocalFixture(t)
			ddl, err := SchemaSQL("canonical_bounds")
			if err != nil {
				t.Fatal(err)
			}
			if _, err = pool.Exec(ctx, ddl); err != nil {
				t.Fatal(err)
			}
			repo, err := NewRepository(pool, "canonical_bounds")
			if err != nil {
				t.Fatal(err)
			}
			if err = repo.DiscoverCanonical(ctx, mint.Token, "canonical"); err != nil {
				t.Fatal(err)
			}
			if tc.setup != "" {
				if _, err = pool.Exec(ctx, tc.setup); err != nil {
					t.Fatal(err)
				}
			}
			switch tc.kind {
			case "discover":
				err = repo.DiscoverCanonical(ctx, mint.Token, "canonical")
			case "bound":
				_, err = repo.CatchUpCanonicalAt(ctx, mint.Token, "canonical", "01", 10)
			case "overflow":
				_, err = repo.CatchUpCanonicalAt(ctx, mint.Token, "canonical", "18446744073709551616", 10)
			default:
				_, err = repo.CatchUpCanonical(ctx, mint.Token, "canonical", 10)
			}
			if err == nil || tc.want != nil && !errors.Is(err, tc.want) {
				t.Fatalf("canonical authority: %v; want %v", err, tc.want)
			}
			var count int
			if err = pool.QueryRow(ctx, "SELECT count(*) FROM canonical_bounds.transfers").Scan(&count); err != nil || count != 0 {
				t.Fatalf("partial catchup: %d %v", count, err)
			}
		})
	}
}

func TestHolderMaterializationRejectsIncoherentLedger(t *testing.T) {
	for _, tc := range []struct{ name, setup, kind string }{
		{"last event JSON", "UPDATE snapshot_bounds.tokens SET last_event='[]'", "materialize"},
		{"first event JSON", "UPDATE snapshot_bounds.tokens SET first_event='[]'", "materialize"},
		{"provenance absent", "DELETE FROM snapshot_bounds.provenance", "materialize"},
		{"provenance JSON", "UPDATE snapshot_bounds.provenance SET payload='[]'", "materialize"},
		{"provenance recipient", "UPDATE snapshot_bounds.provenance SET payload=jsonb_set(payload,'{recipient}','\"0x0000000000000000000000000000000000000099\"')", "materialize"},
		{"aggregate conservation", "UPDATE snapshot_bounds.aggregates SET raw=99", "materialize"},
		{"snapshot insertion", "ALTER TABLE snapshot_bounds.snapshots ADD CHECK (false) NOT VALID", "materialize"},
		{"snapshot balance insertion", "ALTER TABLE snapshot_bounds.snapshot_balances ADD CHECK (false) NOT VALID", "materialize"},
		{"snapshot metadata JSON", "UPDATE snapshot_bounds.snapshots SET metadata='[]'", "page"},
		{"snapshot balance storage", "ALTER TABLE snapshot_bounds.snapshot_balances RENAME TO unavailable_balances", "page"},
		{"snapshot live token storage", "ALTER TABLE snapshot_bounds.tokens RENAME TO unavailable_tokens", "page"},
		{"snapshot position JSON", "UPDATE snapshot_bounds.tokens SET last_event='[]'", "snapshot"},
		{"snapshot balance storage", "ALTER TABLE snapshot_bounds.balances RENAME TO unavailable_balances", "snapshot"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, pool, mint := canonicalLocalFixture(t)
			ddl, err := SchemaSQL("snapshot_bounds")
			if err != nil {
				t.Fatal(err)
			}
			if _, err = pool.Exec(ctx, ddl); err != nil {
				t.Fatal(err)
			}
			repo, err := NewRepository(pool, "snapshot_bounds")
			if err != nil {
				t.Fatal(err)
			}
			if err = repo.Discover(ctx, mint.Token, 6, "10", true); err != nil {
				t.Fatal(err)
			}
			if _, err = repo.Apply(ctx, mint); err != nil {
				t.Fatal(err)
			}
			cursor, err := repo.Materialize(ctx, mint.Token, addr(83), "")
			if err != nil {
				t.Fatal(err)
			}
			if _, err = pool.Exec(ctx, tc.setup); err != nil {
				t.Fatal(err)
			}
			switch tc.kind {
			case "page":
				_, err = repo.ReadPage(ctx, mint.Token, addr(83), cursor, 10)
			case "snapshot":
				_, err = repo.Snapshot(ctx, mint.Token, 10)
			default:
				_, err = repo.Materialize(ctx, mint.Token, addr(83), "")
			}
			if err == nil {
				t.Fatal("incoherent ledger returned a snapshot")
			}
			if tc.kind == "materialize" {
				var count int
				if err = pool.QueryRow(ctx, "SELECT count(*) FROM snapshot_bounds.snapshots").Scan(&count); err != nil || count != 1 {
					t.Fatalf("partial snapshot: %d %v", count, err)
				}
			}
		})
	}
}
