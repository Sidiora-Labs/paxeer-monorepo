package layerxledger

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/stretchr/testify/require"
)

func TestPublishedLedgerAndAuthABIServeProbedViews(t *testing.T) {
	for _, published := range []struct {
		name string
		path string
	}{
		{"ledger", "abi.json"},
		{"auth", "../layerxauth/abi.json"},
	} {
		t.Run(published.name, func(t *testing.T) {
			data, err := os.ReadFile(published.path)
			require.NoError(t, err, "read published %s ABI", published.name)
			parsed, err := abi.JSON(bytes.NewReader(data))
			require.NoError(t, err, "parse published %s ABI", published.name)
			if published.name == "auth" {
				method, ok := parsed.Methods["recordLifecycle"]
				require.True(t, ok)
				require.Equal(t, "recordLifecycle(bytes,bytes32)", method.Sig)
				require.Equal(t, "nonpayable", method.StateMutability)
				require.Equal(t, "(uint64)", ledgerArgumentTypes(method.Outputs))
			}
			require.NotEmpty(t, parsed.Methods)
			if published.name == "ledger" {
				for _, view := range []struct {
					name    string
					outputs string
				}{
					{"getAsset", "(bytes32,string,string,uint8,uint128,bytes32,bytes32,uint8,uint8,bytes,bool,uint128)"},
					{"totalUnits", "(uint128)"},
				} {
					t.Run(view.name, func(t *testing.T) {
						method, ok := parsed.Methods[view.name]
						require.True(t, ok, "published ledger ABI must declare %s", view.name)
						require.Equal(t, abi.Function, method.Type)
						require.Equal(t, view.name+"(bytes32)", method.Sig)
						require.Equal(t, "view", method.StateMutability)
						require.NotEmpty(t, method.Outputs)
						require.Equal(t, view.outputs, ledgerArgumentTypes(method.Outputs))
					})
				}
			}
		})
	}
}

func TestPublishedLedgerAndAuthABIFrozen(t *testing.T) {
	for _, frozen := range []struct {
		name   string
		path   string
		sha256 string
	}{
		{"ledger", "abi.json", "2c438c81cce9661e7b8a664667b480b70a8a9a9e8952ff347bd31d7cffd8f047"},
		{"auth", "../layerxauth/abi.json", "b05e1f3aa1d1c2647041c2fc2ab841afe42ee6cbb1b9fce8881480a49d6f8fc9"},
	} {
		t.Run(frozen.name, func(t *testing.T) {
			data, err := os.ReadFile(frozen.path)
			require.NoError(t, err, "read published %s ABI", frozen.name)
			digest := sha256.Sum256(data)
			require.Equal(t, frozen.sha256, hex.EncodeToString(digest[:]),
				"published %s ABI must remain byte-identical to the frozen digest", frozen.name)
		})
	}
}
