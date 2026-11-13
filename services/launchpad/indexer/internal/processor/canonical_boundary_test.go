package processor

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/receipt"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/source"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func TestQualifiedCanonicalCommitRejectsUnavailableDurableTables(t *testing.T) {
	ctx := context.Background()
	h := newLifecycleCoverageHarness(t)
	h.inventory.batch(t, ctx, 33963716, 33963716)
	b, receipts, err := h.Processor.deps.QualifiedBatchSource.FetchQualifiedBatch(ctx, 33963717, 33963733)
	if err != nil {
		t.Fatal(err)
	}
	b.StreamID = realStream
	b.Owner = inventoryTestOwner
	b.PreviousHeight = 33963716
	b.Fence = h.Processor.canonicalFence
	b.BatchID = "qualified-table-boundaries"
	for _, table := range []string{"indexer_quarantine", "indexer_token_transfers", "indexer_accounting_fills", "delivery_partition_producers", "delivery_partition_positions", "delivery_outbox", "indexer_ingest_outbox", "indexer_capability_degradation"} {
		t.Run(table, func(t *testing.T) {
			if _, err := h.Pool.Exec(ctx, `ALTER TABLE indexer.`+table+` RENAME TO boundary_unavailable`); err != nil {
				t.Fatal(err)
			}
			defer func() {
				if _, err := h.Pool.Exec(ctx, `ALTER TABLE indexer.boundary_unavailable RENAME TO `+table); err != nil {
					t.Fatal(err)
				}
			}()
			if err := h.Processor.ProcessQualifiedCanonicalBatch(ctx, b, receipts); err == nil {
				t.Fatal("canonical commit acknowledged unavailable durable table")
			}
			height, err := h.Store.GetCanonicalWatermark(ctx, 125, realStream)
			if err != nil || height != 33963716 {
				t.Fatalf("failed projection advanced watermark %d %v", height, err)
			}
		})
	}
	p := h.Processor
	version := p.deps.InventoryProjectionVersion
	p.deps.InventoryProjectionVersion = ""
	if err = p.ProcessQualifiedCanonicalBatch(ctx, b, receipts); err == nil || !strings.Contains(err.Error(), "inventory projection version") {
		t.Fatalf("missing inventory version %v", err)
	}
	p.deps.InventoryProjectionVersion = version
	for _, bad := range [][]*receipt.QualifiedReceipt{{nil}, {new(receipt.QualifiedReceipt)}} {
		if err = p.ProcessQualifiedCanonicalBatch(ctx, b, bad); err == nil {
			t.Fatal("unissued receipt committed")
		}
	}
	if err = p.ProcessQualifiedCanonicalBatch(ctx, b, receipts); err != nil {
		t.Fatal(err)
	}
	height, err := h.Store.GetCanonicalWatermark(ctx, 125, realStream)
	if err != nil || height != 33963733 {
		t.Fatalf("qualified retry watermark %d %v", height, err)
	}
}

func TestSourceQuarantineClassifiesOptionalAndRepeatedRequiredInputs(t *testing.T) {
	ctx, _, pool := replayPostgres(t)
	b := prepare(t, pool)
	p := processorFor(pool)
	emitter := common.HexToAddress(b.Records[0].Emitter)
	registry := sourceRegistry(t, emitter)
	result := source.FetchResult{BlockTimestamps: map[uint64]int64{101: 123456}}
	for _, raw := range b.Records {
		l := types.Log{Address: emitter, Data: raw.Data, BlockNumber: uint64(raw.Height), BlockHash: common.HexToHash(raw.BlockHash), TxHash: common.HexToHash(raw.TransactionHash), TxIndex: uint(raw.TransactionIndex), Index: uint(raw.LogIndex)}
		for _, topic := range raw.Topics {
			l.Topics = append(l.Topics, common.HexToHash(topic))
		}
		result.Logs = append(result.Logs, l)
	}
	p.deps.Quarantine = &QuarantinePolicy{Registry: registry, SourceStream: "optional"}
	if err := p.RetainSourcePoison(ctx, result); err != nil {
		t.Fatalf("valid source input %v", err)
	}
	result.Logs = result.Logs[:1]
	result.Logs[0].Data = []byte{1}
	decoded, _ := registry.Decode(result.Logs[0])
	if decoded == nil || decoded.ABIRevision == "" {
		t.Fatal("approved ABI absent")
	}
	key := strings.ToLower(emitter.Hex()) + ":" + strings.ToLower(result.Logs[0].Topics[0].Hex())
	p.deps.Quarantine.OptionalEmitterTopics = map[string]OptionalFamilyPolicy{key: {ABIRevision: decoded.ABIRevision, Requirement: "optional.telemetry"}}
	for i := 0; i < 2; i++ {
		if err := p.RetainSourcePoison(ctx, result); err != nil {
			t.Fatalf("optional source input retry %d: %v", i, err)
		}
	}
	var required bool
	var attempts int
	if err := pool.QueryRow(ctx, `SELECT required,attempt_count FROM indexer.indexer_quarantine WHERE stream_id='optional'`).Scan(&required, &attempts); err != nil || required || attempts != 1 {
		t.Fatalf("optional classification %v %d %v", required, attempts, err)
	}
	p.deps.Quarantine.OptionalEmitterTopics[key] = OptionalFamilyPolicy{ABIRevision: strings.Repeat("a", 64), Requirement: "optional.telemetry"}
	if err := p.RetainSourcePoison(ctx, result); err == nil {
		t.Fatal("conflicting optional ABI admitted")
	}
	p.deps.Quarantine.OptionalEmitterTopics = nil
	p.deps.Quarantine.SourceStream = "required"
	for i := 0; i < 2; i++ {
		if err := p.RetainSourcePoison(ctx, result); !errors.Is(err, store.ErrRequiredPoison) {
			t.Fatalf("required source retry %d: %v", i, err)
		}
	}
	if err := pool.QueryRow(ctx, `SELECT required,attempt_count FROM indexer.indexer_quarantine WHERE stream_id='required'`).Scan(&required, &attempts); err != nil || !required || attempts != 1 {
		t.Fatalf("required classification %v %d %v", required, attempts, err)
	}
	original := result.Logs[0]
	result.Logs[0].Removed = true
	if err := p.RetainSourcePoison(ctx, result); !errors.Is(err, store.ErrIntegrity) {
		t.Fatalf("removed log retained %v", err)
	}
	result.Logs[0] = original
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `ALTER TABLE indexer.indexer_quarantine RENAME TO quarantine_unavailable`); err != nil {
		t.Fatal(err)
	}
	q := New(Deps{Store: store.NewFromDBTX(tx), ChainID: 125, Quarantine: p.deps.Quarantine})
	if err = q.RetainSourcePoison(ctx, result); err == nil {
		t.Fatal("unavailable quarantine acknowledged")
	}
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	// A retained raw log remains immutable even when another classification is requested.
	result.Logs[0].Data = []byte{2}
	p.deps.Quarantine.SourceStream = "conflicting"
	if err = p.RetainSourcePoison(ctx, result); !errors.Is(err, store.ErrIntegrity) {
		body, _ := json.Marshal(result.Logs[0])
		t.Fatalf("changed raw source admitted %s %v", body, err)
	}
}
