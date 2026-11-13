package l17

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestHistoryCommitValidatesSnapshotAndRecordOrder(t *testing.T) {
	ctx := context.Background()
	r := phase2Repository(t)
	summary := phase2Commit(t, r, "history-binding-source", "1", 1)
	first := portfolioHistoryRecord(summary)
	for _, tc := range []struct {
		name                    string
		owner, snapshot, source string
		records                 []HistoryRecord
		want                    error
	}{
		{name: "owner syntax", owner: "invalid", records: []HistoryRecord{first}, want: ErrInvalid},
		{name: "snapshot syntax", snapshot: "invalid", records: []HistoryRecord{first}, want: ErrInvalid},
		{name: "source bound", source: strings.Repeat("s", 257), records: []HistoryRecord{first}, want: ErrInvalid},
		{name: "record schema", records: []HistoryRecord{{}}, want: ErrInvalid},
		{name: "duplicate identity", records: []HistoryRecord{first, first}, want: ErrInvalid},
		{name: "duplicate position", records: func() []HistoryRecord {
			other := first
			other.EventID = "other-event"
			return []HistoryRecord{first, other}
		}(), want: ErrInvalid},
		{name: "snapshot ownership", owner: token, records: []HistoryRecord{first}, want: ErrUnauthorized},
		{name: "missing snapshot", snapshot: uuid.NewString(), records: []HistoryRecord{first}, want: ErrUnauthorized},
		{name: "snapshot frontier", records: func() []HistoryRecord {
			other := first
			other.Position.Height = "999999"
			return []HistoryRecord{other}
		}(), want: ErrInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := r.Pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer rollbackPortfolio(tx)
			owner := tc.owner
			if owner == "" {
				owner = wallet
			}
			snapshot := tc.snapshot
			if snapshot == "" {
				snapshot = summary.SnapshotID
			}
			source := tc.source
			if source == "" {
				source = "history-bindings"
			}
			if err = r.CommitHistoryTx(ctx, tx, owner, snapshot, source, tc.records); !errors.Is(err, tc.want) {
				t.Fatal(err, tc.want)
			}
			var count int
			if err = tx.QueryRow(ctx, `SELECT count(*) FROM portfolio_history_snapshots`).Scan(&count); err != nil || count != 0 {
				t.Fatal(count, err)
			}
		})
	}
	if err := r.CommitHistoryTx(ctx, nil, wallet, summary.SnapshotID, "history", nil); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if err := r.CommitHistory(ctx, wallet, summary.SnapshotID, "history-positive", []HistoryRecord{first}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		snapshot     string
		after, limit int
		now          time.Time
		want         error
	}{
		{summary.SnapshotID, -2, 1, time.Now(), ErrInvalid},
		{"invalid", -1, 1, time.Now(), ErrInvalid},
		{uuid.NewString(), -1, 1, time.Now(), ErrRestart},
		{summary.SnapshotID, -1, 1, time.Now().Add(2 * time.Hour), ErrRestart},
	} {
		if _, err := r.HistoryPage(ctx, wallet, wallet, tc.snapshot, tc.after, tc.limit, tc.now); !errors.Is(err, tc.want) {
			t.Fatal(tc, err)
		}
	}
	if _, err := r.Pool.Exec(ctx, `UPDATE portfolio_history_rows SET payload='"invalid"'::jsonb WHERE snapshot_id=$1`, summary.SnapshotID); err != nil {
		t.Fatal(err)
	}
	if _, err := r.HistoryPage(ctx, wallet, wallet, summary.SnapshotID, -1, 10, time.Now()); err == nil {
		t.Fatal("untyped history returned")
	}
}

func TestHistoryMetadataBindsQuotePeriodsAndOwner(t *testing.T) {
	ctx := context.Background()
	r := phase2Repository(t)
	summary := phase2Commit(t, r, "metadata-binding-source", "1", 1)
	start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	chain, currency, quote := "125", "USDL", "0x85FcD13735F4309833A503EE804ea32395851479"
	decimals := 6
	base := SnapshotHistoryMetadata{SourceEvent: "history-metadata", Start: start, End: end, Quote: AssetRef{Kind: "erc20", ChainID: &chain, Address: &quote, Decimals: &decimals, CurrencyCode: &currency}, Periods: []ReturnPeriod{{Start: start, End: end, OpeningEquityRaw: "100", ClosingEquityBeforeFlowRaw: "110", ExternalFlowAtEndRaw: "0", Complete: true}}}
	for _, tc := range []struct {
		name   string
		change func(*SnapshotHistoryMetadata)
		owner  string
		valid  bool
	}{
		{name: "complete periods", valid: true},
		{name: "source identity", change: func(m *SnapshotHistoryMetadata) { m.SourceEvent = "" }},
		{name: "quote kind", change: func(m *SnapshotHistoryMetadata) { m.Quote.Kind = "native" }},
		{name: "quote syntax", change: func(m *SnapshotHistoryMetadata) { m.Quote.Address = ptr("invalid") }},
		{name: "quote identity", change: func(m *SnapshotHistoryMetadata) { m.Quote.Address = ptr(token) }},
		{name: "period range", change: func(m *SnapshotHistoryMetadata) { m.Periods[0].Start = m.Start.Add(time.Hour) }},
		{name: "period equity", change: func(m *SnapshotHistoryMetadata) { m.Periods[0].OpeningEquityRaw = "invalid" }},
		{name: "owner binding", owner: token},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := json.Marshal(base)
			if err != nil {
				t.Fatal(err)
			}
			var m SnapshotHistoryMetadata
			if err = json.Unmarshal(raw, &m); err != nil {
				t.Fatal(err)
			}
			if tc.change != nil {
				tc.change(&m)
			}
			owner := tc.owner
			if owner == "" {
				owner = wallet
			}
			tx, err := r.Pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer rollbackPortfolio(tx)
			err = r.CommitHistoryMetadataTx(ctx, tx, owner, summary.SnapshotID, m)
			if tc.valid {
				if err != nil {
					t.Fatal(err)
				}
				if err = r.CommitHistoryMetadataTx(ctx, tx, owner, summary.SnapshotID, m); err != nil {
					t.Fatal(err)
				}
				m.SourceEvent = "other"
				if err = r.CommitHistoryMetadataTx(ctx, tx, owner, summary.SnapshotID, m); !errors.Is(err, ErrConflict) {
					t.Fatal(err)
				}
			} else if err == nil {
				t.Fatal("unbound history metadata accepted")
			}
		})
	}
	tx, err := r.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = r.CommitHistoryMetadataTx(ctx, tx, wallet, summary.SnapshotID, base); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	share, err := r.ShareV2(ctx, wallet, wallet, summary.SnapshotID, false, time.Now())
	if err != nil || share.HistoricalReturn == nil || share.HistoricalReturn.Return == nil || share.HistoricalReturn.Return.Numerator != "1" || share.HistoricalReturn.Return.Denominator != "10" {
		t.Fatal(share, err)
	}
	result, err := r.HistoricalReturn(ctx, wallet, wallet, summary.SnapshotID, start, end)
	if err != nil || result.Return == nil || result.Return.Numerator != "1" || result.Return.Denominator != "10" {
		t.Fatal(result, err)
	}
	if _, _, err = r.HistorySeries(ctx, wallet, wallet, start, end, "1d", 1); err != nil {
		t.Fatal(err)
	}
}
