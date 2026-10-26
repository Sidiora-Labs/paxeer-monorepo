package perps_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"testing"

	exchangetypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxexchange/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/perps"
)

func TestPerpsAddressIsLiteral1015(t *testing.T) {
	const want = "0x0000000000000000000000000000000000001015"
	if perps.PerpsAddress != want {
		t.Fatalf("PerpsAddress = %q, want %q", perps.PerpsAddress, want)
	}
	if perps.PerpsAddress != exchangetypes.ExchangeAddress {
		t.Fatalf("PerpsAddress = %q, legacy exchange address = %q", perps.PerpsAddress, exchangetypes.ExchangeAddress)
	}

	source, err := parser.ParseFile(token.NewFileSet(), "perps.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, decl := range source.Decls {
		constants, ok := decl.(*ast.GenDecl)
		if !ok || constants.Tok != token.CONST {
			continue
		}
		for _, spec := range constants.Specs {
			value := spec.(*ast.ValueSpec)
			for i, name := range value.Names {
				if name.Name != "PerpsAddress" {
					continue
				}
				if i >= len(value.Values) {
					t.Fatal("PerpsAddress must have an explicit string literal")
				}
				literal, ok := value.Values[i].(*ast.BasicLit)
				if !ok || literal.Kind != token.STRING {
					t.Fatal("PerpsAddress must be a string literal")
				}
				got, err := strconv.Unquote(literal.Value)
				if err != nil || got != want {
					t.Fatalf("PerpsAddress literal = %s, want %q (unquote error: %v)", literal.Value, want, err)
				}
				return
			}
		}
	}
	t.Fatal("PerpsAddress constant declaration not found")
}
