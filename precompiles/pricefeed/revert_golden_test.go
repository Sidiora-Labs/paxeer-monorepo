package pricefeed_test

import (
	"encoding/hex"
	"encoding/json"
	"math/big"
	"os"
	"testing"

	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/pricefeed"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type revertGoldenCall1020 struct {
	precompile *pricefeed.Precompile
	input      []byte
	readOnly   bool
	delegate   bool
}

func (c revertGoldenCall1020) Run(h *harness) ([]byte, error) {
	return c.precompile.Run(h.evm, h.caller, h.caller, c.input, big.NewInt(0), c.readOnly, c.delegate, nil)
}

type revertGoldenEntry1020 struct {
	Reason string `json:"reason"`
	Data   string `json:"data"`
}

func TestRevertTextGolden1020(t *testing.T) {
	wantJSON, err := os.ReadFile("testdata/revert_golden.json")
	require.NoError(t, err)
	var want map[string]revertGoldenEntry1020
	require.NoError(t, json.Unmarshal(wantJSON, &want))
	got := make(map[string]revertGoldenEntry1020)
	record := func(t *testing.T, name string, data []byte) {
		t.Helper()
		require.GreaterOrEqual(t, len(data), 4)
		assert.Equal(t, []byte{0x08, 0xc3, 0x79, 0xa0}, data[:4])
		reason, err := abi.UnpackRevert(data)
		require.NoError(t, err)
		expected, ok := want[name]
		require.True(t, ok, "missing golden for %s", name)
		expectedData, err := hex.DecodeString(expected.Data)
		require.NoError(t, err)
		assert.Equal(t, expectedData, data, "revert bytes for %s", name)
		assert.Equal(t, expected.Reason, reason, "revert text for %s", name)
		got[name] = revertGoldenEntry1020{Reason: reason, Data: hex.EncodeToString(data)}
	}

	ids := make([]*big.Int, 65)
	for i := range ids {
		ids[i] = big.NewInt(0)
	}
	for _, tc := range []struct {
		name          string
		method        string
		args          []interface{}
		readOnly      bool
		delegate      bool
		missingKeeper bool
		selectorOnly  bool
	}{
		{name: "delegatecall", method: "getMarketCount", delegate: true},
		{name: "staticcall_write", method: "revokeFeedKey", readOnly: true},
		{name: "missing_keeper", method: "getMarketCount", missingKeeper: true},
		{name: "get_prices_65_ids", method: "getPrices", args: []interface{}{ids}, readOnly: true},
		{name: "unknown_market", method: "getPrice", args: []interface{}{big.NewInt(40)}, readOnly: true},
		{name: "unknown_selector", readOnly: true},
		{name: "wrong_argument_count", method: "getPrice", readOnly: true, selectorOnly: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			assert.Equal(t, pricefeedAddress, h.precompile.Address())
			call := revertGoldenCall1020{precompile: h.precompile, readOnly: tc.readOnly, delegate: tc.delegate}
			if tc.missingKeeper {
				call.precompile, err = pricefeed.NewPrecompileWithKeeper(nil, nil)
				require.NoError(t, err)
			}
			switch {
			case tc.method == "":
				call.input = []byte{0xff, 0xff, 0xff, 0xff}
			case tc.selectorOnly:
				call.input = h.method(tc.method).ID
			default:
				call.input = h.input(tc.method, tc.args...)
			}
			data, err := call.Run(h)
			require.ErrorIs(t, err, vm.ErrExecutionReverted)
			record(t, tc.name, data)
		})
	}

	for _, name := range []string{"unknown_method", "executor_wrong_argument_count"} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t)
			method := h.method("getPrice")
			if name == "unknown_method" {
				method = abi.NewMethod("unknownMethod", "unknownMethod", abi.Function, "view", true, false, nil, nil)
			}
			data, err := h.precompile.GetExecutor().Execute(h.ctx(), &method, h.caller, h.caller, nil, big.NewInt(0), true, h.evm, nil)
			require.Error(t, err)
			require.Nil(t, data)
			record(t, name, pcommon.RevertReason(err))
		})
	}

	gotJSON, err := json.MarshalIndent(got, "", "  ")
	require.NoError(t, err)
	assert.Equal(t, wantJSON, append(gotJSON, '\n'), "revert golden must match byte for byte")
}
