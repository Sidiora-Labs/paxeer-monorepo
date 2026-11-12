package interactions

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestContractFollowReadsCommittedProfilesAndRevisions(t *testing.T) {
	f := connected(t)
	follower, target := f.wallets[0], f.wallets[1]
	router := chi.NewRouter()
	f.http.RegisterContractRoutes(router)
	request := func(method, path, actor string, headers map[string]string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, nil)
		if actor != "" {
			req = req.WithContext(f.ctx[actor])
		}
		for key, value := range headers {
			req.Header.Set(key, value)
		}
		rw := httptest.NewRecorder()
		router.ServeHTTP(rw, req)
		return rw
	}
	created := request(http.MethodPut, "/v2/me/following/"+target, follower, map[string]string{"Idempotency-Key": "contract-follow"})
	if created.Code != http.StatusOK {
		t.Fatalf("contract follow = %d %s", created.Code, created.Body.String())
	}
	stats := request(http.MethodGet, "/v2/profiles/"+target+"/follow-stats", follower, nil)
	if stats.Code != http.StatusOK || stats.Header().Get("ETag") == "" {
		t.Fatalf("follow stats = %d %s", stats.Code, stats.Body.String())
	}
	var totals struct {
		Data struct {
			Followers   int64 `json:"followers"`
			IsFollowing bool  `json:"isFollowing"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stats.Body.Bytes(), &totals); err != nil || totals.Data.Followers != 1 || !totals.Data.IsFollowing {
		t.Fatalf("follow stats = %+v: %v", totals, err)
	}
	cached := request(http.MethodGet, "/v2/profiles/"+target+"/follow-stats", follower, map[string]string{"If-None-Match": stats.Header().Get("ETag")})
	if cached.Code != http.StatusNotModified {
		t.Fatalf("unchanged graph = %d %s", cached.Code, cached.Body.String())
	}
	page := request(http.MethodGet, "/v2/profiles/"+target+"/followers?limit=10", "", nil)
	if page.Code != http.StatusOK {
		t.Fatalf("followers = %d %s", page.Code, page.Body.String())
	}
	var followers struct {
		Data struct {
			Items []struct {
				Wallet string `json:"wallet"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(page.Body.Bytes(), &followers); err != nil || len(followers.Data.Items) != 1 || followers.Data.Items[0].Wallet != follower {
		t.Fatalf("follower profiles = %+v: %v", followers, err)
	}
	removed := request(http.MethodDelete, "/v2/me/following/"+target, follower, map[string]string{"Idempotency-Key": "contract-unfollow"})
	if removed.Code != http.StatusNoContent {
		t.Fatalf("contract unfollow = %d %s", removed.Code, removed.Body.String())
	}
	page = request(http.MethodGet, "/v2/profiles/"+target+"/followers?limit=10", "", nil)
	if err := json.Unmarshal(page.Body.Bytes(), &followers); err != nil || page.Code != http.StatusOK || len(followers.Data.Items) != 0 {
		t.Fatalf("empty follower profiles = %d %+v: %v", page.Code, followers, err)
	}
}
