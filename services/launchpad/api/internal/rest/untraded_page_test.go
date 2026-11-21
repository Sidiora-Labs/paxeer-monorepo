package rest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store/accountingtest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	interfaces "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func TestUntradedPoolPageRoutes(t *testing.T) {
	f := accountingtest.New(t)
	h := AccountingV2Handler{Store: f.Model, SigningKey: []byte(strings.Repeat("k", 32))}
	router := chi.NewRouter()
	RegisterAccountingV2(router, f.Model, h.SigningKey)
	for _, candles := range []bool{false, true} {
		name, path := "TradesResponse", "/trades?limit=5"
		request := store.AccountingPageRequest{Pool: accountingtest.Pool, Limit: 5}
		if candles {
			name, path = "CandlesResponse", "/candles?limit=5&interval=1m&from=2023-11-14T22:13:00.000Z&to=2023-11-14T22:14:00.000Z"
			request.Interval, request.From, request.To = "1m", 1699999980000, 1700000040000
		}
		t.Run(name, func(t *testing.T) {
			get := func(pool, suffix string) *httptest.ResponseRecorder {
				t.Helper()
				response := httptest.NewRecorder()
				router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v2/pools/"+pool+path+suffix, nil))
				return response
			}
			var identity string
			for i := 0; i < 2; i++ {
				response := get(accountingtest.Pool, "")
				if response.Code != 200 || response.Header().Get("Cache-Control") != "no-store" || interfaces.ValidateJSON(name, response.Body.Bytes()) != nil {
					t.Fatalf("empty page: %d %s %v", response.Code, response.Body.String(), response.Header())
				}
				var page map[string]any
				if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil {
					t.Fatal(err)
				}
				data := page["data"].(map[string]any)
				items, ok := data["items"].([]any)
				if !ok || len(items) != 0 || page["revision"] != nil || page["watermark"] != nil || page["nextCursor"] != nil || page["requestId"] != response.Header().Get("X-Request-ID") || response.Header().Get("X-Schema-Version") != "2" {
					t.Fatalf("empty page metadata: %s", response.Body.String())
				}
				if candles {
					id, ok := data["snapshotId"].(string)
					if !ok {
						t.Fatal(data)
					}
					if _, err := uuid.Parse(id); err != nil || data["seriesModelVersion"] != accountingtest.Version || i > 0 && id != identity {
						t.Fatalf("candle identity: %v", data)
					}
					identity = id
				}
			}
			response := get(accountingtest.UnknownPool, "")
			if response.Code != 503 || !strings.Contains(response.Body.String(), "DEPENDENCY_UNAVAILABLE") {
				t.Fatalf("unknown pool: %d %s", response.Code, response.Body.String())
			}
			request.SnapshotObservedAt = time.Now().Add(-time.Second).UTC().Format("2006-01-02T15:04:05.000Z")
			request.AfterOrder = &accounting.SourcePosition{ChainID: "125", Height: "110", TransactionIndex: "0", LogIndex: "0"}
			request.AfterEventID = strings.Repeat("c", 64)
			boundary := int64(1)
			if candles {
				boundary = 1699999980
			}
			token, err := h.sign(request, candles, uuid.NewString(), boundary, 1)
			if err != nil {
				t.Fatal(err)
			}
			for _, cursor := range []string{token, "invalid"} {
				response = get(accountingtest.Pool, "&cursor="+url.QueryEscape(cursor))
				if response.Code != 400 || !strings.Contains(response.Body.String(), "CURSOR_INVALID") {
					t.Fatalf("continuation: %d %s", response.Code, response.Body.String())
				}
			}
		})
	}
	fill, position := f.Publish(t)
	for _, path := range []string{"/trades", "/candles?interval=1m&from=2023-11-14T22:13:00.000Z&to=2023-11-14T22:14:00.000Z"} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v2/pools/"+accountingtest.Pool+path, nil))
		if response.Code != 503 || !strings.Contains(response.Body.String(), "DEPENDENCY_UNAVAILABLE") {
			t.Fatalf("uncommitted position: %d %s", response.Code, response.Body.String())
		}
	}
	f.Commit(t, fill, position)
	f.Exec(t, `UPDATE candles.delivery_partition_commits SET position='{}'`)
	for _, path := range []string{"/trades", "/candles?interval=1m&from=2023-11-14T22:13:00.000Z&to=2023-11-14T22:14:00.000Z"} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v2/pools/"+accountingtest.Pool+path, nil))
		if response.Code != 503 || !strings.Contains(response.Body.String(), "INTEGRITY_QUARANTINED") {
			t.Fatalf("invalid position: %d %s", response.Code, response.Body.String())
		}
	}
}
