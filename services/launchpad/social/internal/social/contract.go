package social

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/db/sqlcdb"
)

// RegisterContractRoutes mounts the frozen v2 follow-graph paths over the
// chat.follows store for the non-connected runtime. IF_FOLLOWERS is not served
// here: its items are profiles, which this runtime has no authority for.
func (h *Handlers) RegisterContractRoutes(r chi.Router) {
	r.Get("/v2/profiles/{wallet}/follow-stats", h.contractFollowStats)
	r.Put("/v2/me/following/{wallet}", h.contractFollow)
	r.Delete("/v2/me/following/{wallet}", h.contractUnfollow)
}

func contractEnvelope(r *http.Request, data any, at time.Time) map[string]any {
	return map[string]any{"requestId": contractRequestID(r), "data": data, "nextCursor": nil, "watermark": nil, revisionField: nil, "quality": "complete", "generatedAt": at.UTC().Format("2006-01-02T15:04:05.000Z")}
}

func contractRequestID(r *http.Request) string {
	id, err := uuid.Parse(r.Header.Get("X-Request-ID"))
	if err != nil || id == uuid.Nil || len(r.Header.Get("X-Request-ID")) != 36 {
		id = uuid.New()
	}
	r.Header.Set("X-Request-ID", id.String())
	return id.String()
}

func (h *Handlers) contractError(w http.ResponseWriter, r *http.Request, status int, message string) {
	code := dependencyUnavailableCode
	switch status {
	case http.StatusBadRequest:
		code = "INVALID_REQUEST"
	case http.StatusUnauthorized:
		code = "AUTH_REQUIRED"
	case http.StatusForbidden:
		code = "SCOPE_DENIED"
	case http.StatusNotFound:
		code = "NOT_FOUND"
	}
	id := contractRequestID(r)
	raw, err := json.Marshal(v2.Error{Code: code, Message: message, RequestId: id, Retryable: status == http.StatusServiceUnavailable, FieldErrors: []v2.FieldError{}})
	if err == nil {
		err = v2.ValidateJSON("Error", raw)
	}
	if err != nil {
		h.logErr("encode frozen follow error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("X-Request-ID", id)
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Add("Vary", "Cookie")
	w.Header().Add("Vary", "Authorization")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(raw)
}

func (h *Handlers) contractFailure(w http.ResponseWriter, r *http.Request, op string, err error) {
	h.logErr(op, err)
	status := http.StatusServiceUnavailable
	if errors.Is(err, pgx.ErrNoRows) {
		status = http.StatusNotFound
	}
	h.contractError(w, r, status, http.StatusText(status))
}

func (h *Handlers) writeContract(w http.ResponseWriter, r *http.Request, data any) {
	w.Header().Set("X-Request-ID", contractRequestID(r))
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Add("Vary", "Cookie")
	w.Header().Add("Vary", "Authorization")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(contractEnvelope(r, data, h.clock()))
}

func (h *Handlers) contractFollowStats(w http.ResponseWriter, r *http.Request) {
	wallet := normalizeAddr(chi.URLParam(r, "wallet"))
	if !addrRe.MatchString(wallet) {
		h.contractError(w, r, http.StatusBadRequest, "invalid wallet address")
		return
	}
	if h.q == nil {
		h.contractError(w, r, http.StatusServiceUnavailable, "Follow authority unavailable")
		return
	}
	followers, err := h.q.CountFollowers(r.Context(), wallet)
	if err != nil {
		h.contractFailure(w, r, "count followers", err)
		return
	}
	following, err := h.q.CountFollowing(r.Context(), wallet)
	if err != nil {
		h.contractFailure(w, r, "count following", err)
		return
	}
	isFollowing := false
	if act, ok := actor(r); ok && act != wallet {
		isFollowing, err = h.q.IsFollowing(r.Context(), sqlcdb.IsFollowingParams{Follower: act, Followee: wallet})
		if err != nil {
			h.contractFailure(w, r, "is following", err)
			return
		}
	}
	h.writeContract(w, r, v2.FollowStatsData{Followers: v2.SafeInteger(followers), Following: v2.SafeInteger(following), IsFollowing: isFollowing})
}

func (h *Handlers) contractFollow(w http.ResponseWriter, r *http.Request) {
	act, ok := actor(r)
	if !ok {
		h.contractError(w, r, http.StatusUnauthorized, "missing or invalid "+actorHeader)
		return
	}
	followee := normalizeAddr(chi.URLParam(r, "wallet"))
	if !addrRe.MatchString(followee) {
		h.contractError(w, r, http.StatusBadRequest, "invalid wallet address")
		return
	}
	if followee == act {
		h.contractError(w, r, http.StatusBadRequest, "Cannot follow yourself")
		return
	}
	if h.q == nil {
		h.contractError(w, r, http.StatusServiceUnavailable, "Follow authority unavailable")
		return
	}
	if err := h.q.Follow(r.Context(), sqlcdb.FollowParams{Follower: act, Followee: followee, CreatedAt: h.clock().Unix()}); err != nil {
		h.contractFailure(w, r, "follow", err)
		return
	}
	h.writeContract(w, r, v2.FollowData{Following: true})
}

func (h *Handlers) contractUnfollow(w http.ResponseWriter, r *http.Request) {
	act, ok := actor(r)
	if !ok {
		h.contractError(w, r, http.StatusUnauthorized, "missing or invalid "+actorHeader)
		return
	}
	followee := normalizeAddr(chi.URLParam(r, "wallet"))
	if !addrRe.MatchString(followee) {
		h.contractError(w, r, http.StatusBadRequest, "invalid wallet address")
		return
	}
	if h.q == nil {
		h.contractError(w, r, http.StatusServiceUnavailable, "Follow authority unavailable")
		return
	}
	if err := h.q.Unfollow(r.Context(), sqlcdb.UnfollowParams{Follower: act, Followee: followee}); err != nil {
		h.contractFailure(w, r, "unfollow", err)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Request-ID", contractRequestID(r))
	w.WriteHeader(http.StatusNoContent)
}
