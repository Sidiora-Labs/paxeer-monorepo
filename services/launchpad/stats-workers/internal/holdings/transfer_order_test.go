package holdings

import (
	"fmt"
	"testing"
)

func TestNumericOrderTransferPages(t *testing.T) {
	ctx, pool, mint := canonicalLocalFixture(t)
	const schema = "numeric_transfer_pages"
	ddl, err := SchemaSQL(schema)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	repo, err := NewRepository(pool, schema)
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		"DELETE FROM indexer.indexer_token_transfers",
		"DELETE FROM indexer.indexer_raw_logs WHERE log_index=1",
		"UPDATE indexer.indexer_raw_logs SET log_index=12 WHERE log_index=0",
		"UPDATE indexer.indexer_canonical_creations SET log_index=12",
		"UPDATE indexer.indexer_block_completeness SET raw_count=1",
		"UPDATE indexer.indexer_ingest_commits SET record_count=1",
	} {
		if _, err = pool.Exec(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	recipient := addr(86)
	for i := 0; i < 12; i++ {
		transfer := event(mint.Token, mint.To, recipient, fmt.Sprint(i), i)
		transfer.TransactionHash = mint.TransactionHash
		if i == 0 {
			transfer.From, transfer.To, transfer.Amount = Zero, mint.To, "100"
		}
		holderIdleTransfer(t, ctx, pool, transfer)
	}
	if err = repo.DiscoverCanonical(ctx, mint.Token, "canonical"); err != nil {
		t.Fatal(err)
	}
	applied := 0
	for page := 0; page < 4; page++ {
		batch, err := repo.catchUpCanonicalAtOnce(ctx, mint.Token, "canonical", "10", 5)
		if err != nil || batch.Incident {
			t.Fatalf("transfer page %d: %+v %v", page, batch, err)
		}
		want := 5
		if page == 2 {
			want = 2
		}
		if page == 3 {
			want = 0
		}
		applied += want
		if batch.Applied != want || batch.More != (page < 2) {
			t.Fatalf("transfer page %d: %+v, want %d applied", page, batch, want)
		}
		snapshot, err := repo.Snapshot(ctx, mint.Token, 10)
		if err != nil || snapshot.Position == nil {
			t.Fatalf("transfer checkpoint: %+v %v", snapshot, err)
		}
		if snapshot.Position.LogIndex != fmt.Sprint(applied-1) {
			t.Fatalf("checkpoint log index %s, want %d", snapshot.Position.LogIndex, applied-1)
		}
		rows, err := pool.Query(ctx, "SELECT event->>'logIndex' FROM "+schema+".transfers WHERE token=$1 ORDER BY revision", mint.Token)
		if err != nil {
			t.Fatal(err)
		}
		count := 0
		for rows.Next() {
			var index string
			if err = rows.Scan(&index); err != nil {
				rows.Close()
				t.Fatal(err)
			}
			if index != fmt.Sprint(count) {
				rows.Close()
				t.Fatalf("applied transfer %d has log index %s", count, index)
			}
			count++
		}
		rows.Close()
		if err = rows.Err(); err != nil {
			t.Fatal(err)
		}
		if count != applied {
			t.Fatalf("applied transfer count %d, want %d", count, applied)
		}
		sent := (applied - 1) * applied / 2
		balances := map[string]string{}
		for _, balance := range snapshot.Balances {
			balances[balance.Address] = balance.Raw
		}
		if snapshot.Supply != "100" || len(balances) != 2 || balances[mint.To] != fmt.Sprint(100-sent) || balances[recipient] != fmt.Sprint(sent) {
			t.Fatalf("transfer sums after %d events: %+v", applied, snapshot)
		}
		if page >= 2 && snapshot.Coverage != "complete" {
			t.Fatalf("final transfer coverage: %+v", snapshot)
		}
	}
}
