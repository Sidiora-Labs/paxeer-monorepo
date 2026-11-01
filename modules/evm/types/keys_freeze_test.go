package types_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
)

func TestEVMStorePrefixesFrozen(t *testing.T) {
	frozen := []struct {
		name   string
		value  byte
		unused bool
	}{
		{"EVMAddressToPaxAddressKeyPrefix", 0x01, false},
		{"PaxAddressToEVMAddressKeyPrefix", 0x02, false},
		{"StateKeyPrefix", 0x03, false},
		{"TransientStateKeyPrefix", 0x04, false},
		{"AccountTransientStateKeyPrefix", 0x05, false},
		{"TransientModuleStateKeyPrefix", 0x06, false},
		{"CodeKeyPrefix", 0x07, false},
		{"CodeHashKeyPrefix", 0x08, false},
		{"CodeSizeKeyPrefix", 0x09, false},
		{"NonceKeyPrefix", 0x0a, false},
		{"ReceiptKeyPrefix", 0x0b, false},
		{"WhitelistedCodeHashesForBankSendPrefix", 0x0c, false},
		{"BlockBloomPrefix", 0x0d, false},
		{"TxHashesPrefix", 0x0e, false},
		{"WhitelistedCodeHashesForDelegateCallPrefix", 0x0f, false},
		{"TxHashPrefix", 0x10, true},
		{"TxBloomPrefix", 0x11, true},
		{"ReplaySeenAddrPrefix", 0x12, false},
		{"ReplayedHeight", 0x13, false},
		{"ReplayInitialHeight", 0x14, false},
		{"PointerRegistryPrefix", 0x15, false},
		{"PointerCWCodePrefix", 0x16, false},
		{"PointerReverseRegistryPrefix", 0x17, false},
		{"AnteSurplusPrefix", 0x18, false},
		{"DeferredInfoPrefix", 0x19, false},
		{"LegacyBlockBloomCutoffHeightKey", 0x1a, false},
		{"BaseFeePerGasPrefix", 0x1b, false},
		{"NextBaseFeePerGasPrefix", 0x1c, false},
		{"EvmOnlyBlockBloomPrefix", 0x1d, false},
		{"ZeroStorageCleanupCheckpointKey", 0x1e, false},
		{"NonceBumpPrefix", 0x1f, false},
		{"EVMAddressToLayerXDidKeyPrefix", 0x20, false},
		{"LayerXDidToEVMAddressKeyPrefix", 0x21, false},
		{"LayerXBindNonceKeyPrefix", 0x22, false},
		{"AccountFeeDenomKeyPrefix", 0x23, false},
		{"AnteFeeTokenChargePrefix", 0x24, false},
	}

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate keys.go: caller information unavailable")
	}
	source, err := parser.ParseFile(token.NewFileSet(), filepath.Join(filepath.Dir(filename), "keys.go"), nil, 0)
	if err != nil {
		t.Fatalf("parse keys.go: %v", err)
	}

	// The first var block holds store prefixes; the next holds pointer subtypes.
	var prefixes *ast.GenDecl
	for _, declaration := range source.Decls {
		if group, ok := declaration.(*ast.GenDecl); ok && group.Tok == token.VAR {
			prefixes = group
			break
		}
	}
	if prefixes == nil {
		t.Fatal("keys.go: store prefix declaration block removed")
	}

	expected := make(map[string]byte)
	reserved := make(map[byte]string)
	for _, entry := range frozen {
		if entry.unused {
			reserved[entry.value] = entry.name
		} else {
			expected[entry.name] = entry.value
		}
	}
	actual := make(map[string]byte)
	owners := make(map[byte]string)
	for _, declaration := range prefixes.Specs {
		spec := declaration.(*ast.ValueSpec)
		for i, name := range spec.Names {
			if _, exists := expected[name.Name]; !exists {
				t.Errorf("added store prefix %s", name.Name)
			}
			if i >= len(spec.Values) {
				t.Errorf("changed store prefix %s: missing byte-slice initializer", name.Name)
				continue
			}
			literal, ok := spec.Values[i].(*ast.CompositeLit)
			if !ok {
				t.Errorf("changed store prefix %s: expected a one-byte slice literal", name.Name)
				continue
			}
			slice, ok := literal.Type.(*ast.ArrayType)
			if !ok {
				t.Errorf("changed store prefix %s: expected []byte", name.Name)
				continue
			}
			element, ok := slice.Elt.(*ast.Ident)
			if !ok || element.Name != "byte" || slice.Len != nil || len(literal.Elts) != 1 {
				t.Errorf("changed store prefix %s: expected a one-byte []byte literal", name.Name)
				continue
			}
			value, ok := literal.Elts[0].(*ast.BasicLit)
			if !ok || value.Kind != token.INT {
				t.Errorf("changed store prefix %s: expected a literal prefix byte", name.Name)
				continue
			}
			parsed, err := strconv.ParseUint(value.Value, 0, 8)
			if err != nil {
				t.Errorf("changed store prefix %s: invalid prefix byte %s: %v", name.Name, value.Value, err)
				continue
			}
			prefix := byte(parsed)
			actual[name.Name] = prefix
			if previous, exists := owners[prefix]; exists {
				t.Errorf("store prefixes %s and %s share byte 0x%02x", previous, name.Name, prefix)
			}
			owners[prefix] = name.Name
			if unusedName, exists := reserved[prefix]; exists {
				t.Errorf("store prefix %s uses reserved byte 0x%02x (%s must remain unused)", name.Name, prefix, unusedName)
			}
		}
	}
	for _, entry := range frozen {
		if entry.unused {
			continue
		}
		value, exists := actual[entry.name]
		if !exists {
			t.Errorf("removed store prefix %s (expected 0x%02x)", entry.name, entry.value)
		} else if value != entry.value {
			t.Errorf("changed store prefix %s: got 0x%02x, want 0x%02x", entry.name, value, entry.value)
		}
	}
}
