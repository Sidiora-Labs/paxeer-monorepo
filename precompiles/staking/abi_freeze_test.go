package staking_test

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/staking"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

func GetABI(t *testing.T) abi.ABI {
	t.Helper()
	precompile, err := staking.NewPrecompile(testkeeper.EVMTestApp.GetPrecompileKeepers())
	if err != nil {
		t.Fatalf("construct staking precompile: %v", err)
	}
	if got, want := precompile.Address(), common.HexToAddress("0x0000000000000000000000000000000000001005"); got != want {
		t.Errorf("staking precompile address = %s, want %s", got.Hex(), want.Hex())
	}
	return precompile.GetABI()
}

func TestStakingABIFrozen(t *testing.T) {
	served := GetABI(t)
	data, err := f.ReadFile("abi.json")
	if err != nil {
		t.Fatalf("read embedded staking abi.json: %v", err)
	}
	embedded, err := abi.JSON(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("parse embedded staking abi.json: %v", err)
	}
	if !reflect.DeepEqual(served, embedded) {
		t.Error("staking precompile GetABI() differs from embedded abi.json")
	}

	wantMethods := map[string]string{
		"createValidator(string,string,string,string,string,uint256)": "0xefec2dde",
		"delegate(string)":                             "0x9ddb511a",
		"delegation(address,string)":                   "0x241774e6",
		"delegatorDelegations(address,bytes)":          "0x4e762a8c",
		"delegatorUnbondingDelegations(address,bytes)": "0xe39fcf43",
		"delegatorValidator(address,string)":           "0xf4fb6391",
		"delegatorValidators(address,bytes)":           "0x21860cc0",
		"editValidator(string,string,uint256)":         "0x5c501c10",
		"historicalInfo(int64)":                        "0xc5e6e7d4",
		"params()":                                     "0xcff0ab96",
		"pool()":                                       "0x16f0115b",
		"redelegate(string,string,uint256)":            "0x7dd0209d",
		"redelegations(string,string,string,bytes)":    "0x7ac3ee49",
		"unbondingDelegation(address,string)":          "0xa03ffee1",
		"undelegate(string,uint256)":                   "0x8dfc8897",
		"validator(string)":                            "0x0bc82a17",
		"validatorDelegations(string,bytes)":           "0xb93ccf61",
		"validatorUnbondingDelegations(string,bytes)":  "0x4c6e766b",
		"validators(string,bytes)":                     "0x99d080fa",
	}
	wantEvents := map[string]string{
		"Delegate(address,string,uint256)":                   "0x00e1ef0aa5cbcdc2ae3ce2840984590ab429a9579c9a4b89ca93d77105d40e4a",
		"DelegationRewardsWithdrawn(address,string,uint256)": "0xe9a2d1cd042608da4ebdf1fafb4a4658dfc6954ce121138e784b3f9372505d11",
		"Redelegate(address,string,string,uint256)":          "0x1b26c9b9360749ef957cd7ebe494c66814ab181ada52377ae3036fa24e9befa7",
		"Undelegate(address,string,uint256)":                 "0xc5c77bd61c771d282f85e9d57c074cc5da688158db5f4422a376569a2c69a573",
		"ValidatorCreated(address,string,string)":            "0xb07ac1a8f86c67f4f2d948b6d93c8cedb3a821e5ee79475a9bbc1269a8753f37",
		"ValidatorEdited(address,string,string)":             "0x3ac1616ab72092b243fb318e94d11451de40a59822d4afc8bbb58f74710da225",
	}

	if got := len(served.Methods); got != 19 {
		t.Errorf("staking method count = %d, want 19", got)
	}
	gotMethods := make(map[string]string, len(served.Methods))
	for _, method := range served.Methods {
		selector := fmt.Sprintf("0x%x", method.ID)
		t.Logf("method %s = %s", method.Sig, selector)
		if len(method.ID) != 4 {
			t.Errorf("method %s selector has %d bytes, want 4", method.Sig, len(method.ID))
		}
		gotMethods[method.Sig] = selector
		if _, ok := wantMethods[method.Sig]; !ok {
			t.Errorf("unexpected staking method %s with selector %s", method.Sig, selector)
		}
	}
	for signature, want := range wantMethods {
		if got, ok := gotMethods[signature]; !ok {
			t.Errorf("missing staking method %s (frozen selector %s)", signature, want)
		} else if got != want {
			t.Errorf("staking method %s selector = %s, want %s", signature, got, want)
		}
	}

	if got := len(served.Events); got != 6 {
		t.Errorf("staking event count = %d, want 6", got)
	}
	gotEvents := make(map[string]string, len(served.Events))
	for _, event := range served.Events {
		topic := event.ID.Hex()
		t.Logf("event %s = %s", event.Sig, topic)
		gotEvents[event.Sig] = topic
		if _, ok := wantEvents[event.Sig]; !ok {
			t.Errorf("unexpected staking event %s with topic %s", event.Sig, topic)
		}
	}
	for signature, want := range wantEvents {
		if got, ok := gotEvents[signature]; !ok {
			t.Errorf("missing staking event %s (frozen topic %s)", signature, want)
		} else if got != want {
			t.Errorf("staking event %s topic = %s, want %s", signature, got, want)
		}
	}
}
