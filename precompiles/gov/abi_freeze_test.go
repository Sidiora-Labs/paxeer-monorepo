package gov_test

import (
	_ "embed"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/gov"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

//go:embed abi.json
var govFrozenABIJSON string

func TestGovABIFrozen(t *testing.T) {
	precompile, err := gov.NewPrecompile(testkeeper.EVMTestApp.GetPrecompileKeepers())
	if err != nil {
		t.Fatalf("construct gov precompile: %v", err)
	}
	if got, want := precompile.Address(), common.HexToAddress("0x0000000000000000000000000000000000001006"); got != want {
		t.Errorf("gov precompile address = %s, want %s", got.Hex(), want.Hex())
	}

	embeddedABI, err := abi.JSON(strings.NewReader(govFrozenABIJSON))
	if err != nil {
		t.Fatalf("parse embedded abi.json: %v", err)
	}
	var servedABI abi.ABI = precompile.GetABI()
	if !reflect.DeepEqual(servedABI, embeddedABI) {
		t.Error("gov GetABI() differs from embedded abi.json")
	}

	wantMethods := map[string]string{
		"deposit(uint64)":                       "0x13765838",
		"submitProposal(string)":                "0x373d6d5e",
		"vote(uint64,int32)":                    "0x833f8a9b",
		"voteWeighted(uint64,(int32,string)[])": "0x59634a88",
	}
	wantEvents := map[string]string{}
	if len(servedABI.Methods) != len(wantMethods) {
		t.Errorf("gov method count = %d, want %d", len(servedABI.Methods), len(wantMethods))
	}
	gotMethods := make(map[string]string, len(servedABI.Methods))
	for _, method := range servedABI.Methods {
		selector := fmt.Sprintf("0x%x", method.ID)
		t.Logf("method %s selector %s", method.Sig, selector)
		gotMethods[method.Sig] = selector
		if want, ok := wantMethods[method.Sig]; !ok {
			t.Errorf("unexpected gov method %s with selector %s", method.Sig, selector)
		} else if len(method.ID) != 4 || selector != want {
			t.Errorf("gov method %s selector = %s, want %s (4 bytes)", method.Sig, selector, want)
		}
	}
	for signature, selector := range wantMethods {
		if _, ok := gotMethods[signature]; !ok {
			t.Errorf("missing gov method %s with frozen selector %s", signature, selector)
		}
	}

	if len(servedABI.Events) != len(wantEvents) {
		t.Errorf("gov event count = %d, want %d", len(servedABI.Events), len(wantEvents))
	}
	for _, event := range servedABI.Events {
		topic := event.ID.Hex()
		t.Logf("event %s topic %s", event.Sig, topic)
		if want, ok := wantEvents[event.Sig]; !ok {
			t.Errorf("unexpected gov event %s with topic %s", event.Sig, topic)
		} else if topic != want {
			t.Errorf("gov event %s topic = %s, want %s", event.Sig, topic, want)
		}
	}
}
