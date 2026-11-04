package social_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/db/sqlcdb"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/fanout"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/social"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/pkg/types"
)

const adminKey = "admin-secret"

type harness struct {
	q       *sqlcdb.Queries
	pool    *pgxpool.Pool
	baseURL string
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	pool := internaltest.NewPostgres(t)
	q := sqlcdb.New(pool)
	h := social.New(social.Deps{Queries: q, MaxMessageLength: 500, AdminAPIKey: adminKey})
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return &harness{q: q, pool: pool, baseURL: srv.URL}
}

// do issues a request, optionally setting the trusted actor header.
func (h *harness) do(t *testing.T, method, path, actor string, body any) (*http.Response, []byte) {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, h.baseURL+path, rdr)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if actor != "" {
		req.Header.Set("X-Actor-Wallet", actor)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	out, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	return resp, out
}

// raw issues a request with explicit headers (for admin api-key + bad bodies).
func (h *harness) raw(t *testing.T, method, path string, headers map[string]string, rawBody string) (*http.Response, []byte) {
	t.Helper()
	var rdr io.Reader
	if rawBody != "" {
		rdr = bytes.NewReader([]byte(rawBody))
	}
	req, err := http.NewRequest(method, h.baseURL+path, rdr)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	out, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	return resp, out
}

func addr(n int) string { return fmt.Sprintf("0x%040x", n) }

func (h *harness) seedPoolMessage(t *testing.T, id, pool, sender, content string, createdAt int64) {
	t.Helper()
	if err := h.q.InsertPoolMessage(context.Background(), sqlcdb.InsertPoolMessageParams{
		ID: id, PoolAddress: pool, Sender: sender, Content: content, CreatedAt: createdAt,
	}); err != nil {
		t.Fatalf("seed pool message: %v", err)
	}
}

// ── Pool chat ─────────────────────────────────────────────────────────────────

type durableHarness struct {
	store        *fanout.DurableStore
	authenticate fanout.AuthenticateUpgrade
	sessions     map[string]string
	server       *httptest.Server
}

