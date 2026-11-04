package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/fanout"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/interactions"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/safety"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const safetyProjectionGeneration = "social_safety_visibility_v1"

func projectSafetyMessage(m fanout.DurableMessage, v safety.Visibility, erased bool, applied int64) (fanout.DurableMessage, error) {
	if (v.Revision == 0 && v.Status != "") || v.Revision < 0 || (v.Revision > 0 && (v.TargetType != "message" || v.TargetID != m.ID || (v.Status != "removed" && v.Status != "restored"))) || applied < 0 || applied > v.Revision || m.Revision < 1 || v.Revision-applied > math.MaxInt64-m.Revision {
		return m, safety.ErrInvalid
	}
	// Include the authoritative overlay revision even before the consumer has
	// emitted its replay event, and on restoration as well as removal.
	m.Revision += v.Revision - applied
	if m.Deleted || erased || v.Status == "removed" {
		m.Deleted = true
		m.Content = ""
		m.ReplyTo = ""
	}
	return m, nil
}
func projectMessageFromStore(ctx context.Context, db *pgxpool.Pool, m fanout.DurableMessage) (fanout.DurableMessage, error) {
	if db == nil {
		return m, safety.ErrUnavailable
	}
	var erased bool
	var applied int64
	v := safety.Visibility{TargetType: "message", TargetID: m.ID}
	err := db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM chat.interaction_erased_wallets WHERE wallet=$1),COALESCE(v.revision,0),COALESCE(v.status,''),COALESCE(p.revision,0) FROM (SELECT 1) seed LEFT JOIN chat.safety_visibility v ON v.target_type='message' AND v.target_id=$2 LEFT JOIN chat.interaction_feed_projection p ON p.generation=$3 AND p.aggregate_id='message:'||$2`, m.Sender, m.ID, safetyProjectionGeneration).Scan(&erased, &v.Revision, &v.Status, &applied)
	if err != nil {
		return m, err
	}
	return projectSafetyMessage(m, v, erased, applied)
}

// Source bodies remain under the existing source-table retention policy.
// Moderation changes only the overlay, revisions and replay; author deletion
// and account erasure still clear the source through their original paths.
func (r *socialConnectedRuntime) applyVisibilityOverlay(ctx context.Context, tx pgx.Tx, v safety.Visibility) error {
	if tx == nil {
		return safety.ErrUnavailable
	}
	if v.TargetID == "" || v.Revision < 1 || (v.Status != "removed" && v.Status != "restored") {
		return safety.ErrInvalid
	}
	if v.TargetType != "message" && v.TargetType != "comment" {
		return nil
	}
	key := v.TargetType + ":" + v.TargetID
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, key); err != nil {
		return err
	}
	var current safety.Visibility
	current.TargetType, current.TargetID = v.TargetType, v.TargetID
	if err := tx.QueryRow(ctx, `SELECT revision,status,case_id::text FROM chat.safety_visibility WHERE target_type=$1 AND target_id=$2`, v.TargetType, v.TargetID).Scan(&current.Revision, &current.Status, &current.CaseID); err != nil {
		return err
	}
	if current.Revision > v.Revision {
		return nil
	}
	if current != v {
		return safety.ErrConflict
	}
	var applied int64
	err := tx.QueryRow(ctx, `SELECT revision FROM chat.interaction_feed_projection WHERE generation=$1 AND aggregate_id=$2`, safetyProjectionGeneration, key).Scan(&applied)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if applied >= v.Revision {
		return nil
	}
	var actor string
	if v.TargetType == "message" {
		actor, err = r.messageVisibilityEvent(ctx, tx, v, applied)
	} else {
		actor, err = r.commentVisibilityEvent(ctx, tx, v)
	}
	if err != nil {
		return err
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO chat.interaction_feed_projection(generation,aggregate_id,revision,actor,removed,payload) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(generation,aggregate_id) DO UPDATE SET revision=EXCLUDED.revision,actor=EXCLUDED.actor,removed=EXCLUDED.removed,payload=EXCLUDED.payload`, safetyProjectionGeneration, key, v.Revision, actor, v.Status == "removed", raw)
	return err
}
func (r *socialConnectedRuntime) messageVisibilityEvent(ctx context.Context, tx pgx.Tx, v safety.Visibility, applied int64) (string, error) {
	var m fanout.DurableMessage
	var kind string
	err := tx.QueryRow(ctx, `SELECT m.id,m.conversation_id,m.sender,m.content,m.reply_to,m.sequence,m.revision,extract(epoch from m.created_at)::bigint,m.deleted_at IS NOT NULL,m.client_id,c.kind FROM chat.chat_messages m JOIN chat.chat_conversations c ON c.id=m.conversation_id WHERE m.id=$1 FOR UPDATE OF m,c`, v.TargetID).Scan(&m.ID, &m.ConversationID, &m.Sender, &m.Content, &m.ReplyTo, &m.Sequence, &m.Revision, &m.CreatedAt, &m.Deleted, &m.ClientID, &kind)
	if err != nil {
		return "", err
	}
	m.Type = kind
	channels := []string{}
	namespace := ""
	if r.chat != nil {
		namespace = r.chat.ChannelNamespace
	}
	if kind == "pool" {
		m.Type = "pool_message"
		m.PoolAddress = strings.TrimPrefix(m.ConversationID, "pool:")
		channels = append(channels, namespace+"pool:"+m.PoolAddress)
	} else if kind == "dm" {
		rows, e := tx.Query(ctx, `SELECT wallet FROM chat.chat_members WHERE conversation_id=$1 ORDER BY wallet`, m.ConversationID)
		if e != nil {
			return "", e
		}
		for rows.Next() {
			var wallet string
			if e = rows.Scan(&wallet); e != nil {
				rows.Close()
				return "", e
			}
			channels = append(channels, fanout.WalletChannel(namespace, wallet))
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return "", e
		}
	} else {
		return "", safety.ErrInvalid
	}
	var erased bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM chat.interaction_erased_wallets WHERE wallet=$1)`, m.Sender).Scan(&erased); err != nil {
		return "", err
	}
	projected, err := projectSafetyMessage(m, v, erased, applied)
	if err != nil {
		return "", err
	}
	m.Revision = projected.Revision
	if _, err = tx.Exec(ctx, `UPDATE chat.chat_messages SET revision=$2 WHERE id=$1`, m.ID, m.Revision); err != nil {
		return "", err
	}
	if _, err = tx.Exec(ctx, `UPDATE chat.chat_conversations SET history_version=history_version+1 WHERE id=$1`, m.ConversationID); err != nil {
		return "", err
	}
	payload, err := json.Marshal(projected)
	if err != nil {
		return "", err
	}
	wireChannels, err := json.Marshal(channels)
	if err != nil {
		return "", err
	}
	_, err = tx.Exec(ctx, `INSERT INTO chat.chat_outbox(event_id,message_id,revision,payload,channels) VALUES($1,$2,$3,$4,$5)`, fmt.Sprintf("safety_message_%s_%d", m.ID, v.Revision), m.ID, m.Revision, payload, wireChannels)
	return m.Sender, err
}
func projectSafetyComment(v interactions.CommentView, visibility safety.Visibility, removed, erased bool, revision int64) interactions.PublicEvent {
	event := interactions.PublicEvent{AggregateID: "comment:" + v.ID, Revision: revision, Actor: v.Author, Action: "comment", Target: v.ID, Pool: v.Pool, Body: v.Body, Removed: removed || erased || visibility.Status == "removed"}
	if v.Parent != nil {
		event.Parent = *v.Parent
	}
	if event.Removed {
		event.Body = ""
		event.Parent = ""
	}
	return event
}
func (r *socialConnectedRuntime) commentVisibilityEvent(ctx context.Context, tx pgx.Tx, v safety.Visibility) (string, error) {
	var c interactions.CommentView
	var removed, erased bool
	err := tx.QueryRow(ctx, `SELECT c.id,c.pool,c.author,c.parent,c.body,c.removed,EXISTS(SELECT 1 FROM chat.interaction_erased_wallets WHERE wallet=c.author) FROM chat.interaction_comments c WHERE c.id=$1 FOR UPDATE`, v.TargetID).Scan(&c.ID, &c.Pool, &c.Author, &c.Parent, &c.Body, &removed, &erased)
	if err != nil {
		return "", err
	}
	if _, err = tx.Exec(ctx, `UPDATE chat.interaction_comments SET revision=revision+1 WHERE id=$1`, c.ID); err != nil {
		return "", err
	}
	var revision int64
	aggregate := "comment:" + c.ID
	err = tx.QueryRow(ctx, `INSERT INTO chat.interaction_aggregate_revisions AS a(aggregate_id,revision) VALUES($1,1) ON CONFLICT(aggregate_id) DO UPDATE SET revision=a.revision+1 RETURNING revision`, aggregate).Scan(&revision)
	if err != nil {
		return "", err
	}
	event := projectSafetyComment(c, v, removed, erased, revision)
	payload, err := json.Marshal(event)
	if err != nil {
		return "", err
	}
	if _, err = interactions.DecodePublicEvent(payload); err != nil {
		return "", err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO chat.interaction_outbox(actor,action,target,aggregate_id,revision,removed,visibility) VALUES($1,'comment',$2,$3,$4,$5,'public')`, c.Author, c.ID, aggregate, revision, event.Removed); err != nil {
		return "", err
	}
	id := fmt.Sprintf("social_safety_comment_%s_%d", c.ID, v.Revision)
	var at time.Time
	if err = tx.QueryRow(ctx, `SELECT transaction_timestamp()`).Scan(&at); err != nil {
		return "", err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO chat.interaction_public_events(event_id,aggregate_id,revision,payload,payload_hash,created_at) VALUES($1,$2,$3,$4,$5,$6)`, id, aggregate, revision, payload, delivery.Hash(payload), at); err != nil {
		return "", err
	}
	for _, scope := range []string{"pool:" + c.Pool, "thread:" + c.ID} {
		if _, err = tx.Exec(ctx, `INSERT INTO chat.interaction_visibility_revision AS a(scope,revision) VALUES($1,1) ON CONFLICT(scope) DO UPDATE SET revision=a.revision+1`, scope); err != nil {
			return "", err
		}
	}
	if r.delivery != nil {
		position, e := r.sourceOrder.Read(ctx, tx)
		if e != nil {
			return "", e
		}
		err = r.delivery.Enqueue(ctx, tx, r.binding.InteractionSubjects["social.comment.changed"], delivery.Envelope{EventID: id, EventType: "social.comment.changed", SchemaVersion: 1, ChainID: 125, SourceOrder: position, EmittedAt: at.UTC(), TraceID: id, PayloadHash: delivery.Hash(payload), Payload: payload})
	}
	return c.Author, err
}

// The platform assembly uses the same consumer with a durable projection
// cursor. It reads authoritative visibility, so it does not need UPDATE on
// safety_outbox (the platform credential is intentionally not granted it).
func (r *socialConnectedRuntime) visibilityOnce(ctx context.Context) error {
	rows, err := r.pool.Query(ctx, `SELECT v.target_type,v.target_id,v.status,v.case_id::text,v.revision FROM chat.safety_visibility v WHERE v.target_type IN ('message','comment') AND NOT EXISTS(SELECT 1 FROM chat.interaction_feed_projection p WHERE p.generation=$1 AND p.aggregate_id=v.target_type||':'||v.target_id AND p.revision>=v.revision) ORDER BY v.target_type,v.target_id LIMIT 32`, safetyProjectionGeneration)
	if err != nil {
		return err
	}
	values := []safety.Visibility{}
	for rows.Next() {
		var v safety.Visibility
		if err = rows.Scan(&v.TargetType, &v.TargetID, &v.Status, &v.CaseID, &v.Revision); err != nil {
			rows.Close()
			return err
		}
		values = append(values, v)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, v := range values {
		tx, e := r.pool.Begin(ctx)
		if e != nil {
			return e
		}
		e = r.applyVisibility(ctx, tx, v)
		if e == nil {
			e = tx.Commit(ctx)
		} else {
			_ = tx.Rollback(ctx)
		}
		if e != nil {
			return e
		}
	}
	return nil
}
func (r *socialConnectedRuntime) runVisibility(ctx context.Context) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		if err := r.visibilityOnce(ctx); err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
