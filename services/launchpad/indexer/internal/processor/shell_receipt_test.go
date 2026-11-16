package processor

import (
	"context"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/internaltest/chainfixture"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/receipt"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store"
)

func TestShellReceiptProcessorLeavesSetAsideLogsOutOfReplay(t *testing.T) {
	block := chainfixture.NewShellBlock(t, false)
	regular, shells, err := receipt.SeparateShellReceipts(block.Transactions, block.Receipts, block.Hash, block.Header.Number, block.Header.Bloom, block.Header.ReceiptHash, true)
	if err != nil || len(regular) != 0 || len(shells) != 1 || len(shells[0].Logs) == 0 {
		t.Fatalf("shell block receipt coverage: %v", err)
	}
	shell := shells[0]
	if !receipt.IsShellReceipt(shell) {
		t.Fatal("shell receipt marker was not recognized")
	}
	batch := store.CanonicalBatch{ChainID: 125, Blocks: []store.BlockCompleteness{{Height: block.Header.Number.Int64(), Hash: block.Hash.Hex(), RawCount: 0, FetchComplete: true, RequiredDecodeComplete: true}}}
	segment, err := qualifiedSegment(context.Background(), "shell", batch, nil)
	if err != nil || len(segment.Records) != 0 || len(segment.Events) != 0 {
		t.Fatalf("shell receipt entered replay projection: %+v, %v", segment, err)
	}
}
