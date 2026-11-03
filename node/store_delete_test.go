package app

import (
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	launchpadtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	anchortypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxanchor/types"
	ledgertypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	anchorprecompile "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxanchor"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	bankkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/keeper"
	upgradetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/types"
	storekeys "github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/keys"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

func TestAnchorStoreDeletedAtStep(t *testing.T) {
	require.Equal(t, "layerxanchor", retiredAnchorStoreName)
	require.Contains(t, storekeys.MemIAVLStoreKeys, retiredAnchorStoreName)
	require.True(t, storekeys.IsMemIAVLStoreKey(retiredAnchorStoreName))
	remaining, err := storekeys.AllModulesExcept(retiredAnchorStoreName)
	require.NoError(t, err)
	require.NotContains(t, remaining, retiredAnchorStoreName)
	require.Equal(t, "v7.0-anchor-store-delete", AnchorStoreDeleteUpgrade)
	plans := knownUpgradePlans()
	require.Equal(t, []string{MergeUpgrade, AnchorStoreDeleteUpgrade}, plans[len(plans)-2:])
	upgrades, ok := layerxStoreUpgrades(AnchorStoreDeleteUpgrade)
	require.True(t, ok)
	require.Equal(t, []string{retiredAnchorStoreName}, upgrades.Deleted)
	require.Empty(t, upgrades.Added)
	require.Empty(t, upgrades.Renamed)
	merge, ok := layerxStoreUpgrades(MergeUpgrade)
	require.True(t, ok)
	require.Empty(t, merge.Deleted)
	require.Contains(t, kvStoreKeyNames, retiredAnchorStoreName)

	w := NewTestWrapper(t, time.Now().UTC(), secp256k1.GenPrivKey().PubKey(), false)
	a, ctx := w.App, w.Ctx.WithBlockHeight(100)
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	require.NotNil(t, a.GetKey(retiredAnchorStoreName))
	require.Equal(t, anchortypes.StoreKey, a.GetKey(retiredAnchorStoreName).Name())
	require.NotContains(t, a.mm.Modules, anchortypes.ModuleName)
	require.NotContains(t, a.mm.OrderInitGenesis, anchortypes.ModuleName)
	require.True(t, a.UpgradeKeeper.HasHandler(AnchorStoreDeleteUpgrade))
	params := ledgertypes.DefaultParams()
	params.Paused = true
	require.NoError(t, a.LayerXLedgerKeeper.SetParams(ctx, params))
	ctx.KVStore(a.GetKey(upgradetypes.StoreKey)).Delete(append([]byte{upgradetypes.VersionMapByte}, []byte(ledgertypes.ModuleName)...))
	require.NotContains(t, a.UpgradeKeeper.GetModuleVersionMap(ctx), ledgertypes.ModuleName)
	a.UpgradeKeeper.SetDone(ctx.WithBlockHeight(99), MergeUpgrade)
	plan := upgradetypes.Plan{Name: AnchorStoreDeleteUpgrade, Height: ctx.BlockHeight()}
	require.NoError(t, a.UpgradeKeeper.ScheduleUpgrade(ctx, plan))
	a.UpgradeKeeper.ApplyUpgrade(ctx, plan)
	require.Equal(t, a.mm.GetVersionMap(), a.UpgradeKeeper.GetModuleVersionMap(ctx))
	require.Equal(t, ledgertypes.DefaultGenesis(), a.LayerXLedgerKeeper.ExportGenesis(ctx))
	require.Equal(t, ctx.BlockHeight(), a.UpgradeKeeper.GetDoneHeight(ctx, AnchorStoreDeleteUpgrade))
	require.Equal(t, int64(99), a.UpgradeKeeper.GetDoneHeight(ctx, MergeUpgrade))
}

func TestAnchorPrecompileStaysRegisteredWithoutModule(t *testing.T) {
	address := common.HexToAddress(anchorprecompile.LayerXAnchorAddress)
	require.Equal(t, common.HexToAddress("0x1014"), address)
	modules, registered := activationPrecompileGate().Modules[address]
	require.True(t, registered)
	require.NotContains(t, modules, anchortypes.ModuleName)

	w := NewTestWrapper(t, time.Now().UTC(), secp256k1.GenPrivKey().PubKey(), true)
	a, ctx := w.App, w.Ctx.WithBlockHeight(100)
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	require.NotContains(t, a.mm.Modules, anchortypes.ModuleName)
	require.NotContains(t, a.UpgradeKeeper.GetModuleVersionMap(ctx), anchortypes.ModuleName)
	require.Zero(t, a.UpgradeKeeper.GetDoneHeight(ctx, ActivationUpgrade))
	before := a.EvmKeeper.CustomPrecompiles(ctx)[address]
	require.NotNil(t, before)
	precompile, ok := before.(*pcommon.Precompile)
	require.True(t, ok)
	contract := precompile.GetABI()
	input, err := contract.Pack(anchorprecompile.LatestFinalizedMethod)
	require.NoError(t, err)
	caller := common.Address{1}
	a.UpgradeKeeper.SetDone(ctx.WithBlockHeight(99), MergeUpgrade)
	plan := upgradetypes.Plan{Name: AnchorStoreDeleteUpgrade, Height: ctx.BlockHeight()}
	require.NoError(t, a.UpgradeKeeper.ScheduleUpgrade(ctx, plan))
	stateDB := state.NewDBImpl(ctx.WithBlockHeight(plan.Height-1), &a.EvmKeeper, true)
	evm := &vm.EVM{StateDB: stateDB}
	output, err := before.Run(evm, caller, caller, input, nil, true, false, nil)
	require.NoError(t, err)
	values, err := contract.Unpack(anchorprecompile.LatestFinalizedMethod, output)
	require.NoError(t, err)
	require.Equal(t, []interface{}{uint64(0), false}, values)
	a.UpgradeKeeper.ApplyUpgrade(ctx, plan)
	for _, height := range []int64{plan.Height, plan.Height + 1} {
		for _, tracing := range []bool{false, true} {
			at := ctx.WithBlockHeight(height).WithIsTracing(tracing)
			registered := a.EvmKeeper.CustomPrecompiles(at)[address]
			require.Same(t, before, registered)
			require.Contains(t, a.EvmKeeper.GetCustomPrecompilesVersions(at), address)
			stateDB = state.NewDBImpl(at, &a.EvmKeeper, true)
			evm = &vm.EVM{StateDB: stateDB}
			output, err = registered.Run(evm, caller, caller, input, nil, true, false, nil)
			require.ErrorIs(t, err, vm.ErrExecutionReverted)
			reason, err := abi.UnpackRevert(output)
			require.NoError(t, err)
			require.Equal(t, "LXP_MODULE_DISABLED: module disabled; layerxAnchor retired", reason)
		}
	}
	require.NotNil(t, NewPrecompileKeepers(a).AnchorK())
}

type RegisterBalanceChangeHook func(string, bankkeeper.BalanceChangeHook)

func TestBankKeepersCarryNoLaunchpadBalanceHook(t *testing.T) {
	w := NewTestWrapper(t, time.Now().UTC(), secp256k1.GenPrivKey().PubKey(), false)
	a := w.App
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	legacyLaunchpad := NewPrecompileKeepers(a).LaunchpadV610K()
	require.NotNil(t, legacyLaunchpad)
	hook := legacyLaunchpad.BeforeBalanceChange
	for name, register := range map[string]RegisterBalanceChangeHook{
		"BankKeeper":     a.BankKeeper.(bankkeeper.BaseKeeper).RegisterBalanceChangeHook,
		"GigaBankKeeper": a.GigaBankKeeper.RegisterBalanceChangeHook,
	} {
		t.Run(name, func(t *testing.T) {
			require.NotPanics(t, func() {
				register(launchpadtypes.ModuleName, hook)
			})
			require.PanicsWithValue(t, "bank: duplicate balance change hook \"launchpad\"", func() {
				register(launchpadtypes.ModuleName, hook)
			})
		})
	}
}