func newDurableHarness(t *testing.T, wallets []string, pools map[string]string) *durableHarness {
	t.Helper()
	store, authenticate, sessions := social.NewRepair68ChatFixture(t, wallets, pools)
	r := chi.NewRouter()
	social.New(social.Deps{Durable: store, AuthenticateChat: authenticate}).RegisterRoutes(r)
	server := httptest.NewServer(r)
	t.Cleanup(server.Close)
	return &durableHarness{store: store, authenticate: authenticate, sessions: sessions, server: server}
}
func (h *durableHarness) ctx(t *testing.T, wallet string) context.Context {
	t.Helper()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Fixture-Session", h.sessions[wallet])
	principal, err := h.authenticate(req)
	if err != nil {
		t.Fatal(err)
	}
	return fanout.WithPrincipal(context.Background(), principal)
}
func (h *durableHarness) request(t *testing.T, method, path, wallet, revision string, body any, status int) []byte {
	t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, h.server.URL+path, reader)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Fixture-Session", h.sessions[wallet])
	if revision != "" {
		req.Header.Set("If-Match", revision)
	}
	resp, err := h.server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != status {
		t.Fatalf("%s %s: %d want %d: %s", method, path, resp.StatusCode, status, raw)
	}
	return raw
}
func TestPoolMessages(t *testing.T) {
	pool, alice, bob, creator := addr(1), addr(0xa11ce), addr(0xb0b), addr(0xc0ffee)
	h := newDurableHarness(t, []string{alice, bob, creator}, map[string]string{pool: creator})
	for _, wallet := range []string{alice, bob, creator} {
		if err := h.store.JoinPool(h.ctx(t, wallet), wallet, pool); err != nil {
			t.Fatal(err)
		}
	}
	messages := []fanout.DurableMessage{}
	for i, in := range []struct{ sender, content string }{{alice, "first"}, {alice, "second"}, {bob, "third"}} {
		m, err := h.store.Send(h.ctx(t, in.sender), fanout.SendInput{Actor: in.sender, Kind: "pool", Target: pool, ClientID: fmt.Sprint(i), Content: in.content})
		if err != nil {
			t.Fatal(err)
		}
		messages = append(messages, m)
	}
	path := "/pool/" + pool + "/messages"
	decode := func(raw []byte) fanout.HistoryPage {
		t.Helper()
		var page fanout.HistoryPage
		if err := json.Unmarshal(raw, &page); err != nil {
			t.Fatal(err)
		}
		return page
	}
	var cursor string
	t.Run("list oldest-first within page", func(t *testing.T) {
		raw := h.request(t, "GET", path+"?limit=2", alice, "", nil, 200)
		got := decode(raw)
		if len(got.Messages) != 2 || got.Messages[0].ID != messages[0].ID || got.Messages[1].ID != messages[1].ID || got.Next == "" {
			t.Fatalf("durable order/page: %+v", got)
		}
		var more struct {
			HasMore bool `json:"has_more"`
		}
		if err := json.Unmarshal(raw, &more); err != nil || !more.HasMore {
			t.Fatalf("missing has_more: %s; %v", raw, err)
		}
		cursor = got.Next
	})
	t.Run("signed cursor paginates", func(t *testing.T) {
		got := decode(h.request(t, "GET", path+"?cursor="+cursor, alice, "", nil, 200))
		if len(got.Messages) != 1 || got.Messages[0].ID != messages[2].ID {
			t.Fatalf("cursor page: %+v", got)
		}
		h.request(t, "GET", path+"?before="+messages[2].ID, alice, "", nil, 400)
	})
	t.Run("invalid pool 400", func(t *testing.T) { h.request(t, "GET", "/pool/not-an-addr/messages", alice, "", nil, 400) })
	t.Run("edit own message", func(t *testing.T) {
		h.request(t, "PATCH", path+"/"+messages[0].ID, alice, "1", types.EditRequest{Content: "edited"}, 200)
		got := decode(h.request(t, "GET", path, alice, "", nil, 200))
		if got.Messages[0].Content != "edited" || got.Messages[0].Revision != 2 || got.Messages[0].ID != messages[0].ID {
			t.Fatalf("edit lost identity: %+v", got)
		}
	})
	t.Run("edit foreign message refuses without revealing ownership", func(t *testing.T) {
		h.request(t, "PATCH", path+"/"+messages[2].ID, alice, "1", types.EditRequest{Content: "x"}, 404)
	})
	t.Run("edit missing actor 401", func(t *testing.T) {
		h.request(t, "PATCH", path+"/"+messages[0].ID, "", "1", types.EditRequest{Content: "x"}, 401)
	})
	t.Run("edit missing message 404", func(t *testing.T) {
		h.request(t, "PATCH", path+"/missing", alice, "1", types.EditRequest{Content: "x"}, 404)
	})
	t.Run("edit empty content 400", func(t *testing.T) {
		h.request(t, "PATCH", path+"/"+messages[0].ID, alice, "2", types.EditRequest{Content: "   "}, 400)
	})
	t.Run("creator cannot bypass durable sender authority", func(t *testing.T) {
		h.request(t, "DELETE", path+"/"+messages[2].ID, creator, "1", nil, 404)
		h.request(t, "DELETE", path+"/"+messages[2].ID, bob, "1", nil, 200)
		got := decode(h.request(t, "GET", path, alice, "", nil, 200))
		if len(got.Messages) != 3 || got.Messages[2].ID != messages[2].ID || !got.Messages[2].Deleted || got.Messages[2].Content != "" || got.Messages[2].Revision != 2 {
			t.Fatalf("delete lost durable tombstone: %+v", got)
		}
	})
	t.Run("delete by non-owner refuses", func(t *testing.T) { h.request(t, "DELETE", path+"/"+messages[1].ID, bob, "1", nil, 404) })
}

// ── Comments ──────────────────────────────────────────────────────────────────

