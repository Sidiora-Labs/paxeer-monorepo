package l17

import (
	"errors"
	"math/big"
	"strings"
	"testing"
)

func TestAggregateRequiresConsistentSnapshotBindings(t *testing.T) {
	p := fixture()
	for _, tc := range []struct {
		name   string
		change func(*Position, *Watermark)
		want   error
	}{
		{"watermark", func(p *Position, w *Watermark) { w.Height = "bad" }, ErrInvalid},
		{"derived amount", func(p *Position, w *Watermark) { p.BalanceRaw = "bad" }, ErrInvalid},
		{"position wallet", func(p *Position, w *Watermark) { p.Wallet = token }, ErrInvalid},
		{"future position", func(p *Position, w *Watermark) { w.Height = "98" }, ErrInvalid},
		{"future revision", func(p *Position, w *Watermark) { p.SourceWatermark.ProjectionRevision = "2" }, ErrInvalid},
		{"quarantined snapshot", func(p *Position, w *Watermark) { w.Integrity = statusQuarantined }, ErrInvalid},
		{"price block disagreement", func(p *Position, w *Watermark) {
			w.Height = "100"
			p.SourceWatermark.Height = "100"
			p.MarkPrice.Source.Position.BlockHash = "0x" + strings.Repeat("c", 64)
		}, ErrConflict},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := fixture()
			w := p.SourceWatermark
			tc.change(&p, &w)
			positions := []Position{p}
			if tc.name == "price block disagreement" {
				q := fixture()
				q.PositionID = "125:prior"
				q.MarkPrice.Source.Position.Height = "98"
				positions = append([]Position{q}, p)
			}
			if _, err := Aggregate(wallet, "snapshot", w, positions); !errors.Is(err, tc.want) {
				t.Fatal(err)
			}
		})
	}
	p.BalanceRaw = "0"
	p.KnownQuantityRaw = "0"
	p.UnknownQuantityRaw = "0"
	p.KnownRemainingCostUsdlRaw = "0"
	p.RealizedBasisQuality = statusComplete
	got, value, err := Derive(p)
	if err != nil || value.Sign() != 0 || got.BasisCoverage != (Ratio{"1", "1"}) {
		t.Fatal(got, value, err)
	}
	summary, err := Aggregate(wallet, "snapshot", p.SourceWatermark, []Position{p})
	if err != nil || summary.RealizedPnlRaw == nil || *summary.RealizedPnlRaw != "25000000" || summary.CompleteValueUsdlRaw == nil || *summary.CompleteValueUsdlRaw != "0" {
		t.Fatal(summary, err)
	}
	for _, tc := range []struct{ owner, revision string }{{"invalid", "1"}, {wallet, "0"}, {wallet, new(big.Int).Lsh(big.NewInt(1), 256).String()}} {
		if _, err := CacheKey(tc.owner, wallet, tc.revision, true); !errors.Is(err, ErrInvalid) {
			t.Fatal(tc, err)
		}
	}
	if _, err := EquityChange("bad", "10", "0", "0", "0", "0", true); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
}
