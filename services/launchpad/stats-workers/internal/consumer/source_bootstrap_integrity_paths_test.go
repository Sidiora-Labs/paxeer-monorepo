package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/projectionstats"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/jackc/pgx/v5"
)

func TestSourceBootstrapChecksStoredPositionPublicationAndReplay(t *testing.T) {
	ctx := context.Background()
	chain, log, timestamp := localAccountingChain(t)
	for _, tc := range []struct{ name, sql, want string }{
		{"position JSON", `UPDATE indexer.delivery_partition_positions SET position=convert_to('"invalid"','UTF8')`, "cannot unmarshal"},
		{"position sequence", `UPDATE indexer.delivery_partition_positions SET position=convert_to(jsonb_set(convert_from(position,'UTF8')::jsonb,'{sequence}','"2"')::text,'UTF8')`, "conflicting canonical event"},
		{"position effect", `UPDATE indexer.delivery_partition_positions SET position=convert_to(jsonb_set(convert_from(position,'UTF8')::jsonb,'{previousEffectIdentity}','"changed"')::text,'UTF8')`, "conflicting canonical event"},
		{"publication absent", `DELETE FROM indexer.indexer_ingest_outbox`, "no rows"},
		{"publication version", `UPDATE indexer.indexer_ingest_outbox SET payload=jsonb_set(payload,'{version}','"other"')`, "accounting"},
		{"publication position", `UPDATE indexer.indexer_ingest_outbox SET payload=jsonb_set(payload,'{positions,0,sequence}','"2"')`, "accounting identity mismatch"},
		{"watermark absent", `DELETE FROM indexer.indexer_ingest_watermarks`, "source unavailable"},
		{"watermark behind", `UPDATE indexer.indexer_ingest_watermarks SET raw_complete_height=0,typed_complete_height=0`, "source unavailable"},
		{"watermark relation", `ALTER VIEW indexer.indexer_ingest_watermarks_v1 RENAME TO saved_watermarks`, "does not exist"},
		{"position relation", `ALTER VIEW indexer.indexer_market_partition_positions_v1 RENAME TO saved_positions`, "does not exist"},
		{"publication relation", `ALTER VIEW indexer.indexer_accounting_publications_v1 RENAME TO saved_publications`, "does not exist"},
		{"fill digest", `UPDATE indexer.indexer_accounting_fills SET source_fact_digest='changed'`, "conflicting canonical event"},
		{"fill relation", `ALTER VIEW indexer.indexer_accounting_fills_v1 RENAME TO saved_fills`, "does not exist"},
		{"committed block", `UPDATE indexer.indexer_ingest_commits SET block_hash='0x'||repeat('f',64)`, "conflicting canonical event"},
		{"raw event bytes", `UPDATE indexer.indexer_raw_logs SET data=decode('01','hex')`, "conflicting canonical event"},
		{"replay range", `UPDATE indexer.indexer_ingest_commits SET last_complete_height=17`, "source unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pool, deliveries, partitions, repo, authority := localAccountingDatabase(t, log)
			p := localAccountingPublication(t, pool, log, timestamp)
			if _, err := pool.Exec(ctx, tc.sql); err != nil {
				t.Fatal(err)
			}
			c, err := NewSourceVerifiedAccountingConsumer(SourceVerifiedAccountingConfig{Pool: pool, Deliveries: deliveries, Partitions: partitions, Repository: repo, Authority: authority, ConsumerID: "stats", Generation: "g1", ProjectionVersion: "projection-v1", Owner: "stats-worker", SourceStream: "canonical", SourcePartitionSchema: "indexer", RPC: chain, MaxReplayBlocks: 16, MaxReplayFills: 16})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = c.lease(ctx, *p.Fill); err == nil || !strings.Contains(strings.ToLower(err.Error()), tc.want) {
				t.Fatalf("bootstrap authority: %v, want %s", err, tc.want)
			}
			var initialized int
			if err = pool.QueryRow(ctx, `SELECT count(*) FROM stats_delivery.delivery_partition_offsets WHERE checkpoint_digest IS NOT NULL`).Scan(&initialized); err != nil || initialized != 0 {
				t.Fatalf("unverified checkpoint retained: %d %v", initialized, err)
			}
		})
	}
}

func TestCanonicalPublicationRechecksSourceWithinTransaction(t *testing.T) {
	ctx := context.Background()
	chain, log, timestamp := localAccountingChain(t)
	pool, deliveries, partitions, repo, authority := localAccountingDatabase(t, log)
	publication := localAccountingPublication(t, pool, log, timestamp)
	c, err := NewSourceVerifiedAccountingConsumer(SourceVerifiedAccountingConfig{Pool: pool, Deliveries: deliveries, Partitions: partitions, Repository: repo, Authority: authority, ConsumerID: "stats", Generation: "g1", ProjectionVersion: "projection-v1", Owner: "stats-worker", SourceStream: "canonical", SourcePartitionSchema: "indexer", RPC: chain, MaxReplayBlocks: 16, MaxReplayFills: 16})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, sql string
		edit      func(*accounting.PublicationV1)
		want      error
		message   string
	}{
		{name: "canonical publication"},
		{name: "missing fill", edit: func(p *accounting.PublicationV1) { p.Fill = nil }, want: projectionstats.ErrIntegrity},
		{name: "malformed fill", edit: func(p *accounting.PublicationV1) { p.Fill.Side = "other" }, want: projectionstats.ErrIntegrity},
		{name: "changed position", sql: `UPDATE indexer.delivery_partition_positions SET position=convert_to(jsonb_set(convert_from(position,'UTF8')::jsonb,'{sequence}','"2"')::text,'UTF8')`, want: projectionstats.ErrIntegrity},
		{name: "missing publication", sql: `DELETE FROM indexer.indexer_ingest_outbox`, want: projectionstats.ErrIntegrity},
		{name: "publication query unavailable", sql: `ALTER VIEW indexer.indexer_accounting_publications_v1 RENAME TO saved_publications`, message: "does not exist"},
		{name: "source query unavailable", sql: `ALTER VIEW indexer.indexer_ingest_watermarks_v1 RENAME TO saved_watermarks`, message: "does not exist"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			if tc.sql != "" {
				if _, err = tx.Exec(ctx, tc.sql); err != nil {
					t.Fatal(err)
				}
			}
			raw, err := json.Marshal(publication)
			if err != nil {
				t.Fatal(err)
			}
			var p accounting.PublicationV1
			if err = json.Unmarshal(raw, &p); err != nil {
				t.Fatal(err)
			}
			if tc.edit != nil {
				tc.edit(&p)
			}
			err = c.consumer.canonicalPublication(ctx, tx, p)
			if tc.want != nil {
				if !errors.Is(err, tc.want) {
					t.Fatalf("publication: %v", err)
				}
			} else if tc.message != "" {
				if err == nil || !strings.Contains(err.Error(), tc.message) {
					t.Fatalf("publication storage: %v", err)
				}
			} else if err != nil {
				t.Fatalf("canonical publication: %v", err)
			}
		})
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if err = c.consumer.canonicalPublication(ctx, tx, publication); !errors.Is(err, pgx.ErrTxClosed) {
		t.Fatalf("closed publication transaction: %v", err)
	}
}
