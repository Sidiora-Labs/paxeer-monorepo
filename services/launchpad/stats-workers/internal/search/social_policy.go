package search

import (
	"context"
	"encoding/json"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/jackc/pgx/v5"
)

const socialPoolVisibilityView = "chat.pool_search_visibility_v2"
const socialPoolVisibilitySQL = `SELECT pool,public,revision FROM ` + socialPoolVisibilityView + ` WHERE ($1='pool' AND lower(pool)=lower($2)) OR ($1='token' AND lower(token)=lower($2)) ORDER BY pool`

func (p *Projection) ConsumeSocialPolicy(ctx context.Context, subject string, wire []byte, message delivery.Delivery) error {
	if p == nil || p.Delivery == nil || p.ConsumerID == "" || p.InboxGeneration == "" || p.InvalidationSubject == "" || p.InvalidationEventType == "" {
		return ErrUnavailable
	}
	bounded, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return p.Delivery.Consume(bounded, p.ConsumerID, p.InboxGeneration, subject, wire, message, func(ctx context.Context, tx pgx.Tx, event delivery.Envelope) error {
		if event.EventType != "social.policy.changed.v2" {
			return ErrInvalid
		}
		var policy struct {
			Kind       string `json:"kind"`
			Visibility *struct {
				TargetType string `json:"targetType"`
				TargetID   string `json:"targetId"`
			} `json:"visibility"`
		}
		if !utf8.Valid(event.Payload) || json.Unmarshal(event.Payload, &policy) != nil {
			return ErrInvalid
		}
		if policy.Kind != "visibility_changed" || policy.Visibility == nil || (policy.Visibility.TargetType != "pool" && policy.Visibility.TargetType != "token") {
			return nil
		}
		if _, err := tx.Exec(ctx, "SET LOCAL statement_timeout='2s'"); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, "SET LOCAL lock_timeout='100ms'"); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, "SELECT revision FROM stats_search.active_v2 WHERE singleton FOR UPDATE"); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, socialPoolVisibilitySQL, policy.Visibility.TargetType, policy.Visibility.TargetID)
		if err != nil {
			return err
		}
		changes := []Change{}
		for rows.Next() {
			var pool string
			var public bool
			var revision int64
			if err = rows.Scan(&pool, &public, &revision); err != nil {
				rows.Close()
				return err
			}
			pool = strings.ToLower(pool)
			if !exactIdentity(pool) || revision < 0 || revision == math.MaxInt64 {
				rows.Close()
				return delivery.ErrIntegrity
			}
			changes = append(changes, Change{Version: 2, Kind: valueVisibility, PoolAddress: pool, Revision: revision + 1, Public: public, Tombstone: false})
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		changed := false
		for _, change := range changes {
			applied, err := applyChangeTx(ctx, tx, change)
			if err != nil {
				return err
			}
			changed = changed || applied
		}
		if !changed {
			return nil
		}
		var revision int64
		if err = tx.QueryRow(ctx, "UPDATE stats_search.active_v2 SET revision=nextval('stats_search.revision_v2'),privacy_revision=privacy_revision+1 WHERE singleton RETURNING revision").Scan(&revision); err != nil {
			return err
		}
		payload, _ := json.Marshal(map[string]any{valueVersion: 2, valueRevision: strconv.FormatInt(revision, 10)})
		notification := delivery.Envelope{EventID: "search_" + delivery.Hash([]byte(event.EventID)), EventType: p.InvalidationEventType, SchemaVersion: 1, ChainID: 125, SourceOrder: event.SourceOrder, EmittedAt: event.EmittedAt, TraceID: event.TraceID, PayloadHash: delivery.Hash(payload), Payload: payload}
		return p.Delivery.Enqueue(ctx, tx, p.InvalidationSubject, notification)
	})
}
