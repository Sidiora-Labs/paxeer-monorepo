package l18

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/jackc/pgx/v5"
)

// RewardDeliveryPolicy admits exactly the subjects emitted by this ledger.
// Return fresh maps so composition cannot mutate a running store's policy.
func RewardDeliveryPolicy(base delivery.Policy) delivery.Policy {
	p := delivery.Policy{Subjects: map[string]bool{}, EventTypes: map[string]bool{}}
	for subject, admitted := range base.Subjects {
		p.Subjects[subject] = admitted
	}
	for event, admitted := range base.EventTypes {
		p.EventTypes[event] = admitted
	}
	for _, subject := range []string{
		"fee.claimed", "airdrop.epoch", "airdrop.claimed",
		"reward.funding", "reward.liability", "reward.review",
		"reward.claim_prepared", "reward.community_authority",
		"reward.payout_intent", "reward.paid", "reward.payout_state",
		"reward.correction_review", "referral.analytics", "referral.bound",
		"referral.code", "referral.consent", "referral.click", "referral.bind",
	} {
		p.Subjects[subject], p.EventTypes[subject] = true, true
	}
	return p
}

// OutboxRuntimeGrantSQL is applied by the schema owner, never by the runtime.
// All other ledger columns remain immutable to the publisher.
func OutboxRuntimeGrantSQL(schema, role string) (string, error) {
	if !schemaName.MatchString(schema) || !schemaName.MatchString(role) {
		return "", ErrInvalid
	}
	return "GRANT UPDATE (\"published_at\") ON TABLE " + pgx.Identifier{schema, "l18_outbox"}.Sanitize() + " TO " + pgx.Identifier{role}.Sanitize(), nil
}

type OutboxBridge struct {
	ledger *Ledger
	store  *delivery.Store
	batch  int
}

func NewOutboxBridge(ctx context.Context, ledger *Ledger, store *delivery.Store, batch int) (*OutboxBridge, error) {
	if ledger == nil || ledger.epochGuard == nil || store == nil || batch < 1 || batch > 100 {
		return nil, ErrInvalid
	}
	tx, err := ledger.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var permitted bool
	err = tx.QueryRow(ctx, `SELECT has_column_privilege(current_user,$1,'published_at','UPDATE')`, pgx.Identifier{ledger.schema, "l18_outbox"}.Sanitize()).Scan(&permitted)
	if err != nil {
		return nil, err
	}
	if !permitted {
		return nil, delivery.ErrAuthorization
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &OutboxBridge{ledger: ledger, store: store, batch: batch}, nil
}

// RunOnce atomically hands committed local events to shared delivery. Here
// published_at records handoff; delivery_outbox owns broker acknowledgement,
// fenced retries and quarantine. A failed transaction leaves both sides intact.
func (b *OutboxBridge) RunOnce(ctx context.Context) error {
	if b == nil || b.ledger == nil || b.store == nil {
		return ErrUnavailable
	}
	tx, err := b.ledger.begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
		defer cancel()
		_ = tx.Rollback(cleanup)
	}()
	rows, err := tx.Query(ctx, `SELECT sequence,identity,topic,payload FROM l18_outbox
 WHERE published_at IS NULL ORDER BY sequence LIMIT $1 FOR UPDATE SKIP LOCKED`, b.batch)
	if err != nil {
		return err
	}
	type pending struct {
		sequence        int64
		identity, topic string
		payload         json.RawMessage
	}
	var events []pending
	for rows.Next() {
		var event pending
		if err = rows.Scan(&event.sequence, &event.identity, &event.topic, &event.payload); err != nil {
			rows.Close()
			return err
		}
		events = append(events, event)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	policy := RewardDeliveryPolicy(delivery.Policy{})
	for _, event := range events {
		if event.sequence < 1 || !policy.Subjects[event.topic] {
			return delivery.ErrAuthorization
		}
		// Local identities contain punctuation forbidden by the shared codec.
		// Bind the complete source identity, schema and topic without truncation.
		identity := "l18_" + fingerprint(struct{ Schema, Identity, Topic string }{b.ledger.schema, event.identity, event.topic})
		envelope := delivery.Envelope{
			EventID: identity, EventType: event.topic, SchemaVersion: 1, ChainID: 125,
			SourceOrder: delivery.SourceOrder{Height: "0", TransactionIndex: "0", LogIndex: strconv.FormatInt(event.sequence, 10)},
			EmittedAt:   time.Now().UTC(), TraceID: identity,
			PayloadHash: delivery.Hash(event.payload), Payload: event.payload,
		}
		if err = b.store.Enqueue(ctx, tx, event.topic, envelope); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `UPDATE l18_outbox SET published_at=clock_timestamp() WHERE sequence=$1 AND published_at IS NULL`, event.sequence)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return ErrConflict
		}
	}
	if err = b.ledger.epochGuard(ctx, tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
