package pointer_test

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/pointer"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestPointerABIFrozen(t *testing.T) {
	precompile, err := pointer.NewPrecompile(testkeeper.EVMTestApp.GetPrecompileKeepers())
	require.NoError(t, err)
	require.Equal(t, common.HexToAddress("0x000000000000000000000000000000000000100b"), precompile.Address())

	abiJSON, err := os.ReadFile("abi.json")
	require.NoError(t, err)
	embeddedABI, err := abi.JSON(bytes.NewReader(abiJSON))
	require.NoError(t, err)
	servedABI := precompile.GetABI()
	require.Equal(t, embeddedABI, servedABI, "served ABI must equal the embedded abi.json")

	solidity, err := os.ReadFile("Pointer.sol")
	require.NoError(t, err)
	declarations := regexp.MustCompile(`(?s)function\s+(\w+)\s*\([^)]*\)\s*external\s*([^;]*);`).FindAllStringSubmatch(string(solidity), -1)
	require.Len(t, declarations, 4, "Solidity must declare all four registration methods")
	solidityMutability := make(map[string]string)
	for _, declaration := range declarations {
		mutability := "nonpayable"
		for _, token := range strings.Fields(declaration[2]) {
			if token == "payable" || token == "view" || token == "pure" {
				mutability = token
			}
		}
		solidityMutability[declaration[1]] = mutability
	}

	wantMethods := map[string]string{
		"addCW1155Pointer(string)": "0xa3b62916",
		"addCW20Pointer(string)":   "0xce875775",
		"addCW721Pointer(string)":  "0x9e5720f0",
		"addNativePointer(string)": "0xc31d960f",
	}
	wantEvents := map[string]string{}
	gotMethods := make(map[string]string, len(servedABI.Methods))
	for _, method := range servedABI.Methods {
		require.Equal(t, "nonpayable", method.StateMutability, "registration accepts zero value only")
		require.False(t, method.IsPayable())
		require.Equal(t, method.StateMutability, solidityMutability[method.Name], "ABI/Solidity mutability for %s", method.Name)
		selector := fmt.Sprintf("0x%x", method.ID)
		t.Logf("method %s selector %s", method.Sig, selector)
		require.Len(t, method.ID, 4, "selector for %s", method.Sig)
		gotMethods[method.Sig] = selector
		t.Run(method.Sig, func(t *testing.T) {
			want, exists := wantMethods[method.Sig]
			require.True(t, exists, "unexpected method %s", method.Sig)
			require.Equal(t, want, selector, "selector changed for %s", method.Sig)
		})
	}
	gotEvents := make(map[string]string, len(servedABI.Events))
	for _, event := range servedABI.Events {
		topic := event.ID.Hex()
		t.Logf("event %s topic %s", event.Sig, topic)
		gotEvents[event.Sig] = topic
	}
	require.Len(t, servedABI.Methods, 4, "pointer must serve exactly four methods")
	require.Equal(t, wantMethods, gotMethods, "pointer method signatures and selectors changed")
	require.Len(t, servedABI.Events, 0, "pointer must serve no event topics")
	require.Equal(t, wantEvents, gotEvents, "pointer event signatures and topics changed")
}
