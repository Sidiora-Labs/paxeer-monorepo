package l17

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDerivedPortfolioRejectsUnboundAmountsAndPrices(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*Position)
	}{
		{"watermark", func(p *Position) { p.SourceWatermark.Height = "bad" }},
		{"revision", func(p *Position) { p.SourceWatermark.ProjectionRevision = "0" }},
		{"observation", func(p *Position) { p.SourceWatermark.ObservedAt = "bad" }},
		{"known", func(p *Position) { p.KnownQuantityRaw = "bad" }},
		{"unknown", func(p *Position) { p.UnknownQuantityRaw = "bad" }},
		{"balance", func(p *Position) { p.BalanceRaw = "bad" }},
		{"cost", func(p *Position) { p.KnownRemainingCostUsdlRaw = "bad" }},
		{"realized", func(p *Position) { p.KnownRealizedSubtotalUsdlRaw = "bad" }},
		{"quantity sum", func(p *Position) { p.BalanceRaw = "11" }},
		{"position", func(p *Position) { p.PositionID = "" }},
		{"wallet", func(p *Position) { p.Wallet = "bad" }},
		{"token", func(p *Position) { p.TokenAddress = "bad" }},
		{"realized quality", func(p *Position) { p.RealizedBasisQuality = "bad" }},
		{"unpriced mark", func(p *Position) { p.PricingQuality = "unpriced" }},
		{"pricing quality", func(p *Position) { p.PricingQuality = "bad" }},
		{"missing mark", func(p *Position) { p.MarkPrice = nil }},
		{"source", func(p *Position) { p.MarkPrice.Source.Kind = "bad" }},
		{"price height", func(p *Position) { p.MarkPrice.Source.Position.Height = "bad" }},
		{"price transaction", func(p *Position) { p.MarkPrice.Source.Position.TransactionIndex = "bad" }},
		{"price log", func(p *Position) { p.MarkPrice.Source.Position.LogIndex = "bad" }},
		{"price hash", func(p *Position) { p.MarkPrice.Source.Position.TransactionHash = "bad" }},
		{"price observation", func(p *Position) { p.MarkPrice.Source.ObservedAt = "bad" }},
		{"price numerator", func(p *Position) { p.MarkPrice.Numerator = "bad" }},
		{"price denominator", func(p *Position) { p.MarkPrice.Denominator = "0" }},
		{"token decimals", func(p *Position) { p.MarkPrice.BaseAsset.Decimals = nil }},
		{"quote", func(p *Position) { p.MarkPrice.QuoteAsset.Kind = "native" }},
		{"reduced price", func(p *Position) { p.MarkPrice.Numerator = "6"; p.MarkPrice.Denominator = "2" }},
		{"base identity", func(p *Position) { p.MarkPrice.BaseAsset.Address = ptr(wallet) }},
		{"valuation overflow", func(p *Position) { p.MarkPrice.Numerator = strings.Repeat("9", 300) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := fixture()
			tc.change(&p)
			if _, _, err := Derive(p); !errors.Is(err, ErrInvalid) {
				t.Fatal(err)
			}
		})
	}
	p := fixture()
	p.KnownQuantityRaw = "0"
	p.UnknownQuantityRaw = "10"
	p.KnownRemainingCostUsdlRaw = "0"
	actual, _, err := Derive(p)
	if err != nil || actual.BasisQuality != "unknown" || actual.RemainingCostUsdlRaw != nil {
		t.Fatal(actual, err)
	}
	p = fixture()
	p.KnownRealizedSubtotalUsdlRaw = strings.Repeat("9", 156)
	second := p
	second.PositionID = "125:second"
	if _, err = Aggregate(wallet, "", p.SourceWatermark, []Position{p, second}); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
}

