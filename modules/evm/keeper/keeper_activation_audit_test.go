package keeper_test

import (
	"testing"

	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	putils "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

func TestActivationAddressesWithoutPrecompile(t *testing.T) {
	const upgrade = "v7.0.0"
	merge := evmkeeper.CustomPrecompileActivation{
		Upgrade: upgrade,
		Modules: map[common.Address][]string{
			common.HexToAddress("0x101A"): {"layerxledger"},
			common.HexToAddress("0x101B"): {"layerxpay"},
			common.HexToAddress("0x101C"): {"layerxauth"},
			common.HexToAddress("0x101D"): {"layerxprograms"},
			common.HexToAddress("0x0901"): {"layerxagent"},
			common.HexToAddress("0x0902"): {"layerxagent"},
			common.HexToAddress("0x0903"): {"layerxagent", "layerxexchange"},
			common.HexToAddress("0x0904"): {"layerxagent"},
			common.HexToAddress("0x0905"): {"layerxagent"},
			common.HexToAddress("0x0906"): {"layerxpay"},
			common.HexToAddress("0x0907"): {"layerxagent"},
			common.HexToAddress("0x0908"): {"layerxagent"},
		},
	}
	all := []common.Address{
		common.HexToAddress("0x0901"), common.HexToAddress("0x0902"),
		common.HexToAddress("0x0903"), common.HexToAddress("0x0904"),
		common.HexToAddress("0x0905"), common.HexToAddress("0x0906"),
		common.HexToAddress("0x0907"), common.HexToAddress("0x0908"),
		common.HexToAddress("0x101A"), common.HexToAddress("0x101B"),
		common.HexToAddress("0x101C"), common.HexToAddress("0x101D"),
	}
	identity := vm.PrecompiledContractsBerlin[common.HexToAddress("0x04")]
	require.NotNil(t, identity)
	versioned := putils.VersionedPrecompiles{upgrade: identity}
	complete := make(map[common.Address]putils.VersionedPrecompiles, len(all))
	for _, addr := range all {
		complete[addr] = versioned
	}

	for _, tc := range []struct {
		name       string
		registered map[common.Address]putils.VersionedPrecompiles
		want       []common.Address
	}{
		{name: "nil registry", want: all},
		{name: "empty registry", registered: map[common.Address]putils.VersionedPrecompiles{}, want: all},
		{
			name: "partial registry with unrelated and historical entries",
			registered: map[common.Address]putils.VersionedPrecompiles{
				common.HexToAddress("0x1001"): versioned,
				all[0]:                        versioned,
				all[3]:                        {"v6.0.0": identity},
				all[9]:                        {},
				all[10]:                       nil,
			},
			want: []common.Address{all[1], all[2], all[4], all[5], all[6], all[7], all[8], all[11]},
		},
		{name: "complete registry", registered: complete},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var k evmkeeper.Keeper
			k.SetCustomPrecompileActivation([]evmkeeper.CustomPrecompileActivation{merge})
			k.SetCustomPrecompiles(tc.registered, upgrade)
			got := k.ActivationAddressesWithoutPrecompile()
			require.Equal(t, tc.want, got)
			if len(got) > 0 {
				got[0] = common.Address{}
				require.Equal(t, tc.want, k.ActivationAddressesWithoutPrecompile())
			}
			k.SetCustomPrecompiles(complete, upgrade)
			require.Empty(t, k.ActivationAddressesWithoutPrecompile())
		})
	}

	t.Run("no activations", func(t *testing.T) {
		var k evmkeeper.Keeper
		require.Empty(t, k.ActivationAddressesWithoutPrecompile())
		k.SetCustomPrecompiles(complete, upgrade)
		require.Empty(t, k.ActivationAddressesWithoutPrecompile())
	})

	t.Run("multiple activation entries and replacement", func(t *testing.T) {
		var k evmkeeper.Keeper
		k.SetCustomPrecompiles(map[common.Address]putils.VersionedPrecompiles{all[0]: versioned}, upgrade)
		k.SetCustomPrecompileActivation([]evmkeeper.CustomPrecompileActivation{
			{Upgrade: "v8.0.0", Modules: map[common.Address][]string{all[8]: {"layerxledger"}}},
			{Upgrade: upgrade, Modules: map[common.Address][]string{all[0]: {"layerxagent"}}},
		})
		require.Equal(t, []common.Address{all[8]}, k.ActivationAddressesWithoutPrecompile())
		k.SetCustomPrecompileActivation([]evmkeeper.CustomPrecompileActivation{merge})
		require.Equal(t, all[1:], k.ActivationAddressesWithoutPrecompile())
		k.SetCustomPrecompileActivation([]evmkeeper.CustomPrecompileActivation{
			{Upgrade: "v8.0.0", Modules: map[common.Address][]string{all[8]: {"layerxledger"}}},
			{Upgrade: upgrade, Modules: map[common.Address][]string{all[1]: {"layerxagent"}}},
		})
		require.Equal(t, []common.Address{all[1], all[8]}, k.ActivationAddressesWithoutPrecompile())
	})
}
