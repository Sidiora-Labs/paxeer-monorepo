package pointerview_test

import (
	_ "embed"
	"fmt"
	"strings"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/pointerview"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

//go:embed abi.json
var pointerViewABIJSON string

func TestPointerViewABIFrozen(t *testing.T) {
	wantMethods := map[string]string{
		"getCW1155Pointer(string)": "0x3555cb52",
		"getCW20Pointer(string)":   "0xdaed51b5",
		"getCW721Pointer(string)":  "0xf0299ce6",
		"getNativePointer(string)": "0x331df31c",
	}
	wantEvents := map[string]string{}

	p, err := pointerview.NewPrecompile(testkeeper.EVMTestApp.GetPrecompileKeepers())
	require.NoError(t, err)
	require.NotNil(t, p)
	require.Equal(t, common.HexToAddress("0x100A"), p.Address())

	var served abi.ABI = p.GetABI()
	embedded, err := abi.JSON(strings.NewReader(pointerViewABIJSON))
	require.NoError(t, err)
	require.Equal(t, embedded, served, "served ABI must equal embedded abi.json")

	methods := make(map[string]string, len(served.Methods))
	for _, method := range served.Methods {
		selector := fmt.Sprintf("0x%x", method.ID)
		t.Logf("method %s: %s", method.Sig, selector)
		require.Len(t, method.ID, 4, "selector width for %s", method.Sig)
		methods[method.Sig] = selector
		if _, ok := wantMethods[method.Sig]; !ok {
			t.Errorf("unexpected method %s: %s", method.Sig, selector)
		}
	}
	require.Len(t, served.Methods, 4, "pointerview method count changed")
	for signature, selector := range wantMethods {
		t.Run(signature, func(t *testing.T) {
			actual, ok := methods[signature]
			require.True(t, ok, "missing frozen method %s", signature)
			require.Equal(t, selector, actual, "selector changed for %s", signature)
		})
	}

	events := make(map[string]string, len(served.Events))
	for _, event := range served.Events {
		topic := event.ID.Hex()
		t.Logf("event %s: %s", event.Sig, topic)
		events[event.Sig] = topic
	}
	require.Len(t, served.Events, 0, "pointerview event count changed: %v", events)
	require.Equal(t, wantEvents, events, "pointerview event topics changed")
}
