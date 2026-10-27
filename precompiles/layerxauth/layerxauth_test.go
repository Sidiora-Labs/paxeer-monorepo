package layerxauth_test

import (
	"math/big"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxauth"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

func TestAuthMergeConstructorRegistered(t *testing.T) {
	h := newHarness(t)
	address := common.HexToAddress("0x000000000000000000000000000000000000101C")
	require.Equal(t, address, common.HexToAddress(layerxauth.AuthAddress))
	var name string
	for _, entry := range precompiles.MergeAddressTable {
		if entry.Address == address {
			name = entry.Name
		}
	}
	require.Equal(t, "layerxauth", name)
	built, err := layerxauth.NewMergePrecompile(h.keepers, precompiles.MergeUpgrade)
	require.NoError(t, err)
	require.Equal(t, address, built.Address())
	require.Equal(t, name, built.GetName())
	set := precompiles.GetCustomPrecompiles(precompiles.MergeUpgrade, h.keepers)
	versions, found := set[address]
	require.True(t, found)
	served, ok := versions[precompiles.MergeUpgrade].(precompiles.IPrecompile)
	require.True(t, ok)
	require.NotNil(t, served)
	require.Equal(t, address, served.Address())
	require.Equal(t, name, served.GetName())
	require.Equal(t, built.GetABI(), served.GetABI())
	require.EqualError(t, precompiles.RegisterMergeConstructor(address, layerxauth.NewMergePrecompile),
		"merge constructor for "+address.Hex()+" already registered")

	a := built.GetABI()
	for methodName, method := range a.Methods {
		t.Run(methodName, func(t *testing.T) {
			row, found := layerxauth.GasTable[methodName]
			require.True(t, found)
			require.Equal(t, !method.IsConstant(), row.Write)
			input := append(append([]byte(nil), method.ID...), make([]byte, 128)...)
			require.Equal(t, row.Gas(input[4:]), built.RequiredGas(input))
			require.Equal(t, row.Gas(input[4:]), h.precompile.GetExecutor().RequiredGas(input[4:], &method))
		})
	}
	for _, keepers := range []utils.Keepers{nil, &utils.EmptyKeepers{}, authKeepers{PrecompileKeepers: h.keepers.PrecompileKeepers}} {
		p, err := layerxauth.NewMergePrecompile(keepers, precompiles.MergeUpgrade)
		require.NoError(t, err)
		input, err := a.Pack("revokeGrant", [32]byte{1}, uint8(1), uint64(1))
		require.NoError(t, err)
		evm, _ := h.evm()
		data, err := p.Run(evm, h.caller, address, input, nil, false, false, nil)
		require.ErrorIs(t, err, vm.ErrExecutionReverted)
		assertLxpRevert(t, data, "LXP_PRECOMPILE_NO_CONTEXT: layerxauth keeper is unavailable")
	}
}

func TestAuthWriteRefusals(t *testing.T) {
	h := newHarness(t)
	a := h.precompile.GetABI()
	writes := map[string][]interface{}{
		"grantSession":          {[]byte{1, 2, 3}, uint64(100), [32]byte{1}},
		"grantSessionSuccessor": {[]byte{1, 2, 3}, uint64(100), [32]byte{1}, [32]byte{2}, [32]byte{3}},
		"grantCapability":       {[]byte{1, 2, 3}},
		"revokeGrant":           {[32]byte{1}, uint8(1), uint64(1)},
	}
	for method, args := range writes {
		t.Run(method, func(t *testing.T) {
			input, err := a.Pack(method, args...)
			require.NoError(t, err)
			cases := []struct {
				name     string
				caller   common.Address
				value    *big.Int
				readOnly bool
				delegate bool
				reason   string
			}{
				{"staticcall", h.caller, nil, true, false, "LXP_AUTH_SCOPE: cannot call a layerxauth state change from staticcall"},
				{"delegatecall", h.caller, nil, false, true, "LXP_AUTH_SCOPE: cannot delegatecall layerxauth"},
				{"value", h.caller, big.NewInt(1), false, false, "LXP_PRECOMPILE_NON_PAYABLE: sending funds to a non-payable function"},
				{"unbound", h.unbound, nil, false, false, "LXP_AUTH_SCOPE"},
			}
			for _, test := range cases {
				t.Run(test.name, func(t *testing.T) {
					evm, db := h.evm()
					before := hostSnapshot(t, db.Ctx(), h.hostKey)
					data, err := h.precompile.Run(evm, test.caller, h.precompile.Address(), input, test.value, test.readOnly, test.delegate, nil)
					require.ErrorIs(t, err, vm.ErrExecutionReverted)
					assertLxpRevert(t, data, test.reason)
					if test.name != "value" {
						require.ErrorIs(t, db.GetPrecompileError(), types.ErrAuthScope)
					}
					require.Equal(t, before, hostSnapshot(t, db.Ctx(), h.hostKey))
					require.Empty(t, db.Ctx().EventManager().Events())
				})
			}
			t.Run("boundCallerReachesDispatch", func(t *testing.T) {
				for _, value := range []*big.Int{nil, big.NewInt(0)} {
					evm, db := h.evm()
					before := hostSnapshot(t, db.Ctx(), h.hostKey)
					data, err := h.precompile.Run(evm, h.caller, h.unbound, input, value, false, false, nil)
					require.ErrorIs(t, err, vm.ErrExecutionReverted)
					assertLxpRevert(t, data, "LXP_PRECOMPILE_UNKNOWN_METHOD: layerxauth method "+method+" is unavailable")
					require.Equal(t, before, hostSnapshot(t, db.Ctx(), h.hostKey))
				}
			})
		})
	}
}
