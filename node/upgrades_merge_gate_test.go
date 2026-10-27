package app

import (
	"strings"
	"testing"
	"time"

	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	agenttypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxagent/types"
	exchangetypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxexchange/types"
	paytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/eip712"
	putils "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	upgradetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestMergePrecompileGate(t *testing.T) {
	wrapper := NewTestWrapper(t, time.Now().UTC(), secp256k1.GenPrivKey().PubKey(), true)
	a, ctx := wrapper.App, wrapper.Ctx
	contract, err := eip712.NewPrecompile(&a.UpgradeKeeper, MergeUpgrade, 256, 256)
	require.NoError(t, err)
	all := make(map[common.Address]putils.VersionedPrecompiles, len(precompiles.MergeAddressTable))
	for _, entry := range precompiles.MergeAddressTable {
		all[entry.Address] = putils.VersionedPrecompiles{LatestUpgrade: contract, MergeUpgrade: contract}
	}
	a.EvmKeeper.SetCustomPrecompiles(all, LatestUpgrade)
	require.Zero(t, a.UpgradeKeeper.GetDoneHeight(ctx, MergeUpgrade))
	for _, tracing := range []bool{false, true} {
		before := ctx.WithBlockHeight(99).WithIsTracing(tracing)
		for _, entry := range precompiles.MergeAddressTable {
			require.NotContains(t, a.EvmKeeper.CustomPrecompiles(before), entry.Address)
			require.NotContains(t, a.EvmKeeper.GetCustomPrecompilesVersions(before), entry.Address)
		}
	}
	a.UpgradeKeeper.SetDone(ctx.WithBlockHeight(100), MergeUpgrade)
	for _, tracing := range []bool{false, true} {
		for _, height := range []int64{99, 100, 101} {
			at := ctx.WithBlockHeight(height).WithIsTracing(tracing)
			served := a.EvmKeeper.CustomPrecompiles(at)
			for _, entry := range precompiles.MergeAddressTable {
				if height < 100 {
					require.NotContains(t, served, entry.Address)
				} else {
					require.Same(t, contract, served[entry.Address])
				}
			}
		}
	}
}

func TestMergePrecompileGateUsesModuleNameConstants(t *testing.T) {
	gate := mergePrecompileGate()
	require.Equal(t, MergeUpgrade, gate.Upgrade)
	require.Len(t, gate.Modules, 12)
	for _, address := range []string{"0x101B", "0x0906"} {
		require.Equal(t, []string{paytypes.ModuleName}, gate.Modules[common.HexToAddress(address)])
	}
	for _, address := range []string{"0x0901", "0x0902", "0x0903", "0x0904", "0x0905", "0x0907", "0x0908"} {
		want := []string{agenttypes.ModuleName}
		if address == "0x0903" {
			want = append(want, exchangetypes.ModuleName)
		}
		require.Equal(t, want, gate.Modules[common.HexToAddress(address)])
	}
	require.Equal(t, []string{"layerxprograms"}, gate.Modules[common.HexToAddress("0x101D")])
}

func TestMergePlanRefusesMissingMergeConstructors(t *testing.T) {
	w := NewTestWrapper(t, time.Now().UTC(), secp256k1.GenPrivKey().PubKey(), true)
	a, ctx := w.App, w.Ctx.WithBlockHeight(mergeHeight)
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	rewindBeforeMerge(t, a, ctx)
	versions := a.UpgradeKeeper.GetModuleVersionMap(ctx)
	plan := upgradetypes.Plan{Name: MergeUpgrade, Height: mergeHeight, Info: activationInfo(t)}
	all, err := precompiles.GetCustomPrecompilesE(LatestUpgrade, a.GetPrecompileKeepers())
	require.NoError(t, err)
	t.Run("registered constructors", func(t *testing.T) {
		a.EvmKeeper.SetCustomPrecompiles(all, LatestUpgrade)
		a.EvmKeeper.SetCustomPrecompileActivation([]evmkeeper.CustomPrecompileActivation{activationPrecompileGate()})
		require.Empty(t, a.EvmKeeper.ActivationAddressesWithoutPrecompile())
		missing := precompiles.MissingMergeConstructors()
		require.NotEmpty(t, missing)
		cache, _ := ctx.CacheContext()
		before := snapshotStores(t, a, cache)
		returned, err := a.mergeUpgradePlan(cache, plan, versions)
		require.ErrorContains(t, err, "missing merge constructors or activation precompiles")
		for _, entry := range missing {
			require.ErrorContains(t, err, entry.Address.Hex())
			require.Equal(t, 1, strings.Count(err.Error(), entry.Address.Hex()))
		}
		require.Equal(t, versions, returned)
		require.Equal(t, before, snapshotStores(t, a, cache))
		require.False(t, a.Sequenced(cache, mergeHeight+1))
		require.Zero(t, a.UpgradeKeeper.GetDoneHeight(cache, MergeUpgrade))
	})
	a.EvmKeeper.SetCustomPrecompileActivation([]evmkeeper.CustomPrecompileActivation{mergePrecompileGate()})
	cases := []struct {
		name      string
		addresses []common.Address
	}{{name: "all merge addresses"}}
	for _, entry := range precompiles.MergeAddressTable {
		cases[0].addresses = append(cases[0].addresses, entry.Address)
		cases = append(cases, struct {
			name      string
			addresses []common.Address
		}{entry.Address.Hex(), []common.Address{entry.Address}})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			contracts := make(map[common.Address]putils.VersionedPrecompiles, len(all))
			for address, contract := range all {
				contracts[address] = contract
			}
			for _, address := range tc.addresses {
				delete(contracts, address)
			}
			a.EvmKeeper.SetCustomPrecompiles(contracts, LatestUpgrade)
			missing := a.EvmKeeper.ActivationAddressesWithoutPrecompile()
			cache, _ := ctx.CacheContext()
			before := snapshotStores(t, a, cache)
			returned, err := a.mergeUpgradePlan(cache, plan, versions)
			require.ErrorContains(t, err, "missing merge constructors or activation precompiles")
			for _, address := range tc.addresses {
				require.Contains(t, missing, address)
				require.ErrorContains(t, err, address.Hex())
				require.Equal(t, 1, strings.Count(err.Error(), address.Hex()))
			}
			for _, entry := range precompiles.MissingMergeConstructors() {
				require.ErrorContains(t, err, entry.Address.Hex())
			}
			require.Equal(t, versions, returned)
			require.Equal(t, before, snapshotStores(t, a, cache))
			require.False(t, a.Sequenced(cache, mergeHeight+1))
			require.Zero(t, a.UpgradeKeeper.GetDoneHeight(cache, MergeUpgrade))
		})
	}
}
