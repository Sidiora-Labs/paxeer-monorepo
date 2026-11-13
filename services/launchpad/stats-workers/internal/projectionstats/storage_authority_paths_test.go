package projectionstats

import (
	"errors"
	"math/big"
	"strconv"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestPublicProjectionPropagatesUnavailableStorage(t *testing.T) {
	f := newPublicFixture(t)
	fills := f.ingestReal()
	fill := fills["sell"]
	p := f.projector()
	stamp, err := strconv.ParseInt(fill.BlockTimestampSeconds, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	anchor := publicAnchor{Position: Position(fill.Position), timestamp: stamp}
	for _, tc := range []struct {
		name, table string
		run         func(pgx.Tx) error
	}{
		{"swap source", "indexer.indexer_raw_logs_v1", func(tx pgx.Tx) error { _, e := p.swapPriceTx(f.ctx, tx, fill, stamp); return e }},
		{"reserve source", "stats.accounting_market_commits", func(tx pgx.Tx) error { _, e := p.realUsdlTx(f.ctx, tx, anchor, f.real.Pool, nil); return e }},
		{"latest fill", "stats.accounting_market_commits", func(tx pgx.Tx) error { _, e := p.latestFillTx(f.ctx, tx, anchor, f.real.Pool, -1); return e }},
		{"reserve checkpoint", "stats.public_stats_sync_baselines", func(tx pgx.Tx) error {
			_, e := p.syncEventsTx(f.ctx, tx, anchor, []string{f.real.EventEmitter}, f.real.Pool)
			return e
		}},
		{"reserve raw logs", "indexer.indexer_raw_logs_v1", func(tx pgx.Tx) error {
			_, e := p.syncEventsTx(f.ctx, tx, anchor, []string{f.real.EventEmitter}, f.real.Pool)
			return e
		}},
		{"holder metadata", publicTestHolders + ".tokens", func(tx pgx.Tx) error {
			_, e := p.holdingsAtAnchorTx(f.ctx, tx, anchor, f.real.Token, f.real.Pool, f.real.Transactions[0].From)
			return e
		}},
		{"holder transfers", "indexer.indexer_token_transfers_v1", func(tx pgx.Tx) error {
			_, e := p.holdingsAtAnchorTx(f.ctx, tx, anchor, f.real.Token, f.real.Pool, f.real.Transactions[0].From)
			return e
		}},
		{"holder balances", publicTestHolders + ".balances", func(tx pgx.Tx) error {
			_, e := p.holdingsAtAnchorTx(f.ctx, tx, anchor, f.real.Token, f.real.Pool, f.real.Transactions[0].From)
			return e
		}},
		{"pool totals", "stats.trade_facts", func(tx pgx.Tx) error {
			_, _, e := p.poolStatsTx(f.ctx, tx, anchor, nil, f.real.Pool, f.real.Token, f.real.Transactions[0].From, stamp)
			return e
		}},
		{"window baseline", "stats.accounting_market_commits", func(tx pgx.Tx) error {
			_, _, e := p.windowChange(f.ctx, tx, anchor, f.real.Pool, nil, nil, stamp, stamp-60)
			return e
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := f.pg.Begin(f.ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(f.ctx)
			if _, err = tx.Exec(f.ctx, "ALTER TABLE "+tc.table+" RENAME TO unavailable_storage"); err != nil {
				t.Fatal(err)
			}
			err = tc.run(tx)
			var pgerr *pgconn.PgError
			if !errors.As(err, &pgerr) || pgerr.Code != "42P01" {
				t.Fatalf("source storage rejection: %v", err)
			}
		})
	}
}

func TestPublicProjectionChecksRawSwapAuthority(t *testing.T) {
	f := newPublicFixture(t)
	fills := f.ingestReal()
	fill := fills["sell"]
	p := f.projector()
	stamp, err := strconv.ParseInt(fill.BlockTimestampSeconds, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, query string }{
		{"absent raw event", ""},
		{"topic shape", "UPDATE indexer.indexer_raw_logs SET topics='{}' WHERE transaction_hash=$1 AND log_index=$2"},
		{"ABI byte count", "UPDATE indexer.indexer_raw_logs SET data=decode('01','hex') WHERE transaction_hash=$1 AND log_index=$2"},
		{"emitter binding", "UPDATE indexer.indexer_raw_logs SET emitter='0x" + strings.Repeat("f", 40) + "' WHERE transaction_hash=$1 AND log_index=$2"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := f.pg.Begin(f.ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(f.ctx)
			checked := fill
			if tc.query == "" {
				checked.Position.LogIndex = "99999"
			} else if _, err = tx.Exec(f.ctx, tc.query, fill.Position.TransactionHash, fill.Position.LogIndex); err != nil {
				t.Fatal(err)
			}
			if _, err = p.swapPriceTx(f.ctx, tx, checked, stamp); !errors.Is(err, ErrIntegrity) {
				t.Fatalf("raw authority: %v", err)
			}
		})
	}
	tx, err := f.pg.Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(f.ctx)
	if _, err = p.swapPriceTx(f.ctx, tx, fill, stamp+1); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("event timestamp: %v", err)
	}
	for _, fields := range [][3]string{{"01", "0", "0"}, {"1", "01", "0"}, {"1", "0", "01"}} {
		if _, _, _, err = positionInts(fields[0], fields[1], fields[2]); !errors.Is(err, ErrIntegrity) {
			t.Fatalf("position %v: %v", fields, err)
		}
	}
	anchor := publicAnchor{Position: Position(fill.Position), timestamp: stamp}
	for _, tc := range []struct {
		name           string
		current        *priceEvent
		syncs          []syncEvent
		created, start int64
		want, quality  string
	}{
		{"unchanged old pool", nil, nil, stamp - 100, stamp - 50, "0", "complete"},
		{"zero baseline", &priceEvent{price: big.NewInt(1)}, []syncEvent{{priceEvent: priceEvent{height: big.NewInt(1), transactionIndex: big.NewInt(0), logIndex: big.NewInt(0), timestamp: 1, price: big.NewInt(0)}}}, 0, 2, "", "unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			value, quality, err := p.windowChange(f.ctx, tx, anchor, "0x"+strings.Repeat("f", 40), tc.syncs, tc.current, tc.created, tc.start)
			if err != nil || quality != tc.quality || (tc.want == "" && value != nil) || (tc.want != "" && (value == nil || *value != tc.want)) {
				t.Fatalf("price baseline: %v %s %v", value, quality, err)
			}
		})
	}
}

func TestAccountingTransactionRollsBackRejectedWrites(t *testing.T) {
	ctx, pool, service, _ := localReadFixture(t)
	r := service.repository
	in := input(fixture("buy", "7"), 119)
	for _, table := range []string{"inbox", "canonical_blocks", "canonical_transactions", "canonical_positions", "canonical_pool_assets", "minute_buckets", "trade_facts", "fee_groups", "outbox"} {
		t.Run(table, func(t *testing.T) {
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			if _, err = tx.Exec(ctx, "ALTER TABLE stats_local."+table+" ADD CHECK (false) NOT VALID"); err != nil {
				t.Fatal(err)
			}
			_, err = r.ApplyTx(ctx, tx, in)
			var pgerr *pgconn.PgError
			if !errors.As(err, &pgerr) || pgerr.Code != "23514" {
				t.Fatalf("accounting storage rejection: %v", err)
			}
			if err = tx.Rollback(ctx); err != nil {
				t.Fatal(err)
			}
			var count int
			if err = pool.QueryRow(ctx, "SELECT count(*) FROM stats_local.inbox").Scan(&count); err != nil || count != 0 {
				t.Fatalf("uncommitted inbox: %d %v", count, err)
			}
		})
	}
	for _, tc := range []struct {
		name string
		edit func(*ApplyInput)
	}{
		{"consumer absent", func(in *ApplyInput) { in.ConsumerVersion = "" }},
		{"projection length", func(in *ApplyInput) { in.ProjectionVersion = strings.Repeat("a", 65) }},
		{"future timestamp", func(in *ApplyInput) { in.BlockTimestamp = in.WatermarkTimestamp + 1 }},
		{"invalid fee", func(in *ApplyInput) { in.Trade.FeeRaw = "01" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			changed := in
			tc.edit(&changed)
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			want := ErrInvalid
			if tc.name == "future timestamp" {
				want = ErrIntegrity
			}
			if _, err = r.ApplyTx(ctx, tx, changed); !errors.Is(err, want) {
				t.Fatalf("accounting input: %v; want %v", err, want)
			}
		})
	}
	batchErr := &BatchApplyError{Index: 3, Cause: ErrIntegrity}
	if !errors.Is(batchErr, ErrIntegrity) || !strings.Contains(batchErr.Error(), "input3") {
		t.Fatalf("batch cause: %v", batchErr)
	}
	identityErr := &AccountingIdentityConflict{Pools: []string{in.Trade.PoolAddress}}
	if !errors.Is(identityErr, ErrIntegrity) || identityErr.Error() != "stats: canonical accounting identity conflict" {
		t.Fatalf("identity cause: %v", identityErr)
	}
}
