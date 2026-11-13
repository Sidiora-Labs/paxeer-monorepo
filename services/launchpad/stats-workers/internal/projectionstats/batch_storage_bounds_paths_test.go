package projectionstats

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestBatchAccumulatorPreservesExactAmountsAndRejectsInvalidState(t *testing.T) {
	ctx, r := repository(t)
	in := input(fixture("buy", "7"), 119)
	amounts, err := Account(in.Trade, in.Assets, 30)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, setup string
		edit        func(*ApplyInput, *Accounting, *statsBatch)
		want        error
		state       string
	}{
		{name: "timestamp", edit: func(in *ApplyInput, _ *Accounting, _ *statsBatch) { in.BlockTimestamp = -1 }, want: ErrInvalid},
		{name: "volume", edit: func(_ *ApplyInput, a *Accounting, _ *statsBatch) { a.TradedUSDLRaw = "01" }, want: ErrInvalid},
		{name: "fee", edit: func(in *ApplyInput, _ *Accounting, _ *statsBatch) { in.Trade.FeeRaw = "01" }, want: ErrInvalid},
		{name: "revision overflow", edit: func(in *ApplyInput, _ *Accounting, b *statsBatch) {
			b.buckets[batchBucketKey{in.Trade.PoolAddress, in.ProjectionVersion, 60}] = &batchBucket{revision: 1<<63 - 1, volume: new(big.Int), close: in.Trade}
		}, want: ErrIntegrity},
		{name: "bucket unavailable", setup: "ALTER TABLE " + r.schema + ".minute_buckets RENAME TO saved_buckets", state: "42P01"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := r.pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			if tc.setup != "" {
				if _, err = tx.Exec(ctx, tc.setup); err != nil {
					t.Fatal(err)
				}
			}
			changed, a, b := in, amounts, newStatsBatch()
			if tc.edit != nil {
				tc.edit(&changed, &a, b)
			}
			_, err = b.enter(ctx, tx, r, changed, a)
			if tc.want != nil {
				if !errors.Is(err, tc.want) {
					t.Fatalf("batch validation: %v", err)
				}
			} else {
				var sqlerr *pgconn.PgError
				if !errors.As(err, &sqlerr) || sqlerr.Code != tc.state {
					t.Fatalf("batch storage: %v", err)
				}
			}
		})
	}
	for _, table := range []string{"minute_buckets", "fee_groups"} {
		t.Run(table, func(t *testing.T) {
			tx, err := r.pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			b := newStatsBatch()
			if revision, err := b.enter(ctx, tx, r, in, amounts); err != nil || revision != 1 {
				t.Fatalf("batch input: %d %v", revision, err)
			}
			if _, err = tx.Exec(ctx, "ALTER TABLE "+r.schema+"."+table+" ADD CHECK(false) NOT VALID"); err != nil {
				t.Fatal(err)
			}
			var sqlerr *pgconn.PgError
			if err = b.flush(ctx, tx, r); !errors.As(err, &sqlerr) || sqlerr.Code != "23514" {
				t.Fatalf("batch write: %v", err)
			}
		})
	}
}

func TestTradingFeeReconciliationUsesEveryAssetUnitAndDisposition(t *testing.T) {
	watermark := fixture("buy", "7").Position
	fees := []AssetFee{{token, 18, valueRetainedTokenReserve, "2"}, {token, 6, valueRetainedTokenReserve, "3"}, {token, 6, valueDistributedUsdl, "4"}}
	got, err := ReconcileTradingFees(fees, []AssetFee{{usdl, 6, valueDistributedUsdl, "5"}}, "0", watermark, "1")
	if err != nil || got.Watermark != watermark || len(got.Disagreements) != 4 {
		t.Fatalf("fee disagreement: %+v %v", got, err)
	}
	for i, d := range got.Disagreements {
		if d.FromHeight != "0" || d.ThroughHeight != watermark.Height || d.Revision != "1" || d.ChainID != "125" || d.RawUnit == "" {
			t.Fatalf("fee units %d: %+v", i, d)
		}
		if i > 0 {
			p := got.Disagreements[i-1]
			if p.AssetAddress > d.AssetAddress || p.AssetAddress == d.AssetAddress && (p.AssetDecimals > d.AssetDecimals || p.AssetDecimals == d.AssetDecimals && p.Disposition > d.Disposition) {
				t.Fatalf("fee ordering: %+v", got.Disagreements)
			}
		}
		if d.AssetAddress == usdl {
			if d.ExpectedRaw != "0" || d.ObservedRaw != "5" {
				t.Fatal(d)
			}
		} else if d.ObservedRaw != "0" {
			t.Fatal(d)
		}
	}
	for _, side := range []bool{false, true} {
		for _, fee := range []AssetFee{{"invalid", 6, valueDistributedUsdl, "1"}, {token, -1, valueDistributedUsdl, "1"}, {token, 6, "unknown", "1"}, {token, 6, valueDistributedUsdl, "01"}} {
			expected, observed := []AssetFee{fee}, []AssetFee(nil)
			if side {
				expected, observed = observed, expected
			}
			if _, err := ReconcileTradingFees(expected, observed, "0", watermark, "1"); !errors.Is(err, ErrInvalid) {
				t.Fatalf("malformed fee: %+v %v", fee, err)
			}
		}
	}
}

func TestTradeCursorRejectsChangedWatermarkAndTrailingValues(t *testing.T) {
	f := Filter{token, "", "v1"}
	position := fixture("buy", "7").Position
	for _, tc := range []struct {
		watermark string
		p         Position
	}{{"invalid", position}, {"0", position}} {
		if _, err := EncodeCursor(f, tc.p, tc.watermark); !errors.Is(err, ErrInvalid) {
			t.Fatalf("cursor bounds: %v", err)
		}
	}
	valid, err := EncodeCursor(f, position, position.Height)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeCursor(f, valid)
	if err != nil || decoded.Position != position {
		t.Fatalf("cursor roundtrip: %+v %v", decoded, err)
	}
	for _, tc := range []struct {
		name   string
		edit   func(*Cursor)
		suffix string
	}{{name: "watermark", edit: func(c *Cursor) { c.WatermarkHeight = "invalid" }}, {name: "future position", edit: func(c *Cursor) { c.WatermarkHeight = "0" }}, {name: "extra value", suffix: " null"}, {name: "unknown field", suffix: "field"}} {
		t.Run(tc.name, func(t *testing.T) {
			c := decoded
			if tc.edit != nil {
				tc.edit(&c)
			}
			raw, err := json.Marshal(c)
			if err != nil {
				t.Fatal(err)
			}
			if tc.suffix == "field" {
				raw = []byte(strings.TrimSuffix(string(raw), "}") + `,"unknown":1}`)
			} else {
				raw = append(raw, tc.suffix...)
			}
			if _, err := DecodeCursor(f, base64.RawURLEncoding.EncodeToString(raw)); !errors.Is(err, ErrInvalid) {
				t.Fatalf("cursor integrity: %v", err)
			}
		})
	}
}
