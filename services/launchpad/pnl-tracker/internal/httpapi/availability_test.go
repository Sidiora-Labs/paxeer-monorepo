package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/card"
	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/httpapi"
	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/ogrender"
	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/pnlcache"
	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/referral"
	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/store"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestArchiveImageRendersAndCachesItsStoredSnapshot(t *testing.T) {
	ctx := context.Background()
	st := store.New(internaltest.NewPostgres(t))
	rdb := internaltest.NewRedis(t)
	id, code := uuid.NewString(), strings.ReplaceAll(uuid.NewString(), "-", "")[:16]
	snapshot := card.Snapshot{Version: 1, PoolAddress: "pool", TokenName: "Archived portfolio", Position: card.Position{TotalUsdlSpent: "1", TotalTokensBought: "1", TotalUsdlReceived: "0", TotalTokensSold: "0", AvgCostBasis: "1", CurrentHoldings: "1", RealizedPnlUsdl: "0"}}
	raw, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if err = st.InsertCard(ctx, store.CardRow{CardID: id, ShortCode: code, OwnerAddress: "owner", PoolAddress: "pool", TokenAddress: "token", Snapshot: raw, CreatedAt: 1}); err != nil {
		t.Fatal(err)
	}
	for _, renderer := range []*ogrender.Renderer{nil, ogrender.New("")} {
		router := chi.NewRouter()
		httpapi.RegisterCards(router, httpapi.CardDeps{Cards: card.New(st, "https://sidiora.fun", "https://og.example", nil), Renderer: renderer, Redis: rdb, ShareLabel: func(code string) string { return "sidiora.fun/pnl/" + code }})
		response := serve(t, router, "GET", "/pnl/cards/"+id+"/og.png", nil, nil)
		if renderer == nil {
			if response.Code != 500 || response.Body.String() != "{\"statusCode\":500,\"error\":\"Internal Server Error\",\"message\":\"An unexpected error occurred\"}\n" {
				t.Fatal(response.Code, response.Body.String())
			}
			continue
		}
		config, err := png.DecodeConfig(bytes.NewReader(response.Body.Bytes()))
		if err != nil || response.Code != 200 || config.Width != 1200 || config.Height != 630 {
			t.Fatal(response.Code, config, err)
		}
		cached, err := rdb.Get(ctx, pnlcache.KeyOG(id)).Bytes()
		if err != nil || !bytes.Equal(cached, response.Body.Bytes()) {
			t.Fatal(len(cached), err)
		}
		again := serve(t, router, "GET", "/pnl/cards/"+id+"/og.png", nil, nil)
		if again.Code != 200 || !bytes.Equal(again.Body.Bytes(), cached) {
			t.Fatal(again.Code)
		}
	}
}

func TestReadRoutesReportDatabaseCancellation(t *testing.T) {
	ctx := context.Background()
	pool := internaltest.NewPostgres(t)
	internaltest.EnsureIndexerSchema(t, pool)
	cfg := pool.Config()
	tracer := new(internaltest.QueryCancellation)
	cfg.ConnConfig.Tracer = tracer
	traced, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer traced.Close()
	st := store.New(traced)
	router := chi.NewRouter()
	rdb := internaltest.NewRedis(t)
	httpapi.RegisterReads(router, st, rdb)
	httpapi.RegisterCards(router, httpapi.CardDeps{Cards: card.New(st, "https://sidiora.fun", "https://og.example", nil)})
	httpapi.RegisterStatus(router, httpapi.StatusDeps{Store: st, StartedAt: time.Now()})
	httpapi.RegisterReferrals(router, referral.New(st, quietLogger(), 1, func() int64 { return 1 }))
	for _, tc := range []struct {
		path  string
		query int
	}{
		{"/users/" + uuid.NewString() + "/positions", 1}, {"/users/" + uuid.NewString() + "/portfolio", 1}, {"/users/user/positions/pool", 1}, {"/users/user/trades", 1},
		{"/pnl/cards/" + uuid.NewString(), 1}, {"/pnl/cards/" + uuid.NewString() + "/og.png", 1}, {"/referrals/owner/stats", 1}, {"/status", 1}, {"/status", 2}, {"/status", 3},
	} {
		t.Run(tc.path+string(rune('0'+tc.query)), func(t *testing.T) {
			tracer.Start(tc.query)
			out := serve(t, router, "GET", tc.path, nil, nil)
			queries := tracer.Stop()
			if out.Code != 500 || len(queries) != tc.query || !json.Valid(out.Body.Bytes()) {
				t.Fatal(out.Code, queries, out.Body.String())
			}
		})
	}
	request := httptest.NewRequest(http.MethodPost, "/pnl/events", strings.NewReader(`{"type":"click","shortCode":"known"}`))
	tracer.Start(1)
	out := httptest.NewRecorder()
	router.ServeHTTP(out, request)
	tracer.Stop()
	if out.Code != 500 || out.Body.String() != "{\"statusCode\":500,\"error\":\"Internal Server Error\",\"message\":\"An unexpected error occurred\"}\n" {
		t.Fatal(out.Code, out.Body.String())
	}
	out = serve(t, router, "POST", "/pnl/events", []byte(`{`), nil)
	if out.Code != 400 || !strings.Contains(out.Body.String(), "invalid JSON body") {
		t.Fatal(out.Code, out.Body.String())
	}
}

func TestReadRoutesExposeEmptyCollectionsAndClampNegativeLag(t *testing.T) {
	ctx := context.Background()
	st := store.New(internaltest.NewPostgres(t))
	internaltest.EnsureIndexerSchema(t, st.Pool())
	rdb := internaltest.NewRedis(t)
	router := chi.NewRouter()
	httpapi.RegisterReads(router, st, rdb)
	httpapi.RegisterStatus(router, httpapi.StatusDeps{Store: st, StartedAt: time.Now()})
	owner := uuid.NewString()
	out := serve(t, router, "GET", "/users/"+owner+"/positions", nil, nil)
	if out.Code != 200 || !strings.Contains(out.Body.String(), `"positions":[]`) {
		t.Fatal(out.Code, out.Body.String())
	}
	out = serve(t, router, "GET", "/users/"+owner+"/trades?limit=invalid&from=invalid&to=invalid", nil, nil)
	if out.Code != 200 || !strings.Contains(out.Body.String(), `"trades":[]`) {
		t.Fatal(out.Code, out.Body.String())
	}
	foldBuy(t, ctx, st, "ahead-buy-0", "user", "pool", "token", "100", oneToken, 10, 1)
	out = serve(t, router, "GET", "/status", nil, nil)
	var status struct{ ConsumerLag, ConsumerBlock int64 }
	if err := json.Unmarshal(out.Body.Bytes(), &status); err != nil || out.Code != 200 || status.ConsumerLag != 0 || status.ConsumerBlock != 10 {
		t.Fatal(status, out.Body.String(), err)
	}
}
