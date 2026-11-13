package chain

import (
	"context"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/chain/profile"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func TestBoundedLogsValidateCommittedEvents(t *testing.T) {
	header := testHeader(100)
	blockHash := consensusHash(header)
	address := common.HexToAddress("0x1111111111111111111111111111111111111111")
	topic := common.HexToHash("0x01")
	base := types.Log{Address: address, Topics: []common.Hash{topic}, Data: []byte{1, 2}, BlockNumber: 100, BlockHash: blockHash, TxHash: common.HexToHash("0x11"), TxIndex: 1, Index: 1}
	query := ethereum.FilterQuery{FromBlock: big.NewInt(100), ToBlock: big.NewInt(100), Addresses: []common.Address{address}, Topics: [][]common.Hash{{topic}}}
	tests := []struct {
		name  string
		logs  []types.Log
		valid bool
		mode  string
	}{
		{name: "committed", logs: []types.Log{base}, valid: true},
		{name: "interior header", logs: []types.Log{func() types.Log {
			v := base
			v.BlockNumber = 101
			v.BlockHash = consensusHash(testHeader(101))
			return v
		}()}, valid: true, mode: "interior"},
		{name: "missing boundary", mode: "missing boundary"},
		{name: "unavailable boundary", mode: "unavailable boundary"},
		{name: "missing interior", logs: []types.Log{func() types.Log {
			v := base
			v.BlockNumber = 101
			v.BlockHash = consensusHash(testHeader(101))
			return v
		}()}, mode: "missing interior"},
		{name: "unavailable interior", logs: []types.Log{func() types.Log {
			v := base
			v.BlockNumber = 101
			v.BlockHash = consensusHash(testHeader(101))
			return v
		}()}, mode: "unavailable interior"},
		{name: "wildcard topic", logs: []types.Log{base}, valid: true, mode: "wildcard"},

		{name: "removed", logs: []types.Log{func() types.Log { v := base; v.Removed = true; return v }()}},
		{name: "wrong block", logs: []types.Log{func() types.Log { v := base; v.BlockNumber = 99; return v }()}},
		{name: "wrong hash", logs: []types.Log{func() types.Log { v := base; v.BlockHash = common.HexToHash("0x22"); return v }()}},
		{name: "wrong address", logs: []types.Log{func() types.Log {
			v := base
			v.Address = common.HexToAddress("0x2222222222222222222222222222222222222222")
			return v
		}()}},
		{name: "wrong topic", logs: []types.Log{func() types.Log { v := base; v.Topics = []common.Hash{common.HexToHash("0x02")}; return v }()}},
		{name: "missing topic", logs: []types.Log{func() types.Log { v := base; v.Topics = nil; return v }()}},
		{name: "duplicate", logs: []types.Log{base, base}},
		{name: "descending index", logs: []types.Log{func() types.Log { v := base; v.Index = 2; return v }(), base}},
		{name: "descending transaction", logs: []types.Log{func() types.Log { v := base; v.TxIndex = 2; return v }(), func() types.Log { v := base; v.Index = 2; return v }()}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var requests atomic.Int64
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request struct {
					ID     json.RawMessage   `json:"id"`
					Method string            `json:"method"`
					Params []json.RawMessage `json:"params"`
				}
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
					http.Error(w, "invalid request", http.StatusBadRequest)
					return
				}
				requests.Add(1)
				var result any
				switch request.Method {
				case "eth_chainId":
					result = "0x7d"
				case "eth_getBlockByNumber":
					selected := header
					var requested string
					if len(request.Params) > 0 {
						if err := json.Unmarshal(request.Params[0], &requested); err != nil {
							t.Error(err)
						}
					}
					if requested == "0x65" {
						selected = testHeader(101)
					}
					if requested == "0x66" {
						selected = testHeader(102)
					}
					result = reportedJSON(selected)
					unavailable := (requested == "0x66" && tc.mode == "unavailable boundary") || (requested == "0x65" && tc.mode == "unavailable interior")
					if unavailable {
						http.Error(w, "unavailable committed header", 400)
						return
					}
					if requested == "0x66" && tc.mode == "missing boundary" || requested == "0x65" && tc.mode == "missing interior" {
						result = nil
					}
				case "eth_getLogs":
					result = tc.logs
				default:
					t.Errorf("unexpected RPC method %s", request.Method)
					http.Error(w, "unexpected method", http.StatusBadRequest)
					return
				}
				if err := json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": request.ID, "result": result}); err != nil {
					t.Error(err)
				}
			}))
			t.Cleanup(server.Close)
			client, err := NewVerifiedClient(context.Background(), server.URL, "", VerifiedConfig{profile.Paxeer("e1", "d1"), 100, blockHash})
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			bounded := query
			if tc.mode != "" && tc.mode != "wildcard" {
				bounded.ToBlock = big.NewInt(102)
			}
			if tc.mode == "wildcard" {
				bounded.Topics = [][]common.Hash{{}}
			}
			logs, err := client.LogsBounded(context.Background(), bounded, 3)
			if tc.valid {
				if err != nil || len(logs) != 1 || logs[0].TxHash != base.TxHash || logs[0].Data[0] != 1 {
					t.Fatalf("committed log = %+v, %v", logs, err)
				}
			} else if err == nil || len(logs) != 0 {
				t.Fatalf("invalid committed log accepted: %+v, %v", logs, err)
			}
			if requests.Load() < 3 {
				t.Fatalf("committed RPC checks = %d", requests.Load())
			}
		})
	}
}
