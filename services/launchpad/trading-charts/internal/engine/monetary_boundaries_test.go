package engine

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
)

func TestRationalPresentationAndSellFeeBoundaries(t *testing.T) {
	for _, scale := range []int{-1, 256} {
		if value, err := (Rational{"1", "2"}).Render(scale); value != "" || !errors.Is(err, ErrInvalidFill) {
			t.Fatalf("presentation scale %d: %q, %v", scale, value, err)
		}
	}
	if value, err := (Rational{"1", "0"}).Render(2); value != "" || !errors.Is(err, ErrInvalidFill) {
		t.Fatalf("zero denominator: %q, %v", value, err)
	}
	for _, tc := range []struct{ numerator, want string }{{"5", "2"}, {"7", "4"}} {
		if value, err := (Rational{tc.numerator, "2"}).Render(0); err != nil || value != tc.want {
			t.Fatalf("whole-unit half-even rounding: %q, %v", value, err)
		}
	}
	invalid := fillFixture()
	invalid.PaidInputRaw = "invalid"
	if price, err := invalid.FeeExclusivePrice(); price != nil || !errors.Is(err, ErrInvalidFill) {
		t.Fatalf("invalid gross price: %+v, %v", price, err)
	}
	sell := fillFixture()
	sell.Side, sell.PaidInputRaw, sell.FeeDisposition = "sell", sell.TokenAmountRaw, "retained_token_reserve"
	sell.FeeRaw = sell.PaidInputRaw
	priced, net := sell.PaidInputRaw, "0"
	sell.InputDerivationQuality, sell.PricedInputRaw, sell.AmmInputAfterFeeRaw = "exact", &priced, &net
	if price, err := sell.FeeExclusivePrice(); price != nil || !errors.Is(err, ErrInvalidFill) {
		t.Fatalf("zero net token input: %+v, %v", price, err)
	}
	if bar, err := FoldExact(nil, sell, "1m", true); err != nil || bar.SellVolumeUsdlRaw != sell.UsdlAmountRaw || bar.BuyVolumeUsdlRaw != "0" || bar.Trades != 1 {
		t.Fatalf("sell candle volumes: %+v, %v", bar, err)
	}
	if _, err := FoldExact(nil, sell, "unsupported", false); !errors.Is(err, ErrUnsupportedResolution) {
		t.Fatalf("unsupported exact interval: %v", err)
	}
	if _, err := BucketStart(-1, "1m"); !errors.Is(err, ErrInvalidFill) {
		t.Fatalf("negative bucket time: %v", err)
	}
}

func TestCanonicalCandleRejectsUnrenderableTimesAndVolumes(t *testing.T) {
	source := seriesSource(0, 1704067260, "")
	folded, err := FoldQualified(nil, source, "1m", false)
	if err != nil {
		t.Fatal(err)
	}
	bar, err := DescribeSeries(folded, SeriesCheckpoint{Height: 200, Timestamp: 1704067380}, time.Unix(1704067380, 0))
	if err != nil {
		t.Fatal(err)
	}
	bar.DisplayRevision = 1
	for _, tc := range []struct {
		name string
		edit func(*SeriesCandle)
	}{
		{"end after last calendar year", func(c *SeriesCandle) { c.Start = 253402300740; c.IntervalEnd = 253402300800 }},
		{"noncanonical volume", func(c *SeriesCandle) { c.VolumeUsdlRaw = "01" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			changed := bar
			tc.edit(&changed)
			if _, err := CanonicalCandle(changed, map[string]accounting.FillV1{source.SourceFillID: source}); !errors.Is(err, ErrInvalidFill) {
				t.Fatalf("invalid canonical candle: %v", err)
			}
		})
	}
	source.BlockTimestampSeconds = "253402300800"
	if err := accounting.ValidateFillV1(source); err != nil {
		t.Fatal(err)
	}
	if _, err := canonicalPrice(bar.Open, qualifiedSource(source), bar, source); !errors.Is(err, ErrInvalidFill) {
		t.Fatalf("source time outside presentation calendar: %v", err)
	}
	checkpoint, _, _ := retainedArchiveSeed(t)
	checkpoint.Manifest, checkpoint.RetainedManifest = strings.Repeat("z", 64), strings.Repeat("z", 64)
	raw, err := json.Marshal(checkpoint)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := decodeReconstructionCheckpoint(raw, checkpoint.Pool, checkpoint.From, checkpoint.To, checkpoint.Manifest, checkpoint.Epoch); !errors.Is(err, ErrReconstructionCheckpoint) {
		t.Fatalf("nonhex manifest commitment: %v", err)
	}
	cache := newDedupCache(time.Second, 1)
	now := time.Unix(100, 0)
	cache.now = func() time.Time { return now }
	cache.add("expired")
	now = now.Add(2 * time.Second)
	cache.add("fresh")
	if len(cache.entries) != 1 || cache.has("expired") || !cache.has("fresh") {
		t.Fatalf("expired entries at capacity: %v", cache.entries)
	}
}

