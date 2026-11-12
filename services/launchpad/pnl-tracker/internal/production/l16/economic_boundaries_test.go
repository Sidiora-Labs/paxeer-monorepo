package l16

import (
	"context"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"math/big"
	"strings"
	"testing"
	"time"
)

func TestLegValidationRejectsUnboundEconomics(t *testing.T) {
	leg := testLeg(1)
	for _, tc := range []struct {
		name   string
		change func(*Leg)
	}{
		{"height", func(l *Leg) { l.Height = 0 }},
		{"quantity", func(l *Leg) { l.Quantity = "0" }},
		{"consideration", func(l *Leg) { s := "bad"; l.ActualUSDL = &s }},
		{"identity", func(l *Leg) { l.Identity.Chain = "1" }},
		{"wallet", func(l *Leg) { l.Wallet = "bad" }},
		{"settlement", func(l *Leg) { l.SettlementReference = "" }},
		{"predecessor", func(l *Leg) { l.Predecessor = strings.Repeat("p", 257) }},
		{"actor", func(l *Leg) { l.RouterCaller = "bad" }},
		{"buy attribution", func(l *Leg) { l.Payer = "" }},
		{"sell attribution", func(l *Leg) { l.Kind = sideSell; l.Payer = "" }},
		{"movement consideration", func(l *Leg) { l.Kind = movementMint }},
		{"kind", func(l *Leg) { l.Kind = "other" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bad := leg
			tc.change(&bad)
			if err := bad.Validate(); err == nil {
				t.Fatal(bad)
			}
			if _, err := Apply(Empty(), bad); err == nil {
				t.Fatal("invalid leg applied")
			}
		})
	}
}

func TestPortfolioBatchPreservesBasisStates(t *testing.T) {
	fill, publication := inventoryWireFixture(t, 1, sideBuy)
	row := portfolioRow{LogicalPartition: publication.Positions[0].PartitionKey, Position: publication.Positions[0], Fill: fill, Revision: 1}
	wallet := strings.Split(row.LogicalPartition, ":")[1]
	for _, tc := range []struct {
		name     string
		state    Inventory
		basis    string
		complete bool
	}{
		{"empty", Empty(), statusComplete, true},
		{"unknown", Inventory{Unknown: "7", Known: "0", Cost: "0", Realized: "0", UnknownProceeds: "1", IncompleteRealized: true, Version: 2}, statusUnknown, false},
		{"partial", Inventory{Unknown: "7", Known: "3", Cost: "9", Realized: "2", UnknownProceeds: "1", IncompleteRealized: true, Version: 2}, statusPartial, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			current := row
			current.Inventory = tc.state
			update, watermark, occurred, err := portfolioBatch("g1", wallet, publication.Positions[0].EffectIdentity, []portfolioRow{current})
			if err != nil || occurred == "" || watermark.Height != "100" || update.Summary.BasisQuality != tc.basis || (update.Summary.RealizedPnlRaw != nil) != tc.complete || !update.CompleteBatch {
				t.Fatal(update, watermark, occurred, err)
			}
			if tc.name == "empty" && (update.Summary.CompleteValueUsdlRaw == nil || update.Summary.PricingQuality != statusComplete) {
				t.Fatal(update)
			}
		})
	}
	row.Inventory = Empty()
	for _, change := range []func(*portfolioRow){
		func(r *portfolioRow) { r.Inventory.Known = "bad" },
		func(r *portfolioRow) { r.Fill.BlockTimestampSeconds = "overflow" },
		func(r *portfolioRow) { r.Position.PartitionKey = "125:bad:bad" },
		func(r *portfolioRow) { r.Revision = 0 },
	} {
		bad := row
		change(&bad)
		if _, _, _, err := portfolioBatch("g1", wallet, publication.Positions[0].EffectIdentity, []portfolioRow{bad}); err == nil {
			t.Fatal(bad)
		}
	}
}