func TestComments(t *testing.T) {
	h := newHarness(t)
	pool := addr(10)
	alice := addr(0xa1)
	bob := addr(0xb2)

	var rootID string
	t.Run("create comment", func(t *testing.T) {
		resp, out := h.do(t, http.MethodPost, "/pool/"+pool+"/comments", alice, types.CreateCommentRequest{Content: "<b>gm</b>"})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d (%s)", resp.StatusCode, out)
		}
		var got types.CreatedResponse
		_ = json.Unmarshal(out, &got)
		if !got.Success || got.ID == "" {
			t.Fatalf("resp = %+v", got)
		}
		rootID = got.ID
	})

	t.Run("create reply with valid parent", func(t *testing.T) {
		resp, out := h.do(t, http.MethodPost, "/pool/"+pool+"/comments", bob, types.CreateCommentRequest{Content: "reply", ParentID: &rootID})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d (%s)", resp.StatusCode, out)
		}
	})

	t.Run("create reply with bad parent 400", func(t *testing.T) {
		bad := "does-not-exist"
		resp, _ := h.do(t, http.MethodPost, "/pool/"+pool+"/comments", bob, types.CreateCommentRequest{Content: "x", ParentID: &bad})
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", resp.StatusCode)
		}
	})

	t.Run("create missing actor 401", func(t *testing.T) {
		resp, _ := h.do(t, http.MethodPost, "/pool/"+pool+"/comments", "", types.CreateCommentRequest{Content: "x"})
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", resp.StatusCode)
		}
	})

	t.Run("create empty content 400", func(t *testing.T) {
		resp, _ := h.do(t, http.MethodPost, "/pool/"+pool+"/comments", alice, types.CreateCommentRequest{Content: "<br>"})
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", resp.StatusCode)
		}
	})

	t.Run("list comments sanitized + like count", func(t *testing.T) {
		resp, out := h.do(t, http.MethodGet, "/pool/"+pool+"/comments", "", nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status = %d", resp.StatusCode)
		}
		var got types.CommentsResponse
		_ = json.Unmarshal(out, &got)
		if len(got.Comments) != 2 {
			t.Fatalf("len = %d, want 2", len(got.Comments))
		}
		var root *types.Comment
		for i := range got.Comments {
			if got.Comments[i].ID == rootID {
				root = &got.Comments[i]
			}
		}
		if root == nil || root.Content != "gm" {
			t.Fatalf("root content = %v (want sanitized 'gm')", root)
		}
		if root.LikeCount != 0 {
			t.Errorf("like_count = %d, want 0", root.LikeCount)
		}
	})

	t.Run("like is idempotent and counted", func(t *testing.T) {
		for i := 0; i < 2; i++ {
			resp, _ := h.do(t, http.MethodPut, "/comments/"+rootID+"/like", bob, nil)
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("like status = %d", resp.StatusCode)
			}
		}
		_, out := h.do(t, http.MethodGet, "/pool/"+pool+"/comments", "", nil)
		var got types.CommentsResponse
		_ = json.Unmarshal(out, &got)
		for _, c := range got.Comments {
			if c.ID == rootID && c.LikeCount != 1 {
				t.Errorf("like_count = %d, want 1 (idempotent)", c.LikeCount)
			}
		}
		// Unlike.
		h.do(t, http.MethodDelete, "/comments/"+rootID+"/like", bob, nil)
		_, out = h.do(t, http.MethodGet, "/pool/"+pool+"/comments", "", nil)
		_ = json.Unmarshal(out, &got)
		for _, c := range got.Comments {
			if c.ID == rootID && c.LikeCount != 0 {
				t.Errorf("like_count after unlike = %d, want 0", c.LikeCount)
			}
		}
	})

	t.Run("like missing comment 404", func(t *testing.T) {
		resp, _ := h.do(t, http.MethodPut, "/comments/nope/like", bob, nil)
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("status = %d, want 404", resp.StatusCode)
		}
	})

	t.Run("edit own comment, foreign 403", func(t *testing.T) {
		resp, _ := h.do(t, http.MethodPatch, "/comments/"+rootID, alice, types.EditRequest{Content: "edited"})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("own edit status = %d", resp.StatusCode)
		}
		resp, _ = h.do(t, http.MethodPatch, "/comments/"+rootID, bob, types.EditRequest{Content: "x"})
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("foreign edit status = %d, want 403", resp.StatusCode)
		}
	})

	t.Run("delete by author", func(t *testing.T) {
		resp, _ := h.do(t, http.MethodDelete, "/comments/"+rootID, alice, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("delete status = %d", resp.StatusCode)
		}
	})

	t.Run("banned actor cannot comment", func(t *testing.T) {
		banned := addr(0xbad)
		if err := h.q.InsertBan(context.Background(), sqlcdb.InsertBanParams{
			ID: "ban1", Wallet: banned, BannedBy: addr(0xc0ffee), CreatedAt: 1,
		}); err != nil {
			t.Fatalf("seed ban: %v", err)
		}
		resp, _ := h.do(t, http.MethodPost, "/pool/"+pool+"/comments", banned, types.CreateCommentRequest{Content: "spam"})
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("status = %d, want 403 (banned)", resp.StatusCode)
		}
	})
}

// ── DMs (reads) ───────────────────────────────────────────────────────────────

