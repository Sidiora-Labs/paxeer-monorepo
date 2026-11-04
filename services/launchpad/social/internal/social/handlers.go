// Package social implements the media/social HTTP handlers: pool chat history +
// edit/delete, DM conversation reads, the threaded comments feed, the followers
// graph, and the moderation (bans/reports) admin surface. Realtime sends (pool
// messages + DMs) are handled by the internal/fanout WS hub; this package owns
// the REST surface.
//
// Identity is sign-free: writes read the actor wallet from the trusted
// X-Actor-Wallet header that media/gateway injects after a one-time auth. Admin
// routes are gated by CHAT_ADMIN_API_KEY.
package social

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/fanout"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/interactions"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/safety"
	sharedhttp "github.com/Sidiora-Technologies/KindleLaunch/shared/http"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/db/sqlcdb"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/pkg/types"
)

const (
	// defaultPageLimit / maxPageLimit bound history pagination (parity with TS).
	defaultPageLimit int32 = 50
	maxPageLimit     int32 = 100
	// bodyLimit bounds JSON request bodies.
	bodyLimit = 1 << 20
	// editMaxLen caps message/comment edits (parity with the TS 2000-char cap).
	editMaxLen = 2000
)

// Deps are the handler dependencies.
type Deps struct {
	// Durable and AuthenticateChat bind legacy chat URLs to the same store and
	// verified session authority as /ws. Missing bindings refuse chat requests.
	Durable          *fanout.DurableStore
	AuthenticateChat fanout.AuthenticateUpgrade
	Queries          sqlcdb.Querier
	MaxMessageLength int
	AdminAPIKey      string
	Logger           *slog.Logger
	// Clock defaults to time.Now.
	Clock func() time.Time
	// PoolCreator resolves a pool's creator for creator-authorized deletes.
	// Nil reads indexer.pools through Queries.GetPoolCreator.
	PoolCreator func(ctx context.Context, pool string) (string, error)
}

// Handlers serves the social REST routes.
type Handlers struct {
	chat             *fanout.DurableStore
	authenticateChat fanout.AuthenticateUpgrade
	q                sqlcdb.Querier
	maxMsgLen        int
	adminKey         string
	logger           *slog.Logger
	clock            func() time.Time
	creator          func(ctx context.Context, pool string) (string, error)
}

// New constructs Handlers, applying defaults for optional dependencies.
func New(d Deps) *Handlers {
	clock := d.Clock
	if clock == nil {
		clock = time.Now
	}
	maxLen := d.MaxMessageLength
	if maxLen <= 0 {
		maxLen = 500
	}
	creator := d.PoolCreator
	if creator == nil && d.Queries != nil {
		creator = d.Queries.GetPoolCreator
	}
	return &Handlers{
		chat:             d.Durable,
		authenticateChat: d.AuthenticateChat,
		q:                d.Queries,
		maxMsgLen:        maxLen,
		adminKey:         d.AdminAPIKey,
		logger:           d.Logger,
		clock:            clock,
		creator:          creator,
	}
}

// RegisterRoutes mounts the social endpoints onto r.
func (h *Handlers) RegisterRoutes(r chi.Router) {
	// Pool chat history + moderation of own messages.
	r.Get("/pool/{poolAddress}/messages", h.listMessages)
	r.Patch("/pool/{poolAddress}/messages/{id}", h.editMessage)
	r.Delete("/pool/{poolAddress}/messages/{id}", h.deleteMessage)

	// Comments feed.
	r.Get("/pool/{poolAddress}/comments", h.listComments)
	r.Post("/pool/{poolAddress}/comments", h.createComment)
	r.Patch("/comments/{id}", h.editComment)
	r.Delete("/comments/{id}", h.deleteComment)
	r.Put("/comments/{id}/like", h.likeComment)
	r.Delete("/comments/{id}/like", h.unlikeComment)

	// Direct messages (reads; sends happen over WS).
	r.Get("/dm/conversations", h.listConversations)
	r.Get("/dm/conversations/{id}/messages", h.listDmMessages)

	// Followers graph.
	r.Put("/follow/{followee}", h.follow)
	r.Delete("/follow/{followee}", h.unfollow)
	r.Get("/users/{wallet}/followers", h.listFollowers)
	r.Get("/users/{wallet}/following", h.listFollowing)
	r.Get("/users/{wallet}/follow-stats", h.followStats)

	// Moderation: public report + admin-gated routes.
	r.Post("/report", h.report)
	r.Group(func(gr chi.Router) {
		gr.Use(h.adminAuth)
		gr.Delete("/admin/messages/{messageId}", h.adminDeleteMessage)
		gr.Post("/admin/bans", h.adminCreateBan)
		gr.Delete("/admin/bans/{banId}", h.adminDeleteBan)
		gr.Get("/admin/bans", h.adminListBans)
		gr.Get("/admin/reports", h.adminListReports)
		gr.Patch("/admin/reports/{reportId}", h.adminResolveReport)
	})
}

