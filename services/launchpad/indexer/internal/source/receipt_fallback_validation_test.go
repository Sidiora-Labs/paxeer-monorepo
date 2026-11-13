package source

import (
	"bytes"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

func TestCommittedReceiptFallbackAuthenticatesActualTransactions(t *testing.T) {
	ctx, client, endpoint := genuineHistoricalNode(t)
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	sender := crypto.PubkeyToAddress(key.PublicKey)
	if err = client.Client().CallContext(ctx, nil, "anvil_setBalance", sender.Hex(), "0x3635c9adc5dea00000"); err != nil {
		t.Fatal(err)
	}
	parsed, code, revision, manifest := historicalArtifact(t)
	auth, err := bind.NewKeyedTransactorWithChainID(key, big.NewInt(125))
	if err != nil {
		t.Fatal(err)
	}
	auth.Context = ctx
	token, tx, _, err := bind.DeployContract(auth, parsed, code, client, "Settlement", "SET", big.NewInt(1000), sender)
	if err != nil {
		t.Fatal(err)
	}
	mined, err := bind.WaitMined(ctx, client, tx)
	if err != nil || len(mined.Logs) != 1 {
		t.Fatalf("actual mint %+v %v", mined, err)
	}
	var mode atomic.Value
	mode.Store("fallback")
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
			return
		}
		var q struct {
			ID     json.RawMessage
			Method string
		}
		if err = json.Unmarshal(body, &q); err != nil {
			t.Error(err)
			return
		}
		current := mode.Load().(string)
		if q.Method == "eth_getBlockReceipts" && strings.HasPrefix(current, "bulk-") {
			if err = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": q.ID, "error": map[string]any{"code": -32601, "message": "method unavailable"}}); err != nil {
				t.Error(err)
			}
			return
		}
		if q.Method == "eth_getLogs" && !strings.HasPrefix(current, "wire-") {
			if err = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": q.ID, "error": map[string]any{"code": -32005, "message": "too many results"}}); err != nil {
				t.Error(err)
			}
			return
		}
		req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, endpoint, bytes.NewReader(body))
		if err != nil {
			t.Error(err)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		response, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Error(err)
			return
		}
		defer response.Body.Close()
		var out map[string]json.RawMessage
		if err = json.NewDecoder(response.Body).Decode(&out); err != nil {
			t.Error(err)
			return
		}
		if q.Method == "eth_getBlockReceipts" && strings.HasPrefix(current, "archive-") {
			var receipts []map[string]any
			if err = json.Unmarshal(out["result"], &receipts); err != nil || len(receipts) != 1 {
				t.Errorf("actual receipt list %v", err)
				return
			}
			switch current {
			case "archive-nil-receipt":
				receipts[0] = nil
			case "archive-nil-log":
				receipts[0]["logs"] = []any{nil}
			case "archive-log-coordinate":
				receipts[0]["logs"].([]any)[0].(map[string]any)["logIndex"] = "0x1"
			case "archive-root":
				receipts[0]["cumulativeGasUsed"] = "0x1"
			}
			out["result"], err = json.Marshal(receipts)
			if err != nil {
				t.Error(err)
				return
			}
		}
		if q.Method == "eth_getTransactionReceipt" && current != "fallback" {
			var receipt map[string]any
			if err = json.Unmarshal(out["result"], &receipt); err != nil {
				t.Error(err)
				return
			}
			switch current {
			case "receipt-height":
				receipt["blockNumber"] = "0x2"
			case "receipt-transaction":
				receipt["transactionHash"] = common.HexToHash("0x1").Hex()
			case "nil-log":
				receipt["logs"] = []any{nil}
			case "log-coordinate":
				receipt["logs"].([]any)[0].(map[string]any)["logIndex"] = "0x1"
			case "receipt-error", "bulk-receipt-error":
				delete(out, "result")
				out["error"] = json.RawMessage(`{"code":-32000,"message":"receipt unavailable"}`)
			}
			if current != "receipt-error" && current != "bulk-receipt-error" {
				out["result"], err = json.Marshal(receipt)
				if err != nil {
					t.Error(err)
					return
				}
			}
		}
		if q.Method == "eth_getLogs" && strings.HasPrefix(current, "wire-") {
			var logs []map[string]any
			if err = json.Unmarshal(out["result"], &logs); err != nil || len(logs) != 1 {
				t.Errorf("actual logs %v %v", logs, err)
				return
			}
			switch current {
			case "wire-prefix":
				logs[0]["data"] = "01"
			case "wire-data":
				logs[0]["data"] = "0xzz"
			case "wire-height":
				logs[0]["blockNumber"] = "0xzz"
			case "wire-index":
				logs[0]["logIndex"] = "0x100000000"
			case "wire-transaction":
				logs[0]["transactionIndex"] = "bad"
			case "wire-hash":
				logs[0]["blockHash"] = "0x1"
			case "wire-removed":
				logs[0]["removed"] = true
			case "wire-block":
				logs[0]["blockHash"] = common.HexToHash("0x1").Hex()
			case "wire-duplicate":
				logs = append(logs, logs[0])
			}
			out["result"], err = json.Marshal(logs)
			if err != nil {
				t.Error(err)
				return
			}
		}
		if err = json.NewEncoder(w).Encode(out); err != nil {
			t.Error(err)
		}
	}))
	defer proxy.Close()
	genesis, err := client.HeaderByNumber(ctx, big.NewInt(0))
	if err != nil {
		t.Fatal(err)
	}
	s, err := NewCommittedSource(ctx, CommittedOptions{Endpoints: map[string]string{"primary": proxy.URL}, GenesisHash: genesis.Hash(), ManifestDigest: manifest, Epochs: []AddressEpoch{{Address: token.Hex(), Role: "SidioraERC20", ABIRevision: revision, ManifestDigest: manifest, ActivationHeight: 1}}})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	endpointRecord := s.endpoints[0]
	block, record, err := s.block(ctx, endpointRecord, "0x1", false)
	if err != nil {
		t.Fatal(err)
	}
	for _, identity := range []string{"keccak", "reported"} {
		s.opts.BlockIdentity = identity
		logs, err := s.logs(ctx, endpointRecord, 1, 1, []string{token.Hex()}, false, map[uint64]batchBlock{1: {block, record}}, func() error { return nil })
		if err != nil || len(logs) != 1 || logs[0].TxHash != tx.Hash() || logs[0].Address != token {
			t.Fatalf("authenticated %s logs %+v %v", identity, logs, err)
		}
	}
	s.opts.BlockIdentity = "keccak"
	for _, name := range []string{"archive-nil-receipt", "archive-nil-log", "archive-log-coordinate", "archive-root"} {
		t.Run(name, func(t *testing.T) {
			mode.Store(name)
			if logs, err := s.receiptLogs(ctx, endpointRecord, block, record, 1, []common.Address{token}, false); err == nil || len(logs) != 0 {
				t.Fatalf("unauthenticated archive receipt logs %+v: %v", logs, err)
			}
		})
	}
	for _, name := range []string{"bulk-individual", "bulk-receipt-error"} {
		t.Run("archive-"+name, func(t *testing.T) {
			mode.Store(name)
			logs, err := s.receiptLogs(ctx, endpointRecord, block, record, 1, []common.Address{token}, false)
			if name == "bulk-individual" {
				if err != nil || len(logs) != 1 || logs[0].TxHash != tx.Hash() || logs[0].Index != 0 || logs[0].Address != token {
					t.Fatalf("individual archive receipt logs %+v: %v", logs, err)
				}
			} else if strings.HasPrefix(name, "bulk-") && err == nil {
				t.Fatal("unavailable individual receipt admitted")
			}
		})
	}
	for _, name := range []string{"receipt-height", "receipt-transaction", "nil-log", "log-coordinate", "receipt-error", "wire-prefix", "wire-data", "wire-height", "wire-index", "wire-transaction", "wire-hash", "wire-removed", "wire-block", "wire-duplicate"} {
		t.Run(name, func(t *testing.T) {
			mode.Store(name)
			result, err := s.fetch(ctx, endpointRecord, FetchOptions{FromBlock: 1, ToBlock: 1}, false, s.opts.Epochs)
			if err == nil || result.Complete {
				t.Fatalf("unauthenticated logs admitted %+v %v", result, err)
			}
		})
	}
	mode.Store("fallback")
	for _, name := range []string{"root", "transaction", "count", "bloom"} {
		t.Run(name, func(t *testing.T) {
			b := block
			b.Transactions = append([]ethTxWire(nil), block.Transactions...)
			switch name {
			case "root":
				b.ReceiptsRoot = "invalid"
			case "transaction":
				b.Transactions[0].Hash = "invalid"
			case "count":
				b.Transactions = make([]ethTxWire, 4097)
			case "bloom":
				b.LogsBloom = "0x" + strings.Repeat("0", 512)
				s.opts.BlockIdentity = "reported"
			}
			if _, err := s.logs(ctx, endpointRecord, 1, 1, []string{token.Hex()}, false, map[uint64]batchBlock{1: {b, record}}, func() error { return nil }); err == nil {
				t.Fatal("invalid fallback block admitted")
			}
		})
	}
	s.opts.BlockIdentity = "keccak"
	if _, got, err := s.batchBlock(ctx, endpointRecord, 1, false, map[uint64]batchBlock{}); err != nil || got.Hash != record.Hash {
		t.Fatalf("uncached block %+v %v", got, err)
	}
	if s.Metrics().ReceiptFallbacks < 4 {
		t.Fatalf("fallback metrics %+v", s.Metrics())
	}
}
