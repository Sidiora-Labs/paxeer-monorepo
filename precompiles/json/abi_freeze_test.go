package json_test

import (
	"bytes"
	"encoding/hex"
	"os"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/json"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestJsonABIFrozen(t *testing.T) {
	wantMethods := map[string]string{
		"extractAsBytes(bytes,string)":          "08d858e5",
		"extractAsBytesList(bytes,string)":      "87cdf621",
		"extractAsUint256(bytes,string)":        "5a558982",
		"extractAsBytesFromArray(bytes,uint16)": "b0bf8a47",
	}
	wantEvents := map[string]string{}

	p, err := json.NewPrecompile(nil)
	require.NoError(t, err)
	require.Equal(t, common.HexToAddress("0x1003"), p.Address())
	servedABI := p.GetABI()

	gotMethods := make(map[string]string, len(servedABI.Methods))
	for _, method := range servedABI.Methods {
		require.Len(t, method.ID, 4, "selector width for %s", method.Sig)
		selector := hex.EncodeToString(method.ID)
		t.Logf("method %s: 0x%s", method.Sig, selector)
		gotMethods[method.Sig] = selector
	}
	gotEvents := make(map[string]string, len(servedABI.Events))
	for _, event := range servedABI.Events {
		topic := event.ID.Hex()
		t.Logf("event %s: %s", event.Sig, topic)
		gotEvents[event.Sig] = topic
	}

	t.Run("methods", func(t *testing.T) {
		require.Equal(t, wantMethods, gotMethods, "json method signatures and selectors must remain frozen")
		require.Len(t, servedABI.Methods, 4)
	})
	t.Run("events", func(t *testing.T) {
		require.Equal(t, wantEvents, gotEvents, "json event signatures and topics must remain frozen")
		require.Empty(t, servedABI.Events)
	})
	t.Run("embedded ABI", func(t *testing.T) {
		abiJSON, err := os.ReadFile("abi.json")
		require.NoError(t, err)
		embeddedABI, err := abi.JSON(bytes.NewReader(abiJSON))
		require.NoError(t, err)
		require.Equal(t, embeddedABI, servedABI, "served ABI must equal abi.json embedded by the json package")
	})
}
