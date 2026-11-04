package blockdbtest

import (
	"context"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/ledger_db/block"
)

func TestRepairM_1159_MemoryBuilderInstancesAreIndependent(t *testing.T) {
	for _, samePath := range []bool{true, false} {
		name := "different_paths"
		if samePath {
			name = "same_path"
		}
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			backend := newMemBlockDBBuilder()
			requireTrue(t, backend.ephemeral, "memory backend must declare ephemeral semantics")
			firstPath := t.TempDir()
			secondPath := firstPath
			if !samePath {
				secondPath = t.TempDir()
			}
			first, err := backend.builder(firstPath)
			requireNoError(t, err)
			t.Cleanup(func() { requireNoError(t, first.Close(ctx)) })
			original := makeBlock(11, 3)
			requireNoError(t, first.WriteBlock(ctx, original))
			requireNoError(t, first.Flush(ctx))
			requireRepairM1159Block(t, ctx, first, original)

			second, err := backend.builder(secondPath)
			requireNoError(t, err)
			t.Cleanup(func() { requireNoError(t, second.Close(ctx)) })
			requireTrue(t, first != second, "builder reused an existing instance")
			requireBlockAbsent(t, ctx, second, original)
			requireEmptyBlockDB(t, ctx, second)

			other := makeBlock(22, 2)
			requireNoError(t, second.WriteBlock(ctx, other))
			requireNoError(t, second.Flush(ctx))
			requireRepairM1159Block(t, ctx, second, other)
			requireBlockAbsent(t, ctx, first, other)
			requireRepairM1159Block(t, ctx, first, original)
		})
	}
}

func TestRepairM_1159_MemoryReopenIsEphemeral(t *testing.T) {
	ctx := context.Background()
	backend := newMemBlockDBBuilder()
	requireTrue(t, backend.ephemeral, "memory backend must declare ephemeral semantics")
	path := t.TempDir()
	first, err := backend.builder(path)
	requireNoError(t, err)
	closed := false
	t.Cleanup(func() {
		if !closed {
			requireNoError(t, first.Close(ctx))
		}
	})
	original := makeBlock(7, 4)
	requireNoError(t, first.WriteBlock(ctx, original))
	requireNoError(t, first.Flush(ctx))
	requireRepairM1159Block(t, ctx, first, original)
	requireNoError(t, first.Close(ctx))
	closed = true

	reopened, err := backend.builder(path)
	requireNoError(t, err)
	t.Cleanup(func() { requireNoError(t, reopened.Close(ctx)) })
	requireTrue(t, first != reopened, "reopen returned the closed instance")
	requireBlockAbsent(t, ctx, reopened, original)
	requireEmptyBlockDB(t, ctx, reopened)

	next := makeBlock(8, 2)
	requireNoError(t, reopened.WriteBlock(ctx, next))
	requireNoError(t, reopened.Flush(ctx))
	requireRepairM1159Block(t, ctx, reopened, next)
	requireBlockAbsent(t, ctx, reopened, original)
	lowest, err := reopened.GetLowestBlockHeight(ctx)
	requireNoError(t, err)
	requireTrue(t, lowest == next.Height, "lowest height: expected %d, got %d", next.Height, lowest)
	highest, err := reopened.GetHighestBlockHeight(ctx)
	requireNoError(t, err)
	requireTrue(t, highest == next.Height, "highest height: expected %d, got %d", next.Height, highest)
}

func TestRepairM_1159_ReopenContracts(t *testing.T) {
	t.Run("close_and_reopen", TestCloseAndReopen)
	t.Run("close_and_reopen_then_write", TestCloseAndReopenThenWrite)
}

func requireRepairM1159Block(t *testing.T, ctx context.Context, db block.BlockDB, expected *block.BinaryBlock) {
	t.Helper()
	byHeight, ok, err := db.GetBlockByHeight(ctx, expected.Height)
	requireNoError(t, err)
	requireTrue(t, ok && byHeight != nil, "missing block at height %d", expected.Height)
	requireBlockEqual(t, expected, byHeight)
	byHash, ok, err := db.GetBlockByHash(ctx, expected.Hash)
	requireNoError(t, err)
	requireTrue(t, ok && byHash != nil, "missing block with hash %x", expected.Hash)
	requireBlockEqual(t, expected, byHash)
	for _, tx := range expected.Transactions {
		got, ok, err := db.GetTransactionByHash(ctx, tx.Hash)
		requireNoError(t, err)
		requireTrue(t, ok && got != nil, "missing transaction with hash %x", tx.Hash)
		requireBytesEqual(t, tx.Hash, got.Hash, "transaction hash")
		requireBytesEqual(t, tx.Transaction, got.Transaction, "transaction data")
	}
}
