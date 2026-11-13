package source_test

import (
	"context"
	"errors"
	"math"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/source"
	"github.com/ethereum/go-ethereum/common"
)

func TestLiveQualificationRequiresTheDurableParentAndOpenSource(t *testing.T) {
	f := repair56RPCSetup(t)
	rpc, authority, _ := f.openRPC(t)
	if f.committed.Anchor == nil || f.committed.Anchor.Height != repair56Creation-1 {
		t.Fatal("recorded parent anchor absent")
	}
	parent := f.committed.Anchor.Hash
	live := &source.LiveQualifiedSource{RPC: rpc, Authority: authority, DurableHeight: func() uint64 { return repair56Creation - 1 }, PriorHash: func(context.Context, uint64) (common.Hash, error) { return parent, nil }}
	b, q, err := live.FetchQualifiedBatch(f.ctx, repair56Creation, repair56Creation)
	if err != nil || len(b.Blocks) != 1 || len(q) != 1 || b.Blocks[0].Height != int64(repair56Creation) {
		t.Fatalf("durable parent qualification %+v %d %v", b, len(q), err)
	}
	live.PriorHash = func(context.Context, uint64) (common.Hash, error) { return common.Hash{}, nil }
	if _, _, err = live.FetchQualifiedBatch(f.ctx, repair56Creation, repair56Creation); err == nil {
		t.Fatal("different durable parent admitted")
	}
	live.PriorHash = func(context.Context, uint64) (common.Hash, error) { return common.Hash{}, context.Canceled }
	if _, _, err = live.FetchQualifiedBatch(f.ctx, repair56Creation, repair56Creation); !errors.Is(err, context.Canceled) {
		t.Fatalf("unavailable durable parent %v", err)
	}
	for _, bounds := range [][2]uint64{{2, 1}, {0, 64}, {math.MaxUint64, math.MaxUint64}} {
		if _, _, err = live.FetchQualifiedBatch(f.ctx, bounds[0], bounds[1]); err == nil {
			t.Fatalf("unbounded live range %v", bounds)
		}
	}
	live.DurableHeight = func() uint64 { return math.MaxUint64 }
	if _, _, err = live.FetchQualifiedBatch(f.ctx, repair56Creation, repair56Creation); err == nil {
		t.Fatal("overflowed durable checkpoint admitted")
	}
	live.DurableHeight = func() uint64 { return repair56Creation - 1 }
	history := &source.HistoricalQualifiedSource{History: &source.HistoricalSource{RPC: rpc}, Authority: authority, DurableHeight: live.DurableHeight}
	for _, bounds := range [][2]uint64{{2, 1}, {0, 64}, {math.MaxUint64, math.MaxUint64}} {
		if _, _, err = history.FetchQualifiedBatch(f.ctx, bounds[0], bounds[1]); err == nil {
			t.Fatalf("unbounded historical range %v", bounds)
		}
	}
	if err = rpc.Close(); err != nil {
		t.Fatal(err)
	}
	if _, _, err = live.FetchQualifiedBatch(f.ctx, repair56Creation, repair56Creation); err == nil {
		t.Fatal("closed live source admitted")
	}
	if _, _, err = history.FetchQualifiedBatch(f.ctx, repair56Creation, repair56Creation); err == nil {
		t.Fatal("closed historical source admitted")
	}
}
