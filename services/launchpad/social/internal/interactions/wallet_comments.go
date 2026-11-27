package interactions

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

const walletCommentsResource = "wallet_comments"

type walletCommentsCursor struct {
	Scope  string    `json:"scope"`
	Wallet string    `json:"wallet"`
	At     time.Time `json:"at"`
	ID     string    `json:"id"`
}

const walletCommentsQuery = `SELECT c.id,c.pool,c.author,c.parent,c.body,(SELECT count(*) FROM interaction_reactions r WHERE r.target=c.id AND NOT EXISTS(SELECT 1 FROM interaction_erased_wallets e WHERE e.wallet=r.actor))::text,c.revision::text,c.edited_at IS NOT NULL,statement_timestamp(),c.created_at FROM interaction_comments c WHERE c.author=$1 AND ($2::timestamptz IS NULL OR (c.created_at,c.id)<($2,cast($3 AS text))) AND NOT c.removed AND NOT EXISTS(SELECT 1 FROM interaction_erased_wallets e WHERE e.wallet=c.author) ORDER BY c.created_at DESC,c.id DESC LIMIT $4`

func (h *HTTP) walletCommentsSnapshot(ctx context.Context, wallet string, limit int, cursor string) ([]CommentView, *string, string, error) {
	out := []CommentView{}
	wallet, err := Wallet(wallet)
	if err != nil {
		return out, nil, "", err
	}
	if limit < 1 || limit > 100 || len(cursor) > 1024 {
		return out, nil, "", ErrInvalid
	}
	var at *time.Time
	id := ""
	if cursor != "" {
		raw, decodeErr := base64.RawURLEncoding.DecodeString(cursor)
		var position walletCommentsCursor
		if decodeErr != nil || json.Unmarshal(raw, &position) != nil || position.Scope != walletCommentsResource || position.Wallet != wallet || position.At.IsZero() {
			return out, nil, "", ErrInvalid
		}
		if _, parseErr := uuid.Parse(position.ID); parseErr != nil {
			return out, nil, "", ErrInvalid
		}
		at, id = &position.At, position.ID
	}
	tx, err := h.Store.DB.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return out, nil, "", err
	}
	defer rollback(tx)
	var revision, overlay int64
	if err = tx.QueryRow(ctx, `SELECT COALESCE((SELECT revision FROM interaction_visibility_revision WHERE scope=$1),0),COALESCE((SELECT revision FROM interaction_visibility_revision WHERE scope='erasure_overlay'),0)`, "comments:wallet:"+wallet).Scan(&revision, &overlay); err != nil {
		return out, nil, "", err
	}
	tag := `"` + CacheKey(wallet, "", walletCommentsResource+":"+strconv.Itoa(limit)+":"+cursor+":"+strconv.FormatInt(overlay, 10), revision) + `"`
	rows, err := tx.Query(ctx, walletCommentsQuery, wallet, at, id, limit+1)
	if err != nil {
		return out, nil, "", err
	}
	defer rows.Close()
	var last walletCommentsCursor
	for rows.Next() {
		var view CommentView
		var countedAt, createdAt time.Time
		if err = rows.Scan(&view.ID, &view.Pool, &view.Author, &view.Parent, &view.Body, &view.Likes, &view.Revision, &view.Edited, &countedAt, &createdAt); err != nil {
			return out, nil, "", err
		}
		view.CountAsOf = countedAt.UTC().Format(time.RFC3339Nano)
		out = append(out, view)
		if len(out) == limit {
			last = walletCommentsCursor{walletCommentsResource, wallet, createdAt, view.ID}
		}
	}
	if err = rows.Err(); err != nil {
		return out, nil, "", err
	}
	var next *string
	if len(out) > limit {
		out = out[:limit]
		raw, encodeErr := json.Marshal(last)
		if encodeErr != nil {
			return out, nil, "", encodeErr
		}
		encoded := base64.RawURLEncoding.EncodeToString(raw)
		next = &encoded
	}
	return out, next, tag, nil
}
