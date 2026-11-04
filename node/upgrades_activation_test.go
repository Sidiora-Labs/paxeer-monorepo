package app

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	launchpadtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	layerxagenttypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxagent/types"
	layerxanchortypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxanchor/types"
	layerxbridgetypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxbridge/types"
	layerxchainstypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxchains/types"
	layerxcustodytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxcustody/types"
	layerxexchangetypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxexchange/types"
	perpstypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	pricefeedtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/types"
	spottypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/spot/types"
	xwebtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/types"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	anchorprecompile "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxanchor"
	verifyprecompile "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxverify"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	upgradetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/keys"
	scconfig "github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
	storageproto "github.com/Sidiora-Labs/Paxeer-X-Network/storage/proto"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/state_db/sc/composite"
	"github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
	"golang.org/x/mod/semver"
)

// preForkStoreCount is the number of stores a chain that applied none of the
// LayerX plans mounted. The activation plan mounts every store the application
// carries beyond them, so a store added to the application either joins the plan
// or is accounted for here.
const preForkStoreCount = 20

// protoState is the encoding a module defines for the state it stores. A module
// whose state carries no proto encoding keeps it as JSON, and is compared
// through that encoding instead.
type protoState interface {
	Marshal() ([]byte, error)
}

// requireSameState compares a module's stored state against the state its own
// default genesis carries, through the encoding that module stores.
func requireSameState(t *testing.T, name string, want, got any) {
	t.Helper()
	require.Equal(t, encodeState(t, want), encodeState(t, got), name)
}

func encodeState(t *testing.T, state any) []byte {
	t.Helper()
	if message, ok := state.(protoState); ok {
		encoded, err := message.Marshal()
		require.NoError(t, err)
		return encoded
	}
	encoded, err := json.Marshal(state)
	require.NoError(t, err)
	return encoded
}

// rewindBeforeActivation turns the test application into a chain that predates
// the fork: the stores of the modules the activation plan initialises hold
// nothing and the module version map knows none of them, exactly as on a chain
// that applied neither the v6.6 nor the v6.8 plan.
func rewindBeforeActivation(t *testing.T, a *App, ctx sdk.Context) {
	t.Helper()
	for _, name := range activationStoreUpgrades().Added {
		if name == layerxanchortypes.StoreKey {
			continue // Historical anchor state is retained, never reinitialized.
		}
		store := ctx.KVStore(a.GetKey(name))
		iter := store.Iterator(nil, nil)
		var stale [][]byte
		for ; iter.Valid(); iter.Next() {
			stale = append(stale, iter.Key())
		}
		require.NoError(t, iter.Close())
		require.NotEmpty(t, stale, name)
		for _, key := range stale {
			store.Delete(key)
		}
	}
	upgradeStore := ctx.KVStore(a.GetKey(upgradetypes.StoreKey))
	for _, name := range activationModules() {
		upgradeStore.Delete(append([]byte{upgradetypes.VersionMapByte}, name...))
	}
	versions := a.UpgradeKeeper.GetModuleVersionMap(ctx)
	for _, name := range activationModules() {
		require.NotContains(t, versions, name)
	}
	require.False(t, a.activationModulesPresent(ctx))
	require.Zero(t, a.UpgradeKeeper.GetDoneHeight(ctx, ActivationUpgrade))
}

func TestActivationUpgradeIsRegisteredBesideTheTagList(t *testing.T) {
	tags, err := f.ReadFile("tags")
	require.NoError(t, err)
	names := parseUpgradesList(string(tags))
	require.NotContains(t, names, ActivationUpgrade)
	require.Equal(t, MergeUpgrade, LatestUpgrade)
	require.Equal(t, MergeUpgrade, names[len(names)-1])
	require.Equal(t, 1, semver.Compare(LatestUpgrade, ActivationUpgrade))

	a := NewTestWrapper(t, time.Now().UTC(), secp256k1.GenPrivKey().PubKey(), false).App
	require.True(t, a.UpgradeKeeper.HasHandler(ActivationUpgrade))
	require.True(t, a.UpgradeKeeper.HasHandler(xwebUpgrade))
	require.True(t, a.UpgradeKeeper.HasHandler(sidioraFeeTokenUpgrade))
}

