package interactions_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/devsocial"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/migrate"
	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
)

type directRouteHarness struct {
	t      *testing.T
	server *httptest.Server
	db     *pgx.Conn
	pool   string
	token  string
	owner  string
}

func newDirectRouteHarness(t *testing.T) *directRouteHarness {
	t.Helper()
	ctx := context.Background()
	admin := os.Getenv("SOCIAL_FIXTURE_POSTGRES_DSN")
	if admin == "" {
		admin = "postgres://kindle_fixture:kindle_fixture_password@127.0.0.1:25432/kindle_fixture?sslmode=disable"
	}
	dsn := internaltest.DevelopmentPostgresDSN(t, admin)
	if err := migrate.Up(ctx, dsn); err != nil {
		t.Fatal(err)
	}
	runtimeDSN, err := devsocial.RuntimeDSN(dsn)
	if err != nil {
		t.Fatal(err)
	}
	db, err := pgx.Connect(ctx, runtimeDSN)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close(context.Background()) })
	pool, err := pgxpool.New(ctx, runtimeDSN)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	uri := os.Getenv("SOCIAL_FIXTURE_REDIS_URL")
	if uri == "" {
		uri = "redis://:kindle_fixture_password@127.0.0.1:26379/9"
	}
	options, err := goredis.ParseURL(uri)
	if err != nil {
		t.Fatal(err)
	}
	redis := goredis.NewClient(options)
	t.Cleanup(func() { _ = redis.Close() })
	h := &directRouteHarness{t: t, db: db, pool: directAddr(0x9001), token: directAddr(0x7001), owner: directAddr(0xc0ffee)}
	if _, err = db.Exec(ctx, `INSERT INTO indexer.pools(pool_address,token_address,creator,optical,pool_id,nft_id,created_at,created_block,tx_hash) VALUES($1,$2,$3,$4,$5,1,$6,100,$7)`, h.pool, h.token, h.owner, directAddr(0xb7), "0x"+strings.Repeat("ab", 32), time.Now().Unix(), "0x"+strings.Repeat("cd", 32)); err != nil {
		t.Fatal(err)
	}
	runtime, err := devsocial.New(ctx, devsocial.Deps{Pool: pool, Redis: redis, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	router := chi.NewRouter()
	if err = runtime.Register(router); err != nil {
		t.Fatal(err)
	}
	h.server = httptest.NewServer(router)
	t.Cleanup(h.server.Close)
	return h
}

func directAddr(n int) string { return fmt.Sprintf("0x%040x", n) }

type directResponse struct {
	status int
	body   []byte
}

func (h *directRouteHarness) do(method, path, actor string, body any, headers map[string]string) directResponse {
	h.t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			h.t.Fatal(err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, h.server.URL+path, reader)
	if err != nil {
		h.t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if actor != "" {
		req.Header.Set(devsocial.ActorHeader, actor)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		h.t.Fatal(err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		h.t.Fatal(err)
	}
	return directResponse{res.StatusCode, raw}
}

func (h *directRouteHarness) expect(r directResponse, status int, schema string) map[string]any {
	h.t.Helper()
	if r.status != status {
		h.t.Fatalf("status %d, want %d: %s", r.status, status, r.body)
	}
	if schema == "" {
		return nil
	}
	if err := v2.ValidateJSON(schema, r.body); err != nil {
		h.t.Fatalf("%s: %v: %s", schema, err, r.body)
	}
	var out map[string]any
	if err := json.Unmarshal(r.body, &out); err != nil {
		h.t.Fatal(err)
	}
	return out
}

func directData(m map[string]any) map[string]any { return m["data"].(map[string]any) }
func directItems(m map[string]any) []any         { return directData(m)["items"].([]any) }

func TestDevelopmentRoutesExerciseInteractions(t *testing.T) {
	h := newDirectRouteHarness(t)
	alice, bob := directAddr(0xa11ce), directAddr(0xb0b)
	comment := map[string]any{"body": "first", "clientCommentId": uuid.NewString()}
	// ── comments (contract + /v2/social share interaction_comments) ────────
	c := directData(h.expect(h.do("POST", "/v2/pools/"+h.pool+"/comments", alice, comment, nil), 201, "CommentCreateResponse"))["comment"].(map[string]any)
	if c["authorWallet"] != alice || c["revision"] != "1" || c["likeCount"] != "0" {
		t.Fatalf("comment = %v", c)
	}
	cid := c["commentId"].(string)
	if again := directData(h.expect(h.do("POST", "/v2/pools/"+h.pool+"/comments", alice, comment, nil), 201, "CommentCreateResponse"))["comment"].(map[string]any); again["commentId"] != cid {
		t.Fatalf("comment retry = %v", again)
	}
	h.expect(h.do("POST", "/v2/pools/"+directAddr(0xdead)+"/comments", alice, map[string]any{"body": "x", "clientCommentId": uuid.NewString()}, nil), 404, "Error")
	reply := directData(h.expect(h.do("POST", "/v2/pools/"+h.pool+"/comments", bob, map[string]any{"body": "reply", "clientCommentId": uuid.NewString(), "parentCommentId": cid}, nil), 201, "CommentCreateResponse"))["comment"].(map[string]any)
	if reply["parentCommentId"] != cid {
		t.Fatalf("reply = %v", reply)
	}
	like := directData(h.expect(h.do("PUT", "/v2/comments/"+cid+"/likes/me", bob, nil, nil), 200, "CommentLikeResponse"))
	if like["likeCount"] != "1" || like["liked"] != true {
		t.Fatalf("like = %v", like)
	}
	var socialList struct {
		Data struct {
			Items []map[string]any `json:"items"`
		} `json:"data"`
	}
	r := h.do("GET", "/v2/social/pool/"+h.pool+"/comments?limit=50", "", nil, nil)
	if r.status != 200 || json.Unmarshal(r.body, &socialList) != nil || len(socialList.Data.Items) != 2 {
		t.Fatalf("social list %d %s", r.status, r.body)
	}
	edited := directData(h.expect(h.do("PATCH", "/v2/comments/"+cid, alice, map[string]any{"body": "first, edited", "expectedRevision": "1"}, nil), 200, "CommentEditResponse"))["comment"].(map[string]any)
	if edited["revision"] != "2" || edited["body"] != "first, edited" {
		t.Fatalf("edit = %v", edited)
	}
	h.expect(h.do("PATCH", "/v2/comments/"+cid, alice, map[string]any{"body": "stale", "expectedRevision": "1"}, nil), 409, "Error")
	h.expect(h.do("PATCH", "/v2/comments/"+cid, bob, map[string]any{"body": "not mine", "expectedRevision": "2"}, nil), 403, "Error")
	h.expect(h.do("DELETE", "/v2/comments/"+cid+"/likes/me", bob, nil, nil), 204, "")
	if social := h.do("POST", "/v2/social/pool/"+h.pool+"/comments", bob, map[string]any{"body": "via social", "clientCommentId": uuid.NewString()}, map[string]string{"Idempotency-Key": uuid.NewString()}); social.status != 200 {
		t.Fatalf("social create %d %s", social.status, social.body)
	}
	report := directData(h.expect(h.do("POST", "/v2/reports", bob, map[string]any{"targetType": "comment", "targetId": cid, "reason": "spam", "evidence": "repeated"}, nil), 201, "ReportCreateResponse"))["report"].(map[string]any)
	if report["state"] != "open" || report["reason"] != "spam" || report["reporterWallet"] != bob {
		t.Fatalf("report = %v", report)
	}
	h.expect(h.do("POST", "/v2/reports", bob, map[string]any{"targetType": "comment", "targetId": uuid.NewString(), "reason": "spam", "evidence": ""}, nil), 404, "Error")
	h.expect(h.do("DELETE", "/v2/comments/"+cid, alice, nil, nil), 204, "")
	if got := directItems(h.expect(h.do("GET", "/v2/pools/"+h.pool+"/comments", "", nil, nil), 200, "CommentsListResponse")); len(got) != 2 {
		t.Fatalf("after delete = %v", got)
	}

	// ── follows through /v2/social and the contract route share one graph ──
	if f := h.do("PUT", "/v2/social/follow/"+alice, bob, nil, nil); f.status != 200 {
		t.Fatalf("social follow %d %s", f.status, f.body)
	}
	fsResponse := h.do("GET", "/v2/social/users/"+alice+"/follow-stats", bob, nil, nil)
	var fsEnvelope struct {
		Data map[string]any `json:"data"`
	}
	if fsResponse.status != 200 || json.Unmarshal(fsResponse.body, &fsEnvelope) != nil {
		t.Fatalf("follow stats = %d %s", fsResponse.status, fsResponse.body)
	}
	fs := fsEnvelope.Data
	if fs["followers"] != "1" || fs["is_following"] != true {
		t.Fatalf("follow stats = %v", fs)
	}

}
