package addr_test

import (
	_ "embed"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/addr"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

//go:embed abi.json
var addrFrozenABIJSON string

func TestAddrABIFrozen(t *testing.T) {
	precompile, err := addr.NewPrecompile(testkeeper.EVMTestApp.GetPrecompileKeepers())
	require.NoError(t, err)
	require.Equal(t, common.HexToAddress("0x0000000000000000000000000000000000001004"), precompile.Address())

	servedABI := precompile.GetABI()
	wantMethods := map[string]string{
		"associate(string,string,string,string)": "0x8941d0e3",
		"associatePubKey(string)":                "0x1494082b",
		"getPaxAddr(address)":                    "0x7e34d1dd",
		"getEvmAddr(string)":                     "0x1778e539",
		"bindLayerX(bytes32,bytes)":              "0xdd9aa628",
		"unbindLayerX()":                         "0x6b3407d9",
		"getLayerXDid(address)":                  "0xd3600b24",
		"getEvmAddrByLayerX(bytes32)":            "0xc413bad3",
		"layerXBindNonce(address)":               "0xcedd9ba2",
		"getUnifiedAccount(address)":             "0x357feed6",
	}
	methodSelectors := make(map[string]string, len(servedABI.Methods))
	for _, method := range servedABI.Methods {
		require.Len(t, method.ID, 4, "selector width for %s", method.Sig)
		selector := "0x" + hex.EncodeToString(method.ID)
		t.Logf("method %s: %s", method.Sig, selector)
		methodSelectors[method.Sig] = selector
		want, ok := wantMethods[method.Sig]
		require.True(t, ok, "unexpected method %s with selector %s", method.Sig, selector)
		require.Equal(t, want, selector, "selector changed for %s", method.Sig)
	}
	for signature, selector := range wantMethods {
		require.Equal(t, selector, methodSelectors[signature], "missing or changed method %s", signature)
	}
	require.Len(t, servedABI.Methods, 10)

	wantEvents := map[string]string{
		"LayerXBound(address,bytes32,uint64)":   "0x79b5f3e386759c93c56bf67a868c5f49afe6afaf286aa99bbc8e800ca4b7c856",
		"LayerXUnbound(address,bytes32,uint64)": "0xce3f8d62ef8d76530f28a650078b3a45e48fbc97596eeb704b1bb00d3946847d",
	}
	eventTopics := make(map[string]string, len(servedABI.Events))
	for _, event := range servedABI.Events {
		topic := event.ID.Hex()
		t.Logf("event %s: %s", event.Sig, topic)
		eventTopics[event.Sig] = topic
		want, ok := wantEvents[event.Sig]
		require.True(t, ok, "unexpected event %s with topic %s", event.Sig, topic)
		require.Equal(t, want, topic, "topic changed for %s", event.Sig)
		require.False(t, event.Anonymous, "event %s must emit its signature topic", event.Sig)
	}
	for signature, topic := range wantEvents {
		require.Equal(t, topic, eventTopics[signature], "missing or changed event %s", signature)
	}
	require.Len(t, servedABI.Events, 2)

	embeddedABI, err := abi.JSON(strings.NewReader(addrFrozenABIJSON))
	require.NoError(t, err)
	require.Equal(t, embeddedABI, servedABI, "served ABI must equal embedded abi.json")
}
