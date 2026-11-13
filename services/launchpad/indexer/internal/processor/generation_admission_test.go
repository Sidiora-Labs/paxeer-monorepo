package processor

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
)

func TestGenerationApplicationRejectsChangedRawAndUnavailableTables(t *testing.T) {
	ctx := context.Background()
	h := newLifecycleCoverageHarness(t)
	h.inventory.batch(t, ctx, 33963716, 33963716)
	request := authorizeCoverageReplay(t, h, "reconstruct", "generation-admission", 33963733, 33963733)
	projection, err := NewGenerationReplayProjection(h.Replay, request)
	if err != nil {
		t.Fatal(err)
	}
	segment, err := h.Replay.fetch(ctx, request, 33963733, 33963733)
	if err != nil {
		t.Fatal(err)
	}
	var input delivery.ArchiveRaw
	for _, raw := range segment.Records {
		digest, err := store.RawDigest(raw)
		if err != nil {
			t.Fatal(err)
		}
		for _, event := range segment.Events {
			if event.RawDigest == digest {
				input = delivery.ArchiveRaw{Height: raw.Height, TransactionHash: raw.TransactionHash, BlockHash: raw.BlockHash, Emitter: raw.Emitter, TransactionIndex: raw.TransactionIndex, LogIndex: raw.LogIndex, Topics: raw.Topics, Data: raw.Data}
				break
			}
		}
		if input.Height != 0 {
			break
		}
	}
	if input.Height == 0 {
		t.Fatal("qualified event raw input absent")
	}
	for _, tc := range []struct {
		name, sql string
		edit      func(*delivery.ArchiveRaw)
	}{
		{name: "malformed identity", edit: func(r *delivery.ArchiveRaw) { r.TransactionHash = "invalid" }},
		{name: "changed payload", edit: func(r *delivery.ArchiveRaw) { r.Data = append(append([]byte(nil), r.Data...), 1) }},
		{name: "raw table", sql: `ALTER TABLE indexer.indexer_generation_raw RENAME TO unavailable_generation`},
		{name: "raw acknowledgment", sql: `CREATE FUNCTION indexer.suppress_generation() RETURNS trigger LANGUAGE plpgsql AS 'BEGIN RETURN NULL; END'; CREATE TRIGGER suppress_generation BEFORE INSERT ON indexer.indexer_generation_raw FOR EACH ROW EXECUTE FUNCTION indexer.suppress_generation()`},
		{name: "outbox table", sql: `ALTER TABLE indexer.delivery_outbox RENAME TO unavailable_delivery`},
		{name: "effect table", sql: `ALTER TABLE indexer.indexer_replay_effects RENAME TO unavailable_effect`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := h.Pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			if tc.sql != "" {
				if _, err = tx.Exec(ctx, tc.sql); err != nil {
					t.Fatal(err)
				}
			}
			raw := input
			if tc.edit != nil {
				tc.edit(&raw)
			}
			if err = projection.ApplyRawTx(ctx, tx, request.Generation, raw); err == nil {
				t.Fatal("unbound generation input acknowledged")
			}
			if err = tx.Rollback(ctx); err != nil {
				t.Fatal(err)
			}
			var count int
			if err = h.Pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM indexer.indexer_generation_raw)+(SELECT count(*) FROM indexer.indexer_replay_effects)`).Scan(&count); err != nil || count != 0 {
				t.Fatalf("rejected generation rows %d %v", count, err)
			}
		})
	}
	tx, err := h.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err = projection.ApplyRawTx(ctx, tx, request.Generation, input); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `UPDATE indexer.indexer_generation_raw SET payload='{}'::jsonb`); err != nil {
		t.Fatal(err)
	}
	if err = projection.ApplyRawTx(ctx, tx, request.Generation, input); !errors.Is(err, store.ErrIntegrity) {
		t.Fatalf("changed generation acknowledged %v", err)
	}
}

func TestMeasurementFilesRejectUnavailableAndUnpinnedInputs(t *testing.T) {
	spec := repairM1000MeasurementSpec(t)
	if err := validateCandidateFiles(nil, spec.CandidateSHA256); err == nil {
		t.Fatal("empty source inventory accepted")
	}
	missing := filepath.Join(t.TempDir(), "absent")
	if err := validateCandidateFiles(map[string]string{missing: strings.Repeat("1", 64)}, spec.CandidateSHA256); !os.IsNotExist(err) {
		t.Fatalf("unavailable source %v", err)
	}
	for _, path := range []string{missing, t.TempDir()} {
		if _, _, err := readTopology(path, strings.Repeat("1", 64)); err == nil {
			t.Fatal("unavailable topology accepted")
		}
	}
	if _, _, err := readTopology(spec.TopologyPath, strings.Repeat("0", 64)); err == nil {
		t.Fatal("unpinned topology accepted")
	}
	path := filepath.Join(t.TempDir(), "checkpoint.json")
	body := []byte(`{}`)
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadRestoreCheckpoint(path, delivery.Hash(body)); err == nil {
		t.Fatal("incomplete checkpoint accepted")
	}
	for _, path := range []string{missing, t.TempDir()} {
		if _, err := loadRestoreCheckpoint(path, strings.Repeat("0", 64)); err == nil {
			t.Fatal("unavailable checkpoint accepted")
		}
	}
	ctx := context.Background()
	h := newLifecycleCoverageHarness(t)
	for _, path := range []string{missing, t.TempDir()} {
		if _, err := h.ObserveInfrastructureFailure(ctx, spec, path, strings.Repeat("0", 64), 1, 1); err == nil {
			t.Fatal("unavailable infrastructure observation accepted")
		}
	}
	if _, err := (*IngestMeasurementHarness)(nil).ObserveInfrastructureFailure(ctx, spec, path, delivery.Hash(body), 1, 1); err == nil || !strings.Contains(err.Error(), "providers unavailable") {
		t.Fatalf("unbound infrastructure providers %v", err)
	}
}
