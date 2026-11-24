package app

import (
	"context"
	"errors"
	"math"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/internaltest/chainfixture"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/receipt"
	"github.com/ethereum/go-ethereum/ethclient"
)

func TestMarketReadsRejectUnboundedRanges(t *testing.T) {
	ctx := context.Background()
	s := recoveringQualifiedSource{}
	for _, bounds := range [][2]uint64{
		{1, 0}, {0, 64}, {0, math.MaxUint64},
		{math.MaxUint64, 0}, {math.MaxUint64 - 64, math.MaxUint64},
	} {
		reads, err := s.readMarketHeights(ctx, bounds[0], bounds[1], 1)
		if reads != nil || !errors.Is(err, receipt.ErrQualification) {
			t.Fatalf("read range %v = %v, %v", bounds, reads, err)
		}
		addresses, blocks, err := s.validateMarketReads(ctx, bounds[0], bounds[1], nil)
		if addresses != nil || blocks != nil || !errors.Is(err, receipt.ErrBinding) {
			t.Fatalf("validate range %v = %v, %v, %v", bounds, addresses, blocks, err)
		}
	}
	fixture := chainfixture.New(t)
	client, err := ethclient.DialContext(ctx, fixture.Serve(t))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	s.client = client
	s.blockIdentity = receipt.BlockIdentityReported
	const height = 33963716
	reads, err := s.readMarketHeights(ctx, height, height, 1)
	if err != nil || len(reads) != 1 {
		t.Fatalf("recorded height = %d reads, %v", len(reads), err)
	}
	addresses, blocks, err := s.validateMarketReads(ctx, height, height, reads)
	if err != nil || len(addresses) == 0 || len(blocks) != 1 || blocks[height].Height != height {
		t.Fatalf("recorded market = %v, %v, %v", addresses, blocks, err)
	}
}
