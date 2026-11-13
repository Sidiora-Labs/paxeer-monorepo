package devsocial

import (
	"context"
	"net/http"
	"testing"

	"github.com/google/uuid"
)

func TestDevelopmentFollowRoutesCommitRemovalAndRejectUnavailablePolicy(t *testing.T) {
	h := newDirectRouteHarness(t)
	actor, target := directAddr(0xa11ce), directAddr(0xb0b)
	path := "/v2/social/follow/" + target
	if response := h.do(http.MethodPut, path, actor, nil, nil); response.status != http.StatusOK {
		t.Fatalf("follow = %d %s", response.status, response.body)
	}
	if response := h.do(http.MethodDelete, path, actor, nil, nil); response.status != http.StatusNoContent {
		t.Fatalf("unfollow = %d %s", response.status, response.body)
	}
	var count int
	if err := h.db.QueryRow(context.Background(), `SELECT count(*) FROM chat.follows WHERE follower=$1 AND followee=$2`, actor, target).Scan(&count); err != nil || count != 0 {
		t.Fatalf("removed follow edge = %d: %v", count, err)
	}
	if response := h.do(http.MethodPut, path, "", nil, nil); response.status != http.StatusUnauthorized {
		t.Fatalf("unverified follow = %d %s", response.status, response.body)
	}
	if response := h.do(http.MethodPut, "/v2/social/follow/"+actor, actor, nil, nil); response.status != http.StatusBadRequest {
		t.Fatalf("self follow = %d %s", response.status, response.body)
	}
	for _, path := range []string{
		"/v2/social/users/invalid/follow-stats",
		"/v2/social/users/invalid/followers",
		"/v2/social/users/" + actor + "/followers?limit=invalid",
		"/v2/social/users/" + actor + "/following?limit=0",
	} {
		if response := h.do(http.MethodGet, path, actor, nil, nil); response.status != http.StatusBadRequest {
			t.Fatalf("invalid follow read %s = %d %s", path, response.status, response.body)
		}
	}
	if _, err := h.db.Exec(context.Background(), `INSERT INTO chat.safety_bans(id,wallet,scope,resource,actor) VALUES($1,$2,'global','',$3)`, uuid.NewString(), actor, target); err != nil {
		t.Fatal(err)
	}
	if response := h.do(http.MethodPut, path, actor, nil, nil); response.status != http.StatusForbidden {
		t.Fatalf("banned follow = %d %s", response.status, response.body)
	}
	h.runtime.pool.Close()
	if response := h.do(http.MethodPut, path, actor, nil, nil); response.status != http.StatusServiceUnavailable {
		t.Fatalf("unavailable follow policy = %d %s", response.status, response.body)
	}
	if response := h.do(http.MethodDelete, path, actor, nil, nil); response.status != http.StatusServiceUnavailable {
		t.Fatalf("unavailable unfollow = %d %s", response.status, response.body)
	}
	for _, path := range []string{
		"/v2/social/users/" + actor + "/follow-stats",
		"/v2/social/users/" + actor + "/followers",
		"/v2/social/users/" + actor + "/following",
	} {
		if response := h.do(http.MethodGet, path, actor, nil, nil); response.status != http.StatusServiceUnavailable {
			t.Fatalf("unavailable follow read %s = %d %s", path, response.status, response.body)
		}
	}
}
