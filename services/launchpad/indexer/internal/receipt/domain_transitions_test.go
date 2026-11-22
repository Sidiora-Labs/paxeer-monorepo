package receipt

import (
	"bytes"
	"context"
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func TestSignedDomainRejectsInvalidEarlierRegistryTransitions(t *testing.T) {
	ctx := context.Background()
	input := receiptReplayInputsFromProducer(t)
	base, closeBase := receiptReplayOpen(t, ctx, input)
	defer closeBase()
	bundle, err := base.Bundle(ctx, input.TenantSwap)
	if err != nil {
		t.Fatal(err)
	}
	if err := base.ValidateSignedDomain(ctx, bundle); err != nil {
		t.Fatal(err)
	}
	native, err := base.Bundle(ctx, input.NativeSwap)
	if err != nil {
		t.Fatal(err)
	}
	registry, err := abi.JSON(bytes.NewReader(virtualChainABIJSON))
	if err != nil {
		t.Fatal(err)
	}
	topic := common.BigToHash(new(big.Int).Sub(bundle.Transaction.ChainId(), new(big.Int).SetUint64(virtualChainIDBase)))
	creation := func(chain uint64) *types.Log {
		t.Helper()
		data, err := registry.Events["ChainCreated"].Inputs.NonIndexed().Pack("Tenant", chain)
		if err != nil {
			t.Fatal(err)
		}
		return &types.Log{Address: virtualChainRegistry, Topics: []common.Hash{registry.Events["ChainCreated"].ID, topic, common.BytesToHash(virtualChainRegistry.Bytes())}, Data: data}
	}
	change := func(status uint8) *types.Log {
		t.Helper()
		data, err := registry.Events["ChainStatusChanged"].Inputs.NonIndexed().Pack(status)
		if err != nil {
			t.Fatal(err)
		}
		return &types.Log{Address: virtualChainRegistry, Topics: []common.Hash{registry.Events["ChainStatusChanged"].ID, topic}, Data: data}
	}
	chain := bundle.Transaction.ChainId().Uint64()
	for _, name := range []string{"paused", "retired transition", "status before creation", "duplicate creation", "creation topics", "creation data", "creation padding", "creation domain", "status topics", "status data", "status padding", "status zero", "status range", "transaction identity", "receipt index", "header identity"} {
		t.Run(name, func(t *testing.T) {
			b, err := clone(bundle)
			if err != nil {
				t.Fatal(err)
			}
			created, paused := creation(chain), change(2)
			logs := []*types.Log{created, paused}
			switch name {
			case "retired transition":
				logs = []*types.Log{created, change(4), change(1)}
			case "status before creation":
				logs = []*types.Log{change(1), created}
			case "duplicate creation":
				logs = []*types.Log{created, creation(chain)}
			case "creation topics":
				created.Topics = created.Topics[:2]
			case "creation data":
				created.Data = []byte{1}
			case "creation padding":
				created.Data = append(created.Data, make([]byte, 32)...)
			case "creation domain":
				logs[0] = creation(chain + 1)
			case "status topics":
				paused.Topics = append(paused.Topics, common.Hash{})
			case "status data":
				paused.Data = []byte{1}
			case "status padding":
				paused.Data = append(paused.Data, make([]byte, 32)...)
			case "status zero":
				logs[1] = change(0)
			case "status range":
				logs[1] = change(5)
			}
			// Failed receipts and unrelated logs cannot establish admission.
			failed := &types.Receipt{Status: types.ReceiptStatusFailed, Logs: []*types.Log{creation(chain)}}
			unrelated := &types.Receipt{Status: types.ReceiptStatusSuccessful, Logs: []*types.Log{{Address: common.Address{}}, {Address: virtualChainRegistry}, {Address: virtualChainRegistry, Topics: []common.Hash{created.Topics[0], common.Hash{}}}}}
			prior := &types.Receipt{Status: types.ReceiptStatusSuccessful, Logs: logs}
			b.Receipt.TransactionIndex = 3
			block := pinnedBlock{Header: b.Header, Hash: bundleBlockHash(b), Transactions: types.Transactions{native.Transaction, native.Transaction, native.Transaction, b.Transaction}}
			if name == "transaction identity" {
				block.Transactions[3] = native.Transaction
			}
			if name == "header identity" {
				block.Header = types.CopyHeader(b.Header)
				block.Header.Time++
			}
			cached := &rpcReadState{}
			cached.storeBlock(block.Hash, cachedBlock{block: block, receipts: types.Receipts{failed, unrelated, prior, b.Receipt}})
			if name == "receipt index" {
				b.Receipt.TransactionIndex = 4
			}
			authority := &RPCQualification{client: base.client, options: base.options, reads: cached}
			before, err := digest(b)
			if err != nil {
				t.Fatal(err)
			}
			if err := authority.ValidateSignedDomain(ctx, b); !errors.Is(err, ErrQualification) {
				t.Fatalf("invalid registry transition admitted: %v", err)
			}
			after, err := digest(b)
			if err != nil || before != after {
				t.Fatalf("admission changed caller material: %v", err)
			}
		})
	}
}