func TestMergePlanIsRegisteredBesideTheTagList(t *testing.T) {
	require.Equal(t, "v7.0", MergeUpgrade)
	require.Equal(t, []string{"v6.9", "v6.10", "v6.12.0", "v7.0", "v7.0-anchor-store-delete"}, knownUpgradePlans())

	tags, err := f.ReadFile("tags")
	require.NoError(t, err)
	names := parseUpgradesList(string(tags))
	require.Contains(t, names, MergeUpgrade)
	require.Equal(t, MergeUpgrade, LatestUpgrade)
	require.Equal(t, LatestUpgrade, names[len(names)-1])
	require.Equal(t, 1, semver.Compare(MergeUpgrade, V6120Upgrade))

	gate := mergePrecompileGate()
	require.Equal(t, MergeUpgrade, gate.Upgrade)
	want := []common.Address{
		common.HexToAddress("0x101A"),
		common.HexToAddress("0x101B"),
		common.HexToAddress("0x101C"),
		common.HexToAddress("0x101D"),
		common.HexToAddress("0x101E"),
		common.HexToAddress("0x0901"),
		common.HexToAddress("0x0902"),
		common.HexToAddress("0x0903"),
		common.HexToAddress("0x0904"),
		common.HexToAddress("0x0905"),
		common.HexToAddress("0x0906"),
		common.HexToAddress("0x0907"),
		common.HexToAddress("0x0908"),
	}
	got := make([]common.Address, 0, len(gate.Modules))
	activation := activationPrecompileGate()
	for addr := range gate.Modules {
		got = append(got, addr)
		require.NotContains(t, activation.Modules, addr)
	}
	require.ElementsMatch(t, want, got)
	require.Equal(t, []string{layerxchainstypes.ModuleName}, gate.Modules[common.HexToAddress("0x101E")])
}

func expectedMergeStores() []string {
	return []string{keys.SequencerStoreKey, keys.LayerXLedgerStoreKey, keys.LayerXAuthStoreKey, keys.LayerXPayStoreKey, keys.LayerXChainsStoreKey, keys.LayerXProgramsStoreKey, keys.LayerXAgentStoreKey, keys.LayerXHostStoreKey}
}

