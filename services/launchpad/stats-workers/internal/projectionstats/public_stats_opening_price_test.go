package projectionstats

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func (f *publicFixture) openingState(virtual, real, token string, height, logIndex int64) {
	f.t.Helper()
	first := f.real.Transactions[0]
	poolID := poolTopic(f.real.Pool)
	f.exec(`INSERT INTO indexer.pools(pool_address,token_address,creator,optical,pool_id,created_at,created_block,tx_hash) VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT DO NOTHING`, strings.ToUpper(f.real.Pool), f.real.Token, first.From, f.real.Factory, poolID, f.real.Blocks[strconv.FormatInt(first.BlockNumber, 10)].Timestamp, first.BlockNumber, first.Hash)
	f.exec(`INSERT INTO indexer.pool_state_snapshots(id,pool_id,virtual_reserve,real_reserve,token_reserve,price,block_number,block_timestamp,tx_hash,log_index) VALUES($1,$2,$3,$4,$5,'0',$6,$7,$8,$9)`, strconv.FormatInt(height, 10)+":"+strconv.FormatInt(logIndex, 10), poolID, virtual, real, token, height, f.real.Blocks[strconv.FormatInt(first.BlockNumber, 10)].Timestamp, first.Hash, logIndex)
}

func (f *publicFixture) creationHoldings() {
	f.t.Helper()
	first := f.real.Transactions[0]
	if err := f.holdings.Discover(f.ctx, f.real.Token, f.real.TokenDecimals, strconv.FormatInt(first.BlockNumber, 10), true); err != nil {
		f.t.Fatal(err)
	}
	for _, l := range first.Logs {
		if l.Address == f.real.Token && l.Topics[0] == transferTopic {
			f.rawLog(first, l)
			f.transfer(first, l)
		}
	}
}

func TestPublicStatsOpeningPriceCreation(t *testing.T) {
	for _, tc := range []struct {
		name, virtual, real, token, price string
		holdings                          bool
		heightOffset, logOffset           int64
	}{
		{"creation holdings", "10000000000", "0", "1000000000000000", "10000000000000", true, 0, 0},
		{"holdings unavailable", "10000000000", "0", "1000000000000000", "10000000000000", false, 0, 0},
		{"real reserve and rounding", "2", "3", "3", "1666666666666666666", false, 0, 0},
		{"missing state", "", "", "", "", true, 0, 0},
		{"zero token reserve", "10000000000", "0", "0", "", true, 0, 0},
		{"later block", "10000000000", "0", "1000000000000000", "", true, 1, 0},
		{"later log", "10000000000", "0", "1000000000000000", "", true, 0, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newPublicFixture(t)
			first := f.real.Transactions[0]
			f.creation(first, 14)
			if tc.virtual != "" {
				f.openingState(tc.virtual, tc.real, tc.token, first.BlockNumber+tc.heightOffset, 14+tc.logOffset)
			}
			if tc.holdings {
				f.creationHoldings()
			}
			f.advance(first.BlockNumber, first.BlockHash)
			result, err := f.projector().RefreshOnce(f.ctx, time.Unix(f.real.Blocks["33963749"].Timestamp+60, 0))
			if err != nil || result.Appended != 1 || result.Prices != 0 {
				t.Fatal(result, err)
			}
			doc := f.frame(f.real.Pool).frame.Payload
			if tc.price == "" {
				if doc.SpotPriceWad != nil || doc.SpotPriceUsdlRaw != nil || doc.TotalSupplyValuationUsdlRaw != nil || doc.CirculatingMarketCapUsdlRaw != nil || doc.SupplyValuationQuality != valueUnavailable {
					t.Fatalf("unpriced creation: %+v", doc)
				}
			} else {
				if doc.SpotPriceWad == nil || *doc.SpotPriceWad != tc.price {
					t.Fatalf("opening mark: %+v", doc)
				}
				if tc.holdings {
					if doc.SpotPriceUsdlRaw == nil || *doc.SpotPriceUsdlRaw != "10" || doc.TotalSupplyValuationUsdlRaw == nil || *doc.TotalSupplyValuationUsdlRaw != "10000000000" || doc.CirculatingMarketCapUsdlRaw == nil || *doc.CirculatingMarketCapUsdlRaw != "0" || doc.SupplyValuationQuality != "verified" {
						t.Fatalf("creation valuation: %+v", doc)
					}
				} else if doc.SpotPriceUsdlRaw != nil || doc.TotalSupplyValuationUsdlRaw != nil || doc.CirculatingMarketCapUsdlRaw != nil || doc.SupplyValuationQuality != valueUnavailable {
					t.Fatalf("unmeasured holdings: %+v", doc)
				}
			}
			for i, change := range []*string{doc.PriceChangeBps24h, doc.PriceChangeBps5m, doc.PriceChangeBps1h, doc.PriceChangeBps6h} {
				quality := []string{doc.ChangeBaselineQuality, doc.ChangeBaselineQuality5m, doc.ChangeBaselineQuality1h, doc.ChangeBaselineQuality6h}[i]
				if change == nil || *change != "0" || quality != "insufficient_history" {
					t.Fatalf("creation window %d: %v %s", i, change, quality)
				}
			}
		})
	}
}

