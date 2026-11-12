package engine

import (
	"context"
	"errors"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
)

func TestQualifiedCandleRejectsChangedSourceIdentityAndActorOverflow(t *testing.T) {
	source := seriesSource(0, 1704067260, "")
	initial, err := FoldQualified(nil, source, "1m", false)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		edit func(*QualifiedCandle)
	}{
		{"token", func(bar *QualifiedCandle) { bar.Token.Address = "0x0000000000000000000000000000000000000001" }},
		{"opening order", func(bar *QualifiedCandle) { bar.OpenSource.Position.Height = "999" }},
		{"price source", func(bar *QualifiedCandle) { bar.HighSource.SourceFillID = "changed" }},
		{"unknown actor count", func(bar *QualifiedCandle) { bar.UnknownActorTradeCount = ^uint64(0) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prior := initial
			tc.edit(&prior)
			_, err := FoldQualified(&prior, source, "1m", false)
			if !errors.Is(err, ErrInvalidFill) {
				t.Fatalf("changed qualified source admitted: %v", err)
			}
		})
	}
}

func TestExactFillRequiresIdentityBeforeDurableWrite(t *testing.T) {
	valid := fillFixture()
	for _, tc := range []struct {
		name string
		edit func(*Fill)
	}{
		{"emitter", func(f *Fill) { f.EmittingContract = "" }},
		{"code epoch", func(f *Fill) { f.CodeEpoch = "" }},
		{"pool", func(f *Fill) { f.PoolAddress = "" }},
		{"transaction", func(f *Fill) { f.TransactionHash = "" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fill := valid
			tc.edit(&fill)
			if err := ProcessExactFill(context.Background(), nil, fill, 1); !errors.Is(err, ErrInvalidFill) {
				t.Fatalf("incomplete exact identity: %v", err)
			}
			if err := ProcessExactFillTx(context.Background(), nil, nil, fill, 1); !errors.Is(err, ErrInvalidFill) {
				t.Fatalf("incomplete exact transaction identity: %v", err)
			}
		})
	}
	if err := ProcessExactFillTx(context.Background(), nil, nil, valid, 1); !errors.Is(err, ErrInvalidFill) {
		t.Fatalf("missing durable transaction: %v", err)
	}
	qualified := seriesSource(0, 1704067260, "")
	if err := ApplyQualifiedTx(context.Background(), nil, nil, qualified, 1); !errors.Is(err, ErrInvalidFill) {
		t.Fatalf("missing qualified transaction: %v", err)
	}
	invalid := accounting.FillV1{}
	if err := ApplyQualifiedTx(context.Background(), nil, &store.ExactStore{}, invalid, 1); err == nil {
		t.Fatal("invalid canonical fill admitted")
	}
}
