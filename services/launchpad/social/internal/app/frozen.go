package app

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/common"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/devsocial"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/interactions"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/safety"
	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	goredis "github.com/redis/go-redis/v9"
)

// The frozen adapters share the bound chat cursor and request principal. Only
// the callout/bounty routes are delegated; connected chat and interactions
// retain their bound stores, policy, outbox and notification authority.
func (r *socialConnectedRuntime) prepareFrozen(ctx context.Context, redis *goredis.Client, logger *slog.Logger) error {
	rt, err := devsocial.New(ctx, devsocial.Deps{Pool: r.pool, Redis: redis, Logger: logger, Authenticate: contextPrincipal, CursorKey: r.chat.CursorKey, Pools: devsocial.CanonicalPools})
	if err != nil {
		return err
	}
	rt.Chat = r.chat
	r.frozen = rt
	return nil
}

type frozenRoute struct{ method, path string }

var frozenStoreRoutes = []frozenRoute{
	{getMethod, "/v2/pools/{poolAddress}/callouts"},
	{getMethod, "/v2/bounties"}, {postMethod, "/v2/bounties"}, {getMethod, "/v2/bounties/stats"},
	{getMethod, "/v2/bounties/{bountyId}"}, {getMethod, "/v2/bounties/{bountyId}/submissions"},
	{postMethod, "/v2/bounties/{bountyId}/submissions"}, {postMethod, "/v2/bounties/{bountyId}/submissions/{submissionId}:accept"},
}

func (r *socialConnectedRuntime) registerFrozen(mux chi.Router) error {
	routes := chi.NewRouter()
	if err := r.frozen.Register(routes); err != nil {
		return err
	}
	for _, route := range frozenStoreRoutes {
		mux.Method(route.method, route.path, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			// A separately registered router needs its own route context; retaining
			// the outer chi context would skip matching and lose its path parameters.
			ctx := context.WithValue(req.Context(), chi.RouteCtxKey, chi.NewRouteContext())
			routes.ServeHTTP(w, req.WithContext(ctx))
		}))
	}
	mux.Post("/v2/callouts", r.createFrozenCallout)
	comments := &socialComments{pool: r.pool, store: r.store}
	mux.Post("/v2/pools/{poolAddress}/comments", comments.create)
	mux.Patch("/v2/comments/{commentId}", comments.edit)
	mux.Delete("/v2/comments/{commentId}", comments.remove)
	mux.Put("/v2/comments/{commentId}/likes/me", comments.like)
	mux.Delete("/v2/comments/{commentId}/likes/me", comments.like)
	mux.Group(func(g chi.Router) { g.Use(r.safetyMiddleware); g.Post("/v2/reports", r.createFrozenReport) })
	return nil
}

func frozenPublicRead(r *http.Request) bool {
	if r == nil || r.URL == nil || r.Method != http.MethodGet {
		return false
	}
	p := strings.Split(r.URL.Path, "/")
	if len(p) == 5 && p[1] == "v2" && p[2] == "pools" && common.IsAddr(p[3]) && (p[4] == commentsResource || p[4] == "callouts") {
		return true
	}
	if len(p) == 3 && p[1] == "v2" && p[2] == bountiesResource {
		return true
	}
	if len(p) == 4 && p[1] == "v2" && p[2] == bountiesResource {
		if p[3] == "stats" {
			return true
		}
		_, e := uuid.Parse(p[3])
		return e == nil
	}
	if len(p) == 5 && p[1] == "v2" && p[2] == bountiesResource && p[4] == "submissions" {
		_, e := uuid.Parse(p[3])
		return e == nil
	}
	// The interaction comment list is the second view of these same rows.
	return len(p) == 6 && p[1] == "v2" && p[2] == socialService && p[3] == poolResource && common.IsAddr(p[4]) && p[5] == commentsResource
}

