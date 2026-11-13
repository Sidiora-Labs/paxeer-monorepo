package l16

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/production/l17"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
)

func TestMeasurementRequiresSuccessfulBoundedPortfolioTransport(t *testing.T) {
	ctx := context.Background()
	s, pool := recordedRebuildService(t, ctx)
	inventoryCommittedHead(t, ctx, pool)
	identity, principal := inventoryIdentityFixture(t, realMarketCreator)
	worker, err := NewLiveTransferWorker(s, l17.Repository{Pool: pool}, identity, 1)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("MEASURE_PORTFOLIO_ACCESS", "owner-access")
	input := InventoryMeasurementCase{Wallet: principal.Wallet, Token: realMarketToken, AccessTokenEnv: "MEASURE_PORTFOLIO_ACCESS", Operation: operationHTTPPortfolio}
	for _, tc := range []struct {
		name, body, timing string
		status             int
		want               string
	}{
		{"invalid JSON", "{", "", 200, "invalid or oversized"},
		{"oversized JSON", "\"" + strings.Repeat("a", 8*1024*1024) + "\"", "", 200, "invalid or oversized"},
		{"rejected", "{}", "", 403, "request rejected"},
		{"missing timing", "{}", "", 200, "timing missing"},
		{"contradictory timing", "{}", "portfolio_origin;dur=9999999", 200, "timing missing"},
		{"response binding", "{}", "portfolio_origin;dur=0", 200, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Server-Timing", tc.timing)
				w.WriteHeader(tc.status)
				if _, err := w.Write([]byte(tc.body)); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			_, status, facts, err := inventoryMeasurementOperation(ctx, worker, server.Client(), InventoryMeasurementPlan{APIOrigin: server.URL}, input)
			if err == nil || status != tc.status || facts.RoundTripNS == nil || tc.want != "" && !strings.Contains(err.Error(), tc.want) || tc.want == "" && !errors.Is(err, delivery.ErrIntegrity) {
				t.Fatal(status, facts, err)
			}
		})
	}
	dead := httptest.NewServer(http.NotFoundHandler())
	endpoint := dead.URL
	dead.Close()
	if _, _, _, err := inventoryMeasurementOperation(ctx, worker, &http.Client{Timeout: time.Second}, InventoryMeasurementPlan{APIOrigin: endpoint}, input); err == nil {
		t.Fatal("closed origin accepted")
	}
	if _, _, _, err := inventoryMeasurementOperation(ctx, worker, http.DefaultClient, InventoryMeasurementPlan{APIOrigin: "http://["}, input); err == nil {
		t.Fatal("invalid origin accepted")
	}
	input.Operation = "rebuild_owned"
	raw, _, _, err := inventoryMeasurementOperation(ctx, worker, http.DefaultClient, InventoryMeasurementPlan{APIOrigin: "http://localhost"}, input)
	var summary l17.Summary
	if err != nil || json.Unmarshal(raw, &summary) != nil || summary.Wallet != principal.Wallet || summary.PositionCount != 1 {
		t.Fatal(string(raw), err)
	}
}

func TestMeasuredPositionsRequireConsistentDerivedValues(t *testing.T) {
	wallet, token := testLeg(1).Wallet, testLeg(1).Token
	watermark := l17.Watermark{ChainID: "125", Height: "100", BlockHash: "0x" + strings.Repeat("a", 64), ProjectionRevision: "1", ObservedAt: "2026-10-10T00:00:00.000Z", Integrity: statusHealthy}
	position, err := ProjectPosition(wallet, token, Empty(), watermark, nil, statusUnpriced)
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*l17.Position){
		func(p *l17.Position) { p.Wallet = "0x" + strings.Repeat("e", 40) },
		func(p *l17.Position) { p.KnownQuantityRaw = "bad" },
		func(p *l17.Position) { p.BasisQuality = "unknown" },
	} {
		candidate := position
		change(&candidate)
		response := struct {
			Version int `json:"version"`
			l17.PortfolioResponse
		}{3, l17.PortfolioResponse{Algorithm: Algorithm, Page: l17.Page{Summary: l17.Summary{Wallet: wallet, Watermark: watermark}, Positions: []l17.Position{candidate}}}}
		raw, err := json.Marshal(response)
		if err != nil {
			t.Fatal(err)
		}
		if err = validateInventoryHTTPResponse(raw, InventoryMeasurementCase{Wallet: wallet, Token: token, Height: 100, BlockHash: watermark.BlockHash}); err == nil {
			t.Fatal("unbound derived position accepted", string(raw))
		}
	}
}
