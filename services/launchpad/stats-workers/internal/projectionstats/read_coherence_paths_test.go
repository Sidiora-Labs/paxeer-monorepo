package projectionstats

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestCanonicalReadRejectsChangedSourceState(t *testing.T) {
	ctx, pool, service, address := localReadFixture(t)
	assertTradeError := func(want error) {
		t.Helper()
		_, err := service.Trades(ctx, address, "", 10, "", 200)
		if !errors.Is(err, want) {
			t.Fatalf("trade source state: %v, want %v", err, want)
		}
	}
	assertWindowError := func(want error) {
		t.Helper()
		_, err := service.Window(ctx, address, 60, 200)
		if !errors.Is(err, want) {
			t.Fatalf("window source state: %v, want %v", err, want)
		}
	}
	if _, err := service.Trades(ctx, address, "other", 10, "", 200); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid side: %v", err)
	}
	if _, err := service.Trades(ctx, address, "", 101, "", 200); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid limit: %v", err)
	}
	if _, err := service.Window(ctx, address, 86401, 200); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid window: %v", err)
	}
	if _, err := service.Window(ctx, address, 60, -1); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid clock: %v", err)
	}
	if _, err := service.Trades(ctx, "0x"+strings.Repeat("f", 40), "", 10, "", 200); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown pool: %v", err)
	}
	if _, err := service.LastWindow(ctx, address, 60); err == nil {
		t.Fatal("absent window returned")
	}
	if _, err := pool.Exec(ctx, `UPDATE indexer.indexer_ingest_watermarks SET raw_complete_height=11 WHERE stream_id='canonical'`); err != nil {
		t.Fatal(err)
	}
	assertTradeError(ErrUnavailable)
	if _, err := pool.Exec(ctx, `UPDATE indexer.indexer_ingest_watermarks SET raw_complete_height=10 WHERE stream_id='canonical'`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE indexer.indexer_ingest_leases SET expires_at=clock_timestamp()-interval '1 second' WHERE stream_id='canonical'`); err != nil {
		t.Fatal(err)
	}
	assertWindowError(ErrUnavailable)
	if _, err := pool.Exec(ctx, `UPDATE indexer.indexer_ingest_leases SET expires_at=clock_timestamp()+interval '1 hour' WHERE stream_id='canonical'`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE indexer.indexer_block_completeness SET required_decode_complete=false WHERE stream_id='canonical'`); err != nil {
		t.Fatal(err)
	}
	assertTradeError(ErrUnavailable)
	if _, err := pool.Exec(ctx, `UPDATE indexer.indexer_block_completeness SET required_decode_complete=true WHERE stream_id='canonical'`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO stats_local.accounting_pool_fences(pool,frozen) VALUES($1,true)`, address); err != nil {
		t.Fatal(err)
	}
	assertWindowError(ErrIntegrity)
	if _, err := service.PruneExpiredSnapshots(context.Background(), 0); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid prune bound: %v", err)
	}
}
