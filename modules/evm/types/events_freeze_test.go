package types_test

import (
	"go/ast"
	"go/constant"
	"go/parser"
	"go/token"
	"go/types"
	"path/filepath"
	"runtime"
	"testing"
)

func TestEVMEventKeysFrozen(t *testing.T) {
	want := map[string]string{
		"EventTypeAddressAssociated": "address_associated",
		"EventTypePointerRegistered": "pointer_registered",
		"EventTypeSigner":            "signer",
		"EventTypeLayerXBound":       "layerx_bound",
		"EventTypeLayerXUnbound":     "layerx_unbound",
		"AttributeKeyPaxAddress":     "pax_addr",
		"AttributeKeyEvmAddress":     "evm_addr",
		"AttributeKeyPointerType":    "pointer_type",
		"AttributeKeyPointee":        "pointee",
		"AttributeKeyPointerAddress": "pointer_address",
		"AttributeKeyPointerVersion": "pointer_version",
		"AttributeKeyLayerXDid":      "layerx_did",
		"AttributeKeyLayerXNonce":    "layerx_bind_nonce",
	}

	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate events_freeze_test.go")
	}
	source := filepath.Join(filepath.Dir(testFile), "events.go")
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, source, nil, 0)
	if err != nil {
		t.Fatalf("parse events.go: %v", err)
	}
	config := types.Config{}
	pkg, err := config.Check("evm/events", fset, []*ast.File{file}, nil)
	if err != nil {
		t.Fatalf("evaluate events.go constants: %v", err)
	}

	got := make(map[string]constant.Value)
	for _, name := range pkg.Scope().Names() {
		if entry, ok := pkg.Scope().Lookup(name).(*types.Const); ok {
			got[name] = entry.Val()
			if _, exists := want[name]; !exists {
				t.Errorf("added entry %s = %s", name, entry.Val().ExactString())
			}
		}
	}
	for name, expected := range want {
		actual, exists := got[name]
		if !exists {
			t.Errorf("removed entry %s (want %q)", name, expected)
			continue
		}
		if actual.Kind() != constant.String {
			t.Errorf("changed entry %s: got %s (%s), want %q (String)", name, actual.ExactString(), actual.Kind(), expected)
			continue
		}
		if value := constant.StringVal(actual); value != expected {
			t.Errorf("changed entry %s: got %q, want %q", name, value, expected)
		}
	}
}
