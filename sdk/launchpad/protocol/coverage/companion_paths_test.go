package coverage

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestReviewedCompanionEventsMatchTheirNativeFields(t *testing.T) {
	registry, abis, addresses := sourceRegistry(t)
	pool := addresses["SidioraPool"]
	party := common.HexToAddress("0xabcd")
	token := addresses["SidioraERC20"]
	nft := addresses["SidioraNFT"]
	feeNative := map[string]any{
		"pool": pool, "recipient": party, "holder": party, "amount": big.NewInt(10),
		"feeAmount": big.NewInt(10), "poolCut": big.NewInt(3), "protocolCut": big.NewInt(7),
		"totalAmount": big.NewInt(10), "epoch": big.NewInt(2),
	}
	feeMirror := map[string]any{
		"pool": pool, "party": party, "amount": big.NewInt(10),
		"poolCut": big.NewInt(3), "protocolCut": big.NewInt(7), "epoch": big.NewInt(2),
	}
	for _, item := range []struct {
		name string
		kind uint8
	}{
		{"OpticalSurplusRecorded", 7}, {"OpticalSurplusClaimed", 8},
		{"FeeRecorded", 0}, {"FeesBurned", 2}, {"LpRewardsSent", 5},
		{"AirdropClaimed", 4}, {"AirdropTriggered", 3}, {"FeesClaimed", 1},
	} {
		t.Run(item.name, func(t *testing.T) {
			mirrorFields := make(map[string]any, len(feeMirror)+1)
			for key, value := range feeMirror {
				mirrorFields[key] = value
			}
			mirrorFields["kind"] = item.kind
			if item.name == "LpRewardsSent" {
				mirrorFields["party"] = pool
			}
			native := registry.Decode(125, eventLog(t, addresses["FeeAccumulator"], abis["FeeAccumulator"].Events[item.name], feeNative))
			mirror := registry.Decode(125, eventLog(t, addresses["EventEmitter"], abis["EventEmitter"].Events["FeeFlow"], mirrorFields))
			if native.Status != statusDecoded || mirror.Status != statusDecoded {
				t.Fatalf("reviewed companion did not decode: %s %s", native.Status, mirror.Status)
			}
			effect := Effect{Role: "FeeAccumulator", Event: item.name, Fields: native.Fields}
			record := Record{Event: "FeeFlow", Decoded: mirror}
			if !companionMatches(effect, record) {
				t.Fatal("matching native and emitter fields did not associate")
			}
			record.Decoded.Fields["amount"] = big.NewInt(11)
			if companionMatches(effect, record) {
				t.Fatal("different amount associated with native effect")
			}
		})
	}
	for _, item := range []struct {
		role, nativeEvent, mirrorEvent string
		address                        common.Address
		native, mirror                 map[string]any
	}{
		{"SidioraERC20", "Transfer", "TokenTransfer", token, map[string]any{"from": party, "to": pool, "value": big.NewInt(7)}, map[string]any{"token": token, "from": party, "to": pool, "value": big.NewInt(7)}},
		{"SidioraNFT", "Transfer", "NftTransfer", nft, map[string]any{"from": party, "to": pool, "tokenId": big.NewInt(7)}, map[string]any{"nft": nft, "from": party, "to": pool, "tokenId": big.NewInt(7)}},
		{"PECOR", "SimpleSwap", "PecorSwap", addresses["PECOR"], map[string]any{"user": party, "tokenIn": token, "tokenOut": nft, "amountIn": big.NewInt(7), "amountOut": big.NewInt(6)}, map[string]any{"user": party, "tokenIn": token, "tokenOut": nft, "amountIn": big.NewInt(7), "amountOut": big.NewInt(6)}},
		{"PECORVault", "Deposit", "VaultFlow", addresses["PECORVault"], map[string]any{"token": token, "depositor": party, "amount": big.NewInt(7), "newReserve": big.NewInt(9)}, map[string]any{"flowType": uint8(0), "token": token, "party": party, "amount": big.NewInt(7), "newReserve": big.NewInt(9)}},
	} {
		t.Run(item.role, func(t *testing.T) {
			native := registry.Decode(125, eventLog(t, item.address, abis[item.role].Events[item.nativeEvent], item.native))
			mirror := registry.Decode(125, eventLog(t, addresses["EventEmitter"], abis["EventEmitter"].Events[item.mirrorEvent], item.mirror))
			if native.Status != statusDecoded || mirror.Status != statusDecoded {
				t.Fatalf("reviewed companion did not decode: %s %s", native.Status, mirror.Status)
			}
			if !companionMatches(Effect{Role: item.role, Event: item.nativeEvent, Identity: Identity{Emitter: item.address}, Fields: native.Fields}, Record{Event: item.mirrorEvent, Decoded: mirror}) {
				t.Fatal("matching native and emitter fields did not associate")
			}
		})
	}
	creationFields := map[string]any{"token": token, "pool": pool, "creator": party, "nftId": big.NewInt(7), "optical": nft}
	creation := registry.Decode(125, eventLog(t, addresses["SidioraFactory"], abis["SidioraFactory"].Events["MarketCreated"], creationFields))
	if creation.Status != statusDecoded {
		t.Fatal("reviewed factory creation did not decode")
	}
	for _, item := range []struct{ role string }{{"Router"}, {"EventEmitter"}} {
		mirror := registry.Decode(125, eventLog(t, addresses[item.role], abis[item.role].Events["MarketCreated"], creationFields))
		if mirror.Status != statusDecoded || !companionMatches(Effect{Role: "SidioraFactory", Event: "MarketCreated", Fields: creation.Fields}, Record{Event: "MarketCreated", Decoded: mirror}) {
			t.Fatalf("%s creation mirror did not match reviewed factory fields", item.role)
		}
	}
	if companionMatches(Effect{Role: "SidioraFactory", Event: "MarketCreated", Fields: creation.Fields}, Record{Event: "Unknown"}) {
		t.Fatal("unrelated event matched a creation")
	}
	var observed ReceiptResult
	observed.Effects = []Effect{{Role: "Router", Event: "Buy", Kind: "route_execution", Fields: map[string]any{"pool": pool, "buyer": party, "usdlIn": big.NewInt(7), "tokensOut": big.NewInt(6)}}}
	observed.correlateRoutes()
	if !containsDiagnostic(observed.Diagnostics, "route_leg_mismatch") {
		t.Fatal("route without a pool fill gained a companion")
	}
	observed.Diagnostics = nil
	observed.Effects = append(observed.Effects, Effect{Role: roleEventEmitter, Event: "Swap", Kind: effectCoreFill, Fields: map[string]any{"pool": pool, "isBuy": "unknown direction", "sender": party, "amountIn": big.NewInt(7), "amountOut": big.NewInt(6)}})
	observed.correlateRoutes()
	if !containsDiagnostic(observed.Diagnostics, "route_leg_mismatch") || len(observed.Effects[1].Companions) != 0 {
		t.Fatal("pool fill without a trade direction matched a route")
	}
	observed.Diagnostics = nil
	observed.Effects[1].Fields["isBuy"] = true
	observed.Effects[1].Fields["amountOut"] = big.NewInt(5)
	observed.correlateRoutes()
	if !containsDiagnostic(observed.Diagnostics, "route_leg_mismatch") || len(observed.Effects[1].Companions) != 0 {
		t.Fatal("route with a different output matched a pool fill")
	}
}
