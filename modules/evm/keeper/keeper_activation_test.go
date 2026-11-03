package keeper_test

import (
	"testing"
	"time"

	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	launchpadtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	layerxanchortypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxanchor/types"
	layerxbridgetypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxbridge/types"
	layerxcustodytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxcustody/types"
	layerxexchangetypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxexchange/types"
	xwebtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/feetoken"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/launchpad"
	agentoracle "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/oracle"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxanchor"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxbridge"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxcustody"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxexchange"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxverify"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/xweb"
	upgradetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

// forkPrecompileAddresses are the custom precompiles the Paxeer X fork brings.
func forkPrecompileAddresses() []common.Address {
	return []common.Address{
		common.HexToAddress(layerxcustody.LayerXCustodyAddress),
		common.HexToAddress(layerxexchange.ExchangeAddress),
		common.HexToAddress(layerxbridge.BridgeAddress),
		common.HexToAddress(launchpad.LaunchpadAddress),
		common.HexToAddress(feetoken.FeeTokenAddress),
		common.HexToAddress(xweb.XWebAddress),
	}
}

// forkModuleNames are the modules the fork adds, whose version map entries a
// chain that predates the fork does not carry.
func forkModuleNames() []string {
	return []string{
		layerxcustodytypes.ModuleName, layerxanchortypes.ModuleName,
		layerxexchangetypes.ModuleName, layerxbridgetypes.ModuleName,
		launchpadtypes.ModuleName, xwebtypes.ModuleName,
	}
}

func TestSetCustomPrecompileActivationRejectsAnIncompleteRecord(t *testing.T) {
	k, _ := keeper.MockEVMKeeper(t)
	address := common.HexToAddress(layerxverify.LayerXVerifyAddress)
	other := common.HexToAddress(xweb.XWebAddress)
	complete := evmkeeper.CustomPrecompileActivation{Upgrade: app.ActivationUpgrade, Modules: map[common.Address][]string{
		other: {xwebtypes.ModuleName},
	}}
	for name, activations := range map[string][]evmkeeper.CustomPrecompileActivation{
		"no entry":   nil,
		"no upgrade": {{Modules: map[common.Address][]string{address: {xwebtypes.ModuleName}}}},
		"no address": {{Upgrade: app.ActivationUpgrade}},
		"no module": {{Upgrade: app.ActivationUpgrade, Modules: map[common.Address][]string{
			address: {},
		}}},
		"empty module": {{Upgrade: app.ActivationUpgrade, Modules: map[common.Address][]string{
			address: {""},
		}}},
		"incomplete second entry": {complete, {Upgrade: app.MergeUpgrade, Modules: map[common.Address][]string{
			address: {},
		}}},
		"same address twice": {complete, {Upgrade: app.MergeUpgrade, Modules: map[common.Address][]string{
			address: {layerxanchortypes.ModuleName},
			other:   {xwebtypes.ModuleName},
		}}},
	} {
		records := activations
		require.Panics(t, func() { k.SetCustomPrecompileActivation(records) }, name)
	}
	require.PanicsWithValue(t,
		"custom precompile activations "+app.ActivationUpgrade+" and "+app.MergeUpgrade+" both name "+other.Hex(),
		func() {
			k.SetCustomPrecompileActivation([]evmkeeper.CustomPrecompileActivation{complete, {
				Upgrade: app.MergeUpgrade, Modules: map[common.Address][]string{other: {xwebtypes.ModuleName}},
			}})
		})
	require.NotPanics(t, func() {
		k.SetCustomPrecompileActivation([]evmkeeper.CustomPrecompileActivation{complete, {
			Upgrade: app.MergeUpgrade, Modules: map[common.Address][]string{address: {layerxanchortypes.ModuleName}},
		}})
	})
}

func TestCustomPrecompilesWithheldUntilTheActivationPlanIsApplied(t *testing.T) {
	testApp := app.Setup(t, false, true, false)
	ctx := testApp.GetContextForDeliverTx([]byte{}).WithBlockHeight(8).WithBlockTime(time.Now())
	k := &testApp.EvmKeeper
	fork := forkPrecompileAddresses()

	// A chain whose genesis carries the fork modules serves their precompiles already.
	full := k.CustomPrecompiles(ctx)
	verify := common.HexToAddress(layerxverify.LayerXVerifyAddress)
	anchor := common.HexToAddress(layerxanchor.LayerXAnchorAddress)
	oracle := common.HexToAddress(agentoracle.OracleAddress)
	withheld := append(forkPrecompileAddresses(), oracle)
	require.NotContains(t, full, verify)
	require.Contains(t, full, anchor)
	require.Contains(t, full, oracle)
	for _, addr := range fork {
		require.Contains(t, full, addr)
	}

	// Rewind to a chain that predates the fork: the module version map knows
	// none of the modules the fork adds.
	upgradeStore := ctx.KVStore(testApp.GetKey(upgradetypes.StoreKey))
	for _, name := range forkModuleNames() {
		upgradeStore.Delete(append([]byte{upgradetypes.VersionMapByte}, name...))
	}
	versions := testApp.UpgradeKeeper.GetModuleVersionMap(ctx)
	for _, name := range forkModuleNames() {
		require.NotContains(t, versions, name)
	}
	require.Zero(t, testApp.UpgradeKeeper.GetDoneHeight(ctx, app.ActivationUpgrade))

	// The fork precompiles and merge oracle lose their required module entries;
	// every other custom precompile keeps the set it had.
	below := k.CustomPrecompiles(ctx)
	require.Len(t, below, len(full)-len(withheld))
	require.NotContains(t, below, verify)
	require.Contains(t, below, anchor)
	for addr := range full {
		if contains(withheld, addr) {
			require.NotContains(t, below, addr)
			continue
		}
		require.Contains(t, below, addr)
		require.Same(t, full[addr], below[addr])
	}
	// Tracing serves the same set, and neither path charges the caller.
	require.Len(t, k.CustomPrecompiles(ctx.WithIsTracing(true)), len(below))
	gasBefore := ctx.GasMeter().GasConsumed()
	require.Len(t, k.CustomPrecompiles(ctx), len(below))
	require.Equal(t, gasBefore, ctx.GasMeter().GasConsumed())

	const forkHeight = int64(20)
	k.UpgradeKeeper().SetDone(ctx.WithBlockHeight(forkHeight), app.ActivationUpgrade)
	for _, height := range []int64{1, forkHeight - 1} {
		at := k.CustomPrecompiles(ctx.WithBlockHeight(height))
		require.Len(t, at, len(below)-1, height)
		require.NotContains(t, at, verify, height)
		require.NotContains(t, at, anchor, height)
		for _, addr := range withheld {
			require.NotContains(t, at, addr, height)
		}
	}
	for _, height := range []int64{forkHeight, forkHeight + 1} {
		at := k.CustomPrecompiles(ctx.WithBlockHeight(height))
		require.Len(t, at, len(full), height)
		require.Contains(t, at, verify, height)
		require.NotContains(t, at, oracle, height)
		require.Same(t, full[anchor], at[anchor], height)
		for _, addr := range fork {
			require.Contains(t, at, addr, height)
			require.Same(t, full[addr], at[addr], height)
		}
	}
}

func TestCustomPrecompilesWithheldUntilEachEntryPlanIsApplied(t *testing.T) {
	testApp := app.Setup(t, false, true, false)
	ctx := testApp.GetContextForDeliverTx([]byte{}).WithBlockHeight(8).WithBlockTime(time.Now())
	k := &testApp.EvmKeeper
	fork := forkPrecompileAddresses()
	// The exchange and launchpad addresses carry versions keyed to the
	// activation plan, so their version history withholds them below its done
	// height whichever entry names them; both stay in the activation entry.
	fork[2], fork[3] = fork[3], fork[2]
	modules := [][]string{
		{layerxcustodytypes.ModuleName},
		{layerxexchangetypes.ModuleName}, {launchpadtypes.ModuleName},
		{layerxbridgetypes.ModuleName},
		{layerxbridgetypes.ModuleName}, {xwebtypes.ModuleName},
	}
	first, second := fork[:3], fork[3:]
	firstModules := make(map[common.Address][]string, len(first))
	for i, addr := range first {
		firstModules[addr] = modules[i]
	}
	secondModules := make(map[common.Address][]string, len(second))
	for i, addr := range second {
		secondModules[addr] = modules[len(first)+i]
	}
	k.SetCustomPrecompileActivation([]evmkeeper.CustomPrecompileActivation{
		{Upgrade: app.ActivationUpgrade, Modules: firstModules},
		{Upgrade: app.AnchorStoreDeleteUpgrade, Modules: secondModules},
	})

	// While both module sets are known, both entries serve their addresses.
	full := k.CustomPrecompiles(ctx)
	for _, addr := range fork {
		require.Contains(t, full, addr)
	}

	// Rewind to a chain that predates both plans.
	upgradeStore := ctx.KVStore(testApp.GetKey(upgradetypes.StoreKey))
	for _, name := range forkModuleNames() {
		upgradeStore.Delete(append([]byte{upgradetypes.VersionMapByte}, name...))
	}
	require.Zero(t, testApp.UpgradeKeeper.GetDoneHeight(ctx, app.ActivationUpgrade))
	require.Zero(t, testApp.UpgradeKeeper.GetDoneHeight(ctx, app.AnchorStoreDeleteUpgrade))
	below := k.CustomPrecompiles(ctx)
	require.Len(t, below, len(full)-len(fork))
	for addr := range full {
		if contains(fork, addr) {
			require.NotContains(t, below, addr)
			continue
		}
		require.Same(t, full[addr], below[addr])
	}

	// Each address set is withheld until its own plan is applied and served
	// from its own done height, whichever plan is applied first.
	const firstHeight, secondHeight = int64(20), int64(12)
	k.UpgradeKeeper().SetDone(ctx.WithBlockHeight(firstHeight), app.ActivationUpgrade)
	k.UpgradeKeeper().SetDone(ctx.WithBlockHeight(secondHeight), app.AnchorStoreDeleteUpgrade)
	for _, tc := range []struct {
		height   int64
		served   []common.Address
		withheld []common.Address
	}{
		{height: 1, withheld: fork},
		{height: secondHeight - 1, withheld: fork},
		{height: secondHeight, served: second, withheld: first},
		{height: firstHeight - 1, served: second, withheld: first},
		{height: firstHeight, served: fork},
		{height: firstHeight + 1, served: fork},
	} {
		at := k.CustomPrecompiles(ctx.WithBlockHeight(tc.height))
		require.Len(t, at, len(below)+len(tc.served), tc.height)
		for _, addr := range tc.served {
			require.Same(t, full[addr], at[addr], tc.height)
		}
		for _, addr := range tc.withheld {
			require.NotContains(t, at, addr, tc.height)
		}
		versions := k.GetCustomPrecompilesVersions(ctx.WithBlockHeight(tc.height))
		require.Len(t, versions, len(at), tc.height)
		for addr := range at {
			require.Contains(t, versions, addr, tc.height)
		}
	}
}

func contains(addresses []common.Address, address common.Address) bool {
	for _, candidate := range addresses {
		if candidate == address {
			return true
		}
	}
	return false
}