func TestDMReads(t *testing.T) {
	alice, bob, carol := addr(0xaa), addr(0xbb), addr(0xcc)
	h := newDurableHarness(t, []string{alice, bob, carol}, nil)
	message, err := h.store.Send(h.ctx(t, alice), fanout.SendInput{Actor: alice, Kind: "dm", Target: bob, ClientID: "dm-read-proof", Content: "hey"})
	if err != nil {
		t.Fatal(err)
	}
	path := "/dm/conversations/" + message.ConversationID + "/messages"
	t.Run("list conversations maps peer", func(t *testing.T) {
		raw := h.request(t, "GET", "/dm/conversations", alice, "", nil, 200)
		var got types.DmConversationsResponse
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatal(err)
		}
		if len(got.Conversations) != 1 || got.Conversations[0].Peer != bob || got.Conversations[0].ID != message.ConversationID || got.Conversations[0].LastMessageAt == nil {
			t.Fatalf("durable conversations: %+v", got)
		}
	})
	t.Run("list conversations requires session", func(t *testing.T) { h.request(t, "GET", "/dm/conversations", "", "", nil, 401) })
	t.Run("participant reads messages", func(t *testing.T) {
		raw := h.request(t, "GET", path, bob, "", nil, 200)
		var got fanout.HistoryPage
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatal(err)
		}
		if len(got.Messages) != 1 || got.Messages[0].Content != "hey" || got.Messages[0].ID != message.ID {
			t.Fatalf("durable messages: %+v", got)
		}
	})
	t.Run("non-participant has same refusal as missing conversation", func(t *testing.T) { h.request(t, "GET", path, carol, "", nil, 404) })
	t.Run("missing conversation 404", func(t *testing.T) { h.request(t, "GET", "/dm/conversations/dm:x:y/messages", alice, "", nil, 404) })
}

// ── Followers ─────────────────────────────────────────────────────────────────

func TestFollows(t *testing.T) {
	h := newHarness(t)
	alice := addr(0xa)
	bob := addr(0xb)

	t.Run("follow then stats", func(t *testing.T) {
		resp, _ := h.do(t, http.MethodPut, "/follow/"+bob, alice, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("follow status = %d", resp.StatusCode)
		}
		// Idempotent.
		h.do(t, http.MethodPut, "/follow/"+bob, alice, nil)

		_, out := h.do(t, http.MethodGet, "/users/"+bob+"/follow-stats", alice, nil)
		var st types.FollowStatsResponse
		_ = json.Unmarshal(out, &st)
		if st.Followers != 1 || !st.IsFollowing {
			t.Errorf("stats = %+v (want followers=1, is_following=true)", st)
		}
		_, out = h.do(t, http.MethodGet, "/users/"+alice+"/follow-stats", alice, nil)
		_ = json.Unmarshal(out, &st)
		if st.Following != 1 {
			t.Errorf("alice following = %d, want 1", st.Following)
		}
	})

	t.Run("list followers + following", func(t *testing.T) {
		_, out := h.do(t, http.MethodGet, "/users/"+bob+"/followers", "", nil)
		var fr types.FollowersResponse
		_ = json.Unmarshal(out, &fr)
		if len(fr.Followers) != 1 || fr.Followers[0].Wallet != alice {
			t.Errorf("followers = %+v", fr.Followers)
		}
		_, out = h.do(t, http.MethodGet, "/users/"+alice+"/following", "", nil)
		var fg types.FollowingResponse
		_ = json.Unmarshal(out, &fg)
		if len(fg.Following) != 1 || fg.Following[0].Wallet != bob {
			t.Errorf("following = %+v", fg.Following)
		}
	})

	t.Run("self-follow 400", func(t *testing.T) {
		resp, _ := h.do(t, http.MethodPut, "/follow/"+alice, alice, nil)
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", resp.StatusCode)
		}
	})

	t.Run("follow missing actor 401", func(t *testing.T) {
		resp, _ := h.do(t, http.MethodPut, "/follow/"+bob, "", nil)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", resp.StatusCode)
		}
	})

	t.Run("unfollow", func(t *testing.T) {
		resp, _ := h.do(t, http.MethodDelete, "/follow/"+bob, alice, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("unfollow status = %d", resp.StatusCode)
		}
		_, out := h.do(t, http.MethodGet, "/users/"+bob+"/follow-stats", "", nil)
		var st types.FollowStatsResponse
		_ = json.Unmarshal(out, &st)
		if st.Followers != 0 {
			t.Errorf("followers after unfollow = %d, want 0", st.Followers)
		}
	})
}

// ── Moderation ────────────────────────────────────────────────────────────────

func TestModeration(t *testing.T) {
	social.RunRepairM1029Moderation(t)
}

// TestAdminDisabled verifies that an unconfigured admin key returns 503.
func TestAdminDisabled(t *testing.T) {
	pool := internaltest.NewPostgres(t)
	h := social.New(social.Deps{Queries: sqlcdb.New(pool), MaxMessageLength: 500}) // no admin key
	r := chi.NewRouter()
	h.RegisterRoutes(r)
	srv := httptest.NewServer(r)
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/admin/bans", nil)
	req.Header.Set("X-API-Key", "anything")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", resp.StatusCode)
	}
}

func ptrInt64(v int64) *int64 { return &v }

func min2(a, b string) string {
	if a < b {
		return a
	}
	return b
}
func max2(a, b string) string {
	if a > b {
		return a
	}
	return b
}
