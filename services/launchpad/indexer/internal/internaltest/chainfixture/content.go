package chainfixture

import (
	"encoding/json"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/trie"
)

func NewContentBlock(t testing.TB, transactions, logs int, data []byte) *ShellBlock {
	t.Helper()
	b := NewShellBlock(t, true)
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	address := b.Receipts[0].Logs[0].Address
	b.Transactions = make(types.Transactions, transactions)
	b.Receipts = make(types.Receipts, transactions)
	for i := range b.Transactions {
		tx, err := types.SignTx(types.NewTx(&types.LegacyTx{Nonce: uint64(i), Gas: 21000, GasPrice: big.NewInt(1), To: &address}), types.LatestSignerForChainID(big.NewInt(125)), key)
		if err != nil {
			t.Fatal(err)
		}
		b.Transactions[i] = tx
		b.Receipts[i] = &types.Receipt{Type: tx.Type(), Status: 1, TxHash: tx.Hash(), BlockHash: b.Hash, BlockNumber: new(big.Int).Set(b.Header.Number), TransactionIndex: uint(i), Logs: []*types.Log{}}
	}
	for i := 0; i < logs; i++ {
		r := b.Receipts[0]
		r.Logs = append(r.Logs, &types.Log{Address: address, Data: data, Topics: []common.Hash{}, BlockHash: b.Hash, BlockNumber: b.Header.Number.Uint64(), TxHash: r.TxHash, Index: uint(i)})
	}
	for _, r := range b.Receipts {
		r.Bloom = types.CreateBloom(r)
	}
	b.Header.Bloom = types.MergeBloom(b.Receipts)
	b.Header.TxHash = types.DeriveSha(b.Transactions, trie.NewStackTrie(nil))
	b.Header.ReceiptHash = types.DeriveSha(b.Receipts, trie.NewStackTrie(nil))
	return b
}

func (b *ShellBlock) ServeContent(t testing.TB, receiptFallback bool) string {
	t.Helper()
	byAddress := map[string][]*types.Log{}
	for _, r := range b.Receipts {
		encoded, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		hash, _ := json.Marshal(r.TxHash)
		key := Key("eth_getTransactionReceipt", []json.RawMessage{hash})
		b.Fixture.entries[key] = recorded{Key: key, Result: encoded}
		for _, log := range r.Logs {
			address := strings.ToLower(log.Address.Hex())
			byAddress[address] = append(byAddress[address], log)
		}
	}
	for address, logs := range byAddress {
		params, _ := json.Marshal(map[string]any{"address": []string{address}, "fromBlock": hexutil.EncodeBig(b.Header.Number), "toBlock": hexutil.EncodeBig(b.Header.Number)})
		key := Key("eth_getLogs", []json.RawMessage{params})
		entry := recorded{Key: key}
		if receiptFallback {
			entry.Error = json.RawMessage(`{"code":-32005,"message":"too many results"}`)
		} else {
			entry.Result, _ = json.Marshal(logs)
		}
		b.Fixture.entries[key] = entry
	}
	return b.Serve(t)
}
