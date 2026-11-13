package devsocial

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestDirectFollowPagesReflectCommittedEdges(t *testing.T) {
	h := newDirectRouteHarness(t)
	a, b, c := directAddr(0xa11ce), directAddr(0xb0b), directAddr(0xc0ffee)
	for _, follower := range []string{a, c} {
		response := h.do(http.MethodPut, "/v2/social/follow/"+b, follower, nil, nil)
		if response.status != http.StatusOK {
			t.Fatalf("follow = %d %s", response.status, response.body)
		}
	}
	first := h.do(http.MethodGet, "/v2/social/users/"+b+"/followers?limit=1", "", nil, nil)
	if first.status != http.StatusOK {
		t.Fatalf("first page = %d %s", first.status, first.body)
	}
	var page struct {
		Data struct {
			Items []struct {
				Wallet string `json:"wallet"`
			} `json:"items"`
		} `json:"data"`
		NextCursor *string `json:"nextCursor"`
	}
	if err := json.Unmarshal(first.body, &page); err != nil || len(page.Data.Items) != 1 || page.NextCursor == nil {
		t.Fatalf("first follow page = %+v: %v", page, err)
	}
	second := h.do(http.MethodGet, "/v2/social/users/"+b+"/followers?limit=1&cursor="+*page.NextCursor, "", nil, nil)
	if err := json.Unmarshal(second.body, &page); err != nil || second.status != http.StatusOK || len(page.Data.Items) != 1 || page.NextCursor != nil {
		t.Fatalf("second follow page = %d %+v: %v", second.status, page, err)
	}
	following := h.do(http.MethodGet, "/v2/social/users/"+a+"/following?limit=10", "", nil, nil)
	if err := json.Unmarshal(following.body, &page); err != nil || following.status != http.StatusOK || len(page.Data.Items) != 1 || page.Data.Items[0].Wallet != b {
		t.Fatalf("following page = %d %+v: %v", following.status, page, err)
	}
	bad := h.do(http.MethodGet, "/v2/social/users/"+b+"/followers?cursor=invalid", "", nil, nil)
	if bad.status != http.StatusBadRequest {
		t.Fatalf("invalid cursor = %d %s", bad.status, bad.body)
	}
}
