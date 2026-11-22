package holdings

import (
	"errors"
	"testing"
)

func TestCoinTextCanonicalReplayContinuesWithoutMetadata(t *testing.T) {
	for _, metadata := range []string{`{}`, `{"metadata":null}`} {
		t.Run(metadata, func(t *testing.T) {
			ctx, pool, mint := canonicalLocalFixture(t)
			missing := addr(80)
			if _, err := pool.Exec(ctx, `INSERT INTO indexer.indexer_canonical_creations(token,pool,creator,factory,optical,nft_id,height,block_hash,transaction_hash,transaction_index,log_index,block_timestamp,provenance,payload) SELECT $1,$2,creator,factory,optical,2,height,block_hash,$3,transaction_index,log_index,block_timestamp,provenance,$4 FROM indexer.indexer_canonical_creations WHERE token=$5`, missing, addr(86), "0x"+"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", metadata, mint.Token); err != nil {
				t.Fatal(err)
			}
			ddl, err := SchemaSQL("coin_text_holdings")
			if err != nil {
				t.Fatal(err)
			}
			if _, err = pool.Exec(ctx, ddl); err != nil {
				t.Fatal(err)
			}
			repository, err := NewRepository(pool, "coin_text_holdings")
			if err != nil {
				t.Fatal(err)
			}
			var served []PersistedCheckpoint
			replay := func() error {
				for _, token := range []string{missing, mint.Token} {
					checkpoint, err := repository.RebuildFromCanonical(ctx, pool, "canonical", token, 6, "10", "10", "holdings_v1")
					if errors.Is(err, ErrUnknownToken) {
						continue
					}
					if err != nil {
						return err
					}
					served = append(served, checkpoint)
				}
				return nil
			}
			if err = replay(); err != nil || len(served) != 1 || served[0].Token != mint.Token || served[0].Supply != "100" || served[0].Position == nil || served[0].Position.Identity() != mint.Identity() {
				t.Fatalf("canonical replay: %+v %v", served, err)
			}
			restarted, err := NewRepository(pool, "coin_text_holdings")
			if err != nil {
				t.Fatal(err)
			}
			if _, err = restarted.Snapshot(ctx, missing, 100); !errors.Is(err, ErrUnknownToken) {
				t.Fatalf("metadata-free token projection: %v", err)
			}
			snapshot, err := restarted.Snapshot(ctx, mint.Token, 100)
			if err != nil || snapshot.Supply != "100" || snapshot.Coverage != "complete" || len(snapshot.Balances) != 1 || snapshot.Balances[0].Address != mint.To || snapshot.Balances[0].Raw != "100" {
				t.Fatalf("served token projection: %+v %v", snapshot, err)
			}
			var tokens, checkpoints int
			if err = pool.QueryRow(ctx, "SELECT (SELECT count(*) FROM coin_text_holdings.tokens),(SELECT count(*) FROM coin_text_holdings.checkpoints)").Scan(&tokens, &checkpoints); err != nil || tokens != 1 || checkpoints != 1 {
				t.Fatalf("holdings projections: %d %d %v", tokens, checkpoints, err)
			}
		})
	}
}
