package processor

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum"
)

func TestCreationSnapshotBackfillRepairsMissingCreationState(t *testing.T) {
	h := newCreationSnapshotHarness(t, false)
	h.commit(t)
	ctx := h.node.ctx
	if _, err := h.pool.Exec(ctx, `DELETE FROM indexer.pool_state_snapshots`); err != nil {
		t.Fatal(err)
	}
	h.proc.canonicalRenewed = time.Time{}
	if err := h.proc.backfillCreationSnapshots(ctx); err != nil {
		t.Fatal(err)
	}
	h.assertCreation(t, h.authority, h.created.BlockHash)
	var state string
	if err := h.pool.QueryRow(ctx, `SELECT status FROM indexer.backfill_jobs WHERE id='pool-snapshot:'||lower($1)`, h.address.Hex()).Scan(&state); err != nil || state != "completed" {
		t.Fatalf("backfill state: %s, %v", state, err)
	}
	if err := h.proc.backfillCreationSnapshots(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := h.pool.Exec(ctx, `DELETE FROM indexer.pool_state_snapshots; UPDATE indexer.pools SET created_block=created_block+1000`); err != nil {
		t.Fatal(err)
	}
	if err := h.proc.backfillCreationSnapshots(ctx); err != nil {
		t.Fatal(err)
	}
	if err := h.pool.QueryRow(ctx, `SELECT status FROM indexer.backfill_jobs WHERE id='pool-snapshot:'||lower($1)`, h.address.Hex()).Scan(&state); err != nil || state != "pending" {
		t.Fatalf("unavailable creation state: %s, %v", state, err)
	}
	noReader := *h.proc
	noReader.deps.Reserves = nil
	if err := noReader.backfillCreationSnapshots(ctx); err == nil {
		t.Fatal("missing reserve reader admitted")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if err := h.proc.backfillCreationSnapshots(canceled); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled snapshot pass: %v", err)
	}
}

func TestCreationStateAvailabilityClassification(t *testing.T) {
	for _, message := range []string{"missing trie node", "historical state unavailable", "state unavailable", "state is not available", "state not available", "header not found", "block not found"} {
		if !creationStateUnavailable(errors.New(message)) {
			t.Fatalf("unavailable creation state: %s", message)
		}
	}
	if !creationStateUnavailable(ethereum.NotFound) {
		t.Fatal("missing chain state not classified")
	}
	for _, err := range []error{nil, context.Canceled, context.DeadlineExceeded, errors.New("invalid reserve data")} {
		if creationStateUnavailable(err) {
			t.Fatalf("available or canceled state classified unavailable: %v", err)
		}
	}
}