func TestActivationStoreUpgradesMountEveryForkStore(t *testing.T) {
	upgrades, ok := layerxStoreUpgrades(ActivationUpgrade)
	require.True(t, ok)
	require.Equal(t, activationStoreUpgrades(), upgrades)
	require.Empty(t, upgrades.Deleted)
	require.Empty(t, upgrades.Renamed)
	require.Equal(t, append(v66StoreUpgrades().Added, v68StoreUpgrades().Added...), upgrades.Added)
	require.Subset(t, upgrades.Added, v65StoreUpgrades().Added)

	forkModules := []struct {
		store  string
		module string
	}{
		{layerxcustodytypes.StoreKey, layerxcustodytypes.ModuleName},
		{layerxanchortypes.StoreKey, layerxanchortypes.ModuleName},
		{layerxexchangetypes.StoreKey, layerxexchangetypes.ModuleName},
		{layerxbridgetypes.StoreKey, layerxbridgetypes.ModuleName},
		{launchpadtypes.StoreKey, launchpadtypes.ModuleName},
		{xwebtypes.StoreKey, xwebtypes.ModuleName},
	}
	require.Len(t, upgrades.Added, len(forkModules))
	require.Len(t, activationModules(), len(forkModules))
	merge, ok := layerxStoreUpgrades(MergeUpgrade)
	require.True(t, ok)
	require.Equal(t, mergeStoreUpgrades(), merge)
	require.Equal(t, expectedMergeStores(), merge.Added)
	require.Empty(t, merge.Deleted)
	require.Empty(t, merge.Renamed)
	v6120Stores := []string{perpstypes.StoreKey, spottypes.StoreKey, pricefeedtypes.StoreKey}
	v6120, ok := layerxStoreUpgrades(V6120Upgrade)
	require.True(t, ok)
	require.Equal(t, v6120Stores, v6120.Added)
	require.Len(t, kvStoreKeyNames, preForkStoreCount+len(forkModules)+len(v6120Stores)+len(mergeStoreUpgrades().Added))

	a := NewTestWrapper(t, time.Now().UTC(), secp256k1.GenPrivKey().PubKey(), false).App
	for i, fork := range forkModules {
		require.Equal(t, fork.store, upgrades.Added[i])
		require.Equal(t, fork.module, activationModules()[i])
		require.Contains(t, kvStoreKeyNames, fork.store)
		require.Contains(t, keys.MemIAVLStoreKeys, fork.store)
		require.NotNil(t, a.GetKey(fork.store))
		requireActivationModuleLifecycle(t, a, fork.module)
	}
	for _, store := range v6120Stores {
		require.NotContains(t, upgrades.Added, store)
		require.NotContains(t, merge.Added, store)
		require.Contains(t, kvStoreKeyNames, store)
		require.Contains(t, keys.MemIAVLStoreKeys, store)
		require.NotNil(t, a.GetKey(store))
	}
	for _, store := range merge.Added {
		require.NotContains(t, upgrades.Added, store)
		require.Contains(t, kvStoreKeyNames, store)
		require.Contains(t, keys.MemIAVLStoreKeys, store)
		require.NotNil(t, a.GetKey(store))
	}
}

func TestActivationUpgradeInitialisesEveryForkModuleOnce(t *testing.T) {
	testWrapper := NewTestWrapper(t, time.Now().UTC(), secp256k1.GenPrivKey().PubKey(), true)
	a, ctx := testWrapper.App, testWrapper.Ctx
	rewindBeforeActivation(t, a, ctx)

	plan := upgradetypes.Plan{Name: ActivationUpgrade, Height: ctx.BlockHeight()}
	require.True(t, a.UpgradeKeeper.HasHandler(plan.Name))
	a.UpgradeKeeper.ApplyUpgrade(ctx, plan)

	require.Equal(t, plan.Height, a.UpgradeKeeper.GetDoneHeight(ctx, ActivationUpgrade))
	require.Equal(t, a.mm.GetVersionMap(), a.UpgradeKeeper.GetModuleVersionMap(ctx))
	require.True(t, a.activationModulesPresent(ctx))

	custody := a.LayerXCustodyKeeper.GetParams(ctx)
	requireSameState(t, layerxcustodytypes.ModuleName, &layerxcustodytypes.DefaultGenesis().Params, &custody)
	anchor := a.LayerXAnchorKeeper.GetParams(ctx)
	requireSameState(t, layerxanchortypes.ModuleName, &layerxanchortypes.DefaultGenesis().Params, &anchor)
	exchange := a.LayerXExchangeKeeper.GetParams(ctx)
	requireSameState(t, layerxexchangetypes.ModuleName, &layerxexchangetypes.DefaultGenesis().Params, &exchange)
	bridge := a.LayerXBridgeKeeper.GetParams(ctx)
	requireSameState(t, layerxbridgetypes.ModuleName, &layerxbridgetypes.DefaultGenesis().Params, &bridge)
	launchpad := a.LaunchpadKeeper.GetParams(ctx)
	requireSameState(t, launchpadtypes.ModuleName, &launchpadtypes.DefaultGenesis().Params, &launchpad)
	xweb := a.XWebKeeper.GetParams(ctx)
	requireSameState(t, xwebtypes.ModuleName, &xwebtypes.DefaultGenesis().Params, &xweb)

	// The web module comes up paused, with no attestor and no threshold.
	require.True(t, a.xwebInitialised(ctx))
	require.True(t, a.XWebKeeper.IsPaused(ctx))
	require.Empty(t, a.XWebKeeper.GetAttestorSet(ctx).Attestors)
	require.Zero(t, a.XWebKeeper.Threshold(ctx))
	require.Zero(t, a.XWebKeeper.Nonce(ctx))
	require.True(t, a.xwebLive(ctx))

	// The plan registers no remote asset: the Sidiora pair stays an owner action
	// through the bridge module's own admin path.
	_, found := a.LayerXBridgeKeeper.GetAssetByDenom(ctx, layerxbridgetypes.SidioraDenom())
	require.False(t, found)
}

