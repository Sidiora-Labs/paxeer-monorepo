package processor

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
)

func TestReconstructionObservationsRequireReadableEffectsAndExactOutbox(t *testing.T) {
	ctx := context.Background()
	h := newLifecycleCoverageHarness(t)
	h.inventory.batch(t, ctx, 33963716, 33963716)
	request := authorizeCoverageReplay(t, h, "reconstruct", "observation-admission", 33963733, 33963733)
	projection, err := NewGenerationReplayProjection(h.Replay, request)
	if err != nil {
		t.Fatal(err)
	}
	segment, err := h.Replay.fetch(ctx, request, 33963733, 33963733)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := h.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	for _, raw := range segment.Records {
		if err = projection.ApplyRawTx(ctx, tx, request.Generation, delivery.ArchiveRaw{Height: raw.Height, TransactionHash: raw.TransactionHash, BlockHash: raw.BlockHash, Emitter: raw.Emitter, TransactionIndex: raw.TransactionIndex, LogIndex: raw.LogIndex, Topics: raw.Topics, Data: raw.Data}); err != nil {
			t.Fatal(err)
		}
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	invalidSegment := segment
	invalidSegment.Events = append([]store.ReplayEvent(nil), segment.Events...)
	invalidSegment.Events[0].Payload = json.RawMessage(`{`)
	if err := h.verifyReconstructionOutbox(ctx, request, invalidSegment); err == nil {
		t.Fatal("unencodable replay event admitted")
	}
	baseline, err := h.reconstructionEffects(ctx, request)
	if err != nil || len(baseline) != 64 {
		t.Fatalf("original effects: %s %v", baseline, err)
	}
	if err = h.verifyReconstructionOutbox(ctx, request, segment); err != nil {
		t.Fatal(err)
	}
	if _, err = h.Pool.Exec(ctx, `CREATE TABLE indexer.saved_effects AS TABLE indexer.indexer_replay_effects; CREATE TABLE indexer.saved_outbox AS TABLE indexer.delivery_outbox`); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, sql, restore string
		outbox             bool
	}{
		{"effects unavailable", `ALTER TABLE indexer.indexer_replay_effects RENAME TO effects_unavailable`, `ALTER TABLE indexer.effects_unavailable RENAME TO indexer_replay_effects`, false},
		{"effect kind unavailable", `ALTER TABLE indexer.indexer_replay_effects ALTER COLUMN event_kind DROP NOT NULL;UPDATE indexer.indexer_replay_effects SET event_kind=NULL`, `UPDATE indexer.indexer_replay_effects e SET event_kind=s.event_kind FROM indexer.saved_effects s WHERE e.effect_id=s.effect_id;ALTER TABLE indexer.indexer_replay_effects ALTER COLUMN event_kind SET NOT NULL`, false},
		{"effect payload unavailable", `ALTER TABLE indexer.indexer_replay_effects ALTER COLUMN payload DROP NOT NULL;UPDATE indexer.indexer_replay_effects SET payload=NULL`, `UPDATE indexer.indexer_replay_effects e SET payload=s.payload FROM indexer.saved_effects s WHERE e.effect_id=s.effect_id;ALTER TABLE indexer.indexer_replay_effects ALTER COLUMN payload SET NOT NULL`, false},
		{"outbox unavailable", `ALTER TABLE indexer.delivery_outbox RENAME TO outbox_unavailable`, `ALTER TABLE indexer.outbox_unavailable RENAME TO delivery_outbox`, true},
		{"envelope unavailable", `DELETE FROM indexer.delivery_outbox;INSERT INTO indexer.delivery_outbox SELECT (jsonb_populate_record(NULL::indexer.delivery_outbox,to_jsonb(s)||jsonb_build_object('envelope',decode('7b','hex')))).* FROM indexer.saved_outbox s`, `DELETE FROM indexer.delivery_outbox;INSERT INTO indexer.delivery_outbox SELECT * FROM indexer.saved_outbox`, true},
		{"outbox digest", `DELETE FROM indexer.delivery_outbox;INSERT INTO indexer.delivery_outbox SELECT (jsonb_populate_record(NULL::indexer.delivery_outbox,to_jsonb(s)||jsonb_build_object('payload_hash',repeat('a',64)))).* FROM indexer.saved_outbox s`, `DELETE FROM indexer.delivery_outbox;INSERT INTO indexer.delivery_outbox SELECT * FROM indexer.saved_outbox`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := h.Pool.Exec(ctx, tc.sql); err != nil {
				t.Fatal(err)
			}
			defer func() {
				if _, err := h.Pool.Exec(ctx, tc.restore); err != nil {
					t.Fatal(err)
				}
			}()
			if tc.outbox {
				if err := h.verifyReconstructionOutbox(ctx, request, segment); err == nil {
					t.Fatal("unbound outbox acknowledged")
				}
			} else if digest, err := h.reconstructionEffects(ctx, request); err == nil || digest != "" {
				t.Fatalf("unreadable effects admitted: %s %v", digest, err)
			}
		})
	}
	after, err := h.reconstructionEffects(ctx, request)
	if err != nil || after != baseline {
		t.Fatalf("original effects not retained: %s %v", after, err)
	}
	if err = h.verifyReconstructionOutbox(ctx, request, segment); err != nil {
		t.Fatal(err)
	}
}
