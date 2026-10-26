package solo_test

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/solo"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestSoloABIFrozen(t *testing.T) {
	expectedMethods := map[string]string{
		"claim(bytes)":         "0xc63ff8dd",
		"claimSpecific(bytes)": "0x8043ef1f",
	}
	expectedEvents := map[string]string{}

	p, err := solo.NewPrecompile(testkeeper.EVMTestApp.GetPrecompileKeepers())
	require.NoError(t, err)
	require.NotNil(t, p)
	require.Equal(t, common.HexToAddress("0x000000000000000000000000000000000000100C"), p.Address())

	var servedABI abi.ABI = p.GetABI()
	embeddedJSON, err := solo.F.ReadFile("abi.json")
	require.NoError(t, err)
	embeddedABI, err := abi.JSON(bytes.NewReader(embeddedJSON))
	require.NoError(t, err)
	require.Equal(t, embeddedABI, servedABI, "served ABI differs from embedded solo/abi.json")

	methods := make(map[string]string, len(servedABI.Methods))
	for _, method := range servedABI.Methods {
		selector := "0x" + hex.EncodeToString(method.ID)
		t.Logf("method %s selector %s", method.Sig, selector)
		require.Len(t, method.ID, 4, "selector length for %s", method.Sig)
		methods[method.Sig] = selector
		want, ok := expectedMethods[method.Sig]
		require.True(t, ok, "unexpected method %s with selector %s", method.Sig, selector)
		require.Equal(t, want, selector, "selector changed for %s", method.Sig)
	}
	require.Len(t, servedABI.Methods, 2, "solo method count changed")
	require.Equal(t, expectedMethods, methods, "solo method signatures or selectors changed")

	events := make(map[string]string, len(servedABI.Events))
	for _, event := range servedABI.Events {
		topic := event.ID.Hex()
		t.Logf("event %s topic %s", event.Sig, topic)
		events[event.Sig] = topic
	}
	require.Len(t, servedABI.Events, 0, "solo event count changed")
	require.Equal(t, expectedEvents, events, "solo event signatures or topics changed")
}