func requireActivationModuleLifecycle(t *testing.T, a *App, name string) {
	t.Helper()
	if name == layerxanchortypes.ModuleName {
		require.NotContains(t, a.mm.Modules, name)
		require.NotContains(t, a.mm.OrderInitGenesis, name)
		require.NotContains(t, a.mm.GetVersionMap(), name)
		return
	}
	require.Contains(t, a.mm.Modules, name)
	require.NotZero(t, a.mm.Modules[name].ConsensusVersion())
}

func requireAnchorRetirementRevert(t *testing.T, a *App, ctx sdk.Context, method string, args ...interface{}) {
	t.Helper()
	address := common.HexToAddress(anchorprecompile.LayerXAnchorAddress)
	contract := a.EvmKeeper.CustomPrecompiles(ctx)[address]
	require.NotNil(t, contract)
	typed, ok := contract.(*pcommon.Precompile)
	require.True(t, ok)
	input, err := typed.GetABI().Pack(method, args...)
	require.NoError(t, err)
	evm := &vm.EVM{StateDB: state.NewDBImpl(ctx, &a.EvmKeeper, true)}
	output, err := contract.Run(evm, common.Address{1}, common.Address{1}, input, nil, true, false, nil)
	require.ErrorIs(t, err, vm.ErrExecutionReverted)
	require.Equal(t, pcommon.RetiredRevert(anchorprecompile.PrecompileName), output)
	reason, err := abi.UnpackRevert(output)
	require.NoError(t, err)
	require.Equal(t, "LXP_MODULE_DISABLED: module disabled; layerxAnchor retired", reason)
}