func TestPublicStatsOpeningPriceWindowBaseline(t *testing.T) {
	for _, tc := range []struct {
		name    string
		age     int64
		quality string
	}{
		{"creation inside window", 5, "insufficient_history"},
		{"creation at start", 276, "insufficient_history"},
		{"creation before window", 86400, "complete"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newPublicFixture(t)
			fills := f.ingestReal()
			first := f.real.Transactions[0]
			f.openingState("10000000000", "0", "1000000000000000", first.BlockNumber, 14)
			stamp := f.real.Blocks["33963749"].Timestamp
			f.exec(`UPDATE indexer.indexer_canonical_creations SET block_timestamp=$1`, stamp-tc.age)
			f.advance(33963761, f.real.Blocks["33963761"].Hash)
			result, err := f.projector().RefreshOnce(f.ctx, time.Unix(stamp+60, 0))
			if err != nil || result.Appended != 1 {
				t.Fatal(result, err)
			}
			doc := f.frame(f.real.Pool).frame.Payload
			if doc.SpotPriceWad == nil || *doc.SpotPriceWad != "10098226981692" || doc.SpotPriceUsdlRaw == nil || *doc.SpotPriceUsdlRaw != "10" || doc.TotalSupplyValuationUsdlRaw == nil || *doc.TotalSupplyValuationUsdlRaw != "10098226981" {
				t.Fatalf("event price and valuation: %+v", doc)
			}
			for i, change := range []*string{doc.PriceChangeBps24h, doc.PriceChangeBps5m, doc.PriceChangeBps1h, doc.PriceChangeBps6h} {
				quality := []string{doc.ChangeBaselineQuality, doc.ChangeBaselineQuality5m, doc.ChangeBaselineQuality1h, doc.ChangeBaselineQuality6h}[i]
				wantQuality := "insufficient_history"
				if stamp-tc.age < time.Unix(stamp+60, 0).Truncate(publicStatsClockStep).Unix()-[]int64{86400, 300, 3600, 21600}[i] {
					wantQuality = "complete"
				}
				if change == nil || *change != "98" || quality != wantQuality {
					t.Fatalf("window %d: %v %s, want 98 %s", i, change, quality, wantQuality)
				}
			}
			if doc.ChangeBaselineQuality5m != tc.quality {
				t.Fatal(doc.ChangeBaselineQuality5m, tc.quality)
			}
			f.assertPrice(fills["sell"], "2", 1)
		})
	}
}

func TestPublicStatsOpeningPriceLateState(t *testing.T) {
	f := newPublicFixture(t)
	first := f.real.Transactions[0]
	f.creation(first, 14)
	f.creationHoldings()
	f.advance(first.BlockNumber, first.BlockHash)
	now := time.Unix(f.real.Blocks["33963749"].Timestamp+60, 0)
	result, err := f.projector().RefreshOnce(f.ctx, now)
	if err != nil || result.Appended != 1 {
		t.Fatal(result, err)
	}
	before := f.frame(f.real.Pool)
	if before.frame.Payload.SpotPriceWad != nil || before.frame.Revision != "1" || before.outbox != 1 {
		t.Fatal(string(before.wire))
	}
	f.openingState("10000000000", "0", "1000000000000000", first.BlockNumber, 14)
	result, err = f.projector().RefreshOnce(f.ctx, now.Add(time.Minute))
	if err != nil || result.Appended != 1 || result.Prices != 0 {
		t.Fatal(result, err)
	}
	after := f.frame(f.real.Pool)
	if after.sequence != 2 || after.frame.Revision != "2" || after.outbox != 2 || after.frame.Payload.SourceUpperPosition != before.frame.Payload.SourceUpperPosition || after.frame.Payload.SpotPriceWad == nil || *after.frame.Payload.SpotPriceWad != "10000000000000" || after.frame.Payload.TotalSupplyValuationUsdlRaw == nil || *after.frame.Payload.TotalSupplyValuationUsdlRaw != "10000000000" {
		t.Fatal(string(after.wire))
	}
	result, err = f.projector().RefreshOnce(f.ctx, now.Add(2*time.Minute))
	if err != nil || result.Appended != 0 || result.Prices != 0 {
		t.Fatal(result, err)
	}
}

