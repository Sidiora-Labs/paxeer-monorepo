package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	interfaces "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
)

func TestCanonicalCandleClientFollowsStableContinuation(t *testing.T) {
	revision := interfaces.PositiveUInt256("1")
	watermark := &interfaces.Watermark{ChainId: "125", Height: "1", BlockHash: "0x" + strings.Repeat("a", 64), Integrity: "healthy", ObservedAt: "2026-10-12T00:00:00.000Z", ProjectionRevision: "1"}
	base := interfaces.CandlesResponse{Data: interfaces.CandlesData{Items: []interfaces.Candle{}, SeriesModelVersion: "qualified-exact-v2", SnapshotId: "11111111-1111-4111-8111-111111111111"}, GeneratedAt: "2026-10-12T00:00:00.000Z", Quality: "complete", RequestId: "22222222-2222-4222-8222-222222222222", Revision: &revision, Watermark: watermark}
	cursor := "page-two"
	var calls atomic.Int64
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := base
		if r.URL.Query().Get("cursor") == "" {
			page.NextCursor = &cursor
		} else if r.URL.Query().Get("cursor") != cursor {
			http.Error(w, "unexpected cursor", http.StatusUnprocessableEntity)
			return
		}
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(page); err != nil {
			t.Errorf("canonical page: %v", err)
		}
	}))
	t.Cleanup(origin.Close)
	request := HistoryRequest{Symbol: udfPool, Timeframe: "1m", From: 0, To: 120}
	page, err := (CanonicalCandleClient{Endpoint: origin.URL}).Read(context.Background(), request)
	if err != nil || calls.Load() != 2 || !page.Complete || page.SnapshotID != base.Data.SnapshotId || len(page.Items) != 0 {
		t.Fatalf("stable canonical continuation: %+v, calls=%d, %v", page, calls.Load(), err)
	}
}

func TestCanonicalCandleClientBoundsContinuationAuthority(t *testing.T) {
	revision := interfaces.PositiveUInt256("1")
	watermark := &interfaces.Watermark{ChainId: "125", Height: "1", BlockHash: "0x" + strings.Repeat("a", 64), Integrity: "healthy", ObservedAt: "2026-10-12T00:00:00.000Z", ProjectionRevision: "1"}
	base := interfaces.CandlesResponse{Data: interfaces.CandlesData{Items: []interfaces.Candle{}, SeriesModelVersion: "qualified-exact-v2", SnapshotId: "11111111-1111-4111-8111-111111111111"}, GeneratedAt: "2026-10-12T00:00:00.000Z", Quality: "complete", RequestId: "22222222-2222-4222-8222-222222222222", Revision: &revision, Watermark: watermark}
	request := HistoryRequest{Symbol: udfPool, Timeframe: "1m", From: 0, To: 120}
	for _, tc := range []struct {
		name   string
		change func(*interfaces.CandlesResponse, int)
		want   string
	}{
		{"changed snapshot", func(page *interfaces.CandlesResponse, n int) {
			if n == 1 {
				page.Data.SnapshotId = "33333333-3333-4333-8333-333333333333"
			}
		}, "changed during pagination"},
		{"repeated continuation", func(page *interfaces.CandlesResponse, n int) {}, "invalid canonical continuation"},
		{"unbounded pagination", func(page *interfaces.CandlesResponse, n int) {
			cursor := string(rune('a' + n))
			page.NextCursor = &cursor
		}, "requires narrower range"},
		{"empty continuation", func(page *interfaces.CandlesResponse, n int) {
			cursor := ""
			page.NextCursor = &cursor
		}, "invalid canonical continuation"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int64
			origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				page := base
				n := int(calls.Add(1) - 1)
				if tc.name == "changed snapshot" || tc.name == "repeated continuation" {
					cursor := "next"
					page.NextCursor = &cursor
				}
				tc.change(&page, n)
				w.Header().Set("Content-Type", "application/json")
				if err := json.NewEncoder(w).Encode(page); err != nil {
					t.Error(err)
				}
			}))
			t.Cleanup(origin.Close)
			_, err := (CanonicalCandleClient{Endpoint: origin.URL}).Read(context.Background(), request)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("continuation authority: calls=%d err=%v", calls.Load(), err)
			}
		})
	}
	if _, err := (CanonicalCandleClient{Endpoint: "https://"}).Read(context.Background(), request); err == nil || !strings.Contains(err.Error(), "authority unavailable") {
		t.Fatalf("endpoint without canonical authority: %v", err)
	}
}
