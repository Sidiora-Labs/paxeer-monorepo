package processor

import (
	"context"
	"encoding/json"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
)

func TestGenerationComparisonRejectsUnavailableAndMalformedDurableFacts(t *testing.T) {
	ctx := context.Background()
	h := newLifecycleCoverageHarness(t)
	h.inventory.batch(t, ctx, 33963716, 33963716)
	r := authorizeCoverageReplay(t, h, "reconstruct", "snapshot-admission", 33963733, 33963733)
	p, err := NewGenerationReplayProjection(h.Replay, r)
	if err != nil {
		t.Fatal(err)
	}
	segment, err := h.Replay.fetch(ctx, r, 33963733, 33963733)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := h.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	for _, raw := range segment.Records {
		if err = p.ApplyRawTx(ctx, tx, r.Generation, delivery.ArchiveRaw{Height: raw.Height, TransactionHash: raw.TransactionHash, BlockHash: raw.BlockHash, Emitter: raw.Emitter, TransactionIndex: raw.TransactionIndex, LogIndex: raw.LogIndex, Topics: raw.Topics, Data: raw.Data}); err != nil {
			t.Fatal(err)
		}
	}
	baseline, err := p.ProjectionDigestTx(ctx, tx, r.Generation)
	if err != nil || len(baseline) != 64 {
		t.Fatalf("generation digest %s: %v", baseline, err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, sql string }{
		{"raw table", `ALTER TABLE indexer.indexer_generation_raw RENAME TO raw_unavailable`},
		{"raw shape", `UPDATE indexer.indexer_generation_raw SET payload='[]'`},
		{"effects table", `ALTER TABLE indexer.indexer_replay_effects RENAME TO effects_unavailable`},
		{"outbox table", `ALTER TABLE indexer.delivery_outbox RENAME TO outbox_unavailable`},
		{"count authority", `UPDATE indexer.indexer_generation_raw SET payload=jsonb_set(payload,'{Events}','[]'); ALTER TABLE indexer.indexer_replay_effects RENAME TO effects_unavailable`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, e := h.Pool.Begin(ctx)
			if e != nil {
				t.Fatal(e)
			}
			defer tx.Rollback(ctx)
			if _, e = tx.Exec(ctx, tc.sql); e != nil {
				t.Fatal(e)
			}
			if value, e := p.ProjectionDigestTx(ctx, tx, r.Generation); e == nil || value != "" {
				t.Fatalf("unavailable generation digest %q: %v", value, e)
			}
		})
	}
	for _, tc := range []struct {
		name string
		call func() error
	}{
		{"digest encoding", func() error {
			_, e := generationDigest([]generationFact{{Events: []store.ReplayEvent{{Payload: json.RawMessage(`{`)}}}})
			return e
		}},
		{"envelope encoding", func() error { _, e := canonicalReplayPayload(ReplayEnvelopeV2{Event: json.RawMessage(`{`)}); return e }},
		{"request", func() error { _, e := NewGenerationReplayProjection(h.Replay, store.ReplayRequest{}); return e }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.call(); err == nil {
				t.Fatal("invalid replay representation admitted")
			}
		})
	}
}
