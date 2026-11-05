package app

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/common"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/interactions"
	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type socialComments struct {
	pool  *pgxpool.Pool
	store interactions.Store
}

// Read the source rows and moderation overlay in one snapshot. Neither the
// worker's progress nor a previously cached ETag can make removed content
// visible. An author tombstone and an erasure always take precedence over an
// appealed moderation restoration.
const visibleCommentColumns = `c.id,c.pool,c.author,c.parent,c.body,
 (SELECT count(*) FROM chat.interaction_reactions r WHERE r.target=c.id AND NOT EXISTS(SELECT 1 FROM chat.interaction_erased_wallets e WHERE e.wallet=r.actor))::text,
 c.revision::text,c.edited_at IS NOT NULL`
const visibleCommentPredicate = `NOT c.removed
 AND NOT EXISTS(SELECT 1 FROM chat.interaction_erased_wallets e WHERE e.wallet=c.author)
 AND NOT EXISTS(SELECT 1 FROM chat.safety_visibility v WHERE v.target_type='comment' AND v.target_id=c.id AND v.status='removed')`

func (c *socialComments) byID(ctx context.Context, id string) (interactions.CommentView, error) {
	var v interactions.CommentView
	if c.pool == nil {
		return v, interactions.ErrDependency
	}
	err := c.pool.QueryRow(ctx, `SELECT `+visibleCommentColumns+` FROM chat.interaction_comments c WHERE c.id=$1 AND `+visibleCommentPredicate, id).Scan(&v.ID, &v.Pool, &v.Author, &v.Parent, &v.Body, &v.Likes, &v.Revision, &v.Edited)
	return v, err
}
func (c *socialComments) page(ctx context.Context, pool string, limit int, cursor string) ([]interactions.CommentView, *string, string, error) {
	out := []interactions.CommentView{}
	if c.pool == nil {
		return out, nil, "", interactions.ErrDependency
	}
	tx, err := c.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return out, nil, "", err
	}
	defer tx.Rollback(ctx)
	var overlay string
	err = tx.QueryRow(ctx, `SELECT COALESCE(sum(v.revision),0)::text FROM chat.safety_visibility v JOIN chat.interaction_comments c ON v.target_type='comment' AND v.target_id=c.id WHERE c.pool=$1`, pool).Scan(&overlay)
	if err != nil {
		return out, nil, "", err
	}
	rows, err := tx.Query(ctx, `SELECT `+visibleCommentColumns+`,transaction_timestamp() FROM chat.interaction_comments c WHERE c.pool=$1 AND ($2='' OR c.id<$2) AND `+visibleCommentPredicate+` ORDER BY c.id DESC LIMIT $3`, pool, cursor, limit+1)
	if err != nil {
		return out, nil, "", err
	}
	defer rows.Close()
	for rows.Next() {
		var v interactions.CommentView
		var at time.Time
		if err = rows.Scan(&v.ID, &v.Pool, &v.Author, &v.Parent, &v.Body, &v.Likes, &v.Revision, &v.Edited, &at); err != nil {
			return out, nil, "", err
		}
		v.CountAsOf = at.UTC().Format(time.RFC3339Nano)
		out = append(out, v)
	}
	if err = rows.Err(); err != nil {
		return out, nil, "", err
	}
	var next *string
	if len(out) > limit {
		out = out[:limit]
		v := out[limit-1].ID
		next = &v
	}
	return out, next, commentSnapshotTag(pool, cursor, limit, overlay, out), nil
}
func commentSnapshotTag(pool, cursor string, limit int, overlay string, views []interactions.CommentView) string {
	stable := append([]interactions.CommentView(nil), views...)
	for i := range stable {
		stable[i].CountAsOf = ""
	}
	raw, _ := json.Marshal(struct {
		Pool, Cursor, Overlay string
		Limit                 int
		Items                 []interactions.CommentView
	}{pool, cursor, overlay, limit, stable})
	return fmt.Sprintf(`"%x"`, sha256.Sum256(raw))
}
func commentListPath(path string) (pool string, frozen bool, ok bool) {
	p := strings.Split(path, "/")
	if len(p) == 5 && p[1] == "v2" && p[2] == "pools" && p[4] == "comments" {
		return p[3], true, true
	}
	if len(p) == 6 && p[1] == "v2" && p[2] == "social" && p[3] == "pool" && p[5] == "comments" {
		return p[4], false, true
	}
	return "", false, false
}
func commentReadError(w http.ResponseWriter, req *http.Request, frozen bool, err error) {
	if frozen {
		frozenError(w, req, err)
		return
	}
	status, code := 503, "DEPENDENCY_UNAVAILABLE"
	switch {
	case errors.Is(err, interactions.ErrInvalid):
		status, code = 400, "INVALID_INPUT"
	case errors.Is(err, pgx.ErrNoRows):
		status, code = 404, "NOT_FOUND"
	case errors.Is(err, interactions.ErrDenied):
		status, code = 403, "SCOPE_DENIED"
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"code": code, "retryable": status == 503})
}
func (c *socialComments) reads(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		pool, frozen, ok := commentListPath(req.URL.Path)
		if req.Method != http.MethodGet || !ok {
			next.ServeHTTP(w, req)
			return
		}
		pool = common.NormalizeAddr(pool)
		if !common.IsAddr(pool) {
			commentReadError(w, req, frozen, interactions.ErrInvalid)
			return
		}
		limit := 50
		if raw := req.URL.Query().Get("limit"); raw != "" {
			n, err := strconv.Atoi(raw)
			if err != nil || n < 1 || (frozen && n > 100) {
				commentReadError(w, req, frozen, interactions.ErrInvalid)
				return
			}
			limit = min(n, 100)
		}
		cursor := req.URL.Query().Get("cursor")
		if frozen && cursor != "" {
			id, err := uuid.Parse(cursor)
			if err != nil || len(cursor) != 36 {
				frozenWriteError(w, req, 400, "CURSOR_INVALID")
				return
			}
			cursor = id.String()
		}
		if !frozen && len(cursor) > 160 {
			commentReadError(w, req, false, interactions.ErrInvalid)
			return
		}
		views, after, tag, err := c.page(req.Context(), pool, limit, cursor)
		if err != nil {
			commentReadError(w, req, frozen, err)
			return
		}
		w.Header().Set("ETag", tag)
		w.Header().Set("Cache-Control", "private, no-store")
		w.Header().Add("Vary", "Authorization")
		w.Header().Add("Vary", "Cookie")
		if req.Header.Get("If-None-Match") == tag {
			w.WriteHeader(304)
			return
		}
		if frozen {
			items := make([]v2.Comment, 0, len(views))
			for _, v := range views {
				items = append(items, frozenComment(v))
			}
			frozenData(w, req, 200, "CommentsListResponse", v2.CommentsListData{Items: items}, after)
		} else {
			frozenData(w, req, 200, "", map[string]any{"items": views}, after)
		}
	})
}
