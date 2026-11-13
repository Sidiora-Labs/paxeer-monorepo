package devreads_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/devreads"
	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestDevelopmentReadsValidateEveryQueryAndRequireStorage(t *testing.T) {
	f := setup(t, true)
	bounds := "from=2026-01-01T00:00:00.000Z&to=2026-01-02T00:00:00.000Z"
	for _, route := range []struct {
		path    string
		queries []string
	}{
		{"/v3/me/portfolio", []string{"limit=bad", "limit=101", "cursor=invalid", "bad=1", "limit=1&limit=2", "bad=%zz", strings.Repeat("x", 4097)}},
		{"/v2/me/rewards", []string{"limit=bad", "limit=101", "cursor=invalid", "bad=1"}},
		{"/v2/me/referrals", []string{"bad=1", "campaign=a&campaign=b"}},
		{"/v2/me/portfolio/history", []string{"bad=1", "from=bad", bounds + "&interval=bad", bounds + "&interval=1h&limit=bad", bounds + "&interval=1h&limit=1"}},
		{"/v3/me/pnl/history", []string{"bad=1", "from=bad", bounds + "&interval=bad", bounds + "&limit=bad", bounds + "&interval=1h&limit=1"}},
	} {
		for _, query := range route.queries {
			response := get(t, f.router, route.path+"?"+query, traderBearer)
			if response.Code != 400 || !strings.Contains(response.Body.String(), "FIELD_INVALID") {
				t.Fatal(route.path, query, response.Code, response.Body.String())
			}
		}
	}
	for _, route := range []string{"/v2/me/portfolio/history", "/v3/me/pnl/history", "/v2/me/rewards", "/v2/me/referrals"} {
		response := get(t, f.router, route, "")
		if response.Code != 401 {
			t.Fatal(route, response.Code)
		}
	}
	cfg := f.pool.Config()
	trace := new(internaltest.QueryCancellation)
	cfg.ConnConfig.Tracer = trace
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	endpoint, _ := chainNode(t, 6)
	chain, err := devreads.DialChain(context.Background(), endpoint)
	if err != nil {
		t.Fatal(err)
	}
	defer chain.Close()
	r := chi.NewRouter()
	devreads.Register(r, devreads.Deps{Reader: devreads.Reader{Pool: pool, Chain: chain}, Store: store.New(pool), Identity: auth.HTTPIdentityAuthority{Endpoint: identityGateway(t), ServiceToken: serviceToken, AllowPlainHTTP: true}, CursorSecret: serviceToken, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	for _, path := range []string{"/v3/me/portfolio", "/v2/me/portfolio/history?" + bounds + "&interval=1h", "/v3/me/pnl/history?" + bounds + "&interval=1h", "/v2/me/rewards?limit=100", "/v2/me/referrals"} {
		t.Run(path, func(t *testing.T) {
			trace.Start(0)
			response := get(t, r, path, traderBearer)
			queries := trace.Stop()
			if response.Code != 200 || len(queries) == 0 {
				t.Fatal(response.Code, response.Body.String(), queries)
			}
			for i, query := range queries {
				if strings.EqualFold(query, "rollback") {
					continue
				}
				t.Run(fmt.Sprint(i+1), func(t *testing.T) {
					trace.Start(i + 1)
					response := get(t, r, path, traderBearer)
					observed := trace.Stop()
					if response.Code != 503 || len(observed) != i+1 || observed[i] != query {
						t.Fatal(response.Code, response.Body.String(), observed, query)
					}
				})
			}
		})
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	mounted := chi.NewRouter()
	configuration := config.Config{IdentityURL: identityGateway(t), IdentityServiceToken: serviceToken, WebhookHMACSecret: serviceToken}
	configuration.RPCURL = endpoint
	devreads.Mount(context.Background(), mounted, configuration, pool, store.New(pool), logger)
	if response := get(t, mounted, "/v2/me/rewards", traderBearer); response.Code != 200 {
		t.Fatal(response.Code, response.Body.String())
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err = chain.BlockAt(canceled, 10000); err == nil {
		t.Fatal("cancelled chain read accepted")
	}
	if block, found, err := chain.BlockAt(context.Background(), 10000); err != nil || found || block.Number != 0 {
		t.Fatal(block, found, err)
	}
	if _, err = devreads.DialChain(context.Background(), " "); err == nil {
		t.Fatal("empty endpoint accepted")
	}
}
