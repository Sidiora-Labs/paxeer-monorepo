package l16

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/production/l17"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
)

func TestMeasuredPortfolioResponse(t *testing.T) {
	leg := testLeg(1)
	wallet, token := leg.Wallet, leg.Token
	blockHash := "0x" + strings.Repeat("a", 64)
	watermark := l17.Watermark{ChainID: "125", Height: "100", BlockHash: blockHash, ProjectionRevision: "1", ObservedAt: "2026-10-10T00:00:00.000Z", Integrity: statusHealthy}
	position, err := ProjectPosition(wallet, token, Empty(), watermark, nil, statusUnpriced)
	if err != nil {
		t.Fatal(err)
	}
	response := struct {
		Version int `json:"version"`
		l17.PortfolioResponse
	}{Version: 3, PortfolioResponse: l17.PortfolioResponse{Algorithm: Algorithm, Page: l17.Page{Summary: l17.Summary{Wallet: wallet, Watermark: watermark}, Positions: []l17.Position{position}}}}
	caseInput := InventoryMeasurementCase{Wallet: wallet, Token: token, Height: 100, BlockHash: blockHash}
	check := func(want error) {
		t.Helper()
		raw, err := json.Marshal(response)
		if err != nil {
			t.Fatal(err)
		}
		observed := validateInventoryHTTPResponse(raw, caseInput)
		if want == nil && observed != nil || want != nil && !errors.Is(observed, want) {
			t.Fatalf("measured portfolio response = %v, want %v", observed, want)
		}
	}
	check(nil)
	response.Page.Positions = append(response.Page.Positions, position)
	check(delivery.ErrIntegrity)
	response.Page.Positions = response.Page.Positions[:1]
	caseInput.ExpectedState = &Inventory{Known: "1", Unknown: "0", Cost: "0", Realized: "0"}
	check(delivery.ErrIntegrity)
	caseInput.ExpectedState = nil
	caseInput.PricingQuality = "priced"
	check(delivery.ErrIntegrity)
	caseInput.PricingQuality = ""
	response.Page.Positions[0].TokenAddress = "0x" + strings.Repeat("f", 40)
	check(delivery.ErrGap)
	response.Page.Positions[0].TokenAddress = token
	response.Page.Summary.Wallet = "0x" + strings.Repeat("f", 40)
	check(delivery.ErrIntegrity)
	response.Page.Summary.Wallet = wallet
	response.Version = 2
	check(delivery.ErrIntegrity)
}

func TestMeasurementErrorCategories(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{nil, ""},
		{ErrParity, expectedParity},
		{ErrBaselineUnavailable, "baseline_unavailable"},
		{ErrAttributionUnavailable, "attribution_unavailable"},
		{ErrCostAllocationUnavailable, "cost_allocation_unavailable"},
		{auth.ErrScopeDenied, reasonAuthorization},
		{context.DeadlineExceeded, "deadline"},
		{context.Canceled, "cancelled"},
		{delivery.ErrIntegrity, "integrity"},
		{errors.New("provider response unavailable"), "dependency_or_operation_failure"},
	}
	for _, tc := range cases {
		err := tc.err
		if err != nil {
			err = fmt.Errorf("measurement: %w", err)
		}
		if got := measurementError(err); got != tc.want {
			t.Fatalf("error %v = %q, want %q", tc.err, got, tc.want)
		}
	}
}
