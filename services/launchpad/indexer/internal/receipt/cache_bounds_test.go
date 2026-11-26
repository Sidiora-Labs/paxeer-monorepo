package receipt

import (
	"errors"
	"math/big"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func TestMultiBlockReadBoundsApplyToRetainedReceiptMaterial(t *testing.T) {
	n := privateNode(t)
	_, token, _ := n.deploy(t, "SidioraERC20", "Read bounds", "READ", big.NewInt(1000), n.owner)
	tx, err := token.Transact(n.auth, "transfer", n.owner, big.NewInt(1))
	if err != nil {
		t.Fatal(err)
	}
	mined := n.mined(t, tx)
	ctx := n.ctx
	a := &RPCQualification{client: n.client, options: RPCOptions{Timeout: time.Second}, reads: &rpcReadState{}}
	height := mined.BlockNumber.Uint64()
	reads, err := a.ReadBatch(ctx, height-1, height)
	if err != nil || len(reads) != 2 || reads[0].Height != height-1 || reads[0].Hash != reads[1].ParentHash || reads[1].Height != height || reads[1].Hash != mined.BlockHash || len(reads[1].Receipts) != 1 || !receiptEqual(reads[1].Receipts[0], mined) {
		t.Fatalf("ordered block reads: %+v, %v", reads, err)
	}
	for _, tc := range []struct {
		name string
		edit func(*cachedBlock)
	}{
		{"nil receipt", func(c *cachedBlock) { c.receipts[0] = nil }},
		{"nil log", func(c *cachedBlock) { c.receipts[0].Logs[0] = nil }},
		{"transaction limit", func(c *cachedBlock) {
			c.block.Transactions = make(types.Transactions, 4097)
			for i := range c.block.Transactions {
				c.block.Transactions[i] = tx
			}
		}},
		{"log limit", func(c *cachedBlock) {
			log := c.receipts[0].Logs[0]
			c.receipts[0].Logs = make([]*types.Log, 65537)
			for i := range c.receipts[0].Logs {
				c.receipts[0].Logs[i] = log
			}
		}},
		{"byte limit", func(c *cachedBlock) { c.receipts[0].Logs[0].Data = make([]byte, 32<<20) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			retained, err := clone(reads[1].Receipts)
			if err != nil {
				t.Fatal(err)
			}
			cached := cachedBlock{block: pinnedBlock{Header: reads[1].Header, Hash: reads[1].Hash, Transactions: reads[1].Transactions}, receipts: retained}
			tc.edit(&cached)
			a.reads.storeHeight(height, cached)
			if got, err := a.ReadBatch(ctx, height-1, height); got != nil || !errors.Is(err, ErrQualification) {
				t.Fatalf("unbounded retained read: %v, %v", got, err)
			}
		})
	}
	// A fresh reader must recover the same complete material from the source.
	a.reads = &rpcReadState{}
	recovered, err := a.ReadBatch(ctx, height-1, height)
	if err != nil {
		t.Fatal(err)
	}
	expectedDigest, err := digest(reads)
	if err != nil {
		t.Fatal(err)
	}
	actualDigest, err := digest(recovered)
	if err != nil || actualDigest != expectedDigest {
		t.Fatalf("source material differs: digest=%x expected=%x, %v", actualDigest, expectedDigest, err)
	}
	if _, err := a.ReadBatch(ctx, height+1, height+2); err == nil {
		t.Fatal("unavailable block range admitted")
	}
}

func TestPinnedCachesEvictAtCapacityAndRetainBlockIdentity(t *testing.T) {
	var absent *rpcReadState
	if _, ok := absent.height(1); ok {
		t.Fatal("absent height cache returned a block")
	}
	absent.storeHeight(1, cachedBlock{})
	state := &rpcReadState{}
	value := cachedBlock{block: pinnedBlock{Header: &types.Header{Number: big.NewInt(1)}, Hash: common.HexToHash("0x125")}}
	for i := 0; i < pinnedCacheEntries; i++ {
		state.storeCode(strconv.Itoa(i), "bound-code-epoch")
		state.storeHeight(uint64(i), value)
	}
	if got, ok := state.code("0"); !ok || got != "bound-code-epoch" {
		t.Fatalf("code cache before rollover: %q, %v", got, ok)
	}
	state.storeCode("next", "next-code-epoch")
	state.storeHeight(pinnedCacheEntries, value)
	if _, ok := state.code("0"); ok {
		t.Fatal("code cache exceeded its entry capacity")
	}
	if got, ok := state.code("next"); !ok || got != "next-code-epoch" || len(state.codeEpochs) != 1 {
		t.Fatalf("code cache rollover: %q, %v", got, ok)
	}
	if _, ok := state.height(0); ok {
		t.Fatal("height cache exceeded its entry capacity")
	}
	if got, ok := state.height(pinnedCacheEntries); !ok || !reflect.DeepEqual(got, value) || len(state.heights) != 1 {
		t.Fatalf("height cache rollover: %+v, %v", got, ok)
	}
	for i := int64(0); i < 256; i++ {
		state.storeBlock(common.BigToHash(big.NewInt(i)), value)
	}
	state.storeBlock(value.block.Hash, value)
	if _, ok := state.block(common.Hash{}); ok {
		t.Fatal("block cache exceeded its entry capacity")
	}
	if got, ok := state.block(value.block.Hash); !ok || !reflect.DeepEqual(got, value) || len(state.blocks) != 1 {
		t.Fatalf("block cache rollover: %+v, %v", got, ok)
	}
}
