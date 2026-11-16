package app

import (
	"context"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/internaltest/chainfixture"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/receipt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

func TestShellReceiptMarketWalkHeaderCoverage(t *testing.T) {
	for _, name := range chainfixture.ShellCases() {
		t.Run(name, func(t *testing.T) {
			block := chainfixture.NewShellCase(t, name)
			ctx := context.Background()
			client, err := ethclient.DialContext(ctx, block.Serve(t))
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			s := recoveringQualifiedSource{client: client, blockIdentity: receipt.BlockIdentityReported}
			height := block.Header.Number.Uint64()
			read, err := s.readMarketHeight(ctx, height)
			if err != nil {
				t.Fatal(err)
			}
			if len(read.receipts) != len(block.Receipts) || !receipt.IsShellReceipt(read.receipts[len(read.receipts)-1]) {
				t.Fatal("walk did not read shell receipts")
			}
			hints, blocks, err := s.marketAddresses(ctx, height, height)
			if block.Reject {
				if err == nil {
					t.Fatal("invalid receipt coverage admitted market hints")
				}
				return
			}
			if err != nil || len(blocks) != 1 || blocks[height].Hash != block.Hash || blocks[height].TransactionCount != len(block.Transactions) {
				t.Fatalf("market walk blocks=%+v: %v", blocks, err)
			}
			want := map[common.Address]bool{}
			for _, r := range block.Receipts[:len(block.Transactions)] {
				for _, l := range r.Logs {
					want[l.Address] = true
				}
			}
			if len(block.Transactions) == 0 && len(hints) != 0 {
				t.Fatal("shell logs produced market hints")
			}
			for _, address := range hints {
				delete(want, address)
			}
			if len(want) != 0 {
				t.Fatalf("listed log addresses missing from hints: %v", want)
			}
		})
	}
}
