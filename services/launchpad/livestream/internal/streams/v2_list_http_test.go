package streams

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/go-chi/chi/v5"
)

func TestPostgresV2StreamListFiltersLiveAndPool(t *testing.T) {
	f := directoryHarness(t)
	ctx := context.Background()
	for _, live := range f.operations {
		_, generation := f.handlers.viewers.keys(live.ID)
		if err := f.handlers.viewers.client.Set(ctx, generation, time.Now().Add(-time.Second).UnixMilli(), time.Hour).Err(); err != nil {
			t.Fatal(err)
		}
	}
	wallet, pool := randomAddress(t), f.pools[0]
	operation, err := f.store.Create(ctx, CreateIntent{
		Wallet: wallet, Pool: pool, Title: "Failed stream in the matching pool", IdempotencyKey: "v2-list-failed",
	}, CreatorDecision{Wallet: wallet, Pool: pool, Allowed: true, ExpiresAt: time.Now().Add(time.Minute)}, true)
	if err != nil {
		t.Fatal(err)
	}
	failed, err := f.store.FailBeforeProvision(ctx, operation.ID, operation.Revision, ProvisionConfigurationUnavailable)
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, time.October, 26, 15, 4, 5, 123000000, time.UTC)
	f.handlers.clock = func() time.Time { return at }
	get := func(t *testing.T, target string) v2StreamListEnvelope {
		t.Helper()
		response := httptest.NewRecorder()
		f.router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, target, nil))
		if response.Code != http.StatusOK {
			t.Fatalf("GET %s: status %d: %s", target, response.Code, response.Body.String())
		}
		if response.Header().Get("Content-Type") != "application/json" {
			t.Fatal("missing JSON content type", response.Header())
		}
		if err := v2.ValidateJSON("StreamListResponse", response.Body.Bytes()); err != nil {
			t.Fatal(err, response.Body.String())
		}
		var page v2StreamListEnvelope
		if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil {
			t.Fatal(err)
		}
		if !frozenUUID.MatchString(page.RequestID) || page.Quality != "complete" || page.Watermark != nil || page.GeneratedAt != "2026-10-26T15:04:05.123Z" {
			t.Fatalf("unexpected frozen envelope: %+v", page)
		}
		return page
	}
	for _, tc := range []struct {
		name, query string
		want        []string
	}{
		{"all", "", []string{failed.ID, f.operations[2].ID, f.operations[1].ID, f.operations[0].ID}},
		{"live", "?status=live", []string{f.operations[2].ID, f.operations[1].ID, f.operations[0].ID}},
		{"pool", "?poolAddress=" + pool, []string{failed.ID, f.operations[0].ID}},
		{"live_pool", "?status=live&poolAddress=" + pool, []string{f.operations[0].ID}},
		{"mixed_case_pool", "?status=live&poolAddress=0x" + strings.ToUpper(pool[2:]), []string{f.operations[0].ID}},
		{"maximum_limit", "?status=live&limit=100", []string{f.operations[2].ID, f.operations[1].ID, f.operations[0].ID}},
		{"empty", "?status=live&poolAddress=" + randomAddress(t), []string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			page := get(t, "/v2/streams"+tc.query)
			got := make([]string, 0, len(page.Data.Items))
			for _, item := range page.Data.Items {
				got = append(got, item.StreamID)
				if item.StreamID == failed.ID {
					if item.Status != "failed" || item.PlaybackURL != nil {
						t.Fatal("failed stream projection", item)
					}
				} else if item.Status != "live" || item.PlaybackURL == nil || item.ViewerCount == nil || item.ViewerQuality != "verified" {
					t.Fatal("live stream projection", item)
				}
				if strings.Contains(tc.query, "poolAddress=") && item.PoolAddress != pool {
					t.Fatal("pool filter leaked a stream", item)
				}
			}
			if !reflect.DeepEqual(got, tc.want) || page.NextCursor != nil {
				t.Fatalf("items = %v, want %v; cursor = %v", got, tc.want, page.NextCursor)
			}
			if len(tc.want) == 0 && page.Revision != nil || len(tc.want) > 0 && page.Revision == nil {
				t.Fatal("revision presence does not match list", page.Revision)
			}
		})
	}
	var cursor *string
	for i := 2; i >= 0; i-- {
		target := "/v2/streams?status=live&limit=1"
		if cursor != nil {
			target += "&cursor=" + url.QueryEscape(*cursor)
		}
		page := get(t, target)
		if len(page.Data.Items) != 1 || page.Data.Items[0].StreamID != f.operations[i].ID {
			t.Fatalf("cursor page %d: %+v", i, page)
		}
		if (page.NextCursor != nil) != (i > 0) {
			t.Fatal("incorrect pagination termination", i, page.NextCursor)
		}
		if page.NextCursor != nil {
			for _, changedFilter := range []string{"", "status=live&poolAddress=" + pool + "&"} {
				response := httptest.NewRecorder()
				f.router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v2/streams?"+changedFilter+"cursor="+url.QueryEscape(*page.NextCursor), nil))
				if response.Code != http.StatusBadRequest {
					t.Fatal("cursor crossed its filter binding", response.Code, response.Body.String())
				}
				if err := v2.ValidateJSON("Error", response.Body.Bytes()); err != nil {
					t.Fatal(err)
				}
			}
		}
		cursor = page.NextCursor
	}
}