func TestPublicStatsOpeningPriceRuntimeRole(t *testing.T) {
	ctx := context.Background()
	database := coordinatortest.Bootstrap(t, os.Getenv("COORDINATOR_TEST_POSTGRES_DSN"), "stats_runtime")
	runtime := database.Runtimes["stats_runtime"]
	pool, err := pgxpool.New(ctx, runtime.DSN)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	want, err := production.LoadExpectedSchema(runtime.ExpectedSchema, runtime.ExpectedSchemaSHA256)
	if err != nil {
		t.Fatal(err)
	}
	if err = production.CheckReadiness(ctx, pool, want); err != nil {
		t.Fatal(err)
	}
	admin, err := pgxpool.New(ctx, database.AdminDSN)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(admin.Close)
	var real realPool125
	raw, err := os.ReadFile("testdata/real_pool_125.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &real); err != nil {
		t.Fatal(err)
	}
	f := &publicFixture{t: t, ctx: ctx, pg: admin, real: real}
	first := real.Transactions[0]
	f.creation(first, 14)
	f.openingState("10000000000", "0", "1000000000000000", first.BlockNumber, 14)
	repo, err := NewRepository(pool, "stats")
	if err != nil {
		t.Fatal(err)
	}
	f.repo = repo
	p := f.projector()
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	price, err := p.openingPriceTx(ctx, tx, real.Pool)
	if err != nil || price == nil || price.String() != "10000000000000" {
		t.Fatal(price, err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err = production.CheckReadiness(ctx, pool, want); err != nil {
		t.Fatal(err)
	}
}

func TestPublicStatsOpeningPriceSourceIntegrity(t *testing.T) {
	f := newPublicFixture(t)
	first := f.real.Transactions[0]
	f.creation(first, 14)
	f.openingState("10000000000", "0", "1000000000000000", first.BlockNumber, 14)
	for _, column := range []string{"virtual_reserve", "real_reserve", "token_reserve"} {
		t.Run(column, func(t *testing.T) {
			tx, err := f.pg.Begin(f.ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(f.ctx)
			if _, err = tx.Exec(f.ctx, "UPDATE indexer.pool_state_snapshots SET "+pgx.Identifier{column}.Sanitize()+"='-1'"); err != nil {
				t.Fatal(err)
			}
			if _, err = f.projector().openingPriceTx(f.ctx, tx, f.real.Pool); !errors.Is(err, ErrIntegrity) {
				t.Fatal(err)
			}
		})
	}
	for _, table := range []string{"pool_state_snapshots_v1", "pools_read_v1"} {
		t.Run(table, func(t *testing.T) {
			tx, err := f.pg.Begin(f.ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(f.ctx)
			if _, err = tx.Exec(f.ctx, "ALTER VIEW indexer."+table+" RENAME TO unavailable_source"); err != nil {
				t.Fatal(err)
			}
			if _, err = f.projector().openingPriceTx(f.ctx, tx, f.real.Pool); err == nil {
				t.Fatal("missing source accepted")
			}
		})
	}
	tx, err := f.pg.Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(f.ctx)
	stamp := f.real.Blocks["33963749"].Timestamp
	anchor := publicAnchor{Position: Position{ChainID: "125", Height: "33963749", TransactionIndex: "0", LogIndex: "0"}, timestamp: stamp}
	for _, current := range []*priceEvent{nil, {price: big.NewInt(1)}} {
		change, quality, err := f.projector().windowChange(f.ctx, tx, anchor, f.real.Pool, nil, current, big.NewInt(0), stamp-10, stamp-5)
		if err != nil {
			t.Fatal(err)
		}
		if current == nil {
			if change == nil || *change != "0" || quality != valueComplete {
				t.Fatal(change, quality)
			}
		} else if change != nil || quality != valueUnavailable {
			t.Fatal(change, quality)
		}
	}
}
