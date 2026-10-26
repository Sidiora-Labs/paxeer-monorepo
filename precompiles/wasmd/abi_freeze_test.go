package wasmd_test

import (
	"bytes"
	_ "embed"
	"fmt"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/wasmd"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

//go:embed abi.json
var wasmdFrozenABIJSON []byte

func TestWasmdABIFrozen(t *testing.T) {
	wantMethods := map[string]string{
		"execute(string,bytes,bytes)":                   "0x44d227ae",
		"execute_batch((string,bytes,bytes)[])":         "0xbd49828b",
		"instantiate(uint64,string,bytes,string,bytes)": "0x12978014",
		"query(string,bytes)":                           "0x06d81d29",
	}
	wantEvents := map[string]string{}

	currentABI := wasmd.GetABI()
	gotMethods := make(map[string]string, len(currentABI.Methods))
	for _, method := range currentABI.Methods {
		selector := fmt.Sprintf("0x%x", method.ID)
		t.Logf("method %s: %s", method.Sig, selector)
		require.Len(t, method.ID, 4, "selector length for %s", method.Sig)
		gotMethods[method.Sig] = selector
	}
	gotEvents := make(map[string]string, len(currentABI.Events))
	for _, event := range currentABI.Events {
		topic := event.ID.Hex()
		t.Logf("event %s: %s", event.Sig, topic)
		gotEvents[event.Sig] = topic
	}
	require.Len(t, currentABI.Methods, 4, "wasmd method inventory changed: %v", gotMethods)
	require.Len(t, currentABI.Events, 0, "wasmd event inventory changed: %v", gotEvents)
	for signature, selector := range wantMethods {
		require.Equal(t, selector, gotMethods[signature], "selector changed for %s", signature)
	}
	require.Equal(t, wantMethods, gotMethods, "wasmd method signatures and selectors changed")
	require.Equal(t, wantEvents, gotEvents, "wasmd event signatures and topics changed")

	embeddedABI, err := abi.JSON(bytes.NewReader(wasmdFrozenABIJSON))
	require.NoError(t, err)
	require.Equal(t, embeddedABI, currentABI, "GetABI differs from embedded abi.json")

	testApp := app.Setup(t, false, false, false)
	precompile, err := wasmd.NewPrecompile(testApp.GetPrecompileKeepers())
	require.NoError(t, err)
	require.NotNil(t, precompile)
	require.Equal(t, common.HexToAddress("0x0000000000000000000000000000000000001002"), precompile.Address(), "wasmd precompile address changed")
	require.Equal(t, embeddedABI, precompile.ABI, "served ABI differs from embedded abi.json")
}