func TestDevelopmentStorageRejectsUnavailableRelations(t *testing.T) {
	for _, tc := range []struct {
		name, statement, want string
		call                  func(*Builder, developmentKV, SwapEvent) error
	}{
		{"candle key time", "", "invalid syntax", func(_ *Builder, kv developmentKV, s SwapEvent) error {
			_, err := kv.Get(developmentKey("candle", s.PoolAddress, "1m", "invalid"))
			return err
		}},
		{"legacy candle relation", `ALTER TABLE candles.candles RENAME TO candles_held`, "candles.candles", func(_ *Builder, kv developmentKV, s SwapEvent) error {
			_, err := kv.Get(developmentKey("candle", s.PoolAddress, "1m", "1704067260"))
			return err
		}},
		{"legacy cursor relation", `ALTER TABLE candles.candle_cursors RENAME TO candle_cursors_held`, "candle_cursors", func(_ *Builder, kv developmentKV, s SwapEvent) error {
			_, err := kv.Get(developmentKey("cursor", s.PoolAddress, "1m"))
			return err
		}},
		{"candle write", `ALTER TABLE candles.candles RENAME TO candles_held`, "candles.candles", func(_ *Builder, kv developmentKV, s SwapEvent) error {
			return saveDevelopment(kv, developmentKey("candle", s.PoolAddress, "1m", "1704067260"), developmentCandle{})
		}},
		{"cursor write", `ALTER TABLE candles.candle_cursors RENAME TO candle_cursors_held`, "candle_cursors", func(_ *Builder, kv developmentKV, s SwapEvent) error {
			return saveDevelopment(kv, developmentKey("cursor", s.PoolAddress, "1m"), store.CursorRow{})
		}},
		{"source claim", `ALTER TABLE candles.processed_swaps RENAME TO processed_swaps_held`, "processed_swaps", func(_ *Builder, kv developmentKV, s SwapEvent) error {
			_, err := kv.Claim(s)
			return err
		}},
		{"unencodable document", "", "unsupported type", func(_ *Builder, kv developmentKV, s SwapEvent) error {
			return saveDevelopment(kv, developmentKey("outbox", s.PoolAddress), make(chan int))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			builder, pool := chartDevelopmentBuilder(t)
			if tc.statement != "" {
				if _, err := pool.Exec(ctx, tc.statement); err != nil {
					t.Fatal(err)
				}
			}
			swap := developmentCandleTestSwap(0, 1704067260, "1000000000000000000")
			err := builder.storage.Transaction(ctx, swap.PoolAddress, swap.TxHash, func(kv developmentKV) error { return tc.call(builder, kv, swap) })
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("unavailable development storage: %v", err)
			}
			var count int
			if err := pool.QueryRow(ctx, `SELECT count(*) FROM candles.development_state`).Scan(&count); err != nil || count != 0 {
				t.Fatalf("failed transaction retained state: %d, %v", count, err)
			}
		})
	}
}

