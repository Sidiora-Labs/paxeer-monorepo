package l17

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
)

func TestPortfolioServiceRejectsInvalidPagingAndExpiredSnapshots(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	repository := phase2Repository(t)
	summary := phase2Commit(t, repository, "response-binding", "1", 2)
	key := []byte(strings.Repeat("k", 32))
	service := Service{Repository: repository, CursorKey: key}
	for _, tc := range []struct {
		wallet, cursor string
		limit          int
		want           error
	}{
		{"bad", "", 10, ErrInvalid}, {wallet, "", 101, ErrInvalid}, {wallet, "bad", 10, ErrInvalid},
	} {
		if _, err := service.Portfolio(ctx, tc.wallet, tc.cursor, tc.limit, now); !errors.Is(err, tc.want) {
			t.Fatal(tc, err)
		}
	}
	cursor := Cursor{Wallet: wallet, Snapshot: summary.SnapshotID, Revision: "1", Filter: filterAll, Sort: sortSnapshotOrdinalV1, After: -1, Limit: 1, Expires: now.Add(time.Minute).Unix()}
	signed, err := SignPortfolioCursor(key, cursor)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.Portfolio(ctx, wallet, signed, 2, now); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	cursor.Revision = "2"
	signed, err = SignPortfolioCursor(key, cursor)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = service.Portfolio(ctx, wallet, signed, 1, now); !errors.Is(err, ErrRestart) {
		t.Fatal(err)
	}
	short := service
	short.CursorKey = []byte("short")
	if _, err = short.Portfolio(ctx, wallet, "", 1, now); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if _, _, _, err = (Repository{}).Latest(ctx, wallet, now); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
	if _, _, _, err = repository.Latest(ctx, "bad", now); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if _, err = repository.Pool.Exec(ctx, `UPDATE portfolio_snapshots SET expires_at=$1`, now.Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, _, expires, err := repository.Latest(ctx, wallet, now); !errors.Is(err, ErrRestart) || !expires.Before(now) {
		t.Fatal(expires, err)
	}
	if _, err = service.Portfolio(ctx, wallet, "", 1, now); !errors.Is(err, ErrRestart) {
		t.Fatal(err)
	}
	admission, err := NewAdmission(1, 1)
	if err != nil {
		t.Fatal(err)
	}
	release, err := admission.Acquire(ctx, wallet)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	repository.ReadAdmission = admission
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, _, _, err = repository.Latest(canceled, wallet, now); err == nil {
		t.Fatal("cancelled admission accepted")
	}
}

func TestPortfolioHTTPRejectsCorruptStoredViews(t *testing.T) {
	ctx := context.Background()
	repository := phase2Repository(t)
	summary := phase2Commit(t, repository, "http-view-binding", "1", 1)
	authority, _ := portfolioIdentity(t)
	service := Service{Repository: repository, Authority: authority, CursorKey: []byte(strings.Repeat("k", 32))}
	router := chi.NewRouter()
	service.RegisterHTTP(router)
	router.Get("/v3/me/portfolio", service.PortfolioV3HTTP)
	router.Get("/v3/wallets/{wallet}/portfolio", service.PublicPortfolioV3HTTP)
	for _, route := range []string{"/v3/wallets/bad/portfolio", "/v3/me/portfolio?invalid=%zz", "/v3/me/portfolio?limit=1&limit=2"} {
		portfolioRequest(t, router, "GET", route, nil, 400)
	}
	out := httptest.NewRecorder()
	service.PortfolioV3HTTP(out, httptest.NewRequest("GET", "/v3/me/portfolio", nil))
	if out.Code != 403 {
		t.Fatal(out.Code, out.Body.String())
	}
	original, err := json.Marshal(summary)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		change func(map[string]any)
		raw    string
		want   int
	}{
		{name: "summary type", raw: "[]", want: 503},
		{name: "summary schema", change: func(m map[string]any) { m["wallet"] = "bad" }, want: 503},
		{name: "revision binding", change: func(m map[string]any) { m["watermark"].(map[string]any)["projectionRevision"] = "2" }, want: 409},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var value map[string]any
			if err := json.Unmarshal(original, &value); err != nil {
				t.Fatal(err)
			}
			if tc.change != nil {
				tc.change(value)
			}
			raw, err := json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
			if tc.raw != "" {
				raw = []byte(tc.raw)
			}
			if _, err = repository.Pool.Exec(ctx, `UPDATE portfolio_snapshots SET summary=$1 WHERE id=$2`, raw, summary.SnapshotID); err != nil {
				t.Fatal(err)
			}
			defer func() {
				if _, err := repository.Pool.Exec(ctx, `UPDATE portfolio_snapshots SET summary=$1 WHERE id=$2`, original, summary.SnapshotID); err != nil {
					t.Error(err)
				}
			}()
			portfolioRequest(t, router, "GET", "/v2/me/portfolio", nil, tc.want)
		})
	}
	var raw []byte
	if err = repository.Pool.QueryRow(ctx, `SELECT payload FROM portfolio_snapshot_rows WHERE snapshot_id=$1`, summary.SnapshotID).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if _, err = repository.Pool.Exec(ctx, `UPDATE portfolio_snapshot_rows SET payload='[]'::jsonb WHERE snapshot_id=$1`, summary.SnapshotID); err != nil {
		t.Fatal(err)
	}
	portfolioRequest(t, router, "GET", "/v2/me/portfolio", nil, 503)
	portfolioRequest(t, router, "GET", "/v3/me/portfolio", nil, 503)
	if _, err = repository.Pool.Exec(ctx, `UPDATE portfolio_snapshot_rows SET payload=$1 WHERE snapshot_id=$2`, raw, summary.SnapshotID); err != nil {
		t.Fatal(err)
	}
	if _, err = repository.Pool.Exec(ctx, `UPDATE portfolio_snapshot_rows SET payload=jsonb_set(payload,'{pricingQuality}','"unpriced"')- 'markPrice' WHERE snapshot_id=$1`, summary.SnapshotID); err != nil {
		t.Fatal(err)
	}
	response, err := service.Portfolio(ctx, wallet, "", 1, time.Now())
	if err != nil || len(response.Errors) != 1 || !strings.Contains(strings.Join(response.Errors[response.Page.Positions[0].PositionID], ","), "current_valuation_unavailable") {
		t.Fatal(response, err)
	}
	service.Display = &DisplayProvider{}
	response, err = service.Portfolio(ctx, wallet, "", 1, time.Now())
	if err != nil || response.Display == nil || response.Display.UsdlRaw == "" {
		t.Fatal(response, err)
	}
}
