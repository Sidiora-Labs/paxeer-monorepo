//go:build ignore

// Record the pure accountId derivation used by the ledger's Go view harness.
// Run from the repository root; this recorder is never part of the offline gate.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
)

func main() {
	const fixtures = "services/interop/crates/x-websearch/tests/fixtures/"
	const output = "services/agent/sdk/typescript/test/recorded-web-ledger.json"
	read := func(path string) []byte {
		data, err := os.ReadFile(path)
		if err != nil {
			panic(err)
		}
		return data
	}
	var config struct {
		Assets map[string]struct {
			AssetID string `json:"asset_id"`
		} `json:"assets"`
	}
	if err := json.Unmarshal(read(fixtures+"config/valid.json"), &config); err != nil {
		panic(err)
	}
	var exchange struct {
		Exchange []struct {
			Request struct {
				Headers map[string]json.RawMessage `json:"headers"`
			} `json:"request"`
		} `json:"exchange"`
	}
	if err := json.Unmarshal(read(fixtures+"client-exchange.json"), &exchange); err != nil {
		panic(err)
	}
	var address string
	if err := json.Unmarshal(exchange.Exchange[0].Request.Headers["LAYERX-PAYER-ADDRESS"], &address); err != nil {
		panic(err)
	}
	var payment struct {
		Accepted struct {
			Extra struct {
				LayerX struct {
					Account string `json:"account"`
				} `json:"layerx"`
			} `json:"extra"`
		} `json:"accepted"`
	}
	if err := json.Unmarshal(exchange.Exchange[1].Request.Headers["PAYMENT-SIGNATURE"], &payment); err != nil {
		panic(err)
	}
	owners := []string{strings.Split(payment.Accepted.Extra.LayerX.Account, ":")[3], strings.Repeat("0", 24) + strings.TrimPrefix(address, "0x")}
	type call struct {
		Method string        `json:"method"`
		Params []interface{} `json:"params"`
		Result string        `json:"result"`
	}
	sources := map[string]string{}
	for _, path := range []string{"precompiles/layerxledger/views.go", "precompiles/layerxledger/abi.json", "modules/lxphost/accountmap.go", "layerxproof/codec/state.go"} {
		digest := sha256.Sum256(read(path))
		sources[path] = hex.EncodeToString(digest[:])
	}
	// Pin the actual view -> host -> codec call chain alongside the recorded values.
	if !strings.Contains(string(read("precompiles/layerxledger/views.go")), "lxphost.AssetAccountID(input.Owner, input.AssetID)") ||
		!strings.Contains(string(read("modules/lxphost/accountmap.go")), "codec.DeriveAccountID([]byte(name))") {
		panic("accountId's production derivation changed; review the recorder")
	}
	calls := []call{}
	for _, owner := range owners {
		for _, currency := range []string{"SID", "PAX", "USDC", "USDL"} {
			asset := config.Assets[currency].AssetID
			name := "agent:did:layerx:" + owner + ":asset:" + asset
			// This is the production codec called by AssetAccountID/accountId, not a test hash implementation.
			id, err := codec.DeriveAccountID([]byte(name))
			if err != nil {
				panic(err)
			}
			calls = append(calls, call{Method: "eth_call", Params: []interface{}{map[string]string{
				"to": "0x000000000000000000000000000000000000101a", "data": "0x3f050cbc" + owner + asset,
			}, "pending"}, Result: "0x" + hex.EncodeToString(id[:])})
		}
	}
	recording := struct {
		Producer string            `json:"producer"`
		Sources  map[string]string `json:"sources"`
		Calls    []call            `json:"calls"`
	}{"Go accountId view derivation: precompiles/layerxledger/views.go -> modules/lxphost/accountmap.go -> layerxproof/codec.DeriveAccountID; bytes32 ABI output; pure and independent of block state", sources, calls}
	data, err := json.MarshalIndent(recording, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(output, append(data, '\n'), 0644); err != nil {
		panic(err)
	}
	fmt.Println(output)
}
