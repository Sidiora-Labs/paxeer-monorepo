package app

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	sharedhttp "github.com/Sidiora-Technologies/KindleLaunch/shared/http"
	sharedprocess "github.com/Sidiora-Technologies/KindleLaunch/shared/process"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/telemetry"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func TestApplicationOpenFillMetrics(t *testing.T) {
	f := publicationRecoveryNewFixture(t)
	f.close()
	metrics, err := telemetry.New("rankingd", "qualification", "zone-a")
	if err != nil {
		t.Fatal(err)
	}
	pool := publicationRecoveryOpenPool(t, f.dsn)
	cache := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1"})
	if err := cache.Close(); err != nil {
		t.Fatal(err)
	}
	app, err := newNativeApp(context.Background(), f.cfg, quietLogger(), pool, cache, sharedprocess.NewTracker(), metrics, map[string]sharedhttp.Check{}, nil)
	if err != nil {
		pool.Close()
		t.Fatal(err)
	}
	f.app = app
	assertMetrics := func(all, newOnly int) {
		t.Helper()
		response := httptest.NewRecorder()
		app.Router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, metricsPath, nil))
		body := response.Body.String()
		for _, want := range []string{
			"# TYPE ranking_open_fill_pools gauge\n",
			fmt.Sprintf("ranking_open_fill_pools{run=%q} %d\n", "all", all),
			fmt.Sprintf("ranking_open_fill_pools{run=%q} %d\n", "new", newOnly),
			"sli_read_eligible_requests_total{",
		} {
			if response.Code != http.StatusOK || !strings.Contains(body, want) {
				t.Fatalf("metrics response missing %q: %d %s", want, response.Code, body)
			}
		}
	}
	assertMetrics(0, 0)
	for i, newOnly := range []bool{false, true} {
		// Each pool has two fills; the gauge counts pools independently per run kind.
		if _, err := pool.Exec(context.Background(), `UPDATE stats.accounting_market_commits SET projection_version='other-version' WHERE pool=$1`, fmt.Sprintf("0x%040x", i+1)); err != nil {
			t.Fatal(err)
		}
		if _, err := app.native.coordinator.Refresh(context.Background(), uuid.NewString(), f.cutoff, newOnly); err != nil {
			t.Fatal(err)
		}
		assertMetrics(1, i*2)
	}
	response := httptest.NewRecorder()
	app.Router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, metricsPath, nil))
	if response.Code != http.StatusMethodNotAllowed || strings.Contains(response.Body.String(), "ranking_open_fill_pools") {
		t.Fatalf("metrics method response: %d %s", response.Code, response.Body.String())
	}
}
