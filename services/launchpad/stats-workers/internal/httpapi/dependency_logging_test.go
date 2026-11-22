package httpapi

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/analytics"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/holdings"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/projectionstats"
	canonicalsearch "github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/search"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func TestReadDependencyErrorsNameTheirCause(t *testing.T) {
	pool := internaltest.NewPostgres(t)
	pool.Close()
	redis := internaltest.NewRedis(t)
	if err := redis.Close(); err != nil {
		t.Fatal(err)
	}
	st := store.New(pool)
	router := chi.NewRouter()
	RegisterPlatform(router, st, redis)
	RegisterPoolStats(router, st, redis)
	RegisterPoolTransactions(router, st)
	RegisterPoolAnalytics(router, st)
	RegisterPoolHolders(router, st)
	RegisterCrossTokenSwaps(router, st)
	RegisterSearch(router, st, redis)
	RegisterPressure(router, st, redis)
	service, err := canonicalsearch.NewService(canonicalsearch.Store{Pool: pool}, nil, 1)
	if err != nil {
		t.Fatal(err)
	}
	RegisterIntegratedSearch(router, service, nil)
	holders, err := holdings.NewRepository(pool, "stats_transfer_holdings")
	if err != nil {
		t.Fatal(err)
	}
	RegisterTransferHolders(router, st, holders)
	RegisterDurableAnalytics(router, analytics.ReactionAuthority{Pool: pool, Schema: "stats"})
	repository, err := projectionstats.NewRepository(pool, "stats")
	if err != nil {
		t.Fatal(err)
	}
	reads, err := projectionstats.NewReadService(repository, projectionstats.ReadConfig{SourceSchema: "indexer", SourceStream: "chain125-canonical", ProjectionVersion: "accounting-market-v1", CursorKey: []byte(strings.Repeat("k", 32)), CursorTTL: 10 * time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	(projectionstats.PublicHandlers{Service: reads}).Register(router)
	var output bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&output, nil)))
	defer slog.SetDefault(previous)
	address := "0x" + strings.Repeat("a", 40)
	const masked500 = `{"statusCode":500,"error":"Internal Server Error","message":"An unexpected error occurred"}` + "\n"
	const masked503 = `{"statusCode":503,"error":"Internal Server Error","message":"An unexpected error occurred"}` + "\n"
	for _, route := range []struct {
		path   string
		status int
		code   string
	}{
		{"/stats/platform", 500, masked500},
		{"/stats/" + address, 500, masked500},
		{"/stats/batch?pools=" + address, 500, masked500},
		{"/stats/" + address + "/transactions", 500, masked500},
		{"/stats/" + address + "/holders", 500, masked500},
		{"/stats/" + address + "/creator-activity", 500, masked500},
		{"/stats/" + address + "/risk", 500, masked500},
		{"/stats/" + address + "/whales", 503, masked503},
		{"/stats/" + address + "/holders/distribution", 503, masked503},
		{"/v2/stats/" + address + "/risk", 503, masked503},
		{"/v2/stats/" + address + "/pressure", 503, masked503},
		{"/v2/stats/" + address + "/analytics/revisions", 503, masked503},
		{"/stats/" + address + "/pressure", 500, masked500},
		{"/stats/cross-token-swaps/" + address, 500, masked500},
		{"/search?q=token", 500, masked500},
		{"/v2/search?q=token&type=pool", 503, "DEPENDENCY_UNAVAILABLE"},
		{"/v2/search/expanded?q=token&type=pool", 503, "DEPENDENCY_UNAVAILABLE"},
		{"/v1/pools/" + address + "/holders", 503, "DEPENDENCY_UNAVAILABLE"},
		{"/v2/stats/platform", 503, "unavailable"},
	} {
		t.Run(route.path, func(t *testing.T) {
			output.Reset()
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest("GET", route.path, nil))
			if response.Code != route.status || !strings.Contains(response.Body.String(), route.code) {
				t.Fatalf("response: %d %s", response.Code, response.Body.String())
			}
			if response.Header().Get("Content-Type") != "application/json" {
				t.Fatalf("response content type: %s", response.Header().Get("Content-Type"))
			}
			if (route.code == masked500 || route.code == masked503) && response.Body.String() != route.code {
				t.Fatalf("masked error envelope: %s", response.Body.String())
			}
			if !strings.Contains(output.String(), "closed pool") {
				t.Fatalf("dependency cause absent: %s", output.String())
			}
			if strings.Contains(response.Body.String(), "closed pool") {
				t.Fatal("private dependency cause in public response")
			}
		})
	}
	admission := &ReactionAdmission{Redis: redis, Namespace: "stats", WalletPerMinute: 6, IPPerMinute: 100}
	err = admission.Admit(context.Background(), address, "127.0.0.1")
	if !errors.Is(err, analytics.ErrDependencyUnavailable) || !strings.Contains(err.Error(), "closed") {
		t.Fatalf("reaction dependency: %v", err)
	}
	output.Reset()
	response := httptest.NewRecorder()
	durableReactionError(response, err)
	if response.Code != 503 || !strings.Contains(output.String(), "closed") {
		t.Fatalf("reaction response: %d %s", response.Code, output.String())
	}
	_, err = service.Personalize(context.Background(), canonicalsearch.IntegratedPage{}, auth.IdentityPrincipal{AccountID: uuid.NewString(), SessionID: uuid.NewString(), Wallet: address, AccessExpiresAt: time.Now().Add(time.Minute), ProofAt: time.Now()})
	if !errors.Is(err, canonicalsearch.ErrUnavailable) || !strings.Contains(err.Error(), "closed pool") {
		t.Fatalf("personalized search storage cause: %v", err)
	}
	for _, kind := range []string{"pool", "profile"} {
		_, err := (canonicalsearch.Store{Pool: pool}).SearchIntegrated(context.Background(), canonicalsearch.IntegratedRequest{Request: canonicalsearch.Request{Query: "token", Kind: kind, Limit: 10}, Sort: "address"})
		if !errors.Is(err, canonicalsearch.ErrUnavailable) || !strings.Contains(err.Error(), "closed pool") {
			t.Fatalf("%s storage cause: %v", kind, err)
		}
	}
}
