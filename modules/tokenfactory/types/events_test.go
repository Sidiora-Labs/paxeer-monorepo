package types

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestEventAttributeNamesFrozen(t *testing.T) {
	attributes := map[string]struct {
		got  string
		want string
	}{
		"AttributeAmount":            {AttributeAmount, "amount"},
		"AttributeCreator":           {AttributeCreator, "creator"},
		"AttributeSubdenom":          {AttributeSubdenom, "subdenom"},
		"AttributeNewTokenDenom":     {AttributeNewTokenDenom, "new_token_denom"},
		"AttributeUpdatedTokenDenom": {AttributeUpdatedTokenDenom, "updated_token_denom"},
		"AttributeMintToAddress":     {AttributeMintToAddress, "mint_to_address"},
		"AttributeBurnFromAddress":   {AttributeBurnFromAddress, "burn_from_address"},
		"AttributePaused":            {AttributePaused, "paused"},
		"AttributeDenom":             {AttributeDenom, "denom"},
		"AttributeNewAdmin":          {AttributeNewAdmin, "new_admin"},
		"AttributeDenomMetadata":     {AttributeDenomMetadata, "denom_metadata"},
		"AttributeAllowList":         {AttributeAllowList, "denom_allow_list"},
	}
	for name, attribute := range attributes {
		if attribute.got != attribute.want {
			t.Errorf("%s = %q, want %q", name, attribute.got, attribute.want)
		}
	}

	file, err := parser.ParseFile(token.NewFileSet(), "events.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, decl := range file.Decls {
		constants, ok := decl.(*ast.GenDecl)
		if !ok || constants.Tok != token.CONST {
			continue
		}
		for _, spec := range constants.Specs {
			for _, name := range spec.(*ast.ValueSpec).Names {
				count++
				if _, ok := attributes[name.Name]; !ok {
					t.Errorf("unexpected event attribute constant %s", name.Name)
				}
			}
		}
	}
	if count != len(attributes) {
		t.Errorf("event attribute count = %d, want %d", count, len(attributes))
	}
}
