package decode

import (
	"math/big"
	"reflect"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func TestOptionalCapabilitiesRequireActiveBindingsAndExposeOracleUnits(t *testing.T) {
	body, catalog := frozen(t)
	ds := []Deployment{}
	var emitter Deployment
	for i, c := range catalog.Contracts {
		if !optionalRole(c.Name) && c.Name != "EventEmitter" {
			continue
		}
		d := Deployment{Address: common.BigToAddress(big.NewInt(int64(i + 1))), Role: c.Name, ABIRevision: c.ABIRevision, Optional: optionalRole(c.Name), Enabled: true}
		if c.Name == "EventEmitter" {
			a, err := abi.JSON(strings.NewReader(string(c.ABI)))
			if err != nil {
				t.Fatal(err)
			}
			for _, e := range a.Events {
				if optionalEvent(e.Name) {
					d.EnabledOptionalEvents = append(d.EnabledOptionalEvents, e.Name)
				}
			}
			emitter = d
		}
		ds = append(ds, d)
	}
	r, err := NewRegistry(body, ds)
	if err != nil {
		t.Fatal(err)
	}
	policies := []CapabilityPolicy{}
	for _, family := range []string{"pecor", "orders", "vault", "routing", "tracker", "oracle"} {
		policies = append(policies, CapabilityPolicy{Family: family, ProjectionReady: true, PublicEnabled: true})
	}
	available, err := r.OptionalAvailability(Order{Block: 70}, policies)
	if err != nil || len(available) != 6 {
		t.Fatalf("optional availability: %+v %v", available, err)
	}
	for _, a := range available {
		if !a.Available || !a.DeploymentPresent || !a.ABISupported || !a.CoverageReady || len(a.Missing) != 0 {
			t.Fatalf("bound optional capability: %+v", a)
		}
	}
	var nilRegistry *Registry
	if _, err = nilRegistry.OptionalAvailability(Order{}, policies); err == nil {
		t.Fatal("absent registry offered capabilities")
	}
	if _, err = r.OptionalAvailability(Order{}, append(policies, policies[0])); err == nil {
		t.Fatal("duplicate capability accepted")
	}
	for _, d := range ds {
		if d.Role != "PriceOracle" && d.Role != "OracleHub" {
			continue
		}
		_, a := deployment(t, d.Role, d.Address)
		for _, event := range a.Events {
			log, want := syntheticLog(t, event, d.Address)
			observed, err := r.DecodeOracle(log)
			if err != nil || observed.Event != event.Name || !observed.Indicative || observed.Identity.LogIndex != log.Index || observed.ABIRevision != d.ABIRevision {
				t.Fatalf("oracle %s: %+v %v", event.Name, observed, err)
			}
			if value, ok := want["price"].(string); ok {
				if observed.RawValue == nil || *observed.RawValue != value {
					t.Fatal("oracle exact price lost")
				}
				if d.Role == "PriceOracle" && event.Name == "PriceUpdated" && (observed.Scale == nil || *observed.Scale != 18) {
					t.Fatal("oracle price scale lost")
				}
			} else if observed.Status != "lifecycle" {
				t.Fatalf("oracle lifecycle status %s", observed.Status)
			}
		}
	}
	_, a := deployment(t, "EventEmitter", emitter.Address)
	invalid, _ := syntheticLog(t, a.Events["Swap"], emitter.Address)
	if _, err = r.DecodeOracle(invalid); err == nil {
		t.Fatal("core event offered as oracle telemetry")
	}
	for _, d := range ds {
		if d.Role != "PECOR" {
			continue
		}
		_, a := deployment(t, d.Role, d.Address)
		log, _ := syntheticLog(t, a.Events["SimpleSwap"], d.Address)
		batch, err := r.DecodeOptionalBatch([]types.Log{log, log})
		if err != nil || len(batch.Optional) != 1 || len(batch.Core) != 0 || len(batch.Failures) != 0 {
			t.Fatalf("optional dedup: %+v %v", batch, err)
		}
		log.Data = []byte{1}
		batch, err = r.DecodeOptionalBatch([]types.Log{log})
		if err != nil || len(batch.Failures) != 1 || batch.Failures[0].Family != "pecor" {
			t.Fatalf("optional quarantine: %+v %v", batch, err)
		}
	}
	if _, err = nilRegistry.DecodeOptionalBatch(nil); err == nil {
		t.Fatal("absent optional decoder accepted")
	}
}

func TestOptionalCompanionsMatchExactActionAndEconomicFields(t *testing.T) {
	base := func() map[string]any {
		return map[string]any{"orderId": "1", "orderType": "0", "user": "user", "amount": "7", "targetPrice": "9", "tokenIn": "in", "tokenOut": "out", "amountIn": "11", "amountOut": "13", "executionPrice": "17", "activationPrice": "19", "stopPrice": "21", "limitPrice": "23", "token": "token", "newReserve": "29", "nativeAmount": "31", "tokenAmount": "37", "stablecoinAmount": "41", "stablecoin": "stable", "depositor": "party", "recipient": "party", "recovered": "43", "newReserves": "47"}
	}
	cases := []struct {
		role, event, companionRole, companion string
		native, mirror                        map[string]any
		wrapped                               common.Address
	}{
		{"PECOROrders", "LimitOrderCreated", "TransactionTracker", "LimitOrderPlaced", map[string]any{"orderType": "0"}, map[string]any{"isBuy": true}, common.Address{}},
		{"PECOROrders", "LimitOrderCreated", "TransactionTracker", "StopLossPlaced", map[string]any{"orderType": "2"}, map[string]any{"tokenAmount": "7", "triggerPrice": "9"}, common.Address{}},
		{"PECOROrders", "StopLimitOrderCreated", "TransactionTracker", "StopLimitPlaced", nil, nil, common.Address{}},
		{"PECOROrders", "LimitOrderExecuted", "TransactionTracker", "LimitOrderExecuted", nil, nil, common.Address{}},
		{"PECOROrders", "StopLimitExecuted", "TransactionTracker", "StopLimitExecuted", nil, nil, common.Address{}},
		{"PECOROrders", "StopLimitActivated", "TransactionTracker", "StopLimitActivated", nil, nil, common.Address{}},
		{"PECOROrders", "LimitOrderCancelled", "TransactionTracker", "LimitOrderCancelled", nil, nil, common.Address{}},
		{"PECOROrders", "LimitOrderCreated", "EventEmitter", "PecorOrderCreated", map[string]any{"orderType": "0"}, map[string]any{"orderType": "0", "orderKind": "0"}, common.Address{}},
		{"PECOROrders", "StopLimitOrderCreated", "EventEmitter", "PecorOrderCreated", nil, map[string]any{"orderKind": "1"}, common.Address{}},
		{"PECOROrders", "LimitOrderCancelled", "EventEmitter", "PecorOrderLifecycle", nil, map[string]any{"orderKind": "0", "phase": "2"}, common.Address{}},
		{"PECOROrders", "StopLimitCancelled", "EventEmitter", "PecorOrderLifecycle", nil, map[string]any{"orderKind": "1", "phase": "2"}, common.Address{}},
		{"PECOROrders", "LimitOrderExecuted", "EventEmitter", "PecorOrderLifecycle", nil, map[string]any{"orderKind": "0", "phase": "1", "price": "17"}, common.Address{}},
		{"PECOROrders", "StopLimitExecuted", "EventEmitter", "PecorOrderLifecycle", nil, map[string]any{"orderKind": "1", "phase": "1", "price": "17"}, common.Address{}},
		{"PECOROrders", "StopLimitActivated", "EventEmitter", "PecorOrderLifecycle", nil, map[string]any{"orderKind": "1", "phase": "0", "price": "19"}, common.Address{}},
		{"PECOR", "SimpleSwap", "TransactionTracker", "TradeExecuted", nil, nil, common.Address{}},
		{"PECOR", "SimpleSwap", "EventEmitter", "PecorSwap", nil, map[string]any{"swapKind": "1"}, common.Address{}},
		{"PECOR", "MarketOrderExecuted", "TransactionTracker", "MarketTrade", map[string]any{"isBuy": true}, map[string]any{"isBuy": true}, common.Address{}},
		{"PECOR", "MarketOrderExecuted", "EventEmitter", "PecorSwap", map[string]any{"isBuy": true}, map[string]any{"swapKind": "2", "tokenIn": "stable", "tokenOut": "token", "amountIn": "41", "amountOut": "37"}, common.Address{}},
		{"PECOR", "MarketOrderExecuted", "TransactionTracker", "TradeExecuted", map[string]any{"isBuy": false}, map[string]any{"tokenIn": "token", "tokenOut": "stable", "amountIn": "37", "amountOut": "41"}, common.Address{}},
		{"PECOR", "NativeSwap", "EventEmitter", "PecorSwap", map[string]any{"nativeIsInput": true, "tokenOther": "other"}, map[string]any{"swapKind": "3", "tokenIn": strings.ToLower(common.HexToAddress("0x125").Hex()), "tokenOut": "other", "amountIn": "31", "amountOut": "37"}, common.HexToAddress("0x125")},
		{"PECOR", "NativeSwap", "TransactionTracker", "TradeExecuted", map[string]any{"nativeIsInput": false, "tokenOther": "other"}, map[string]any{"tokenOut": strings.ToLower(common.HexToAddress("0x125").Hex()), "tokenIn": "other", "amountOut": "31", "amountIn": "37"}, common.HexToAddress("0x125")},
		{"MetaAGRouter", "BestRouteSwap", "EventEmitter", "BestRouteSwap", map[string]any{"adapterId": "route"}, map[string]any{"routeId": "route"}, common.Address{}},
		{"PECORVault", "Deposit", "TransactionTracker", "LiquidityAdded", nil, map[string]any{"provider": "party"}, common.Address{}},
		{"PECORVault", "Withdrawal", "TransactionTracker", "LiquidityRemoved", nil, map[string]any{"remover": "party"}, common.Address{}},
	}
	for _, flow := range []struct{ event, flow string }{{"Deposit", "0"}, {"Withdrawal", "1"}, {"NativeDeposit", "2"}, {"NativeWithdrawal", "3"}, {"EmergencyWithdraw", "4"}, {"ReservesSync", "5"}} {
		mirror := map[string]any{"flowType": flow.flow, "party": "party"}
		if flow.flow == "2" || flow.flow == "3" {
			mirror["token"] = (common.Address{}).Hex()
		}
		if flow.flow == "5" {
			mirror["newReserve"] = "47"
			mirror["amount"] = "43"
		}
		cases = append(cases, struct {
			role, event, companionRole, companion string
			native, mirror                        map[string]any
			wrapped                               common.Address
		}{"PECORVault", flow.event, "EventEmitter", "VaultFlow", nil, mirror, common.Address{}})
	}
	for _, tc := range cases {
		t.Run(tc.role+"-"+tc.event+"-"+tc.companionRole, func(t *testing.T) {
			n, c := base(), base()
			for k, v := range tc.native {
				n[k] = v
			}
			for k, v := range tc.mirror {
				c[k] = v
			}
			native := &DecodedEnvelope{EmitterRole: tc.role, EventName: tc.event, Event: &Event{Args: n}}
			companion := &DecodedEnvelope{EmitterRole: tc.companionRole, EventName: tc.companion, Event: &Event{Args: c}}
			if !companionMatches(native, companion, tc.wrapped) {
				t.Fatalf("bound companion rejected %v/%v", n, c)
			}
			companion.EventName = "Unrelated"
			if companionMatches(native, companion, tc.wrapped) {
				t.Fatal("unrelated companion accepted")
			}
			native.EventName = "Unrelated"
			if companionMatches(native, companion, tc.wrapped) {
				t.Fatal("unrelated native action accepted")
			}
		})
	}
	if equalFields(map[string]any{"v": "1"}, map[string]any{}, "v") {
		t.Fatal("missing correlation field matched")
	}
	if !reflect.DeepEqual(eventActors(&DecodedEnvelope{}).EvidenceIdentity, []RawIdentity{{}}) {
		t.Fatal("unavailable actors lost their raw identity")
	}
}
