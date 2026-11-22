package receipt

import (
	"context"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/decode"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/internaltest/chainfixture"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
)

func TestBlockContentBoundsShell(t *testing.T) {
	for _, tc := range []struct {
		name                          string
		transactions, logs, dataBytes int
	}{{"transactions", 4097, 1, 0}, {"receipt logs", 1, 4097, 0}, {"log data", 1, 1, (64 << 10) + 1}} {
		t.Run(tc.name, func(t *testing.T) {
			b := chainfixture.NewContentBlock(t, tc.transactions, tc.logs, make([]byte, tc.dataBytes))
			regular, shells, err := SeparateShellReceipts(b.Transactions, b.Receipts, b.Hash, b.Header.Number, b.Header.Bloom, b.Header.ReceiptHash, false)
			if err != nil || len(regular) != tc.transactions || len(shells) != 0 || len(regular[0].Logs) != tc.logs {
				t.Fatalf("block content: receipts=%d shells=%d: %v", len(regular), len(shells), err)
			}
		})
	}
	b := chainfixture.NewContentBlock(t, 1, 1, nil)
	shared := make([]byte, 1<<20)
	base := *b.Receipts[0].Logs[0]
	b.Receipts[0].Logs = make([]*types.Log, decode.MaxBlockLogBytes/(256+len(shared))+1)
	for i := range b.Receipts[0].Logs {
		log := base
		log.Index, log.Data = uint(i), shared
		b.Receipts[0].Logs[i] = &log
	}
	if _, _, err := SeparateShellReceipts(b.Transactions, b.Receipts, b.Hash, b.Header.Number, b.Header.Bloom, b.Header.ReceiptHash, true); !errors.Is(err, ErrQualification) {
		t.Fatalf("block byte bound: %v", err)
	}
}

func TestBlockContentBoundsBundle(t *testing.T) {
	for _, tc := range []struct {
		name            string
		logs, dataBytes int
	}{{"receipt logs", 4097, 0}, {"log data", 1, decode.MaxFieldBytes + 1}, {"raw envelope data", 1, decode.MaxEnvelopeBytes + 1}} {
		t.Run(tc.name, func(t *testing.T) {
			block := chainfixture.NewContentBlock(t, 1, tc.logs, make([]byte, tc.dataBytes))
			b := Bundle{Receipt: block.Receipts[0], Transaction: block.Transactions[0], Header: block.Header, BlockHash: block.Hash}
			if err := boundedBundle(b); err != nil {
				t.Fatal(err)
			}
			if sender, err := coordinates(b); err != nil || sender == "" {
				t.Fatalf("bundle coordinates: %q: %v", sender, err)
			}
		})
	}
	block := chainfixture.NewContentBlock(t, 1, 1, nil)
	b := Bundle{Receipt: block.Receipts[0], Transaction: block.Transactions[0], Header: block.Header, BlockHash: block.Hash}
	shared := make([]byte, 1<<20)
	base := *b.Receipt.Logs[0]
	b.Receipt.Logs = make([]*types.Log, maxBundleBytes/(256+len(shared))+1)
	for i := range b.Receipt.Logs {
		log := base
		log.Index, log.Data = uint(i), shared
		b.Receipt.Logs[i] = &log
	}
	if err := boundedBundle(b); !errors.Is(err, ErrQualification) {
		t.Fatalf("bundle byte bound: %v", err)
	}
	if _, err := coordinates(b); !errors.Is(err, ErrQualification) {
		t.Fatalf("coordinates byte bound: %v", err)
	}
}

func TestBlockContentBoundsPrepaymentReceipts(t *testing.T) {
	block := chainfixture.NewContentBlock(t, 1, 1, nil)
	prior := chainfixture.NewContentBlock(t, 4097, 0, nil)
	b := Bundle{Receipt: block.Receipts[0], Transaction: block.Transactions[0], Header: block.Header, BlockHash: block.Hash, PrepayReceipts: prior.Receipts}
	for _, r := range b.PrepayReceipts {
		r.BlockNumber = new(big.Int).Sub(block.Header.Number, big.NewInt(1))
		r.BlockHash = block.Header.ParentHash
	}
	if err := boundedBundle(b); err != nil {
		t.Fatal(err)
	}
	if _, err := coordinates(b); err != nil {
		t.Fatal(err)
	}
}

func TestBlockContentBoundsRPCBlockReads(t *testing.T) {
	ctx := context.Background()
	b := chainfixture.NewContentBlock(t, 4097, 1, make([]byte, (64<<10)+1))
	client, closeClient, err := NewBoundedRPCClient(ctx, b.Serve(t), "")
	if err != nil {
		t.Fatal(err)
	}
	defer closeClient()
	a := &RPCQualification{client: client, options: RPCOptions{BlockIdentity: BlockIdentityReported, Timeout: 15 * time.Second}}
	raw, err := a.rawBlock(ctx, valueEthGetBlockByNumber, hexutil.EncodeBig(b.Header.Number), false)
	if err != nil || len(raw) <= rpcHeaderResponseBytes {
		t.Fatalf("block response budget: bytes=%d: %v", len(raw), err)
	}
	if _, hash, err := a.headerByNumber(ctx, b.Header.Number); err != nil || hash != b.Hash {
		t.Fatalf("block header read: %v", err)
	}
	block, regular, skipped, err := a.pinnedBlockAtHeight(ctx, b.Header.Number.Uint64(), nil)
	if err != nil || skipped || len(block.Transactions) != 4097 || len(regular) != 4097 || len(regular[0].Logs[0].Data) != (64<<10)+1 {
		t.Fatalf("pinned block content: transactions=%d receipts=%d: %v", len(block.Transactions), len(regular), err)
	}
}
