package store

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store/canonicaltest"
)

func TestBackfillIntentsCommitOnceAndRejectChangedPayload(t *testing.T) {
	ctx := context.Background()
	_, pool := canonicaltest.New(t)
	st := New(pool)
	fence, err := st.AcquireLease(ctx, 125, "backfill-intents", "worker", 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO indexer.indexer_ingest_watermarks VALUES(125,'backfill-intents',100,100)`); err != nil {
		t.Fatal(err)
	}
	hash := "0x" + strings.Repeat("1", 64)
	transaction := "0x" + strings.Repeat("2", 64)
	batch := CanonicalBatch{ChainID: 125, StreamID: "backfill-intents", Owner: "worker", BatchID: "block101", Fence: fence, PreviousHeight: 100, Blocks: []BlockCompleteness{{Height: 101, Hash: hash, RawCount: 1, FetchComplete: true, RequiredDecodeComplete: true}}, Records: []RawRecord{{Height: 101, BlockHash: hash, TransactionHash: transaction, Emitter: "0x" + strings.Repeat("3", 40), Topics: []string{}, Data: []byte{1}}}}
	intents := []OutboxIntent{{TransactionHash: transaction, Kind: "Swap", Payload: json.RawMessage(`{"amount":"1"}`)}}
	if err := (CanonicalTx{}).InsertBackfillOutboxIntents(ctx, batch, intents); !errors.Is(err, ErrIntegrity) {
		t.Fatalf("missing transaction: %v", err)
	}
	err = st.CommitProjectedCanonicalTx(ctx, batch, func(ctx context.Context, canonical CanonicalTx) ([]OutboxIntent, error) {
		for range 2 {
			if err := canonical.InsertBackfillOutboxIntents(ctx, batch, intents); err != nil {
				return nil, err
			}
		}
		return intents, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"changed payload", "supplied mismatch", "invalid batch", "canceled"} {
		t.Run(mode, func(t *testing.T) {
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = tx.Rollback(ctx) }()
			canonical := CanonicalTx{Tx: tx}
			candidate := batch
			changed := append([]OutboxIntent(nil), intents...)
			changed[0].Payload = json.RawMessage(`{"amount":"2"}`)
			callCtx := ctx
			switch mode {
			case "supplied mismatch":
				candidate.Outbox = intents
			case "invalid batch":
				candidate.ChainID = 0
			case "canceled":
				var cancel context.CancelFunc
				callCtx, cancel = context.WithCancel(ctx)
				cancel()
			}
			err = canonical.InsertBackfillOutboxIntents(callCtx, candidate, changed)
			if mode == "changed payload" || mode == "supplied mismatch" {
				if !errors.Is(err, ErrIntegrity) {
					t.Fatalf("immutable intent: %v", err)
				}
			} else if mode == "canceled" {
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("canceled intent: %v", err)
				}
			} else if err == nil || err.Error() != "invalid canonical batch bounds" {
				t.Fatalf("invalid batch: %v", err)
			}
		})
	}
	var height int64
	var count int
	var payload []byte
	if err := pool.QueryRow(ctx, `SELECT raw_complete_height,(SELECT count(*) FROM indexer.indexer_ingest_outbox),(SELECT payload FROM indexer.indexer_ingest_outbox) FROM indexer.indexer_ingest_watermarks WHERE stream_id='backfill-intents'`).Scan(&height, &count, &payload); err != nil {
		t.Fatal(err)
	}
	var values map[string]string
	if err := json.Unmarshal(payload, &values); err != nil || height != 101 || count != 1 || values["amount"] != "1" {
		t.Fatalf("durable intent: height=%d count=%d payload=%s error=%v", height, count, payload, err)
	}
}
