package app

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/internaltest/chainfixture"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/receipt"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

func contentMarketRead(b *chainfixture.ShellBlock) marketHeightRead {
	return marketHeightRead{header: *b.Header, hash: b.Hash, transactions: b.Transactions, receipts: b.Receipts}
}

func TestBlockContentBoundsMarketRange(t *testing.T) {
	ctx := context.Background()
	s := recoveringQualifiedSource{blockIdentity: receipt.BlockIdentityReported}
	b := chainfixture.NewContentBlock(t, 4097, 65537, nil)
	height := b.Header.Number.Uint64()
	hints, blocks, err := s.validateMarketReads(ctx, height, height, []marketHeightRead{contentMarketRead(b)})
	if err != nil || len(hints) != 1 || len(blocks) != 1 || blocks[height].TransactionCount != 4097 {
		t.Fatalf("single block market content: hints=%d blocks=%d: %v", len(hints), len(blocks), err)
	}
	for _, mode := range []string{"transactions", "logs", "bytes"} {
		t.Run(mode, func(t *testing.T) {
			first := b
			if mode != "transactions" {
				first = chainfixture.NewContentBlock(t, 1, 1, nil)
				logCount := 32769
				var data []byte
				if mode == "bytes" {
					logCount, data = 17, make([]byte, 1<<20)
				}
				base := *first.Receipts[0].Logs[0]
				first.Receipts[0].Logs = make([]*types.Log, logCount)
				for i := range first.Receipts[0].Logs {
					log := base
					log.Index, log.Data = uint(i), data
					first.Receipts[0].Logs[i] = &log
				}
			}
			second := chainfixture.NewContentBlock(t, 1, 0, nil)
			second.Header.Number = new(big.Int).Add(first.Header.Number, big.NewInt(1))
			second.Header.ParentHash = first.Hash
			second.Hash[0] ^= 1
			if mode != "transactions" {
				r := *first.Receipts[0]
				r.BlockHash, r.BlockNumber = second.Hash, second.Header.Number
				r.TxHash = second.Transactions[0].Hash()
				r.Logs = make([]*types.Log, len(first.Receipts[0].Logs))
				for i, base := range first.Receipts[0].Logs {
					log := *base
					log.BlockNumber, log.BlockHash, log.TxHash = second.Header.Number.Uint64(), second.Hash, r.TxHash
					r.Logs[i] = &log
				}
				second.Receipts = types.Receipts{&r}
				second.Header.Bloom = first.Header.Bloom
			}
			if _, _, err := s.validateMarketReads(ctx, height, height+1, []marketHeightRead{contentMarketRead(first), contentMarketRead(second)}); !errors.Is(err, receipt.ErrQualification) {
				t.Fatalf("multi-block %s bound: %v", mode, err)
			}
		})
	}
}

func TestBlockContentBoundsMarketReadStopsAtRangeBudget(t *testing.T) {
	b := chainfixture.NewContentBlock(t, 4097, 1, nil)
	client, err := ethclient.DialContext(context.Background(), b.Serve(t))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	s := recoveringQualifiedSource{client: client, blockIdentity: receipt.BlockIdentityReported}
	height := b.Header.Number.Uint64()
	if _, err := s.readMarketHeights(context.Background(), height, height+2, 1); !errors.Is(err, receipt.ErrQualification) {
		t.Fatalf("market read range budget: %v", err)
	}
}
