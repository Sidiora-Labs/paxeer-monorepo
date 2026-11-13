package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/store"
	"github.com/go-chi/chi/v5"
)

func TestSeriesRuntimeRoutesAndOperatorAdmission(t *testing.T) {
	ctx := context.Background()
	pool := internaltest.OpenPool(t, internaltest.NewDatabase(t))
	for _, schema := range []func(string) (string, error){store.ExactSchemaDDL, store.SeriesSchemaDDL, store.RebuildSchemaDDL} {
		ddl, err := schema("candles")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, ddl); err != nil {
			t.Fatal(err)
		}
	}
	exact, err := store.NewExact(pool, "candles")
	if err != nil {
		t.Fatal(err)
	}
	router := chi.NewRouter()
	archive := &store.ArchiveReader{Exact: exact}
	registerSeriesRuntime(router, exact, archive, "operator-secret")
	address := "0x" + strings.Repeat("a", 40)
	for _, tc := range []struct {
		path   string
		status int
	}{
		{"/v2/series/" + address + "/candles?" + strings.Repeat("x", 16385), http.StatusUnprocessableEntity},
		{"/v2/series/" + address + "/candles?interval=1m&from=%zz&to=120", http.StatusUnprocessableEntity},
		{"/v2/series/" + address + "/candles?interval=1m&interval=5m&from=0&to=120", http.StatusUnprocessableEntity},
		{"/v2/series/invalid/candles?interval=1m&from=0&to=120", http.StatusUnprocessableEntity},
		{"/v2/series/" + address + "/candles?interval=1m&from=no&to=120", http.StatusUnprocessableEntity},
		{"/v2/series/" + address + "/candles?interval=1m&from=0&to=120&dense=wrong", http.StatusUnprocessableEntity},
		{"/v2/series/" + address + "/candles?interval=1m&from=0&to=120&unexpected=1", http.StatusUnprocessableEntity},
		{"/v2/series/" + address + "/candles?interval=1m&from=0&to=120", http.StatusOK},
		{"/v2/series/" + address + "/candles?interval=1m&from=0&to=120&dense=true", http.StatusOK},
	} {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tc.path, nil))
		if rec.Code != tc.status {
			t.Fatalf("series request %q: %d, %s", tc.path, rec.Code, rec.Body.String())
		}
		if tc.status == http.StatusOK && (!strings.Contains(rec.Body.String(), `"noData":true`) || rec.Header().Get("Cache-Control") != "private, no-cache") {
			t.Fatalf("empty canonical series: %s", rec.Body.String())
		}
	}
	if _, err = pool.Exec(ctx, `INSERT INTO candles.chart_series_clock(chain_id,pool,revision,watermark_height,watermark_timestamp,has_gaps) VALUES('125',$1,1,1,120,false)`, address); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO candles.chart_pool_fences(chain_id,pool,epoch,frozen) VALUES('125',$1,1,false)`, address); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `ALTER TABLE candles.chart_exact RENAME TO chart_exact_held`); err != nil {
		t.Fatal(err)
	}
	unavailable := httptest.NewRecorder()
	router.ServeHTTP(unavailable, httptest.NewRequest(http.MethodGet, "/v2/series/"+address+"/candles?interval=1m&from=0&to=120", nil))
	if unavailable.Code != http.StatusServiceUnavailable || !strings.Contains(unavailable.Body.String(), "canonical series unavailable") {
		t.Fatalf("unavailable series: %d %s", unavailable.Code, unavailable.Body.String())
	}
	if _, err = pool.Exec(ctx, `ALTER TABLE candles.chart_exact_held RENAME TO chart_exact`); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		path, body, authorization string
		status                    int
	}{
		{"/internal/v2/charts/retention:apply", `{}`, "", http.StatusUnauthorized},
		{"/internal/v2/charts/retention:apply", `{}`, "Bearer operator-secret", http.StatusUnprocessableEntity},
		{"/internal/v2/charts/retention:apply", `{"pool":"` + address + `","interval":"1m","from":0,"to":60}`, "Bearer operator-secret", http.StatusConflict},
		{"/internal/v2/charts/archive:rebuild", `{}`, "", http.StatusUnauthorized},
		{"/internal/v2/charts/archive:rebuild", `{}`, "Bearer other-operator", http.StatusUnauthorized},
		{"/internal/v2/charts/archive:rebuild", `{}`, "Bearer operator-secret", http.StatusUnprocessableEntity},
		{"/internal/v2/charts/archive:rebuild", `{"runId":"run1","pool":"` + address + `","from":0,"to":60,"manifests":[]}`, "Bearer operator-secret", http.StatusServiceUnavailable},
	} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(tc.body))
		if tc.authorization != "" {
			req.Header.Set("Authorization", tc.authorization)
		}
		router.ServeHTTP(rec, req)
		if rec.Code != tc.status || rec.Body.Len() == 0 {
			t.Fatalf("operator request %q: %d, %s", tc.path, rec.Code, rec.Body.String())
		}
	}
	if _, err := pool.Exec(ctx, `INSERT INTO candles.chart_archive_checkpoints(pool,from_time,to_time,manifest_digest) VALUES($1,0,60,$2)`, address, strings.Repeat("a", 64)); err != nil {
		t.Fatal(err)
	}
	retention := httptest.NewRecorder()
	retentionRequest := httptest.NewRequest(http.MethodPost, "/internal/v2/charts/retention:apply", strings.NewReader(`{"pool":"`+address+`","interval":"1m","from":0,"to":60}`))
	retentionRequest.Header.Set("Authorization", "Bearer operator-secret")
	router.ServeHTTP(retention, retentionRequest)
	if retention.Code != http.StatusOK || !strings.Contains(retention.Body.String(), `"deletedRows":0`) {
		t.Fatalf("authorized empty retention range: %d %s", retention.Code, retention.Body.String())
	}
	withoutArchive := chi.NewRouter()
	registerSeriesRuntime(withoutArchive, exact, nil, "")
	for _, path := range []string{"/internal/v2/charts/retention:apply", "/internal/v2/charts/archive:rebuild"} {
		rec := httptest.NewRecorder()
		withoutArchive.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`)))
		if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "archive authority unavailable") {
			t.Fatalf("unavailable archive %s: %d %s", path, rec.Code, rec.Body.String())
		}
	}
}
