package projectionstats

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestPublicStatisticsRejectsChangedCommittedSources(t *testing.T) {
	f := newPublicFixture(t)
	fills := f.ingestReal()
	fill := fills["sell"]
	p := f.projector()
	anchor := publicAnchor{Position: Position(fill.Position)}
	watermark := CanonicalWatermark{Height: fill.Position.Height}
	for _, tc := range []struct {
		name, sql string
		run       func(pgx.Tx) error
		want      error
		code      string
	}{
		{name: "anchor trade relation", sql: "ALTER TABLE stats.trade_facts RENAME TO saved_trades", code: "42P01", run: func(tx pgx.Tx) error { _, _, e := p.anchorTx(f.ctx, tx, watermark); return e }},
		{name: "anchor block relation", sql: "ALTER VIEW indexer.indexer_block_completeness_v1 RENAME TO saved_blocks", code: "42P01", run: func(tx pgx.Tx) error { _, _, e := p.anchorTx(f.ctx, tx, watermark); return e }},
		{name: "anchor completeness", sql: "UPDATE indexer.indexer_block_completeness SET required_decode_complete=false", want: ErrUnavailable, run: func(tx pgx.Tx) error { _, _, e := p.anchorTx(f.ctx, tx, watermark); return e }},
		{name: "anchor hash", sql: `UPDATE stats.trade_facts SET payload=jsonb_set(payload,'{position,blockHash}','"invalid"')`, want: ErrIntegrity, run: func(tx pgx.Tx) error { _, _, e := p.anchorTx(f.ctx, tx, watermark); return e }},
		{name: "emitter relation", sql: "ALTER TABLE stats.accounting_market_commits RENAME TO saved_commits", code: "42P01", run: func(tx pgx.Tx) error { _, e := p.emittersTx(f.ctx, tx); return e }},
		{name: "emitter address", sql: `UPDATE stats.accounting_market_commits SET fill=jsonb_set(fill,'{actualEmitter}','"invalid"')`, want: ErrIntegrity, run: func(tx pgx.Tx) error { _, e := p.emittersTx(f.ctx, tx); return e }},
		{name: "emitter value", sql: `UPDATE stats.accounting_market_commits SET fill=fill-'actualEmitter'`, code: "", run: func(tx pgx.Tx) error { _, e := p.emittersTx(f.ctx, tx); return e }},
		{name: "latest fill document", sql: `UPDATE stats.accounting_market_commits SET fill=jsonb_set(fill,'{side}','"invalid"')`, want: ErrIntegrity, run: func(tx pgx.Tx) error { _, e := p.latestFillTx(f.ctx, tx, anchor, f.real.Pool, -1); return e }},
		{name: "latest timestamp", sql: `UPDATE stats.trade_facts SET block_timestamp=block_timestamp+1`, want: ErrIntegrity, run: func(tx pgx.Tx) error { _, e := p.latestFillTx(f.ctx, tx, anchor, f.real.Pool, -1); return e }},
		{name: "backing side", sql: `UPDATE stats.accounting_market_commits SET fill=jsonb_set(fill,'{side}','"invalid"')`, want: ErrIntegrity, run: func(tx pgx.Tx) error { _, e := p.realUsdlTx(f.ctx, tx, anchor, f.real.Pool, nil); return e }},
		{name: "backing underflow", sql: `UPDATE stats.accounting_market_commits SET fill=jsonb_set(fill,'{usdlAmountRaw}',to_jsonb(repeat('9',77))) WHERE fill->>'side'='sell'`, want: ErrIntegrity, run: func(tx pgx.Tx) error { _, e := p.realUsdlTx(f.ctx, tx, anchor, f.real.Pool, nil); return e }},
		{name: "holder supply conservation", sql: "UPDATE " + publicTestHolders + ".tokens SET supply=0", want: ErrIntegrity, run: func(tx pgx.Tx) error {
			_, e := p.holdingsAtAnchorTx(f.ctx, tx, anchor, f.real.Token, f.real.Pool, f.real.Transactions[0].From)
			return e
		}},
		{name: "stats stream relation", sql: "ALTER TABLE api_stream.stream_projection RENAME TO saved_stream", code: "42P01", run: func(tx pgx.Tx) error {
			_, e := p.appendTx(f.ctx, tx, anchor, PublicPoolStats{PoolAddress: f.real.Pool}, time.Now())
			return e
		}},
		{name: "price stream relation", sql: "ALTER TABLE api_stream.stream_projection RENAME TO saved_stream", code: "42P01", run: func(tx pgx.Tx) error {
			_, e := p.appendPriceTx(f.ctx, tx, committedFill{fill: fill}, time.Now())
			return e
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := f.pg.Begin(f.ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(f.ctx)
			if _, err = tx.Exec(f.ctx, tc.sql); err != nil {
				t.Fatal(err)
			}
			err = tc.run(tx)
			if tc.want != nil {
				if !errors.Is(err, tc.want) {
					t.Fatalf("committed source: %v, want %v", err, tc.want)
				}
			} else if tc.code != "" {
				var pgerr *pgconn.PgError
				if !errors.As(err, &pgerr) || pgerr.Code != tc.code {
					t.Fatalf("source storage: %v, want SQLSTATE %s", err, tc.code)
				}
			} else if err == nil || !strings.Contains(err.Error(), "cannot scan NULL") {
				t.Fatalf("missing emitter: %v", err)
			}
		})
	}
}
