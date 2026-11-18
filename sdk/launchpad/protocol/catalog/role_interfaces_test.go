package catalog

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
)

func TestCatalogCarriesRoleInterfaceSchemas(t *testing.T) {
	bundles, err := Bundles()
	if err != nil {
		t.Fatal(err)
	}
	byName := make(map[string]abi.ABI)
	for _, bundle := range bundles {
		parsed, err := abi.JSON(bytes.NewReader(bundle.ABI))
		if err != nil {
			t.Fatal(err)
		}
		byName[bundle.Name] = parsed
	}
	native, err := NativeLaunchpadABI()
	if err != nil {
		t.Fatal(err)
	}
	byName["ChainLaunch"] = native
	paths, err := filepath.Glob("../../../../precompiles/launchpad/abi/*.json")
	if err != nil || len(paths) != 12 {
		t.Fatal("role interface inventory differs", len(paths), err)
	}
	for _, path := range paths {
		name := filepath.Base(path)
		name = name[:len(name)-len(".json")]
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			producer, err := abi.JSON(bytes.NewReader(raw))
			if err != nil {
				t.Fatal(err)
			}
			consumer, ok := byName[name]
			if !ok {
				t.Fatal("role interface has no catalog family")
			}
			for method, expected := range producer.Methods {
				if !reflect.DeepEqual(consumer.Methods[method], expected) {
					t.Errorf("method %s differs from its role interface", method)
				}
			}
			for event, expected := range producer.Events {
				if !reflect.DeepEqual(consumer.Events[event], expected) {
					t.Errorf("event %s differs from its role interface", event)
				}
			}
			for abiError, expected := range producer.Errors {
				if !reflect.DeepEqual(consumer.Errors[abiError], expected) {
					t.Errorf("error %s differs from its role interface", abiError)
				}
			}
			if name != "ChainLaunch" && (len(consumer.Methods) != len(producer.Methods) || len(consumer.Events) != len(producer.Events) || len(consumer.Errors) != len(producer.Errors)) {
				t.Fatal("catalog and role interface inventories differ")
			}
		})
	}
	index, err := Index()
	if err != nil {
		t.Fatal(err)
	}
	var indexed []struct {
		Identity Bundle
		Entries  []struct{ Kind, Signature, Selector string }
	}
	if err := json.Unmarshal(index, &indexed); err != nil || len(indexed) != len(bundles)+1 {
		t.Fatal("catalog index inventory differs", err)
	}
	for _, family := range indexed {
		parsed := byName[family.Identity.Name]
		if family.Identity.Name == "NativeLaunchpad" {
			parsed = native
		}
		for _, entry := range family.Entries {
			if entry.Kind == "event" {
				event, err := parsed.EventByID(common.HexToHash(entry.Selector))
				if err != nil || event.Sig != entry.Signature {
					t.Fatalf("indexed event differs: %s %s", family.Identity.Name, entry.Signature)
				}
			} else if entry.Kind == "function" {
				method, err := parsed.MethodById(common.FromHex(entry.Selector))
				if err != nil || method.Sig != entry.Signature {
					t.Fatalf("indexed function differs: %s %s", family.Identity.Name, entry.Signature)
				}
			} else {
				t.Fatal("unknown catalog entry kind", entry.Kind)
			}
		}
	}
}
