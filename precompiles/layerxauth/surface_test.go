package layerxauth_test

import (
	"math/big"
	"strings"
	"testing"

	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxauth"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

func TestAuthSurfaceFrozen(t *testing.T) {
	id := [32]byte{1}
	arguments := map[string][]interface{}{
		"recordLifecycle":       {[]byte{0x71, 1, 0, 2}, id},
		"grantSession":          {[]byte{1}, uint64(100), id},
		"grantSessionSuccessor": {[]byte{1}, uint64(100), id, [32]byte{2}, [32]byte{3}},
		"grantCapability":       {[]byte{1}},
		"revokeGrant":           {id, uint8(1), uint64(1)},
		"getGrant":              {id},
		"isLive":                {id, uint64(1), uint64(1_800_000_000_000)},
		"sessionKey":            {id},
		"capabilityScope":       {id},
		"checkActivity":         {id, uint32(0x00010005), id, big.NewInt(1)},
	}
	a := layerxauth.GetABI()
	require.Len(t, a.Methods, 10)
	require.Len(t, arguments, 10)
	require.Len(t, layerxauth.GasTable, 10)
	for name, method := range a.Methods {
		t.Run(name, func(t *testing.T) {
			args, found := arguments[name]
			require.True(t, found, "unexpected auth method %s", name)
			packed, err := method.Inputs.Pack(args...)
			require.NoError(t, err)
			input := append(append([]byte(nil), method.ID...), packed...)
			h := newHarness(t)
			require.Equal(t, a, h.precompile.GetABI())
			row, found := layerxauth.GasTable[name]
			require.True(t, found, "missing gas row for %s", name)
			require.Equal(t, !method.IsConstant(), row.Write)
			for _, payload := range [][]byte{packed, nil, {1}, make([]byte, 32), make([]byte, 33)} {
				calldata := append(append([]byte(nil), method.ID...), payload...)
				require.Equal(t, row.Gas(payload), h.precompile.RequiredGas(calldata), "wrapper gas for %d bytes", len(payload))
				require.Equal(t, row.Gas(payload), h.precompile.GetExecutor().RequiredGas(payload, &method), "executor gas for %d bytes", len(payload))
			}
			evm, _ := h.evm()
			data, err := h.precompile.Run(evm, h.caller, h.precompile.Address(), input, nil, method.IsConstant(), false, nil)
			if err != nil {
				require.ErrorIs(t, err, vm.ErrExecutionReverted)
				reason, unpackErr := abi.UnpackRevert(data)
				require.NoError(t, unpackErr)
				require.True(t, strings.HasPrefix(reason, "LXP_"), reason)
				require.NotContains(t, reason, "UNKNOWN_METHOD")
				require.NotContains(t, reason, "no method with id")
				require.NotContains(t, reason, "PRECOMPILE_NO_CONTEXT")
			} else {
				_, err := method.Outputs.Unpack(data)
				require.NoError(t, err)
			}
		})
	}
}

func TestPrecompileSurface(t *testing.T) {
	expected := pcommon.GasTable{
		"recordLifecycle":       {Write: true, Surcharge: 36680},
		"grantSession":          {Write: true, Surcharge: 36680},
		"grantSessionSuccessor": {Write: true, Surcharge: 38600},
		"grantCapability":       {Write: true, Surcharge: 34760},
		"revokeGrant":           {Write: true, Surcharge: 5000},
		"getGrant":              {Write: false, Surcharge: 1108},
		"isLive":                {Write: false, Surcharge: 1300},
		"sessionKey":            {Write: false, Surcharge: 1108},
		"capabilityScope":       {Write: false, Surcharge: 1108},
		"checkActivity":         {Write: false, Surcharge: 1396},
	}
	h := newHarness(t)
	a := h.precompile.GetABI()
	require.Equal(t, layerxauth.GetABI(), a)
	require.NotContains(t, a.Methods, "resultByIdempotency")
	require.NotContains(t, layerxauth.GasTable, "resultByIdempotency")
	require.Len(t, a.Methods, len(expected))
	require.Equal(t, expected, layerxauth.GasTable)
	for name, method := range a.Methods {
		t.Run(name, func(t *testing.T) {
			row, found := expected[name]
			require.True(t, found, "unexpected auth method %s", name)
			require.Equal(t, row.Write, !method.IsConstant())
			for _, payload := range [][]byte{nil, {1}, make([]byte, 32), make([]byte, 33)} {
				input := append(append([]byte(nil), method.ID...), payload...)
				gas := pcommon.DefaultGasCost(payload, row.Write) + row.Surcharge
				require.Equal(t, gas, h.precompile.RequiredGas(input))
				require.Equal(t, gas, h.precompile.GetExecutor().RequiredGas(payload, &method))
			}
		})
	}
}
