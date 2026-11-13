package social_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/pkg/types"
)

func TestCommentRoutesPreserveOwnershipAndInputBounds(t *testing.T) {
	h := newHarness(t)
	pool, otherPool := addr(10), addr(11)
	alice, bob := addr(0xa1), addr(0xb2)
	created, raw := h.do(t, http.MethodPost, "/pool/"+pool+"/comments", alice, types.CreateCommentRequest{Content: "kept comment"})
	if created.StatusCode != http.StatusOK {
		t.Fatalf("comment create = %d %s", created.StatusCode, raw)
	}
	var reply types.CreatedResponse
	if err := json.Unmarshal(raw, &reply); err != nil || reply.ID == "" {
		t.Fatalf("comment identity = %+v: %v", reply, err)
	}
	for _, tc := range []struct {
		method, path, actor string
		body                any
		status              int
	}{
		{http.MethodGet, "/pool/invalid/comments", "", nil, http.StatusBadRequest},
		{http.MethodPost, "/pool/invalid/comments", alice, types.CreateCommentRequest{Content: "hello"}, http.StatusBadRequest},
		{http.MethodPost, "/pool/" + otherPool + "/comments", alice, types.CreateCommentRequest{Content: "reply", ParentID: &reply.ID}, http.StatusBadRequest},
		{http.MethodPost, "/pool/" + pool + "/comments", alice, types.CreateCommentRequest{Content: strings.Repeat("a", 501)}, http.StatusBadRequest},
		{http.MethodPatch, "/comments/" + reply.ID, "", types.EditRequest{Content: "edit"}, http.StatusUnauthorized},
		{http.MethodPatch, "/comments/" + reply.ID, alice, types.EditRequest{Content: "  "}, http.StatusBadRequest},
		{http.MethodPatch, "/comments/" + reply.ID, alice, types.EditRequest{Content: strings.Repeat("a", 10001)}, http.StatusBadRequest},
		{http.MethodPatch, "/comments/absent", alice, types.EditRequest{Content: "edit"}, http.StatusNotFound},
		{http.MethodDelete, "/comments/" + reply.ID, "", nil, http.StatusUnauthorized},
		{http.MethodDelete, "/comments/" + reply.ID, bob, nil, http.StatusForbidden},
		{http.MethodPut, "/comments/" + reply.ID + "/like", "", nil, http.StatusUnauthorized},
		{http.MethodDelete, "/comments/" + reply.ID + "/like", "", nil, http.StatusUnauthorized},
	} {
		response, body := h.do(t, tc.method, tc.path, tc.actor, tc.body)
		if response.StatusCode != tc.status {
			t.Fatalf("%s %s = %d, want %d: %s", tc.method, tc.path, response.StatusCode, tc.status, body)
		}
	}
	response, raw := h.raw(t, http.MethodPost, "/pool/"+pool+"/comments", map[string]string{"X-Actor-Wallet": alice}, `{"content":`)
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("malformed comment = %d %s", response.StatusCode, raw)
	}
	response, raw = h.raw(t, http.MethodPatch, "/comments/"+reply.ID, map[string]string{"X-Actor-Wallet": alice}, `{"content":`)
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("malformed edit = %d %s", response.StatusCode, raw)
	}
	response, raw = h.do(t, http.MethodDelete, "/comments/"+reply.ID, alice, nil)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("author deletion = %d %s", response.StatusCode, raw)
	}
	response, raw = h.do(t, http.MethodPatch, "/comments/"+reply.ID, alice, types.EditRequest{Content: "cannot restore"})
	if response.StatusCode != http.StatusGone {
		t.Fatalf("deleted edit = %d %s", response.StatusCode, raw)
	}
	var deleted bool
	if err := h.pool.QueryRow(context.Background(), `SELECT deleted FROM comments WHERE id=$1`, reply.ID).Scan(&deleted); err != nil || !deleted {
		t.Fatalf("stored deletion = %t: %v", deleted, err)
	}
}
