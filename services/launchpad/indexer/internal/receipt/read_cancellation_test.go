package receipt

import (
	"context"
	"errors"
	"math/big"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

func TestPinnedScalarReadsHonorCancellationAndDecimalCapacity(t *testing.T) {
	n := privateNode(t)
	authority := &RPCQualification{client: n.client, reads: &rpcReadState{calls: make(chan struct{}, 1), decimals: map[string]*rpcDecimalRead{}}}
	address := n.owner
	block := common.HexToHash("0x125")
	ctx, cancel := context.WithCancel(n.ctx)
	cancel()
	for _, read := range []func(context.Context) ([]byte, error){
		func(c context.Context) ([]byte, error) { return authority.codeAt(c, address, block) },
		func(c context.Context) ([]byte, error) { return authority.storageAt(c, address, common.Hash{}, block) },
		func(c context.Context) ([]byte, error) { return authority.word(c, address, "decimals()", block) },
		func(c context.Context) ([]byte, error) { return authority.decimals(c, address, "epoch", block) },
	} {
		if value, err := read(ctx); value != nil || !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled pinned read: %x, %v", value, err)
		}
	}
	for _, read := range []func(context.Context) error{
		func(c context.Context) error { _, _, err := authority.headerByNumber(c, big.NewInt(0)); return err },
		func(c context.Context) error { _, err := authority.headerByHash(c, block); return err },
		func(c context.Context) error { _, err := authority.blockByHash(c, block); return err },
	} {
		if err := read(ctx); !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled block identity read: %v", err)
		}
	}
	if _, err := authority.blockByHash(n.ctx, block); err == nil {
		t.Fatal("unavailable block identity admitted")
	}
	if _, _, err := authority.pinnedBlockAt(n.ctx, block, 0); !errors.Is(err, ErrQualification) {
		t.Fatalf("unavailable pinned block: %v", err)
	}
	authority.reads.calls <- struct{}{}
	bounded, stop := context.WithTimeout(n.ctx, 10*time.Millisecond)
	defer stop()
	if value, err := authority.codeAt(bounded, address, block); value != nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("queued read deadline: %x, %v", value, err)
	}
	if len(authority.reads.calls) != 1 {
		t.Fatal("queued cancellation released an active permit")
	}
	<-authority.reads.calls
	for i := 0; i < 4096; i++ {
		authority.reads.decimals[strconv.Itoa(i)] = &rpcDecimalRead{done: make(chan struct{})}
	}
	if value, err := authority.decimals(n.ctx, address, "next", block); value != nil || !errors.Is(err, ErrQualification) || len(authority.reads.decimals) != 4096 {
		t.Fatalf("decimal cache capacity: %x, %v", value, err)
	}
	key := strings.ToLower(address.Hex()) + "/waiting"
	authority.reads.decimals = map[string]*rpcDecimalRead{key: {done: make(chan struct{})}}
	waiting, stopWaiting := context.WithTimeout(n.ctx, 10*time.Millisecond)
	defer stopWaiting()
	if value, err := authority.decimals(waiting, address, "waiting", block); value != nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("coalesced read deadline: %x, %v", value, err)
	}
}
