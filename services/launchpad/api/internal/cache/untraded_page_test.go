package cache

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/rest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store/accountingtest"
	interfaces "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	sharedredis "github.com/Sidiora-Technologies/KindleLaunch/shared/redis"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func TestUntradedPoolPageCacheAndFirstCommit(t *testing.T) {
	f := accountingtest.New(t)
	internaltest.ApplyReadMigrations(t, f.DB, "90000")
	client, err := sharedredis.NewBoundedClient(internaltest.NewRedisURL(t), 200*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	config := runtimeAdmissionConfig(t, 0)
	config.ReadModel = f.Config
	config.Adapter.Redis = client
	config.Adapter.Namespace = "untraded_pages"
	config.Adapter.Budget = PostgresQueryBudget{Pool: f.DB, Schema: "api_limits", Namespace: "untraded_pages", Limit: 100, QueryCost: 1, Window: time.Hour}
	config.Adapter.Local.FetchTimeout = time.Second
	config.Adapter.RedisTimeout = 200 * time.Millisecond
	config.Adapter.LeaseTTL = 2 * time.Second
	config.Allocation.RequestLimit = 100
	a := runtimeAdmissionRuntime(t, config)
	router := chi.NewRouter()
	if err := a.bindings.Register(router, a.admission); err != nil {
		t.Fatal(err)
	}
	codec := rest.AccountingV2Handler{SigningKey: config.SigningKey}
	market := accountingtest.New(t)
	fill, position := market.Publish(t)
	market.Commit(t, fill, position)
	for _, candles := range []bool{false, true} {
		name, path, route := "TradesResponse", "/trades?limit=5", literalAccountingTrades
		request := store.AccountingPageRequest{Pool: accountingtest.Pool, Limit: 5}
		if candles {
			name, path, route = "CandlesResponse", "/candles?limit=5&interval=1m&from=2023-11-14T22:13:00.000Z&to=2023-11-14T22:14:00.000Z", literalAccountingCandles
			request.Interval, request.From, request.To = "1m", 1699999980000, 1700000040000
		}
		t.Run(name, func(t *testing.T) {
			get := func(pool, suffix string) *httptest.ResponseRecorder {
				t.Helper()
				response := httptest.NewRecorder()
				router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v2/pools/"+pool+path+suffix, nil))
				return response
			}
			for i := 0; i < 2; i++ {
				response := get(accountingtest.Pool, "")
				if response.Code != 200 || response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("ETag") != "" || interfaces.ValidateJSON(name, response.Body.Bytes()) != nil {
					t.Fatalf("empty cached route: %d %s %v", response.Code, response.Body.String(), response.Header())
				}
				var envelope map[string]any
				if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
					t.Fatal(err)
				}
				data := envelope["data"].(map[string]any)
				items, ok := data["items"].([]any)
				if !ok || len(items) != 0 || envelope["revision"] != nil || envelope["watermark"] != nil || envelope["nextCursor"] != nil || envelope["requestId"] != response.Header().Get("X-Request-ID") || response.Header().Get("X-Schema-Version") != "2" {
					t.Fatalf("empty envelope: %s", response.Body.String())
				}
				if candles {
					id, ok := data["snapshotId"].(string)
					if !ok {
						t.Fatal(data)
					}
					if _, err := uuid.Parse(id); err != nil || data["seriesModelVersion"] != accountingtest.Version {
						t.Fatalf("candle identity: %v", data)
					}
				}
				parameters := canonicalParameters(request, url.Values{}, candles)
				parameters.Set("sourceRevision", "unobserved")
				policy, err := a.adapter.policy(route)
				if err != nil {
					t.Fatal(err)
				}
				key, err := a.adapter.key(policy, VerifiedScope{}, parameters)
				if err != nil {
					t.Fatal(err)
				}
				if _, found := a.adapter.localGet(key); found {
					t.Fatal("untraded representation retained locally")
				}
				if _, found, err := sharedredis.GetPolicySnapshot(t.Context(), client, key); err != nil || found {
					t.Fatalf("untraded representation retained in Redis: %t %v", found, err)
				}
				if stats := a.adapter.Stats(); stats.Entries != 0 || stats.Bytes != 0 {
					t.Fatalf("empty page cache: %+v", stats)
				}
			}
			response := get(accountingtest.UnknownPool, "")
			if response.Code != 503 || !strings.Contains(response.Body.String(), "DEPENDENCY_UNAVAILABLE") {
				t.Fatalf("unknown pool: %d %s", response.Code, response.Body.String())
			}
			var page store.AccountingPage
			var err error
			if candles {
				page, err = market.Model.Candles(t.Context(), request)
				boundary := int64(1699999980)
				page.NextStart = &boundary
			} else {
				page, err = market.Model.Trades(t.Context(), request)
				boundary := int64(2)
				page.NextSequence = &boundary
				page.NextOrder = &fill.Position
				page.NextEventID = fill.SourceFillID
			}
			if err != nil {
				t.Fatal(err)
			}
			wire, err := codec.EncodeCachePage(request, candles, page, uuid.NewString())
			if err != nil {
				t.Fatal(err)
			}
			var continuation struct {
				NextCursor string `json:"nextCursor"`
			}
			if err = json.Unmarshal(wire, &continuation); err != nil || continuation.NextCursor == "" {
				t.Fatalf("continuation: %s %v", wire, err)
			}
			for _, token := range []string{continuation.NextCursor, "invalid"} {
				response = get(accountingtest.Pool, "&cursor="+url.QueryEscape(token))
				if response.Code != 400 || !strings.Contains(response.Body.String(), "CURSOR_INVALID") {
					t.Fatalf("continuation status: %d %s", response.Code, response.Body.String())
				}
			}
			source := AccountingProjectionSource{Store: f.Model, Generation: accountingtest.Generation, Candles: candles, Request: func(ProjectionRequest) (store.AccountingPageRequest, error) { return request, nil }, Encode: func(page store.AccountingPage) ([]byte, error) {
				return codec.EncodeCachePage(request, candles, page, uuid.NewString())
			}}
			_, err = source.Fetch(t.Context(), ProjectionRequest{Generation: accountingtest.Generation})
			var untraded *untradedAccountingPage
			if !errors.Is(err, store.ErrAccountingUntraded) || !errors.As(err, &untraded) || interfaces.ValidateJSON(name, untraded.projection.Body) != nil {
				t.Fatalf("uncacheable source: %v", err)
			}
		})
	}
	fill, position = f.Publish(t)
	paths := []string{"/trades?limit=5", "/candles?limit=5&interval=1m&from=2023-11-14T22:13:00.000Z&to=2023-11-14T22:14:00.000Z"}
	for _, path := range paths {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v2/pools/"+accountingtest.Pool+path, nil))
		if response.Code != 200 || interfaces.ValidateJSON("TradesResponse", response.Body.Bytes()) != nil && interfaces.ValidateJSON("CandlesResponse", response.Body.Bytes()) != nil {
			t.Fatalf("market awaiting consumer commits: %d %s", response.Code, response.Body.String())
		}
		var pending struct {
			Revision   *string         `json:"revision"`
			Watermark  json.RawMessage `json:"watermark"`
			NextCursor *string         `json:"nextCursor"`
			Data       struct {
				Items []json.RawMessage `json:"items"`
			} `json:"data"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &pending); err != nil || pending.Revision != nil || string(pending.Watermark) != "null" || pending.NextCursor != nil || pending.Data.Items == nil || len(pending.Data.Items) != 0 {
			t.Fatalf("pending market envelope: %s %v", response.Body.String(), err)
		}
	}
	f.Commit(t, fill, position)
	for i, path := range paths {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v2/pools/"+accountingtest.Pool+path, nil))
		name := "TradesResponse"
		if i == 1 {
			name = "CandlesResponse"
		}
		if response.Code != 200 || interfaces.ValidateJSON(name, response.Body.Bytes()) != nil || response.Header().Get("X-Projection-Revision") != "1" {
			t.Fatalf("first commit: %d %s", response.Code, response.Body.String())
		}
		var page struct {
			Revision string `json:"revision"`
			Data     struct {
				Items []json.RawMessage `json:"items"`
			} `json:"data"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil || page.Revision != "1" || len(page.Data.Items) != 1 {
			t.Fatalf("first market item: %s %v", response.Body.String(), err)
		}
	}
	f.Exec(t, `UPDATE candles.delivery_partition_commits SET position='{}'`)
	for _, path := range paths {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v2/pools/"+accountingtest.Pool+strings.Replace(path, "limit=5", "limit=6", 1), nil))
		if response.Code != 503 || !strings.Contains(response.Body.String(), "INTEGRITY_QUARANTINED") {
			t.Fatalf("invalid market: %d %s", response.Code, response.Body.String())
		}
	}

}
