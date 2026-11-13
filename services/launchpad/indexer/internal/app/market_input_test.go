package app

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/internaltest/chainfixture"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/receipt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

func TestMarketHintScanRejectsAlteredRecordedBlockAndReceipt(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name, method string
		edit         func(map[string]json.RawMessage)
	}{
		{"height", "eth_getBlockByNumber", func(m map[string]json.RawMessage) { m["number"] = json.RawMessage(`"0x1"`) }},
		{"hash", "eth_getBlockByNumber", func(m map[string]json.RawMessage) { m["hash"], _ = json.Marshal(common.Hash{}) }},
		{"timestamp", "eth_getBlockByNumber", func(m map[string]json.RawMessage) { m["timestamp"] = json.RawMessage(`"0x0"`) }},
		{"transaction body", "eth_getBlockByNumber", func(m map[string]json.RawMessage) { m["transactions"] = json.RawMessage(`[{}]`) }},
		{"transaction count", "eth_getBlockByNumber", func(m map[string]json.RawMessage) { m["transactions"], _ = json.Marshal(make([]any, 4097)) }},
		{"transaction hash", "eth_getBlockByNumber", func(m map[string]json.RawMessage) {
			var txs []map[string]json.RawMessage
			_ = json.Unmarshal(m["transactions"], &txs)
			txs[0]["hash"], _ = json.Marshal(common.HexToHash("0x1"))
			m["transactions"], _ = json.Marshal(txs)
		}},
		{"receipt height", "eth_getBlockReceipts", func(m map[string]json.RawMessage) { m["blockNumber"] = json.RawMessage(`"0x1"`) }},
		{"receipt block", "eth_getBlockReceipts", func(m map[string]json.RawMessage) { m["blockHash"], _ = json.Marshal(common.HexToHash("0x1")) }},
		{"receipt bloom", "eth_getBlockReceipts", func(m map[string]json.RawMessage) { m["logsBloom"], _ = json.Marshal("0x" + strings.Repeat("0", 512)) }},
		{"receipt logs", "eth_getBlockReceipts", func(m map[string]json.RawMessage) { m["logs"], _ = json.Marshal(make([]any, 4097)) }},
		{"log coordinates", "eth_getBlockReceipts", func(m map[string]json.RawMessage) {
			var logs []map[string]json.RawMessage
			_ = json.Unmarshal(m["logs"], &logs)
			logs[0]["logIndex"] = json.RawMessage(`"0xffff"`)
			m["logs"], _ = json.Marshal(logs)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := chainfixture.New(t)
			f.Mutate = func(key string, raw json.RawMessage) json.RawMessage {
				if !strings.HasPrefix(key, tc.method+" ") {
					return raw
				}
				var receipts []json.RawMessage
				if tc.method == "eth_getBlockReceipts" {
					if err := json.Unmarshal(raw, &receipts); err != nil || len(receipts) == 0 {
						t.Errorf("recorded receipts: %v", err)
						return raw
					}
					raw = receipts[0]
				}
				var value map[string]json.RawMessage
				if err := json.Unmarshal(raw, &value); err != nil {
					t.Fatal(err)
				}
				tc.edit(value)
				changed, err := json.Marshal(value)
				if err != nil {
					t.Fatal(err)
				}
				if receipts != nil {
					receipts[0] = changed
					changed, err = json.Marshal(receipts)
					if err != nil {
						t.Errorf("encode receipts: %v", err)
						return raw
					}
				}
				return changed
			}
			client, err := ethclient.DialContext(ctx, f.Serve(t))
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			s := recoveringQualifiedSource{client: client, blockIdentity: receipt.BlockIdentityReported}
			if _, _, err = s.marketAddresses(ctx, 33963716, 33963716); err == nil {
				t.Fatal("altered source hints accepted")
			}
		})
	}
	f := chainfixture.New(t)
	client, err := ethclient.DialContext(ctx, f.Serve(t))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	s := recoveringQualifiedSource{client: client, blockIdentity: receipt.BlockIdentityKeccak}
	if _, _, err = s.marketAddresses(ctx, 33963716, 33963716); !errors.Is(err, receipt.ErrBinding) {
		t.Fatalf("reported identity interpreted as Ethereum hash: %v", err)
	}
	s.blockIdentity = receipt.BlockIdentityReported
	hints, blocks, err := s.marketAddresses(ctx, 33963716, 33963716)
	if err != nil || len(hints) == 0 || len(blocks) != 1 || blocks[33963716].Height != 33963716 {
		t.Fatalf("source hints %d %+v %v", len(hints), blocks, err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, _, err = s.marketAddresses(canceled, 33963716, 33963716); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled source scan: %v", err)
	}
	for _, inventory := range []postgresMarketInventory{{}} {
		if _, err = inventory.Page(ctx, []common.Address{hints[0]}, math.MaxUint64, ""); !errors.Is(err, receipt.ErrQualification) {
			t.Fatalf("unbound inventory: %v", err)
		}
	}
}