// ── Pool chat ─────────────────────────────────────────────────────────────────

func (h *Handlers) chatRequest(w http.ResponseWriter, r *http.Request) (*http.Request, string, bool) {
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Add("Vary", "Cookie")
	w.Header().Add("Vary", "Authorization")
	if h.chat == nil || h.chat.Pool == nil || h.chat.Policy == nil || h.chat.ReadPolicy == nil || h.authenticateChat == nil {
		h.chatError(w, fanout.ErrUnavailable, fanout.HistoryPage{})
		return r, "", false
	}
	principal, err := h.authenticateChat(r)
	if err != nil {
		sharedhttp.WriteError(w, http.StatusUnauthorized, "Unauthorized", "Chat authentication required")
		return r, "", false
	}
	ctx := fanout.WithPrincipal(r.Context(), principal)
	if _, ok := fanout.PrincipalFromContext(ctx); !ok {
		sharedhttp.WriteError(w, http.StatusUnauthorized, "Unauthorized", "Chat authentication required")
		return r, "", false
	}
	return r.WithContext(ctx), principal.Wallet, true
}

func (h *Handlers) chatError(w http.ResponseWriter, err error, page fanout.HistoryPage) {
	w.Header().Set("Cache-Control", "private, no-store")
	status, code := http.StatusServiceUnavailable, "chat_unavailable"
	switch {
	case errors.Is(err, fanout.ErrNotFound), errors.Is(err, pgx.ErrNoRows):
		status, code = http.StatusNotFound, "conversation not found"
	case errors.Is(err, interactions.ErrDenied), errors.Is(err, safety.ErrForbidden):
		status, code = http.StatusForbidden, "chat access denied"
	case errors.Is(err, fanout.ErrConflict):
		status, code = http.StatusConflict, "revision conflict"
	case errors.Is(err, fanout.ErrDeleted):
		status, code = http.StatusGone, "message deleted"
	case errors.Is(err, fanout.ErrInvalid), errors.Is(err, fanout.ErrCursor):
		status, code = http.StatusBadRequest, "invalid chat input"
	case errors.Is(err, fanout.ErrReplayExpired), errors.Is(err, fanout.ErrSnapshotRequired):
		status, code = http.StatusGone, err.Error()
	default:
		h.logErr("durable chat", err)
	}
	sharedhttp.WriteJSON(w, status, map[string]any{"error": code, "next_cursor": page.Next, "reset_required": page.Reset})
}

func (h *Handlers) chatHistory(w http.ResponseWriter, r *http.Request, conversation string) {
	r, act, ok := h.chatRequest(w, r)
	if !ok {
		return
	}
	if r.URL.Query().Get("before") != "" {
		h.chatError(w, fanout.ErrCursor, fanout.HistoryPage{})
		return
	}
	limit := 50
	if raw := r.URL.Query().Get("limit"); raw != "" {
		var err error
		limit, err = strconv.Atoi(raw)
		if err != nil || limit < 1 || limit > 100 {
			h.chatError(w, fanout.ErrInvalid, fanout.HistoryPage{})
			return
		}
	}
	page, err := h.chat.History(r.Context(), act, conversation, r.URL.Query().Get("cursor"), limit)
	if err != nil {
		h.chatError(w, err, page)
		return
	}
	sharedhttp.WriteJSON(w, http.StatusOK, map[string]any{"messages": page.Messages, "has_more": len(page.Messages) == limit, "next_cursor": page.Next, "reset_required": page.Reset, "read_through_sequence": strconv.FormatInt(page.ReadThrough, 10)})
}

