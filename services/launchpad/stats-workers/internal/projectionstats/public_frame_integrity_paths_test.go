package projectionstats

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestPublicFramesBindTopicsWindowsAndWatermarks(t *testing.T) {
	f := newPublicFixture(t)
	fills := f.ingestReal()
	f.advance(33963761, f.real.Blocks["33963761"].Hash)
	p := f.projector()
	now := time.Unix(f.real.Blocks["33963749"].Timestamp+60, 0)
	if result, err := p.RefreshOnce(f.ctx, now); err != nil || result.Appended != 1 || result.Prices != 1 {
		t.Fatalf("public frames: %+v %v", result, err)
	}
	original := f.frame(f.real.Pool).frame
	for _, tc := range []struct {
		name string
		edit func(*publicStatsFrame)
	}{
		{"window duration", func(v *publicStatsFrame) { v.Payload.WindowStart = v.Payload.WindowEnd }},
		{"topic", func(v *publicStatsFrame) { v.Topic = "market.stats.0x" + strings.Repeat("a", 40) }},
		{"watermark", func(v *publicStatsFrame) { v.Watermark.Height = "1" }},
		{"coalescing", func(v *publicStatsFrame) {
			v.Coalesced = true
			first, last := "1", "1"
			v.SourceSequenceFirst = &first
			v.SourceSequenceLast = &last
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			frame := original
			tc.edit(&frame)
			raw, err := json.Marshal(frame)
			if err != nil {
				t.Fatal(err)
			}
			if err = ValidatePublicStatsFrame(raw); !errors.Is(err, ErrIntegrity) {
				t.Fatalf("stats frame: %v", err)
			}
		})
	}
	var raw []byte
	if err := f.pg.QueryRow(f.ctx, `SELECT frame FROM api_stream.stream_projection WHERE topic=$1`, "market.price."+f.real.Pool).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var price publicPriceFrame
	if err := json.Unmarshal(raw, &price); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		edit func(*publicPriceFrame)
	}{
		{"revision", func(v *publicPriceFrame) { v.Watermark.ProjectionRevision = "100" }},
		{"height", func(v *publicPriceFrame) { v.Watermark.Height = "1" }},
		{"coalescing", func(v *publicPriceFrame) {
			v.Coalesced = true
			first, last := "1", "1"
			v.SourceSequenceFirst = &first
			v.SourceSequenceLast = &last
		}},
	} {
		t.Run("price "+tc.name, func(t *testing.T) {
			frame := price
			tc.edit(&frame)
			wire, err := json.Marshal(frame)
			if err != nil {
				t.Fatal(err)
			}
			if err = ValidatePublicPriceFrame(wire); !errors.Is(err, ErrIntegrity) {
				t.Fatalf("price frame: %v", err)
			}
		})
	}
	for _, tc := range []struct {
		name string
		edit func(*committedFill)
	}{
		{"zero token", func(v *committedFill) { v.fill.TokenAmountRaw = "0" }},
		{"invalid USDL", func(v *committedFill) { v.fill.USDLAmountRaw = "01" }},
		{"invalid time", func(v *committedFill) { v.fill.BlockTimestampSeconds = "not-a-time" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			last := committedFill{fill: fills["sell"]}
			tc.edit(&last)
			if _, err := PublicPriceOf(last.fill); !errors.Is(err, ErrIntegrity) {
				t.Fatalf("price source: %v", err)
			}
		})
	}
	anchor := publicAnchor{Position: original.Payload.SourceUpperPosition, timestamp: f.real.Blocks["33963749"].Timestamp}
	for _, tc := range []struct {
		name, sql string
		run       func(pgx.Tx) error
	}{
		{"stats head", `UPDATE api_stream.stream_projection SET frame=convert_to('{}','UTF8') WHERE topic LIKE 'market.stats.%'`, func(tx pgx.Tx) error { _, e := p.appendTx(f.ctx, tx, anchor, original.Payload, now); return e }},
		{"price head", `UPDATE api_stream.stream_projection SET frame=convert_to('{}','UTF8') WHERE topic LIKE 'market.price.%'`, func(tx pgx.Tx) error {
			_, e := p.appendPriceTx(f.ctx, tx, committedFill{fill: fills["sell"], sequence: 100}, now)
			return e
		}},
		{"stats invalid anchor", "", func(tx pgx.Tx) error {
			a := anchor
			a.LogIndex = "invalid"
			_, e := p.appendTx(f.ctx, tx, a, original.Payload, now)
			return e
		}},
		{"stats invalid payload", "", func(tx pgx.Tx) error {
			doc := original.Payload
			doc.WindowStart = doc.WindowEnd
			_, e := p.appendTx(f.ctx, tx, anchor, doc, now)
			return e
		}},
		{"price invalid source", "", func(tx pgx.Tx) error {
			last := committedFill{fill: fills["sell"], sequence: 100}
			last.fill.TokenAmountRaw = "0"
			_, e := p.appendPriceTx(f.ctx, tx, last, now)
			return e
		}},
		{"price invalid observation", "", func(tx pgx.Tx) error {
			_, e := p.appendPriceTx(f.ctx, tx, committedFill{fill: fills["sell"], sequence: 100}, time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC))
			return e
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := f.pg.Begin(f.ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(f.ctx)
			if tc.sql != "" {
				if _, err = tx.Exec(f.ctx, tc.sql); err != nil {
					t.Fatal(err)
				}
			}
			if err = tc.run(tx); !errors.Is(err, ErrIntegrity) && !errors.Is(err, ErrInvalid) {
				t.Fatalf("public append integrity: %v", err)
			}
		})
	}
	if PoolTopicID(f.real.Pool) != poolTopic(f.real.Pool) {
		t.Fatal("pool topic differs")
	}
}

