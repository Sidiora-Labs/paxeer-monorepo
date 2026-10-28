package layerxauth_test

import (
	"math/big"
	"strings"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxauth"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

func TestAuthSurfaceFrozen(t *testing.T) {
	id := [32]byte{1}
	arguments := map[string][]interface{}{
		"grantSession":          {[]byte{1}, uint64(100), id},
		"grantSessionSuccessor": {[]byte{1}, uint64(100), id, [32]byte{2}, [32]byte{3}},
		"grantCapability":       {[]byte{1}},
		"revokeGrant":           {id, uint8(1), uint64(1)},
		"resultByIdempotency":   {id, id},
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