func frozenError(w http.ResponseWriter, req *http.Request, err error) {
	status, code := 503, dependencyUnavailableCode
	switch {
	case errors.Is(err, interactions.ErrInvalid), errors.Is(err, safety.ErrInvalid):
		status, code = 422, "FIELD_INVALID"
	case errors.Is(err, interactions.ErrDenied), errors.Is(err, safety.ErrForbidden):
		status, code = 403, scopeDeniedCode
	case errors.Is(err, interactions.ErrRevision), errors.Is(err, safety.ErrConflict):
		status, code = 409, "REVISION_CONFLICT"
	case errors.Is(err, interactions.ErrDepth):
		status, code = 422, "THREAD_DEPTH_EXCEEDED"
	case errors.Is(err, pgx.ErrNoRows):
		status, code = 404, notFoundCode
	}
	frozenWriteError(w, req, status, code)
}
func frozenWriteError(w http.ResponseWriter, req *http.Request, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v2.Error{Code: code, Message: "Social request rejected", RequestId: frozenRequestID(req), Retryable: status == 503, FieldErrors: []v2.FieldError{}})
}
func frozenRequestID(req *http.Request) string {
	if id, e := uuid.Parse(req.Header.Get("X-Request-ID")); e == nil && id != uuid.Nil {
		return id.String()
	}
	return uuid.NewString()
}
func frozenData(w http.ResponseWriter, req *http.Request, status int, schema string, data any, next *string) {
	raw, err := json.Marshal(map[string]any{"requestId": frozenRequestID(req), "data": data, "nextCursor": next, "watermark": nil, "revision": nil, "quality": "complete", "generatedAt": time.Now().UTC().Format("2006-01-02T15:04:05.000Z")})
	if err == nil && schema != "" {
		err = v2.ValidateJSON(schema, raw)
	}
	if err != nil {
		frozenWriteError(w, req, 503, "INTEGRITY_QUARANTINED")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Add("Vary", "Authorization")
	w.Header().Add("Vary", "Cookie")
	w.WriteHeader(status)
	_, _ = w.Write(raw)
}
func frozenDecode(w http.ResponseWriter, req *http.Request, schema string, value any) bool {
	raw, err := io.ReadAll(http.MaxBytesReader(w, req.Body, 64<<10))
	if err == nil {
		err = v2.ValidateJSON(schema, raw)
	}
	if err == nil {
		err = json.Unmarshal(raw, value)
	}
	if err != nil {
		frozenWriteError(w, req, 422, "FIELD_INVALID")
		return false
	}
	return true
}
func frozenKey(req *http.Request) (string, error) {
	key := req.Header.Get("Idempotency-Key")
	if key == "" {
		key = "request:" + frozenRequestID(req)
	}
	if len(key) > 128 {
		return "", interactions.ErrInvalid
	}
	for _, c := range key {
		if c < 0x20 || c == 0x7f {
			return "", interactions.ErrInvalid
		}
	}
	return key, nil
}
func frozenActor(w http.ResponseWriter, req *http.Request) (string, bool) {
	p, err := contextPrincipal(req)
	if err != nil {
		frozenWriteError(w, req, 401, "AUTH_EXPIRED")
		return "", false
	}
	return p.Wallet, true
}
func frozenComment(v interactions.CommentView) v2.Comment {
	return v2.Comment{CommentId: v.ID, PoolAddress: v.Pool, AuthorWallet: v.Author, ParentCommentId: v.Parent, Body: v.Body, LikeCount: v2.UInt256(v.Likes), Revision: v2.UInt256(v.Revision)}
}
func (c *socialComments) create(w http.ResponseWriter, req *http.Request) {
	actor, ok := frozenActor(w, req)
	if !ok {
		return
	}
	var body v2.CommentCreateBody
	if !frozenDecode(w, req, "CommentCreateBody", &body) {
		return
	}
	client, err := uuid.Parse(body.ClientCommentId)
	if err != nil {
		frozenError(w, req, interactions.ErrInvalid)
		return
	}
	in := interactions.Request{Actor: actor, Pool: common.NormalizeAddr(chi.URLParam(req, "poolAddress")), Key: "comment:" + client.String(), Body: body.Body}
	if body.ParentCommentId.Present {
		in.Parent = strings.ToLower(body.ParentCommentId.Value)
	}
	if err = c.store.CreateComment(req.Context(), in); err != nil {
		frozenError(w, req, err)
		return
	}
	var raw []byte
	if err = c.pool.QueryRow(req.Context(), `SELECT payload FROM chat.interaction_mutations WHERE actor=$1 AND key=$2`, actor, in.Key).Scan(&raw); err != nil {
		frozenError(w, req, err)
		return
	}
	var saved interactions.Request
	if err = json.Unmarshal(raw, &saved); err != nil || saved.ID == "" {
		frozenError(w, req, interactions.ErrDependency)
		return
	}
	view, err := c.byID(req.Context(), saved.ID)
	if err != nil {
		frozenError(w, req, err)
		return
	}
	frozenData(w, req, 201, "CommentCreateResponse", v2.CommentCreateData{Comment: frozenComment(view)}, nil)
}
func (c *socialComments) edit(w http.ResponseWriter, req *http.Request) {
	actor, ok := frozenActor(w, req)
	if !ok {
		return
	}
	var body v2.CommentEditBody
	if !frozenDecode(w, req, "CommentEditBody", &body) {
		return
	}
	revision, err := strconv.ParseInt(string(body.ExpectedRevision), 10, 64)
	if err != nil {
		frozenError(w, req, interactions.ErrInvalid)
		return
	}
	key, err := frozenKey(req)
	if err != nil {
		frozenError(w, req, err)
		return
	}
	id := chi.URLParam(req, "commentId")
	if _, err = c.byID(req.Context(), id); err == nil {
		err = c.store.EditComment(req.Context(), interactions.Request{Actor: actor, ID: id, Key: key, Body: body.Body, Revision: revision})
	}
	if err != nil {
		frozenError(w, req, err)
		return
	}
	view, err := c.byID(req.Context(), id)
	if err != nil {
		frozenError(w, req, err)
		return
	}
	frozenData(w, req, 200, "CommentEditResponse", v2.CommentEditData{Comment: frozenComment(view)}, nil)
}
func (c *socialComments) remove(w http.ResponseWriter, req *http.Request) {
	actor, ok := frozenActor(w, req)
	if !ok {
		return
	}
	id := chi.URLParam(req, "commentId")
	var revision int64
	err := c.pool.QueryRow(req.Context(), `SELECT revision FROM chat.interaction_comments WHERE id=$1 AND NOT removed`, id).Scan(&revision)
	if err == nil {
		var key string
		key, err = frozenKey(req)
		if err == nil {
			err = c.store.EditComment(req.Context(), interactions.Request{Actor: actor, ID: id, Key: key, Revision: revision, Remove: true})
		}
	}
	if err != nil {
		frozenError(w, req, err)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(204)
}
func (c *socialComments) like(w http.ResponseWriter, req *http.Request) {
	actor, ok := frozenActor(w, req)
	if !ok {
		return
	}
	id := chi.URLParam(req, "commentId")
	if _, err := c.byID(req.Context(), id); err != nil {
		frozenError(w, req, err)
		return
	}
	key, err := frozenKey(req)
	if err != nil {
		frozenError(w, req, err)
		return
	}
	remove := req.Method == http.MethodDelete
	if err = c.store.React(req.Context(), interactions.Request{Actor: actor, ID: id, Key: key, Kind: "like", Remove: remove}); err != nil {
		frozenError(w, req, err)
		return
	}
	if remove {
		w.Header().Set("Cache-Control", "private, no-store")
		w.WriteHeader(204)
		return
	}
	n, err := c.store.Count(req.Context(), id, "like")
	if err != nil {
		frozenError(w, req, err)
		return
	}
	frozenData(w, req, 200, "CommentLikeResponse", v2.CommentLikeData{LikeCount: v2.UInt256(strconv.FormatInt(n, 10)), Liked: true}, nil)
}

func (r *socialConnectedRuntime) createFrozenReport(w http.ResponseWriter, req *http.Request) {
	actor, ok := frozenActor(w, req)
	if !ok {
		return
	}
	var body v2.ReportCreateBody
	if !frozenDecode(w, req, "ReportCreateBody", &body) {
		return
	}
	report, err := r.safety.Report(req.Context(), safety.Report{TargetType: body.TargetType, TargetID: body.TargetId, Category: body.Reason, Description: body.Evidence})
	if err != nil {
		frozenError(w, req, err)
		return
	}
	var category, state string
	if err = r.pool.QueryRow(req.Context(), `SELECT category,state FROM chat.safety_cases WHERE id=$1`, report.ID).Scan(&category, &state); err != nil {
		frozenError(w, req, err)
		return
	}
	report.State = state
	out, err := frozenReport(report, actor, category)
	if err != nil {
		frozenError(w, req, err)
		return
	}
	frozenData(w, req, 201, "ReportCreateResponse", v2.ReportCreateData{Report: out}, nil)
}
func frozenReport(report safety.Case, actor, category string) (v2.Report, error) {
	state := ""
	switch report.State {
	case "open":
		state = "open"
	case "triaged", "appealed":
		state = "reviewing"
	case "actioned":
		state = "resolved"
	case "dismissed":
		state = "dismissed"
	default:
		return v2.Report{}, interactions.ErrDependency
	}
	reason := strings.ToLower(strings.TrimSpace(category))
	switch reason {
	case "spam", "abuse", "fraud", "copyright", "other":
	default:
		reason = "other"
	}
	return v2.Report{ReportId: report.ID, ReporterWallet: actor, TargetType: report.TargetType, TargetId: report.TargetID, Reason: reason, State: state}, nil
}

func (r *socialConnectedRuntime) createFrozenCallout(w http.ResponseWriter, req *http.Request) {
	actor, ok := frozenActor(w, req)
	if !ok {
		return
	}
	var body v2.CalloutCreateBody
	if !frozenDecode(w, req, "CalloutCreateBody", &body) {
		return
	}
	client, err := uuid.Parse(body.ClientCalloutId)
	if err != nil || interactions.Text(body.Body) != nil {
		frozenError(w, req, interactions.ErrInvalid)
		return
	}
	tx, err := r.pool.Begin(req.Context())
	if err != nil {
		frozenError(w, req, err)
		return
	}
	defer rollbackTx(req.Context(), tx)
	ctx := req.Context()
	pool := strings.ToLower(body.PoolAddress)
	// Use the bound account/session, canonical-pool and safety authorities in
	// the same transaction that records the callout and its reference authority.
	err = r.store.Authority.Check(ctx, tx, interactions.Request{Actor: actor, Pool: pool, Action: commentAction})
	if err == nil {
		_, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "callout:"+actor+":"+client.String())
	}
	var out v2.Callout
	var revision int64
	var at time.Time
	scan := func(row pgx.Row) error {
		return row.Scan(&out.CalloutId, &out.AuthorWallet, &out.PoolAddress, &out.Body, &out.Direction, &out.Disclosure, &revision, &at)
	}
	const columns = `callout_id::text,author,pool,body,direction,disclosure,revision,created_at`
	if err == nil {
		err = scan(tx.QueryRow(ctx, `SELECT `+columns+` FROM chat.v2_callouts WHERE author=$1 AND client_callout_id=$2`, actor, client))
		if err == nil && (out.PoolAddress != pool || out.Body != body.Body || out.Direction != body.Direction || out.Disclosure != body.Disclosure) {
			err = interactions.ErrRevision
		}
		if errors.Is(err, pgx.ErrNoRows) {
			var creator string
			err = tx.QueryRow(ctx, `SELECT chat.interaction_pool_creator($1::text)`, pool).Scan(&creator)
			if err == nil && body.Disclosure == "creator" && creator != actor {
				err = interactions.ErrDenied
			}
			if err == nil {
				err = scan(tx.QueryRow(ctx, `INSERT INTO chat.v2_callouts(callout_id,author,client_callout_id,pool,body,direction,disclosure) VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING `+columns, uuid.New(), actor, client, pool, body.Body, body.Direction, body.Disclosure))
			}
		}
	}
	if err == nil {
		err = interactions.ApplyCalloutAuthorityTx(ctx, tx, out.CalloutId, revision, actor, false)
	}
	if err == nil {
		err = tx.Commit(ctx)
	}
	if err != nil {
		frozenError(w, req, err)
		return
	}
	out.Revision = v2.UInt256(strconv.FormatInt(revision, 10))
	out.CreatedAt = at.UTC().Format("2006-01-02T15:04:05.000Z")
	frozenData(w, req, 201, "CalloutCreateResponse", v2.CalloutCreateData{Callout: out}, nil)
}
