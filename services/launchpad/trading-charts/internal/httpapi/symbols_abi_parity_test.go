package httpapi

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

func TestERC20MetadataSelectorsMatchTokenABI(t *testing.T) {
	abiPath := filepath.Join("..", "..", "..", "..", "..", "precompiles", "launchpad", "abi", "SidioraERC20.json")
	raw, err := os.ReadFile(abiPath)
	if err != nil {
		t.Fatalf("read launchpad token ABI: %v", err)
	}
	tokenABI, err := abi.JSON(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("parse launchpad token ABI: %v", err)
	}
	methods := []string{"name", "symbol", "decimals", "totalSupply"}
	if len(erc20MetadataSelectors) != len(methods) {
		t.Fatalf("metadata selector count = %d, want %d", len(erc20MetadataSelectors), len(methods))
	}
	for i, name := range methods {
		method, ok := tokenABI.Methods[name]
		if !ok {
			t.Fatalf("launchpad token ABI is missing method %s", name)
		}
		want := hexutil.Encode(method.ID)
		if got := erc20MetadataSelectors[i]; got != want {
			t.Errorf("metadata selector[%d] (%s) = %s, want %s from launchpad token ABI", i, name, got, want)
		}
	}
}
