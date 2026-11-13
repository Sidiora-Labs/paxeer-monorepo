package capabilities

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
)

func TestReportedReaderRejectsMissingAndInconsistentBlocks(t *testing.T) {
	hash := common.Hash{1}
	for _, mode := range []string{"null", "malformed header", "missing hash", "invalid hash", "wrong height", "overflow height", "null transaction", "rpc error"} {
		t.Run(mode, func(t *testing.T) {
			header := &types.Header{Number: big.NewInt(7), Difficulty: big.NewInt(0)}
			encoded, err := json.Marshal(header)
			if err != nil {
				t.Fatal(err)
			}
			var body map[string]any
			if err := json.Unmarshal(encoded, &body); err != nil {
				t.Fatal(err)
			}
			body["hash"] = hash.Hex()
			var result any = body
			switch mode {
			case "null":
				result = nil
			case "malformed header":
				result = "not a block"
			case "missing hash":
				delete(body, "hash")
			case "invalid hash":
				body["hash"] = "0x00"
			case "wrong height":
				body["number"] = "0x8"
			case "overflow height":
				body["number"] = "0x10000000000000000"
			case "null transaction":
				body["transactions"] = []any{nil}
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var req struct{ ID json.RawMessage }
				if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
					t.Error(err)
					return
				}
				reply := map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result}
				if mode == "rpc error" {
					delete(reply, "result")
					reply["error"] = map[string]any{"code": -32000, "message": "block unavailable"}
				}
				if err := json.NewEncoder(w).Encode(reply); err != nil {
					t.Error(err)
				}
			}))
			defer server.Close()
			client, err := rpc.DialContext(context.Background(), server.URL)
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			reader, err := NewPinnedRPCReader(client)
			if err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			if mode == "null transaction" {
				if block, err := reader.ReportedBlockByHash(ctx, hash); err == nil || block != nil {
					t.Fatalf("null transaction accepted: %v %v", block, err)
				}
			} else {
				if header, _, err := reader.ReportedHeaderByNumber(ctx, big.NewInt(7)); err == nil || header != nil {
					t.Fatalf("inconsistent reported block: %v %v", header, err)
				}
			}
			if mode != "wrong height" && mode != "overflow height" && mode != "null transaction" {
				if header, err := reader.HeaderByReportedHash(ctx, hash); err == nil || header != nil {
					t.Fatalf("invalid reported hash: %v %v", header, err)
				}
			}
		})
	}
	var absent *PinnedRPCReader
	if _, _, _, err := absent.rawReportedBlock(context.Background(), "eth_getBlockByNumber", "0x7", false); err == nil {
		t.Fatal("missing reader admitted")
	}
	chain := repair38OpenChain(t, true)
	pinned := reportedPinnedReader{chain: chain.reader, reads: chain.reader, blocks: map[common.Hash]*types.Header{}}
	for name, call := range map[string]func() error{
		"header": func() error { _, err := pinned.HeaderByHash(context.Background(), hash); return err },
		"storage": func() error {
			_, err := pinned.StorageAtHash(context.Background(), common.Address{}, common.Hash{}, hash)
			return err
		},
		"call": func() error {
			_, err := pinned.CallContractAtHash(context.Background(), ethereum.CallMsg{}, hash)
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			if err := call(); !errors.Is(err, errUnpinnedBlock) {
				t.Fatalf("unbound numeric read: %v", err)
			}
		})
	}
	for _, call := range []func() error{
		func() error {
			_, err := chain.engine.call(context.Background(), common.Address{}, "invalid", hash)
			return err
		},
		func() error {
			_, _, err := chain.engine.read(context.Background(), common.Address{}, "invalid", hash)
			return err
		},
	} {
		if err := call(); err == nil {
			t.Fatal("malformed signature admitted")
		}
	}
	request := chain.request(PlainQuote, chain.peer.blocks[1])
	request.Pool = chain.peer.epoch.Pool
	chain.peer.override(chain.peer.address("Quoter"), "quoteExactInput(address,uint256,bool)", make([]byte, 4097), request.Pool, big.NewInt(1000000), true)
	if _, _, err := chain.engine.read(context.Background(), chain.peer.address("Quoter"), "quoteExactInput(address,uint256,bool)", request.BlockHash, request.Pool, big.NewInt(1000000), true); err == nil {
		t.Fatal("oversized contract output admitted")
	}
	candidate := *chain.engine
	candidate.manifest.DeploymentVersion = "unbound"
	if _, err := candidate.nativeDeployment(); err == nil {
		t.Fatal("unapproved deployment admitted")
	} else if !strings.Contains(err.Error(), "identity_unverified") {
		t.Fatal(err)
	}
}
