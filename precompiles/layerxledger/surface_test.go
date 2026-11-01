package layerxledger_test

import (
	"math/big"
	"strings"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxledger"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

func TestLedgerSurfaceFrozen(t *testing.T) {
	id := [32]byte{1}
	arguments := map[string][]interface{}{
		"register":      {id, [32]byte{2}, "TST", "Test asset", uint8(6), big.NewInt(100), uint8(0), []byte{}},
		"pause":         {id},
		"unpause":       {id},
		"openAccount":   {id},
		"send":          {[]byte{1}},
		"draw":          {id, id, big.NewInt(1), id, uint64(0), id, id},
		"payExact":      {id, id, id, big.NewInt(1), uint64(0), id},
		"issueGrant":    {[]byte{1}},
		"revokeGrant":   {id, uint64(1)},
		"mint":          {id, id, big.NewInt(1)},
		"burn":          {id, id, big.NewInt(1)},
		"getAsset":      {id},
		"balanceOf":     {id, id},
		"totalUnits":    {id},
		"accountId":     {id, id},
		"accountOwner":  {id},
		"nextSequence":  {id, id},
		"getPayerGrant": {id},
	}
	a := layerxledger.GetABI()
	require.Len(t, a.Methods, 18)
	require.Len(t, arguments, 18)
	require.Len(t, layerxledger.GasTable, 18)
	for name, method := range a.Methods {
		t.Run(name, func(t *testing.T) {
			args, found := arguments[name]
			require.True(t, found, "unexpected ledger method %s", name)
			packed, err := method.Inputs.Pack(args...)
			require.NoError(t, err)
			input := append(append([]byte(nil), method.ID...), packed...)
			h := newHarness(t)
			h.bindCaller(t)
			require.Equal(t, a, h.precompile.GetABI())
			row, found := layerxledger.GasTable[name]
			require.True(t, found, "missing gas row for %s", name)
			require.Equal(t, !method.IsConstant(), row.Write)
			for _, payload := range [][]byte{packed, nil, {1}, make([]byte, 32), make([]byte, 33)} {
				calldata := append(append([]byte(nil), method.ID...), payload...)
				require.Equal(t, row.Gas(payload), h.precompile.RequiredGas(calldata), "wrapper gas for %d bytes", len(payload))
				require.Equal(t, row.Gas(payload), h.precompile.GetExecutor().RequiredGas(payload, &method), "executor gas for %d bytes", len(payload))
			}
			data, err := h.precompile.Run(h.evm, h.caller, h.precompile.Address(), input, nil, method.IsConstant(), false, nil)
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
