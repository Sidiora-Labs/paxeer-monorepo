package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/internaltest"
	"github.com/google/uuid"
)

func TestReactionCountsReadEveryStoredVote(t *testing.T) {
	ctx := context.Background()
	redis := internaltest.NewRedis(t)
	pool := uuid.NewString()
	for _, vote := range []struct {
		name    string
		wallets []string
	}{
		{"bullish", []string{"a", "b"}}, {"hot", []string{"a"}}, {"diamond", []string{"a", "b", "c"}},
		{"bearish", []string{"a"}}, {"trash", []string{"b"}}, {"warning", []string{"c", "d"}},
	} {
		key := "reactions:" + pool + ":" + vote.name
		if err := redis.SAdd(ctx, key, vote.wallets).Err(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := redis.Del(context.Background(), key).Err(); err != nil {
				t.Error(err)
			}
		})
	}
	counts, err := countReactions(ctx, redis, pool)
	if err != nil || counts != (reactionCounts{Bullish: 2, Hot: 1, Diamond: 3, Bearish: 1, Trash: 1, Warning: 2}) || counts.total() != 10 {
		t.Fatalf("stored reaction counts: %+v %v", counts, err)
	}
	cancelled, stop := context.WithCancel(ctx)
	stop()
	if _, err := countReactions(cancelled, redis, pool); err == nil {
		t.Fatal("cancelled read succeeded")
	}
}

func TestExpandedSearchErrorProvidesRetryMetadata(t *testing.T) {
	for _, tc := range []struct {
		status      int
		code, retry string
		retryable   bool
	}{
		{429, "RATE_LIMITED", "60", true},
		{401, "AUTH_EXPIRED", "", false},
		{503, "DEPENDENCY_UNAVAILABLE", "1", true},
	} {
		response := httptest.NewRecorder()
		writeExpandedSearchError(response, httptest.NewRequest(http.MethodGet, "/v2/search/expanded", nil), tc.status, tc.code)
		var body struct {
			Code         string `json:"code"`
			Retryable    bool   `json:"retryable"`
			RetryAfterMS *int   `json:"retryAfterMs"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if response.Code != tc.status || body.Code != tc.code || body.Retryable != tc.retryable || response.Header().Get("Retry-After") != tc.retry {
			t.Fatalf("search error: %d %+v headers=%v", response.Code, body, response.Header())
		}
		if tc.status == 429 && (body.RetryAfterMS == nil || *body.RetryAfterMS != 60000) {
			t.Fatalf("rate limit delay: %+v", body)
		}
		if tc.status == 401 && body.RetryAfterMS != nil {
			t.Fatalf("authentication delay: %+v", body)
		}
	}
}
