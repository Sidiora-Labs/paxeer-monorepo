package devsocial

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

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/fanout"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/migrate"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
)

type directRouteHarness struct {
	t       *testing.T
	server  *httptest.Server
	db      *pgx.Conn
	runtime *Runtime
	redis   *goredis.Client
	pool    string
	token   string
	owner   string
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
	runtimeDSN, err := RuntimeDSN(dsn)
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
	runtime, err := New(ctx, Deps{Pool: pool, Redis: redis, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatal(err)
	}
	router := chi.NewRouter()
	if err = runtime.Register(router); err != nil {
		t.Fatal(err)
	}
	h.server = httptest.NewServer(router)
	h.runtime = runtime
	h.redis = redis
	t.Cleanup(h.server.Close)
	return h
}

func TestDevelopmentRelayPublishesCommittedDirectMessage(t *testing.T) {
	h := newDirectRouteHarness(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	subscriber := h.redis.Subscribe(ctx, fanout.WalletChannel("", directAddr(0xb0b)))
	defer func() { _ = subscriber.Close() }()
	if _, err := subscriber.Receive(ctx); err != nil {
		t.Fatal(err)
	}
	message, err := h.runtime.Chat.Send(ctx, fanout.SendInput{Actor: directAddr(0xa11ce), Kind: "dm", Target: directAddr(0xb0b), Content: "committed relay message", ClientID: uuid.NewString()})
	if err != nil {
		t.Fatal(err)
	}
	if err := h.runtime.RelayOnce(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case event := <-subscriber.Channel():
		if event == nil || !strings.Contains(event.Payload, message.ID) {
			t.Fatalf("published direct message = %+v", event)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	var delivered bool
	if err := h.db.QueryRow(ctx, `SELECT delivered_at IS NOT NULL FROM chat_outbox WHERE message_id=$1`, message.ID).Scan(&delivered); err != nil || !delivered {
		t.Fatalf("committed relay delivery = %t: %v", delivered, err)
	}
	stopCtx, stop := context.WithCancel(context.Background())
	stop()
	if err := h.runtime.Run(stopCtx); err != nil {
		t.Fatalf("stopped relay = %v", err)
	}
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
		req.Header.Set(ActorHeader, actor)
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

func TestDevelopmentRoutesOnRuntime(t *testing.T) {
	h := newDirectRouteHarness(t)
	alice, bob := directAddr(0xa11ce), directAddr(0xb0b)

	// ── anonymous reads: real empty collections ────────────────────────────
	if got := directItems(h.expect(h.do("GET", "/v2/pools/"+h.pool+"/callouts", "", nil, nil), 200, "CalloutListResponse")); len(got) != 0 {
		t.Fatalf("callouts = %v", got)
	}
	if got := directItems(h.expect(h.do("GET", "/v2/bounties", "", nil, nil), 200, "BountyListResponse")); len(got) != 0 {
		t.Fatalf("bounties = %v", got)
	}
	stats := directData(h.expect(h.do("GET", "/v2/bounties/stats", "", nil, nil), 200, "BountiesStatsResponse"))
	if stats["openCount"] != float64(0) || stats["unclaimedPoolUsdlRaw"] != "0" || stats["paidOutUsdlRaw"] != "0" || stats["submissionsCount"] != float64(0) {
		t.Fatalf("stats = %v", stats)
	}
	if got := directItems(h.expect(h.do("GET", "/v2/pools/"+h.pool+"/comments", "", nil, nil), 200, "CommentsListResponse")); len(got) != 0 {
		t.Fatalf("comments = %v", got)
	}
	h.expect(h.do("GET", "/v2/bounties?state=bogus", "", nil, nil), 422, "Error")
	h.expect(h.do("GET", "/v2/bounties?poolAddress=invalid", "", nil, nil), 422, "Error")
	h.expect(h.do("GET", "/v2/bounties?cursor=nope", "", nil, nil), 400, "Error")

	// ── writes without a resolved actor are 401 ────────────────────────────
	callout := map[string]any{"poolAddress": h.pool, "body": "accumulating here", "direction": "bullish", "disclosure": "organic", "clientCalloutId": uuid.NewString()}
	bounty := map[string]any{"title": "Best meme", "description": "Make the best meme for the coin.", "assetAddress": strings.ToLower(deployment.SettlementAddress), "budgetRaw": "5000000", "poolAddress": h.pool}
	comment := map[string]any{"body": "first", "clientCommentId": uuid.NewString()}
	for _, c := range []struct {
		method, path string
		body         any
	}{
		{"POST", "/v2/callouts", callout},
		{"POST", "/v2/bounties", bounty},
		{"POST", "/v2/pools/" + h.pool + "/comments", comment},
		{"POST", "/v2/reports", map[string]any{"targetType": "token", "targetId": h.token, "reason": "spam", "evidence": ""}},
		{"POST", "/v2/chat/read", map[string]any{"conversation_id": "pool:" + h.pool, "through_sequence": "1"}},
	} {
		r := h.expect(h.do(c.method, c.path, "", c.body, nil), 401, "Error")
		if r["code"] != "AUTH_EXPIRED" {
			t.Fatalf("%s: %v", c.path, r)
		}
	}
	if r := h.do("POST", "/v2/social/pool/"+h.pool+"/comments", "", comment, map[string]string{"Idempotency-Key": "k"}); r.status != 401 {
		t.Fatalf("social write without actor: %d %s", r.status, r.body)
	}
	if r := h.do("POST", "/v2/callouts", "not-a-wallet", callout, nil); r.status != 401 {
		t.Fatalf("malformed actor: %d", r.status)
	}
	if r := h.do("GET", "/v2/chat/history?conversation_id=pool:"+h.pool, "", nil, nil); r.status != 401 {
		t.Fatalf("anonymous chat history: %d", r.status)
	}

	// ── callouts ────────────────────────────────────────────────────────────
	created := directData(h.expect(h.do("POST", "/v2/callouts", alice, callout, nil), 201, "CalloutCreateResponse"))["callout"].(map[string]any)
	if created["authorWallet"] != alice || created["poolAddress"] != h.pool || created["revision"] != "1" {
		t.Fatalf("callout = %v", created)
	}
	again := directData(h.expect(h.do("POST", "/v2/callouts", alice, callout, nil), 201, "CalloutCreateResponse"))["callout"].(map[string]any)
	if again["calloutId"] != created["calloutId"] {
		t.Fatalf("retry created a second callout: %v vs %v", again, created)
	}
	changed := map[string]any{"poolAddress": h.pool, "body": "changed", "direction": "bullish", "disclosure": "organic", "clientCalloutId": callout["clientCalloutId"]}
	h.expect(h.do("POST", "/v2/callouts", alice, changed, nil), 409, "Error")
	creatorOnly := map[string]any{"poolAddress": h.pool, "body": "I made this", "direction": "neutral", "disclosure": "creator", "clientCalloutId": uuid.NewString()}
	h.expect(h.do("POST", "/v2/callouts", alice, creatorOnly, nil), 403, "Error")
	h.expect(h.do("POST", "/v2/callouts", h.owner, creatorOnly, nil), 201, "CalloutCreateResponse")
	unknown := map[string]any{"poolAddress": directAddr(0xdead), "body": "x", "direction": "bearish", "disclosure": "organic", "clientCalloutId": uuid.NewString()}
	h.expect(h.do("POST", "/v2/callouts", alice, unknown, nil), 404, "Error")
	h.expect(h.do("POST", "/v2/callouts", alice, map[string]any{"poolAddress": h.pool}, nil), 422, "Error")
	list := h.expect(h.do("GET", "/v2/pools/"+h.pool+"/callouts?limit=1", "", nil, nil), 200, "CalloutListResponse")
	if len(directItems(list)) != 1 || list["nextCursor"] == nil {
		t.Fatalf("page 1 = %s", list)
	}
	list = h.expect(h.do("GET", "/v2/pools/"+h.pool+"/callouts?limit=1&cursor="+list["nextCursor"].(string), "", nil, nil), 200, "CalloutListResponse")
	if len(directItems(list)) != 1 || list["nextCursor"] != nil || directItems(list)[0].(map[string]any)["calloutId"] != created["calloutId"] {
		t.Fatalf("page 2 = %v", list)
	}

	// ── bounties ────────────────────────────────────────────────────────────
	key := map[string]string{"Idempotency-Key": uuid.NewString()}
	b := directData(h.expect(h.do("POST", "/v2/bounties", alice, bounty, key), 201, "BountyCreateResponse"))
	if b["fundingTransactionRequest"] != nil {
		t.Fatalf("funding request without escrow: %v", b)
	}
	made := b["bounty"].(map[string]any)
	if made["state"] != "draft" || made["creatorWallet"] != alice || made["rewardUsdlRaw"] != "5000000" || made["poolAddress"] != h.pool {
		t.Fatalf("bounty = %v", made)
	}
	if again := directData(h.expect(h.do("POST", "/v2/bounties", alice, bounty, key), 201, "BountyCreateResponse"))["bounty"].(map[string]any); again["bountyId"] != made["bountyId"] {
		t.Fatalf("idempotent create returned %v", again)
	}
	other := map[string]any{"title": "Other", "description": "d", "assetAddress": strings.ToLower(deployment.SettlementAddress), "budgetRaw": "1"}
	h.expect(h.do("POST", "/v2/bounties", alice, other, key), 409, "Error")
	badAsset := map[string]any{"title": "t", "description": "d", "assetAddress": directAddr(0x5), "budgetRaw": "1"}
	h.expect(h.do("POST", "/v2/bounties", alice, badAsset, nil), 422, "Error")
	tokenBounty := map[string]any{"title": "Token reward", "description": "Paid in the coin", "assetAddress": h.token, "budgetRaw": "100", "poolAddress": h.pool}
	tb := directData(h.expect(h.do("POST", "/v2/bounties", bob, tokenBounty, nil), 201, "BountyCreateResponse"))["bounty"].(map[string]any)
	if _, ok := tb["rewardUsdlRaw"]; ok {
		t.Fatalf("token bounty claims a USDL value: %v", tb)
	}
	id := made["bountyId"].(string)
	if got := directItems(h.expect(h.do("GET", "/v2/bounties", "", nil, nil), 200, "BountyListResponse")); len(got) != 0 {
		t.Fatalf("drafts listed publicly: %v", got)
	}
	if got := directItems(h.expect(h.do("GET", "/v2/bounties", alice, nil, nil), 200, "BountyListResponse")); len(got) != 1 {
		t.Fatalf("creator list = %v", got)
	}
	h.expect(h.do("GET", "/v2/bounties/"+id, "", nil, nil), 404, "Error")
	h.expect(h.do("GET", "/v2/bounties/"+id, alice, nil, nil), 200, "BountyGetResponse")
	submission := map[string]any{"body": "here is mine", "clientSubmissionId": uuid.NewString(), "evidenceUrls": []string{"https://example.org/meme.png"}}
	h.expect(h.do("POST", "/v2/bounties/"+id+"/submissions", bob, submission, nil), 404, "Error")

	// Funding is an on-chain escrow step chain 125 cannot perform yet; the
	// fixture records a funded bounty directly in the store to exercise the
	// open-bounty paths.
	if _, err := h.db.Exec(context.Background(), `UPDATE chat.v2_bounties SET state='open' WHERE bounty_id=$1`, id); err != nil {
		t.Fatal(err)
	}
	h.expect(h.do("POST", "/v2/bounties/"+id+"/submissions", alice, map[string]any{"body": "self", "clientSubmissionId": uuid.NewString(), "evidenceUrls": []string{"https://example.org/a"}}, nil), 403, "Error")
	sub := directData(h.expect(h.do("POST", "/v2/bounties/"+id+"/submissions", bob, submission, nil), 201, "BountySubmitResponse"))["submission"].(map[string]any)
	repeatedSubmission := directData(h.expect(h.do("POST", "/v2/bounties/"+id+"/submissions", bob, submission, nil), 201, "BountySubmitResponse"))["submission"].(map[string]any)
	if repeatedSubmission["submissionId"] != sub["submissionId"] {
		t.Fatalf("committed submission retry = %v vs %v", repeatedSubmission, sub)
	}
	changedSubmission := map[string]any{"body": "different community entry", "clientSubmissionId": submission["clientSubmissionId"], "evidenceUrls": submission["evidenceUrls"]}
	h.expect(h.do("POST", "/v2/bounties/"+id+"/submissions", bob, changedSubmission, nil), 409, "Error")
	if sub["state"] != "submitted" || sub["awardRaw"] != nil || sub["authorWallet"] != bob {
		t.Fatalf("submission = %v", sub)
	}
	if got := directItems(h.expect(h.do("GET", "/v2/bounties/"+id+"/submissions", "", nil, nil), 200, "BountySubmissionsResponse")); len(got) != 1 {
		t.Fatalf("submissions = %v", got)
	}
	stats = directData(h.expect(h.do("GET", "/v2/bounties/stats", "", nil, nil), 200, "BountiesStatsResponse"))
	if stats["openCount"] != float64(1) || stats["unclaimedPoolUsdlRaw"] != "5000000" || stats["submissionsCount"] != float64(1) {
		t.Fatalf("stats = %v", stats)
	}
	acceptPath := "/v2/bounties/" + id + "/submissions/" + sub["submissionId"].(string) + ":accept"
	h.expect(h.do("POST", acceptPath, bob, map[string]any{"awardRaw": "1", "expectedBountyRevision": "1"}, nil), 403, "Error")
	h.expect(h.do("POST", acceptPath, alice, map[string]any{"awardRaw": "1", "expectedBountyRevision": "7"}, nil), 409, "Error")
	h.expect(h.do("POST", acceptPath, alice, map[string]any{"awardRaw": "5000001", "expectedBountyRevision": "1"}, nil), 422, "Error")
	accepted := directData(h.expect(h.do("POST", acceptPath, alice, map[string]any{"awardRaw": "2000000", "expectedBountyRevision": "1"}, map[string]string{"Idempotency-Key": "accept-1"}), 201, "BountyAcceptResponse"))
	if accepted["settlementRequest"] != nil || accepted["submission"].(map[string]any)["state"] != "accepted" || accepted["submission"].(map[string]any)["awardRaw"] != "2000000" {
		t.Fatalf("accept = %v", accepted)
	}
	h.expect(h.do("POST", acceptPath, alice, map[string]any{"awardRaw": "2000000", "expectedBountyRevision": "1"}, map[string]string{"Idempotency-Key": "accept-1"}), 201, "BountyAcceptResponse")
	got := directData(h.expect(h.do("GET", "/v2/bounties/"+id, "", nil, nil), 200, "BountyGetResponse"))["bounty"].(map[string]any)
	if got["state"] != "reviewing" || got["winnersCount"] != float64(1) {
		t.Fatalf("after accept = %v", got)
	}
	stats = directData(h.expect(h.do("GET", "/v2/bounties/stats", "", nil, nil), 200, "BountiesStatsResponse"))
	if stats["openCount"] != float64(0) || stats["unclaimedPoolUsdlRaw"] != "3000000" {
		t.Fatalf("stats after accept = %v", stats)
	}
	if got := directItems(h.expect(h.do("GET", "/v2/bounties?state=reviewing&poolAddress="+h.pool, "", nil, nil), 200, "BountyListResponse")); len(got) != 1 {
		t.Fatalf("filtered list = %v", got)
	}

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
