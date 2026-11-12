package receipt

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

func TestMetadataRequiresEveryPinnedContractBinding(t *testing.T) {
	ctx := context.Background()
	original := repair53InputsFromProducer(t)
	baseline, closeBaseline := repair53Open(t, ctx, original)
	defer closeBaseline()
	if err := baseline.Rehydrate(ctx, original.Markets); err != nil {
		t.Fatal(err)
	}
	baseline = baseline.current()
	bundle, err := baseline.Bundle(ctx, original.NativeSwap)
	if err != nil {
		t.Fatal(err)
	}
	pool, token := original.Markets[0].Pool, original.Markets[0].Token
	metadata, err := baseline.StableMetadata(ctx, pool, bundle.BlockHash)
	if err != nil || metadata.Token != token {
		t.Fatalf("source metadata %+v %v", metadata, err)
	}
	for _, tc := range []struct {
		name, signature string
		address         common.Address
		result          string
	}{
		{"token read", "tokenAddress()", pool, "error"},
		{"settlement binding", "usdlAddress()", pool, "zero"},
		{"emitter binding", "eventEmitter()", pool, "zero"},
		{"configuration binding", "protocolConfig()", pool, "zero"},
		{"registration", "isRegisteredPool(address)", common.HexToAddress(baseline.options.Manifest.Roles["PoolRegistry"].Address), "zero"},
		{"token registration", "getPoolByToken(address)", common.HexToAddress(baseline.options.Manifest.Roles["PoolRegistry"].Address), "zero"},
		{"token decimal width", "decimals()", token, "wide"},
		{"settlement decimal scale", "decimals()", metadata.Settlement, "one"},
		{"truncated token word", "tokenAddress()", pool, "short"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input, err := clone(original)
			if err != nil {
				t.Fatal(err)
			}
			selector := common.Bytes2Hex(crypto.Keccak256([]byte(tc.signature))[:4])
			changed := 0
			for key, response := range input.Responses {
				if !strings.HasPrefix(key, "eth_call ") || !strings.Contains(strings.ToLower(key), strings.ToLower(tc.address.Hex())) || !strings.Contains(strings.ToLower(key), selector) || !strings.Contains(strings.ToLower(key), strings.ToLower(bundle.BlockHash.Hex())) {
					continue
				}
				response.Error = nil
				switch tc.result {
				case "error":
					response.Result = nil
					response.Error = json.RawMessage(`{"code":-32000,"message":"pinned state unavailable"}`)
				case "zero":
					response.Result = json.RawMessage(`"0x` + strings.Repeat("0", 64) + `"`)
				case "wide":
					response.Result = json.RawMessage(`"0x` + strings.Repeat("0", 61) + `100"`)
				case "one":
					response.Result = json.RawMessage(`"0x` + strings.Repeat("0", 63) + `1"`)
				case "short":
					response.Result = json.RawMessage(`"0x01"`)
				}
				input.Responses[key] = response
				changed++
			}
			if changed == 0 {
				t.Fatal("pinned read absent from producer transcript")
			}
			input = repair53Replay(t, input)
			a, closeClient := repair53Open(t, ctx, input)
			defer closeClient()
			if err = a.Rehydrate(ctx, input.Markets); err != nil {
				t.Fatal(err)
			}
			if _, err = a.current().StableMetadata(ctx, pool, bundle.BlockHash); err == nil {
				t.Fatal("unbound pinned metadata admitted")
			}
		})
	}
}

func TestPinnedReceiptFallbackAndMalformedResponseAdmission(t *testing.T) {
	ctx := context.Background()
	original := repair53InputsFromProducer(t)
	base, closeBase := repair53Open(t, ctx, original)
	defer closeBase()
	bundle, err := base.Bundle(ctx, original.NativeSwap)
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"individual receipts", "receipt unavailable", "receipt count", "receipt bloom", "receipt identity", "log identity", "log removed", "log topics", "header bloom"} {
		t.Run(mode, func(t *testing.T) {
			input, err := clone(original)
			if err != nil {
				t.Fatal(err)
			}
			changed := false
			for key, response := range input.Responses {
				if !strings.Contains(strings.ToLower(key), strings.ToLower(bundle.BlockHash.Hex())) {
					continue
				}
				if strings.HasPrefix(key, "eth_getBlockReceipts ") {
					var receipts []map[string]json.RawMessage
					if err = json.Unmarshal(response.Result, &receipts); err != nil {
						t.Fatal(err)
					}
					switch mode {
					case "individual receipts":
						response.Result = nil
						response.Error = json.RawMessage(`{"code":-32601,"message":"method unavailable"}`)
					case "receipt unavailable":
						response.Result = nil
						response.Error = json.RawMessage(`{"code":-32000,"message":"receipt unavailable"}`)
					case "receipt count":
						receipts = nil
					case "receipt bloom":
						receipts[0]["logsBloom"], _ = json.Marshal("0x" + strings.Repeat("0", 512))
					case "receipt identity":
						receipts[0]["blockHash"], _ = json.Marshal(common.HexToHash("0x1"))
					case "log identity", "log removed", "log topics":
						var logs []map[string]json.RawMessage
						if err = json.Unmarshal(receipts[0]["logs"], &logs); err != nil || len(logs) == 0 {
							t.Fatalf("recorded receipt logs %v", err)
						}
						switch mode {
						case "log identity":
							logs[0]["transactionHash"], _ = json.Marshal(common.HexToHash("0x1"))
						case "log removed":
							logs[0]["removed"] = json.RawMessage(`true`)
						case "log topics":
							logs[0]["topics"], _ = json.Marshal(make([]common.Hash, 5))
						}
						receipts[0]["logs"], _ = json.Marshal(logs)
					}
					if mode != "individual receipts" && mode != "receipt unavailable" {
						response.Result, err = json.Marshal(receipts)
						if err != nil {
							t.Fatal(err)
						}
					}
					input.Responses[key] = response
					changed = true
				}
				if mode == "header bloom" && strings.HasPrefix(key, "eth_getBlockByHash ") {
					var block map[string]json.RawMessage
					if err = json.Unmarshal(response.Result, &block); err != nil {
						t.Fatal(err)
					}
					block["logsBloom"], _ = json.Marshal("0x" + strings.Repeat("0", 512))
					response.Result, err = json.Marshal(block)
					if err != nil {
						t.Fatal(err)
					}
					input.Responses[key] = response
					changed = true
				}
			}
			if !changed {
				t.Fatal("block receipts absent from producer transcript")
			}
			input = repair53Replay(t, input)
			a, closeClient := repair53Open(t, ctx, input)
			defer closeClient()
			_, receipts, err := a.pinnedBlock(ctx, bundle.BlockHash)
			if mode == "individual receipts" {
				if err != nil || len(receipts) == 0 {
					t.Fatalf("individual receipt fallback %v", err)
				}
			} else if err == nil {
				t.Fatal("malformed pinned receipts admitted")
			}
		})
	}
}
