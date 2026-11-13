package app

import (
	"context"
	"fmt"
	"math"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/receipt"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/source"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store/canonicaltest"
	"github.com/ethereum/go-ethereum/common"
)

func TestRuntimeIntegerRefusals(t *testing.T) {
	ctx := context.Background()
	for _, chainID := range []int{math.MinInt32 - 1, math.MaxInt32 + 1} {
		t.Run(fmt.Sprintf("chain %d", chainID), func(t *testing.T) {
			cfg := config.Config{}
			cfg.ChainID = chainID
			got, err := New(ctx, cfg, nil)
			want := fmt.Sprintf("integer %d is outside the range of int32", chainID)
			if got != nil || err == nil || err.Error() != want {
				t.Fatalf("New chain %d = %v, %v; want %q", chainID, got, err, want)
			}
		})
	}
	in := renderInputs(t)
	for _, height := range []int64{math.MinInt64, -1} {
		t.Run(fmt.Sprintf("checkpoint %d", height), func(t *testing.T) {
			lifecycle := lifecycleRuntimeDocument{
				InitialCheckpointHeight: height,
				Committed: source.CommittedOptions{Anchor: &source.CommittedAnchor{
					Height: in.Anchor.Height, Hash: in.Anchor.Hash,
				}},
			}
			err := bindRuntimeAnchor(&lifecycle, receipt.RPCOptions{Anchor: &in.Anchor})
			want := fmt.Sprintf("integer %d is outside the range of uint64", height)
			if err == nil || err.Error() != want {
				t.Fatalf("bindRuntimeAnchor = %v; want %q", err, want)
			}
		})
	}
	_, pool := canonicaltest.New(t)
	parameters := DefaultDeliveryParameters()
	parameters.MaxDeliveryLagMS = 60000
	for _, height := range []uint64{math.MaxInt64 + 1, math.MaxUint64} {
		t.Run(fmt.Sprintf("anchor %d", height), func(t *testing.T) {
			want := fmt.Sprintf("integer %d is outside the range of int64", height)
			anchor := in.Anchor
			anchor.Height = height
			if err := attestDurableAnchor(ctx, pool, "canonical", anchor); err == nil || err.Error() != want {
				t.Fatalf("attestDurableAnchor = %v; want %q", err, want)
			}
			if hash, err := durableBlockHash(ctx, pool, "canonical", height, lifecycleRuntimeDocument{}); hash != (common.Hash{}) || err == nil || err.Error() != want {
				t.Fatalf("durableBlockHash = %s, %v; want %q", hash, err, want)
			}
			input := in
			input.Anchor = anchor
			if documents, err := RenderRuntimeDocuments(input, parameters); documents != nil || err == nil || err.Error() != want {
				t.Fatalf("RenderRuntimeDocuments = %v, %v; want %q", documents, err, want)
			}
		})
	}
}