func TestEconomicRecordsRejectUnverifiableMovements(t *testing.T) {
	leg := testLeg(1)
	at := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	m := movement{TransactionHash: leg.Identity.Transaction, LogIndex: 1, Height: 100, BlockHash: "0x" + strings.Repeat("b", 64), From: zeroAddress, To: leg.Wallet, Amount: "5"}
	record, include, err := normalizeMovement(leg.Wallet, leg.Token, m, nil, at, "movement")
	if err != nil || !include || record.Leg.Kind != movementMint {
		t.Fatal(record, include, err)
	}
	for _, tc := range []struct {
		name       string
		movement   movement
		settlement *receiptSettlement
		wantError  bool
	}{
		{"self", func() movement { x := m; x.From = x.To; return x }(), nil, false},
		{"zero", func() movement { x := m; x.Amount = "0"; return x }(), nil, false},
		{"negative height", func() movement { x := m; x.Height = -1; return x }(), nil, true},
		{"invalid quantity", func() movement { x := m; x.Amount = "bad"; return x }(), nil, true},
		{"different receipt movement", m, &receiptSettlement{}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, include, err := normalizeMovement(leg.Wallet, leg.Token, tc.movement, tc.settlement, at, "movement")
			if include || (err != nil) != tc.wantError {
				t.Fatal(include, err)
			}
		})
	}
	state, err := Apply(Empty(), record.Leg)
	if err != nil {
		t.Fatal(err)
	}
	record.State = state
	if actual, err := replayEconomicRecord(Empty(), record); err != nil || actual != state {
		t.Fatal(actual, err)
	}
	for _, change := range []func(*EconomicRecord){
		func(r *EconomicRecord) { r.Leg.Quantity = "bad" },
		func(r *EconomicRecord) { r.Allocation = &Disposal{} },
		func(r *EconomicRecord) { r.State.Known = "1" },
		func(r *EconomicRecord) { r.Leg.Kind = movementBurn; r.Allocation = &Disposal{} },
	} {
		bad := record
		change(&bad)
		if _, err := replayEconomicRecord(Empty(), bad); err == nil {
			t.Fatal(bad)
		}
	}
	invalid := Empty()
	invalid.Known = "bad"
	if invalid.CompleteRealized() != nil {
		t.Fatal("invalid inventory has complete realization")
	}
	if _, err := Average(invalid, nil); err == nil {
		t.Fatal("invalid inventory averaged")
	}
	if _, err := Unrealized(invalid, nil, false); err == nil {
		t.Fatal("invalid inventory valued")
	}
	overflow := new(big.Int).Lsh(big.NewInt(1), 256).String()
	for _, quantity := range []string{"0", overflow} {
		if _, err := Acquire(Empty(), quantity, nil); err == nil {
			t.Fatal(quantity)
		}
		if _, _, err := Dispose(Empty(), quantity, nil); err == nil {
			t.Fatal(quantity)
		}
	}
	if _, err := RebuildDay(context.Background(), invalid, nil, at, true); err == nil {
		t.Fatal("invalid seed accepted")
	}
}

func TestIntervalEconomicsRejectsInvalidAllocations(t *testing.T) {
	leg := testLeg(1)
	leg.Kind = sideSell
	leg.Payer = leg.Wallet
	leg.Sender = leg.Wallet
	at := time.Date(2026, 1, 1, 1, 0, 0, 0, time.UTC)
	record := EconomicRecord{TimedLeg: TimedLeg{Leg: leg, CommittedAt: at}, BlockHash: "0x" + strings.Repeat("b", 64), Allocation: &Disposal{Known: "7", Unknown: "3", Cost: "30", KnownProceeds: "70", UnknownProceeds: "30"}, FeeAsset: &accounting.Asset{Address: leg.Token, Decimals: 18}, FeeRaw: "1", GasPAXRaw: "2"}
	items, err := intervalRecords([]EconomicRecord{record, record}, time.Hour)
	if err != nil || len(items) != 1 || items[0].TradeCount != 1 || items[0].SaleCount != 1 || items[0].AccountingComplete || items[0].KnownNetGainUsdlRaw != "40" || items[0].UnknownProceedsUsdlRaw != "30" || items[0].GasPaxRaw != "2" || len(items[0].Fees) != 1 || items[0].Fees[0].AmountRaw != "1" {
		t.Fatal(items, err)
	}
	for _, tc := range []struct {
		name   string
		change func(*EconomicRecord)
	}{
		{"identity", func(r *EconomicRecord) { r.Leg.Identity.Chain = "1" }},
		{"quantity", func(r *EconomicRecord) { r.Leg.Quantity = "bad" }},
		{"allocation required", func(r *EconomicRecord) { r.Allocation = nil }},
		{"cost", func(r *EconomicRecord) { r.Allocation.Cost = "bad" }},
		{"known proceeds", func(r *EconomicRecord) { r.Allocation.KnownProceeds = "bad" }},
		{"unknown proceeds", func(r *EconomicRecord) { r.Allocation.UnknownProceeds = "bad" }},
		{"known quantity", func(r *EconomicRecord) { r.Allocation.Known = "bad" }},
		{"unknown quantity", func(r *EconomicRecord) { r.Allocation.Unknown = "bad" }},
		{"quantity balance", func(r *EconomicRecord) { r.Allocation.Unknown = "2" }},
		{"proceeds balance", func(r *EconomicRecord) { r.Allocation.UnknownProceeds = "29" }},
		{"fee", func(r *EconomicRecord) { r.FeeRaw = "bad" }},
		{"gas", func(r *EconomicRecord) { r.GasPAXRaw = "bad" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bad := record
			allocation := *record.Allocation
			bad.Allocation = &allocation
			tc.change(&bad)
			if _, err := intervalRecords([]EconomicRecord{bad}, time.Hour); err == nil {
				t.Fatal(bad)
			}
		})
	}
	changed := record
	changed.FeeRaw = "2"
	if _, err := intervalRecords([]EconomicRecord{record, changed}, time.Hour); err == nil {
		t.Fatal("conflicting record accepted")
	}
	changed = record
	changed.Leg.Identity.Log++
	changed.GasPAXRaw = "3"
	if _, err := intervalRecords([]EconomicRecord{record, changed}, time.Hour); err == nil {
		t.Fatal("conflicting transaction gas accepted")
	}
}
