package consumer

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/projectionstats"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestAccountingHooksBindLeaseAndSourceAuthority(t *testing.T) {
	ctx := context.Background()
	chain, log, timestamp := localAccountingChain(t)
	pool, store, partitions, repo, authority := localAccountingDatabase(t, log)
	publication := localAccountingPublication(t, pool, log, timestamp)
	c, err := NewSourceVerifiedAccountingConsumer(SourceVerifiedAccountingConfig{Pool: pool, Deliveries: store, Partitions: partitions, Repository: repo, Authority: authority, ConsumerID: "stats", Generation: "g1", ProjectionVersion: "projection-v1", Owner: "stats-worker", SourceStream: "canonical", SourcePartitionSchema: "indexer", RPC: chain, MaxReplayBlocks: 16, MaxReplayFills: 16})
	if err != nil {
		t.Fatal(err)
	}
	lease, err := c.lease(ctx, *publication.Fill)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := accounting.CanonicalPublicationV1(publication)
	if err != nil {
		t.Fatal(err)
	}
	envelope := delivery.Envelope{EventID: publication.PublicationID, EventType: "accounting.market.v1", SchemaVersion: 1, ChainID: 125, SourceOrder: delivery.SourceOrder{Height: publication.Fill.Position.Height, TransactionIndex: publication.Fill.Position.TransactionIndex, LogIndex: publication.Fill.Position.LogIndex}, EmittedAt: time.Now().UTC(), TraceID: "stats_hooks", PayloadHash: delivery.Hash(raw), Payload: raw}
	hooks := c.consumer.hooks(raw, lease)
	for _, tc := range []struct {
		name, sql                    string
		effect                       bool
		wrongEnvelope, wrongPosition bool
		class                        delivery.FailureClass
		code                         string
	}{
		{name: "guard envelope binding", wrongEnvelope: true, class: delivery.PartitionBinding},
		{name: "effect envelope binding", wrongEnvelope: true, effect: true, class: delivery.PartitionBinding},
		{name: "effect guarded position", effect: true, wrongPosition: true, class: delivery.PartitionBinding},
		{name: "guard original digest", sql: `UPDATE indexer.indexer_accounting_fills SET source_fact_digest='changed'`, class: delivery.Authority},
		{name: "guard original relation", sql: `ALTER VIEW indexer.indexer_accounting_fills_v1 RENAME TO saved_fills`, code: "42P01"},
		{name: "guard canonical emitter", sql: `UPDATE indexer.indexer_raw_logs SET emitter='0x0000000000000000000000000000000000000097'`, class: delivery.Authority},
		{name: "guard canonical relation", sql: `ALTER VIEW indexer.indexer_raw_logs_v1 RENAME TO saved_logs`, code: "42P01"},
		{name: "effect original digest", effect: true, sql: `UPDATE indexer.indexer_accounting_fills SET source_fact_digest='changed'`, class: delivery.Authority},
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
			e := envelope
			if tc.wrongEnvelope {
				e.EventID = "different-publication"
			}
			if tc.effect {
				guarded := delivery.GuardedPosition{Position: publication.Positions[0]}
				if tc.wrongPosition {
					guarded.Position.Sequence = "2"
				}
				err = hooks.Effect(ctx, tx, e, guarded)
			} else {
				_, err = hooks.Guard(ctx, tx, e)
			}
			if tc.class != "" {
				var failure *delivery.IntegrityFailure
				if !errors.As(err, &failure) || failure.Class != tc.class {
					t.Fatalf("hook integrity: %v, want %s", err, tc.class)
				}
			} else {
				var storage *pgconn.PgError
				if !errors.As(err, &storage) || storage.Code != tc.code {
					t.Fatalf("hook storage: %v, want %s", err, tc.code)
				}
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
	if _, err = hooks.Guard(ctx, tx, envelope); !errors.Is(err, pgx.ErrTxClosed) {
		t.Fatalf("closed guard transaction: %v", err)
	}
	tx, err = pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	guarded, err := hooks.Guard(ctx, tx, envelope)
	if err != nil {
		t.Fatal(err)
	}
	if err = hooks.Effect(ctx, tx, envelope, guarded); err != nil {
		t.Fatal(err)
	}
	if err = hooks.Advance(ctx, tx, envelope, guarded); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var trades, commits int
	if err = pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM stats_projection.trade_facts),(SELECT count(*) FROM stats_projection.accounting_market_commits)`).Scan(&trades, &commits); err != nil || trades != 1 || commits != 1 {
		t.Fatalf("single publication effect: %d %d %v", trades, commits, err)
	}
	var absent *AccountingConsumer
	if err = absent.Consume(ctx, "", nil, nil, lease); !errors.Is(err, projectionstats.ErrInvalid) {
		t.Fatalf("absent consumer: %v", err)
	}
}
