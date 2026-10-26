package ibc_test

import (
	"bytes"
	_ "embed"
	"fmt"
	"reflect"
	"testing"

	node "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/ibc"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

//go:embed abi.json
var ibcFrozenABIJSON []byte

func GetABI(t *testing.T) abi.ABI {
	t.Helper()
	parsed, err := abi.JSON(bytes.NewReader(ibcFrozenABIJSON))
	if err != nil {
		t.Fatalf("parse embedded ibc abi.json: %v", err)
	}
	return parsed
}

func TestIbcABIFrozen(t *testing.T) {
	precompile, err := ibc.NewPrecompile(&node.PrecompileKeepers{})
	if err != nil {
		t.Fatalf("construct ibc precompile: %v", err)
	}
	if got, want := precompile.Address(), common.HexToAddress("0x0000000000000000000000000000000000001009"); got != want {
		t.Errorf("ibc precompile address = %s, want %s", got.Hex(), want.Hex())
	}

	served := precompile.GetABI()
	if !reflect.DeepEqual(served, GetABI(t)) {
		t.Error("ibc precompile GetABI differs from embedded abi.json")
	}

	wantMethods := map[string]string{
		"transfer(string,string,string,string,uint256,uint64,uint64,uint64,string)": "0x98a955c7",
		"transferWithDefaultTimeout(string,string,string,string,uint256,string)":    "0x97ba602b",
	}
	gotMethods := make(map[string]string, len(served.Methods))
	for _, method := range served.Methods {
		selector := fmt.Sprintf("0x%x", method.ID)
		t.Logf("method %s: %s", method.Sig, selector)
		if len(method.ID) != 4 {
			t.Errorf("method %s selector has %d bytes, want 4", method.Sig, len(method.ID))
		}
		if _, duplicate := gotMethods[method.Sig]; duplicate {
			t.Errorf("duplicate method signature %s", method.Sig)
		}
		gotMethods[method.Sig] = selector
		if want, exists := wantMethods[method.Sig]; !exists {
			t.Errorf("unexpected method %s with selector %s", method.Sig, selector)
		} else if selector != want {
			t.Errorf("method %s selector = %s, want %s", method.Sig, selector, want)
		}
	}
	for signature, selector := range wantMethods {
		if _, exists := gotMethods[signature]; !exists {
			t.Errorf("missing method %s with selector %s", signature, selector)
		}
	}
	if len(served.Methods) != len(wantMethods) {
		t.Errorf("ibc method count = %d, want %d", len(served.Methods), len(wantMethods))
	}

	wantEvents := map[string]string{}
	gotEvents := make(map[string]string, len(served.Events))
	for _, event := range served.Events {
		topic := event.ID.Hex()
		t.Logf("event %s: %s", event.Sig, topic)
		gotEvents[event.Sig] = topic
	}
	if !reflect.DeepEqual(gotEvents, wantEvents) {
		t.Errorf("ibc event topics = %v, want %v", gotEvents, wantEvents)
	}
	if len(served.Events) != len(wantEvents) {
		t.Errorf("ibc event count = %d, want %d", len(served.Events), len(wantEvents))
	}
}
