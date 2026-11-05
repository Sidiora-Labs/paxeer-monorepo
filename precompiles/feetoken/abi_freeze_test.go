package feetoken_test

import (
	"bytes"
	_ "embed"
	"fmt"
	"sort"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/feetoken"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

//go:embed abi.json
var feeTokenABIJSON []byte

func TestFeeTokenABIFrozen(t *testing.T) {
	precompile, err := feetoken.NewPrecompile(testkeeper.EVMTestApp.GetPrecompileKeepers())
	require.NoError(t, err)
	require.Equal(t, common.HexToAddress("0x0000000000000000000000000000000000001018"), precompile.Address())

	var servedABI abi.ABI = precompile.GetABI()
	embeddedABI, err := abi.JSON(bytes.NewReader(feeTokenABIJSON))
	require.NoError(t, err)
	require.Equal(t, embeddedABI, servedABI, "served ABI must equal embedded abi.json")

	wantMethods := map[string]string{
		"clearFeeDenom()":      "0xc541f958",
		"getFeeDenom(address)": "0x345ed2a4",
		"setFeeDenom(string)":  "0x16dd2643",
	}
	wantEvents := map[string]string{}
	gotMethods := make(map[string]string, len(servedABI.Methods))
	gotEvents := make(map[string]string, len(servedABI.Events))
	var inventory []string
	for _, method := range servedABI.Methods {
		require.Len(t, method.ID, 4, "selector for %s", method.Sig)
		selector := fmt.Sprintf("0x%x", method.ID)
		gotMethods[method.Sig] = selector
		inventory = append(inventory, fmt.Sprintf("method %s %s", method.Sig, selector))
		want, ok := wantMethods[method.Sig]
		if !ok {
			t.Errorf("unexpected method %s with selector %s", method.Sig, selector)
		} else if selector != want {
			t.Errorf("selector for %s moved: got %s, want %s", method.Sig, selector, want)
		}
	}
	for _, event := range servedABI.Events {
		topic := event.ID.Hex()
		gotEvents[event.Sig] = topic
		inventory = append(inventory, fmt.Sprintf("event %s %s", event.Sig, topic))
	}
	sort.Strings(inventory)
	for _, entry := range inventory {
		t.Log(entry)
	}
	t.Logf("ABI inventory: %d methods, %d events", len(servedABI.Methods), len(servedABI.Events))
	require.Len(t, servedABI.Methods, 3, "frozen method count")
	require.Equal(t, wantMethods, gotMethods, "frozen method signatures and selectors")
	require.Len(t, servedABI.Events, 0, "frozen event count")
	require.Equal(t, wantEvents, gotEvents, "frozen event signatures and topics")
}