func TestActivationServesTheForkPrecompilesOnlyFromItsHeight(t *testing.T) {
	w := NewTestWrapper(t, time.Now().UTC(), secp256k1.GenPrivKey().PubKey(), true)
	a, ctx := w.App, w.Ctx.WithBlockHeight(mergeHeight)
	t.Cleanup(func() { require.NoError(t, a.Close()) })

	gate := activationPrecompileGate()
	require.Equal(t, ActivationUpgrade, gate.Upgrade)
	want := map[common.Address][]string{
		common.HexToAddress("0x1012"): {upgradetypes.ModuleName},
		common.HexToAddress("0x1013"): {layerxcustodytypes.ModuleName},
		common.HexToAddress("0x1014"): {upgradetypes.ModuleName},
		common.HexToAddress("0x1015"): {layerxexchangetypes.ModuleName},
		common.HexToAddress("0x1016"): {layerxbridgetypes.ModuleName},
		common.HexToAddress("0x1017"): {launchpadtypes.ModuleName},
		common.HexToAddress("0x1018"): {layerxbridgetypes.ModuleName},
		common.HexToAddress("0x1019"): {xwebtypes.ModuleName},
	}
	require.Equal(t, want, gate.Modules)
	require.Contains(t, gate.Modules, common.HexToAddress(verifyprecompile.LayerXVerifyAddress))
	require.Equal(t, common.Address(layerxanchortypes.AnchorPrecompileAddress), common.HexToAddress(anchorprecompile.LayerXAnchorAddress))
	initial := a.EvmKeeper.CustomPrecompiles(ctx)
	for address := range gate.Modules {
		require.Contains(t, a.customPrecompiles, address)
		require.Contains(t, initial, address)
	}
	for address := range a.customPrecompiles {
		_, gated := gate.Modules[address]
		value := address.Big().Int64()
		require.Equal(t, value >= 0x1012 && value <= 0x1019, gated, address.Hex())
	}

	rewindBeforeActivation(t, a, ctx)
	a.refreshXWebPrecompile(ctx)
	below := a.EvmKeeper.CustomPrecompiles(ctx)
	for address, dependencies := range want {
		if dependencies[0] == upgradetypes.ModuleName {
			require.Contains(t, below, address)
		} else {
			require.NotContains(t, below, address)
		}
	}
	require.Equal(t, repairM1142Addresses(below), repairM1142Addresses(a.EvmKeeper.CustomPrecompiles(ctx.WithIsTracing(true))))
	a.UpgradeKeeper.ApplyUpgrade(ctx, upgradetypes.Plan{Name: ActivationUpgrade, Height: ctx.BlockHeight()})
	at := a.EvmKeeper.CustomPrecompiles(ctx)
	// Restoring exchange also satisfies the merge oracle's real dependency;
	// assert the complete resulting set, including every unchanged contract.
	oracle := common.HexToAddress("0x0903")
	require.Equal(t, []string{layerxagenttypes.ModuleName, layerxexchangetypes.ModuleName}, mergePrecompileGate().Modules[oracle])
	require.NotContains(t, below, oracle)
	require.Contains(t, initial, oracle)
	wantAt := make(map[common.Address]vm.PrecompiledContract, len(initial))
	for address, contract := range below {
		wantAt[address] = contract
	}
	for address := range want {
		wantAt[address] = initial[address]
	}
	wantAt[oracle] = initial[oracle]
	require.Equal(t, repairM1142Addresses(wantAt), repairM1142Addresses(at))
	for address, contract := range below {
		require.Same(t, contract, at[address])
	}
	for address := range gate.Modules {
		require.Contains(t, at, address)
	}
	require.Equal(t, repairM1142Addresses(at), repairM1142Addresses(a.EvmKeeper.CustomPrecompiles(ctx.WithIsTracing(true))))
	for _, tracing := range []bool{false, true} {
		requireRepairM1142RestoredCalls(t, a, ctx.WithIsTracing(tracing), below, false)
	}
	before := ctx.WithBlockHeight(ctx.BlockHeight() - 1)
	servedBefore := a.EvmKeeper.CustomPrecompiles(before)
	require.Len(t, servedBefore, len(at)-len(gate.Modules))
	wantBefore := make(map[common.Address]vm.PrecompiledContract, len(at)-len(gate.Modules))
	for address, contract := range at {
		if _, gated := gate.Modules[address]; !gated {
			wantBefore[address] = contract
		}
	}
	require.Equal(t, repairM1142Addresses(wantBefore), repairM1142Addresses(servedBefore))
	require.Equal(t, repairM1142Addresses(servedBefore), repairM1142Addresses(a.EvmKeeper.CustomPrecompiles(before.WithIsTracing(true))))
	for address := range gate.Modules {
		require.NotContains(t, servedBefore, address)
	}

	// The live upgrade dependency keeps the retirement surface available even
	// though anchor is absent from the module manager and version map.
	a.UpgradeKeeper.ApplyUpgrade(ctx, upgradetypes.Plan{Name: MergeUpgrade, Height: mergeHeight, Info: activationInfo(t)})
	require.NotContains(t, a.UpgradeKeeper.GetModuleVersionMap(ctx), layerxanchortypes.ModuleName)
	for _, tracing := range []bool{false, true} {
		requireAnchorRetirementRevert(t, a, ctx.WithIsTracing(tracing), anchorprecompile.FinalizeMethod, uint64(1))
		requireRepairM1142OracleCall(t, a, ctx.WithIsTracing(tracing), true)
	}
}

