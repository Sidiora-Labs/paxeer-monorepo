package receipt

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/decode"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func TestPinnedReceiptReadersRejectCancellationAndMissingState(t *testing.T) {
	input := repair53InputsFromProducer(t)
	ctx := context.Background()
	authority, closeClient := repair53Open(t, ctx, input)
	defer closeClient()
	bundle, err := authority.Bundle(ctx, input.NativeSwap)
	if err != nil {
		t.Fatal(err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	for _, tc := range []struct {
		name string
		call func() error
	}{
		{"block number", func() error { _, _, e := authority.headerByNumber(canceled, bundle.Header.Number); return e }},
		{"block hash", func() error { _, e := authority.headerByHash(canceled, bundle.BlockHash); return e }},
		{"full block", func() error { _, e := authority.blockByHash(canceled, bundle.BlockHash); return e }},
		{"block pin", func() error { _, _, e := authority.pinnedBlockAt(canceled, bundle.BlockHash, 0); return e }},
		{"bundle", func() error { _, e := authority.Bundle(canceled, input.NativeSwap); return e }},
		{"market recovery", func() error { return authority.Rehydrate(canceled, input.Markets) }},
		{"range", func() error {
			_, _, e := authority.FetchQualifiedBatch(canceled, bundle.Header.Number.Uint64(), bundle.Header.Number.Uint64())
			return e
		}},
		{"code", func() error { _, e := authority.codeAt(canceled, input.Markets[0].Token, bundle.BlockHash); return e }},
		{"storage", func() error {
			_, e := authority.storageAt(canceled, input.Markets[0].Pool, common.Hash{}, bundle.BlockHash)
			return e
		}},
		{"decimal", func() error {
			_, e := authority.decimals(canceled, input.Markets[0].Token, "epoch", bundle.BlockHash)
			return e
		}},
		{"word", func() error {
			_, e := authority.word(canceled, input.Markets[0].Token, "decimals()", bundle.BlockHash)
			return e
		}},
		{"address", func() error {
			_, e := authority.address(canceled, input.Markets[0].Token, "factory()", bundle.BlockHash)
			return e
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if e := tc.call(); e == nil {
				t.Fatal("cancelled source read accepted")
			}
		})
	}
	for _, a := range []*RPCQualification{nil, {}, {reads: &rpcReadState{}}} {
		if _, _, err := a.scalarRead(ctx); !errors.Is(err, ErrQualification) {
			t.Fatalf("missing read state: %v", err)
		}
		if _, err := a.decimals(ctx, input.Markets[0].Token, "epoch", bundle.BlockHash); !errors.Is(err, ErrQualification) {
			t.Fatalf("missing decimal state: %v", err)
		}
	}
	for _, market := range []DiscoveredMarket{{Pool: input.Markets[0].Pool}, {Token: input.Markets[0].Token}, {Token: input.Markets[0].Pool, Pool: input.Markets[0].Pool}, {Token: input.Markets[0].Token, Pool: input.Markets[0].Pool, Height: math.MaxUint64}} {
		if err = authority.Rehydrate(ctx, []DiscoveredMarket{market}); !errors.Is(err, ErrQualification) {
			t.Fatalf("invalid market recovered: %v", err)
		}
	}
	if err = authority.Rehydrate(ctx, make([]DiscoveredMarket, 2049)); !errors.Is(err, ErrQualification) {
		t.Fatalf("unbounded market inventory: %v", err)
	}
	if err = authority.Rehydrate(ctx, nil); err != nil {
		t.Fatal(err)
	}
	for _, o := range []RPCOptions{{}, {Discovery: append(input.Options.Discovery, input.Options.Discovery...)}} {
		if _, err = authority.discoveryEpoch(o, input.Markets[0].Pool, valueSidioraPool, decode.Order{}); !errors.Is(err, ErrQualification) {
			t.Fatalf("ambiguous discovery: %v", err)
		}
	}
	for _, tc := range []struct {
		address common.Address
		epoch   string
		block   common.Hash
	}{{common.Address{}, "epoch", bundle.BlockHash}, {input.Markets[0].Token, "", bundle.BlockHash}, {input.Markets[0].Token, "epoch", common.Hash{}}} {
		if _, err = authority.decimals(ctx, tc.address, tc.epoch, tc.block); !errors.Is(err, ErrQualification) {
			t.Fatalf("unbound decimals: %v", err)
		}
	}
	saturated := &RPCQualification{options: RPCOptions{Timeout: time.Second}, reads: &rpcReadState{calls: make(chan struct{}, 1), decimals: map[string]*rpcDecimalRead{}}}
	for i := 0; i < 4096; i++ {
		saturated.reads.decimals[new(big.Int).SetInt64(int64(i)).String()] = &rpcDecimalRead{}
	}
	if _, err = saturated.decimals(ctx, input.Markets[0].Token, "epoch", bundle.BlockHash); !errors.Is(err, ErrQualification) {
		t.Fatalf("decimal bound: %v", err)
	}
}

func TestBlockParsingBindsReportedIdentityAndSignedTransactionCoordinates(t *testing.T) {
	input := repair53InputsFromProducer(t)
	a, closeClient := repair53Open(t, context.Background(), input)
	defer closeClient()
	bundle, err := a.Bundle(context.Background(), input.NativeSwap)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := a.rawBlock(context.Background(), valueEthGetBlockByHash, bundle.BlockHash, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		edit func(map[string]json.RawMessage)
	}{
		{"reported hash", func(m map[string]json.RawMessage) { delete(m, "hash") }},
		{"zero hash", func(m map[string]json.RawMessage) { m["hash"], _ = json.Marshal(common.Hash{}) }},
		{"number", func(m map[string]json.RawMessage) { m["number"] = json.RawMessage(`"-1"`) }},
		{"transaction encoding", func(m map[string]json.RawMessage) { m["transactions"] = json.RawMessage(`[{}]`) }},
		{"transaction count", func(m map[string]json.RawMessage) { m["transactions"], _ = json.Marshal(make([]any, 4097)) }},
		{"transaction hash", func(m map[string]json.RawMessage) {
			var txs []map[string]json.RawMessage
			_ = json.Unmarshal(m["transactions"], &txs)
			txs[0]["hash"], _ = json.Marshal(common.HexToHash("0x1"))
			m["transactions"], _ = json.Marshal(txs)
		}},
		{"transaction block", func(m map[string]json.RawMessage) {
			var txs []map[string]json.RawMessage
			_ = json.Unmarshal(m["transactions"], &txs)
			txs[0]["blockHash"], _ = json.Marshal(common.HexToHash("0x1"))
			m["transactions"], _ = json.Marshal(txs)
		}},
		{"transaction index", func(m map[string]json.RawMessage) {
			var txs []map[string]json.RawMessage
			_ = json.Unmarshal(m["transactions"], &txs)
			txs[0]["transactionIndex"] = json.RawMessage(`"0xffff"`)
			m["transactions"], _ = json.Marshal(txs)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var value map[string]json.RawMessage
			if err := json.Unmarshal(raw, &value); err != nil {
				t.Fatal(err)
			}
			tc.edit(value)
			changed, e := json.Marshal(value)
			if e != nil {
				t.Fatal(e)
			}
			if _, e = a.parseBlock(changed); !errors.Is(e, ErrQualification) {
				t.Fatalf("changed block parsed: %v", e)
			}
		})
	}
	original, err := a.parseBlock(raw)
	if err != nil || original.Hash != bundle.BlockHash {
		t.Fatalf("recorded block %+v %v", original, err)
	}
	headerRaw, err := json.Marshal(bundle.Header)
	if err != nil {
		t.Fatal(err)
	}
	ethereumAuthority := &RPCQualification{}
	header, hash, err := ethereumAuthority.identifiedHeader(headerRaw)
	if err != nil || hash != header.Hash() {
		t.Fatalf("Ethereum identity %v %v", hash, err)
	}
	var value map[string]json.RawMessage
	if err = json.Unmarshal(headerRaw, &value); err != nil {
		t.Fatal(err)
	}
	value["hash"], _ = json.Marshal(common.HexToHash("0x1"))
	changed, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = ethereumAuthority.identifiedHeader(changed); !errors.Is(err, ErrQualification) {
		t.Fatalf("changed Ethereum identity: %v", err)
	}
	changedBlock := original
	changedHeader := types.CopyHeader(original.Header)
	changedHeader.Bloom = types.Bloom{}
	changedBlock.Header = changedHeader
	if _, _, err = a.pinnedReceipts(context.Background(), changedBlock); !errors.Is(err, ErrQualification) {
		t.Fatalf("changed merged bloom: %v", err)
	}
}