func TestCanonicalReadPropagatesStorageRejections(t *testing.T) {
	ctx, pool, s, market := localReadFixture(t)
	for _, tc := range []struct {
		name, change, restore string
		run                   func() error
	}{
		{"fences", "ALTER TABLE stats_local.accounting_pool_fences RENAME TO saved_fences", "ALTER TABLE stats_local.saved_fences RENAME TO accounting_pool_fences", func() error { _, e := s.Trades(ctx, market, "", 10, "", 200); return e }},
		{"source fills", "ALTER VIEW indexer.indexer_accounting_fills_v1 RENAME TO saved_fills", "ALTER VIEW indexer.saved_fills RENAME TO indexer_accounting_fills_v1", func() error { _, e := s.Window(ctx, market, 60, 200); return e }},
		{"trade facts", "ALTER TABLE stats_local.trade_facts RENAME TO saved_trades", "ALTER TABLE stats_local.saved_trades RENAME TO trade_facts", func() error { _, e := s.Trades(ctx, market, "", 10, "", 200); return e }},
		{"read snapshots", "ALTER TABLE stats_local.phase2_read_snapshots ADD CONSTRAINT rejected_snapshot CHECK(false) NOT VALID", "ALTER TABLE stats_local.phase2_read_snapshots DROP CONSTRAINT rejected_snapshot", func() error { _, e := s.Trades(ctx, market, "", 10, "", 200); return e }},
		{"window write", "ALTER TABLE stats_local.phase2_windows ADD CONSTRAINT rejected_window CHECK(false) NOT VALID", "ALTER TABLE stats_local.phase2_windows DROP CONSTRAINT rejected_window", func() error { _, e := s.Window(ctx, market, 60, 200); return e }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := pool.Exec(ctx, tc.change); err != nil {
				t.Fatal(err)
			}
			defer func() {
				if _, err := pool.Exec(ctx, tc.restore); err != nil {
					t.Error(err)
				}
			}()
			var sqlerr *pgconn.PgError
			if err := tc.run(); !errors.As(err, &sqlerr) || (sqlerr.Code != "42P01" && sqlerr.Code != "23514") {
				t.Fatalf("read storage: %v", err)
			}
		})
	}
}
