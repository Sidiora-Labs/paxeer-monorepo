package distribution_test

import (
	"encoding/hex"
	"strings"
	"testing"

	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/distribution"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestDistributionABIFrozen(t *testing.T) {
	testApp := testkeeper.EVMTestApp
	precompile, err := distribution.NewPrecompile(&app.PrecompileKeepers{
		DistributionKeeper: testApp.DistrKeeper,
		EVMKeeper:          &testApp.EvmKeeper,
	})
	require.NoError(t, err)
	require.Equal(t, common.HexToAddress("0x0000000000000000000000000000000000001007"), precompile.Address())

	embeddedJSON, err := f.ReadFile("abi.json")
	require.NoError(t, err)
	embeddedABI, err := abi.JSON(strings.NewReader(string(embeddedJSON)))
	require.NoError(t, err)
	servedABI := precompile.GetABI()
	require.Equal(t, embeddedABI, servedABI, "served ABI must equal embedded abi.json")

	methods := map[string]string{
		"rewards(address)":                            "0x0700037d",
		"setWithdrawAddress(address)":                 "0x3ab1a494",
		"withdrawDelegationRewards(string)":           "0x0442b1ca",
		"withdrawMultipleDelegationRewards(string[])": "0xe35b426f",
		"withdrawValidatorCommission()":               "0x0bde076d",
	}
	if len(servedABI.Methods) != len(methods) {
		t.Errorf("method count = %d, want %d", len(servedABI.Methods), len(methods))
	}
	seenMethods := make(map[string]bool, len(servedABI.Methods))
	for _, method := range servedABI.Methods {
		selector := "0x" + hex.EncodeToString(method.ID)
		t.Logf("method %s: %s", method.Sig, selector)
		seenMethods[method.Sig] = true
		want, ok := methods[method.Sig]
		if !ok {
			t.Errorf("unexpected method %s: %s", method.Sig, selector)
			continue
		}
		if len(method.ID) != 4 || selector != want {
			t.Errorf("method %s selector = %s (%d bytes), want %s (4 bytes)", method.Sig, selector, len(method.ID), want)
		}
	}
	for signature := range methods {
		if !seenMethods[signature] {
			t.Errorf("missing frozen method %s", signature)
		}
	}

	events := map[string]string{
		"DelegationRewardsWithdrawn(address,string,uint256)":             "0xe9a2d1cd042608da4ebdf1fafb4a4658dfc6954ce121138e784b3f9372505d11",
		"MultipleDelegationRewardsWithdrawn(address,string[],uint256[])": "0xded60ee1fd7214fb7c36e0d0a674961f0b3a146cee8b2a5dfac9e524da628439",
		"ValidatorCommissionWithdrawn(string,uint256)":                   "0xe40546e817dc4728b4185567545ed4559d2f8df7d691e4e9275c4668144701f7",
		"WithdrawAddressSet(address,address)":                            "0x81ef977572555fac5ac57a0e17fdf5feaf98f544e9af11713615da4e1f34ff0b",
	}
	if len(servedABI.Events) != len(events) {
		t.Errorf("event count = %d, want %d", len(servedABI.Events), len(events))
	}
	seenEvents := make(map[string]bool, len(servedABI.Events))
	for _, event := range servedABI.Events {
		topic := event.ID.Hex()
		t.Logf("event %s: %s", event.Sig, topic)
		seenEvents[event.Sig] = true
		want, ok := events[event.Sig]
		if !ok {
			t.Errorf("unexpected event %s: %s", event.Sig, topic)
			continue
		}
		if topic != want {
			t.Errorf("event %s topic = %s, want %s", event.Sig, topic, want)
		}
	}
	for signature := range events {
		if !seenEvents[signature] {
			t.Errorf("missing frozen event %s", signature)
		}
	}
}