// writeActivationUpgradeInfo writes the upgrade info file an operator leaves for
// the activation plan, through the keeper that writes it and into the home the
// application reads, so the begin blocker reads exactly what a node reads.
func writeActivationUpgradeInfo(t *testing.T, a *App, height int64) {
	t.Helper()
	require.NoError(t, a.UpgradeKeeper.DumpUpgradeInfoToDisk(height, ActivationUpgrade))
	info, err := a.UpgradeKeeper.ReadUpgradeInfoFromDisk()
	require.NoError(t, err)
	require.Equal(t, ActivationUpgrade, info.Name)
	require.Equal(t, height, info.Height)
}

// recordPreviousProposer records the proposer of the block before the one the
// test runs, which the distribution module's begin blocker reads in every block
// after the first.
func recordPreviousProposer(a *App, ctx sdk.Context, valPub cryptotypes.PubKey) {
	a.DistrKeeper.SetPreviousProposerConsAddr(ctx, sdk.ConsAddress(valPub.Address()))
}

func TestActivationAppliesThePlanInTheBlockTheUpgradeInfoNames(t *testing.T) {
	valPub := secp256k1.GenPrivKey().PubKey()
	testWrapper := NewTestWrapper(t, time.Now().UTC(), valPub, true)
	a, ctx := testWrapper.App, testWrapper.Ctx
	rewindBeforeActivation(t, a, ctx)
	require.Empty(t, a.unmountedActivationStores())
	reporter, ok := a.CommitMultiStore().(pendingStoreReporter)
	require.True(t, ok)
	for _, name := range activationStoreUpgrades().Added {
		require.False(t, reporter.IsPendingStore(name), name)
	}

	const height = int64(42)
	writeActivationUpgradeInfo(t, a, height)
	at := ctx.WithBlockHeight(height)
	recordPreviousProposer(a, at, valPub)

	gate := activationPrecompileGate()
	for addr := range gate.Modules {
		require.NotContains(t, a.EvmKeeper.CustomPrecompiles(at), addr, addr.Hex())
	}

	a.BeginBlock(at, height, nil, nil, false)

	// The plan ran on the standard path: the handler initialised every module the
	// version map lacked, the done height is the block's own and no plan is left.
	require.Equal(t, height, a.UpgradeKeeper.GetDoneHeight(at, ActivationUpgrade))
	versions := a.UpgradeKeeper.GetModuleVersionMap(at)
	require.Equal(t, a.mm.GetVersionMap(), versions)
	for _, name := range activationModules() {
		require.Contains(t, versions, name)
	}
	require.True(t, a.activationModulesPresent(at))
	_, found := a.UpgradeKeeper.GetUpgradePlan(at)
	require.False(t, found)

	// The web module comes up paused on an empty attestor set, as its own genesis
	// leaves it.
	require.True(t, a.xwebInitialised(at))
	require.True(t, a.XWebKeeper.IsPaused(at))
	require.Empty(t, a.XWebKeeper.GetAttestorSet(at).Attestors)

	served := a.EvmKeeper.CustomPrecompiles(at)
	for addr := range gate.Modules {
		require.Contains(t, served, addr, addr.Hex())
	}
	below := a.EvmKeeper.CustomPrecompiles(at.WithBlockHeight(height - 1))
	require.Len(t, below, len(served)-len(gate.Modules))
	for addr := range gate.Modules {
		require.NotContains(t, below, addr, addr.Hex())
	}

	// The file is still on disk in the blocks that follow, and none of them applies
	// the plan a second time.
	next := ctx.WithBlockHeight(height + 1)
	recordPreviousProposer(a, next, valPub)
	a.BeginBlock(next, height+1, nil, nil, false)
	require.Equal(t, height, a.UpgradeKeeper.GetDoneHeight(next, ActivationUpgrade))
	require.Equal(t, versions, a.UpgradeKeeper.GetModuleVersionMap(next))
	_, found = a.UpgradeKeeper.GetUpgradePlan(next)
	require.False(t, found)
}

