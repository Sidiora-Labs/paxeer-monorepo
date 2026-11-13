package interactions

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

func TestConnectedHTTPMutationsAndReads(t *testing.T) {
	f := connected(t)
	a, b := f.wallets[0], f.wallets[1]
	routes := f.http.Routes()
	request := func(actor, method, path, key string, body any, want int) map[string]any {
		t.Helper()
		var raw []byte
		if body != nil {
			var err error
			raw, err = json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
		}
		r := httptest.NewRequest(method, path, bytes.NewReader(raw))
		if actor != "" {
			r = r.WithContext(f.ctx[actor])
		}
		if key != "" {
			r.Header.Set("Idempotency-Key", key)
		}
		if body != nil {
			r.Header.Set("Content-Type", "application/json")
		}
		response := httptest.NewRecorder()
		routes.ServeHTTP(response, r)
		if response.Code != want {
			t.Fatalf("%s %s = %d, want %d: %s", method, path, response.Code, want, response.Body.String())
		}
		if response.Body.Len() == 0 {
			return nil
		}
		var out map[string]any
		if err := json.Unmarshal(response.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	data := func(response map[string]any) map[string]any {
		t.Helper()
		value, ok := response["data"].(map[string]any)
		if !ok {
			t.Fatalf("response data = %v", response)
		}
		return value
	}
	privacy := data(request(a, http.MethodGet, "/me/social/privacy", "", nil, http.StatusOK))
	if privacy["inbox"] != "everyone" {
		t.Fatalf("initial privacy = %v", privacy)
	}
	request(a, http.MethodPatch, "/me/social/privacy", "", map[string]any{"inbox": "followed_only", "expectedRevision": "1"}, http.StatusOK)
	privacy = data(request(a, http.MethodGet, "/me/social/privacy", "", nil, http.StatusOK))
	if privacy["inbox"] != "followed_only" {
		t.Fatalf("updated privacy = %v", privacy)
	}
	request(a, http.MethodPut, "/me/social/blocks/"+b, "", nil, http.StatusOK)
	request(a, http.MethodDelete, "/me/social/blocks/"+b, "", nil, http.StatusOK)
	request(a, http.MethodPut, "/follow/"+b, "follow-a-b", nil, http.StatusOK)
	following := data(request(a, http.MethodGet, "/users/"+a+"/following?limit=10", "", nil, http.StatusOK))
	if items, ok := following["items"].([]any); !ok || len(items) != 1 {
		t.Fatalf("following page = %v", following)
	}
	stats := data(request(a, http.MethodGet, "/users/"+b+"/follow-stats", "", nil, http.StatusOK))
	if stats["followers"] != "1" || stats["is_following"] != true {
		t.Fatalf("follow stats = %v", stats)
	}
	request(a, http.MethodDelete, "/follow/"+b, "unfollow-a-b", nil, http.StatusNoContent)

	clientID := uuid.NewString()
	created := data(request(a, http.MethodPost, "/pool/"+f.pool+"/comments", "comment-a", map[string]any{"body": "visible comment", "clientCommentId": clientID}, http.StatusOK))
	comment, ok := created["comment"].(map[string]any)
	if !ok || comment["body"] != "visible comment" {
		t.Fatalf("created comment = %v", created)
	}
	id, ok := comment["commentId"].(string)
	if !ok || id == "" {
		t.Fatalf("comment id = %v", comment)
	}
	liked := data(request(b, http.MethodPut, "/comments/"+id+"/like", "like-a", nil, http.StatusOK))
	if liked["likeCount"] != "1" || liked["liked"] != true {
		t.Fatalf("comment reaction = %v", liked)
	}
	request(b, http.MethodDelete, "/comments/"+id+"/like", "unlike-a", nil, http.StatusNoContent)
	edited := data(request(a, http.MethodPatch, "/comments/"+id, "edit-a", map[string]any{"body": "edited comment", "expectedRevision": "1"}, http.StatusOK))
	if edited["comment"].(map[string]any)["body"] != "edited comment" {
		t.Fatalf("edited comment = %v", edited)
	}
	pageRequest := httptest.NewRequest(http.MethodGet, "/pool/"+f.pool+"/comments?limit=10", nil)
	pageResponse := httptest.NewRecorder()
	routes.ServeHTTP(pageResponse, pageRequest)
	if pageResponse.Code != http.StatusOK || pageResponse.Header().Get("ETag") == "" {
		t.Fatalf("comment page = %d %s", pageResponse.Code, pageResponse.Body.String())
	}
	pageRequest.Header.Set("If-None-Match", pageResponse.Header().Get("ETag"))
	cachedPage := httptest.NewRecorder()
	routes.ServeHTTP(cachedPage, pageRequest)
	if cachedPage.Code != http.StatusNotModified {
		t.Fatalf("unchanged comment page = %d %s", cachedPage.Code, cachedPage.Body.String())
	}
	calloutID := uuid.NewString()
	tx, err := f.db.Begin(f.ctx[a])
	if err != nil {
		t.Fatal(err)
	}
	if err := ApplyCalloutAuthorityTx(f.ctx[a], tx, calloutID, 1, a, false); err != nil {
		_ = tx.Rollback(f.ctx[a])
		t.Fatal(err)
	}
	if err := tx.Commit(f.ctx[a]); err != nil {
		t.Fatal(err)
	}
	request(a, http.MethodPost, "/callouts/"+calloutID+"/references", "reference-callout", map[string]any{"revision": "1", "mentions": []string{b}, "transactionHash": ""}, http.StatusOK)
	var intents int
	if err := f.db.QueryRow(f.ctx[a], `SELECT count(*) FROM interaction_notification_intents WHERE target=$1 AND recipient=$2`, calloutID, b).Scan(&intents); err != nil || intents != 1 {
		t.Fatalf("callout intent count = %d: %v", intents, err)
	}
	for _, tc := range []struct {
		name, actor, method, path, key string
		body                           any
		status                         int
	}{
		{"following limit", a, http.MethodGet, "/users/" + a + "/following?limit=0", "", nil, http.StatusBadRequest},
		{"comments limit", a, http.MethodGet, "/pool/" + f.pool + "/comments?limit=0", "", nil, http.StatusBadRequest},
		{"private read", "", http.MethodGet, "/me/social/privacy", "", nil, http.StatusForbidden},
		{"follow key", a, http.MethodPut, "/follow/" + b, "", nil, http.StatusBadRequest},
		{"comment client id", a, http.MethodPost, "/pool/" + f.pool + "/comments", "bad-comment", map[string]string{"body": "new comment", "clientCommentId": "invalid"}, http.StatusBadRequest},
		{"edit revision", a, http.MethodPatch, "/comments/" + id, "bad-edit", map[string]string{"body": "changed", "expectedRevision": "invalid"}, http.StatusBadRequest},
		{"hide revision", b, http.MethodPost, "/comments/" + id + "/hide", "bad-hide", map[string]string{"poolAddress": f.pool, "reason": "review", "expectedRevision": "invalid"}, http.StatusBadRequest},
		{"callout revision", a, http.MethodPost, "/callouts/" + calloutID + "/references", "bad-reference", map[string]any{"revision": "invalid", "mentions": []string{b}}, http.StatusBadRequest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := request(tc.actor, tc.method, tc.path, tc.key, tc.body, tc.status)
			if result["code"] != "INVALID_INPUT" && !(tc.name == "private read" && result["code"] == "SCOPE_DENIED") {
				t.Fatalf("request error = %v", result)
			}
		})
	}
	request(b, http.MethodPost, "/comments/"+id+"/hide", "hide-a", map[string]any{"poolAddress": f.pool, "reason": "community policy", "expectedRevision": "2"}, http.StatusOK)
	page := data(request(a, http.MethodGet, "/pool/"+f.pool+"/comments", "", nil, http.StatusOK))
	if items, ok := page["items"].([]any); !ok || len(items) != 0 {
		t.Fatalf("hidden comment page = %v", page)
	}
	c := f.wallets[2]
	if _, err := f.db.Exec(f.ctx[a], `INSERT INTO interaction_follows(follower,followee) VALUES($1,$3),($2,$3)`, b, c, a); err != nil {
		t.Fatal(err)
	}
	firstFollows := request(a, http.MethodGet, "/users/"+a+"/followers?limit=1", "", nil, http.StatusOK)
	if items, ok := data(firstFollows)["items"].([]any); !ok || len(items) != 1 || firstFollows["nextCursor"] == nil {
		t.Fatalf("first following page = %v", firstFollows)
	}
	secondFollows := request(a, http.MethodGet, "/users/"+a+"/followers?limit=1&cursor="+firstFollows["nextCursor"].(string), "", nil, http.StatusOK)
	if items, ok := data(secondFollows)["items"].([]any); !ok || len(items) != 1 || secondFollows["nextCursor"] != nil {
		t.Fatalf("second following page = %v", secondFollows)
	}
	for _, body := range []string{"First page comment", "Second page comment"} {
		request(a, http.MethodPost, "/pool/"+f.pool+"/comments", "page-"+body, map[string]any{"body": body, "clientCommentId": uuid.NewString()}, http.StatusOK)
	}
	firstComments := request(a, http.MethodGet, "/pool/"+f.pool+"/comments?limit=1", "", nil, http.StatusOK)
	if items, ok := data(firstComments)["items"].([]any); !ok || len(items) != 1 || firstComments["nextCursor"] == nil {
		t.Fatalf("first comment page = %v", firstComments)
	}
	secondComments := request(a, http.MethodGet, "/pool/"+f.pool+"/comments?limit=1&cursor="+firstComments["nextCursor"].(string), "", nil, http.StatusOK)
	if items, ok := data(secondComments)["items"].([]any); !ok || len(items) != 1 || secondComments["nextCursor"] != nil {
		t.Fatalf("second comment page = %v", secondComments)
	}
}