func (h *Handlers) listMessages(w http.ResponseWriter, r *http.Request) {
	pool := normalizeAddr(chi.URLParam(r, "poolAddress"))
	if !addrRe.MatchString(pool) {
		h.chatError(w, fanout.ErrInvalid, fanout.HistoryPage{})
		return
	}
	h.chatHistory(w, r, "pool:"+pool)
}

func (h *Handlers) editMessage(w http.ResponseWriter, r *http.Request) {
	h.reviseChatMessage(w, r, false)
}

func (h *Handlers) deleteMessage(w http.ResponseWriter, r *http.Request) {
	h.reviseChatMessage(w, r, true)
}

func (h *Handlers) reviseChatMessage(w http.ResponseWriter, r *http.Request, remove bool) {
	pool := normalizeAddr(chi.URLParam(r, "poolAddress"))
	if !addrRe.MatchString(pool) {
		h.chatError(w, fanout.ErrInvalid, fanout.HistoryPage{})
		return
	}
	r, act, ok := h.chatRequest(w, r)
	if !ok {
		return
	}
	raw := r.Header.Get("If-Match")
	if len(raw) >= 2 && raw[0] == '"' && raw[len(raw)-1] == '"' {
		raw = raw[1 : len(raw)-1]
	}
	revision, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || revision < 1 {
		h.chatError(w, fanout.ErrInvalid, fanout.HistoryPage{})
		return
	}
	conv, id := "pool:"+pool, chi.URLParam(r, "id")
	if err = h.chat.Authorized(r.Context(), act, conv); err != nil {
		h.chatError(w, err, fanout.HistoryPage{})
		return
	}
	var exists bool
	if err = h.chat.Pool.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM chat_messages WHERE id=$1 AND conversation_id=$2)`, id, conv).Scan(&exists); err != nil {
		h.chatError(w, err, fanout.HistoryPage{})
		return
	}
	if !exists {
		h.chatError(w, fanout.ErrNotFound, fanout.HistoryPage{})
		return
	}
	content := ""
	if !remove {
		var body types.EditRequest
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, bodyLimit))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&body) != nil || decoder.Decode(&struct{}{}) != io.EOF {
			h.chatError(w, fanout.ErrInvalid, fanout.HistoryPage{})
			return
		}
		content = body.Content
	}
	message, err := h.chat.Revise(r.Context(), act, id, revision, content, remove)
	if err != nil {
		h.chatError(w, err, fanout.HistoryPage{})
		return
	}
	w.Header().Set("ETag", `"`+strconv.FormatInt(message.Revision, 10)+`"`)
	sharedhttp.WriteJSON(w, http.StatusOK, map[string]any{"success": true, "message": message})
}

// ── Direct messages (reads) ───────────────────────────────────────────────────

func (h *Handlers) listConversations(w http.ResponseWriter, r *http.Request) {
	r, act, ok := h.chatRequest(w, r)
	if !ok {
		return
	}
	ids, err := h.chat.List(r.Context(), act, 100)
	if err != nil {
		h.chatError(w, err, fanout.HistoryPage{})
		return
	}
	out := make([]types.DmConversationSummary, 0, len(ids))
	for _, id := range ids {
		if err = h.chat.Authorized(r.Context(), act, id); err != nil {
			h.chatError(w, err, fanout.HistoryPage{})
			return
		}
		var summary types.DmConversationSummary
		summary.ID = id
		err = h.chat.Pool.QueryRow(r.Context(), `SELECT peer.wallet,extract(epoch from max(msg.created_at))::bigint FROM chat_members own JOIN chat_members peer ON peer.conversation_id=own.conversation_id AND peer.wallet<>own.wallet LEFT JOIN chat_messages msg ON msg.conversation_id=own.conversation_id WHERE own.conversation_id=$1 AND own.wallet=$2 GROUP BY peer.wallet`, id, act).Scan(&summary.Peer, &summary.LastMessageAt)
		if err != nil {
			h.chatError(w, err, fanout.HistoryPage{})
			return
		}
		out = append(out, summary)
	}
	sharedhttp.WriteJSON(w, http.StatusOK, types.DmConversationsResponse{Conversations: out})
}

func (h *Handlers) listDmMessages(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if strings.HasPrefix(id, "pool:") {
		h.chatError(w, fanout.ErrNotFound, fanout.HistoryPage{})
		return
	}
	h.chatHistory(w, r, id)
}

// ── shared helpers ────────────────────────────────────────────────────────────

// decodeEdit decodes + validates an EditRequest body, writing the error response
// itself and returning ok=false on failure.
func (h *Handlers) decodeEdit(w http.ResponseWriter, r *http.Request) (content string, ok bool) {
	var req types.EditRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, bodyLimit)).Decode(&req); err != nil {
		sharedhttp.WriteError(w, http.StatusBadRequest, "Bad Request", "invalid JSON body")
		return "", false
	}
	trimmed := strings.TrimSpace(req.Content)
	if trimmed == "" {
		sharedhttp.WriteError(w, http.StatusBadRequest, "Bad Request", "content is required")
		return "", false
	}
	if len(trimmed) > editMaxLen {
		sharedhttp.WriteError(w, http.StatusBadRequest, "Bad Request", "content too long")
		return "", false
	}
	return trimmed, true
}

// isPoolCreator reports whether wallet is the creator of pool (cross-schema read
// of the indexer, Deps.PoolCreator). A missing pool row yields false (not an
// error to the caller).
func (h *Handlers) isPoolCreator(ctx context.Context, pool, wallet string) bool {
	if h.creator == nil {
		return false
	}
	creator, err := h.creator(ctx, pool)
	if err != nil {
		return false
	}
	return normalizeAddr(creator) == wallet
}

// isBanned reports whether wallet currently has an active ban (global or for the
// given pool, when non-empty). Unavailable authority denies writes.
func (h *Handlers) isBanned(ctx context.Context, wallet, pool string) bool {
	banned, e := h.checkedBan(ctx, wallet, pool)
	return e != nil || banned
}

func (h *Handlers) checkedBan(ctx context.Context, wallet, pool string) (bool, error) {
	if store, _ := ctx.Value(moderationKey{}).(*safety.Store); store != nil {
		p, e := safety.Actor(ctx, h.clock())
		if e != nil {
			return false, e
		}
		if p.Wallet != wallet {
			return false, safety.ErrForbidden
		}
		scope, resource := "global", ""
		if pool != "" {
			scope, resource = "pool", pool
		}
		decision := store.Check(ctx, p.Wallet, scope, resource)
		if decision == safety.Unavailable {
			return false, safety.ErrUnavailable
		}
		if decision == safety.Deny {
			return true, nil
		}
		if decision != safety.Allow {
			return false, safety.ErrUnavailable
		}
		return false, nil
	}
	if h.q == nil {
		return false, safety.ErrUnavailable
	}
	var poolArg *string
	if pool != "" {
		poolArg = &pool
	}
	bans, e := h.q.ActiveBans(ctx, sqlcdb.ActiveBansParams{Wallet: wallet, Now: int8Value(h.clock().Unix()), PoolAddress: poolArg})
	if e != nil {
		h.logErr("active bans", e)
		return false, safety.ErrUnavailable
	}
	return len(bans) > 0, nil
}

// adminAuth gates /admin/* routes with a constant-time API-key comparison
// (parity with the TS timingSafeEqual check).
func (h *Handlers) adminAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if h.adminKey == "" {
			sharedhttp.WriteError(w, http.StatusServiceUnavailable, "Service Unavailable", "Admin API not configured")
			return
		}
		provided := r.Header.Get("X-API-Key")
		if provided == "" {
			sharedhttp.WriteError(w, http.StatusUnauthorized, "Unauthorized", "Missing API key")
			return
		}
		if subtle.ConstantTimeEq(int32(len(provided)), int32(len(h.adminKey))) != 1 ||
			subtle.ConstantTimeCompare([]byte(provided), []byte(h.adminKey)) != 1 {
			sharedhttp.WriteError(w, http.StatusForbidden, "Forbidden", "Invalid API key")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Handlers) fail(w http.ResponseWriter, op string, err error) {
	h.logErr(op, err)
	sharedhttp.WriteError(w, http.StatusInternalServerError, "Internal Server Error", "")
}

func (h *Handlers) logErr(op string, err error) {
	if h.logger != nil {
		h.logger.Error("social handler error", slog.String("op", op), slog.String("err", err.Error()))
	}
}
