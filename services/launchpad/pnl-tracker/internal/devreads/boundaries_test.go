package devreads

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/production/l17"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

func TestDevelopmentErrorsAndQueryBounds(t *testing.T) {
	h := handlers{Deps: Deps{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}}
	for _, tc := range []struct {
		err    error
		status int
		code   string
	}{
		{errAuthRequired, 401, "AUTH_REQUIRED"}, {l17.ErrInvalid, 400, "FIELD_INVALID"}, {l17.ErrUnauthorized, 403, "SCOPE_DENIED"}, {l17.ErrRestart, 409, "REVISION_MISMATCH"}, {l17.ErrBusy, 429, "RATE_LIMITED"}, {pgx.ErrNoRows, 404, "NOT_FOUND"}, {context.DeadlineExceeded, 503, "DEPENDENCY_UNAVAILABLE"}, {errors.New("storage unavailable"), 503, "DEPENDENCY_UNAVAILABLE"},
	} {
		out := httptest.NewRecorder()
		h.fail(out, tc.err)
		if out.Code != tc.status || !strings.Contains(out.Body.String(), tc.code) || out.Header().Get("Cache-Control") != "private, no-store" {
			t.Fatal(out.Code, out.Body.String())
		}
	}
	for _, query := range []string{"x=1", "limit=1&limit=2", "limit=%zz", strings.Repeat("x", 4097)} {
		request := httptest.NewRequest("GET", "/", nil)
		request.URL.RawQuery = query
		if _, err := strictQuery(request, "limit"); !errors.Is(err, l17.ErrInvalid) {
			t.Fatal(query, err)
		}
	}
	if q, err := strictQuery(httptest.NewRequest("GET", "/?limit=1", nil), "limit"); err != nil || q.Get("limit") != "1" {
		t.Fatal(q, err)
	}
	out := httptest.NewRecorder()
	h.write(out, "ConnectedPortfolioV3Response", make(chan int))
	if out.Code != 503 {
		t.Fatal(out.Code, out.Body.String())
	}
	out = httptest.NewRecorder()
	h.write(out, "ConnectedPortfolioV3Response", map[string]any{"unbound": true})
	if out.Code != 503 {
		t.Fatal(out.Code, out.Body.String())
	}
	if _, err := DialChain(context.Background(), ":"); err == nil {
		t.Fatal("invalid RPC endpoint accepted")
	}
	var chain *Chain
	if _, err := chain.Head(context.Background()); err == nil {
		t.Fatal("missing chain anchor accepted")
	}
	chain.Close()
	for _, production := range []bool{true, false} {
		r := chi.NewRouter()
		cfg := config.Config{}
		if production {
			cfg.NodeEnv = "production"
		} else {
			cfg.RPCURL = ":"
		}
		Mount(context.Background(), r, cfg, nil, nil, h.Logger)
		out := httptest.NewRecorder()
		r.ServeHTTP(out, httptest.NewRequest("GET", "/v3/me/portfolio", nil))
		want := 401
		if production {
			want = 404
		}
		if out.Code != want {
			t.Fatal(out.Code, out.Body.String())
		}
	}
}

func TestDevelopmentReplayPreservesUnknownBasis(t *testing.T) {
	rp := newReplay()
	buy := replayTrade{id: "buy", pool: "pool", token: "invalid", isBuy: true, usdl: "100", amount: "10", ts: 1}
	gain, basis, known := rp.apply(buy)
	if !known || gain.Sign() != 0 || basis.Sign() != 0 || len(rp.states()) != 0 {
		t.Fatal(gain, basis, known)
	}
	sale := buy
	sale.isBuy = false
	sale.usdl = "bad"
	if gain, basis, known := rp.apply(sale); known || gain != nil || basis != nil {
		t.Fatal(gain, basis, known)
	}
	if _, _, known := rp.apply(buy); known || len(rp.positions) != 0 {
		t.Fatal("broken pool replayed")
	}
	if orZero("") != "0" || orZero("2") != "2" {
		t.Fatal("decimal default")
	}
	if _, err := (Reader{}).Snapshot(context.Background(), "invalid"); !errors.Is(err, l17.ErrInvalid) {
		t.Fatal(err)
	}
	if _, err := (Reader{}).Snapshot(context.Background(), "0x"+strings.Repeat("a", 40)); !errors.Is(err, l17.ErrUnavailable) {
		t.Fatal(err)
	}
}