func TestActivationIgnoresAnUpgradeInfoThatNamesAnotherHeight(t *testing.T) {
	valPub := secp256k1.GenPrivKey().PubKey()
	testWrapper := NewTestWrapper(t, time.Now().UTC(), valPub, true)
	a, ctx := testWrapper.App, testWrapper.Ctx
	rewindBeforeActivation(t, a, ctx)

	const height = int64(42)
	writeActivationUpgradeInfo(t, a, height+5)
	at := ctx.WithBlockHeight(height)
	recordPreviousProposer(a, at, valPub)
	before := a.UpgradeKeeper.GetModuleVersionMap(at)

	a.BeginBlock(at, height, nil, nil, false)

	require.Zero(t, a.UpgradeKeeper.GetDoneHeight(at, ActivationUpgrade))
	require.Equal(t, before, a.UpgradeKeeper.GetModuleVersionMap(at))
	require.False(t, a.activationModulesPresent(at))
	_, found := a.UpgradeKeeper.GetUpgradePlan(at)
	require.False(t, found)
	for addr := range activationPrecompileGate().Modules {
		require.NotContains(t, a.EvmKeeper.CustomPrecompiles(at), addr, addr.Hex())
	}
	for _, name := range activationStoreUpgrades().Added {
		iter := at.KVStore(a.GetKey(name)).Iterator(nil, nil)
		require.False(t, iter.Valid(), name)
		require.NoError(t, iter.Close())
	}
}

func TestActivationStopsTheBlockTheStoreLoaderMountedNoActivationStoresFor(t *testing.T) {
	valPub := secp256k1.GenPrivKey().PubKey()
	testWrapper := NewTestWrapper(t, time.Now().UTC(), valPub, true)
	a, ctx := testWrapper.App, testWrapper.Ctx
	rewindBeforeActivation(t, a, ctx)

	const height = int64(42)
	writeActivationUpgradeInfo(t, a, height)
	at := ctx.WithBlockHeight(height)
	recordPreviousProposer(a, at, valPub)
	before := a.UpgradeKeeper.GetModuleVersionMap(at)

	// A process the upgrade store loader never ran in carries the plan's store keys
	// without the commit multistore holding a store for any of them.
	mounted := make(map[string]*sdk.KVStoreKey, len(activationStoreUpgrades().Added))
	for _, name := range activationStoreUpgrades().Added {
		mounted[name] = a.keys[name]
		a.keys[name] = sdk.NewKVStoreKey(name)
	}
	require.Equal(t, activationStoreUpgrades().Added, a.unmountedActivationStores())

	recovered := func() (value any) {
		defer func() { value = recover() }()
		a.BeginBlock(at, height, nil, nil, false)
		return nil
	}()
	for name, key := range mounted {
		a.keys[name] = key
	}

	require.NotNil(t, recovered)
	message := fmt.Sprint(recovered)
	require.Contains(t, message, "the store loader did not add the activation stores")
	require.Contains(t, message, "the upgrade info height must equal the last committed height plus one when the process starts")
	for _, name := range activationStoreUpgrades().Added {
		require.Contains(t, message, name)
	}

	// The block wrote nothing: no plan, no done height, no module and no store entry.
	require.Zero(t, a.UpgradeKeeper.GetDoneHeight(at, ActivationUpgrade))
	require.Equal(t, before, a.UpgradeKeeper.GetModuleVersionMap(at))
	require.False(t, a.activationModulesPresent(at))
	_, found := a.UpgradeKeeper.GetUpgradePlan(at)
	require.False(t, found)
	for _, name := range activationStoreUpgrades().Added {
		iter := at.KVStore(a.GetKey(name)).Iterator(nil, nil)
		require.False(t, iter.Valid(), name)
		require.NoError(t, iter.Close())
	}
}