func TestHistoryRecordsRequireCanonicalAssetsAndPositions(t *testing.T) {
	summary := Summary{Watermark: fixture().SourceWatermark}
	record := portfolioHistoryRecord(summary)
	if err := validateHistory(record); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		change func(*HistoryRecord)
	}{
		{"event", func(h *HistoryRecord) { h.EventID = "!" }},
		{"kind", func(h *HistoryRecord) { h.Kind = "other" }},
		{"amount", func(h *HistoryRecord) { h.AmountRaw = "-1" }},
		{"quote", func(h *HistoryRecord) { h.QuoteAmountUsdlRaw = ptr("bad") }},
		{"basis", func(h *HistoryRecord) { h.BasisQuality = "bad" }},
		{"asset decimals", func(h *HistoryRecord) { h.Asset.Decimals = nil }},
		{"asset address", func(h *HistoryRecord) { h.Asset.Address = ptr("bad") }},
		{"chain", func(h *HistoryRecord) { h.Position.ChainID = "1" }},
		{"height", func(h *HistoryRecord) { h.Position.Height = "bad" }},
		{"hash", func(h *HistoryRecord) { h.Position.BlockHash = "0x" + strings.Repeat("z", 64) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bad := record
			tc.change(&bad)
			if err := validateHistory(bad); !errors.Is(err, ErrInvalid) {
				t.Fatal(err)
			}
		})
	}
}

func TestPortfolioFailuresHaveStablePublicResponses(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
		code   string
		retry  bool
	}{
		{ErrInvalid, 400, "FIELD_INVALID", false}, {ErrRestart, 409, "REVISION_MISMATCH", false},
		{ErrConflict, 409, "IDEMPOTENCY_CONFLICT", false}, {ErrUnauthorized, 403, "SCOPE_DENIED", false},
		{ErrGone, 410, "RESOURCE_DELETED", false}, {ErrNotFound, 404, "NOT_FOUND", false},
		{ErrBusy, 429, "RATE_LIMITED", true}, {context.DeadlineExceeded, 503, "DEPENDENCY_UNAVAILABLE", true},
	} {
		t.Run(tc.code, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			writeFailure(recorder, tc.err)
			var body struct {
				Code      string `json:"code"`
				Retryable bool   `json:"retryable"`
				RequestID string `json:"requestId"`
			}
			if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil || recorder.Code != tc.status || body.Code != tc.code || body.Retryable != tc.retry || !validUUID(body.RequestID) {
				t.Fatal(recorder.Code, body, err)
			}
		})
	}
	authority, p := portfolioIdentity(t)
	request := httptest.NewRequest("GET", "/v2/me/portfolio", nil)
	request.Header.Set("Authorization", "Bearer portfolio-owner")
	service := Service{Authority: authority}
	if actual, err := service.Principal(context.Background(), request, p.Wallet); err != nil || actual.AccountID != p.AccountID {
		t.Fatal(actual, err)
	}
	if _, err := service.Principal(context.Background(), request, token); !errors.Is(err, ErrUnauthorized) {
		t.Fatal(err)
	}
	request.Header.Del("Authorization")
	if _, err := service.Principal(context.Background(), request, p.Wallet); err == nil {
		t.Fatal("missing credential accepted")
	}
	if _, err := (Service{}).Principal(context.Background(), request, p.Wallet); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
}

func TestTimeWeightedReturnRejectsInvalidPeriods(t *testing.T) {
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	period := ReturnPeriod{Start: start, End: start.Add(time.Hour), OpeningEquityRaw: "10", ClosingEquityBeforeFlowRaw: "11", ExternalFlowAtEndRaw: "0", Complete: true}
	for _, change := range []func(*ReturnPeriod){
		func(p *ReturnPeriod) { p.Start = time.Time{} }, func(p *ReturnPeriod) { p.OpeningEquityRaw = "bad" },
		func(p *ReturnPeriod) { p.ClosingEquityBeforeFlowRaw = "bad" }, func(p *ReturnPeriod) { p.ExternalFlowAtEndRaw = "bad" },
		func(p *ReturnPeriod) { p.ExternalFlowAtEndRaw = "-12" },
	} {
		bad := period
		change(&bad)
		if _, err := TimeWeightedReturn([]ReturnPeriod{bad}); !errors.Is(err, ErrInvalid) {
			t.Fatal(bad, err)
		}
	}
	if _, err := TimeWeightedReturn(nil); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	next := period
	next.Start = period.End
	next.End = next.Start.Add(time.Hour)
	next.OpeningEquityRaw = "12"
	if _, err := TimeWeightedReturn([]ReturnPeriod{period, next}); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
}
