package coverage

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

func TestReviewedTypedEmitterSelectorsAndCanonicalPermissionEpochs(t *testing.T) {
	seen := map[[4]byte]bool{}
	for _, name := range []string{"Swap", "ContractUpgraded", "CircuitBreakerTriggered"} {
		selector, ok := TypedSelector(name)
		if !ok || selector == ([4]byte{}) || seen[selector] {
			t.Fatalf("reviewed %s selector absent or duplicated", name)
		}
		seen[selector] = true
	}
	if _, ok := TypedSelector("UnknownEvent"); ok {
		t.Fatal("unreviewed event selector accepted")
	}
	emitter, caller := common.HexToAddress("0x12"), common.HexToAddress("0x34")
	selector, _ := TypedSelector("Swap")
	from := Position{Block: 12, Transaction: 1, Log: 2}
	until := Position{Block: 12, Transaction: 2}
	epoch := PermissionEpoch{ChainID: 125, Emitter: emitter, Caller: caller, Selector: selector, From: from, Until: &until}
	if !CallerAllowedAt([]PermissionEpoch{epoch}, 125, emitter, caller, selector, from) {
		t.Fatal("canonical permission start rejected")
	}
	if CallerAllowedAt([]PermissionEpoch{epoch, epoch}, 125, emitter, caller, selector, from) {
		t.Fatal("ambiguous permission epochs admitted")
	}
	reversed := epoch
	reversed.From = until
	reversed.Until = &from
	if CallerAllowedAt([]PermissionEpoch{reversed}, 125, emitter, caller, selector, from) {
		t.Fatal("reversed permission epoch admitted")
	}
	if CallerAllowedAt(nil, 1, emitter, caller, selector, from) || CallerAllowed(nil, 125, common.Address{}, caller, selector, 12) {
		t.Fatal("unbound permission admitted")
	}
	if CoreSwapCoverage(true) != "authorized_mirror_required" {
		t.Fatal("authorized swap coverage state differs")
	}
}

func TestReviewedAuthorityClassifiesControlReserveAndRouteEvents(t *testing.T) {
	for _, item := range []struct {
		role, event, kind, effect string
		callerRequired            bool
	}{
		{roleEventEmitter, "RoleGranted", kindPrimary, effectControlTransition, false},
		{roleEventEmitter, "PoolStateUpdated", kindConditionalPrimary, effectReserveState, true},
		{roleEventEmitter, "ConfigUpdated", kindConditionalPrimary, "configuration", true},
		{"MetaAGRouter", "BestRouteSwap", kindPrimary, "route_execution", false},
		{rolePECORVault, "TokenRegistered", kindPrimary, "asset_configuration", false},
	} {
		rule := Authority(item.role, item.event)
		if rule.Kind != item.kind || rule.Effect != item.effect || rule.RequiresCallerPermission != item.callerRequired {
			t.Fatalf("%s:%s authority differs: %+v", item.role, item.event, rule)
		}
	}
}

func TestReviewedPermissionHistoryRejectsEmptyCallerAndZeroSelector(t *testing.T) {
	emitter := common.HexToAddress("0x12")
	caller := common.HexToAddress("0x34")
	selector, ok := TypedSelector("Swap")
	if !ok {
		t.Fatal("reviewed swap selector absent")
	}
	position := Position{Block: 12, Transaction: 1, Log: 2}
	epoch := PermissionEpoch{ChainID: 125, Emitter: emitter, Caller: caller, Selector: selector, From: position}
	for _, item := range []struct {
		emitter, caller common.Address
		selector        [4]byte
	}{
		{common.Address{}, caller, selector},
		{emitter, common.Address{}, selector},
		{emitter, caller, [4]byte{}},
	} {
		if CallerAllowedAt([]PermissionEpoch{epoch}, 125, item.emitter, item.caller, item.selector, position) {
			t.Fatal("permission without complete caller identity admitted")
		}
	}
	if CallerAllowedAt(nil, 125, emitter, caller, selector, position) {
		t.Fatal("permission absent from history admitted")
	}
	if !CallerAllowedAt([]PermissionEpoch{epoch}, 125, emitter, caller, selector, position) {
		t.Fatal("reviewed permission epoch rejected")
	}
}

func TestMeasurementRPCFailureRetainsContextAndHidesTransportDetails(t *testing.T) {
	for _, cause := range []error{context.Canceled, context.DeadlineExceeded} {
		mapped := measurementRPCFailure("eth_call", cause)
		if !errors.Is(mapped, cause) || !strings.Contains(mapped.Error(), "eth_call") {
			t.Fatalf("context error identity lost: %v", mapped)
		}
	}
	if mapped := measurementRPCFailure("eth_call", errors.New("private transport detail")); strings.Contains(mapped.Error(), "private transport detail") || !strings.Contains(mapped.Error(), "historical RPC unavailable") {
		t.Fatalf("transport detail reached caller: %v", mapped)
	}
}
