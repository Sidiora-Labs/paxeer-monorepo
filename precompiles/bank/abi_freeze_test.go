package bank_test

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/bank"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBankABIFrozen(t *testing.T) {
	wantMethods := map[string]string{
		"all_balances(address)":                        "0x0c5e2789",
		"balance(address,string)":                      "0x16cadeab",
		"decimals(string)":                             "0x3b2b3204",
		"name(string)":                                 "0x5b43bc99",
		"send(address,address,string,uint256)":         "0x5c05961b",
		"send(address,address,string,uint256,bytes32)": "0x2e4c85ac",
		"sendNative(string)":                           "0x6ff45dad",
		"supply(string)":                               "0x6923a1fd",
		"symbol(string)":                               "0x41bb0559",
	}
	wantEvents := map[string]string{
		"LayerXReceipt(address,address,string,uint256,bytes32)": "0x4b11729a809ec729868d9ba54f3192aa4179fe63668e9f742aa9b25de21de65f",
	}

	bankABI := bank.GetABI()
	assert.Len(t, bankABI.Methods, 9, "bank 0x1001 method count changed")
	gotMethods := make(map[string]string, len(bankABI.Methods))
	for _, method := range bankABI.Methods {
		assert.Len(t, method.ID, 4, "selector width for %s", method.Sig)
		selector := "0x" + hex.EncodeToString(method.ID)
		gotMethods[method.Sig] = selector
		t.Logf("method %s: %s", method.Sig, selector)
		assert.Equal(t, wantMethods[method.Sig], selector, "selector for %s changed", method.Sig)
	}
	assert.Equal(t, wantMethods, gotMethods, "bank 0x1001 method signatures and selectors changed")

	assert.Len(t, bankABI.Events, 1, "bank 0x1001 event count changed")
	gotEvents := make(map[string]string, len(bankABI.Events))
	for _, event := range bankABI.Events {
		topic := event.ID.Hex()
		gotEvents[event.Sig] = topic
		t.Logf("event %s: %s", event.Sig, topic)
		assert.Equal(t, wantEvents[event.Sig], topic, "topic for %s changed", event.Sig)
		assert.False(t, event.Anonymous, "event %s must emit its signature topic", event.Sig)
	}
	assert.Equal(t, wantEvents, gotEvents, "bank 0x1001 event signatures and topics changed")

	embeddedJSON, err := f.ReadFile("abi.json")
	require.NoError(t, err)
	embeddedABI, err := abi.JSON(bytes.NewReader(embeddedJSON))
	require.NoError(t, err)
	assert.Equal(t, embeddedABI, bankABI, "GetABI must match embedded abi.json")

	precompile, err := bank.NewPrecompile(testkeeper.EVMTestApp.GetPrecompileKeepers())
	require.NoError(t, err)
	require.NotNil(t, precompile)
	assert.Equal(t, common.HexToAddress("0x0000000000000000000000000000000000001001"), precompile.Address(), "bank precompile address changed")
	assert.Equal(t, embeddedABI, precompile.ABI, "served bank ABI must match embedded abi.json")
}