func TestActivationStopsTheBlockWhoseStateCommitmentStoreServesAPlanStoreAsPending(t *testing.T) {
	valPub := secp256k1.GenPrivKey().PubKey()
	testWrapper := NewTestWrapper(t, time.Now().UTC(), valPub, true)
	before, ctx := testWrapper.App, testWrapper.Ctx
	rewindBeforeActivation(t, before, ctx)
	infoPath, err := before.UpgradeKeeper.GetUpgradeInfoPath()
	require.NoError(t, err)
	home := filepath.Dir(filepath.Dir(infoPath))

	// The rewound chain commits and the process stops.
	before.SetDeliverStateToCommit()
	before.WriteState().Commit(true)
	require.NoError(t, before.Close())

	// The state then loses the web store's tree, as a state that predates the
	// store carries none, while the binary that next opens it still mounts it.
	sc, err := composite.NewCompositeCommitStore(t.Context(), home, scconfig.DefaultStateCommitConfig())
	require.NoError(t, err)
	require.NoError(t, sc.Initialize(keys.MemIAVLStoreKeys))
	_, err = sc.LoadVersion(0, false)
	require.NoError(t, err)
	require.NoError(t, sc.ApplyUpgrades([]*storageproto.TreeNameUpgrade{{Name: xwebtypes.StoreKey, Delete: true}}))
	_, err = sc.Commit()
	require.NoError(t, err)
	require.NoError(t, sc.Close())

	// A process the upgrade store loader never ran in mounts every store, and its
	// state-commitment store serves the web store through the pending placeholder.
	a := New(dbm.NewMemDB(), nil, true, map[int64]bool{}, home, 1, true, config.TestConfig(), MakeEncodingConfig(),
		wasm.EnableAllProposals, TestAppOpts{}, EmptyWasmOpts, nil)
	t.Cleanup(func() { _ = a.Close() })
	key := a.GetKey(xwebtypes.StoreKey)
	require.NotNil(t, key)
	require.NotNil(t, a.CommitMultiStore().GetCommitKVStore(key))
	reporter, ok := a.CommitMultiStore().(pendingStoreReporter)
	require.True(t, ok)
	require.True(t, reporter.IsPendingStore(xwebtypes.StoreKey))
	require.Equal(t, []string{xwebtypes.StoreKey}, a.unmountedActivationStores())

	height := a.LastBlockHeight() + 1
	writeActivationUpgradeInfo(t, a, height)
	at := ctx.WithMultiStore(a.CommitMultiStore().CacheMultiStore()).WithBlockHeight(height)
	recordPreviousProposer(a, at, valPub)
	versions := a.UpgradeKeeper.GetModuleVersionMap(at)
	for _, name := range activationModules() {
		require.NotContains(t, versions, name)
	}

	recovered := func() (value any) {
		defer func() { value = recover() }()
		a.BeginBlock(at, height, nil, nil, false)
		return nil
	}()

	require.NotNil(t, recovered)
	message := fmt.Sprint(recovered)
	require.Contains(t, message, "the store loader did not add the activation stores "+xwebtypes.StoreKey+":")
	require.Contains(t, message, "the upgrade info height must equal the last committed height plus one when the process starts")

	// The block wrote nothing: no plan, no done height, no module and no store entry.
	require.Zero(t, a.UpgradeKeeper.GetDoneHeight(at, ActivationUpgrade))
	require.Equal(t, versions, a.UpgradeKeeper.GetModuleVersionMap(at))
	require.False(t, a.activationModulesPresent(at))
	_, found := a.UpgradeKeeper.GetUpgradePlan(at)
	require.False(t, found)
	for _, name := range activationStoreUpgrades().Added {
		iter := at.KVStore(a.GetKey(name)).Iterator(nil, nil)
		require.False(t, iter.Valid(), name)
		require.NoError(t, iter.Close())
	}
}
