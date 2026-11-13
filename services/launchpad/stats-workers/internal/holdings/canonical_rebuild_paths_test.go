package holdings

import (
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestCanonicalRebuildRejectsChangedSourceCoverage(t *testing.T) {
	for _, tc := range []struct {
		name, query, stream, creation, through string
		decimals                               uint8
		want                                   error
	}{
		{"creation metadata", "", "canonical", "10", "10", 5, ErrConflict},
		{"reversed range", "", "canonical", "10", "9", 6, ErrOrder},
		{"unknown stream", "", "other", "10", "10", 6, ErrIntegrity},
		{"incomplete block", `UPDATE indexer.indexer_block_completeness SET required_decode_complete=false`, "canonical", "10", "10", 6, ErrIntegrity},
		{"missing creation", `DELETE FROM indexer.indexer_token_transfers; DELETE FROM indexer.indexer_canonical_creations`, "canonical", "10", "10", 6, pgx.ErrNoRows},
		{"missing transfer", `DELETE FROM indexer.indexer_token_transfers; DELETE FROM indexer.indexer_raw_logs WHERE log_index=1`, "canonical", "10", "10", 6, ErrIntegrity},
		{"changed transfer topics", `UPDATE indexer.indexer_raw_logs SET topics=jsonb_set(topics,'{1}','"0xinvalid"') WHERE log_index=1`, "canonical", "10", "10", 6, ErrIntegrity},
		{"changed transfer data", `UPDATE indexer.indexer_raw_logs SET data=decode('01','hex') WHERE log_index=1`, "canonical", "10", "10", 6, ErrIntegrity},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, pool, transfer := canonicalLocalFixture(t)
			if tc.query != "" {
				if _, err := pool.Exec(ctx, tc.query); err != nil {
					t.Fatal(err)
				}
			}
			ddl, err := SchemaSQL("rebuild_source")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := pool.Exec(ctx, ddl); err != nil {
				t.Fatal(err)
			}
			repo, err := NewRepository(pool, "rebuild_source")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := repo.RebuildFromCanonical(ctx, pool, tc.stream, transfer.Token, tc.decimals, tc.creation, tc.through, "holdings_v1"); !errors.Is(err, tc.want) {
				t.Fatalf("rebuild source: %v, want %v", err, tc.want)
			}
		})
	}
}

func TestCheckpointBindsCanonicalHeightAndBalances(t *testing.T) {
	ctx, pool, transfer := canonicalLocalFixture(t)
	ddl, err := SchemaSQL("checkpoint_bounds")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	repo, err := NewRepository(pool, "checkpoint_bounds")
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.DiscoverCanonical(ctx, transfer.Token, "canonical"); err != nil {
		t.Fatal(err)
	}
	if batch, err := repo.CatchUpCanonical(ctx, transfer.Token, "canonical", 10); err != nil || batch.Applied != 1 {
		t.Fatalf("canonical balance: %+v %v", batch, err)
	}
	checkpoint, err := repo.Checkpoint(ctx, transfer.Token, transfer.Height, transfer.BlockHash, "holdings_v1")
	if err != nil || checkpoint.Supply != "100" || checkpoint.Position == nil || checkpoint.Position.Identity() != transfer.Identity() {
		t.Fatalf("canonical checkpoint: %+v %v", checkpoint, err)
	}
	for _, tc := range []struct{ name, token, height, hash, version, want string }{
		{"version", transfer.Token, transfer.Height, transfer.BlockHash, "other", ErrIntegrity.Error()},
		{"block hash", transfer.Token, transfer.Height, "0x" + "0000000000000000000000000000000000000000000000000000000000000000", "holdings_v1", ErrIntegrity.Error()},
		{"token", "invalid", transfer.Height, transfer.BlockHash, "holdings_v1", "invalid address"},
		{"height", transfer.Token, "01", transfer.BlockHash, "holdings_v1", "invalid uint256"},
		{"before creation", transfer.Token, "9", transfer.BlockHash, "holdings_v1", ErrOrder.Error()},
		{"changed canonical block", transfer.Token, transfer.Height, "0x" + "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff", "holdings_v1", ErrConflict.Error()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := repo.Checkpoint(ctx, tc.token, tc.height, tc.hash, tc.version); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("checkpoint authority: %v, want %s", err, tc.want)
			}
		})
	}
}
