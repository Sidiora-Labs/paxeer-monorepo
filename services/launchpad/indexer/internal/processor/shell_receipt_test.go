package processor

import (
	"context"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/receipt"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store"
	"github.com/ethereum/go-ethereum/core/types"
)

func TestShellReceiptProcessorLeavesSetAsideLogsOutOfReplay(t *testing.T) {
	shell := &types.Receipt{Type: receipt.ShellReceiptType}
	if !receipt.IsShellReceipt(shell) {
		t.Fatal("shell receipt marker was not recognized")
	}
	batch := store.CanonicalBatch{ChainID: 125, Blocks: []store.BlockCompleteness{{Height: 7, Hash: "0x" + strings.Repeat("1", 64), RawCount: 0, FetchComplete: true, RequiredDecodeComplete: true}}}
	segment, err := qualifiedSegment(context.Background(), "shell", batch, nil)
	if err != nil || len(segment.Records) != 0 || len(segment.Events) != 0 {
		t.Fatalf("shell receipt entered replay projection: %+v, %v", segment, err)
	}
}
