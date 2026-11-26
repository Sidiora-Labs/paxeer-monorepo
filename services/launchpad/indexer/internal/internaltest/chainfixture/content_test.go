package chainfixture

import (
	"bytes"
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/ethereum/go-ethereum/trie"
)

func TestContentBlocksServeSignedTransactionsReceiptsAndLogs(t *testing.T) {
	t.Setenv(RecordEnv, "")
	for _, fallback := range []bool{false, true} {
		t.Run(map[bool]string{false: "logs", true: "receipt fallback"}[fallback], func(t *testing.T) {
			payload := []byte{1, 2, 3, 4}
			block := NewContentBlock(t, 3, 2, payload)
			for _, receipt := range block.Receipts {
				receipt.PostState = []byte{}
			}
			if len(block.Transactions) != 3 || len(block.Receipts) != 3 || len(block.Receipts[0].Logs) != 2 || block.Header.TxHash != types.DeriveSha(block.Transactions, trie.NewStackTrie(nil)) || block.Header.ReceiptHash != types.DeriveSha(block.Receipts, trie.NewStackTrie(nil)) || block.Header.Bloom != types.MergeBloom(block.Receipts) {
				t.Fatal("content roots or membership differ from the signed material")
			}
			client, err := rpc.DialContext(context.Background(), block.ServeContent(t, fallback))
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			var served struct {
				Hash         common.Hash
				Transactions []*types.Transaction
			}
			if err := client.CallContext(context.Background(), &served, "eth_getBlockByNumber", hexutil.EncodeBig(block.Header.Number), true); err != nil || served.Hash != block.Hash || len(served.Transactions) != len(block.Transactions) {
				t.Fatalf("content block: %+v, %v", served, err)
			}
			for i, tx := range block.Transactions {
				if served.Transactions[i].Hash() != tx.Hash() || tx.Nonce() != uint64(i) || tx.ChainId().Uint64() != 125 {
					t.Fatal("served transaction lost its signed identity")
				}
				var receipt types.Receipt
				if err := client.CallContext(context.Background(), &receipt, "eth_getTransactionReceipt", tx.Hash()); err != nil || !reflect.DeepEqual(&receipt, block.Receipts[i]) {
					t.Fatalf("content receipt %d: %+v, %v", i, receipt, err)
				}
				for j, log := range receipt.Logs {
					if log.BlockHash != block.Hash || log.TxHash != tx.Hash() || log.Index != uint(j) || !bytes.Equal(log.Data, payload) {
						t.Fatal("content log coordinates or bytes changed")
					}
				}
			}
			address := block.Receipts[0].Logs[0].Address
			filter := map[string]any{"address": []string{strings.ToLower(address.Hex())}, "fromBlock": hexutil.EncodeBig(block.Header.Number), "toBlock": hexutil.EncodeBig(block.Header.Number)}
			var logs []*types.Log
			err = client.CallContext(context.Background(), &logs, "eth_getLogs", filter)
			if fallback {
				failure, ok := err.(rpc.Error)
				if !ok || failure.ErrorCode() != -32005 {
					t.Fatalf("receipt fallback refusal: %v", err)
				}
			} else if err != nil || !reflect.DeepEqual(logs, block.Receipts[0].Logs) {
				t.Fatalf("content logs: %+v, %v", logs, err)
			}
			var receipts types.Receipts
			if err := client.CallContext(context.Background(), &receipts, "eth_getBlockReceipts", block.Hash); err != nil || !reflect.DeepEqual(receipts, block.Receipts) {
				t.Fatalf("content block receipts: %v", err)
			}
		})
	}
}
