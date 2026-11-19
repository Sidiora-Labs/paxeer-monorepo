package chainfixture

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
)

func TestShellCasesRetainRecordedBlockAndReceiptWire(t *testing.T) {
	t.Setenv(RecordEnv, "")
	cases := ShellCases()
	if !reflect.DeepEqual(cases, []string{"shell only", "mixed", "missing listed bloom", "extra header bloom", "shell own bloom"}) {
		t.Fatalf("shell cases: %v", cases)
	}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			block := NewShellCase(t, name)
			wantListed := 1
			if name == "shell only" {
				wantListed = 0
			}
			if len(block.Transactions) != wantListed || len(block.Receipts) != wantListed+1 || block.Reject != (name != "shell only" && name != "mixed") {
				t.Fatalf("shell block membership: %+v", block)
			}
			shell := block.Receipts[len(block.Receipts)-1]
			if shell.Type != ^uint8(0) || shell.TransactionIndex != uint(wantListed) || shell.BlockHash != block.Hash || shell.BlockNumber.Cmp(block.Header.Number) != 0 || len(shell.Logs) == 0 {
				t.Fatal("shell receipt lost recorded block coordinates")
			}
			listedLogs := 0
			for i, transaction := range block.Transactions {
				if transaction.Hash() != block.Receipts[i].TxHash {
					t.Fatal("listed signed transaction changed")
				}
				listedLogs += len(block.Receipts[i].Logs)
			}
			for i, log := range shell.Logs {
				if log.BlockHash != block.Hash || log.BlockNumber != block.Header.Number.Uint64() || log.TxHash != shell.TxHash || log.TxIndex != shell.TransactionIndex || log.Index != uint(listedLogs+i) {
					t.Fatalf("shell log coordinates: %+v", log)
				}
			}
			client, err := rpc.DialContext(context.Background(), block.Serve(t))
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			var chain string
			if err := client.CallContext(context.Background(), &chain, "eth_chainId"); err != nil || chain != "0x7d" {
				t.Fatalf("recorded chain: %q, %v", chain, err)
			}
			for _, method := range []string{"eth_getBlockByNumber", "eth_getBlockByHash"} {
				var selector any = hexutil.EncodeBig(block.Header.Number)
				if method == "eth_getBlockByHash" {
					selector = block.Hash
				}
				var wire struct {
					Hash            common.Hash
					Bloom           types.Bloom `json:"logsBloom"`
					ReceiptRoot     common.Hash `json:"receiptsRoot"`
					TransactionRoot common.Hash `json:"transactionsRoot"`
					Transactions    []json.RawMessage
				}
				if err := client.CallContext(context.Background(), &wire, method, selector, true); err != nil {
					t.Fatal(err)
				}
				if wire.Hash != block.Hash || wire.Bloom != block.Header.Bloom || wire.ReceiptRoot != block.Header.ReceiptHash || wire.TransactionRoot != block.Header.TxHash || len(wire.Transactions) != wantListed {
					t.Fatalf("served block: %+v", wire)
				}
			}
			for _, selector := range []any{block.Hash, map[string]any{"blockHash": block.Hash, "requireCanonical": true}} {
				var wire []struct {
					Type  string
					Hash  common.Hash  `json:"transactionHash"`
					Block common.Hash  `json:"blockHash"`
					Index hexutil.Uint `json:"transactionIndex"`
					Bloom types.Bloom  `json:"logsBloom"`
					Logs  []*types.Log
				}
				if err := client.CallContext(context.Background(), &wire, "eth_getBlockReceipts", selector); err != nil {
					t.Fatal(err)
				}
				if len(wire) != len(block.Receipts) {
					t.Fatalf("receipt count = %d", len(wire))
				}
				for i, received := range wire {
					expected := block.Receipts[i]
					if received.Hash != expected.TxHash || received.Block != expected.BlockHash || uint(received.Index) != expected.TransactionIndex || received.Bloom != expected.Bloom || !reflect.DeepEqual(received.Logs, expected.Logs) {
						t.Fatalf("served receipt %d: %+v", i, received)
					}
				}
				if wire[len(wire)-1].Type != "0xffffffff" {
					t.Fatalf("shell wire type = %q", wire[len(wire)-1].Type)
				}
			}
		})
	}
}
