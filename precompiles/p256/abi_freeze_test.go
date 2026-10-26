package p256

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

func GetABI(t *testing.T) abi.ABI {
	t.Helper()
	data, err := f.ReadFile("abi.json")
	if err != nil {
		t.Fatalf("read embedded abi.json: %v", err)
	}
	parsed, err := abi.JSON(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("parse embedded abi.json: %v", err)
	}
	return parsed
}

func TestP256ABIFrozen(t *testing.T) {
	embedded := GetABI(t)
	p, err := NewPrecompile(nil)
	if err != nil {
		t.Fatalf("construct p256 precompile: %v", err)
	}
	if got, want := p.Address(), common.HexToAddress("0x0000000000000000000000000000000000001011"); got != want {
		t.Errorf("p256 address = %s, want %s", got.Hex(), want.Hex())
	}
	served := p.GetABI()
	if !reflect.DeepEqual(served, embedded) {
		t.Error("served p256 ABI differs from embedded abi.json")
	}

	wantMethods := map[string]string{
		"verify(bytes)": "0x8e760afe",
	}
	wantEvents := map[string]common.Hash{}
	for source, parsed := range map[string]abi.ABI{"embedded": embedded, "served": served} {
		t.Run(source, func(t *testing.T) {
			if got := len(parsed.Methods); got != len(wantMethods) {
				t.Errorf("method count = %d, want %d", got, len(wantMethods))
			}
			methods := make(map[string]string, len(parsed.Methods))
			for _, method := range parsed.Methods {
				selector := fmt.Sprintf("0x%x", method.ID)
				t.Logf("method %s selector %s", method.Sig, selector)
				methods[method.Sig] = selector
				if len(method.ID) != 4 {
					t.Errorf("method %s selector has %d bytes, want 4", method.Sig, len(method.ID))
				}
				want, ok := wantMethods[method.Sig]
				if !ok {
					t.Errorf("unexpected method %s with selector %s", method.Sig, selector)
				} else if selector != want {
					t.Errorf("method %s selector = %s, want %s", method.Sig, selector, want)
				}
			}
			for signature, selector := range wantMethods {
				if _, ok := methods[signature]; !ok {
					t.Errorf("missing method %s with frozen selector %s", signature, selector)
				}
			}

			if got := len(parsed.Events); got != len(wantEvents) {
				t.Errorf("event count = %d, want %d", got, len(wantEvents))
			}
			events := make(map[string]common.Hash, len(parsed.Events))
			for _, event := range parsed.Events {
				t.Logf("event %s topic %s", event.Sig, event.ID.Hex())
				events[event.Sig] = event.ID
				want, ok := wantEvents[event.Sig]
				if !ok {
					t.Errorf("unexpected event %s with topic %s", event.Sig, event.ID.Hex())
				} else if event.ID != want {
					t.Errorf("event %s topic = %s, want %s", event.Sig, event.ID.Hex(), want.Hex())
				}
			}
			for signature, topic := range wantEvents {
				if _, ok := events[signature]; !ok {
					t.Errorf("missing event %s with frozen topic %s", signature, topic.Hex())
				}
			}
		})
	}
}