func TestDevelopmentFoldRollsBackRejectedWrites(t *testing.T) {
	for _, relation := range []string{"candles", "candle_cursors"} {
		t.Run(relation, func(t *testing.T) {
			ctx := context.Background()
			builder, pool := chartDevelopmentBuilder(t)
			swap := developmentCandleTestSwap(0, 1704067260, "1000000000000000000")
			if err := builder.ProcessSwap(ctx, swap); err != nil {
				t.Fatal(err)
			}
			if _, err := pool.Exec(ctx, `CREATE FUNCTION candles.reject_write() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'candle write refused'; END $$; CREATE TRIGGER reject_write BEFORE INSERT OR UPDATE ON candles.`+relation+` FOR EACH ROW EXECUTE FUNCTION candles.reject_write()`); err != nil {
				t.Fatal(err)
			}
			next := developmentCandleTestSwap(1, 1704067260, "2000000000000000000")
			if err := builder.ProcessSwap(ctx, next); err == nil || !strings.Contains(err.Error(), "candle write refused") {
				t.Fatalf("rejected candle write: %v", err)
			}
			var count int
			if err := pool.QueryRow(ctx, `SELECT count(*) FROM candles.processed_swaps`).Scan(&count); err != nil || count != 1 {
				t.Fatalf("rolled back source claim: %d, %v", count, err)
			}
		})
	}
	t.Run("outbox read and Redis authority", func(t *testing.T) {
		ctx := context.Background()
		builder, pool := chartDevelopmentBuilder(t)
		swap := developmentCandleTestSwap(0, 1704067260, "1000000000000000000")
		if err := builder.storage.Transaction(ctx, swap.PoolAddress, swap.TxHash, func(kv developmentKV) error { return builder.foldSwap(kv, swap) }); err != nil {
			t.Fatal(err)
		}
		builder.redis = nil
		if err := builder.RelayCommitted(ctx, swap.PoolAddress); err == nil || !strings.Contains(err.Error(), "Redis delivery unavailable") {
			t.Fatalf("missing delivery authority: %v", err)
		}
		if _, err := pool.Exec(ctx, `UPDATE candles.development_state SET payload='invalid'::bytea WHERE key=$1`, developmentKey("outbox", swap.PoolAddress)); err != nil {
			t.Fatal(err)
		}
		if err := builder.RelayCommitted(ctx, swap.PoolAddress); err == nil || !strings.Contains(err.Error(), "invalid character") {
			t.Fatalf("malformed committed outbox: %v", err)
		}
		pool.Close()
		if err := builder.RelayCommitted(ctx, swap.PoolAddress); err == nil || !strings.Contains(err.Error(), "closed pool") {
			t.Fatalf("closed transaction store: %v", err)
		}
	})
}

func TestExactTransactionsRejectCorruptStoredEconomics(t *testing.T) {
	ctx := context.Background()
	pool := internaltest.OpenPool(t, internaltest.NewDatabase(t))
	ddl, err := store.ExactSchemaDDL("candles")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	exact, err := store.NewExact(pool, "candles")
	if err != nil {
		t.Fatal(err)
	}
	fill := fillFixture()
	bar, err := FoldExact(nil, fill, "1m", false)
	if err != nil {
		t.Fatal(err)
	}
	bar.VolumeUsdlRaw = "invalid"
	raw, err := json.Marshal(bar)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO candles.chart_exact(chain_id,pool,interval,start,snapshot) VALUES('125',$1,'1m',$2,$3)`, fill.PoolAddress, bar.Start, raw); err != nil {
		t.Fatal(err)
	}
	if err := ProcessExactFill(ctx, exact, fill, 1); !errors.Is(err, ErrInvalidFill) {
		t.Fatalf("invalid stored economics: %v", err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	observed := ProcessExactFillTx(ctx, tx, exact, fill, 1)
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(observed, ErrInvalidFill) {
		t.Fatalf("outer transaction stored economics: %v", observed)
	}
	tx, err = pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := appendCandleStreamTx(ctx, tx, exact, delivery.StreamWriter{}, SeriesCandle{}, accounting.FillV1{}, accounting.PartitionPosition{}, "canonical", nil); !errors.Is(err, ErrInvalidFill) {
		t.Fatalf("unbound stream writer: %v", err)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM candles.chart_inbox`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("corrupt economics claimed a fill: %d, %v", count, err)
	}
}

func TestExactCandleWriteRejectionRollsBackAllIntervals(t *testing.T) {
	ctx := context.Background()
	pool := internaltest.OpenPool(t, internaltest.NewDatabase(t))
	ddl, err := store.ExactSchemaDDL("candles")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	exact, err := store.NewExact(pool, "candles")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `CREATE FUNCTION candles.reject_exact() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'exact write refused'; END $$; CREATE TRIGGER reject_exact BEFORE INSERT ON candles.chart_exact FOR EACH ROW EXECUTE FUNCTION candles.reject_exact()`); err != nil {
		t.Fatal(err)
	}
	fill := fillFixture()
	if err := ProcessExactFill(ctx, exact, fill, 1); err == nil || !strings.Contains(err.Error(), "exact write refused") {
		t.Fatalf("rejected exact write: %v", err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	observed := ProcessExactFillTx(ctx, tx, exact, fill, 1)
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if observed == nil || !strings.Contains(observed.Error(), "exact write refused") {
		t.Fatalf("rejected outer exact write: %v", observed)
	}
	for _, relation := range []string{"chart_inbox", "chart_traders", "chart_exact", "chart_outbox"} {
		var count int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM candles.`+relation).Scan(&count); err != nil || count != 0 {
			t.Fatalf("rolled back %s: %d, %v", relation, count, err)
		}
	}
}