func TestV2StreamListRejectsInvalidQuery(t *testing.T) {
	for _, deps := range []struct {
		name string
		deps Deps
	}{
		{"no_lifecycle", Deps{}},
		{"no_store_connection", Deps{Lifecycle: &LifecycleStore{}}},
	} {
		t.Run(deps.name, func(t *testing.T) {
			router := chi.NewRouter()
			New(deps.deps).RegisterRoutes(router)
			for _, query := range []string{
				"status=ended", "limit=0", "limit=101", "unknown=value",
				"status=", "status=LIVE", "poolAddress=", "poolAddress=not-an-address",
				"poolAddress=0x" + strings.Repeat("g", 40),
				"limit=", "limit=-1", "limit=1.5", "limit=01", "limit=%2B1", "limit=9999999999999999999999999",
				"cursor=", "cursor=" + strings.Repeat("a", 257),
				"status=live&status=ended", "limit=1&limit=2",
				"poolAddress=" + randomAddress(t) + "&poolAddress=" + randomAddress(t),
				"cursor=one&cursor=two", "status=%zz", "status=live;unknown=value",
			} {
				t.Run(query, func(t *testing.T) {
					response := httptest.NewRecorder()
					router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v2/streams?"+query, nil))
					if response.Code != http.StatusBadRequest {
						t.Fatalf("status = %d, want 400: %s", response.Code, response.Body.String())
					}
					if err := v2.ValidateJSON("Error", response.Body.Bytes()); err != nil {
						t.Fatal(err, response.Body.String())
					}
					var body v2.Error
					if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
						t.Fatal(err)
					}
					if body.Code != "INVALID_REQUEST" || body.Retryable || body.RetryAfterMs != nil || !frozenUUID.MatchString(body.RequestId) {
						t.Fatalf("unexpected invalid request error: %+v", body)
					}
				})
			}
		})
	}
	router := chi.NewRouter()
	New(Deps{}).RegisterRoutes(router)
	for _, query := range []string{"", "?status=live", "?limit=1", "?limit=100", "?poolAddress=" + randomAddress(t)} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v2/streams"+query, nil))
		if response.Code != http.StatusServiceUnavailable {
			t.Fatal("valid query should reach the dependency check", query, response.Code, response.Body.String())
		}
		if err := v2.ValidateJSON("Error", response.Body.Bytes()); err != nil {
			t.Fatal(err)
		}
	}
}
