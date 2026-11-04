package card_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/card"
	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/store"
)

const oneToken = "1000000000000000000"

func newService(t *testing.T) (*card.Service, *store.Store) {
	t.Helper()
	st := store.New(internaltest.NewPostgres(t))
	internaltest.EnsureStatsSchema(t, st.Pool())
	internaltest.EnsureMetadataSchema(t, st.Pool())
	svc := card.New(st, "https://sidiora.fun", "https://og.example", func() int64 { return 1234 })
	return svc, st
}

func TestMintCapturesSnapshotAndPersists(t *testing.T) {
	ctx := context.Background()
	svc, st := newService(t)

	// Retirement also applies to owners with a complete position and metadata.
	if _, err := st.FoldTrade(ctx, store.TradeInput{
		ID: "0xb-0", UserAddress: "0xOwner", PoolAddress: "0xPool", TokenAddress: "0xToken",
		IsBuy: true, UsdlAmount: "1000000", TokenAmount: oneToken, Price: "0", Fee: "0",
		BlockNumber: 1, BlockTimestamp: 100, TxHash: "0xb",
	}); err != nil {
		t.Fatalf("fold: %v", err)
	}
	if _, err := st.Pool().Exec(ctx, `
		INSERT INTO stats.pool_stats (pool_address, token_address, price, market_cap, price_change_24h)
		VALUES ('0xpool','0xtoken','2500000','5000000','120')`); err != nil {
		t.Fatalf("seed stats: %v", err)
	}
	if _, err := st.Pool().Exec(ctx, `
		INSERT INTO metadata.token_metadata (token_address, pool_address, name, symbol, created_at)
		VALUES ('0xtoken','0xpool','Dogecoin','DOGE',1)`); err != nil {
		t.Fatalf("seed metadata: %v", err)
	}

	minted, err := svc.Mint(ctx, "0xowner", "0xpool")
	if minted != nil || !errors.Is(err, card.ErrMintRetired) {
		t.Fatalf("mint = %+v, %v; want nil, ErrMintRetired and no share URL", minted, err)
	}
	var cards, referrals int
	if err := st.Pool().QueryRow(ctx, `SELECT (SELECT count(*) FROM pnl.pnl_cards), (SELECT count(*) FROM pnl.referrals)`).Scan(&cards, &referrals); err != nil {
		t.Fatalf("read persisted publication counts: %v", err)
	}
	if cards != 0 || referrals != 0 {
		t.Fatalf("retired mint persisted cards=%d referrals=%d", cards, referrals)
	}
	position, err := st.GetPosition(ctx, "0xowner", "0xpool")
	if err != nil || position == nil || position.CurrentHoldings != oneToken {
		t.Fatalf("position after retired mint = %+v, %v", position, err)
	}

	// Authentic archived rows remain readable without reissuing a public link.
	row, snapshot := repair45ArchiveRow(t)
	if err := st.InsertCard(ctx, row); err != nil {
		t.Fatalf("persist archived disclosure: %v", err)
	}
	got, err := svc.Get(ctx, row.CardID)
	if err != nil || got == nil {
		t.Fatalf("get archived card = %+v, %v", got, err)
	}
	if got.CardID != row.CardID || got.ShortCode != row.ShortCode || got.CreatedAt != row.CreatedAt || !reflect.DeepEqual(got.Snapshot, snapshot) {
		t.Fatalf("hydrated archive differs from stored disclosure: %+v", got)
	}
	if got.ShareURL != "" || got.OgURL != "https://og.example/pnl/cards/"+row.CardID+"/og.png" {
		t.Fatalf("archive URLs = share %q, OG %q", got.ShareURL, got.OgURL)
	}
	persisted, err := st.GetCardByShortCode(ctx, row.ShortCode)
	if err != nil || persisted == nil {
		t.Fatalf("persisted archive changed: %+v, %v", persisted, err)
	}
	var persistedSnapshot card.Snapshot
	if err := json.Unmarshal(persisted.Snapshot, &persistedSnapshot); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(persistedSnapshot, snapshot) {
		t.Fatalf("archive read mutated stored disclosure: %+v", persistedSnapshot)
	}
}

func TestMintRejectsMissingPosition(t *testing.T) {
	ctx := context.Background()
	svc, _ := newService(t)
	if minted, err := svc.Mint(ctx, "0xnobody", "0xpool"); minted != nil || !errors.Is(err, card.ErrMintRetired) {
		t.Fatalf("mint = %+v, %v; want nil, ErrMintRetired and no share URL", minted, err)
	}
}

func TestGetUnknownCard(t *testing.T) {
	ctx := context.Background()
	svc, _ := newService(t)
	got, err := svc.Get(ctx, "missing")
	if err != nil || got != nil {
		t.Fatalf("got = %v err = %v, want nil/nil", got, err)
	}
}

func TestBuildRenderInputFormatting(t *testing.T) {
	price := "2500000"
	snap := card.Snapshot{
		Version:      1,
		TokenName:    "Dogecoin",
		TokenSymbol:  "DOGE",
		TokenAddress: "0xtoken",
		Position: card.Position{
			TotalUsdlSpent:    "4000000",
			TotalUsdlReceived: "5000000",
			AvgCostBasis:      "2000000",
			CurrentHoldings:   oneToken,
			RealizedPnlUsdl:   "3000000",
		},
		Market: card.Market{PriceWad: &price},
	}
	in := card.BuildRenderInput(snap, "sidiora.fun/pnl/abc1234")
	if in.Title != "Dogecoin" {
		t.Errorf("title = %s", in.Title)
	}
	// (5 + 2.5) / 4 = 1.875 -> "1.87x" (truncated to 2 dp).
	if in.Multiple != "1.87x" {
		t.Errorf("multiple = %s, want 1.87x", in.Multiple)
	}
	// 3 + (2.5 - 2) = 3.5 USDL.
	if in.Pnl != "+$3.50" || !in.PnlPositive {
		t.Errorf("pnl = %s positive = %v", in.Pnl, in.PnlPositive)
	}
	if in.Holdings != "1.00 DOGE held" {
		t.Errorf("holdings = %s", in.Holdings)
	}
}

func TestBuildRenderInputLossNoMarket(t *testing.T) {
	snap := card.Snapshot{
		TokenSymbol: "LOSS",
		Position: card.Position{
			TotalUsdlSpent:    "10000000",
			TotalUsdlReceived: "0",
			AvgCostBasis:      "10000000",
			CurrentHoldings:   "0",
			RealizedPnlUsdl:   "-6000000",
		},
	}
	in := card.BuildRenderInput(snap, "f")
	if in.Title != "$LOSS" {
		t.Errorf("title = %s, want $LOSS", in.Title)
	}
	if in.Pnl != "-$6.00" || in.PnlPositive {
		t.Errorf("pnl = %s positive = %v, want -$6.00 / false", in.Pnl, in.PnlPositive)
	}
	// Nothing received and nothing held -> 0.00x.
	if in.Multiple != "0.00x" {
		t.Errorf("multiple = %s, want 0.00x", in.Multiple)
	}
}
