package types_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
	"testing"
)

func TestEVMParamKeysFrozen(t *testing.T) {
	want := map[string]string{
		"KeyFeeTokenDistribution":                   "KeyFeeTokenDistribution",
		"KeyMaxFeeTokenRateAge":                     "KeyMaxFeeTokenRateAge",
		"KeyAllowedFeeDenoms":                       "KeyAllowedFeeDenoms",
		"KeyMaxFeeTokenSpread":                      "KeyMaxFeeTokenSpread",
		"KeyFeeTokenEnabled":                        "KeyFeeTokenEnabled",
		"KeyPriorityNormalizer":                     "KeyPriorityNormalizer",
		"KeyMinFeePerGas":                           "KeyMinFeePerGas",
		"KeyMaxFeePerGas":                           "KeyMaximumFeePerGas",
		"KeyDeliverTxHookWasmGasLimit":              "KeyDeliverTxHookWasmGasLimit",
		"KeyMaxDynamicBaseFeeUpwardAdjustment":      "KeyMaxDynamicBaseFeeUpwardAdjustment",
		"KeyMaxDynamicBaseFeeDownwardAdjustment":    "KeyMaxDynamicBaseFeeDownwardAdjustment",
		"KeyTargetGasUsedPerBlock":                  "KeyTargetGasUsedPerBlock",
		"KeyPaxSstoreSetGasEIP2200":                 "KeyPaxSstoreSetGasEIP2200",
		"KeyBaseFeePerGas":                          "KeyBaseFeePerGas",
		"KeyWhitelistedCwCodeHashesForDelegateCall": "KeyWhitelistedCwCodeHashesForDelegateCall",
		"KeyRegisterPointerDisabled":                "KeyRegisterPointerDisabled",
	}

	source, err := parser.ParseFile(token.NewFileSet(), "params.go", nil, 0)
	if err != nil {
		t.Fatalf("read parameter key declarations from params.go: %v", err)
	}
	got := make(map[string]ast.Expr)
	for _, declaration := range source.Decls {
		group, ok := declaration.(*ast.GenDecl)
		if !ok || (group.Tok != token.VAR && group.Tok != token.CONST) {
			continue
		}
		for _, specification := range group.Specs {
			value := specification.(*ast.ValueSpec)
			for i, name := range value.Names {
				if !strings.HasPrefix(name.Name, "Key") {
					continue
				}
				got[name.Name] = nil
				if i < len(value.Values) {
					got[name.Name] = value.Values[i]
				}
			}
		}
	}

	for name, expression := range got {
		expected, ok := want[name]
		if !ok {
			t.Errorf("added parameter key %s", name)
			continue
		}
		conversion, ok := expression.(*ast.CallExpr)
		if !ok || len(conversion.Args) != 1 || conversion.Ellipsis.IsValid() {
			t.Errorf("changed parameter key %s: want []byte(%q), got %T", name, expected, expression)
			continue
		}
		slice, ok := conversion.Fun.(*ast.ArrayType)
		if !ok || slice.Len != nil {
			t.Errorf("changed parameter key %s: want []byte(%q)", name, expected)
			continue
		}
		element, ok := slice.Elt.(*ast.Ident)
		if !ok || element.Name != "byte" {
			t.Errorf("changed parameter key %s: want []byte(%q)", name, expected)
			continue
		}
		literal, ok := conversion.Args[0].(*ast.BasicLit)
		if !ok || literal.Kind != token.STRING {
			t.Errorf("changed parameter key %s: want []byte(%q)", name, expected)
			continue
		}
		actual, err := strconv.Unquote(literal.Value)
		if err != nil {
			t.Errorf("changed parameter key %s: invalid byte string: %v", name, err)
			continue
		}
		if actual != expected {
			t.Errorf("changed parameter key %s: got %q, want %q", name, actual, expected)
		}
	}
	for name := range want {
		if _, ok := got[name]; !ok {
			t.Errorf("removed parameter key %s", name)
		}
	}
}
