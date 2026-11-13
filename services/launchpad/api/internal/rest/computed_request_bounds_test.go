package rest

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
	"github.com/go-chi/chi/v5"
)

func TestComputedReadRequestsRejectInvalidFields(t *testing.T) {
	db := internaltest.NewPostgres(t)
	model, err := store.NewAccountingReadModel(db, store.AccountingReadConfig{ChartsSchema: "charts", StatsSchema: "stats", ChartsDeliverySchema: "charts", StatsDeliverySchema: "stats", ProjectionVersion: "v1", ProjectionGeneration: "g1", ChartsConsumerID: "charts", StatsConsumerID: "stats", SourceStream: "market", SourcePartitionSchema: "indexer"})
	if err != nil {
		t.Fatal(err)
	}
	st := store.New(db, nil)
	router := chi.NewRouter()
	router.Get("/leaderboards/{category}", leaderboards(model))
	router.Get("/unconfigured/{category}", leaderboards(nil))
	router.Get("/rewards/stats", holderRewardsStats(st))
	router.Get("/rewards/pools", holderRewardsPools(st))
	router.Get("/missing/stats", holderRewardsStats(nil))
	router.Get("/missing/pools", holderRewardsPools(nil))
	for _, tc := range []struct {
		path   string
		status int
		code   string
	}{
		{"/leaderboards/other?window=24h", 404, "NOT_FOUND"},
		{"/leaderboards/volume?window=24h&extra=1", 422, "FIELD_UNKNOWN"},
		{"/leaderboards/volume?window=24h&limit=1&limit=2", 422, "FIELD_UNKNOWN"},
		{"/leaderboards/volume?window=24h&limit=0", 422, "FIELD_INVALID"},
		{"/leaderboards/volume?window=other", 422, "FIELD_INVALID"},
		{"/leaderboards/volume?window=24h&cursor=", 400, "CURSOR_INVALID"},
		{"/leaderboards/volume?window=24h&cursor=invalid", 400, "CURSOR_INVALID"},
		{"/leaderboards/volume?window=24h", 503, "DEPENDENCY_UNAVAILABLE"},
		{"/unconfigured/volume?window=24h", 503, "DEPENDENCY_UNAVAILABLE"},
		{"/rewards/stats?extra=1", 400, "QUERY_INVALID"},
		{"/rewards/stats", 503, "DEPENDENCY_UNAVAILABLE"},
		{"/rewards/pools?extra=1", 422, "FIELD_UNKNOWN"},
		{"/rewards/pools?limit=0", 422, "FIELD_INVALID"},
		{"/rewards/pools?sort=other", 422, "FIELD_INVALID"},
		{"/rewards/pools?cursor=", 400, "CURSOR_INVALID"},
		{"/rewards/pools", 503, "DEPENDENCY_UNAVAILABLE"},
		{"/missing/stats", 503, "DEPENDENCY_UNAVAILABLE"},
		{"/missing/pools", 503, "DEPENDENCY_UNAVAILABLE"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, tc.path, nil))
			var body APIError
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || w.Code != tc.status || body.Code != tc.code {
				t.Fatalf("response: %d %s parse=%v", w.Code, w.Body.String(), err)
			}
		})
	}
}

func TestComputedCursorRejectsInvalidRepresentations(t *testing.T) {
	for _, raw := range []string{"", strings.Repeat("x", 257), "!", base64.RawURLEncoding.EncodeToString([]byte(`{"snapshotId":"s","offset":-1,"windowEnd":1}`)), base64.RawURLEncoding.EncodeToString([]byte(`{"snapshotId":"s","offset":0,"windowEnd":1,"extra":1}`))} {
		if _, err := decodeLeaderboardCursor(raw); err == nil {
			t.Fatalf("invalid leaderboard cursor accepted: %q", raw)
		}
	}
	original := leaderboardCursor{SnapshotID: "snapshot", Offset: 3, WindowEnd: 100}
	if got, err := decodeLeaderboardCursor(encodeLeaderboardCursor(original)); err != nil || got != original {
		t.Fatalf("cursor round trip: %+v %v", got, err)
	}
}
