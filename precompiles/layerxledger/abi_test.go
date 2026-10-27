package layerxledger

import (
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func ledgerArgumentTypes(arguments abi.Arguments) string {
	types := make([]string, len(arguments))
	for i, argument := range arguments {
		types[i] = argument.Type.String()
	}
	return "(" + strings.Join(types, ",") + ")"
}

func ledgerABIEntryCount(t *testing.T, kind string) int {
	t.Helper()
	data, err := f.ReadFile("abi.json")
	require.NoError(t, err)
	var entries []struct {
		Type string `json:"type"`
		Name string `json:"name"`
	}
	require.NoError(t, json.Unmarshal(data, &entries))
	seen := make(map[string]bool)
	count := 0
	for _, entry := range entries {
		if entry.Type == kind {
			require.False(t, seen[entry.Name], "duplicate %s %s", kind, entry.Name)
			seen[entry.Name] = true
			count++
		}
	}
	return count
}

func TestLedgerABIMethods(t *testing.T) {
	parsed := GetABI()
	methods := []struct {
		name, signature, selector, mutability, outputs string
	}{
		{"register", "register(bytes32,bytes32,string,string,uint8,uint128,uint8,bytes)", "fd03012a", "nonpayable", "()"},
		{"pause", "pause(bytes32)", "ed56531a", "nonpayable", "()"},
		{"unpause", "unpause(bytes32)", "2f4dae9f", "nonpayable", "()"},
		{"openAccount", "openAccount(bytes32)", "bdf52150", "nonpayable", "()"},
		{"send", "send(bytes)", "2bb1ae7c", "nonpayable", "()"},
		{"draw", "draw(bytes32,bytes32,uint128,bytes32,uint64,bytes32,bytes32)", "2e89490f", "nonpayable", "()"},
		{"payExact", "payExact(bytes32,bytes32,bytes32,uint128,uint64,bytes32)", "78ea11db", "nonpayable", "()"},
		{"issueGrant", "issueGrant(bytes)", "d0d53480", "nonpayable", "()"},
		{"revokeGrant", "revokeGrant(bytes32,uint64)", "58d183a8", "nonpayable", "()"},
		{"mint", "mint(bytes32,bytes32,uint128)", "f04db938", "nonpayable", "()"},
		{"burn", "burn(bytes32,bytes32,uint128)", "da3c63f3", "nonpayable", "()"},
		{"getAsset", "getAsset(bytes32)", "2cc3ce80", "view", "(bytes32,string,string,uint8,uint128,bytes32,bytes32,uint8,uint8,bytes,bool,uint128)"},
		{"balanceOf", "balanceOf(bytes32,bytes32)", "b1ffc03e", "view", "(uint128)"},
		{"totalUnits", "totalUnits(bytes32)", "53a6332b", "view", "(uint128)"},
		{"accountId", "accountId(bytes32,bytes32)", "3f050cbc", "view", "(bytes32)"},
		{"accountOwner", "accountOwner(bytes32)", "37955ab2", "view", "(bytes32)"},
		{"getPayerGrant", "getPayerGrant(bytes32)", "423fe4b2", "view", "(bytes,uint128,uint128,uint64,uint64,bool,bool)"},
		{"nextSequence", "nextSequence(bytes32,bytes32)", "ce2f921f", "view", "(uint64)"},
	}
	require.Equal(t, 18, ledgerABIEntryCount(t, "function"))
	require.Len(t, parsed.Methods, len(methods))
	for _, want := range methods {
		t.Run(want.name, func(t *testing.T) {
			got, ok := parsed.Methods[want.name]
			require.True(t, ok)
			require.Equal(t, want.name, got.RawName)
			require.Equal(t, abi.Function, got.Type)
			require.Equal(t, want.signature, got.Sig)
			require.Equal(t, want.selector, hex.EncodeToString(got.ID))
			require.Equal(t, want.mutability, got.StateMutability)
			require.False(t, got.IsPayable())
			require.Equal(t, want.outputs, ledgerArgumentTypes(got.Outputs))
			resolved, err := parsed.MethodById(got.ID)
			require.NoError(t, err)
			require.Equal(t, want.name, resolved.Name)
		})
	}
}

func TestLedgerABIEvents(t *testing.T) {
	parsed := GetABI()
	events := []struct {
		name, signature, topic string
		indexed                []bool
	}{
		{"AssetRegistered", "AssetRegistered(bytes32)", "e44622c42196b40148eee03152bf2d34e7e3180112c6a2ca51e16f864b65bd13", []bool{true}},
		{"AssetPaused", "AssetPaused(bytes32)", "e2844e7e535e938024242536fcf78585913acf68e8d61fafa5d36e63a43f4c47", []bool{true}},
		{"AssetUnpaused", "AssetUnpaused(bytes32)", "32f3ed1bf47daea95443414fbdcb1f5df4651a2ea7bb40e4bc41ae16c6f1843d", []bool{true}},
		{"AccountOpened", "AccountOpened(bytes32)", "307d00a86fd0f97ae2d0368268bd5f047f7e8c0d13d1570e72dadd8f9e2fe22f", []bool{true}},
		{"Transfer", "Transfer(bytes32,bytes32,bytes32,uint128,uint8)", "3c136bc6b05877456edd6c1baed12a9d44c364ad7301478c948e7d70dd4b1ca0", []bool{true, true, true, false, false}},
		{"PayerGrantIssued", "PayerGrantIssued(bytes32)", "facf09c43b359fef8d71b96c6e4c823b1f09c539e257224245c24345fa24f66e", []bool{true}},
		{"PayerGrantRevoked", "PayerGrantRevoked(bytes32)", "841096fe471155c0948686c4e7fa3210edcc36f84f8b61f1b508b923fa6a226d", []bool{true}},
		{"PaymentDrawn", "PaymentDrawn(bytes32,bytes32,bytes32,bytes32,uint128,bytes32,bytes32)", "13473d103a2d52172016c872eeae0153e254af3c21a1d987492ab737f1a34fe1", []bool{true, true, true, false, false, false, false}},
		{"PaymentExact", "PaymentExact(bytes32,bytes32,bytes32,bytes32,uint128)", "c4da5d6f757c251cd70947b752917e5671b677517e3d953f362a1df6648ef5b1", []bool{true, true, true, false, false}},
	}
	require.Equal(t, 9, ledgerABIEntryCount(t, "event"))
	require.Len(t, parsed.Events, len(events))
	for _, want := range events {
		t.Run(want.name, func(t *testing.T) {
			got, ok := parsed.Events[want.name]
			require.True(t, ok)
			require.Equal(t, want.name, got.RawName)
			require.Equal(t, want.signature, got.Sig)
			require.Equal(t, common.HexToHash(want.topic), got.ID)
			require.False(t, got.Anonymous)
			indexed := make([]bool, len(got.Inputs))
			for i, input := range got.Inputs {
				indexed[i] = input.Indexed
			}
			require.Equal(t, want.indexed, indexed)
			resolved, err := parsed.EventByID(got.ID)
			require.NoError(t, err)
			require.Equal(t, want.name, resolved.Name)
		})
	}
}
