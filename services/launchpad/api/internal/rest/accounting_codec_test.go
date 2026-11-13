package rest

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
	interfaces "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func TestAccountingCodecParsesAndEncodesPages(t *testing.T) {
	const pool = "0x1111111111111111111111111111111111111111"
	h := AccountingV2Handler{SigningKey: []byte(strings.Repeat("k", 32))}
	request := func(path string) *http.Request {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		route := chi.NewRouteContext()
		route.URLParams.Add("poolAddress", pool)
		return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, route))
	}
	for _, tc := range []struct {
		name, path, interval, side string
		candles                    bool
		valid                      bool
	}{
		{"trades", "/v2/pools/" + pool + "/trades?side=buy&limit=5", "", "buy", false, true},
		{"candles", "/v2/pools/" + pool + "/candles?interval=1m&from=2026-10-12T00%3A00%3A00.000Z&to=2026-10-12T01%3A00%3A00.000Z&limit=5", "1m", "", true, true},
		{"invalid side", "/v2/pools/" + pool + "/trades?side=other", "", "", false, false},
		{"duplicate limit", "/v2/pools/" + pool + "/trades?limit=5&limit=6", "", "", false, false},
		{"invalid cursor", "/v2/pools/" + pool + "/trades?cursor=invalid", "", "", false, false},
		{"reversed range", "/v2/pools/" + pool + "/candles?interval=1m&from=2026-10-12T01%3A00%3A00.000Z&to=2026-10-12T00%3A00%3A00.000Z", "", "", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, err := h.ParseCacheRequest(request(tc.path), tc.candles)
			if tc.valid {
				if err != nil || r.Pool != pool || r.Interval != tc.interval || r.Side != tc.side || r.Limit != 5 {
					t.Fatalf("parsed request: %+v, %v", r, err)
				}
			} else if err == nil {
				t.Fatalf("invalid request accepted: %+v", r)
			}
		})
	}
	watermark := json.RawMessage(`{"chainId":"125","height":"1","blockHash":"0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","projectionRevision":"1","observedAt":"2026-10-12T00:00:00.000Z","integrity":"healthy"}`)
	page := store.AccountingPage{Items: []json.RawMessage{}, SnapshotID: uuid.NewString(), SeriesModelVersion: "market-v1", Revision: "1", Watermark: watermark, GeneratedAt: "2026-10-12T00:00:00.000Z"}
	for _, tc := range []struct {
		candles bool
		name    string
	}{{false, "TradesResponse"}, {true, "CandlesResponse"}} {
		r := store.AccountingPageRequest{Pool: pool, Limit: 5, Interval: "1m", From: 1791763200000, To: 1791766800000}
		body, err := h.EncodeCachePage(r, tc.candles, page, uuid.NewString())
		if err != nil || interfaces.ValidateJSON(tc.name, body) != nil {
			t.Fatalf("%s page: %s, %v", tc.name, body, err)
		}
		var envelope struct {
			Revision   string  `json:"revision"`
			NextCursor *string `json:"nextCursor"`
		}
		if err := json.Unmarshal(body, &envelope); err != nil || envelope.Revision != "1" || envelope.NextCursor != nil {
			t.Fatalf("%s envelope: %+v, %v", tc.name, envelope, err)
		}
	}
	if _, err := h.ParseCacheRequest(request("/v2/pools/"+pool+"/trades?cursor="+url.QueryEscape(strings.Repeat("x", 300))), false); !errors.Is(err, ErrAccountingFields) {
		t.Fatalf("oversized cursor: %v", err)
	}
	for _, candles := range []bool{false, true} {
		page := page
		boundary := int64(1)
		if candles {
			boundary = 1791763200
			page.NextStart = &boundary
		} else {
			page.NextSequence = &boundary
			page.NextOrder = &accounting.SourcePosition{ChainID: "125", Height: "1", TransactionIndex: "0", LogIndex: "0"}
			page.NextEventID = strings.Repeat("a", 64)
		}
		r := store.AccountingPageRequest{Pool: pool, Limit: 5, Interval: "1m", From: 1791763200000, To: 1791766800000}
		body, err := h.EncodeCachePage(r, candles, page, uuid.NewString())
		if err != nil {
			t.Fatal(err)
		}
		var envelope struct{ NextCursor *string `json:"nextCursor"` }
		if err := json.Unmarshal(body, &envelope); err != nil || envelope.NextCursor == nil || *envelope.NextCursor == "" {
			t.Fatalf("page cursor: %s, %v", body, err)
		}
		decoded, err := h.cursor(r, candles, *envelope.NextCursor)
		if err != nil || decoded.ExpectedSnapshot != page.SnapshotID || decoded.CursorExpires == 0 || decoded.SnapshotObservedAt != "2026-10-12T00:00:00.000Z" {
			t.Fatalf("decoded page cursor: %+v, %v", decoded, err)
		}
		if candles {
			if decoded.AfterStart == nil || *decoded.AfterStart != boundary {
				t.Fatalf("candle boundary: %+v", decoded)
			}
		} else if decoded.AfterSequence == nil || *decoded.AfterSequence != boundary || decoded.SnapshotSequence == nil || *decoded.SnapshotSequence != 1 || decoded.AfterEventID != page.NextEventID {
			t.Fatalf("trade boundary: %+v", decoded)
		}
		changed := r
		changed.Limit = 4
		if _, err := h.cursor(changed, candles, *envelope.NextCursor); !errors.Is(err, store.ErrCursorInvalid) {
			t.Fatalf("cursor accepted for another page shape: %v", err)
		}
	}
}

func TestAccountingRoutesClassifyUnservedAndInvalidPages(t *testing.T) {
	const pool = "0x1111111111111111111111111111111111111111"
	h := AccountingV2Handler{SigningKey: []byte(strings.Repeat("k", 32))}
	for _, tc := range []struct {
		name, path, code string
		candles          bool
		status           int
	}{
		{"trades", "/v2/pools/" + pool + "/trades", "DEPENDENCY_UNAVAILABLE", false, 503},
		{"candles", "/v2/pools/" + pool + "/candles?interval=1m&from=2026-10-12T00%3A00%3A00.000Z&to=2026-10-12T01%3A00%3A00.000Z", "DEPENDENCY_UNAVAILABLE", true, 503},
		{"invalid trades", "/v2/pools/" + pool + "/trades?side=invalid", "FIELD_INVALID", false, 422},
		{"invalid candles", "/v2/pools/" + pool + "/candles?interval=1m", "FIELD_INVALID", true, 422},
		{"invalid trade cursor", "/v2/pools/" + pool + "/trades?cursor=invalid", "CURSOR_INVALID", false, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			router := chi.NewRouter()
			RegisterAccountingV2(router, nil, h.SigningKey)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, tc.path, nil))
			if response.Code != tc.status || !strings.Contains(response.Body.String(), `"code":"`+tc.code+`"`) {
				t.Fatalf("%s: status=%d body=%s", tc.path, response.Code, response.Body.String())
			}
		})
	}
}
